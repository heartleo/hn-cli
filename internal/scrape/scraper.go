// Package scrape fetches a Hacker News comment thread by parsing the
// public news.ycombinator.com item page HTML. Unlike the Firebase and
// Algolia clients, this source reflects HN's algorithmic display order at
// every nesting level — because the server bakes that order directly into
// the HTML — at the cost of depending on an undocumented HTML contract.
//
// The `?id=X` page is assumed to render the full, fully-expanded thread in
// one response. Pagination is not handled; callers should fall back to
// Algolia if this source fails.
package scrape

import (
	"context"
	"fmt"
	"net/http"
	"time"

	hn "github.com/heartleo/hn-cli"
)

const (
	defaultBaseURL   = "https://news.ycombinator.com"
	defaultTimeout   = 30 * time.Second
	defaultUserAgent = "hn-cli (+https://github.com/heartleo/hn-cli)"
)

// Scraper retrieves comment threads by scraping the HN item page.
type Scraper struct {
	HTTP      *http.Client
	BaseURL   string
	UserAgent string
}

// NewScraper returns a Scraper with sensible defaults.
func NewScraper() *Scraper {
	return &Scraper{
		HTTP:      &http.Client{Timeout: defaultTimeout},
		BaseURL:   defaultBaseURL,
		UserAgent: defaultUserAgent,
	}
}

// Thread fetches and parses the item page for storyID and returns the full
// comment tree in HN's display order. ctx cancellation aborts the request.
func (s *Scraper) Thread(ctx context.Context, storyID int) ([]*hn.Comment, error) {
	url := fmt.Sprintf("%s/item?id=%d", s.baseURL(), storyID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if s.UserAgent != "" {
		req.Header.Set("User-Agent", s.UserAgent)
	}
	resp, err := s.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("html fetch %d: %w", storyID, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("html fetch %d: %s", storyID, resp.Status)
	}
	return parseThread(resp.Body, storyID)
}

func (s *Scraper) client() *http.Client {
	if s.HTTP != nil {
		return s.HTTP
	}
	return http.DefaultClient
}

func (s *Scraper) baseURL() string {
	if s.BaseURL != "" {
		return s.BaseURL
	}
	return defaultBaseURL
}
