package scrape

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	hn "github.com/heartleo/hn-cli"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// parseThread consumes an HN item page and returns the full comment tree.
// Each comment row's indent depth (stored as the width of its td.ind spacer
// image in 40-pixel steps) is used to rebuild the tree from the flat DOM.
// Rows without a commtext div (flagged/dead comments, or not visible to
// logged-out viewers) are skipped.
func parseThread(r io.Reader, storyID int) ([]*hn.Comment, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("html parse: %w", err)
	}
	flat := make([]*flatComment, 0, 64)
	walkRows(doc, func(row *html.Node) {
		if fc := parseRow(row); fc != nil {
			flat = append(flat, fc)
		}
	})
	return buildTree(flat, storyID), nil
}

type flatComment struct {
	depth int
	c     *hn.Comment
}

// walkRows invokes visit on every tr.athing.comtr in document order. These
// rows are siblings in HN's markup (nesting is purely visual via indent),
// so once a row is visited we can stop descending into it.
func walkRows(n *html.Node, visit func(*html.Node)) {
	if n.Type == html.ElementNode &&
		n.DataAtom == atom.Tr &&
		hasClass(n, "athing") && hasClass(n, "comtr") {
		visit(n)
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walkRows(c, visit)
	}
}

func parseRow(row *html.Node) *flatComment {
	idNum, err := strconv.Atoi(attr(row, "id"))
	if err != nil {
		return nil
	}
	textNode := findNode(row, func(n *html.Node) bool {
		return n.DataAtom == atom.Div && hasClass(n, "commtext")
	})
	if textNode == nil {
		return nil
	}
	text := innerHTML(textNode)
	if strings.TrimSpace(text) == "" {
		return nil
	}
	depth := rowDepth(row)
	user := ""
	if n := findNode(row, func(n *html.Node) bool {
		return n.DataAtom == atom.A && hasClass(n, "hnuser")
	}); n != nil {
		user = textOf(n)
	}
	var ts int64
	if n := findNode(row, func(n *html.Node) bool {
		return n.DataAtom == atom.Span && hasClass(n, "age")
	}); n != nil {
		ts = parseAgeTitle(attr(n, "title"))
	}
	return &flatComment{
		depth: depth,
		c: &hn.Comment{
			Item: hn.Item{
				ID:   idNum,
				By:   user,
				Time: ts,
				Text: text,
				Type: "comment",
			},
			Depth: depth,
		},
	}
}

// rowDepth extracts HN's visual indent depth. HN encodes it as the width
// attribute (in pixels) of the s.gif spacer inside td.ind — 40 px per level.
func rowDepth(row *html.Node) int {
	n := findNode(row, func(n *html.Node) bool {
		return n.DataAtom == atom.Img &&
			n.Parent != nil &&
			n.Parent.DataAtom == atom.Td &&
			hasClass(n.Parent, "ind")
	})
	if n == nil {
		return 0
	}
	w, err := strconv.Atoi(attr(n, "width"))
	if err != nil {
		return 0
	}
	return w / 40
}

// parseAgeTitle extracts unix seconds from HN's age title.
// Format: "2025-11-05T18:22:11 1762369331".
func parseAgeTitle(title string) int64 {
	idx := strings.LastIndex(title, " ")
	if idx <= 0 {
		return 0
	}
	t, err := strconv.ParseInt(title[idx+1:], 10, 64)
	if err != nil {
		return 0
	}
	return t
}

// buildTree rebuilds the comment tree from flat rows (in HN display order)
// using a stack keyed by indent depth.
func buildTree(flat []*flatComment, storyID int) []*hn.Comment {
	roots := make([]*hn.Comment, 0, 16)
	stack := make([]*hn.Comment, 0, 16)
	for _, fc := range flat {
		for len(stack) > fc.depth {
			stack = stack[:len(stack)-1]
		}
		if len(stack) == 0 {
			fc.c.Item.Parent = storyID
			roots = append(roots, fc.c)
		} else {
			parent := stack[len(stack)-1]
			fc.c.Item.Parent = parent.Item.ID
			parent.Children = append(parent.Children, fc.c)
			parent.Item.Kids = append(parent.Item.Kids, fc.c.Item.ID)
		}
		stack = append(stack, fc.c)
	}
	return roots
}

// --- DOM helpers ---

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func hasClass(n *html.Node, class string) bool {
	for _, c := range strings.Fields(attr(n, "class")) {
		if c == class {
			return true
		}
	}
	return false
}

func findNode(root *html.Node, pred func(*html.Node) bool) *html.Node {
	if root == nil {
		return nil
	}
	if root.Type == html.ElementNode && pred(root) {
		return root
	}
	for c := root.FirstChild; c != nil; c = c.NextSibling {
		if got := findNode(c, pred); got != nil {
			return got
		}
	}
	return nil
}

func textOf(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

func innerHTML(n *html.Node) string {
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if err := html.Render(&b, c); err != nil {
			return ""
		}
	}
	return b.String()
}
