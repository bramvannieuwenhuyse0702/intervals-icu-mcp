package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/0xalexb/intervals-icu-mcp/src/app/clients/telegram"
)

var (
	errMissingText = errors.New("text is required")
	errTextTooLong = errors.New("text exceeds 4096 characters")
)

type sendTelegramArgs struct {
	Text string `json:"text" jsonschema:"plain-text message (max 4096 chars, emoji ok, no markdown)"`
}

// NewSendTelegramTool returns a ToolRegistration that registers the send_telegram tool on an MCP server.
func NewSendTelegramTool(tgClient *telegram.Client) ToolRegistration {
	return func(server *mcp.Server) {
		mcp.AddTool(server,
			&mcp.Tool{
				Name: "send_telegram",
				Description: "Sends a plain-text message to the athlete's own Telegram chat " +
					"(fixed chat configured on the server). Use for daily training summaries and alerts.",
			},
			func(ctx context.Context, _ *mcp.CallToolRequest, args sendTelegramArgs) (*mcp.CallToolResult, any, error) {
				if strings.TrimSpace(args.Text) == "" {
					return nil, nil, errMissingText
				}

				if utf8.RuneCountInString(args.Text) > telegram.MaxMessageLength {
					return nil, nil, errTextTooLong
				}

				err := tgClient.SendMessage(ctx, args.Text)
				if err != nil {
					return nil, nil, fmt.Errorf("sending telegram message: %w", err)
				}

				return &mcp.CallToolResult{
					Content: []mcp.Content{
						&mcp.TextContent{Text: "Message sent."},
					},
				}, nil, nil
			},
		)
	}
}
