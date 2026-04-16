package cli

import (
	"fmt"
	"io"
	"net/url"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/heartleo/hn"
	"github.com/mattn/go-runewidth"
)

type storyDelegate struct {
	width           int
	translations    map[int]string
	translating     map[int]bool
	showTranslation map[int]bool
}

func (d storyDelegate) Height() int                             { return 3 }
func (d storyDelegate) Spacing() int                            { return 1 }
func (d storyDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d storyDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	story, ok := item.(hn.Story)
	if !ok {
		return
	}

	selected := index == m.Index()
	width := d.width - 4 // left padding + margin

	// --- Styles ---
	rankStyle := lipgloss.NewStyle().Foreground(currentTheme.Muted)
	titleStyle := lipgloss.NewStyle().Foreground(currentTheme.Title)
	domainStyle := lipgloss.NewStyle().Foreground(currentTheme.Muted)
	metaStyle := lipgloss.NewStyle().Foreground(currentTheme.Muted)
	scoreStyle := lipgloss.NewStyle().Foreground(currentTheme.Score)
	commentStyle := lipgloss.NewStyle().Foreground(currentTheme.Comment)
	cursor := "  "

	bgStyle := lipgloss.NewStyle()
	if selected {
		bg := currentTheme.Surface
		bgStyle = bgStyle.Background(bg)
		cursor = lipgloss.NewStyle().Foreground(currentTheme.Accent).Bold(true).Background(bg).Render("❯ ")
		rankStyle = rankStyle.Background(bg)
		titleStyle = titleStyle.Background(bg).Bold(true)
		domainStyle = domainStyle.Background(bg)
		metaStyle = metaStyle.Background(bg)
		scoreStyle = scoreStyle.Background(bg)
		commentStyle = commentStyle.Background(bg)
	}

	// --- Line 1: rank + title + domain ---
	rank := rankStyle.Render(fmt.Sprintf("%2d.", story.Rank))

	// Extract domain from URL
	domain := ""
	if story.Item.URL != "" {
		if u, err := url.Parse(story.Item.URL); err == nil {
			domain = u.Hostname()
			domain = strings.TrimPrefix(domain, "www.")
		}
	}

	// Title with prefix highlighting
	title := story.Item.Title
	translationLine := ""
	if d.showTranslation[story.Item.ID] {
		if translated := d.translations[story.Item.ID]; translated != "" {
			translationLine = translated
		}
	} else if d.translating[story.Item.ID] {
		translationLine = "translating..."
	}
	domainSuffix := ""
	if domain != "" {
		domainSuffix = bgStyle.Render(" ") + domainStyle.Render("("+domain+")")
	}

	domainWidth := lipgloss.Width(domainSuffix)
	rankWidth := lipgloss.Width(rank)
	// cursor(2) + rank + space(1) + title + domain
	titleMaxWidth := width - 2 - rankWidth - 1 - domainWidth
	if titleMaxWidth < 20 {
		titleMaxWidth = 20
	}

	var styledTitle string
	if strings.HasPrefix(title, "Ask HN:") {
		prefixStyle := lipgloss.NewStyle().Foreground(currentTheme.Warning)
		if selected {
			prefixStyle = prefixStyle.Background(currentTheme.Surface)
		}
		rest := title[7:]
		styledTitle = prefixStyle.Render("Ask HN:") + titleStyle.Render(runewidth.Truncate(rest, titleMaxWidth-7, "…"))
	} else if strings.HasPrefix(title, "Show HN:") {
		prefixStyle := lipgloss.NewStyle().Foreground(currentTheme.Info)
		if selected {
			prefixStyle = prefixStyle.Background(currentTheme.Surface)
		}
		rest := title[8:]
		styledTitle = prefixStyle.Render("Show HN:") + titleStyle.Render(runewidth.Truncate(rest, titleMaxWidth-8, "…"))
	} else {
		styledTitle = titleStyle.Render(runewidth.Truncate(title, titleMaxWidth, "…"))
	}

	line1 := cursor + rank + bgStyle.Render(" ") + styledTitle + domainSuffix

	// Pad line1 to full width for selected background
	if selected {
		pad := width + 2 - lipgloss.Width(line1)
		if pad > 0 {
			line1 += lipgloss.NewStyle().Background(currentTheme.Surface).Render(strings.Repeat(" ", pad))
		}
	}

	// --- Line 2: translation ---
	indent := "      " // align with title start (cursor + rank + space)
	var line2 string
	if translationLine != "" {
		translationMaxWidth := width + 2 - lipgloss.Width(indent)
		if translationMaxWidth < 20 {
			translationMaxWidth = 20
		}
		translated := metaStyle.Render(runewidth.Truncate(translationLine, translationMaxWidth, "…"))
		line2 = indent + translated
	} else {
		line2 = indent
	}

	if selected {
		line2 = bgStyle.Render(indent) + strings.TrimPrefix(line2, indent)
		pad := width + 2 - lipgloss.Width(line2)
		if pad > 0 {
			line2 += bgStyle.Render(strings.Repeat(" ", pad))
		}
	}

	// --- Line 3: points by author time | comments ---
	sp := bgStyle.Render(" ")
	meta := scoreStyle.Render(fmt.Sprintf("%d points", story.Item.Score)) +
		sp + metaStyle.Render("by") +
		sp + metaStyle.Render(story.Item.By) +
		sp + metaStyle.Render(story.Item.RelativeTime()+" ago") +
		sp + commentStyle.Render(fmt.Sprintf("| %d comments", story.Item.Descendants))
	line3 := indent + meta

	if selected {
		line3 = bgStyle.Render(indent) + meta
		pad := width + 2 - lipgloss.Width(line3)
		if pad > 0 {
			line3 += bgStyle.Render(strings.Repeat(" ", pad))
		}
	}

	fmt.Fprintf(w, "%s\n%s\n%s", line1, line2, line3)
}
