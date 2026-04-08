package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/user/goharness/internal/engine"
)

func TestClientStreamMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request failed: %v", err)
		}
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"hello"}],"usage":{"input_tokens":1,"output_tokens":2}}`))
	}))
	defer srv.Close()

	client := NewClient("k", srv.URL)
	stream, err := client.StreamMessage(engine.ApiMessageRequest{Model: "m", Messages: []engine.ConversationMessage{{Role: "user", Text: "hi"}}, MaxTokens: 16})
	if err != nil {
		t.Fatalf("stream message failed: %v", err)
	}
	seenDelta := false
	seenComplete := false
	for ev := range stream {
		if ev.TextDelta != "" {
			seenDelta = true
		}
		if ev.Complete != nil {
			seenComplete = true
		}
	}
	if !seenDelta || !seenComplete {
		t.Fatalf("expected both delta and complete events")
	}
}

func TestClientAuthErrorMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`unauthorized`))
	}))
	defer srv.Close()

	client := NewClient("k", srv.URL)
	_, err := client.callWithRetry(engine.ApiMessageRequest{Model: "m", Messages: []engine.ConversationMessage{{Role: "user", Text: "hi"}}, MaxTokens: 16})
	if err == nil {
		t.Fatalf("expected auth error")
	}
	if err != nil && err.Error() == "" {
		t.Fatalf("expected non-empty auth error")
	}
}
