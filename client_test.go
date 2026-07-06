package hn

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestGetItemFreshRetriesTransportError(t *testing.T) {
	oldDelay := itemFetchRetryBaseDelay
	itemFetchRetryBaseDelay = 0
	defer func() { itemFetchRetryBaseDelay = oldDelay }()

	attempts := 0
	client := NewClient()
	client.http = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			attempts++
			if attempts == 1 {
				return nil, io.EOF
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"id":48796093,"type":"story","title":"ok"}`)),
				Request:    req,
			}, nil
		}),
		Timeout: 30 * time.Second,
	}

	item, err := client.GetItemFresh(48796093)
	if err != nil {
		t.Fatalf("GetItemFresh returned error: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
	if item.ID != 48796093 || item.Type != "story" || item.Title != "ok" {
		t.Fatalf("item = %+v, want decoded story", item)
	}
}

func TestGetItemFreshStopsAfterRetryLimit(t *testing.T) {
	oldDelay := itemFetchRetryBaseDelay
	itemFetchRetryBaseDelay = 0
	defer func() { itemFetchRetryBaseDelay = oldDelay }()

	attempts := 0
	client := NewClient()
	client.http = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			attempts++
			return nil, io.EOF
		}),
		Timeout: 30 * time.Second,
	}

	_, err := client.GetItemFresh(48796093)
	if err == nil {
		t.Fatal("GetItemFresh returned nil error")
	}
	if attempts != itemFetchAttempts {
		t.Fatalf("attempts = %d, want %d", attempts, itemFetchAttempts)
	}
	if !strings.Contains(err.Error(), "fetch item 48796093") {
		t.Fatalf("error = %q, want item context", err.Error())
	}
}
