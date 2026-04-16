package hn

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/sourcegraph/conc/pool"
	"golang.org/x/sync/singleflight"
)

const (
	firebaseBaseURL         = "https://hacker-news.firebaseio.com/v0"
	algoliaBaseURL          = "https://hn.algolia.com/api/v1"
	defaultMaxConcurrent    = 128
	minMaxConcurrent        = 1
	maxMaxConcurrent        = 256
	maxConcurrentEnvVar     = "HN_MAX_CONCURRENT"
	defaultMaxPhase2Retries = 3
)

// subtreeEntry caches a parent's children across two phases:
// Phase 1 fetches the direct children; Phase 2 fetches one more level
// (grandchildren) and merges them onto each Phase 1 child. done is closed
// when Phase 2 reaches a terminal state (success or retries exhausted).
type subtreeEntry struct {
	mu       sync.Mutex
	children []*Comment
	phase    int // 0 = not started, 1 = phase1 done, 2 = phase2 done
	attempts int
	done     chan struct{}
}

// Client provides access to the Hacker News APIs.
type Client struct {
	http             *http.Client
	itemCache        map[int]Item
	maxConcurrent    int
	maxPhase2Retries int
	mu               sync.RWMutex
	sfGroup          singleflight.Group

	// Long-lived context for background subtree fetches. Independent of any
	// UI detail context so fetches survive user navigation.
	ctx    context.Context
	cancel context.CancelFunc

	subtreeMu    sync.Mutex
	subtreeCache map[int]*subtreeEntry // keyed by parent item ID
}

// ClientOption configures a Client.
type ClientOption func(*Client)

// WithMaxConcurrent sets the maximum number of concurrent item requests.
func WithMaxConcurrent(n int) ClientOption {
	return func(c *Client) {
		c.maxConcurrent = normalizeMaxConcurrent(n)
	}
}

// WithMaxPhase2Retries sets the maximum number of Phase 2 retries before
// the subtree fetcher gives up (keeping Phase 1 children intact).
func WithMaxPhase2Retries(n int) ClientOption {
	return func(c *Client) {
		if n < 0 {
			n = 0
		}
		c.maxPhase2Retries = n
	}
}

// MaxConcurrent returns the configured maximum concurrency limit.
func (c *Client) MaxConcurrent() int {
	return c.maxConcurrent
}

// NewClient creates a new HN API client.
func NewClient(opts ...ClientOption) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	c := &Client{
		http:             &http.Client{Timeout: 30 * time.Second},
		itemCache:        make(map[int]Item),
		maxConcurrent:    maxConcurrentFromEnv(),
		maxPhase2Retries: defaultMaxPhase2Retries,
		ctx:              ctx,
		cancel:           cancel,
		subtreeCache:     make(map[int]*subtreeEntry),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	return c
}

