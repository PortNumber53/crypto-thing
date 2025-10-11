package root

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

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
	rootCmd.AddCommand(NewServerCmd())
	rootCmd.AddCommand(NewJobsCmd())
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

func NewServerCmd() *cobra.Command {
	serverCmd := &cobra.Command{Use: "server"}
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show daemon status and active jobs",
		RunE: func(cmd *cobra.Command, args []string) error {
			port := os.Getenv("DAEMON_PORT")
			if port == "" { port = "40000" }
			resp, err := http.Get("http://localhost:" + port + "/status")
			if err != nil { return fmt.Errorf("request failed: %w", err) }
			defer resp.Body.Close()
			b, _ := io.ReadAll(resp.Body)
			var pretty map[string]interface{}
			if json.Unmarshal(b, &pretty) == nil {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(pretty)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(b))
			return nil
		},
	}
	serverCmd.AddCommand(statusCmd)
	return serverCmd
}

func NewJobsCmd() *cobra.Command {
	jobsCmd := &cobra.Command{Use: "jobs"}
	killCmd := &cobra.Command{
		Use:   "kill <ID>",
		Short: "Ask daemon to stop processing a job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			port := os.Getenv("DAEMON_PORT")
			if port == "" { port = "40000" }
			url := fmt.Sprintf("http://localhost:%s/jobs/kill?id=%s", port, id)
			resp, err := http.Get(url)
			if err != nil { return fmt.Errorf("request failed: %w", err) }
			defer resp.Body.Close()
			b, _ := io.ReadAll(resp.Body)
			if resp.StatusCode >= 300 {
				return fmt.Errorf("daemon error: %s", string(b))
			}
			var pretty map[string]interface{}
			if json.Unmarshal(b, &pretty) == nil {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(pretty)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(b))
			return nil
		},
	}
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List active daemon jobs",
		RunE: func(cmd *cobra.Command, args []string) error {
			port := os.Getenv("DAEMON_PORT")
			if port == "" { port = "40000" }
			resp, err := http.Get("http://localhost:" + port + "/status")
			if err != nil { return fmt.Errorf("request failed: %w", err) }
			defer resp.Body.Close()
			b, _ := io.ReadAll(resp.Body)
			// Print only jobs if possible
			var payload map[string]interface{}
			if json.Unmarshal(b, &payload) == nil {
				jobs, _ := payload["jobs"]
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if jobs != nil {
					return enc.Encode(jobs)
				}
				return enc.Encode(payload)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(b))
			return nil
		},
	}
	jobsCmd.AddCommand(killCmd)
	jobsCmd.AddCommand(listCmd)
	return jobsCmd
}
