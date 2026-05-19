package firmware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Release describes a single firmware release from the upstream
// usetrmnl/trmnl-firmware repository.
type Release struct {
	Version     string    `json:"version"`               // "1.8.2" (no "v" prefix)
	Tag         string    `json:"tag"`                   // tag as published, e.g. "v1.8.2"
	Name        string    `json:"name,omitempty"`        // release title
	HTMLURL     string    `json:"html_url,omitempty"`    // link to the GitHub release page
	Body        string    `json:"body,omitempty"`        // changelog markdown
	PublishedAt time.Time `json:"published_at,omitzero"` // when the release was published
	Prerelease  bool      `json:"prerelease,omitempty"`  // true for pre-releases
}

// Status values returned by Status.
const (
	StatusCurrent  = "current"
	StatusOutdated = "outdated"
	StatusAhead    = "ahead"
	StatusUnknown  = "unknown"
)

// DefaultURL is the GitHub Releases endpoint used by ListReleases.
// Exposed as a variable so tests can point ListReleases at a stub.
var DefaultURL = "https://api.github.com/repos/usetrmnl/trmnl-firmware/releases?per_page=30"

const cacheTTL = 15 * time.Minute

var httpClient = &http.Client{Timeout: 15 * time.Second}

var (
	cacheMu     sync.Mutex
	cachedAt    time.Time
	cachedURL   string
	cachedRels  []Release
	cachedError error
)

// resetCache clears the in-process release cache. Test helper.
func resetCache() {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	cachedAt = time.Time{}
	cachedURL = ""
	cachedRels = nil
	cachedError = nil
}

// ListReleases fetches recent firmware releases from
// github.com/usetrmnl/trmnl-firmware. Results are cached in-process for
// 15 minutes to stay well under the 60 req/hr unauthenticated rate
// limit. Draft releases are filtered out; prereleases are kept (callers
// can decide what to do with them).
func ListReleases(ctx context.Context) ([]Release, error) {
	cacheMu.Lock()
	if cachedURL == DefaultURL && time.Since(cachedAt) < cacheTTL && cachedRels != nil {
		out := cachedRels
		cacheMu.Unlock()
		return out, nil
	}
	cacheMu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, DefaultURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github returned status %d", resp.StatusCode)
	}

	var raw []struct {
		TagName     string    `json:"tag_name"`
		Name        string    `json:"name"`
		HTMLURL     string    `json:"html_url"`
		Body        string    `json:"body"`
		PublishedAt time.Time `json:"published_at"`
		Prerelease  bool      `json:"prerelease"`
		Draft       bool      `json:"draft"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode releases: %w", err)
	}

	releases := make([]Release, 0, len(raw))
	for _, r := range raw {
		if r.Draft {
			continue
		}
		releases = append(releases, Release{
			Version:     strings.TrimPrefix(r.TagName, "v"),
			Tag:         r.TagName,
			Name:        r.Name,
			HTMLURL:     r.HTMLURL,
			Body:        r.Body,
			PublishedAt: r.PublishedAt,
			Prerelease:  r.Prerelease,
		})
	}

	cacheMu.Lock()
	cachedAt = time.Now()
	cachedURL = DefaultURL
	cachedRels = releases
	cacheMu.Unlock()

	return releases, nil
}

// Latest returns the highest non-prerelease semver release, or a zero
// Release if no parseable version is present.
func Latest(rels []Release) Release {
	var best Release
	for _, r := range rels {
		if r.Prerelease {
			continue
		}
		if _, ok := parseSemver(r.Version); !ok {
			continue
		}
		if best.Version == "" || Compare(r.Version, best.Version) > 0 {
			best = r
		}
	}
	return best
}

// Compare returns -1, 0, or +1 comparing the semver strings a and b.
// A leading "v", missing patch ("1.8"), and trailing prerelease/build
// metadata ("1.8.2-rc1", "1.8.2+meta") are all accepted. If either
// input fails to parse, Compare returns 0 — so an unknown reported
// version is never reported as newer than a real release.
func Compare(a, b string) int {
	pa, ok := parseSemver(a)
	if !ok {
		return 0
	}
	pb, ok := parseSemver(b)
	if !ok {
		return 0
	}
	for i := range 3 {
		if pa[i] < pb[i] {
			return -1
		}
		if pa[i] > pb[i] {
			return 1
		}
	}
	return 0
}

// Status compares a device's reported firmware version against the
// latest available version and returns one of the Status* constants.
// Returns StatusUnknown if either side is missing or unparseable.
func Status(reported, latest string) string {
	if reported == "" || latest == "" {
		return StatusUnknown
	}
	if _, ok := parseSemver(reported); !ok {
		return StatusUnknown
	}
	if _, ok := parseSemver(latest); !ok {
		return StatusUnknown
	}
	switch Compare(reported, latest) {
	case 0:
		return StatusCurrent
	case -1:
		return StatusOutdated
	case 1:
		return StatusAhead
	}
	return StatusUnknown
}

func parseSemver(s string) ([3]int, bool) {
	var out [3]int
	s = strings.TrimSpace(s)
	if s == "" {
		return out, false
	}
	s = strings.TrimPrefix(s, "v")
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return [3]int{}, false
		}
		out[i] = n
	}
	return out, true
}
