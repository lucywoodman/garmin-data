package garmin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
)

type ssoState struct {
	loginCalls       int
	mfaCalls         int
	diCalls          map[string]int
	mfaCode          string
	rejectFirstNDI   int
	loginResponse    string
	mfaResponse      string
	loginStatus      int
	diStatus         int
	diTokenForClient string
	diRefreshToken   string
}

func newTestServer(t *testing.T, state *ssoState) *httptest.Server {
	t.Helper()
	if state.diCalls == nil {
		state.diCalls = map[string]int{}
	}
	if state.loginStatus == 0 {
		state.loginStatus = http.StatusOK
	}
	if state.diStatus == 0 {
		state.diStatus = http.StatusOK
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/mobile/api/login", func(w http.ResponseWriter, r *http.Request) {
		state.loginCalls++
		http.SetCookie(w, &http.Cookie{Name: "GARMIN-SSO", Value: "sso-session", Path: "/"})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(state.loginStatus)
		_, _ = io.WriteString(w, state.loginResponse)
	})

	mux.HandleFunc("/mobile/api/mfa/verifyCode", func(w http.ResponseWriter, r *http.Request) {
		state.mfaCalls++
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if code, _ := body["mfaVerificationCode"].(string); code != state.mfaCode {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"responseStatus":{"type":"INVALID_CODE"}}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, state.mfaResponse)
	})

	mux.HandleFunc("/di-oauth2-service/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		grantType := r.PostFormValue("grant_type")

		// For refresh_token grant, extract client_id from Authorization header
		clientID := r.PostFormValue("client_id")
		if grantType == "refresh_token" && clientID == "" {
			auth := r.Header.Get("Authorization")
			if basicCreds, ok := strings.CutPrefix(auth, "Basic "); ok {
				decoded, _ := base64.StdEncoding.DecodeString(basicCreds)
				parts := strings.Split(string(decoded), ":")
				if len(parts) > 0 {
					clientID = parts[0]
				}
			}
		}

		state.diCalls[clientID]++

		if state.diStatus != http.StatusOK {
			w.WriteHeader(state.diStatus)
			return
		}
		if state.rejectFirstNDI > 0 {
			state.rejectFirstNDI--
			http.Error(w, `{"error":"invalid_client"}`, http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		token := state.diTokenForClient
		if token == "" {
			token = makeJWT(t, map[string]any{"client_id": clientID})
		}
		_ = json.NewEncoder(w).Encode(diTokenResponse{
			AccessToken:  token,
			RefreshToken: state.diRefreshToken,
		})
	})

	return httptest.NewServer(mux)
}

func makeJWT(t *testing.T, claims map[string]any) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	body := base64.RawURLEncoding.EncodeToString(payload)
	return header + "." + body + ".signature"
}

func testEndpoints(server *httptest.Server) endpoints {
	jar, _ := cookiejar.New(nil)
	return endpoints{
		ssoBase:     server.URL,
		diAuthBase:  server.URL,
		serviceURL:  iosServiceURL,
		clientID:    iosClientID,
		diClientIDs: []string{"FIRST", "SECOND", "THIRD"},
		httpClient:  &http.Client{Jar: jar},
	}
}

func TestLoginHappyPath(t *testing.T) {
	state := &ssoState{
		loginResponse:  `{"responseStatus":{"type":"SUCCESSFUL"},"serviceTicketId":"TICKET-123"}`,
		diRefreshToken: "refresh-xyz",
	}
	srv := newTestServer(t, state)
	defer srv.Close()

	tokens, err := loginWith(context.Background(),
		LoginRequest{Email: "user@example.com", Password: "hunter2"},
		testEndpoints(srv),
	)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if tokens.DIToken == "" {
		t.Error("expected DIToken to be set")
	}
	if tokens.DIRefreshToken != "refresh-xyz" {
		t.Errorf("DIRefreshToken: want refresh-xyz, got %q", tokens.DIRefreshToken)
	}
	if tokens.DIClientID != "FIRST" {
		t.Errorf("DIClientID: want FIRST (from JWT), got %q", tokens.DIClientID)
	}
	if state.loginCalls != 1 {
		t.Errorf("loginCalls: want 1, got %d", state.loginCalls)
	}
	if state.diCalls["FIRST"] != 1 {
		t.Errorf("diCalls[FIRST]: want 1, got %d", state.diCalls["FIRST"])
	}
}

