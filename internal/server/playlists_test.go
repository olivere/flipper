package server

import (
	"io"
	"log/slog"
	"testing"

	"github.com/olivere/flipper/internal/config"
	"github.com/olivere/flipper/internal/screen"

	_ "github.com/olivere/flipper/internal/screen/demo"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func demoEntries(n int) []config.PlaylistEntry {
	entries := make([]config.PlaylistEntry, n)
	for i := range entries {
		entries[i] = config.PlaylistEntry{Screen: "demo", Duration: "60s"}
	}
	return entries
}

func TestBuildPlaylistsNone(t *testing.T) {
	p, err := buildPlaylists(&config.Config{}, screen.NewRegistry(), testLogger())
	if err != nil {
		t.Fatal(err)
	}
	if p != nil {
		t.Errorf("expected nil Playlists, got %v", p)
	}
}

func TestBuildPlaylistsAssignment(t *testing.T) {
	cfg := &config.Config{
		Playlist:  demoEntries(2),
		Playlists: map[string][]config.PlaylistEntry{"office": demoEntries(3)},
		// Lowercase key in the config must match the normalized MAC.
		Devices: map[string]config.DeviceOverride{
			"aa:bb:cc:00:00:01": {Playlist: "office"},
		},
	}
	p, err := buildPlaylists(cfg, screen.NewRegistry(), testLogger())
	if err != nil {
		t.Fatal(err)
	}
	assigned := p.For("AA:BB:CC:00:00:01")
	if assigned == nil || assigned.Len() != 3 {
		t.Fatalf("assigned device should get the 3-entry office playlist, got %v", assigned)
	}
	def := p.For("AA:BB:CC:00:00:02")
	if def == nil || def.Len() != 2 {
		t.Fatalf("unassigned device should get the 2-entry default playlist, got %v", def)
	}
}

func TestBuildPlaylistsSingleNamedBecomesDefault(t *testing.T) {
	cfg := &config.Config{
		Playlists: map[string][]config.PlaylistEntry{"only": demoEntries(2)},
	}
	p, err := buildPlaylists(cfg, screen.NewRegistry(), testLogger())
	if err != nil {
		t.Fatal(err)
	}
	if got := p.For("AA:BB:CC:00:00:01"); got == nil || got.Len() != 2 {
		t.Fatalf("sole named playlist should serve unassigned devices, got %v", got)
	}
}

func TestBuildPlaylistsUnknownAssignment(t *testing.T) {
	cfg := &config.Config{
		Playlists: map[string][]config.PlaylistEntry{"office": demoEntries(1)},
		Devices: map[string]config.DeviceOverride{
			"AA:BB:CC:00:00:01": {Playlist: "nope"},
		},
	}
	if _, err := buildPlaylists(cfg, screen.NewRegistry(), testLogger()); err == nil {
		t.Fatal("expected error for assignment to unknown playlist")
	}
}
