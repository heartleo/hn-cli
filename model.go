package hn

import (
	"fmt"
	"math"
	"time"
)

// Item represents a Hacker News item (story, comment, job, poll, etc.).
type Item struct {
	ID          int    `json:"id"`
	Type        string `json:"type"`
	By          string `json:"by"`
	Time        int64  `json:"time"`
	Text        string `json:"text"`
	URL         string `json:"url"`
	Title       string `json:"title"`
	Score       int    `json:"score"`
	Descendants int    `json:"descendants"`
	Kids        []int  `json:"kids"`
	Parent      int    `json:"parent"`
	Dead        bool   `json:"dead"`
	Deleted     bool   `json:"deleted"`
}

// RelativeTime returns a human-friendly relative time string.
func (it *Item) RelativeTime() string {
	return relativeTime(time.Unix(it.Time, 0))
}

// Story wraps an Item for use in bubbles/list.
type Story struct {
	Item
	Rank   int
	Domain string
}

func (s Story) FilterValue() string { return s.Item.Title }

// Comment is a tree node holding an Item and its children.
type Comment struct {
	Item     Item
	Children []*Comment
	Depth    int
}

// User represents a Hacker News user profile.
type User struct {
	ID        string `json:"id"`
	Created   int64  `json:"created"`
	Karma     int    `json:"karma"`
	About     string `json:"about"`
	Submitted []int  `json:"submitted"`
}

// SearchHit represents a single Algolia search result.
type SearchHit struct {
	ObjectID    string `json:"objectID"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Author      string `json:"author"`
	Points      int    `json:"points"`
	NumComments int    `json:"num_comments"`
	CreatedAt   string `json:"created_at"`
	StoryID     int    `json:"story_id"`
}

// SearchResult represents Algolia search response.
type SearchResult struct {
	Hits        []SearchHit `json:"hits"`
	Page        int         `json:"page"`
	NbPages     int         `json:"nbPages"`
	HitsPerPage int         `json:"hitsPerPage"`
	NbHits      int         `json:"nbHits"`
}

func relativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(math.Round(d.Minutes()))
		return fmt.Sprintf("%dm", m)
	case d < 24*time.Hour:
		h := int(math.Round(d.Hours()))
		return fmt.Sprintf("%dh", h)
	case d < 30*24*time.Hour:
		days := int(math.Round(d.Hours() / 24))
		return fmt.Sprintf("%dd", days)
	case d < 365*24*time.Hour:
		months := int(math.Round(d.Hours() / 24 / 30))
		return fmt.Sprintf("%dmo", months)
	default:
		years := int(math.Round(d.Hours() / 24 / 365))
		return fmt.Sprintf("%dy", years)
	}
}
