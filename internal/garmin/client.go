package garmin

import (
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
	defaultConnectAPIBase = "https://connectapi.garmin.com"

	nativeUserAgent  = "GCM-Android-5.23"
	nativeXUserAgent = "com.garmin.android.apps.connectmobile/5.23; ; " +
		"Google/sdk_gphone64_arm64/google; Android/33; Dalvik/2.1.0"
	nativeAppVersion = "10861"
)

type Client struct {
	tokens       *Tokens
	httpClient   *http.Client
	apiBase      string
	displayName  string
	tokenUpdated bool // tracks if tokens were refreshed this session
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

// TokensUpdated returns true if the tokens were refreshed during this session.
// Callers should save the new tokens if this returns true.
func (c *Client) TokensUpdated() bool {
	return c.tokenUpdated
}

func (c *Client) WithAPIBase(url string) *Client {
	c.apiBase = url
	return c
}

func (c *Client) Get(ctx context.Context, path string, params url.Values) (json.RawMessage, error) {
	body, err := c.get(ctx, path, params)
	if err != nil && (isAuthError(err) && c.tokens.DIRefreshToken != "") {
		// Attempt token refresh once on auth failure
		refreshed, refreshErr := RefreshDIToken(ctx, c.tokens)
		if refreshErr != nil {
			// Both wrapped: callers stop on ErrInvalidCredentials, and the
			// refresh error alone doesn't carry it.
			return nil, fmt.Errorf("token refresh failed: %w (original: %w)", refreshErr, err)
		}
		c.tokens = refreshed
		c.tokenUpdated = true
		// Retry the request with new token
		return c.get(ctx, path, params)
	}
	return body, err
}

func (c *Client) get(ctx context.Context, path string, params url.Values) (json.RawMessage, error) {
	full := c.apiBase + path
	if len(params) > 0 {
		full += "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
	if err != nil {
		return nil, err
	}
	c.setNativeHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, ErrRateLimited
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return nil, fmt.Errorf("%w: HTTP %d on %s", ErrInvalidCredentials, resp.StatusCode, path)
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("GET %s: HTTP %d %s", path, resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return body, nil
}

func isAuthError(err error) bool {
	return errors.Is(err, ErrInvalidCredentials)
}

func (c *Client) setNativeHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.tokens.DIToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", nativeUserAgent)
	req.Header.Set("X-Garmin-User-Agent", nativeXUserAgent)
	req.Header.Set("X-Garmin-Paired-App-Version", nativeAppVersion)
	req.Header.Set("X-Garmin-Client-Platform", "Android")
	req.Header.Set("X-App-Ver", nativeAppVersion)
	req.Header.Set("X-Lang", "en")
	req.Header.Set("X-GCExperience", "GC5")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
}

func (c *Client) DisplayName(ctx context.Context) (string, error) {
	if c.displayName != "" {
		return c.displayName, nil
	}
	raw, err := c.Get(ctx, "/userprofile-service/socialProfile", nil)
	if err != nil {
		return "", fmt.Errorf("fetch social profile: %w", err)
	}
	var profile struct {
		DisplayName string `json:"displayName"`
		UserName    string `json:"userName"`
	}
	if err := json.Unmarshal(raw, &profile); err != nil {
		return "", fmt.Errorf("decode social profile: %w", err)
	}
	c.displayName = profile.DisplayName
	if c.displayName == "" {
		c.displayName = profile.UserName
	}
	if c.displayName == "" {
		return "", fmt.Errorf("social profile returned no displayName or userName")
	}
	return c.displayName, nil
}
