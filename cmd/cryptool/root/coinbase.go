package root

import "github.com/spf13/cobra"

func NewCoinbaseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "coinbase",
		Short: "Coinbase Advanced Trade commands",
	}
	cmd.AddCommand(newCoinbaseDataCmd())
	cmd.AddCommand(newCoinbaseWalletCmd())
	return cmd
}