// Close cancels the client's background context so in-flight subtree
// fetches stop. Safe to call multiple times.
func (c *Client) Close() {
	if c.cancel != nil {
		c.cancel()
	}
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

// GetTopComments fetches first-level comments only, preserving Kids for lazy subtree loading.
func (c *Client) GetTopComments(kidIDs []int) ([]*Comment, error) {
	if len(kidIDs) == 0 {
		return nil, nil
	}

	items, err := c.GetItems(kidIDs)
	if err != nil {
		return nil, err
	}

	comments := make([]*Comment, 0, len(items))
	for _, item := range items {
		if item.ID == 0 || item.Dead || item.Deleted {
			continue
		}
		comments = append(comments, &Comment{
			Item:  item,
			Depth: 0,
		})
	}
	return comments, nil
}

// GetDirectChildren fetches one level of children for the given kid IDs.
// Each returned Comment has Depth set to depth and Children == nil.
// Respects ctx cancellation: returns context.Canceled when ctx is done.
// Uses conc pool bounded to c.maxConcurrent goroutines to avoid leaks.
func (c *Client) GetDirectChildren(ctx context.Context, kidIDs []int, depth int) ([]*Comment, error) {
	if len(kidIDs) == 0 {
		return nil, nil
	}

	p := pool.NewWithResults[*Comment]().
		WithContext(ctx).
		WithFirstError().
		WithMaxGoroutines(c.maxConcurrent)

	for _, id := range kidIDs {
		id := id
		p.Go(func(ctx context.Context) (*Comment, error) {
			reqURL := fmt.Sprintf("%s/item/%d.json", firebaseBaseURL, id)
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
			if err != nil {
				return nil, err
			}
			t := time.Now()
			resp, err := c.http.Do(req)
			if err != nil {
				slog.Debug("GetDirectChildren item error", "id", id, "depth", depth, "elapsed", time.Since(t), "err", err)
				return nil, err
			}
			defer resp.Body.Close()

			var item Item
			if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
				return nil, err
			}
			slog.Debug("GetDirectChildren item ok", "id", id, "depth", depth, "elapsed", time.Since(t), "kids", len(item.Kids))
			if item.ID == 0 || item.Dead || item.Deleted {
				return nil, nil
			}
			c.mu.Lock()
			c.itemCache[id] = item
			c.mu.Unlock()
			return &Comment{Item: item, Depth: depth}, nil
		})
	}

	results, err := p.Wait()
	if err != nil {
		return nil, err
	}

	comments := make([]*Comment, 0, len(results))
	for _, c := range results {
		if c != nil {
			comments = append(comments, c)
		}
	}
	slog.Debug("GetDirectChildren done", "depth", depth, "requested", len(kidIDs), "got", len(comments))
	return comments, nil
}

// EnsureSubtree idempotently kicks off a two-phase fetch for parentID:
//
//	Phase 1: fetch direct children at startDepth.
//	Phase 2: fetch grandchildren at startDepth+1 for every Phase 1 child,
//	         flattening into a single GetDirectChildren call and bucketing
//	         results back onto each level-1 comment.
//
// Returns:
//
//	snapshot  current cached children (nil / Phase 1 / Phase 2).
//	done      channel closed when Phase 2 reaches a terminal state.
//	complete  true when snapshot is already Phase 2 terminal.
//
// Concurrent callers for the same parentID share one fetch. Fetches run
// under the client's own context and survive UI navigation. If a prior
// fetch terminated without reaching Phase 2, the next call restarts the
// failed phase (implicit retry after the attempt budget is exhausted).
func (c *Client) EnsureSubtree(parentID int, kidIDs []int, startDepth int) ([]*Comment, <-chan struct{}, bool) {
	c.subtreeMu.Lock()
	entry, ok := c.subtreeCache[parentID]
	if !ok {
		entry = &subtreeEntry{done: make(chan struct{})}
		c.subtreeCache[parentID] = entry
	}
	c.subtreeMu.Unlock()

	entry.mu.Lock()
	// Already fully loaded: return immediately.
	if entry.phase == 2 {
		children := entry.children
		done := entry.done
		entry.mu.Unlock()
		return children, done, true
	}
	// Terminal but not Phase 2 (Phase 1 failed, or Phase 2 exhausted).
	// Reset and restart the missing phase.
	if isClosed(entry.done) && entry.phase < 2 {
		entry.done = make(chan struct{})
		entry.attempts = 0
		snapshot := entry.children
		phase := entry.phase
		done := entry.done
		entry.mu.Unlock()
		go c.runSubtree(parentID, kidIDs, startDepth, phase)
		return snapshot, done, false
	}
	// First ever call (phase 0 and done open) — launch background fetch.
	if entry.phase == 0 && entry.attempts == 0 {
		entry.attempts = 1 // mark as started to prevent double-launch
		snapshot := entry.children
		done := entry.done
		entry.mu.Unlock()
		go c.runSubtree(parentID, kidIDs, startDepth, 0)
		return snapshot, done, false
	}
	// Already running; just subscribe.
	snapshot := entry.children
	done := entry.done
	entry.mu.Unlock()
	return snapshot, done, false
}

