package cli

import (
	"fmt"
	"strings"

	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	md "github.com/JohannesKaufmann/html-to-markdown/v2"
	hn "github.com/heartleo/hn-cli"
)

const (
	maxIndentDepth    = 5
	commentIndentSize = 4
)

// flatComment is a flattened view of a comment tree node, used for cursor navigation.
type flatComment struct {
	comment   *hn.Comment
	indent    string   // pre-built indent prefix (depth-dependent)
	lines     []string // pre-rendered lines (without indent or select bar)
	startLine int      // first line index in assembled output
	endLine   int      // last line index (exclusive)
}

// markdownCache caches per-comment HTML-to-Markdown conversion (keyed by item ID).
// Stores only the data transformation result — no width or layout information.
type markdownCache map[int]string

type commentTranslationSource func(*hn.Comment) (string, bool)

// getBody returns rendered body lines for a comment.
// bodySource provides original markdown; translationSource optionally appends
// translated markdown with a muted quote gutter.
// Glamour rendering runs fresh with the current width every time.
func getBody(c *hn.Comment, width int, bodySource func(*hn.Comment) string, translationSource commentTranslationSource) []string {
	markdown := bodySource(c)

	depth := c.Depth
	if depth > maxIndentDepth {
		depth = maxIndentDepth
	}
	contentWidth := width - depth*commentIndentSize - 6
	if contentWidth < 30 {
		contentWidth = 30
	}
	body := renderMarkdown(markdown, contentWidth)
	lines := strings.Split(body, "\n")
	if translationSource != nil {
		if translated, ok := translationSource(c); ok && strings.TrimSpace(translated) != "" {
			lines = append(lines, renderTranslationDivider(contentWidth))
			lines = append(lines, renderTranslatedMarkdown(translated, contentWidth)...)
		}
	}
	return lines
}

func renderTranslationDivider(width int) string {
	if width < 1 {
		width = 1
	}
	return lipgloss.NewStyle().
		Foreground(currentTheme.Muted).
		Render(strings.Repeat("\u2500", width))
}

func renderTranslatedMarkdown(markdown string, width int) []string {
	const gutter = "\u2502 "
	gutterStyle := lipgloss.NewStyle().Foreground(currentTheme.Muted)
	contentWidth := width - lipgloss.Width(gutter)
	if contentWidth < 20 {
		contentWidth = 20
	}

	rendered := renderMarkdown(markdown, contentWidth)
	lines := strings.Split(rendered, "\n")
	for i := range lines {
		if strings.TrimSpace(lines[i]) == "" {
			lines[i] = gutterStyle.Render("\u2502")
			continue
		}
		lines[i] = gutterStyle.Render(gutter) + lines[i]
	}
	return lines
}

