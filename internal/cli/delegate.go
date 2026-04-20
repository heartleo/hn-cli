package cli

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	hn "github.com/heartleo/hn-cli"
	"github.com/mattn/go-runewidth"
)

type storyDelegate struct {
	width           int
	translations    map[int]string
	translating     map[int]bool
	showTranslation map[int]bool
}

type storyDelegateStyles struct {
	rank            lipgloss.Style
	title           lipgloss.Style
	domain          lipgloss.Style
	meta            lipgloss.Style
	score           lipgloss.Style
	comment         lipgloss.Style
	askPrefix       lipgloss.Style
	showPrefix      lipgloss.Style
	selectedRank    lipgloss.Style
	selectedTitle   lipgloss.Style
	selectedDomain  lipgloss.Style
	selectedMeta    lipgloss.Style
	selectedScore   lipgloss.Style
	selectedComment lipgloss.Style
	selectedAsk     lipgloss.Style
	selectedShow    lipgloss.Style
	selectedBG      lipgloss.Style
	selectedCursor  string
}

var storyStyles = newStoryDelegateStyles()

func refreshStoryDelegateStyles() {
	storyStyles = newStoryDelegateStyles()
}

func newStoryDelegateStyles() storyDelegateStyles {
	bg := currentTheme.Surface
	return storyDelegateStyles{
		rank:            lipgloss.NewStyle().Foreground(currentTheme.Muted),
		title:           lipgloss.NewStyle().Foreground(currentTheme.Title),
		domain:          lipgloss.NewStyle().Foreground(currentTheme.Muted),
		meta:            lipgloss.NewStyle().Foreground(currentTheme.Muted),
		score:           lipgloss.NewStyle().Foreground(currentTheme.Score),
		comment:         lipgloss.NewStyle().Foreground(currentTheme.Comment),
		askPrefix:       lipgloss.NewStyle().Foreground(currentTheme.Warning),
		showPrefix:      lipgloss.NewStyle().Foreground(currentTheme.Info),
		selectedRank:    lipgloss.NewStyle().Foreground(currentTheme.Muted).Background(bg),
		selectedTitle:   lipgloss.NewStyle().Foreground(currentTheme.Title).Background(bg).Bold(true),
		selectedDomain:  lipgloss.NewStyle().Foreground(currentTheme.Muted).Background(bg),
		selectedMeta:    lipgloss.NewStyle().Foreground(currentTheme.Muted).Background(bg),
		selectedScore:   lipgloss.NewStyle().Foreground(currentTheme.Score).Background(bg),
		selectedComment: lipgloss.NewStyle().Foreground(currentTheme.Comment).Background(bg),
		selectedAsk:     lipgloss.NewStyle().Foreground(currentTheme.Warning).Background(bg),
		selectedShow:    lipgloss.NewStyle().Foreground(currentTheme.Info).Background(bg),
		selectedBG:      lipgloss.NewStyle().Background(bg),
		selectedCursor:  lipgloss.NewStyle().Foreground(currentTheme.Accent).Bold(true).Background(bg).Render("❯ "),
	}
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

	styles := storyStyles
	rankStyle := styles.rank
	titleStyle := styles.title
	domainStyle := styles.domain
	metaStyle := styles.meta
	scoreStyle := styles.score
	commentStyle := styles.comment
	askPrefixStyle := styles.askPrefix
	showPrefixStyle := styles.showPrefix
	cursor := "  "
	spacer := " "

	if selected {
		cursor = styles.selectedCursor
		spacer = styles.selectedBG.Render(" ")
		rankStyle = styles.selectedRank
		titleStyle = styles.selectedTitle
		domainStyle = styles.selectedDomain
		metaStyle = styles.selectedMeta
		scoreStyle = styles.selectedScore
		commentStyle = styles.selectedComment
		askPrefixStyle = styles.selectedAsk
		showPrefixStyle = styles.selectedShow
	}

	// --- Line 1: rank + title + domain ---
	rank := rankStyle.Render(fmt.Sprintf("%2d.", story.Rank))

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
	if story.Domain != "" {
		domainSuffix = spacer + domainStyle.Render("("+story.Domain+")")
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
		rest := title[7:]
		styledTitle = askPrefixStyle.Render("Ask HN:") + titleStyle.Render(runewidth.Truncate(rest, titleMaxWidth-7, "…"))
	} else if strings.HasPrefix(title, "Show HN:") {
		rest := title[8:]
		styledTitle = showPrefixStyle.Render("Show HN:") + titleStyle.Render(runewidth.Truncate(rest, titleMaxWidth-8, "…"))
	} else {
		styledTitle = titleStyle.Render(runewidth.Truncate(title, titleMaxWidth, "…"))
	}

	line1 := cursor + rank + spacer + styledTitle + domainSuffix

	// Pad line1 to full width for selected background
	if selected {
		pad := width + 2 - lipgloss.Width(line1)
		if pad > 0 {
			line1 += styles.selectedBG.Render(strings.Repeat(" ", pad))
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
		line2 = styles.selectedBG.Render(indent) + strings.TrimPrefix(line2, indent)
		pad := width + 2 - lipgloss.Width(line2)
		if pad > 0 {
			line2 += styles.selectedBG.Render(strings.Repeat(" ", pad))
		}
	}

	// --- Line 3: points by author time | comments ---
	sp := spacer
	meta := scoreStyle.Render(fmt.Sprintf("%d points", story.Item.Score)) +
		sp + metaStyle.Render("by") +
		sp + metaStyle.Render(story.Item.By) +
		sp + metaStyle.Render(story.Item.RelativeTime()+" ago") +
		sp + commentStyle.Render(fmt.Sprintf("| %d comments", story.Item.Descendants))
	line3 := indent + meta

	if selected {
		line3 = styles.selectedBG.Render(indent) + meta
		pad := width + 2 - lipgloss.Width(line3)
		if pad > 0 {
			line3 += styles.selectedBG.Render(strings.Repeat(" ", pad))
		}
	}

	fmt.Fprintf(w, "%s\n%s\n%s", line1, line2, line3)
}
