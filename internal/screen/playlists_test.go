package screen

import (
	"testing"
	"time"
)

func TestPlaylistsFor(t *testing.T) {
	def := NewPlaylist()
	def.Add(&fakeScreen{name: "default-a"}, time.Minute)
	office := NewPlaylist()
	office.Add(&fakeScreen{name: "office-a"}, time.Minute)

	p := &Playlists{
		Default:  def,
		ByDevice: map[string]*Playlist{"AA:BB:CC:00:00:01": office},
	}

	if got := p.For("AA:BB:CC:00:00:01"); got != office {
		t.Error("assigned device should get its named playlist")
	}
	if got := p.For("AA:BB:CC:00:00:02"); got != def {
		t.Error("unassigned device should get the default playlist")
	}

	var nilP *Playlists
	if got := nilP.For("AA:BB:CC:00:00:01"); got != nil {
		t.Error("nil Playlists should return nil")
	}
}
