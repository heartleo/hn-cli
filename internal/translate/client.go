package translate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yuin/goldmark"
)

// Client translates text through an OpenAI-compatible chat completions API.
type Client struct {
	APIURL           string
	APIKey           string
	Model            string
	Language         string
	WireAPI          string
	ReasoningEffort  string
	ServiceTier      string
	ChatGPTAccountID string

	http *http.Client
}

// ClientOption configures a translation client.
type ClientOption func(*Client)

// WithReasoningEffort sets the reasoning effort request parameter when non-empty.
func WithReasoningEffort(effort string) ClientOption {
	return func(c *Client) {
		c.ReasoningEffort = strings.TrimSpace(effort)
	}
}

// WithServiceTier sets the service tier request parameter when non-empty.
func WithServiceTier(tier string) ClientOption {
	return func(c *Client) {
		c.ServiceTier = strings.TrimSpace(tier)
	}
}

// WithWireAPI chooses the request wire format when non-empty.
func WithWireAPI(wireAPI string) ClientOption {
	return func(c *Client) {
		c.WireAPI = strings.TrimSpace(wireAPI)
	}
}

// WithChatGPTAccountID sets the Codex ChatGPT account header when non-empty.
func WithChatGPTAccountID(accountID string) ClientOption {
	return func(c *Client) {
		c.ChatGPTAccountID = strings.TrimSpace(accountID)
	}
}

// NewClient creates a translation client from config values.
func NewClient(apiURL, apiKey, model, language string, opts ...ClientOption) *Client {
	c := &Client{
		APIURL:   strings.TrimRight(apiURL, "/"),
		APIKey:   apiKey,
		Model:    model,
		Language: language,
		http:     &http.Client{Timeout: 30 * time.Second},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	return c
}

// Configured reports whether the client has enough settings to call the API.
func (c *Client) Configured() bool {
	return c != nil && c.APIURL != "" && c.APIKey != "" && c.Model != "" && c.Language != ""
}

// Translate translates text and returns only the translated content.
func (c *Client) Translate(ctx context.Context, text string) (string, error) {
	if !c.Configured() {
		return "", errors.New("translation is not configured")
	}
	if strings.TrimSpace(text) == "" {
		return "", nil
	}

	reqBody := chatCompletionRequest{
		Model: c.Model,
		Messages: []chatMessage{
			{
				Role:    "system",
				Content: fmt.Sprintf("Translate the following text to %s. Output only the translation, no explanation.", c.Language),
			},
			{Role: "user", Content: text},
		},
	}

	translated, err := c.complete(ctx, reqBody)
	if err != nil {
		return "", err
	}
	if translated == "" {
		return "", errors.New("translate response is empty")
	}
	return translated, nil
}

// TranslateBatch translates titles in a single API request.
func (c *Client) TranslateBatch(ctx context.Context, titles map[int]string) (map[int]string, error) {
	if !c.Configured() {
		return nil, errors.New("translation is not configured")
	}
	if len(titles) == 0 {
		return map[int]string{}, nil
	}

	items := make([]batchTitle, 0, len(titles))
	for id, title := range titles {
		if strings.TrimSpace(title) != "" {
			items = append(items, batchTitle{ID: id, Title: title})
		}
	}
	if len(items) == 0 {
		return map[int]string{}, nil
	}

	input, err := json.Marshal(items)
	if err != nil {
		return nil, err
	}

	reqBody := chatCompletionRequest{
		Model: c.Model,
		Messages: []chatMessage{
			{
				Role: "system",
				Content: fmt.Sprintf(
					"Translate each Hacker News title to %s. Return only valid JSON: an object whose keys are the input ids and whose values are translated titles. Do not include markdown or explanations.",
					c.Language,
				),
			},
			{Role: "user", Content: string(input)},
		},
	}

	content, err := c.complete(ctx, reqBody)
	if err != nil {
		return nil, err
	}

	var translated map[int]string
	if err := json.Unmarshal([]byte(stripJSONFence(content)), &translated); err != nil {
		return nil, fmt.Errorf("decode batch translation response: %w", err)
	}
	for id, value := range translated {
		if strings.TrimSpace(value) == "" {
			delete(translated, id)
		}
	}
	return translated, nil
}

func (c *Client) complete(ctx context.Context, reqBody chatCompletionRequest) (string, error) {
	if strings.EqualFold(strings.TrimSpace(c.WireAPI), "codex_responses") {
		return c.completeCodexResponses(ctx, reqBody)
	}
	return c.completeChatCompletions(ctx, reqBody)
}

func (c *Client) completeChatCompletions(ctx context.Context, reqBody chatCompletionRequest) (string, error) {
	reqBody.ReasoningEffort = strings.TrimSpace(c.ReasoningEffort)
	reqBody.ServiceTier = strings.TrimSpace(c.ServiceTier)

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.APIURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", translateHTTPError(resp)
	}

	var out chatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", errors.New("translate response has no choices")
	}

	content := strings.TrimSpace(out.Choices[0].Message.Content)
	if content == "" {
		return "", errors.New("translate response is empty")
	}
	return content, nil
}

