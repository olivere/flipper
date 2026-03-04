package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/adrg/xdg"
)

type Config struct {
	Server  ServerConfig  `toml:"server"`
	Device  DeviceConfig  `toml:"device"`
	Screens ScreensConfig `toml:"screens"`
}

type ServerConfig struct {
	Addr      string `toml:"addr"`
	SecretKey string `toml:"secret_key"`
	SetupMode bool   `toml:"setup_mode"`
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
			Addr:      ":3000",
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

func Load(path string) (*Config, error) {
	cfg := defaults()

	if path == "" {
		path = filepath.Join(xdg.ConfigHome, "flipper", "config.toml")
	}

	if data, err := os.ReadFile(path); err == nil {
		if err := toml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parse config %s: %w", path, err)
		}
	}

	applyEnv(&cfg)
	cfg.Screens.Static.Dir = expandHome(cfg.Screens.Static.Dir)

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
		cfg.Server.SetupMode = v == "true" || v == "1"
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
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