func TestLoginMFAPath(t *testing.T) {
	state := &ssoState{
		loginResponse: `{"responseStatus":{"type":"MFA_REQUIRED"},"customerMfaInfo":{"mfaLastMethodUsed":"authenticator"}}`,
		mfaCode:       "654321",
		mfaResponse:   `{"responseStatus":{"type":"SUCCESSFUL"},"serviceTicketId":"TICKET-MFA"}`,
	}
	srv := newTestServer(t, state)
	defer srv.Close()

	var receivedMethod string
	mfaPrompt := func(method string) (string, error) {
		receivedMethod = method
		return "654321", nil
	}

	tokens, err := loginWith(context.Background(),
		LoginRequest{Email: "user@example.com", Password: "hunter2", MFA: mfaPrompt},
		testEndpoints(srv),
	)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if tokens.DIToken == "" {
		t.Error("expected DIToken to be set")
	}
	if receivedMethod != "authenticator" {
		t.Errorf("MFA method passed to prompt: want authenticator, got %q", receivedMethod)
	}
	if state.mfaCalls != 1 {
		t.Errorf("mfaCalls: want 1, got %d", state.mfaCalls)
	}
}

func TestLoginMFARequiredNoPrompt(t *testing.T) {
	state := &ssoState{
		loginResponse: `{"responseStatus":{"type":"MFA_REQUIRED"},"customerMfaInfo":{"mfaLastMethodUsed":"email"}}`,
	}
	srv := newTestServer(t, state)
	defer srv.Close()

	_, err := loginWith(context.Background(),
		LoginRequest{Email: "user@example.com", Password: "hunter2"},
		testEndpoints(srv),
	)
	if !errors.Is(err, ErrMFARequired) {
		t.Fatalf("expected ErrMFARequired, got %v", err)
	}
}

func TestLoginInvalidCredentials(t *testing.T) {
	state := &ssoState{
		loginResponse: `{"responseStatus":{"type":"INVALID_USERNAME_PASSWORD"}}`,
	}
	srv := newTestServer(t, state)
	defer srv.Close()

	_, err := loginWith(context.Background(),
		LoginRequest{Email: "user@example.com", Password: "wrong"},
		testEndpoints(srv),
	)
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLoginRateLimited(t *testing.T) {
	state := &ssoState{loginStatus: http.StatusTooManyRequests}
	srv := newTestServer(t, state)
	defer srv.Close()

	_, err := loginWith(context.Background(),
		LoginRequest{Email: "user@example.com", Password: "hunter2"},
		testEndpoints(srv),
	)
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited, got %v", err)
	}
}

func TestLoginRateLimited429InJSONBody(t *testing.T) {
	state := &ssoState{
		loginResponse: `{"error":{"status-code":"429"}}`,
	}
	srv := newTestServer(t, state)
	defer srv.Close()

	_, err := loginWith(context.Background(),
		LoginRequest{Email: "user@example.com", Password: "hunter2"},
		testEndpoints(srv),
	)
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited from JSON body 429, got %v", err)
	}
}

func TestDIExchangeFallsThroughClientIDs(t *testing.T) {
	state := &ssoState{
		loginResponse:  `{"responseStatus":{"type":"SUCCESSFUL"},"serviceTicketId":"T1"}`,
		rejectFirstNDI: 2,
	}
	srv := newTestServer(t, state)
	defer srv.Close()

	tokens, err := loginWith(context.Background(),
		LoginRequest{Email: "u", Password: "p"},
		testEndpoints(srv),
	)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if state.diCalls["FIRST"] != 1 || state.diCalls["SECOND"] != 1 || state.diCalls["THIRD"] != 1 {
		t.Errorf("expected each of FIRST/SECOND/THIRD called once, got %+v", state.diCalls)
	}
	if tokens.DIClientID != "THIRD" {
		t.Errorf("DIClientID: want THIRD (from JWT of last call), got %q", tokens.DIClientID)
	}
}

