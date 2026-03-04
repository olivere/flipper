package xdg

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigHome(t *testing.T) {
	t.Run("from env", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "/custom/config")
		if got := ConfigHome(); got != "/custom/config" {
			t.Errorf("ConfigHome() = %q, want /custom/config", got)
		}
	})

	t.Run("default", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "")
		home, _ := os.UserHomeDir()
		want := filepath.Join(home, ".config")
		if got := ConfigHome(); got != want {
			t.Errorf("ConfigHome() = %q, want %q", got, want)
		}
	})
}

func TestDataHome(t *testing.T) {
	t.Run("from env", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", "/custom/data")
		if got := DataHome(); got != "/custom/data" {
			t.Errorf("DataHome() = %q, want /custom/data", got)
		}
	})

	t.Run("default", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", "")
		home, _ := os.UserHomeDir()
		want := filepath.Join(home, ".local", "share")
		if got := DataHome(); got != want {
			t.Errorf("DataHome() = %q, want %q", got, want)
		}
	})
}
