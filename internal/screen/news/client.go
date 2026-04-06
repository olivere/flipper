package news

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

var httpClient = &http.Client{Timeout: 15 * time.Second}

// Item represents a single news article from the RSS feed.
type Item struct {
	Title   string
	Source  string
	PubDate time.Time
	Link    string
}

// rss mirrors the Google News RSS 2.0 response structure.
type rss struct {
	Channel struct {
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
}

type rssItem struct {
	Title   string `xml:"title"`
	Link    string `xml:"link"`
	PubDate string `xml:"pubDate"`
	Source  string `xml:"source"`
}

// buildURL constructs the Google News RSS URL from the config.
func buildURL(cfg Config) string {
	ceid := cfg.Country + ":" + cfg.Lang
	if cfg.Search != "" {
		return fmt.Sprintf("https://news.google.com/rss/search?q=%s&hl=%s&gl=%s&ceid=%s",
			url.QueryEscape(cfg.Search), cfg.Lang, cfg.Country, ceid)
	}
	return fmt.Sprintf("https://news.google.com/rss?hl=%s&gl=%s&ceid=%s",
		cfg.Lang, cfg.Country, ceid)
}

// fetch retrieves news items from Google News RSS.
func fetch(ctx context.Context, cfg Config) ([]Item, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, buildURL(cfg), nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch rss: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rss returned status %d", resp.StatusCode)
	}

	var feed rss
	if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return nil, fmt.Errorf("decode rss: %w", err)
	}

	limit := cfg.Limit
	if limit <= 0 || limit > len(feed.Channel.Items) {
		limit = len(feed.Channel.Items)
	}

	items := make([]Item, 0, limit)
	for _, ri := range feed.Channel.Items[:limit] {
		t, _ := time.Parse(time.RFC1123, ri.PubDate)
		items = append(items, Item{
			Title:   ri.Title,
			Source:  ri.Source,
			PubDate: t,
			Link:    ri.Link,
		})
	}
	return items, nil
}

// relativeTime formats a time as a human-readable relative duration.
func relativeTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1m ago"
		}
		return fmt.Sprintf("%dm ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1h ago"
		}
		return fmt.Sprintf("%dh ago", h)
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1d ago"
		}
		return fmt.Sprintf("%dd ago", days)
	}
}
