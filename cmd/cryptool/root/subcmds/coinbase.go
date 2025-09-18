package subcmds

import "github.com/spf13/cobra"

func NewCoinbaseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "coinbase",
		Short: "Coinbase Advanced Trade commands",
	}
	cmd.AddCommand(newCoinbaseDataCmd())
	return cmd
}
