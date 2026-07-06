package screen

// Playlists routes devices to playlists. ByDevice keys are
// normalized device MACs; Default serves devices without an
// assignment.
type Playlists struct {
	Default  *Playlist
	ByDevice map[string]*Playlist
}

// For returns the playlist for deviceID: its assigned playlist when
// one exists, otherwise the default. Returns nil when neither is
// configured.
func (p *Playlists) For(deviceID string) *Playlist {
	if p == nil {
		return nil
	}
	if pl, ok := p.ByDevice[deviceID]; ok {
		return pl
	}
	return p.Default
}
