package digest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Groq is the default backend: OpenAI-compatible, and its free tier covers the
// daily schedule many times over (this generates two digests a day).
//
// The previous default was GitHub Models, which authenticated with the
// workflow's built-in GITHUB_TOKEN and needed no secret at all. GitHub retired
// it on 2026-07-30 — the endpoint now answers 410 for everyone — so a key in
// HN_DIGEST_API_KEY is required from here on.
//
// gpt-oss-120b is the default: on Groq it is both cheaper per token and faster
// than the 70B Llama, with the headroom to summarise 30 headlines well.
// Override with HN_DIGEST_MODEL to use another.
const (
	defaultAPIURL = "https://api.groq.com/openai/v1"
	defaultModel  = "openai/gpt-oss-120b"
)

// LLM calls an OpenAI-compatible chat completions API.
//
// Any provider speaking that shape works — Groq, OpenRouter, Cerebras, DeepSeek,
// a local Ollama — by pointing APIURL at it. Nothing here is Groq-specific
// beyond the defaults.
type LLM struct {
	APIURL string
	APIKey string
	Model  string

	http *http.Client
}

// NewLLM builds a client, falling back to the Groq defaults for any empty field.
func NewLLM(apiURL, apiKey, model string) *LLM {
	if apiURL == "" {
		apiURL = defaultAPIURL
	}
	if model == "" {
		model = defaultModel
	}
	return &LLM{
		APIURL: strings.TrimRight(apiURL, "/"),
		APIKey: apiKey,
		Model:  model,
		// Summarising 30 stories runs long on slower free backends; well above
		// the 30s used elsewhere in this repo for single-shot calls.
		http: &http.Client{Timeout: 3 * time.Minute},
	}
}

// LLMFromEnv builds a client from the environment.
//
// Precedence matches the rest of this repo: explicit env vars win, then the
// defaults above. There is deliberately no GITHUB_TOKEN fallback — it only ever
// worked against GitHub Models, and with a third-party APIURL it would send the
// workflow's token to that provider as a bearer credential.
func LLMFromEnv() *LLM {
	return NewLLM(
		os.Getenv("HN_DIGEST_API_URL"),
		os.Getenv("HN_DIGEST_API_KEY"),
		os.Getenv("HN_DIGEST_MODEL"),
	)
}

// Configured reports whether the client can make a request.
func (l *LLM) Configured() bool {
	return l != nil && l.APIURL != "" && l.APIKey != "" && l.Model != ""
}

// Complete sends one chat completion and returns the message content.
func (l *LLM) Complete(ctx context.Context, system, user string) (string, error) {
	if !l.Configured() {
		return "", errors.New("llm is not configured: set HN_DIGEST_API_KEY")
	}

	body, err := json.Marshal(chatRequest{
		Model: l.Model,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, l.APIURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+l.APIKey)

	resp, err := l.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Include the body: free backends report quota and model-name errors
		// there, and the bare status alone makes those undiagnosable.
		var msg bytes.Buffer
		_, _ = msg.ReadFrom(io.LimitReader(resp.Body, 2<<10))
		return "", fmt.Errorf("llm request failed: %s: %s", resp.Status, strings.TrimSpace(msg.String()))
	}

	var out chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", errors.New("llm response has no choices")
	}

	content := strings.TrimSpace(out.Choices[0].Message.Content)
	if content == "" {
		return "", errors.New("llm response is empty")
	}
	return stripMarkdownFence(content), nil
}

// stripMarkdownFence removes a wrapping ```markdown fence that some models add
// around their output even when not asked.
func stripMarkdownFence(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	if !strings.HasSuffix(strings.TrimRight(s, "\n "), "```") {
		return s
	}
	if idx := strings.Index(s, "\n"); idx != -1 {
		s = s[idx+1:]
	}
	s = strings.TrimRight(s, "\n ")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}
