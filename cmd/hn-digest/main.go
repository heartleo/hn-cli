// Command hn-digest generates the static HN Digest site.
//
// It fetches the Hacker News front page, and — unless the same stories were
// already rendered — asks an LLM to summarise it, writing one HTML page per
// language. The output is committed to the repo, which is what GitHub Pages
// serves and what keeps the hourly schedule alive.
//
// Backend defaults to GitHub Models (free; authenticates with GITHUB_TOKEN).
// Point HN_DIGEST_API_URL at any OpenAI-compatible endpoint to use another.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/heartleo/hn-cli/internal/digest"
)

// state is what the previous run left behind, so this run can tell whether the
// front page actually changed.
type state struct {
	Fingerprint string    `json:"fingerprint"`
	GeneratedAt time.Time `json:"generated_at"`
	Model       string    `json:"model"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "hn-digest: "+err.Error())
		os.Exit(1)
	}
}

func run() error {
	outDir := flag.String("out", "docs", "output directory")
	force := flag.Bool("force", false, "regenerate even if the front page has not changed")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	stories, err := digest.FetchFrontPage(ctx, nil)
	if err != nil {
		return err
	}

	fingerprint := digest.Fingerprint(stories)
	statePath := filepath.Join(*outDir, "state.json")

	if !*force {
		if prev, err := readState(statePath); err == nil && prev.Fingerprint == fingerprint {
			// The same stories are on the page. Regenerating would spend a model
			// call to produce a near-identical digest, so stop here — writing
			// nothing also means no commit, which is the point of the skip.
			fmt.Printf("front page unchanged since %s, skipping\n",
				prev.GeneratedAt.UTC().Format(time.RFC3339))
			return nil
		}
	}

	llm := digest.LLMFromEnv()
	if !llm.Configured() {
		return fmt.Errorf("no API key: set HN_DIGEST_API_KEY, or GITHUB_TOKEN with `permissions: models: read`")
	}

	stats := digest.Summarize(stories)
	now := time.Now().UTC()

	fmt.Printf("front page changed (%d stories, top %d pts), generating with %s\n",
		stats.Count, stats.TopPoints, llm.Model)

	// Generate every language before writing anything. A half-written site —
	// Chinese updated, English stale — is worse than an untouched one.
	pages := make(map[digest.Lang][]byte, len(digest.Langs))
	for _, lang := range digest.Langs {
		html, err := generate(ctx, llm, stories, stats, lang, now)
		if err != nil {
			return fmt.Errorf("%s: %w", lang, err)
		}
		pages[lang] = html
		fmt.Printf("  %s ok (%d bytes)\n", lang, len(html))
	}

	for lang, html := range pages {
		path := filepath.Join(*outDir, lang.Dir(), "index.html")
		if err := writeFile(path, html); err != nil {
			return err
		}
		fmt.Printf("  wrote %s\n", path)
	}

	if err := writeState(statePath, state{
		Fingerprint: fingerprint,
		GeneratedAt: now,
		Model:       llm.Model,
	}); err != nil {
		return err
	}

	fmt.Println("done")
	return nil
}

func generate(ctx context.Context, llm *digest.LLM, stories []digest.Story, stats digest.Stats, lang digest.Lang, now time.Time) ([]byte, error) {
	system, user, err := digest.Prompt(stories, lang)
	if err != nil {
		return nil, err
	}

	markdown, err := llm.Complete(ctx, system, user)
	if err != nil {
		return nil, err
	}

	return digest.Render(markdown, lang, stats, now)
}

func readState(path string) (state, error) {
	var s state
	data, err := os.ReadFile(path)
	if err != nil {
		return s, err
	}
	return s, json.Unmarshal(data, &s)
}

func writeState(path string, s state) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(path, append(data, '\n'))
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