// buildFlatComments flattens visible comments into a list.
// Uses cache so HTML→Glamour rendering happens only once per comment.
func buildFlatComments(comments []*hn.Comment, collapsed map[int]bool, width int, bodySource func(*hn.Comment) string, translationSource commentTranslationSource, commentStatus func(*hn.Comment) string, repliesStatus func(*hn.Comment) string, childrenExpanded func(*hn.Comment) bool) []flatComment {
	var flat []flatComment

	var walk func(comments []*hn.Comment, forceExpanded bool)
	walk = func(comments []*hn.Comment, forceExpanded bool) {
		for _, c := range comments {
			isCollapsed := collapsed[c.Item.ID]

			depth := c.Depth
			if depth > maxIndentDepth {
				depth = maxIndentDepth
			}

			indent := buildIndent(depth)

			// Header styles
			authorStyle := lipgloss.NewStyle().Foreground(currentTheme.Accent).Bold(true)
			timeStyle := lipgloss.NewStyle().Foreground(currentTheme.Muted)
			sepStyle := lipgloss.NewStyle().Foreground(currentTheme.Surface)
			navStyle := lipgloss.NewStyle().Foreground(currentTheme.Muted)

			var lines []string

			if isCollapsed {
				// HN style collapsed: "▶ user · 2h | prev | next [N more]"
				triangle := lipgloss.NewStyle().Foreground(currentTheme.Muted).Render("▶ ")
				header := authorStyle.Render(c.Item.By) +
					timeStyle.Render(" · "+c.Item.RelativeTime())
				// count = self + direct children
				moreCount := len(c.Item.Kids) + 1
				header += sepStyle.Render(" | ") + navStyle.Render(fmt.Sprintf("[%d more]", moreCount))
				if commentStatus != nil {
					if status := commentStatus(c); status != "" {
						header += sepStyle.Render(" | ") + status
					}
				}

				lines = append(lines, triangle+header)
			} else {
				// Expanded: all comments show ▼ (any comment can be collapsed)
				triangle := lipgloss.NewStyle().Foreground(currentTheme.Accent).Render("▼ ")
				header := authorStyle.Render(c.Item.By) + timeStyle.Render(" · "+c.Item.RelativeTime())
				if repliesStatus != nil {
					if status := repliesStatus(c); status != "" {
						header += sepStyle.Render(" | ") + status
					}
				}
				if commentStatus != nil {
					if status := commentStatus(c); status != "" {
						header += sepStyle.Render(" | ") + status
					}
				}
				lines = append(lines, triangle+header)

				// Body from cache
				for _, bodyLine := range getBody(c, width, bodySource, translationSource) {
					lines = append(lines, "  "+bodyLine)
				}
			}
			lines = append(lines, "") // separator

			flat = append(flat, flatComment{
				comment: c,
				indent:  indent,
				lines:   lines,
			})

			expanded := forceExpanded || childrenExpanded == nil || childrenExpanded(c)
			if !isCollapsed && expanded {
				walk(c.Children, expanded)
			}
		}
	}

	walk(comments, false)

	// Compute line ranges
	line := 0
	for i := range flat {
		flat[i].startLine = line
		line += len(flat[i].lines)
		flat[i].endLine = line
	}

	return flat
}

// assembleView combines pre-rendered flat comments with selection highlight.
// This is cheap — just string concatenation with a 2-char prefix per line.
func assembleView(flat []flatComment, selectedIdx, width int) string {
	var b strings.Builder
	selectBarOn := lipgloss.NewStyle().Foreground(currentTheme.Accent).Render("▎ ")
	selectBarOff := "  "

	// Extract raw ANSI codes for background so we can re-inject after internal resets.
	const marker = "\x00"
	bgStyled := lipgloss.NewStyle().Background(currentTheme.Surface).Render(marker)
	bgParts := strings.SplitN(bgStyled, marker, 2)
	bgOpen, bgClose := "", ""
	if len(bgParts) == 2 {
		bgOpen = bgParts[0]
		bgClose = bgParts[1]
	}

	for i, fc := range flat {
		selected := i == selectedIdx
		bar := selectBarOff
		if selected {
			bar = selectBarOn
		}
		for _, line := range fc.lines {
			if selected && bgOpen != "" {
				// Keep the nesting gutter and selection bar transparent.
				highlighted := line
				pad := width - lipgloss.Width(fc.indent) - lipgloss.Width(bar) - lipgloss.Width(highlighted)
				if pad > 0 {
					highlighted += strings.Repeat(" ", pad)
				}
				// Re-apply background after every internal ANSI reset
				highlighted = bgOpen + strings.ReplaceAll(highlighted, bgClose, bgClose+bgOpen) + bgClose
				b.WriteString(fc.indent + bar + highlighted)
			} else {
				b.WriteString(fc.indent + bar + line)
			}
			b.WriteByte('\n')
		}
	}

	return b.String()
}

func buildIndent(depth int) string {
	if depth == 0 {
		return ""
	}
	return strings.Repeat(" ", depth*commentIndentSize)
}

// htmlToMarkdown converts HN HTML content to markdown. Pure data transformation,
// no width dependency. Result is safe to cache indefinitely.
func htmlToMarkdown(htmlStr string) string {
	if htmlStr == "" {
		return ""
	}
	markdown, err := md.ConvertString(htmlStr)
	if err != nil {
		return htmlStr
	}
	return markdown
}

// renderMarkdown renders a markdown string to terminal output with word wrap.
// Width-dependent — must be called fresh on every rebuild.
func renderMarkdown(markdownStr string, width int) string {
	if markdownStr == "" {
		return ""
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return markdownStr
	}
	rendered, err := r.Render(markdownStr)
	if err != nil {
		return markdownStr
	}
	return strings.TrimSpace(rendered)
}