// SubtreeSnapshot returns the current cached state for parentID.
// phase is 0 when no entry exists.
func (c *Client) SubtreeSnapshot(parentID int) ([]*Comment, int) {
	c.subtreeMu.Lock()
	entry, ok := c.subtreeCache[parentID]
	c.subtreeMu.Unlock()
	if !ok {
		return nil, 0
	}
	entry.mu.Lock()
	defer entry.mu.Unlock()
	return entry.children, entry.phase
}

// InvalidateAllSubtrees clears every cached subtree entry. In-flight
// goroutines finish writing to their own (now-orphaned) entry; future
// EnsureSubtree calls allocate fresh entries.
func (c *Client) InvalidateAllSubtrees() {
	c.subtreeMu.Lock()
	c.subtreeCache = make(map[int]*subtreeEntry)
	c.subtreeMu.Unlock()
}

// runSubtree runs Phase 1 (if needed) and Phase 2 (with retries) for an
// entry. It always closes entry.done on return.
func (c *Client) runSubtree(parentID int, kidIDs []int, startDepth, startPhase int) {
	c.subtreeMu.Lock()
	entry := c.subtreeCache[parentID]
	c.subtreeMu.Unlock()
	if entry == nil {
		return
	}

	defer func() {
		entry.mu.Lock()
		done := entry.done
		entry.mu.Unlock()
		select {
		case <-done:
		default:
			close(done)
		}
	}()

	// Phase 1
	if startPhase < 1 {
		children, err := c.GetDirectChildren(c.ctx, kidIDs, startDepth)
		if err != nil {
			slog.Debug("EnsureSubtree phase1 error", "parent", parentID, "err", err)
			entry.mu.Lock()
			entry.attempts = 0 // allow implicit restart
			entry.mu.Unlock()
			return
		}
		entry.mu.Lock()
		entry.children = children
		entry.phase = 1
		entry.mu.Unlock()
	}

	// Phase 2: flatten grandchild IDs and bucket results back.
	entry.mu.Lock()
	level1 := entry.children
	entry.mu.Unlock()

	for attempt := 1; attempt <= c.maxPhase2Retries; attempt++ {
		if c.ctx.Err() != nil {
			return
		}
		var flatIDs []int
		ownerOf := make(map[int]int, 0)
		for _, ch := range level1 {
			if ch == nil {
				continue
			}
			for _, kid := range ch.Item.Kids {
				flatIDs = append(flatIDs, kid)
				ownerOf[kid] = ch.Item.ID
			}
		}
		grandchildren, err := c.GetDirectChildren(c.ctx, flatIDs, startDepth+1)
		if err != nil {
			slog.Debug("EnsureSubtree phase2 error", "parent", parentID, "attempt", attempt, "err", err)
			entry.mu.Lock()
			entry.attempts = attempt
			entry.mu.Unlock()
			if c.ctx.Err() != nil {
				return
			}
			continue
		}
		buckets := make(map[int][]*Comment, len(level1))
		for _, gc := range grandchildren {
			if gc == nil {
				continue
			}
			owner, ok := ownerOf[gc.Item.ID]
			if !ok {
				continue
			}
			buckets[owner] = append(buckets[owner], gc)
		}
		for _, ch := range level1 {
			if ch == nil {
				continue
			}
			ch.Children = orderByKids(ch.Item.Kids, buckets[ch.Item.ID])
		}
		entry.mu.Lock()
		entry.phase = 2
		entry.attempts = attempt
		entry.mu.Unlock()
		return
	}
	slog.Debug("EnsureSubtree phase2 exhausted", "parent", parentID, "retries", c.maxPhase2Retries)
}

// orderByKids reorders items to match the original Kids order, dropping
// any kid IDs that failed to load.
func orderByKids(kidIDs []int, items []*Comment) []*Comment {
	if len(items) == 0 {
		return nil
	}
	byID := make(map[int]*Comment, len(items))
	for _, it := range items {
		if it != nil {
			byID[it.Item.ID] = it
		}
	}
	ordered := make([]*Comment, 0, len(items))
	for _, id := range kidIDs {
		if it, ok := byID[id]; ok {
			ordered = append(ordered, it)
		}
	}
	return ordered
}

// isClosed reports whether a done channel has been closed.
func isClosed(ch chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
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
