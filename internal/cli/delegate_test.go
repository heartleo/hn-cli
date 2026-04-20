package cli

import (
	"bytes"
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	hn "github.com/heartleo/hn-cli"
)

func TestStoryDelegateRendersOriginalAndTranslation(t *testing.T) {
	story := hn.Story{Item: hn.Item{ID: 1, Title: "Original title"}, Rank: 1}
	delegate := storyDelegate{
		width:           80,
		translations:    map[int]string{1: "Translated title"},
		showTranslation: map[int]bool{1: true},
	}
	items := []list.Item{story}
	model := list.New(items, delegate, 80, 10)

	var out bytes.Buffer
	delegate.Render(&out, model, 0, story)

	got := out.String()
	if !strings.Contains(got, "Original title") {
		t.Fatalf("expected original title in render output, got %q", got)
	}
	if !strings.Contains(got, "Translated title") {
		t.Fatalf("expected translated title in render output, got %q", got)
	}
}

func TestStoryDelegateRendersPrecomputedDomain(t *testing.T) {
	story := hn.Story{
		Item:   hn.Item{ID: 1, Title: "Story", URL: "https://www.example.com/post"},
		Rank:   1,
		Domain: "example.com",
	}
	delegate := storyDelegate{width: 80}
	items := []list.Item{story}
	model := list.New(items, delegate, 80, 10)

	var out bytes.Buffer
	delegate.Render(&out, model, 0, story)

	got := out.String()
	if !strings.Contains(got, "(example.com)") {
		t.Fatalf("expected precomputed domain in render output, got %q", got)
	}
}
