package digest

import (
	"bytes"
	"fmt"
	"html/template"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

// Page is everything the HTML shell needs.
type Page struct {
	Lang        Lang
	Body        template.HTML
	Stats       Stats
	GeneratedAt time.Time
}

// Render converts the model's markdown into a full HTML page.
//
// goldmark runs without html.WithUnsafe, so any raw HTML in the markdown is
// escaped rather than emitted. That is the containment for HN titles, which are
// untrusted input that reaches this markdown by way of the model.
func Render(markdown string, lang Lang, stats Stats, generatedAt time.Time) ([]byte, error) {
	md := goldmark.New(goldmark.WithExtensions(extension.GFM))

	var body bytes.Buffer
	if err := md.Convert([]byte(markdown), &body); err != nil {
		return nil, fmt.Errorf("render markdown: %w", err)
	}

	page := Page{
		Lang:        lang,
		Body:        template.HTML(body.String()), //nolint:gosec // goldmark ran in safe mode; raw HTML is escaped
		Stats:       stats,
		GeneratedAt: generatedAt,
	}

	var out bytes.Buffer
	if err := pageTemplate.Execute(&out, page); err != nil {
		return nil, fmt.Errorf("render page: %w", err)
	}
	return out.Bytes(), nil
}

// Text returns per-language chrome. Only the shell is translated here; the
// digest body is written in the target language by the model.
func (p Page) Text(key string) string {
	zh := map[string]string{
		"title":     "HN Digest · Hacker News 每日热点",
		"tagline":   "扫一眼 Hacker News 首页：今天的热点方向和行业分布",
		"stories":   "条",
		"top":       "最高",
		"comments":  "评论",
		"other":     "English",
		"otherHref": "/en/",
		"source":    "数据来自 Hacker News",
	}
	en := map[string]string{
		"title":     "HN Digest · What's on Hacker News",
		"tagline":   "A scan of the Hacker News front page: today's themes and industry mix",
		"stories":   "stories",
		"top":       "top",
		"comments":  "comments",
		"other":     "中文",
		"otherHref": "/",
		"source":    "Data from Hacker News",
	}
	if p.Lang == LangZH {
		return zh[key]
	}
	return en[key]
}

// Dateline is the issue line under the wordmark. There are no issue numbers —
// the page is regenerated in place rather than published in editions — so the
// date of the newest story on the page does that job.
func (p Page) Dateline() string {
	if p.Lang == LangZH {
		return p.Stats.Newest.Format("2006年1月2日")
	}
	return p.Stats.Newest.Format("January 2, 2006")
}

// HTMLLang is the lang attribute for the document.
func (p Page) HTMLLang() string {
	if p.Lang == LangZH {
		return "zh-Hans"
	}
	return "en"
}

// Generated is the build time, emitted as a meta tag rather than as footer text.
// The page rewrites itself hourly, so a reader looking at a cached copy has no
// other way to tell how old it is; keeping it out of the visible footer means it
// costs the reader nothing.
func (p Page) Generated() string {
	return p.GeneratedAt.UTC().Format(time.RFC3339)
}

// Comma formats n with thousands separators.
func (p Page) Comma(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	return string(out)
}

// The palette follows Hacker News Digest rather than news.ycombinator.com: a
// wordmark in a red-to-orange gradient over white, stories led by their rank,
// title set in the brand red, and the metadata demoted to a line of its own.
//
// Measured against white, which decides which of those colours may carry text:
//
//	#d43713  4.82:1  brand red     — clears AA for body text, so titles use it
//	#e88b2b  2.57:1  brand orange  — fails everything; wordmark gradient only
//	#ff6600  2.94:1  HN's orange   — misses even the 3:1 large-text bar
//	#222222 15.91:1  body
//	#6b6b6b  5.33:1  metadata
//
// That last row is why HN itself sits on #f6f6ef: its orange only works over a
// tinted background. On white, the digest's darker red is the one that can be
// text at all, and #ff6600 is confined to non-text marks.
var pageTemplate = template.Must(template.New("page").Parse(`<!DOCTYPE html>
<html lang="{{.HTMLLang}}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Text "title"}}</title>
<meta name="description" content="{{.Text "tagline"}}">
<meta name="generator" content="hn-digest">
<meta name="date" content="{{.Generated}}">
<link rel="icon" href="data:image/svg+xml,<svg xmlns=%22http://www.w3.org/2000/svg%22 viewBox=%220 0 32 32%22><rect width=%2232%22 height=%2232%22 fill=%22%23d43713%22/><text x=%2216%22 y=%2224%22 font-size=%2220%22 text-anchor=%22middle%22 fill=%22white%22 font-family=%22Helvetica,Arial%22 font-weight=%22bold%22>H</text></svg>">
<style>
:root {
  --bg: #ffffff;
  --ink: #222222;
  --muted: #6b6b6b;
  --brand: #d43713;
  --brand-2: #e88b2b;
  --rule: #e1e1e1;
  --surface: #f4f4f4;
}
* { box-sizing: border-box; }
body {
  margin: 0;
  background: var(--bg);
  color: var(--ink);
  /* Latin faces first — none of them carry CJK, so Chinese falls through to the
     CJK entries. No webfont: a webfont could only ever style the English half,
     and would cost every reader a download plus a swap. */
  font-family: "Helvetica Neue", Helvetica, Arial,
               "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", "Noto Sans CJK SC",
               sans-serif;
  /* 16px floor: this page is prose, and mobile bodies below 16px also trigger
     iOS auto-zoom. */
  font-size: 16px;
  line-height: 1.65;
}
/* Sized for both halves of the site at 16px: ~71 Latin characters per line, and
   ~36 Chinese characters — CJK glyphs are a full em, so the same box measures
   very differently in each language. */
.wrap { max-width: 600px; margin: 0 auto; padding: 0 20px 56px; }

/* --- Masthead ------------------------------------------------------------ */
.masthead { padding: 36px 0 28px; border-bottom: 1px solid var(--rule); }
.wordmark {
  margin: 0;
  font-size: 40px;
  font-weight: 800;
  letter-spacing: -0.02em;
  line-height: 1.1;
  background: linear-gradient(90deg, var(--brand) 0%, var(--brand-2) 100%);
  -webkit-background-clip: text;
  background-clip: text;
  color: var(--brand);
}
/* The gradient only paints where the browser can clip it to the glyphs;
   everywhere else the solid --brand above stays, which is the accessible one. */
@supports ((-webkit-background-clip: text) or (background-clip: text)) {
  .wordmark { color: transparent; }
}
.dateline { margin: 6px 0 0; color: var(--muted); font-size: 14px; }
.lang {
  float: right;
  color: var(--muted);
  font-size: 14px;
  text-decoration: none;
  /* 44px tall: the one persistent control on the page is a real touch target. */
  line-height: 44px;
  padding: 0 4px;
}
.lang:hover { color: var(--brand); text-decoration: underline; }

/* --- Sections ------------------------------------------------------------ */
/* Oversized and tightly tracked, carried by whitespace rather than by rules or
   numbers. The size difference alone separates the sections. */
h2 {
  color: var(--ink);
  font-size: 30px;
  font-weight: 800;
  letter-spacing: -0.02em;
  line-height: 1.2;
  margin: 64px 0 18px;
}
h3 { color: var(--ink); font-size: 16px; margin: 20px 0 4px; }
p { margin: 0 0 14px; }
a { color: var(--brand); }
a:visited { color: var(--brand); }

/* --- Story list ---------------------------------------------------------- */
ol { padding-left: 34px; margin: 0; }
ol li { margin-bottom: 26px; }
/* Rank reads as a column label, not as content — the digest's own convention. */
ol li::marker { color: var(--muted); font-size: 14px; }
/* First paragraph is the title, second the metadata, the rest the reason. */
ol li > p { margin: 0 0 4px; }
ol li > p:first-child { font-size: 19px; line-height: 1.35; }
ol li > p:first-child a { text-decoration: none; font-weight: 400; }
ol li > p:first-child a:hover { text-decoration: underline; }
ol li > p:first-child strong { font-weight: 400; }
ol li > p:first-child + p { color: var(--muted); font-size: 14px; margin-bottom: 8px; }
ol li > p:first-child + p code { color: var(--muted); }

code {
  font-family: Menlo, Consolas, monospace;
  font-size: 0.875em;
  background: none;
  padding: 0;
  color: var(--muted);
}
pre {
  background: var(--surface);
  border-radius: 3px;
  padding: 16px;
  overflow-x: auto;
  line-height: 1.5;
}
pre code { font-size: 14px; color: var(--ink); }
blockquote {
  margin: 0 0 14px;
  padding-left: 14px;
  border-left: 3px solid var(--brand);
  color: var(--muted);
}
footer {
  margin-top: 48px;
  background: var(--surface);
  border-radius: 3px;
  padding: 18px;
  color: var(--muted);
  font-size: 13px;
}
footer p { margin: 0 0 6px; }
footer p:last-child { margin: 0; }
@media (max-width: 480px) {
  .wordmark { font-size: 30px; }
  h2 { font-size: 24px; margin: 48px 0 14px; }
  .masthead { padding: 24px 0 20px; }
  /* The bar chart is fixed-width monospace art; let it scroll rather than shrink
     the body text below the 16px floor to make it fit. */
  pre code { font-size: 12px; }
}
</style>
</head>
<body>
<div class="wrap">
  <header class="masthead">
    <a class="lang" href="{{.Text "otherHref"}}">{{.Text "other"}}</a>
    <h1 class="wordmark">HN Digest</h1>
    <p class="dateline">{{.Dateline}} · <code>{{.Stats.Count}}</code> {{.Text "stories"}} · {{.Text "top"}} <code>{{.Stats.TopPoints}}</code> · <code>{{$.Comma .Stats.TotalComments}}</code> {{.Text "comments"}}</p>
  </header>
  {{.Body}}
  <footer>
    <p>{{.Text "source"}}</p>
  </footer>
</div>
</body>
</html>
`))
