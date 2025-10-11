package root

import (
	"fmt"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"cryptool/internal/coinbase"
	"cryptool/internal/config"
	"cryptool/internal/ingest"
)

func newCoinbaseHistoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Fetch 1m candles for all products",
		Long:  `Iterates through all known, tradable products and fetches their 1-minute candle history, filling any gaps.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.FromContext(cmd.Context())
			store := ingest.NewStore(cfg.Database.URL)
			ctx := cmd.Context()

			products, err := store.GetAllProducts(ctx, "coinbase")
			if err != nil {
				return fmt.Errorf("failed to get products: %w", err)
			}

			fmt.Printf("Found %d products to sync\n", len(products))

			// Prefer JWT auth when configured; else fall back to HMAC headers
			var client *coinbase.Client
			if cfg.Coinbase.APIKeyName != "" && cfg.Coinbase.APIPrivateKey != "" {
				jwtClient, err := coinbase.NewClientWithJWT(cfg.Coinbase.APIKeyName, cfg.Coinbase.APIPrivateKey)
				if err != nil {
					return fmt.Errorf("jwt client init: %w", err)
				}
				client = jwtClient
			} else {
				client = coinbase.NewClient(cfg.Coinbase.APIKey, cfg.Coinbase.APISecret, cfg.Coinbase.Passphrase)
			}
			// Apply rate limiting and retries
			client.Configure(cfg.Coinbase.RPM, cfg.Coinbase.MaxRetries, cfg.Coinbase.BackoffMS, cfg.App.Verbose)

			// Helper to clamp to later of two times
			maxTime := func(a, b time.Time) time.Time {
				if a.After(b) { return a }
				return b
			}

			// Pre-compute per-product start dates and global earliest product start
			productStarts := make(map[string]time.Time, len(products))
			globalMin := time.Now().UTC()
			for _, p := range products {
				s, err := store.GetProductNewAt(ctx, "coinbase", p)
				if err != nil {
					fmt.Printf("SKIPPING: could not get start date for %s: %v\n", p, err)
					continue
				}
				productStarts[p] = s
				if s.Before(globalMin) {
					globalMin = s
				}
			}

			granularity := "1m"
			secPerBucket := granularitySeconds(granularity)
			maxBuckets := int64(350)

			// Per-product range fetcher reusing existing logic
			fetchProductRange := func(product string, start, end time.Time) (int, error) {
				totalInserted := 0
				batchCount := 0

				var fetchRecursive func(start, end time.Time) error
				fetchRecursive = func(start, end time.Time) error {
					gapsToFill, err := store.CountGapsToFill(ctx, "coinbase", product, start, end, int(secPerBucket))
					if err != nil {
						return fmt.Errorf("failed to count gaps to fill in range: %w", err)
					}
					if gapsToFill == 0 {
						return nil
					}

					windowSize := int(end.Sub(start).Seconds() / float64(secPerBucket))
					if windowSize < int(maxBuckets) {
						batchCount++
						fmt.Printf("  [%s] Batch %d: %d potential gaps in [%s - %s]\n", product, batchCount, gapsToFill, start.Format(time.RFC3339), end.Format(time.RFC3339))

						apiEnd := end.Add(-time.Second)
						candles, err := client.GetCandlesOnce(ctx, product, start, apiEnd, granularity, maxBuckets)
						if err != nil {
							return fmt.Errorf("coinbase candles batch error: %w", err)
						}

						insertedInBatch, err := store.InsertCandles(ctx, "coinbase", product, candles)
						if err != nil {
							return fmt.Errorf("insert candles: %w", err)
						}
						fmt.Printf("         -> inserted %d of %d candles\n", insertedInBatch, len(candles))
						totalInserted += insertedInBatch

						missingTimestamps, err := store.GetMissingCandleTimestamps(ctx, "coinbase", product, start, end, int(secPerBucket))
						if err != nil {
							return fmt.Errorf("failed to get missing timestamps post-fetch: %w", err)
						}

						const numGapWorkers = 10
						gapJobs := make(chan time.Time, len(missingTimestamps))
						var gapWg sync.WaitGroup

						for w := 1; w <= numGapWorkers; w++ {
							gapWg.Add(1)
							go func() {
								defer gapWg.Done()
								for t := range gapJobs {
									fakeCandle := []coinbase.Candle{{Time: t, Volume: -1}}
									if _, err := store.InsertCandles(ctx, "coinbase", product, fakeCandle); err != nil {
										fmt.Printf("         -> error marking gap for %s: %v\n", t.Format(time.RFC3339), err)
										continue
									}
									fmt.Printf("         -> marking gap at %s as empty\n", t.Format(time.RFC3339))
								}
							}()
						}
						for _, t := range missingTimestamps {
							gapJobs <- t
						}
						close(gapJobs)
						gapWg.Wait()
						return nil
					}

					mid := start.Add(end.Sub(start) / 2).Truncate(time.Duration(secPerBucket) * time.Second)
					if err := fetchRecursive(start, mid); err != nil { return err }
					return fetchRecursive(mid, end)
				}

				if err := fetchRecursive(start, end); err != nil {
					return totalInserted, err
				}
				return totalInserted, nil
			}

			// Day-by-day across products: today back to earliest product start
			now := time.Now().UTC()
			todayEnd := now.Truncate(24 * time.Hour).Add(24 * time.Hour) // exclusive end-of-today
			for dayEnd := todayEnd; dayEnd.After(globalMin); dayEnd = dayEnd.AddDate(0, 0, -1) {
				dayStart := dayEnd.Add(-24 * time.Hour)
				fmt.Printf("\n=== Day window: [%s - %s) ===\n", dayStart.Format("2006-01-02"), dayEnd.Format("2006-01-02"))

				for _, product := range products {
					pStart, ok := productStarts[product]
					if !ok {
						continue
					}
					if pStart.After(dayEnd) {
						continue // product did not exist yet in this window
					}
					effStart := maxTime(pStart, dayStart)
					inserted, err := fetchProductRange(product, effStart, dayEnd)
					if err != nil {
						fmt.Printf("ERROR for %s in day [%s]: %v\n", product, dayStart.Format("2006-01-02"), err)
						continue
					}
					if inserted > 0 {
						fmt.Printf("Inserted %d candles for %s in %s\n", inserted, product, dayStart.Format("2006-01-02"))
					}
				}
			}

			fmt.Println("\n--- All day windows processed ---")
			return nil
		},
	}
	return cmd
}
