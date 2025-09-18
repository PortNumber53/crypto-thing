package root

import (
	"fmt"

	"github.com/spf13/cobra"

	"cryptool/cmd/cryptool/root/subcmds"
	"cryptool/internal/config"
)

var (
	cfgPath      string
	appCfg       *config.Config
	verbose      bool
	coinbaseCreds string
)

var rootCmd = &cobra.Command{
	Use:   "cryptool",
	Short: "cryptool manages crypto data and DB migrations",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Load config once before running subcommands
		c, err := config.Load(cfgPath, coinbaseCreds)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		appCfg = c
		appCfg.App.Verbose = verbose
		// Pass down via context
		ctx := config.WithConfig(cmd.Context(), appCfg)
		cmd.SetContext(ctx)
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgPath, "config", "", "path to config.ini (default: ~/.config/crypto-thing/config.ini)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
	rootCmd.PersistentFlags().StringVar(&coinbaseCreds, "coinbase-creds", "", "path to coinbase credentials json file")

	// Register subcommands
	rootCmd.AddCommand(subcmds.NewMigrateCmd())
	rootCmd.AddCommand(subcmds.NewExchangeCmd())
}

func Execute() error {
	return rootCmd.Execute()
}