func TestDIExchangeAllFail(t *testing.T) {
	state := &ssoState{
		loginResponse:  `{"responseStatus":{"type":"SUCCESSFUL"},"serviceTicketId":"T1"}`,
		rejectFirstNDI: 99,
	}
	srv := newTestServer(t, state)
	defer srv.Close()

	_, err := loginWith(context.Background(),
		LoginRequest{Email: "u", Password: "p"},
		testEndpoints(srv),
	)
	if err == nil {
		t.Fatal("expected error when all DI client IDs fail")
	}
	if !strings.Contains(err.Error(), "DI exchange failed for all client IDs") {
		t.Errorf("error message: want DI exchange wrapper, got %v", err)
	}
}

func TestDIClientIDFromJWTFallback(t *testing.T) {
	state := &ssoState{
		loginResponse:    `{"responseStatus":{"type":"SUCCESSFUL"},"serviceTicketId":"T1"}`,
		diTokenForClient: "not-a-jwt",
	}
	srv := newTestServer(t, state)
	defer srv.Close()

	tokens, err := loginWith(context.Background(),
		LoginRequest{Email: "u", Password: "p"},
		testEndpoints(srv),
	)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if tokens.DIClientID != "FIRST" {
		t.Errorf("DIClientID: want FIRST (fallback when JWT unparseable), got %q", tokens.DIClientID)
	}
}

func TestLoginEmptyCredentials(t *testing.T) {
	_, err := loginWith(context.Background(),
		LoginRequest{Email: "", Password: "p"},
		testEndpoints(httptest.NewServer(http.NewServeMux())),
	)
	if err == nil {
		t.Fatal("expected error for empty email")
	}
}

func TestRefreshDIToken(t *testing.T) {
	state := &ssoState{
		diRefreshToken: "new-refresh-token",
	}
	srv := newTestServer(t, state)
	defer srv.Close()

	oldTokens := &Tokens{
		DIToken:        "old-access-token",
		DIRefreshToken: "old-refresh-token",
		DIClientID:     "FIRST",
	}

	refreshed, err := refreshDITokenWith(context.Background(), oldTokens, testEndpoints(srv))
	if err != nil {
		t.Fatalf("RefreshDIToken: %v", err)
	}
	if refreshed.DIToken == "" {
		t.Error("expected new access token")
	}
	if refreshed.DIRefreshToken != "new-refresh-token" {
		t.Errorf("DIRefreshToken: want new-refresh-token, got %q", refreshed.DIRefreshToken)
	}
	if refreshed.DIClientID != "FIRST" {
		t.Errorf("DIClientID: want FIRST (preserved), got %q", refreshed.DIClientID)
	}
}

func TestRefreshDITokenMissingRefreshToken(t *testing.T) {
	tokens := &Tokens{
		DIToken:        "access",
		DIRefreshToken: "",
		DIClientID:     "CLIENT",
	}
	_, err := refreshDITokenWith(context.Background(), tokens, testEndpoints(httptest.NewServer(http.NewServeMux())))
	if err == nil || !strings.Contains(err.Error(), "no refresh token") {
		t.Fatalf("expected no refresh token error, got %v", err)
	}
}

func TestRefreshDITokenMissingClientID(t *testing.T) {
	tokens := &Tokens{
		DIToken:        "access",
		DIRefreshToken: "refresh",
		DIClientID:     "",
	}
	_, err := refreshDITokenWith(context.Background(), tokens, testEndpoints(httptest.NewServer(http.NewServeMux())))
	if err == nil || !strings.Contains(err.Error(), "no DI client ID") {
		t.Fatalf("expected no DI client ID error, got %v", err)
	}
}
