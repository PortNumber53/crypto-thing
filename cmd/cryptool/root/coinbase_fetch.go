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
		Short: "Fetch historical candles from Coinbase",
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

			gaps, err := store.GetMissingCandleRanges(ctx, "coinbase", product, start, end, secPerBucket)
			if err != nil {
				return fmt.Errorf("failed to get missing candle ranges: %w", err)
			}

			if len(gaps) == 0 {
				fmt.Println("No data to fetch. All candles are up to date.")
				return nil
			}

			for _, gap := range gaps {
				cursor := gap.Start
				for cursor.Before(gap.End) {
					batchCount++
					windowEnd := cursor.Add(time.Duration(secPerBucket*maxBuckets) * time.Second)
					if windowEnd.After(gap.End) {
						windowEnd = gap.End
					}

					fmt.Printf("Batch %d: fetching [%s - %s]\n", batchCount, cursor.Format(time.RFC3339), windowEnd.Format(time.RFC3339))

					candles, err := client.GetCandlesOnce(ctx, product, cursor, windowEnd, granularity, maxBuckets)
					if err != nil {
						return fmt.Errorf("coinbase candles batch error [%s - %s]: %w", cursor.Format(time.RFC3339), windowEnd.Format(time.RFC3339), err)
					}

					insertedInBatch, err := store.InsertCandles(ctx, "coinbase", product, candles)
					if err != nil {
						return fmt.Errorf("insert candles: %w", err)
					}
					fmt.Printf("         -> inserted %d of %d candles\n", insertedInBatch, len(candles))
					totalInserted += insertedInBatch

					cursor = windowEnd
				}
			}

			fmt.Printf("Fetch complete. Inserted %d new candles.\n", totalInserted)

			return nil
		},
	}
	cmd.Flags().StringVar(&product, "product", "", "product id, e.g. BTC-USD")
	cmd.Flags().StringVar(&granularity, "granularity", "1h", "granularity (1m,5m,15m,30m,1h,2h,6h,1d)")
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
        return 0
    }
}
