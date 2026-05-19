package firmware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestCompare(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.8.2", "1.8.1", 1},
		{"1.8.1", "1.8.2", -1},
		{"1.8.2", "1.8.2", 0},
		{"v1.8.2", "1.8.2", 0},
		{"v1.8.2", "v1.8.1", 1},
		{"1.8", "1.8.0", 0},
		{"1.9", "1.8.99", 1},
		{"1.8.2-rc1", "1.8.2", 0},
		{"1.8.2+build5", "1.8.2", 0},
		{"2.0.0", "1.99.99", 1},

		// Parse errors return 0 so unknown versions never win.
		{"", "1.8.2", 0},
		{"1.8.2", "", 0},
		{"not-a-version", "1.8.2", 0},
		{"1.x.0", "1.8.2", 0},
		{"1.8.2.3", "1.8.2", 0},
		{"-1.0.0", "1.0.0", 0},
	}
	for _, tt := range tests {
		if got := Compare(tt.a, tt.b); got != tt.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestStatus(t *testing.T) {
	tests := []struct {
		reported, latest, want string
	}{
		{"1.8.2", "1.8.2", StatusCurrent},
		{"1.7.4", "1.8.2", StatusOutdated},
		{"1.9.0", "1.8.2", StatusAhead},
		{"", "1.8.2", StatusUnknown},
		{"1.8.2", "", StatusUnknown},
		{"garbage", "1.8.2", StatusUnknown},
	}
	for _, tt := range tests {
		if got := Status(tt.reported, tt.latest); got != tt.want {
			t.Errorf("Status(%q, %q) = %q, want %q", tt.reported, tt.latest, got, tt.want)
		}
	}
}

func TestLatestSkipsPrereleases(t *testing.T) {
	rels := []Release{
		{Version: "1.7.4"},
		{Version: "1.8.2"},
		{Version: "1.9.0", Prerelease: true},
		{Version: "1.8.1"},
	}
	got := Latest(rels)
	if got.Version != "1.8.2" {
		t.Errorf("Latest() = %q, want 1.8.2", got.Version)
	}
}

func TestLatestEmpty(t *testing.T) {
	if got := Latest(nil); got.Version != "" {
		t.Errorf("Latest(nil) = %+v, want zero", got)
	}
	// All prereleases → zero.
	got := Latest([]Release{{Version: "1.9.0", Prerelease: true}})
	if got.Version != "" {
		t.Errorf("Latest(all prereleases) = %+v, want zero", got)
	}
}

func TestLatestIgnoresUnparseable(t *testing.T) {
	rels := []Release{
		{Version: "not-a-version"},
		{Version: "1.8.2"},
	}
	if got := Latest(rels); got.Version != "1.8.2" {
		t.Errorf("Latest() = %q, want 1.8.2", got.Version)
	}
}

func TestListReleasesParsesFixture(t *testing.T) {
	resetCache()
	t.Cleanup(resetCache)

	body, err := os.ReadFile("testdata/releases.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Accept"); got != "application/vnd.github+json" {
			t.Errorf("Accept = %q, want application/vnd.github+json", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	origURL := DefaultURL
	DefaultURL = srv.URL
	t.Cleanup(func() { DefaultURL = origURL })

	releases, err := ListReleases(context.Background())
	if err != nil {
		t.Fatalf("ListReleases: %v", err)
	}
	if len(releases) != 4 {
		t.Fatalf("got %d releases, want 4 (draft must be filtered)", len(releases))
	}

	first := releases[0]
	if first.Version != "1.8.2" {
		t.Errorf("first.Version = %q, want 1.8.2", first.Version)
	}
	if first.Tag != "v1.8.2" {
		t.Errorf("first.Tag = %q, want v1.8.2", first.Tag)
	}
	if first.Prerelease {
		t.Errorf("first.Prerelease = true, want false")
	}

	latest := Latest(releases)
	if latest.Version != "1.8.2" {
		t.Errorf("Latest = %q, want 1.8.2 (must skip rc1)", latest.Version)
	}
}

func TestListReleasesCaches(t *testing.T) {
	resetCache()
	t.Cleanup(resetCache)

	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(srv.Close)

	origURL := DefaultURL
	DefaultURL = srv.URL
	t.Cleanup(func() { DefaultURL = origURL })

	for range 3 {
		if _, err := ListReleases(context.Background()); err != nil {
			t.Fatalf("ListReleases: %v", err)
		}
	}
	if hits != 1 {
		t.Errorf("upstream hit %d times, want 1 (cache must serve subsequent calls)", hits)
	}
}

func TestListReleasesServerError(t *testing.T) {
	resetCache()
	t.Cleanup(resetCache)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)

	origURL := DefaultURL
	DefaultURL = srv.URL
	t.Cleanup(func() { DefaultURL = origURL })

	if _, err := ListReleases(context.Background()); err == nil {
		t.Fatalf("expected error on 403, got nil")
	}
}
