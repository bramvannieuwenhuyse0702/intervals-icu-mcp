package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/0xalexb/intervals-icu-mcp/src/app/clients/telegram"
)

func callSendTelegram(t *testing.T, c *telegram.Client, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	mcpServer := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)
	NewSendTelegramTool(c)(mcpServer)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	ctx := context.Background()

	serverCtx, serverCancel := context.WithCancel(ctx)
	serverDone := make(chan struct{})

	go func() {
		defer close(serverDone)

		_ = mcpServer.Run(serverCtx, serverTransport)
	}()

	t.Cleanup(func() {
		serverCancel()
		<-serverDone
	})

	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)

	connectCtx, connectCancel := context.WithTimeout(ctx, 5*time.Second)
	defer connectCancel()

	session, err := mcpClient.Connect(connectCtx, clientTransport, nil)
	if err != nil {
		t.Fatalf("expected client to connect, got: %v", err)
	}

	t.Cleanup(func() { _ = session.Close() })

	callCtx, callCancel := context.WithTimeout(ctx, 5*time.Second)
	defer callCancel()

	result, err := session.CallTool(callCtx, &mcp.CallToolParams{Name: "send_telegram", Arguments: args})
	if err != nil {
		t.Fatalf("expected CallTool to succeed, got: %v", err)
	}

	return result
}

func TestNewSendTelegramTool_Success(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if r.URL.Path != "/botTOKEN/sendMessage" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer apiServer.Close()

	c := telegram.NewTestClient(apiServer.URL, "TOKEN", "42", apiServer.Client())

	result := callSendTelegram(t, c, map[string]any{"text": "🟢 Goed hersteld"})

	if result.IsError {
		t.Fatalf("expected success, got error result: %+v", result.Content)
	}

	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok || text.Text != "Message sent." {
		t.Fatalf("unexpected content: %+v", result.Content)
	}

	if calls.Load() != 1 {
		t.Fatalf("expected 1 API call, got %d", calls.Load())
	}
}

func TestNewSendTelegramTool_Validation(t *testing.T) {
	t.Parallel()

	apiServer := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("API should not be called for validation errors")
	}))
	defer apiServer.Close()

	c := telegram.NewTestClient(apiServer.URL, "TOKEN", "42", apiServer.Client())

	for name, text := range map[string]string{
		"empty":    "   ",
		"too long": strings.Repeat("a", telegram.MaxMessageLength+1),
	} {
		result := callSendTelegram(t, c, map[string]any{"text": text})
		if !result.IsError {
			t.Fatalf("%s: expected IsError to be true", name)
		}
	}
}

func TestNewSendTelegramTool_NotConfigured(t *testing.T) {
	t.Parallel()

	c := telegram.NewClient(telegram.Config{})

	result := callSendTelegram(t, c, map[string]any{"text": "hallo"})
	if !result.IsError {
		t.Fatal("expected IsError to be true when telegram is not configured")
	}
}
