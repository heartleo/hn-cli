// Package algolia fetches a full Hacker News comment thread in a single HTTP
// request via the Algolia HN API. It is an alternative to the Firebase
// per-item fan-out in the root hn package.
//
// Tradeoff: Algolia has roughly 30 minutes of indexing lag, so very recent
// comments on fresh stories may be missing. Callers should fall back to the
// Firebase client if the result is obviously incomplete or the request fails.
package algolia

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	hn "github.com/heartleo/hn-cli"
)

const defaultBaseURL = "https://hn.algolia.com/api/v1"
const defaultTimeout = 30 * time.Second

// Fetcher retrieves comment trees from Algolia's /items endpoint.
type Fetcher struct {
	HTTP    *http.Client
	BaseURL string
}

// NewFetcher returns a Fetcher with sensible defaults.
func NewFetcher() *Fetcher {
	return &Fetcher{
		HTTP:    &http.Client{Timeout: defaultTimeout},
		BaseURL: defaultBaseURL,
	}
}

// Thread fetches the full comment tree for storyID in one HTTP call and
// returns the top-level comments with Children, Kids, and Depth filled in
// for the entire subtree. Dead/deleted nodes (author == nil && text == nil)
// are skipped along with their descendants.
//
// ctx cancellation aborts the in-flight request.
func (f *Fetcher) Thread(ctx context.Context, storyID int) ([]*hn.Comment, error) {
	url := fmt.Sprintf("%s/items/%d", f.baseURL(), storyID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := f.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("algolia fetch %d: %w", storyID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("algolia fetch %d: %s", storyID, resp.Status)
	}

	var root item
	if err := json.NewDecoder(resp.Body).Decode(&root); err != nil {
		return nil, fmt.Errorf("algolia decode %d: %w", storyID, err)
	}

	comments := make([]*hn.Comment, 0, len(root.Children))
	for _, child := range root.Children {
		if c := convert(child, 0); c != nil {
			comments = append(comments, c)
		}
	}
	return comments, nil
}

func (f *Fetcher) client() *http.Client {
	if f.HTTP != nil {
		return f.HTTP
	}
	return http.DefaultClient
}

func (f *Fetcher) baseURL() string {
	if f.BaseURL != "" {
		return f.BaseURL
	}
	return defaultBaseURL
}

// ReorderByKids rearranges comments to match the order of ids in kids.
// Used to overlay HN's algorithmic display order (available only in the
// Firebase parent's Kids array) on top of Algolia's time-sorted children.
// Comments absent from kids are appended after, preserving their Algolia
// order. Ids in kids without a matching comment are skipped.
func ReorderByKids(comments []*hn.Comment, kids []int) []*hn.Comment {
	if len(kids) == 0 || len(comments) <= 1 {
		return comments
	}
	byID := make(map[int]*hn.Comment, len(comments))
	for _, c := range comments {
		if c != nil {
			byID[c.Item.ID] = c
		}
	}
	out := make([]*hn.Comment, 0, len(comments))
	seen := make(map[int]bool, len(comments))
	for _, id := range kids {
		if c, ok := byID[id]; ok && !seen[id] {
			out = append(out, c)
			seen[id] = true
		}
	}
	for _, c := range comments {
		if c != nil && !seen[c.Item.ID] {
			out = append(out, c)
		}
	}
	return out
}

// convert maps an Algolia item to *hn.Comment recursively. Dead nodes
// (nil author and nil text) drop themselves and their subtree — the HN web
// UI does the same.
func convert(it item, depth int) *hn.Comment {
	if it.Author == nil && it.Text == nil {
		return nil
	}
	c := &hn.Comment{
		Item: hn.Item{
			ID:   it.ID,
			Type: it.Type,
			Time: it.CreatedAtI,
		},
		Depth: depth,
	}
	if it.Author != nil {
		c.Item.By = *it.Author
	}
	if it.Text != nil {
		c.Item.Text = *it.Text
	}
	if it.ParentID != nil {
		c.Item.Parent = *it.ParentID
	}
	kids := make([]int, 0, len(it.Children))
	children := make([]*hn.Comment, 0, len(it.Children))
	for _, child := range it.Children {
		sub := convert(child, depth+1)
		if sub == nil {
			continue
		}
		kids = append(kids, sub.Item.ID)
		children = append(children, sub)
	}
	c.Item.Kids = kids
	c.Children = children
	return c
}
