package root

import "github.com/spf13/cobra"

func NewExchangeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exchange",
		Short: "Exchange related commands",
	}
	cmd.AddCommand(NewCoinbaseCmd())
	return cmd
}
