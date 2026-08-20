package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
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
					fmt.Fprintf(
						w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
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
				fmt.Fprintf(
					w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
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
		Short: "Inspect releases and apply firmware updates",
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
				fmt.Fprintf(
					w, "%s\t%s\t%s\t%s\t%s\n",
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

			armedByMAC := map[string]firmware.Arm{}
			if cfg.Firmware.Enabled {
				pending, perr := firmware.NewPending()
				if perr == nil {
					armedByMAC = pending.List()
				}
			}

			if statusJSON {
				// Latest and Armed are intentionally not omitempty:
				// machine consumers need to see "latest": "" when the
				// upstream fetch failed (vs. a device whose telemetry
				// just hasn't arrived yet), and "armed": "" when no
				// OTA is queued — a stable JSON shape lets callers
				// diff rows without per-field presence checks.
				type row struct {
					MAC      string `json:"mac"`
					Name     string `json:"name,omitempty"`
					Model    string `json:"model,omitempty"`
					Firmware string `json:"firmware,omitempty"`
					Latest   string `json:"latest"`
					Armed    string `json:"armed"`
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
						Armed:    armedByMAC[d.MAC].Version,
						Status:   firmware.Status(d.Telemetry.FirmwareVersion, latestVer),
					})
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
			}

			if len(devs) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No devices registered.")
				return nil
			}

			showArmed := len(armedByMAC) > 0
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			if showArmed {
				fmt.Fprintln(w, "MAC\tNAME\tMODEL\tFIRMWARE\tLATEST\tARMED\tSTATUS")
			} else {
				fmt.Fprintln(w, "MAC\tNAME\tMODEL\tFIRMWARE\tLATEST\tSTATUS")
			}
			for _, d := range devs {
				if showArmed {
					fmt.Fprintf(
						w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
						d.MAC,
						valOrDash(d.Name),
						valOrDash(d.Telemetry.Model),
						valOrDash(d.Telemetry.FirmwareVersion),
						valOrDash(latestVer),
						valOrDash(armedByMAC[d.MAC].Version),
						firmware.Status(d.Telemetry.FirmwareVersion, latestVer),
					)
					continue
				}
				fmt.Fprintf(
					w, "%s\t%s\t%s\t%s\t%s\t%s\n",
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

	firmwareCmd.AddCommand(
		list,
		status,
		newFirmwareImportCmd(configPath),
		newFirmwareUpdateCmd(configPath),
		newFirmwareCancelCmd(configPath),
		newFirmwareArmedCmd(configPath),
		newFirmwareBinariesCmd(configPath),
		newFirmwareRemoveCmd(configPath),
	)
	return firmwareCmd
}

// errFirmwareDisabled is returned by the apply-side commands when the
// operator has set firmware.enabled = false in config. The read-only
// commands (list, status) keep working.
var errFirmwareDisabled = errors.New("firmware support is disabled (set [firmware].enabled = true in config)")

func loadFirmwareConfig(configPath string) (*config.Config, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	if !cfg.Firmware.Enabled {
		return nil, errFirmwareDisabled
	}
	return cfg, nil
}

func newFirmwareImportCmd(configPath *string) *cobra.Command {
	var (
		version string
		model   string
	)
	cmd := &cobra.Command{
		Use:   "import <path-or-url>",
		Short: "Add a firmware binary to the local store",
		Long: "Import a .bin file from a local path or http(s) URL. The --version " +
			"and --model values are stored in the manifest and used later when arming " +
			"an update; the destination filename is derived as FW-<version>.<model>.bin. " +
			"Re-importing the same (version, model) with different bytes is refused.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := loadFirmwareConfig(*configPath); err != nil {
				return err
			}
			if version == "" {
				return errors.New("--version is required")
			}
			if model == "" {
				return errors.New("--model is required")
			}
			store, err := firmware.NewStore()
			if err != nil {
				return fmt.Errorf("init store: %w", err)
			}
			bin, err := store.Import(cmd.Context(), args[0], version, model)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"Imported %s\n  version: %s\n  model:   %s\n  size:    %d bytes\n  sha256:  %s\n  source:  %s\n",
				bin.Filename, bin.Version, bin.Model, bin.Size, bin.SHA256, bin.Source)
			return nil
		},
	}
	cmd.Flags().StringVar(&version, "version", "", "firmware version, e.g. 1.8.2 (required)")
	cmd.Flags().StringVar(&model, "model", "", "device model the binary targets, e.g. TRMNL_X (required)")
	return cmd
}

