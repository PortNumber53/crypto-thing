package subcmds

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"cryptool/internal/config"
	"cryptool/internal/migrate"
)

func NewMigrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Database migration commands",
	}

	cmd.AddCommand(newMigrateStatusCmd())
	cmd.AddCommand(newMigrateUpCmd())
	cmd.AddCommand(newMigrateDownCmd())
	cmd.AddCommand(newMigrateResetCmd())
	return cmd
}

func newMigrateStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show migration status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.FromContext(cmd.Context())
			ctx := context.Background()
			return migrate.Status(ctx, cfg.Database.URL)
		},
	}
}

func newMigrateUpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "up",
		Short: "Apply all pending migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.FromContext(cmd.Context())
			ctx := context.Background()
			return migrate.Up(ctx, cfg.Database.URL)
		},
	}
}

func newMigrateDownCmd() *cobra.Command {
	var steps int
	c := &cobra.Command{
		Use:   "down",
		Short: "Rollback migrations (default 1 step)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.FromContext(cmd.Context())
			ctx := context.Background()
			if steps <= 0 {
				steps = 1
			}
			return migrate.Down(ctx, cfg.Database.URL, steps)
		},
	}
	c.Flags().IntVar(&steps, "step", 1, "number of steps to rollback")
	return c
}

func newMigrateResetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reset",
		Short: "Reset database (down to 0, then up)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.FromContext(cmd.Context())
			ctx := context.Background()
			if err := migrate.Reset(ctx, cfg.Database.URL); err != nil {
				return fmt.Errorf("reset failed: %w", err)
			}
			return nil
		},
	}
}
