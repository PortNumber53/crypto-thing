package subcmds

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
		Use:   "fetch <start-date> <end-date>",
		Short: "Fetch historical candles from Coinbase",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return errors.New("requires <start-date> <end-date>")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.FromContext(cmd.Context())
			if product == "" {
				return errors.New("--product is required, e.g. BTC-USD")
			}

			start, err := parseDate(args[0])
			if err != nil {
				return fmt.Errorf("invalid start-date: %w", err)
			}
			end, err := parseDate(args[1])
			if err != nil {
				return fmt.Errorf("invalid end-date: %w", err)
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
			client.Configure(cfg.Coinbase.RPM, cfg.Coinbase.MaxRetries, cfg.Coinbase.BackoffMS)
			// Use command context to allow CTRL+C to cancel
            ctx := cmd.Context()
            // Batch over 350 buckets per request
            secPerBucket := granularitySeconds(granularity)
            if secPerBucket == 0 {
                return fmt.Errorf("unsupported granularity: %s", granularity)
            }
            maxBuckets := int64(350)
            batchCount := 0
            totalInserted := 0
            store := ingest.NewStore(cfg.Database.URL)

            cursor := start.UTC()
            endUTC := end.UTC()
            for cursor.Before(endUTC) {
                // Respect context cancellation
                select {
                case <-ctx.Done():
                    return ctx.Err()
                default:
                }

                windowEnd := cursor.Add(time.Duration(secPerBucket*maxBuckets) * time.Second)
                if windowEnd.After(endUTC) {
                    windowEnd = endUTC
                }
                // Gap-filling: skip fetch if the window is already fully populated in DB
                expected := int((windowEnd.Unix() - cursor.Unix()) / secPerBucket)
                if expected > 0 {
                    existing, err := store.CountCandlesInRange(ctx, "coinbase", product, cursor, windowEnd)
                    if err != nil {
                        return fmt.Errorf("count existing candles error: %w", err)
                    }
                    if existing >= expected {
                        batchCount++
                        fmt.Printf("[%d] skipped window fully present for %s (%s to %s)\n", batchCount, product, cursor.Format(time.RFC3339), windowEnd.Format(time.RFC3339))
                        cursor = windowEnd
                        continue
                    }
                }

                candles, err := client.GetCandlesOnce(ctx, product, cursor, windowEnd, granularity, maxBuckets)
                if err != nil {
                    return fmt.Errorf("coinbase candles batch error [%s - %s]: %w", cursor.Format(time.RFC3339), windowEnd.Format(time.RFC3339), err)
                }
                if len(candles) > 0 {
                    if err := store.InsertCandles(ctx, "coinbase", product, candles); err != nil {
                        return fmt.Errorf("store candles error: %w", err)
                    }
                    totalInserted += len(candles)
                }
                batchCount++
                fmt.Printf("[%d] inserted %d candles for %s (%s to %s)\n", batchCount, len(candles), product, cursor.Format(time.RFC3339), windowEnd.Format(time.RFC3339))
                cursor = windowEnd
            }

            fmt.Printf("Inserted %d candles total for %s\n", totalInserted, product)
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
