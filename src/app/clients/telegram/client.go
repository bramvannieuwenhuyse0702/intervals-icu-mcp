// Package telegram provides a minimal client for sending messages via the Telegram Bot API.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	baseURL           = "https://api.telegram.org"
	httpClientTimeout = 15 * time.Second
	maxResponseSize   = 1 << 20 // 1 MB

	// MaxMessageLength is the maximum length of a Telegram message text (in characters).
	MaxMessageLength = 4096
)

var (
	// ErrNotConfigured is returned when TELEGRAM_BOT_TOKEN or TELEGRAM_CHAT_ID is not set.
	ErrNotConfigured     = errors.New("telegram is not configured (set TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID)")
	errRequestFailed     = errors.New("telegram API request failed")
	errResponseTruncated = errors.New("telegram API response exceeded maximum size")
)

// Config holds the configuration for the Telegram client.
type Config struct {
	BotToken string `json:"-"`
	ChatID   string `json:"-"`
}

// Client sends messages to a single, fixed Telegram chat.
type Client struct {
	baseURL    string
	botToken   string
	chatID     string
	httpClient *http.Client
}

type sendMessageRequest struct {
	ChatID string `json:"chat_id"`
	Text   string `json:"text"`
}

type apiResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
}

// NewClient creates a new Telegram client. Missing configuration is not an error:
// the server still starts, and SendMessage returns ErrNotConfigured.
func NewClient(cfg Config) *Client {
	return &Client{
		baseURL:    baseURL,
		botToken:   cfg.BotToken,
		chatID:     cfg.ChatID,
		httpClient: &http.Client{Timeout: httpClientTimeout},
	}
}

// Configured reports whether both the bot token and chat ID are set.
func (c *Client) Configured() bool {
	return c.botToken != "" && c.chatID != ""
}

// SendMessage sends a plain-text message to the configured chat.
func (c *Client) SendMessage(ctx context.Context, text string) error {
	if !c.Configured() {
		return ErrNotConfigured
	}

	payload, err := json.Marshal(sendMessageRequest{ChatID: c.chatID, Text: text})
	if err != nil {
		return fmt.Errorf("encoding request: %w", err)
	}

	endpoint := c.baseURL + "/bot" + c.botToken + "/sendMessage"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		// Do not wrap: the error could contain the URL (and thus the bot token).
		return errRequestFailed
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %w", errRequestFailed, stripURL(err))
	}

	respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize+1))
	closeErr := resp.Body.Close()

	if readErr != nil {
		return errors.Join(fmt.Errorf("reading response body: %w", readErr), closeErr)
	}

	if int64(len(respBody)) > maxResponseSize {
		return errors.Join(errResponseTruncated, closeErr)
	}

	var parsed apiResponse

	_ = json.Unmarshal(respBody, &parsed)

	if resp.StatusCode >= http.StatusBadRequest || !parsed.OK {
		return errors.Join(
			fmt.Errorf("%w: status %d: %s", errRequestFailed, resp.StatusCode, parsed.Description),
			closeErr,
		)
	}

	if closeErr != nil {
		return fmt.Errorf("closing response body: %w", closeErr)
	}

	return nil
}

// stripURL removes the request URL (which contains the bot token) from transport errors.
func stripURL(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return urlErr.Err
	}

	return err
}
