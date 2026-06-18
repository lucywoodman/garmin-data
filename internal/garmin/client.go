package garmin

import (
	"net/http"
	"time"
)

const defaultConnectAPIBase = "https://connectapi.garmin.com"

type Client struct {
	tokens     *Tokens
	httpClient *http.Client
	apiBase    string
}

func NewClient(tokens *Tokens) *Client {
	return &Client{
		tokens:     tokens,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		apiBase:    defaultConnectAPIBase,
	}
}

func (c *Client) Tokens() *Tokens {
	return c.tokens
}
