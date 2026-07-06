package translate

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func (c *Client) completeCodexResponses(ctx context.Context, reqBody chatCompletionRequest) (string, error) {
	req, err := newCodexResponsesRequest(reqBody, strings.TrimSpace(c.ReasoningEffort), strings.TrimSpace(c.ServiceTier))
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.APIURL+"/responses", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	httpReq.Header.Set("User-Agent", "codex_cli_rs/0.0.0 (hn-cli)")
	httpReq.Header.Set("originator", "codex_cli_rs")
	if c.ChatGPTAccountID != "" {
		httpReq.Header.Set("ChatGPT-Account-ID", c.ChatGPTAccountID)
	}

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", translateHTTPError(resp)
	}

	content, err := readCodexResponsesStream(resp.Body)
	if err != nil {
		return "", err
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return "", errors.New("translate response is empty")
	}
	return content, nil
}

type codexResponsesRequest struct {
	Model        string                   `json:"model"`
	Instructions string                   `json:"instructions,omitempty"`
	Input        []codexResponsesInput    `json:"input"`
	Store        bool                     `json:"store"`
	Stream       bool                     `json:"stream"`
	Include      []string                 `json:"include"`
	Reasoning    *codexResponsesReasoning `json:"reasoning,omitempty"`
	ServiceTier  string                   `json:"service_tier,omitempty"`
}

type codexResponsesReasoning struct {
	Effort  string `json:"effort,omitempty"`
	Summary string `json:"summary,omitempty"`
}

type codexResponsesInput struct {
	Role    string                  `json:"role"`
	Content []codexResponsesContent `json:"content"`
}

type codexResponsesContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type codexStreamEvent struct {
	Type     string               `json:"type"`
	Delta    string               `json:"delta"`
	Item     *codexResponseItem   `json:"item"`
	Response *codexStreamResponse `json:"response"`
	Error    *codexStreamError    `json:"error"`
}

type codexResponseItem struct {
	Content []codexResponsesContent `json:"content"`
}

type codexStreamResponse struct {
	Status            string            `json:"status"`
	Error             *codexStreamError `json:"error"`
	IncompleteDetails any               `json:"incomplete_details"`
}

type codexStreamError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

func newCodexResponsesRequest(reqBody chatCompletionRequest, reasoningEffort, serviceTier string) (codexResponsesRequest, error) {
	var instructions []string
	var input []codexResponsesInput
	for _, msg := range reqBody.Messages {
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		if msg.Role == "system" {
			instructions = append(instructions, content)
			continue
		}
		role := msg.Role
		if role == "" {
			role = "user"
		}
		input = append(input, codexResponsesInput{
			Role: role,
			Content: []codexResponsesContent{{
				Type: "input_text",
				Text: content,
			}},
		})
	}
	if len(input) == 0 {
		return codexResponsesRequest{}, errors.New("translate request has no input")
	}

	req := codexResponsesRequest{
		Model:        reqBody.Model,
		Instructions: strings.Join(instructions, "\n\n"),
		Input:        input,
		Store:        false,
		Stream:       true,
		Include:      []string{},
		ServiceTier:  serviceTier,
	}
	if reasoningEffort != "" {
		req.Reasoning = &codexResponsesReasoning{
			Effort:  reasoningEffort,
			Summary: "auto",
		}
	}
	return req, nil
}

func readCodexResponsesStream(r io.Reader) (string, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var event string
	var data []string
	var deltas strings.Builder
	var fallback strings.Builder
	var streamErr error

	flush := func() {
		if len(data) == 0 || streamErr != nil {
			data = nil
			event = ""
			return
		}
		raw := strings.TrimSpace(strings.Join(data, "\n"))
		data = nil
		if raw == "" || raw == "[DONE]" {
			event = ""
			return
		}
		var ev codexStreamEvent
		if err := json.Unmarshal([]byte(raw), &ev); err != nil {
			event = ""
			return
		}
		eventType := ev.Type
		if eventType == "" {
			eventType = event
		}
		switch eventType {
		case "response.output_text.delta":
			deltas.WriteString(ev.Delta)
		case "response.output_item.done":
			if ev.Item != nil {
				for _, part := range ev.Item.Content {
					if part.Text != "" {
						fallback.WriteString(part.Text)
					}
				}
			}
		case "response.failed":
			streamErr = codexStreamFailure("response failed", &ev)
		case "response.incomplete":
			if deltas.Len() == 0 && fallback.Len() == 0 {
				streamErr = codexStreamFailure("response incomplete", &ev)
			}
		}
		event = ""
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			flush()
			continue
		}
		if strings.HasPrefix(line, "event:") {
			event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if strings.HasPrefix(line, "data:") {
			data = append(data, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	flush()
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if streamErr != nil {
		return "", streamErr
	}
	if deltas.Len() > 0 {
		return deltas.String(), nil
	}
	return fallback.String(), nil
}

func codexStreamFailure(prefix string, ev *codexStreamEvent) error {
	if ev == nil {
		return errors.New(prefix)
	}
	if ev.Error != nil && ev.Error.Message != "" {
		return fmt.Errorf("%s: %s", prefix, ev.Error.Message)
	}
	if ev.Response != nil {
		if ev.Response.Error != nil && ev.Response.Error.Message != "" {
			return fmt.Errorf("%s: %s", prefix, ev.Response.Error.Message)
		}
		if ev.Response.IncompleteDetails != nil {
			return fmt.Errorf("%s: %v", prefix, ev.Response.IncompleteDetails)
		}
		if ev.Response.Status != "" {
			return fmt.Errorf("%s: %s", prefix, ev.Response.Status)
		}
	}
	return errors.New(prefix)
}