// TranslateMarkdown translates markdown content, instructing the model to preserve
// markdown formatting. Returns translated markdown, stripping any code fence the
// model may have added around the output.
func (c *Client) TranslateMarkdown(ctx context.Context, markdown string) (string, error) {
	if !c.Configured() {
		return "", errors.New("translation is not configured")
	}
	if strings.TrimSpace(markdown) == "" {
		return "", nil
	}

	reqBody := chatCompletionRequest{
		Model: c.Model,
		Messages: []chatMessage{
			{
				Role: "system",
				Content: fmt.Sprintf(
					"Translate the following markdown text to %s. "+
						"Preserve all markdown formatting (bold, italic, code blocks, blockquotes, links, lists). "+
						"Output only the translated markdown, no explanation.",
					c.Language,
				),
			},
			{Role: "user", Content: markdown},
		},
	}

	raw, err := c.complete(ctx, reqBody)
	if err != nil {
		return "", err
	}

	translated := stripMarkdownFence(raw)
	if strings.TrimSpace(translated) == "" {
		return "", errors.New("translate response is empty after cleanup")
	}
	if err := validateMarkdown(translated); err != nil {
		return "", err
	}
	return translated, nil
}

func validateMarkdown(markdown string) error {
	if strings.TrimSpace(markdown) == "" {
		return errors.New("markdown is empty")
	}
	if hasUnclosedFence(markdown) {
		return errors.New("markdown response has an unclosed code fence")
	}
	var buf bytes.Buffer
	if err := goldmark.Convert([]byte(markdown), &buf); err != nil {
		return fmt.Errorf("markdown response is invalid: %w", err)
	}
	return nil
}

func hasUnclosedFence(markdown string) bool {
	openFence := ""
	for _, line := range strings.Split(markdown, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if openFence == "```" {
				openFence = ""
			} else if openFence == "" {
				openFence = "```"
			}
			continue
		}
		if strings.HasPrefix(trimmed, "~~~") {
			if openFence == "~~~" {
				openFence = ""
			} else if openFence == "" {
				openFence = "~~~"
			}
		}
	}
	return openFence != ""
}

// stripMarkdownFence removes a wrapping markdown code fence (```markdown ... ```)
// that some models add around their output even when not asked.
func stripMarkdownFence(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	if !strings.HasSuffix(strings.TrimRight(s, "\n "), "```") {
		return s
	}
	// Drop the opening fence line (```markdown or just ```)
	if idx := strings.Index(s, "\n"); idx != -1 {
		s = s[idx+1:]
	}
	// Drop the closing fence
	s = strings.TrimRight(s, "\n ")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

func stripJSONFence(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

type chatCompletionRequest struct {
	Model           string        `json:"model"`
	Messages        []chatMessage `json:"messages"`
	ReasoningEffort string        `json:"reasoning_effort,omitempty"`
	ServiceTier     string        `json:"service_tier,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

type batchTitle struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

func translateHTTPError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	message := strings.TrimSpace(string(body))
	if message == "" {
		return fmt.Errorf("translate request failed: %s", resp.Status)
	}
	if extracted := extractHTTPErrorMessage(message); extracted != "" {
		message = extracted
	}
	return fmt.Errorf("translate request failed: %s: %s", resp.Status, message)
}

func extractHTTPErrorMessage(body string) string {
	var payload struct {
		Detail string `json:"detail"`
		Error  struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return ""
	}
	if strings.TrimSpace(payload.Error.Message) != "" {
		return strings.TrimSpace(payload.Error.Message)
	}
	return strings.TrimSpace(payload.Detail)
}
