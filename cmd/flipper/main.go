package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/olivere/flipper/internal/config"
	"github.com/olivere/flipper/internal/server"
)

func main() {
	var configPath string

	root := &cobra.Command{
		Use:   "flipper",
		Short: "TRMNL e-ink display server",
	}

	root.PersistentFlags().StringVar(&configPath, "config", "", "path to config file")

	serve := &cobra.Command{
		Use:   "serve",
		Short: "Start the display server",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			return server.Run(cmd.Context(), cfg)
		},
	}

	root.AddCommand(serve)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
