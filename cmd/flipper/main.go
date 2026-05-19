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
	"github.com/olivere/flipper/internal/firmware"
	"github.com/olivere/flipper/internal/server"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var configPath string

	root := &cobra.Command{
		Use:          "flipper",
		Short:        "TRMNL e-ink display server",
		SilenceUsage: true,
	}
	root.PersistentFlags().StringVar(&configPath, "config", "", "path to config file")

	root.AddCommand(
		newServeCmd(&configPath),
		newDevicesCmd(&configPath),
		newFirmwareCmd(&configPath),
		newConfigCmd(&configPath),
	)
	return root
}

func newServeCmd(configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the display server",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(*configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			return server.Run(cmd.Context(), cfg)
		},
	}
}

func newDevicesCmd(configPath *string) *cobra.Command {
	var (
		jsonOutput   bool
		checkUpdates bool
	)

	devices := &cobra.Command{
		Use:   "devices",
		Short: "List registered devices and their telemetry",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(*configPath)
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
				return json.NewEncoder(cmd.OutOrStdout()).Encode(devs)
			}

			if len(devs) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No devices registered.")
				return nil
			}

			latestVer := ""
			if checkUpdates {
				latestVer = fetchLatestVersion(cmd)
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			if checkUpdates {
				fmt.Fprintln(w, "MAC\tNAME\tFIRMWARE\tLATEST\tBATTERY\tRSSI\tMODEL\tLAST SEEN")
			} else {
				fmt.Fprintln(w, "MAC\tNAME\tFIRMWARE\tBATTERY\tRSSI\tMODEL\tLAST SEEN")
			}
			for _, d := range devs {
				if checkUpdates {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
						d.MAC,
						valOrDash(d.Name),
						valOrDash(d.Telemetry.FirmwareVersion),
						valOrDash(latestVer),
						formatBattery(d.Telemetry),
						valOrDash(d.Telemetry.WifiRSSI),
						valOrDash(d.Telemetry.Model),
						formatAge(d.LastSeen),
					)
					continue
				}
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
	devices.Flags().BoolVar(&checkUpdates, "check-updates", false, "include latest firmware version from upstream")

	rename := &cobra.Command{
		Use:   "rename <mac> <name>",
		Short: "Set a friendly name for a device",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(*configPath)
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
			fmt.Fprintf(cmd.OutOrStdout(), "Renamed %s to %q\n", args[0], args[1])
			return nil
		},
	}
	remove := &cobra.Command{
		Use:   "remove <mac>",
		Short: "Remove a device from the registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(*configPath)
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
			fmt.Fprintf(cmd.OutOrStdout(), "Removed %s\n", args[0])
			return nil
		},
	}
	devices.AddCommand(rename, remove)
	return devices
}

func newFirmwareCmd(configPath *string) *cobra.Command {
	firmwareCmd := &cobra.Command{
		Use:   "firmware",
		Short: "Inspect TRMNL firmware releases (read-only)",
	}

	var (
		listAll  bool
		listJSON bool
	)
	list := &cobra.Command{
		Use:   "list",
		Short: "Show recent firmware releases from usetrmnl/trmnl-firmware",
		RunE: func(cmd *cobra.Command, args []string) error {
			releases, err := firmware.ListReleases(cmd.Context())
			if err != nil {
				return fmt.Errorf("list releases: %w", err)
			}
			if !listAll && len(releases) > 10 {
				releases = releases[:10]
			}

			if listJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(releases)
			}

			if len(releases) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No releases found.")
				return nil
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "VERSION\tPUBLISHED\tPRERELEASE\tNAME\tURL")
			for _, r := range releases {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					r.Version,
					formatDate(r.PublishedAt),
					yesNo(r.Prerelease),
					valOrDash(r.Name),
					r.HTMLURL,
				)
			}
			return w.Flush()
		},
	}
	list.Flags().BoolVar(&listAll, "all", false, "show every release rather than the last 10")
	list.Flags().BoolVar(&listJSON, "json", false, "output as JSON")

	var statusJSON bool
	status := &cobra.Command{
		Use:   "status",
		Short: "Compare each device's reported firmware against the latest release",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(*configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			reg, err := device.NewRegistry(cfg.Server.SecretKey)
			if err != nil {
				return fmt.Errorf("load devices: %w", err)
			}
			devs := reg.List()
			sort.Slice(devs, func(i, j int) bool { return devs[i].MAC < devs[j].MAC })

			latestVer := fetchLatestVersion(cmd)

			if statusJSON {
				// Latest is intentionally not omitempty: machine
				// consumers need to see "latest": "" when the upstream
				// fetch failed, so they can distinguish that from a
				// device whose telemetry simply hasn't arrived yet
				// (the Status field still encodes "unknown" either
				// way, but a present-but-empty Latest is the explicit
				// signal that the fetch fell through).
				type row struct {
					MAC      string `json:"mac"`
					Name     string `json:"name,omitempty"`
					Model    string `json:"model,omitempty"`
					Firmware string `json:"firmware,omitempty"`
					Latest   string `json:"latest"`
					Status   string `json:"status"`
				}
				out := make([]row, 0, len(devs))
				for _, d := range devs {
					out = append(out, row{
						MAC:      d.MAC,
						Name:     d.Name,
						Model:    d.Telemetry.Model,
						Firmware: d.Telemetry.FirmwareVersion,
						Latest:   latestVer,
						Status:   firmware.Status(d.Telemetry.FirmwareVersion, latestVer),
					})
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
			}

			if len(devs) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No devices registered.")
				return nil
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "MAC\tNAME\tMODEL\tFIRMWARE\tLATEST\tSTATUS")
			for _, d := range devs {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
					d.MAC,
					valOrDash(d.Name),
					valOrDash(d.Telemetry.Model),
					valOrDash(d.Telemetry.FirmwareVersion),
					valOrDash(latestVer),
					firmware.Status(d.Telemetry.FirmwareVersion, latestVer),
				)
			}
			return w.Flush()
		},
	}
	status.Flags().BoolVar(&statusJSON, "json", false, "output as JSON")

	firmwareCmd.AddCommand(list, status)
	return firmwareCmd
}

func newConfigCmd(configPath *string) *cobra.Command {
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
			path := config.Path(*configPath)
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
	return configCmd
}

// fetchLatestVersion returns the highest non-prerelease firmware
// version, or "" if the upstream releases cannot be fetched. A warning
// is written to stderr in that case so the surrounding command can keep
// going with "—" placeholders.
func fetchLatestVersion(cmd *cobra.Command) string {
	releases, err := firmware.ListReleases(cmd.Context())
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not fetch firmware releases: %v\n", err)
		return ""
	}
	return firmware.Latest(releases).Version
}

func valOrDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func formatDate(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Format("2006-01-02")
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
