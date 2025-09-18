package root

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"cryptool/internal/coinbase"
	"cryptool/internal/config"
	"cryptool/internal/ingest"
)

func newCoinbaseDataFetchCmd() *cobra.Command {
	var (
		product     string
		granularity string
	)

	cmd := &cobra.Command{
		Use:   "fetch [start-date] [end-date]",
		Short: "Fetch historical candles, filling any gaps",
		Long: `Fetches historical candle data from Coinbase for a given product.

This command intelligently identifies and fills any gaps in the local database. If start-date and end-date are omitted, it will backfill all data from the product's launch date to the present.`,
		Args: cobra.MaximumNArgs(2), 
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.FromContext(cmd.Context())
			if product == "" {
				return errors.New("--product is required, e.g. BTC-USD")
			}

			var start, end time.Time
			var err error
			store := ingest.NewStore(cfg.Database.URL)

			if len(args) > 0 {
				start, err = parseDate(args[0])
				if err != nil {
					return fmt.Errorf("invalid start-date: %w", err)
				}
			} else {
				start, err = store.GetProductNewAt(cmd.Context(), "coinbase", product)
				if err != nil {
					return fmt.Errorf("get product new_at: %w", err)
				}
			}

			if len(args) > 1 {
				end, err = parseDate(args[1])
				if err != nil {
					return fmt.Errorf("invalid end-date: %w", err)
				}
			} else {
				end = time.Now()
			}
			if !end.After(start) {
				return errors.New("end-date must be after start-date")
			}

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

			// Validate product ID
			products, err := client.GetProducts(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to get products for validation: %w", err)
			}
			validProduct := false
			for _, p := range products {
				if p.ProductID == product {
					validProduct = true
					break
				}
			}
			if !validProduct {
				return fmt.Errorf("invalid product ID: %s", product)
			}

						ctx := cmd.Context()

			secPerBucket := granularitySeconds(granularity)
			maxBuckets := int64(350)
			batchCount := 0
			totalInserted := 0

			var fetchRecursive func(start, end time.Time) error
			fetchRecursive = func(start, end time.Time) error {
				// 1. Count expected vs. actual candles in the range
				expectedCandles := int(end.Sub(start).Seconds() / float64(secPerBucket))
				if expectedCandles == 0 {
					return nil // Nothing to fetch
				}

				actualCandles, err := store.CountCandlesInRange(ctx, "coinbase", product, start, end)
				if err != nil {
					return fmt.Errorf("failed to count candles in range: %w", err)
				}

				missingCandles := expectedCandles - actualCandles
				if missingCandles <= 0 {
					return nil // Range is fully populated
				}

				// 2. If missing candles are within a single API call limit, fetch them
				if expectedCandles <= int(maxBuckets) {
					batchCount++
					fmt.Printf("Batch %d: fetching %d missing candles in [%s - %s]\n", batchCount, missingCandles, start.Format(time.RFC3339), end.Format(time.RFC3339))

					candles, err := client.GetCandlesOnce(ctx, product, start, end, granularity, maxBuckets)
					if err != nil {
						return fmt.Errorf("coinbase candles batch error: %w", err)
					}

					insertedInBatch, err := store.InsertCandles(ctx, "coinbase", product, candles)
					if err != nil {
						return fmt.Errorf("insert candles: %w", err)
					}
					fmt.Printf("         -> inserted %d of %d candles\n", insertedInBatch, len(candles))
					totalInserted += insertedInBatch
					return nil
				}

				// 3. If too many missing, split the range and recurse
				mid := start.Add(end.Sub(start) / 2)
				// Align mid to the granularity bucket
				mid = mid.Truncate(time.Duration(secPerBucket) * time.Second)

				if err := fetchRecursive(start, mid); err != nil {
					return err
				}
				return fetchRecursive(mid, end)
			}

			if err := fetchRecursive(start, end); err != nil {
				return err
			}

			fmt.Printf("Fetch complete. Inserted %d new candles.\n", totalInserted)

			return nil
		},
	}
	cmd.Flags().StringVar(&product, "product", "", "product id, e.g. BTC-USD")
	cmd.Flags().StringVar(&granularity, "granularity", "1h", "candle granularity, e.g., 1m, 5m, 15m, 30m, 1h, 2h, 6h, 1d")
	return cmd
}

func parseDate(s string) (time.Time, error) {
	// Accept RFC3339 or YYYY-MM-DD
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("unsupported date format: %s", s)
}

// granularitySeconds maps user granularity inputs to seconds per bucket
func granularitySeconds(g string) int64 {
    switch g {
    case "1m":
        return 60
    case "5m":
        return 5 * 60
    case "15m":
        return 15 * 60
    case "30m":
        return 30 * 60
    case "1h":
        return 60 * 60
    case "2h":
        return 2 * 60 * 60
    case "6h":
        return 6 * 60 * 60
    case "1d":
        return 24 * 60 * 60
    default:
        return 3600 // 1h
    }
}
