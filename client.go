package hn

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

const (
	firebaseBaseURL      = "https://hacker-news.firebaseio.com/v0"
	algoliaBaseURL       = "https://hn.algolia.com/api/v1"
	defaultMaxConcurrent = 128
	minMaxConcurrent     = 1
	maxMaxConcurrent     = 256
	maxConcurrentEnvVar  = "HN_MAX_CONCURRENT"
)

// Client provides access to the Hacker News APIs.
type Client struct {
	http          *http.Client
	itemCache     map[int]Item
	maxConcurrent int
	mu            sync.RWMutex
	sfGroup       singleflight.Group
}

// ClientOption configures a Client.
type ClientOption func(*Client)

// WithMaxConcurrent sets the maximum number of concurrent item requests.
func WithMaxConcurrent(n int) ClientOption {
	return func(c *Client) {
		c.maxConcurrent = normalizeMaxConcurrent(n)
	}
}

// MaxConcurrent returns the configured maximum concurrency limit.
func (c *Client) MaxConcurrent() int {
	return c.maxConcurrent
}

// NewClient creates a new HN API client.
func NewClient(opts ...ClientOption) *Client {
	c := &Client{
		http:          &http.Client{Timeout: 30 * time.Second},
		itemCache:     make(map[int]Item),
		maxConcurrent: maxConcurrentFromEnv(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	return c
}

func maxConcurrentFromEnv() int {
	raw := os.Getenv(maxConcurrentEnvVar)
	if raw == "" {
		return defaultMaxConcurrent
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return defaultMaxConcurrent
	}
	return normalizeMaxConcurrent(n)
}

func normalizeMaxConcurrent(n int) int {
	if n < minMaxConcurrent {
		return minMaxConcurrent
	}
	if n > maxMaxConcurrent {
		return maxMaxConcurrent
	}
	return n
}

// Category represents a story category on HN.
type Category string

const (
	CategoryTop  Category = "top"
	CategoryNew  Category = "new"
	CategoryBest Category = "best"
	CategoryAsk  Category = "ask"
	CategoryShow Category = "show"
)

// Stories fetches story IDs for the given category.
func (c *Client) Stories(cat Category) ([]int, error) {
	reqURL := fmt.Sprintf("%s/%sstories.json", firebaseBaseURL, cat)
	t := time.Now()
	resp, err := c.http.Get(reqURL)
	if err != nil {
		slog.Debug("Stories error", "cat", cat, "elapsed", time.Since(t), "err", err)
		return nil, fmt.Errorf("fetch %s stories: %w", cat, err)
	}
	defer resp.Body.Close()

	var ids []int
	if err := json.NewDecoder(resp.Body).Decode(&ids); err != nil {
		slog.Debug("Stories decode error", "cat", cat, "elapsed", time.Since(t), "err", err)
		return nil, fmt.Errorf("decode %s stories: %w", cat, err)
	}
	slog.Debug("Stories ok", "cat", cat, "count", len(ids), "elapsed", time.Since(t))
	return ids, nil
}

// GetItem fetches a single item by ID, returning a cached copy if available.
// Concurrent requests for the same uncached ID are deduplicated via singleflight
// so only one HTTP request is made regardless of how many goroutines ask at once.
func (c *Client) GetItem(id int) (*Item, error) {
	c.mu.RLock()
	if item, ok := c.itemCache[id]; ok {
		c.mu.RUnlock()
		return &item, nil
	}
	c.mu.RUnlock()

	v, err, _ := c.sfGroup.Do(strconv.Itoa(id), func() (any, error) {
		return c.GetItemFresh(id)
	})
	if err != nil {
		return nil, err
	}
	return v.(*Item), nil
}

// GetItemFresh fetches a single item by ID and updates the item cache.
func (c *Client) GetItemFresh(id int) (*Item, error) {
	reqURL := fmt.Sprintf("%s/item/%d.json", firebaseBaseURL, id)
	t := time.Now()
	resp, err := c.http.Get(reqURL)
	elapsed := time.Since(t)
	if err != nil {
		slog.Debug("GetItemFresh error", "id", id, "elapsed", elapsed, "err", err)
		return nil, fmt.Errorf("fetch item %d: %w", id, err)
	}
	defer resp.Body.Close()

	var item Item
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		slog.Debug("GetItemFresh decode error", "id", id, "elapsed", elapsed, "err", err)
		return nil, fmt.Errorf("decode item %d: %w", id, err)
	}

	slog.Debug("GetItemFresh ok", "id", id, "elapsed", elapsed, "type", item.Type, "dead", item.Dead, "deleted", item.Deleted, "kids", len(item.Kids))
	c.mu.Lock()
	c.itemCache[id] = item
	c.mu.Unlock()

	return &item, nil
}

// GetItems fetches multiple items concurrently.
func (c *Client) GetItems(ids []int) ([]Item, error) {
	items := make([]Item, len(ids))
	var mu sync.Mutex
	var firstErr error
	sem := make(chan struct{}, c.maxConcurrent)
	var wg sync.WaitGroup

	for i, id := range ids {
		wg.Add(1)
		go func(idx, itemID int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			item, err := c.GetItem(itemID)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				return
			}
			items[idx] = *item
		}(i, id)
	}

	wg.Wait()
	if firstErr != nil {
		return items, firstErr
	}
	return items, nil
}

// GetUser fetches a user profile.
func (c *Client) GetUser(id string) (*User, error) {
	url := fmt.Sprintf("%s/user/%s.json", firebaseBaseURL, id)
	resp, err := c.http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch user %s: %w", id, err)
	}
	defer resp.Body.Close()

	var user User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("decode user %s: %w", id, err)
	}
	return &user, nil
}

// Search searches HN stories via Algolia.
func (c *Client) Search(query string, page int) (*SearchResult, error) {
	searchURL := fmt.Sprintf("%s/search?query=%s&tags=story&page=%d", algoliaBaseURL, url.QueryEscape(query), page)
	resp, err := c.http.Get(searchURL)
	if err != nil {
		return nil, fmt.Errorf("search %q: %w", query, err)
	}
	defer resp.Body.Close()

	var result SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode search %q: %w", query, err)
	}
	return &result, nil
}
