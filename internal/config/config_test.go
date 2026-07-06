package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPerDevicePlaylists(t *testing.T) {
	raw := `
[[playlist]]
screen = "news"
duration = "1m"

[[playlists.office]]
screen = "weather"
duration = "2m"
params.city = "Munich"

[[playlists.office]]
screen = "hackernews"
duration = "60s"

[devices."1C:DB:D4:66:5D:38"]
playlist = "office"
`
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(cfg.Playlist) != 1 {
		t.Errorf("expected 1 default playlist entry, got %d", len(cfg.Playlist))
	}
	office := cfg.Playlists["office"]
	if len(office) != 2 {
		t.Fatalf("expected 2 office entries, got %d", len(office))
	}
	if city, _ := office[0].Params["city"].(string); city != "Munich" {
		t.Errorf("expected city param Munich, got %q", city)
	}
	ovr, ok := cfg.Devices["1C:DB:D4:66:5D:38"]
	if !ok || ovr.Playlist != "office" {
		t.Errorf("expected device assignment to office, got %+v (ok=%v)", ovr, ok)
	}
}
