package root

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"cryptool/internal/coinbase"
	"cryptool/internal/config"
)

func newCoinbaseWalletSyncDownCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "syncdown",
		Short: "Fetch and display account balances from Coinbase",
		Long:  `Fetches all account balances from the Coinbase Advanced Trade API and displays them in a table.`,
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

			accounts, err := client.ListAccounts(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to list accounts: %w", err)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "UUID\tCurrency\tAvailable\tHold")
			for _, acc := range accounts {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", acc.UUID, acc.Currency, acc.AvailableBalance.Value, acc.Hold.Value)
			}
			return w.Flush()
		},
	}
	return cmd
}
