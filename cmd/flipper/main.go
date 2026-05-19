package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/olivere/flipper/internal/config"
	"github.com/olivere/flipper/internal/device"
	"github.com/olivere/flipper/internal/server"
)

func main() {
	var configPath string

	root := &cobra.Command{
		Use:          "flipper",
		Short:        "TRMNL e-ink display server",
		SilenceUsage: true,
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

	var jsonOutput bool

	devices := &cobra.Command{
		Use:   "devices",
		Short: "List registered devices and their telemetry",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			reg, err := device.NewRegistry(cfg.Server.SecretKey)
			if err != nil {
				return fmt.Errorf("load devices: %w", err)
			}
			devs := reg.List()
			sort.Slice(devs, func(i, j int) bool {
				return devs[i].MAC < devs[j].MAC
			})

			if jsonOutput {
				return json.NewEncoder(os.Stdout).Encode(devs)
			}

			if len(devs) == 0 {
				fmt.Println("No devices registered.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "MAC\tNAME\tFIRMWARE\tBATTERY\tRSSI\tMODEL\tLAST SEEN")
			for _, d := range devs {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					d.MAC,
					valOrDash(d.Name),
					valOrDash(d.Telemetry.FirmwareVersion),
					formatBattery(d.Telemetry),
					valOrDash(d.Telemetry.WifiRSSI),
					valOrDash(d.Telemetry.Model),
					formatAge(d.LastSeen),
				)
			}
			return w.Flush()
		},
	}
	devices.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")

	rename := &cobra.Command{
		Use:   "rename <mac> <name>",
		Short: "Set a friendly name for a device",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			reg, err := device.NewRegistry(cfg.Server.SecretKey)
			if err != nil {
				return fmt.Errorf("load devices: %w", err)
			}
			if !reg.SetName(args[0], args[1]) {
				return fmt.Errorf("device %s not found", args[0])
			}
			fmt.Printf("Renamed %s to %q\n", args[0], args[1])
			return nil
		},
	}
	remove := &cobra.Command{
		Use:   "remove <mac>",
		Short: "Remove a device from the registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			reg, err := device.NewRegistry(cfg.Server.SecretKey)
			if err != nil {
				return fmt.Errorf("load devices: %w", err)
			}
			if !reg.Remove(args[0]) {
				return fmt.Errorf("device %s not found", args[0])
			}
			fmt.Printf("Removed %s\n", args[0])
			return nil
		},
	}

	devices.AddCommand(rename, remove)

	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
	}
	configEdit := &cobra.Command{
		Use:   "edit",
		Short: "Open config file in $EDITOR",
		RunE: func(cmd *cobra.Command, args []string) error {
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vi"
			}
			path := config.Path(configPath)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return fmt.Errorf("create config directory: %w", err)
			}
			c := exec.Command(editor, path)
			c.Stdin = os.Stdin
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			return c.Run()
		},
	}
	configCmd.AddCommand(configEdit)

	root.AddCommand(serve, devices, configCmd)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func valOrDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func formatBattery(t device.Telemetry) string {
	if t.BatteryVoltage == "" {
		return "—"
	}
	pct := t.BatteryPercent()
	if pct < 0 {
		return t.BatteryVoltage
	}
	return fmt.Sprintf("%sV (%d%%)", t.BatteryVoltage, pct)
}

func formatAge(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
