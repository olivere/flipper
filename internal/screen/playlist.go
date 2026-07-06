package screen

import (
	"sync"
	"time"
)

// PlaylistEntry pairs a screen with the duration it should be shown.
// A zero Duration means the caller should fall back to the default
// refresh rate.
type PlaylistEntry struct {
	Screen   Screen
	Duration time.Duration
}

// Playlist is an ordered sequence of screens with per-entry durations.
// It replaces the Registry's round-robin rotation when configured.
// Each device advances through the playlist independently, keyed by
// its ID, so multiple devices polling the same server each see the
// full sequence.
type Playlist struct {
	mu      sync.Mutex
	entries []PlaylistEntry
	pos     cursors
}

// NewPlaylist returns an empty playlist.
func NewPlaylist() *Playlist {
	return &Playlist{pos: cursors{}}
}

// Add appends a screen entry to the playlist.
func (p *Playlist) Add(scr Screen, d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.entries = append(p.entries, PlaylistEntry{Screen: scr, Duration: d})
}

// Next returns the entry at deviceID's current position, then advances
// that device's cursor. Returns (nil, 0) if the playlist is empty.
func (p *Playlist) Next(deviceID string) (Screen, time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.entries) == 0 {
		return nil, 0
	}
	e := p.entries[p.pos.next(deviceID, len(p.entries))]
	return e.Screen, e.Duration
}

// Len returns the number of entries in the playlist.
func (p *Playlist) Len() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.entries)
}
