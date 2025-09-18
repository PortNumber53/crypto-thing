package root

import (
	"fmt"

	"github.com/spf13/cobra"

	"cryptool/internal/coinbase"
	"cryptool/internal/config"
	"cryptool/internal/ingest"
)

func newCoinbaseProductsSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync-products",
		Short: "Sync all tradeable products from Coinbase",
		Long:  `Fetches all available tradeable products from the Coinbase Advanced Trade API and upserts them into the local database.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.FromContext(cmd.Context())

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
			client.Configure(cfg.Coinbase.RPM, cfg.Coinbase.MaxRetries, cfg.Coinbase.BackoffMS, cfg.App.Verbose)

			fmt.Println("Fetching products from Coinbase...")
			products, err := client.GetProducts(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to get products: %w", err)
			}
			fmt.Printf("Found %d products.\n", len(products))

			store := ingest.NewStore(cfg.Database.URL)
			fmt.Println("Upserting products into database...")
			rowsAffected, err := store.UpsertProducts(cmd.Context(), "coinbase", products)
			if err != nil {
				return fmt.Errorf("failed to upsert products: %w", err)
			}

			fmt.Printf("Sync complete. Upserted %d products.\n", rowsAffected)
			return nil
		},
	}
	return cmd
}
