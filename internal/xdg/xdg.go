// Package xdg provides XDG Base Directory paths. It respects
// XDG_CONFIG_HOME and XDG_DATA_HOME environment variables but defaults
// to $HOME/.config and $HOME/.local/share per the XDG spec (not macOS
// conventions like ~/Library/Application Support).
package xdg

import (
	"os"
	"path/filepath"
)

// ConfigHome returns the XDG config directory.
// It checks XDG_CONFIG_HOME first, then falls back to $HOME/.config.
func ConfigHome() string {
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".config"
	}
	return filepath.Join(home, ".config")
}

// DataHome returns the XDG data directory.
// It checks XDG_DATA_HOME first, then falls back to $HOME/.local/share.
func DataHome() string {
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".local", "share")
	}
	return filepath.Join(home, ".local", "share")
}
