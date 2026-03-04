package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/olivere/flipper/internal/xdg"
)

type Config struct {
	Server  ServerConfig  `toml:"server"`
	Device  DeviceConfig  `toml:"device"`
	Screens ScreensConfig `toml:"screens"`
}

type ServerConfig struct {
	Addr      string    `toml:"addr"`
	SecretKey string    `toml:"secret_key"`
	SetupMode bool      `toml:"setup_mode"`
	TLS       TLSConfig `toml:"tls"`
}

type TLSConfig struct {
	Disabled bool   `toml:"disabled"`
	CertFile string `toml:"cert_file"`
	KeyFile  string `toml:"key_file"`
}

type DeviceConfig struct {
	Width       int    `toml:"width"`
	Height      int    `toml:"height"`
	Format      string `toml:"format"`
	RefreshRate int    `toml:"refresh_rate"`
}

type ScreensConfig struct {
	Rotate bool               `toml:"rotate"`
	Static StaticScreenConfig `toml:"static"`
}

type StaticScreenConfig struct {
	Dir string `toml:"dir"`
}

func defaults() Config {
	return Config{
		Server: ServerConfig{
			Addr:      ":3443",
			SecretKey: "change-me",
			SetupMode: true,
		},
		Device: DeviceConfig{
			Width:       800,
			Height:      480,
			Format:      "bmp",
			RefreshRate: 900,
		},
		Screens: ScreensConfig{
			Static: StaticScreenConfig{
				Dir: "~/Pictures/trmnl",
			},
		},
	}
}

// Load reads configuration from the TOML file at path (or
// ~/.config/flipper/config.toml when path is empty), then applies
// FLIPPER_* environment variable overrides on top.
func Load(path string) (*Config, error) {
	cfg := defaults()

	if path == "" {
		path = filepath.Join(xdg.ConfigHome(), "flipper", "config.toml")
	}

	if data, err := os.ReadFile(path); err == nil {
		if err := toml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parse config %s: %w", path, err)
		}
	}

	applyEnv(&cfg)
	cfg.Screens.Static.Dir = expandHome(cfg.Screens.Static.Dir)

	checkMacOSMigration(path)

	return &cfg, nil
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("FLIPPER_ADDR"); v != "" {
		cfg.Server.Addr = v
	}
	if v := os.Getenv("FLIPPER_SECRET_KEY"); v != "" {
		cfg.Server.SecretKey = v
	}
	if v := os.Getenv("FLIPPER_SETUP_MODE"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.Server.SetupMode = b
		}
	}
	if v := os.Getenv("FLIPPER_WIDTH"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Device.Width = n
		}
	}
	if v := os.Getenv("FLIPPER_HEIGHT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Device.Height = n
		}
	}
	if v := os.Getenv("FLIPPER_FORMAT"); v != "" {
		cfg.Device.Format = v
	}
	if v := os.Getenv("FLIPPER_REFRESH_RATE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Device.RefreshRate = n
		}
	}
	if v := os.Getenv("FLIPPER_STATIC_DIR"); v != "" {
		cfg.Screens.Static.Dir = v
	}
	if v := os.Getenv("FLIPPER_TLS_DISABLED"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.Server.TLS.Disabled = b
		}
	}
	if v := os.Getenv("FLIPPER_TLS_CERT_FILE"); v != "" {
		cfg.Server.TLS.CertFile = v
	}
	if v := os.Getenv("FLIPPER_TLS_KEY_FILE"); v != "" {
		cfg.Server.TLS.KeyFile = v
	}
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

// checkMacOSMigration logs a warning if the old macOS config path exists
// but the new XDG path does not, helping users migrate.
func checkMacOSMigration(activePath string) {
	if runtime.GOOS != "darwin" {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	oldPath := filepath.Join(home, "Library", "Application Support", "flipper", "config.toml")
	newPath := filepath.Join(xdg.ConfigHome(), "flipper", "config.toml")

	if oldPath == activePath || newPath == activePath {
		return
	}

	if _, err := os.Stat(oldPath); err != nil {
		return
	}
	if _, err := os.Stat(newPath); err == nil {
		return
	}

	slog.Warn("found config at old macOS path; consider moving it",
		"old", oldPath,
		"new", newPath,
	)
}
