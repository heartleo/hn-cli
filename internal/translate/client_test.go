package translate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTranslateCallsChatCompletions(t *testing.T) {
	var gotPath string
	var gotAuth string
	var gotReq chatCompletionRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ni hao"}}]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", "test-model", "Chinese")
	got, err := client.Translate(context.Background(), "hello")
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if got != "ni hao" {
		t.Fatalf("expected translated text, got %q", got)
	}
	if gotPath != "/chat/completions" {
		t.Fatalf("expected /chat/completions path, got %q", gotPath)
	}
	if gotAuth != "Bearer test-key" {
		t.Fatalf("unexpected authorization header: %q", gotAuth)
	}
	if gotReq.Model != "test-model" || len(gotReq.Messages) != 2 || gotReq.Messages[1].Content != "hello" {
		t.Fatalf("unexpected request body: %#v", gotReq)
	}
}

func TestTranslateCallsCodexResponsesStream(t *testing.T) {
	var gotPath string
	var gotAuth string
	var gotOriginator string
	var gotAccountID string
	var gotReq codexResponsesRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotOriginator = r.Header.Get("originator")
		gotAccountID = r.Header.Get("ChatGPT-Account-ID")
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, "event: response.output_text.delta\n")
		_, _ = fmt.Fprint(w, `data: {"type":"response.output_text.delta","delta":"你"}`+"\n\n")
		_, _ = fmt.Fprint(w, "event: response.output_text.delta\n")
		_, _ = fmt.Fprint(w, `data: {"type":"response.output_text.delta","delta":"好"}`+"\n\n")
		_, _ = fmt.Fprint(w, "event: response.completed\n")
		_, _ = fmt.Fprint(w, `data: {"type":"response.completed","response":{"status":"completed"}}`+"\n\n")
	}))
	defer server.Close()

	client := NewClient(
		server.URL,
		"oauth-token",
		"gpt-5.5",
		"Chinese",
		WithWireAPI("codex_responses"),
		WithReasoningEffort("none"),
		WithServiceTier("priority"),
		WithChatGPTAccountID("account-123"),
	)
	got, err := client.Translate(context.Background(), "hello")
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if got != "你好" {
		t.Fatalf("expected translated text, got %q", got)
	}
	if gotPath != "/responses" {
		t.Fatalf("expected /responses path, got %q", gotPath)
	}
	if gotAuth != "Bearer oauth-token" {
		t.Fatalf("unexpected authorization header: %q", gotAuth)
	}
	if gotOriginator != "codex_cli_rs" {
		t.Fatalf("unexpected originator header: %q", gotOriginator)
	}
	if gotAccountID != "account-123" {
		t.Fatalf("unexpected account header: %q", gotAccountID)
	}
	if gotReq.Model != "gpt-5.5" || !gotReq.Stream || gotReq.Store {
		t.Fatalf("unexpected Codex request body: %#v", gotReq)
	}
	if gotReq.Reasoning == nil || gotReq.Reasoning.Effort != "none" {
		t.Fatalf("expected reasoning effort none, got %#v", gotReq.Reasoning)
	}
	if gotReq.ServiceTier != "priority" {
		t.Fatalf("expected service_tier=priority, got %q", gotReq.ServiceTier)
	}
	if len(gotReq.Input) != 1 || gotReq.Input[0].Content[0].Text != "hello" {
		t.Fatalf("unexpected Codex input: %#v", gotReq.Input)
	}
}

func TestTranslateBatchCallsChatCompletionsOnce(t *testing.T) {
	calls := 0
	var gotReq chatCompletionRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("expected /chat/completions path, got %q", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"{\"1\":\"one translated\",\"2\":\"two translated\"}"}}]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", "test-model", "Chinese")
	got, err := client.TranslateBatch(context.Background(), map[int]string{
		1: "one",
		2: "two",
	})
	if err != nil {
		t.Fatalf("translate batch: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected one API request, got %d", calls)
	}
	if got[1] != "one translated" || got[2] != "two translated" {
		t.Fatalf("unexpected translations: %#v", got)
	}
	if gotReq.Model != "test-model" || len(gotReq.Messages) != 2 {
		t.Fatalf("unexpected request body: %#v", gotReq)
	}
}

func TestTranslateSendsReasoningEffortAndServiceTier(t *testing.T) {
	var gotReq chatCompletionRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"你好"}}]}`))
	}))
	defer server.Close()

	client := NewClient(
		server.URL,
		"oauth-token",
		"gpt-5.5",
		"Chinese",
		WithReasoningEffort("minimal"),
		WithServiceTier("fast"),
	)
	if _, err := client.Translate(context.Background(), "hello"); err != nil {
		t.Fatalf("translate: %v", err)
	}
	if gotReq.ReasoningEffort != "minimal" {
		t.Fatalf("expected reasoning_effort=minimal, got %q", gotReq.ReasoningEffort)
	}
	if gotReq.ServiceTier != "fast" {
		t.Fatalf("expected service_tier=fast, got %q", gotReq.ServiceTier)
	}
}

func TestExtractHTTPErrorMessage(t *testing.T) {
	if got := extractHTTPErrorMessage(`{"detail":"Stream must be set to true"}`); got != "Stream must be set to true" {
		t.Fatalf("unexpected detail message: %q", got)
	}
	if got := extractHTTPErrorMessage(`{"error":{"message":"billing_not_active"}}`); got != "billing_not_active" {
		t.Fatalf("unexpected error message: %q", got)
	}
}

func TestTranslateMarkdownSendsAndReturnsMarkdown(t *testing.T) {
	var gotReq chatCompletionRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_, _ = w.Write([]byte("{\"choices\":[{\"message\":{\"role\":\"assistant\",\"content\":\"```markdown\\n**你好** [链接](https://example.com)\\n```\"}}]}"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", "test-model", "Chinese")
	got, err := client.TranslateMarkdown(context.Background(), "**hello** [link](https://example.com)")
	if err != nil {
		t.Fatalf("translate markdown: %v", err)
	}
	if got != "**你好** [链接](https://example.com)" {
		t.Fatalf("expected markdown fence to be stripped, got %q", got)
	}
	if gotReq.Messages[1].Content != "**hello** [link](https://example.com)" {
		t.Fatalf("expected markdown payload, got %#v", gotReq.Messages)
	}
}

func TestTranslateMarkdownRejectsUnclosedFence(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("{\"choices\":[{\"message\":{\"role\":\"assistant\",\"content\":\"```go\\nfmt.Println(1)\"}}]}"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", "test-model", "Chinese")
	if _, err := client.TranslateMarkdown(context.Background(), "hello"); err == nil {
		t.Fatal("expected invalid markdown error")
	}
}
