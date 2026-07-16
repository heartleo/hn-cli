// Package digest builds a Hacker News front-page digest: it fetches the page,
// asks an LLM to summarise it, and renders the result as a static HTML page.
package digest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// frontPageURL returns the whole HN front page in a single request. Algolia's
// front_page tag carries points and comment counts inline, so unlike Firebase's
// topstories.json (1 + 30 requests) no per-item fan-out is needed.
const frontPageURL = "https://hn.algolia.com/api/v1/search" +
	"?tags=front_page&hitsPerPage=30" +
	"&attributesToRetrieve=title,points,num_comments,url,objectID,created_at"

// Story is one front-page story.
//
// URL is empty for self-posts (Ask HN / Show HN); callers must fall back to the
// discussion page rather than emit a bare title.
type Story struct {
	Title       string    `json:"title"`
	Points      int       `json:"points"`
	NumComments int       `json:"num_comments"`
	URL         string    `json:"url"`
	ObjectID    string    `json:"objectID"`
	CreatedAt   time.Time `json:"created_at"`
}

// DiscussionURL is the HN thread for the story.
func (s Story) DiscussionURL() string {
	return "https://news.ycombinator.com/item?id=" + s.ObjectID
}

// ArticleURL is the linked article, falling back to the discussion page for
// self-posts so that every story is clickable.
func (s Story) ArticleURL() string {
	if s.URL == "" {
		return s.DiscussionURL()
	}
	return s.URL
}

// Domain is the source host, as HN itself shows beside every title.
//
// It is the signal a reader uses to decide whether to click at all — an
// announcement on the vendor's own domain reads differently from a blog
// relaying it. Empty for self-posts, which have no source but HN.
//
// This is computed here rather than asked of the model: URL parsing has one
// right answer, and a model inventing a plausible-looking host would be
// indistinguishable from a correct one.
func (s Story) Domain() string {
	if s.URL == "" {
		return ""
	}
	u, err := url.Parse(s.URL)
	if err != nil || u.Host == "" {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(u.Host), "www.")
}

// FetchFrontPage returns the front page in HN's display order.
//
// The result is the front page as it looks right now, not "stories posted
// today" — long-running stories stay up for days. That is the intended
// meaning: the question is what is on HN, not what was posted today.
func FetchFrontPage(ctx context.Context, client *http.Client) ([]Story, error) {
	return fetchFrontPage(ctx, client, frontPageURL)
}

// fetchFrontPage takes the URL so tests can point it at a server instead of
// reaching the real Algolia.
func fetchFrontPage(ctx context.Context, client *http.Client, url string) ([]Story, error) {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch front page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch front page: %s", resp.Status)
	}

	var out struct {
		Hits []Story `json:"hits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode front page: %w", err)
	}
	if len(out.Hits) == 0 {
		return nil, fmt.Errorf("front page returned no stories")
	}
	return out.Hits, nil
}

// Fingerprint identifies which stories are on the front page, ignoring their
// order and their scores.
//
// Scores are deliberately excluded: they tick constantly, so including them
// would make every hourly run look like a change and defeat the skip.
func Fingerprint(stories []Story) string {
	ids := make([]string, 0, len(stories))
	for _, s := range stories {
		ids = append(ids, s.ObjectID)
	}
	sort.Strings(ids)
	sum := sha256.Sum256([]byte(strings.Join(ids, ",")))
	return hex.EncodeToString(sum[:])
}

// Stats summarises the front page for the page header.
type Stats struct {
	Count         int
	TopPoints     int
	TotalComments int
	Newest        time.Time
}

// Summarize computes the header stats.
func Summarize(stories []Story) Stats {
	st := Stats{Count: len(stories)}
	for _, s := range stories {
		st.TotalComments += s.NumComments
		if s.Points > st.TopPoints {
			st.TopPoints = s.Points
		}
		if s.CreatedAt.After(st.Newest) {
			st.Newest = s.CreatedAt
		}
	}
	return st
}
