package root

import (
	"fmt"
	"log"
	"net/http"

	"cryptool/internal/api"
	"cryptool/internal/config"

	"github.com/spf13/cobra"
)

func NewServeCmd() *cobra.Command {
	var port string
	var host string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the REST API server for the frontend",
		Long: `Start an HTTP REST API server that exposes products, candles,
backtest results, and strategies for the web frontend.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.FromContext(cmd.Context())

			srv, err := api.NewServer(cfg)
			if err != nil {
				return fmt.Errorf("api server init: %w", err)
			}
			defer srv.Close()

			addr := host + ":" + port
			log.Printf("API server listening on http://%s", addr)
			return http.ListenAndServe(addr, srv.Handler())
		},
	}

	cmd.Flags().StringVarP(&port, "port", "p", "18811", "port to listen on")
	cmd.Flags().StringVar(&host, "host", "0.0.0.0", "host to bind to")
	return cmd
}
