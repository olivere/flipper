package news

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/olivere/flipper/internal/display"
	"github.com/olivere/flipper/internal/screen"
)

func TestBuildURL(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want string
	}{
		{
			name: "top headlines",
			cfg:  Config{Lang: "en", Country: "US"},
			want: "https://news.google.com/rss?hl=en&gl=US&ceid=US:en",
		},
		{
			name: "search query",
			cfg:  Config{Search: "AI regulation", Lang: "en", Country: "US"},
			want: "https://news.google.com/rss/search?q=AI+regulation&hl=en&gl=US&ceid=US:en",
		},
		{
			name: "german news",
			cfg:  Config{Lang: "de", Country: "DE"},
			want: "https://news.google.com/rss?hl=de&gl=DE&ceid=DE:de",
		},
		{
			name: "german search",
			cfg:  Config{Search: "Technologie", Lang: "de", Country: "DE"},
			want: "https://news.google.com/rss/search?q=Technologie&hl=de&gl=DE&ceid=DE:de",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildURL(tt.cfg)
			if got != tt.want {
				t.Errorf("buildURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

const sampleRSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Google News</title>
    <item>
      <title>First headline</title>
      <link>https://news.google.com/articles/1</link>
      <pubDate>Mon, 06 Apr 2026 10:00:00 GMT</pubDate>
      <source>Reuters</source>
    </item>
    <item>
      <title>Second headline</title>
      <link>https://news.google.com/articles/2</link>
      <pubDate>Mon, 06 Apr 2026 09:00:00 GMT</pubDate>
      <source>BBC</source>
    </item>
    <item>
      <title>Third headline</title>
      <link>https://news.google.com/articles/3</link>
      <pubDate>Mon, 06 Apr 2026 08:00:00 GMT</pubDate>
      <source>CNN</source>
    </item>
  </channel>
</rss>`

func TestParseRSS(t *testing.T) {
	var feed rss
	if err := xml.NewDecoder(strings.NewReader(sampleRSS)).Decode(&feed); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if got := len(feed.Channel.Items); got != 3 {
		t.Fatalf("items = %d, want 3", got)
	}

	item := feed.Channel.Items[0]
	if item.Title != "First headline" {
		t.Errorf("title = %q, want %q", item.Title, "First headline")
	}
	if item.Source != "Reuters" {
		t.Errorf("source = %q, want %q", item.Source, "Reuters")
	}
}

func TestFetchWithTestServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(sampleRSS))
	}))
	defer srv.Close()

	// Override the URL by testing the parse logic directly since buildURL
	// hardcodes the Google domain. We test the full flow via the test server.
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()

	var feed rss
	if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if got := len(feed.Channel.Items); got != 3 {
		t.Fatalf("items = %d, want 3", got)
	}
}

func TestRelativeTime(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name string
		t    time.Time
		want string
	}{
		{"zero", time.Time{}, ""},
		{"just now", now.Add(-30 * time.Second), "just now"},
		{"minutes", now.Add(-5 * time.Minute), "5m ago"},
		{"one minute", now.Add(-90 * time.Second), "1m ago"},
		{"hours", now.Add(-3 * time.Hour), "3h ago"},
		{"one hour", now.Add(-90 * time.Minute), "1h ago"},
		{"days", now.Add(-48 * time.Hour), "2d ago"},
		{"one day", now.Add(-36 * time.Hour), "1d ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := relativeTime(tt.t)
			if got != tt.want {
				t.Errorf("relativeTime() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenderProducesImage(t *testing.T) {
	s := New(Config{})
	// Pre-fill cache to avoid hitting the network.
	s.cache = []Item{
		{Title: "Test headline", Source: "Test", PubDate: time.Now()},
	}
	s.expiry = time.Now().Add(time.Hour)

	img, err := s.Render(context.Background(), screen.RenderOpts{
		Width:     800,
		Height:    480,
		ColorMode: display.ColorBW,
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if img == nil {
		t.Fatal("render returned nil image")
	}

	bounds := img.Bounds()
	if bounds.Dx() != 800 || bounds.Dy() != 480 {
		t.Errorf("image size = %dx%d, want 800x480", bounds.Dx(), bounds.Dy())
	}
}

func TestRenderErrorScreen(t *testing.T) {
	s := New(Config{})
	// No cache, no network — will produce an error screen.
	// We test renderError directly since getData would try the network.
	opts := screen.RenderOpts{
		Width:     800,
		Height:    480,
		ColorMode: display.ColorBW,
	}

	img := s.renderError(opts, fmt.Errorf("test error"))
	if img == nil {
		t.Fatal("renderError returned nil image")
	}
}
