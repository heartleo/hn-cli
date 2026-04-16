package cli

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/heartleo/hn"
)

func TestHtmlToMarkdown(t *testing.T) {
	html := "Hello <i>world</i>"
	md := htmlToMarkdown(html)
	if md == "" {
		t.Fatal("expected non-empty markdown")
	}
	if md == html {
		t.Fatal("expected markdown conversion, got raw HTML back")
	}
}

func TestHtmlToMarkdownEmpty(t *testing.T) {
	if got := htmlToMarkdown(""); got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

func TestRenderMarkdownRespectsWidth(t *testing.T) {
	md := "This is a fairly long line that should wrap differently at different widths for testing purposes"
	narrow := renderMarkdown(md, 30)
	wide := renderMarkdown(md, 200)
	if narrow == wide {
		t.Fatal("expected different output at different widths")
	}
}

func TestRenderTranslatedMarkdownAddsGutterToEveryLine(t *testing.T) {
	got := renderTranslatedMarkdown("This is a fairly long translated comment that should wrap across multiple lines for testing.\n\nSecond paragraph.", 32)
	if len(got) < 3 {
		t.Fatalf("expected translated markdown to render multiple lines, got %#v", got)
	}
	for _, line := range got {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.Contains(line, "\u2502") {
			t.Fatalf("expected quote gutter on every translated line, got %#v", got)
		}
	}
}

func TestGetBodyAddsDividerBeforeTranslation(t *testing.T) {
	comment := &hn.Comment{Item: hn.Item{ID: 1, Text: "Original comment"}}
	got := getBody(comment, 80,
		func(c *hn.Comment) string { return c.Item.Text },
		func(c *hn.Comment) (string, bool) { return "Translated comment", true },
	)
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, "\u2500\u2500") {
		t.Fatalf("expected translation divider, got %#v", got)
	}
	dividerIndex := strings.Index(joined, "\u2500")
	gutterIndex := strings.Index(joined, "\u2502")
	if dividerIndex < 0 || gutterIndex < 0 || dividerIndex > gutterIndex {
		t.Fatalf("expected divider before translated quote gutter, got %#v", got)
	}
}

func TestBuildIndentUsesWiderCommentNesting(t *testing.T) {
	if got := buildIndent(2); got != strings.Repeat(" ", 8) {
		t.Fatalf("expected two levels to indent 8 spaces, got %q", got)
	}
}

func TestAssembleViewKeepsSelectionGutterTransparent(t *testing.T) {
	const marker = "\x00"
	bgStyled := lipgloss.NewStyle().Background(currentTheme.Surface).Render(marker)
	bgParts := strings.SplitN(bgStyled, marker, 2)
	if len(bgParts) != 2 {
		t.Fatal("expected background ANSI markers")
	}

	bar := lipgloss.NewStyle().Foreground(currentTheme.Accent).Render("▎ ")
	view := assembleView([]flatComment{
		{
			indent: "  ",
			lines:  []string{"child comment"},
		},
	}, 0, 30)

	expectedPrefix := "  " + bar + bgParts[0]
	if !strings.HasPrefix(view, expectedPrefix) {
		t.Fatalf("expected background after child indent and selection bar, got %q", view)
	}
}
