package digest

import (
	"bytes"
	"fmt"
	"html"
	"html/template"
	"regexp"
	"strconv"
	"strings"
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
//
// The trailing industry-mix bar block is pulled out of the markdown first and
// re-rendered as a CSS bar chart (see extractChart); the monospace block stays
// as the fallback for output that doesn't follow the format.
func Render(markdown string, lang Lang, stats Stats, generatedAt time.Time) ([]byte, error) {
	md := goldmark.New(goldmark.WithExtensions(extension.GFM))

	source, bars, hasChart := extractChart(markdown)

	var body bytes.Buffer
	if err := md.Convert([]byte(source), &body); err != nil {
		return nil, fmt.Errorf("render markdown: %w", err)
	}
	out := body.String()
	if hasChart {
		out += renderChart(bars)
	}

	page := Page{
		Lang:        lang,
		Body:        template.HTML(out), //nolint:gosec // goldmark ran in safe mode; chart names are escaped in renderChart
		Stats:       stats,
		GeneratedAt: generatedAt,
	}

	var pageOut bytes.Buffer
	if err := pageTemplate.Execute(&pageOut, page); err != nil {
		return nil, fmt.Errorf("render page: %w", err)
	}
	return pageOut.Bytes(), nil
}

// chartBar is one parsed industry-mix row.
type chartBar struct {
	Name  string
	Count int
}

// trailingFence matches a fenced code block at the very end of the markdown —
// where the prompt contract puts the industry-mix bars ("the bar block is the
// last thing on the page"). If the model disobeys and appends commentary after
// it, the block no longer matches and the page keeps the monospace rendering.
var trailingFence = regexp.MustCompile("(?s)```[^\\n]*\n(.*?)```\\s*$")

// chartLine parses one bar row. The model's own run of █ characters is the
// delimiter between the name and the count, which is what makes the monospace
// format double as a data format.
var chartLine = regexp.MustCompile(`^(.+?)\s*█+\s*(\d+)\s*$`)

// extractChart splits the trailing bar block off the markdown and parses it
// into rows. ok is false — and the markdown returned untouched — whenever the
// block is missing or any line doesn't parse, so a model slip degrades to the
// pre rendering instead of a broken chart.
func extractChart(markdown string) (rest string, bars []chartBar, ok bool) {
	m := trailingFence.FindStringSubmatchIndex(markdown)
	if m == nil {
		return markdown, nil, false
	}
	for _, line := range strings.Split(markdown[m[2]:m[3]], "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lm := chartLine.FindStringSubmatch(line)
		if lm == nil {
			return markdown, nil, false
		}
		n, err := strconv.Atoi(lm[2])
		if err != nil || n <= 0 {
			return markdown, nil, false
		}
		bars = append(bars, chartBar{Name: lm[1], Count: n})
	}
	if len(bars) == 0 {
		return markdown, nil, false
	}
	return strings.TrimRight(markdown[:m[0]], "\n") + "\n", bars, true
}

// renderChart draws the rows as CSS bars, widths proportional to the largest
// count. Names come from the model by way of untrusted HN titles, so they are
// escaped here the same way goldmark escapes the rest of the body.
func renderChart(bars []chartBar) string {
	max := 0
	for _, b := range bars {
		if b.Count > max {
			max = b.Count
		}
	}
	var sb strings.Builder
	sb.WriteString(`<div class="chart">`)
	for _, b := range bars {
		fmt.Fprintf(&sb,
			`<div class="chart-row"><span class="chart-name">%s</span>`+
				`<span class="chart-track"><span class="chart-bar" style="width:%.0f%%"></span></span>`+
				`<span class="chart-n">%d</span></div>`,
			html.EscapeString(b.Name), float64(b.Count)/float64(max)*100, b.Count)
	}
	sb.WriteString(`</div>`)
	return sb.String()
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
		"source":    "数据来自",
	}
	en := map[string]string{
		"title":     "HN Digest · What's on Hacker News",
		"tagline":   "A scan of the Hacker News front page: today's themes and industry mix",
		"stories":   "stories",
		"top":       "top",
		"comments":  "comments",
		"other":     "中文",
		"otherHref": "/",
		"source":    "Data from",
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
// The page rewrites itself daily, so a reader looking at a cached copy has no
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

// The palette keeps the digest's white ground and the red-to-orange gradient of
// the wordmark, but red is rationed. Ten stories on one page means ten titles;
// set them all in red and nothing stands out — the prose links, the ranks, and
// the section marks all compete at the same volume. So titles stay ink, and red
// is reserved for what is actionable or structural: links, rank markers, the
// gradient ticks, the chart bars, and hover states.
//
// Measured against white, which decides which colours may carry text:
//
//	#d43713  4.82:1  brand red     — clears AA for body text, so links use it
//	#e88b2b  2.57:1  brand orange  — fails everything; gradient stops only
//	#ff6600  2.94:1  HN's orange   — misses even the 3:1 large-text bar
//	#222222 15.91:1  body and titles
//	#6b6b6b  5.33:1  metadata
//
// That last row is why HN itself sits on #f6f6ef: its orange only works over a
// tinted background. On white, the digest's darker red is the one that can be
// text at all, and #ff6600 may only ever paint a mark, like the Y in the footer.
var pageTemplate = template.Must(template.New("page").Parse(`<!DOCTYPE html>
<html lang="{{.HTMLLang}}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Text "title"}}</title>
<meta name="description" content="{{.Text "tagline"}}">
<meta property="og:title" content="{{.Text "title"}}">
<meta property="og:description" content="{{.Text "tagline"}}">
<meta property="og:type" content="website">
<meta name="theme-color" content="#ffffff">
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
  --rule: #e9e6e1;
  --surface: #f8f6f3;
  --track: #ece7e1;
}
* { box-sizing: border-box; }
html { -webkit-text-size-adjust: 100%; }
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
  -webkit-font-smoothing: antialiased;
}
::selection { background: #f6d9cd; }

a { color: var(--brand); text-underline-offset: 3px; }
a:visited { color: var(--brand); }
a:focus-visible { outline: 2px solid var(--brand); outline-offset: 2px; border-radius: 2px; }

/* 640px box minus the 20px gutters leaves a 600px content column: ~71 Latin
   characters per line, and ~36 Chinese characters — CJK glyphs are a full em,
   so the same box measures very differently in each language. */
.wrap { max-width: 640px; margin: 0 auto; padding: 0 20px 64px; }
/* The one place the gradient runs full-width: a thin brand bar across the top
   edge, so the page reads as a designed object before a word is read. */
.topbar { height: 3px; background: linear-gradient(90deg, var(--brand), var(--brand-2)); }

/* --- Masthead ------------------------------------------------------------ */
.masthead { position: relative; padding: 40px 0 26px; border-bottom: 1px solid var(--rule); }
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
/* The date is the issue identity, so it sits on its own line in ink; the stats
   that used to share its line are chips now, below it. */
.dateline { margin: 8px 0 0; color: var(--ink); font-size: 15px; font-weight: 500; }
/* Header stats as chips: the numbers are what a returning reader scans for, so
   they get tabular figures and a strong weight instead of inline prose. */
.stats {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  list-style: none;
  margin: 14px 0 0;
  padding: 0;
}
.stats li {
  background: var(--surface);
  border: 1px solid var(--rule);
  border-radius: 999px;
  padding: 3px 12px;
  color: var(--muted);
  font-size: 13px;
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
}
.stats li strong { color: var(--ink); font-weight: 600; }
/* The persistent controls — the author's GitHub and the language toggle — sit
   as a pill cluster at the masthead's top right. Both are 44px touch targets. */
.mastnav { position: absolute; top: 40px; right: 0; display: flex; align-items: center; gap: 10px; }
.gh {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 44px;
  height: 44px;
  border: 1px solid var(--rule);
  border-radius: 999px;
  color: var(--muted);
  transition: color 0.15s, border-color 0.15s;
}
.gh:hover { color: var(--ink); border-color: var(--ink); }
.gh svg { width: 20px; height: 20px; display: block; fill: currentColor; }
.lang {
  border: 1px solid var(--rule);
  border-radius: 999px;
  padding: 0 16px;
  /* 44px of line-height inside the border: the persistent controls on the page
     are real touch targets. */
  line-height: 44px;
  color: var(--muted);
  font-size: 14px;
  text-decoration: none;
  transition: color 0.15s, border-color 0.15s;
}
.lang:hover { color: var(--brand); border-color: var(--brand); }

/* --- Sections ------------------------------------------------------------ */
/* Oversized and tightly tracked, and now opened by a short gradient tick — the
   size still separates the sections, the tick ties them back to the brand. */
h2 {
  color: var(--ink);
  font-size: 30px;
  font-weight: 800;
  letter-spacing: -0.02em;
  line-height: 1.25;
  margin: 56px 0 14px;
  text-wrap: balance;
}
h2::before {
  content: "";
  display: block;
  width: 28px;
  height: 4px;
  border-radius: 2px;
  background: linear-gradient(90deg, var(--brand), var(--brand-2));
  margin-bottom: 14px;
}
h3 { color: var(--ink); font-size: 16px; margin: 20px 0 4px; }
p { margin: 0 0 14px; }

/* --- Story list ---------------------------------------------------------- */
ol { padding-left: 36px; margin: 0; }
/* Hairlines between stories, not whitespace alone: ten entries with ranks,
   titles, metadata chips, and reasons need the rows to read as rows. */
ol li { padding: 18px 0; border-bottom: 1px solid var(--rule); }
ol li:last-child { border-bottom: 0; }
/* The rank is structural, not content — it is one of the few places brand red
   is allowed to be loud. */
ol li::marker { color: var(--brand); font-weight: 700; font-size: 14px; font-variant-numeric: tabular-nums; }
/* First paragraph is the title, second the metadata, the rest the reason. */
ol li > p { margin: 0 0 4px; }
ol li > p:first-child { font-size: 19px; line-height: 1.4; text-wrap: balance; }
/* Titles stay ink; red on a title is the hover state, not the default. A
   visited title drops to muted — "already read" is information too. */
ol li > p:first-child a { color: var(--ink); text-decoration: none; }
ol li > p:first-child a:visited { color: var(--muted); }
ol li > p:first-child a:hover { color: var(--brand); text-decoration: underline; }
ol li > p:first-child strong { font-weight: 600; }
/* The metadata line: domain, score, and comment count as small chips, the
   discussion link demoted to a plain underlined grey — it is the secondary
   action on every row. */
ol li > p:first-child + p { color: var(--muted); font-size: 14px; line-height: 2; margin: 6px 0 8px; }
ol li > p:first-child + p code {
  background: var(--surface);
  border: 1px solid var(--rule);
  border-radius: 5px;
  padding: 1px 7px;
  font-size: 13px;
  color: var(--muted);
  white-space: nowrap;
}
ol li > p:first-child + p a { color: var(--muted); }
ol li > p:first-child + p a:hover { color: var(--brand); }

code {
  font-family: Menlo, Consolas, "Courier New", monospace;
  font-size: 0.875em;
  color: var(--muted);
  font-variant-numeric: tabular-nums;
}
/* pre is the fallback rendering of the industry-mix block, kept for model
   output that doesn't parse into chart rows (see extractChart). */
pre {
  background: var(--surface);
  border: 1px solid var(--rule);
  border-radius: 10px;
  box-shadow: 0 1px 2px rgba(34, 34, 34, 0.05);
  padding: 20px;
  overflow-x: auto;
  line-height: 1.6;
}
pre code { font-size: 14px; color: var(--ink); background: none; border: 0; padding: 0; }

/* --- Industry mix chart -------------------------------------------------- */
/* One grid for the whole panel, with each row as display:contents, so the name
   column sizes itself to the longest name across ALL rows and the bars line up.
   Widths are proportional to the largest count, which takes the full track. */
.chart {
  display: grid;
  grid-template-columns: max-content 1fr 3ch;
  column-gap: 14px;
  row-gap: 12px;
  align-items: center;
  background: var(--surface);
  border: 1px solid var(--rule);
  border-radius: 10px;
  box-shadow: 0 1px 2px rgba(34, 34, 34, 0.05);
  padding: 20px;
  font-size: 14px;
}
.chart-row { display: contents; }
.chart-name { color: var(--ink); }
.chart-track { height: 10px; border-radius: 999px; background: var(--track); overflow: hidden; }
.chart-bar {
  display: block;
  height: 100%;
  min-width: 4px;
  border-radius: 999px;
  background: linear-gradient(90deg, var(--brand), var(--brand-2));
  transition: filter 0.15s;
}
.chart-row:hover .chart-bar { filter: brightness(1.08); }
.chart-row:hover .chart-name { font-weight: 600; }
.chart-n { color: var(--muted); text-align: right; font-variant-numeric: tabular-nums; }

blockquote {
  margin: 0 0 14px;
  padding: 2px 0 2px 14px;
  border-left: 3px solid var(--brand);
  color: var(--muted);
}
/* The footer is a hairline, not a box: it is the least important thing on the
   page and should look like it. The source link itself gets the full treatment
   — the Y mark plus an ink link that warms to the brand on hover. */
footer {
  margin-top: 56px;
  padding-top: 20px;
  border-top: 1px solid var(--rule);
  color: var(--muted);
  font-size: 13px;
}
footer p { margin: 0; }
footer a {
  color: var(--ink);
  font-weight: 500;
  text-decoration: none;
  border-bottom: 1px solid var(--rule);
  padding-bottom: 1px;
  transition: color 0.15s, border-color 0.15s;
}
footer a:hover { color: var(--brand); border-color: var(--brand); }
footer .yn { width: 14px; height: 14px; vertical-align: -2px; margin-right: 5px; border-radius: 3px; }
@media (max-width: 480px) {
  .wordmark { font-size: 32px; }
  .masthead { padding: 30px 0 22px; }
  .mastnav { top: 28px; }
  .lang { padding: 0 13px; font-size: 13px; }
  h2 { font-size: 24px; margin: 44px 0 14px; }
  ol { padding-left: 30px; }
  ol li { padding: 16px 0; }
  .chart { padding: 16px; column-gap: 10px; font-size: 13px; }
  /* The fallback bar chart is fixed-width monospace art; let it scroll rather
     than shrink the body text below the 16px floor to make it fit. */
  pre code { font-size: 12px; }
}
</style>
</head>
<body>
<div class="topbar"></div>
<div class="wrap">
  <header class="masthead">
    <nav class="mastnav">
      <a class="gh" href="https://github.com/heartleo" aria-label="GitHub"><svg viewBox="0 0 16 16" aria-hidden="true"><path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27s1.36.09 2 .27c1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0 0 16 8c0-4.42-3.58-8-8-8z"/></svg></a>
      <a class="lang" href="{{.Text "otherHref"}}">{{.Text "other"}}</a>
    </nav>
    <h1 class="wordmark">HN Digest</h1>
    <p class="dateline">{{.Dateline}}</p>
    <ul class="stats">
      <li><strong>{{.Stats.Count}}</strong> {{.Text "stories"}}</li>
      <li>{{.Text "top"}} <strong>{{.Stats.TopPoints}}</strong></li>
      <li><strong>{{$.Comma .Stats.TotalComments}}</strong> {{.Text "comments"}}</li>
    </ul>
  </header>
  {{.Body}}
  <footer>
    <p>{{.Text "source"}} <a class="src" href="https://news.ycombinator.com"><svg class="yn" viewBox="0 0 16 16" aria-hidden="true"><rect width="16" height="16" rx="3" fill="#ff6600"/><text x="8" y="12.5" font-size="11" text-anchor="middle" fill="#ffffff" font-family="Helvetica, Arial" font-weight="bold">Y</text></svg>Hacker News</a></p>
  </footer>
</div>
</body>
</html>
`))
