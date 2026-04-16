package translate

import (
	"context"
	"encoding/json"
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
