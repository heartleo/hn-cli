package digest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testStories() []Story {
	at := time.Date(2026, 7, 16, 3, 51, 25, 0, time.UTC)
	return []Story{
		{Title: "Inkling: Our Open-Weights Model", Points: 837, NumComments: 212,
			URL: "https://thinkingmachines.ai/news/introducing-inkling/", ObjectID: "48924912", CreatedAt: at},
		{Title: "Show HN: One More Letter", Points: 59, NumComments: 38,
			URL: "", ObjectID: "48928402", CreatedAt: at.Add(-time.Hour)},
	}
}

func TestDomain(t *testing.T) {
	stories := testStories()

	// HN shows the source beside every title, and it is what a reader judges a
	// link by before clicking.
	if got := stories[0].Domain(); got != "thinkingmachines.ai" {
		t.Errorf("Domain() = %q, want thinkingmachines.ai", got)
	}
	// A self-post has no source but HN itself.
	if got := stories[1].Domain(); got != "" {
		t.Errorf("self-post Domain() = %q, want empty", got)
	}

	for _, tc := range []struct{ url, want string }{
		{"https://www.reuters.com/business/x", "reuters.com"}, // www. is noise
		{"https://GitHub.com/xai-org/grok", "github.com"},     // host is case-insensitive
		{"https://blog.mozilla.org/a/b", "blog.mozilla.org"},  // subdomains carry meaning
		{"not a url", ""},
	} {
		if got := (Story{URL: tc.url}).Domain(); got != tc.want {
			t.Errorf("Domain(%q) = %q, want %q", tc.url, got, tc.want)
		}
	}
}

func TestPromptCarriesDomain(t *testing.T) {
	_, user, err := Prompt(testStories(), LangEN)
	if err != nil {
		t.Fatal(err)
	}
	// Computed here, not left to the model: URL parsing has one right answer,
	// and an invented host would look just as plausible as a correct one.
	if !strings.Contains(user, `"domain": "thinkingmachines.ai"`) {
		t.Error("prompt does not carry the pre-computed domain")
	}
}

func TestArticleURLFallsBackToDiscussionForSelfPosts(t *testing.T) {
	stories := testStories()

	if got := stories[0].ArticleURL(); got != "https://thinkingmachines.ai/news/introducing-inkling/" {
		t.Errorf("link post: got %q, want the article url", got)
	}
	// A self-post has no url; emitting a bare title would break the rule that
	// every story on the page is clickable.
	want := "https://news.ycombinator.com/item?id=48928402"
	if got := stories[1].ArticleURL(); got != want {
		t.Errorf("self-post: got %q, want %q", got, want)
	}
}

func TestFingerprintIgnoresScoresAndOrder(t *testing.T) {
	base := testStories()
	fp := Fingerprint(base)

	// Scores tick constantly. If they fed the fingerprint, every scheduled run
	// would look like a change and the skip would never fire.
	bumped := testStories()
	bumped[0].Points += 40
	bumped[1].NumComments += 7
	if got := Fingerprint(bumped); got != fp {
		t.Error("fingerprint changed when only scores moved; the unchanged skip would never fire")
	}

	reordered := []Story{base[1], base[0]}
	if got := Fingerprint(reordered); got != fp {
		t.Error("fingerprint changed when only rank moved")
	}

	swapped := testStories()
	swapped[0].ObjectID = "99999999"
	if got := Fingerprint(swapped); got == fp {
		t.Error("fingerprint unchanged when a different story is on the page")
	}
}

func TestSummarize(t *testing.T) {
	got := Summarize(testStories())

	if got.Count != 2 {
		t.Errorf("Count = %d, want 2", got.Count)
	}
	if got.TopPoints != 837 {
		t.Errorf("TopPoints = %d, want 837", got.TopPoints)
	}
	if got.TotalComments != 250 {
		t.Errorf("TotalComments = %d, want 250", got.TotalComments)
	}
	if want := time.Date(2026, 7, 16, 3, 51, 25, 0, time.UTC); !got.Newest.Equal(want) {
		t.Errorf("Newest = %v, want %v", got.Newest, want)
	}
}

func TestFetchFrontPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hits":[
			{"title":"A","points":100,"num_comments":10,"url":"https://a.example","objectID":"1","created_at":"2026-07-16T03:51:25Z"},
			{"title":"B","points":50,"num_comments":5,"url":null,"objectID":"2","created_at":"2026-07-16T01:00:00Z"}
		]}`))
	}))
	defer srv.Close()

	stories, err := fetchFrontPage(context.Background(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if len(stories) != 2 {
		t.Fatalf("got %d stories, want 2", len(stories))
	}
	// Algolia sends null for self-posts; it must land as an empty string, not
	// the literal "null".
	if stories[1].URL != "" {
		t.Errorf("self-post URL = %q, want empty", stories[1].URL)
	}
	if stories[0].CreatedAt.IsZero() {
		t.Error("created_at did not decode")
	}
}

func TestFetchFrontPageRejectsEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"hits":[]}`))
	}))
	defer srv.Close()

	// An empty front page means something upstream broke. Returning no error
	// would let the run publish a digest of nothing.
	if _, err := fetchFrontPage(context.Background(), srv.Client(), srv.URL); err == nil {
		t.Error("want an error when the front page comes back empty")
	}
}

func TestFetchFrontPageRejectsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	if _, err := fetchFrontPage(context.Background(), srv.Client(), srv.URL); err == nil {
		t.Error("want an error on a 503")
	}
}

func TestPromptCarriesDiscussionURLAndStoryCount(t *testing.T) {
	system, user, err := Prompt(testStories(), LangZH)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(system, "Simplified Chinese") {
		t.Error("system prompt does not name the output language")
	}
	// The page has no asker, so the count the model must sum the bars to has to
	// be stated explicitly.
	if !strings.Contains(system, "sum to 2") {
		t.Error("system prompt does not pin the story count for the bar chart")
	}
	if !strings.Contains(user, "https://news.ycombinator.com/item?id=48928402") {
		t.Error("user prompt does not carry the resolved discussion url")
	}
	if strings.Contains(user, "_highlightResult") {
		t.Error("user prompt leaks Algolia highlight noise")
	}

	systemEN, _, err := Prompt(testStories(), LangEN)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(systemEN, "English") {
		t.Error("English prompt does not name the output language")
	}
}

func TestLLMDefaultsToFreeGroq(t *testing.T) {
	llm := NewLLM("", "tok", "")

	if llm.APIURL != "https://api.groq.com/openai/v1" {
		t.Errorf("APIURL = %q, want the Groq endpoint", llm.APIURL)
	}
	if llm.Model != "openai/gpt-oss-120b" {
		t.Errorf("Model = %q, want openai/gpt-oss-120b", llm.Model)
	}
	if !llm.Configured() {
		t.Error("a client with a key should be configured")
	}
	if NewLLM("", "", "").Configured() {
		t.Error("a client without a key must not report configured")
	}

	// Any OpenAI-compatible endpoint must override cleanly, trailing slash included.
	custom := NewLLM("https://openrouter.ai/api/v1/", "k", "deepseek/deepseek-chat")
	if custom.APIURL != "https://openrouter.ai/api/v1" {
		t.Errorf("APIURL = %q, want the trailing slash trimmed", custom.APIURL)
	}
}

// GITHUB_TOKEN used to stand in for a missing key, back when the default
// backend was GitHub Models. That fallback is gone: with a third-party APIURL
// it would hand the workflow's token to that provider.
func TestLLMFromEnvIgnoresGitHubToken(t *testing.T) {
	t.Setenv("HN_DIGEST_API_KEY", "")
	t.Setenv("GITHUB_TOKEN", "ghs_secret")

	if llm := LLMFromEnv(); llm.Configured() {
		t.Errorf("APIKey = %q, want no key without HN_DIGEST_API_KEY", llm.APIKey)
	}
}

func TestLLMComplete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("Authorization = %q, want Bearer tok", got)
		}
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %q, want /chat/completions", r.URL.Path)
		}
		// Models sometimes wrap markdown output in a fence even when not asked.
		_, _ = w.Write([]byte("{\"choices\":[{\"message\":{\"content\":\"```markdown\\n## 1 · Overview\\n\\nhi\\n```\"}}]}"))
	}))
	defer srv.Close()

	llm := NewLLM(srv.URL, "tok", "m")
	got, err := llm.Complete(context.Background(), "sys", "usr")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "```") {
		t.Errorf("fence not stripped: %q", got)
	}
	if !strings.HasPrefix(got, "## 1 · Overview") {
		t.Errorf("got %q, want the markdown body", got)
	}
}

func TestLLMCompleteSurfacesErrorBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"rate limit exceeded"}}`))
	}))
	defer srv.Close()

	_, err := NewLLM(srv.URL, "tok", "m").Complete(context.Background(), "s", "u")
	if err == nil {
		t.Fatal("want an error on 429")
	}
	// Free backends report quota problems in the body; a bare status is undiagnosable.
	if !strings.Contains(err.Error(), "rate limit exceeded") {
		t.Errorf("error does not carry the response body: %v", err)
	}
}

func TestRenderNeutralizesRawHTMLFromUntrustedTitles(t *testing.T) {
	// HN titles are submitted by anyone and reach this markdown by way of the
	// model, so the markdown itself is untrusted. goldmark runs without
	// html.WithUnsafe, which replaces raw HTML with a comment rather than
	// emitting it.
	md := "## 1 · Overview\n\nA title: <img src=x onerror=alert(1)>\n\n<script>alert(2)</script>\n"

	out, err := Render(md, LangEN, Stats{Count: 30, TopPoints: 837, TotalComments: 1836}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	html := string(out)

	for _, payload := range []string{"<img src=x", "onerror=alert", "<script>alert(2)"} {
		if strings.Contains(html, payload) {
			t.Errorf("raw HTML reached the page: %q", payload)
		}
	}
	if !strings.Contains(html, "raw HTML omitted") {
		t.Error("expected goldmark to omit the raw HTML; the safe-mode guarantee may have regressed")
	}
}

func TestRenderPage(t *testing.T) {
	md := "## 1 · Overview\n\nToday.\n\n## 3 · Industry mix\n\n```\nDeveloper tools ██ 2\n```\n"
	stats := Stats{Count: 30, TopPoints: 837, TotalComments: 1836}

	zh, err := Render(md, LangZH, stats, time.Date(2026, 7, 16, 4, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	got := string(zh)

	for _, want := range []string{
		`lang="zh-Hans"`,
		`href="/en/"`, // cross-link to the other language
		"1,836",       // thousands separator
		"#d43713",     // brand red — 4.82:1 on white, the one brand colour that may be text
		"#ffffff",     // white ground, as the digest uses
		"#6b6b6b",     // metadata grey, 5.33:1
		"HN Digest",   // wordmark
		// The build time left the visible footer, but the page rewrites itself
		// daily — without it a reader on a cached copy cannot tell its age.
		`<meta name="date" content="2026-07-16T04:00:00Z">`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("zh page missing %q", want)
		}
	}
	// Sections carry their own weight; a "1 ·" in front of a 30px heading is noise.
	if strings.Contains(got, "font-size: 13px;\n  font-weight: 700;\n  letter-spacing: 0.08em") {
		t.Error("section headings reverted to the small uppercase label treatment")
	}
	// #e88b2b is 2.57:1 and #ff6600 is 2.94:1 on white — neither clears even the
	// 3:1 large-text bar. The orange may only appear as the wordmark gradient's
	// far end, never as a colour some text is set in.
	if strings.Contains(got, "color: #e88b2b") || strings.Contains(got, "color: #ff6600") {
		t.Error("an orange that fails contrast on white is being used as a text colour")
	}
	// 16px floor: prose, and below 16px iOS auto-zooms the page on focus.
	if !strings.Contains(got, "font-size: 16px") {
		t.Error("body is not 16px; mobile body text must not drop below the 16px floor")
	}
	// The language toggle is the page's only persistent control.
	if !strings.Contains(got, "line-height: 44px") {
		t.Error("the language toggle lost its 44px touch target")
	}
	// Both halves of the site are served by system faces. A webfont could only
	// style the Latin half — no Latin face carries CJK — so it would cost every
	// reader a download and a swap to style half a page.
	for _, want := range []string{`"Helvetica Neue"`, `"PingFang SC"`, `"Microsoft YaHei"`} {
		if !strings.Contains(got, want) {
			t.Errorf("font stack missing %s", want)
		}
	}
	for _, banned := range []string{"fonts.googleapis.com", "@font-face", "@import"} {
		if strings.Contains(got, banned) {
			t.Errorf("page loads a webfont (%s); it could only style the Latin half", banned)
		}
	}

	en, err := Render(md, LangEN, stats, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(en), `lang="en"`) {
		t.Error("en page has the wrong lang attribute")
	}
	if !strings.Contains(string(en), `href="/"`) {
		t.Error("en page does not link back to the Chinese page")
	}
}

func TestRenderSeparatesItemLineFromItsReason(t *testing.T) {
	// A soft line break collapses to a space in HTML, so the reason would run
	// into the title. The blank line is what keeps them apart — the prompt
	// mandates it, and this pins the rendering that mandate exists for.
	md := "## 2 · Hot\n\n1. **[Title](https://a.example)** · `837 pts`\n\n   Why it matters.\n"

	out, err := Render(md, LangEN, Stats{}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)

	if !strings.Contains(got, "<p>Why it matters.</p>") {
		t.Error("reason did not render as its own paragraph; it will run into the title")
	}
}

func TestLangDir(t *testing.T) {
	if got := LangZH.Dir(); got != "" {
		t.Errorf("zh dir = %q, want the site root", got)
	}
	if got := LangEN.Dir(); got != "en" {
		t.Errorf("en dir = %q, want en", got)
	}
}

func TestComma(t *testing.T) {
	p := Page{}
	for _, tc := range []struct {
		in   int
		want string
	}{{0, "0"}, {999, "999"}, {1000, "1,000"}, {1836, "1,836"}, {1234567, "1,234,567"}} {
		if got := p.Comma(tc.in); got != tc.want {
			t.Errorf("Comma(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
