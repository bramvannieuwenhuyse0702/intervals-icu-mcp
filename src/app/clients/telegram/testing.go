package telegram

import "net/http"

// NewTestClient creates a Client with a custom base URL and HTTP client for testing.
func NewTestClient(baseURL, botToken, chatID string, httpClient *http.Client) *Client {
	return &Client{
		baseURL:    baseURL,
		botToken:   botToken,
		chatID:     chatID,
		httpClient: httpClient,
	}
}
