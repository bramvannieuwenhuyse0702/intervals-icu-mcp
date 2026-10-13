package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendMessage_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		if r.URL.Path != "/botTOKEN123/sendMessage" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decoding body: %v", err)
		}

		if body["chat_id"] != "42" || body["text"] != "hallo 🟢" {
			t.Errorf("unexpected body: %v", body)
		}

		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	defer srv.Close()

	c := NewTestClient(srv.URL, "TOKEN123", "42", srv.Client())

	if err := c.SendMessage(context.Background(), "hallo 🟢"); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestSendMessage_NotConfigured(t *testing.T) {
	t.Parallel()

	c := NewClient(Config{})

	if c.Configured() {
		t.Fatal("expected Configured() to be false")
	}

	err := c.SendMessage(context.Background(), "x")
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

func TestSendMessage_APIErrorDoesNotLeakToken(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"ok":false,"description":"Bad Request: chat not found"}`))
	}))
	defer srv.Close()

	c := NewTestClient(srv.URL, "SECRET-TOKEN", "42", srv.Client())

	err := c.SendMessage(context.Background(), "x")
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "chat not found") {
		t.Fatalf("expected API description in error, got %v", err)
	}

	if strings.Contains(err.Error(), "SECRET-TOKEN") {
		t.Fatalf("error leaks bot token: %v", err)
	}
}

func TestSendMessage_TransportErrorDoesNotLeakToken(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	srv.Close() // closed server -> connection refused

	c := NewTestClient(srv.URL, "SECRET-TOKEN", "42", http.DefaultClient)

	err := c.SendMessage(context.Background(), "x")
	if err == nil {
		t.Fatal("expected error")
	}

	if strings.Contains(err.Error(), "SECRET-TOKEN") {
		t.Fatalf("error leaks bot token: %v", err)
	}
}
