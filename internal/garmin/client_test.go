package garmin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func newClientFor(server *httptest.Server) *Client {
	c := NewClient(&Tokens{DIToken: "test-bearer", DIClientID: "TEST"})
	c.apiBase = server.URL
	return c
}

func TestGetSendsBearerAndNativeHeaders(t *testing.T) {
	var seen http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Clone()
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	_, err := newClientFor(srv).Get(context.Background(), "/x", nil)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if got := seen.Get("Authorization"); got != "Bearer test-bearer" {
		t.Errorf("Authorization: want Bearer test-bearer, got %q", got)
	}
	if got := seen.Get("X-Garmin-Client-Platform"); got != "Android" {
		t.Errorf("X-Garmin-Client-Platform: want Android, got %q", got)
	}
	if got := seen.Get("User-Agent"); got != nativeUserAgent {
		t.Errorf("User-Agent: want %q, got %q", nativeUserAgent, got)
	}
}

func TestGetAttachesQueryParams(t *testing.T) {
	var seenQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenQuery = r.URL.Query()
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	_, err := newClientFor(srv).Get(context.Background(), "/x", url.Values{"a": {"1"}, "b": {"two"}})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if seenQuery.Get("a") != "1" || seenQuery.Get("b") != "two" {
		t.Errorf("query: want a=1 b=two, got %v", seenQuery)
	}
}

func TestGet429ReturnsErrRateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	_, err := newClientFor(srv).Get(context.Background(), "/x", nil)
	if !errors.Is(err, ErrRateLimited) {
		t.Errorf("want ErrRateLimited, got %v", err)
	}
}

func TestGet401ReturnsErrInvalidCredentials(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := newClientFor(srv).Get(context.Background(), "/x", nil)
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("want ErrInvalidCredentials, got %v", err)
	}
}

// A 401 whose refresh also fails is still an auth failure. The sync loop stops
// on ErrInvalidCredentials, and losing it here meant a dead token skipped every
// metric of every day and exited 0.
func TestGet401WithFailedRefreshStillReturnsErrInvalidCredentials(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	// No DI client ID makes the refresh fail before it reaches the network.
	client := NewClient(&Tokens{DIToken: "t", DIRefreshToken: "r"}).WithAPIBase(srv.URL)
	_, err := client.Get(context.Background(), "/x", nil)
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("want ErrInvalidCredentials, got %v", err)
	}
}

func TestDisplayNameCachesAfterFirstFetch(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/userprofile-service/socialProfile" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		calls++
		_, _ = w.Write([]byte(`{"displayName":"lucy123"}`))
	}))
	defer srv.Close()

	c := newClientFor(srv)
	for i := 0; i < 3; i++ {
		name, err := c.DisplayName(context.Background())
		if err != nil {
			t.Fatalf("DisplayName: %v", err)
		}
		if name != "lucy123" {
			t.Errorf("want lucy123, got %q", name)
		}
	}
	if calls != 1 {
		t.Errorf("want 1 social profile call, got %d", calls)
	}
}

func TestDisplayNameFallsBackToUserName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"userName":"lucy-fallback"}`))
	}))
	defer srv.Close()

	name, err := newClientFor(srv).DisplayName(context.Background())
	if err != nil {
		t.Fatalf("DisplayName: %v", err)
	}
	if name != "lucy-fallback" {
		t.Errorf("want lucy-fallback, got %q", name)
	}
}
