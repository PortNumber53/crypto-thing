package root

import "github.com/spf13/cobra"

func newCoinbaseWalletCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "wallet",
        Short: "Wallet related commands for Coinbase",
    }
    cmd.AddCommand(newCoinbaseWalletSyncDownCmd())
    return cmd
}
