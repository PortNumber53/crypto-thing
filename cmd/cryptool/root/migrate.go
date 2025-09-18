package root

import (
	"embed"
	"fmt"
	"github.com/spf13/cobra"

	"cryptool/internal/config"
	"cryptool/internal/migrate"
)

func NewMigrateCmd(migrationsFS embed.FS) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Database migration commands",
	}

	// Helper to get config from context
	getConfig := func(cmd *cobra.Command) *config.Config {
		return config.FromContext(cmd.Context())
	}

	// Status command
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show migration status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := getConfig(cmd)
			if err := migrate.Status(cmd.Context(), cfg.Database.URL, migrationsFS); err != nil {
				return fmt.Errorf("status failed: %w", err)
			}
			return nil
		},
	}

	// Up command
	upCmd := &cobra.Command{
		Use:   "up",
		Short: "Apply all up migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := getConfig(cmd)
			if err := migrate.Up(cmd.Context(), cfg.Database.URL, migrationsFS); err != nil {
				return fmt.Errorf("up failed: %w", err)
			}
			return nil
		},
	}

	// Down command
	var steps int
	downCmd := &cobra.Command{
		Use:   "down",
		Short: "Roll back a migration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := getConfig(cmd)
			if err := migrate.Down(cmd.Context(), cfg.Database.URL, steps, migrationsFS); err != nil {
				return fmt.Errorf("down failed: %w", err)
			}
			return nil
		},
	}
	downCmd.Flags().IntVar(&steps, "step", 1, "number of migrations to roll back")

	// Reset command
	resetCmd := &cobra.Command{
		Use:   "reset",
		Short: "Reset database (down to 0, then up)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := getConfig(cmd)
			if err := migrate.Reset(cmd.Context(), cfg.Database.URL, migrationsFS); err != nil {
				return fmt.Errorf("reset failed: %w", err)
			}
			return nil
		},
	}

	cmd.AddCommand(statusCmd, upCmd, downCmd, resetCmd)
	return cmd
}
