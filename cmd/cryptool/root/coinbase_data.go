package root

import "github.com/spf13/cobra"

func newCoinbaseDataCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "data",
		Short: "Data related commands for Coinbase",
	}
	cmd.AddCommand(newCoinbaseDataFetchCmd())
	return cmd
}