func newFirmwareUpdateCmd(configPath *string) *cobra.Command {
	var (
		yes   bool
		force bool
	)
	cmd := &cobra.Command{
		Use:   "update <mac> <version>",
		Short: "Arm a one-shot firmware OTA for one device",
		Long: "Arm a one-shot firmware update for one device. The binary is " +
			"resolved by (version, device-reported model); a mismatch refuses " +
			"the arm. The flash runs on the next /api/display poll and the " +
			"arm clears itself after a single dispatch. A failed flash does " +
			"NOT auto-retry — re-arm explicitly if needed.",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadFirmwareConfig(*configPath)
			if err != nil {
				return err
			}
			mac := strings.TrimSpace(args[0])
			version := strings.TrimPrefix(strings.TrimSpace(args[1]), "v")

			reg, err := device.NewRegistry(cfg.Server.SecretKey)
			if err != nil {
				return fmt.Errorf("load devices: %w", err)
			}
			dev, ok := reg.Get(mac)
			if !ok {
				return fmt.Errorf("device %s is not registered", mac)
			}
			if dev.Telemetry.Model == "" {
				return fmt.Errorf("device %s has not reported a Model yet — wait for it to poll once, then retry", dev.MAC)
			}

			store, err := firmware.NewStore()
			if err != nil {
				return fmt.Errorf("init store: %w", err)
			}
			bin, ok := store.Find(version, dev.Telemetry.Model)
			if !ok {
				return fmt.Errorf("no binary for version %s, model %s — import it first with `flipper firmware import`",
					version, dev.Telemetry.Model)
			}
			if !force && dev.Telemetry.FirmwareVersion != "" && firmware.Compare(dev.Telemetry.FirmwareVersion, version) == 0 {
				return fmt.Errorf("device %s already reports version %s — pass --force to arm anyway", dev.MAC, version)
			}

			pending, err := firmware.NewPending()
			if err != nil {
				return fmt.Errorf("init pending: %w", err)
			}

			binPath := filepath.Join(store.Dir(), bin.Filename)
			fmt.Fprintf(
				cmd.OutOrStdout(),
				"Device:    %s%s\nReported:  %s (%s)\nTarget:    %s (%s)  sha256=%s\nFile:      %s\n\n",
				dev.MAC,
				maybeQuotedName(dev.Name),
				valOrDash(dev.Telemetry.FirmwareVersion), valOrDash(dev.Telemetry.Model),
				bin.Version, bin.Model, bin.SHA256,
				binPath,
			)
			fmt.Fprintln(cmd.OutOrStdout(), "Flashing the wrong firmware can brick the device.")
			fmt.Fprintln(cmd.OutOrStdout(), "The flash is one-shot: it runs on the next /api/display poll, then auto-clears.")
			fmt.Fprintln(cmd.OutOrStdout())

			if !yes {
				if !confirmYes(cmd) {
					fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
					return nil
				}
			}

			arm := firmware.Arm{
				Filename: bin.Filename,
				Version:  bin.Version,
				Model:    bin.Model,
				ArmedAt:  time.Now().UTC(),
			}
			if err := pending.Arm(dev.MAC, arm); err != nil {
				return fmt.Errorf("arm: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Armed %s for %s. It will flash on the next /api/display poll.\n",
				bin.Filename, dev.MAC)
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the interactive confirmation prompt")
	cmd.Flags().BoolVar(&force, "force", false, "arm even when the device already reports the target version")
	return cmd
}

func confirmYes(cmd *cobra.Command) bool {
	fmt.Fprint(cmd.OutOrStdout(), "Proceed? [type 'yes' to confirm]: ")
	in := cmd.InOrStdin()
	scanner := bufio.NewScanner(in)
	if !scanner.Scan() {
		return false
	}
	return strings.TrimSpace(scanner.Text()) == "yes"
}

func newFirmwareCancelCmd(configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "cancel <mac>",
		Short: "Disarm a pending firmware update for a device",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := loadFirmwareConfig(*configPath); err != nil {
				return err
			}
			pending, err := firmware.NewPending()
			if err != nil {
				return err
			}
			mac := strings.TrimSpace(args[0])
			if !pending.Disarm(mac) {
				return fmt.Errorf("no firmware update armed for %s", mac)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Disarmed %s\n", strings.ToUpper(mac))
			return nil
		},
	}
}

func newFirmwareArmedCmd(configPath *string) *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "armed",
		Short: "List devices with a pending firmware update",
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := loadFirmwareConfig(*configPath); err != nil {
				return err
			}
			pending, err := firmware.NewPending()
			if err != nil {
				return err
			}
			armed := pending.List()

			if jsonOut {
				type row struct {
					MAC      string    `json:"mac"`
					Filename string    `json:"filename"`
					Version  string    `json:"version"`
					Model    string    `json:"model"`
					ArmedAt  time.Time `json:"armed_at"`
				}
				out := make([]row, 0, len(armed))
				for mac, a := range armed {
					out = append(out, row{MAC: mac, Filename: a.Filename, Version: a.Version, Model: a.Model, ArmedAt: a.ArmedAt})
				}
				sort.Slice(out, func(i, j int) bool { return out[i].MAC < out[j].MAC })
				return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
			}

			if len(armed) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No devices armed.")
				return nil
			}
			macs := make([]string, 0, len(armed))
			for mac := range armed {
				macs = append(macs, mac)
			}
			sort.Strings(macs)
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "MAC\tVERSION\tMODEL\tFILE\tARMED")
			for _, mac := range macs {
				a := armed[mac]
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", mac, a.Version, a.Model, a.Filename, formatAge(a.ArmedAt))
			}
			return w.Flush()
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output as JSON")
	return cmd
}

