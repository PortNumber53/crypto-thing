package root

import (
	"embed"
	"fmt"

	"github.com/spf13/cobra"
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
	rootCmd.PersistentFlags().StringVar(&cfgPath, "config", "", "path to config file (default: reads .env from current directory, then uses CRYPTO_CONFIG_FILE variable)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
	rootCmd.PersistentFlags().StringVar(&coinbaseCreds, "coinbase-creds", "", "path to coinbase credentials json file")
}

func Execute(migrationsFS embed.FS) error {
	rootCmd.AddCommand(NewMigrateCmd(migrationsFS))
	rootCmd.AddCommand(NewExchangeCmd())
	rootCmd.AddCommand(NewDaemonCmd())
	rootCmd.AddCommand(NewClientCmd())
	return rootCmd.Execute()
}

// NewDaemonCmd creates the daemon command
func NewDaemonCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "daemon",
		Short: "Start the crypto tool as a daemon with websocket interface",
		Long: `Start the crypto tool as a daemon that exposes a websocket interface
for other applications to send commands to. The daemon runs continuously and
accepts commands like migrate:status, coinbase:fetch, etc.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("daemon functionality not yet implemented")
		},
	}
}

// NewClientCmd creates the client command
func NewClientCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "client",
		Short: "Connect to crypto daemon via websocket",
		Long: `Connect to a running crypto daemon via websocket and send commands interactively.
The daemon must be running for this to work.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("client functionality not yet implemented")
		},
	}
}