func newFirmwareBinariesCmd(configPath *string) *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "binaries",
		Short: "List firmware binaries imported into the local store",
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := loadFirmwareConfig(*configPath); err != nil {
				return err
			}
			store, err := firmware.NewStore()
			if err != nil {
				return err
			}
			bins := store.List()
			sort.Slice(bins, func(i, j int) bool {
				if bins[i].Model != bins[j].Model {
					return bins[i].Model < bins[j].Model
				}
				return firmware.Compare(bins[i].Version, bins[j].Version) < 0
			})

			if jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(bins)
			}

			if len(bins) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No binaries imported.")
				return nil
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "VERSION\tMODEL\tSIZE\tSHA256\tIMPORTED\tFILE")
			for _, b := range bins {
				fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\n",
					b.Version, b.Model, b.Size, shortSHA(b.SHA256), formatDate(b.ImportedAt), b.Filename)
			}
			return w.Flush()
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output as JSON")
	return cmd
}

func newFirmwareRemoveCmd(configPath *string) *cobra.Command {
	var model string
	cmd := &cobra.Command{
		Use:   "remove <version>",
		Short: "Delete a firmware binary from the local store",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := loadFirmwareConfig(*configPath); err != nil {
				return err
			}
			if model == "" {
				return errors.New("--model is required")
			}
			store, err := firmware.NewStore()
			if err != nil {
				return err
			}
			if err := store.Remove(args[0], model); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed firmware %s/%s\n",
				strings.TrimPrefix(strings.TrimSpace(args[0]), "v"), model)
			return nil
		},
	}
	cmd.Flags().StringVar(&model, "model", "", "device model the binary targets (required)")
	return cmd
}

func maybeQuotedName(name string) string {
	if name == "" {
		return ""
	}
	return fmt.Sprintf(" (%q)", name)
}

func shortSHA(s string) string {
	if len(s) <= 12 {
		return s
	}
	return s[:12]
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
