package garmin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

const (
	defaultSSOBase    = "https://sso.garmin.com"
	defaultDIAuthBase = "https://diauth.garmin.com"
	iosClientID       = "GCM_IOS_DARK"
	iosServiceURL     = "https://mobile.integration.garmin.com/gcm/ios"
	iosLoginUA        = "Mozilla/5.0 (iPhone; CPU iPhone OS 18_7 like Mac OS X) " +
		"AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148"
	diGrantType = "https://connectapi.garmin.com/di-oauth2-service/oauth/grant/service_ticket"
)

var defaultDIClientIDs = []string{
	"GARMIN_CONNECT_MOBILE_ANDROID_DI_2025Q2",
	"GARMIN_CONNECT_MOBILE_ANDROID_DI_2024Q4",
	"GARMIN_CONNECT_MOBILE_ANDROID_DI",
	"GARMIN_CONNECT_MOBILE_IOS_DI",
}

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrRateLimited        = errors.New("rate limited by Garmin (HTTP 429)")
	ErrMFARequired        = errors.New("MFA required but no prompt provided")
)

type MFAPrompt func(method string) (string, error)

type LoginRequest struct {
	Email    string
	Password string
	MFA      MFAPrompt
}

type endpoints struct {
	ssoBase     string
	diAuthBase  string
	serviceURL  string
	clientID    string
	diClientIDs []string
	httpClient  *http.Client
}

func defaultEndpoints() endpoints {
	jar, _ := cookiejar.New(nil)
	return endpoints{
		ssoBase:     defaultSSOBase,
		diAuthBase:  defaultDIAuthBase,
		serviceURL:  iosServiceURL,
		clientID:    iosClientID,
		diClientIDs: defaultDIClientIDs,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Jar:     jar,
		},
	}
}

func Login(ctx context.Context, req LoginRequest) (*Tokens, error) {
	return loginWith(ctx, req, defaultEndpoints())
}

func loginWith(ctx context.Context, req LoginRequest, eps endpoints) (*Tokens, error) {
	if req.Email == "" || req.Password == "" {
		return nil, errors.New("email and password are required")
	}
	if eps.httpClient.Jar == nil {
		jar, _ := cookiejar.New(nil)
		eps.httpClient.Jar = jar
	}

	ticket, mfaMethod, err := mobileLogin(ctx, req.Email, req.Password, eps)
	if err != nil {
		return nil, err
	}

	if mfaMethod != "" {
		if req.MFA == nil {
			return nil, ErrMFARequired
		}
		code, err := req.MFA(mfaMethod)
		if err != nil {
			return nil, fmt.Errorf("MFA prompt: %w", err)
		}
		ticket, err = verifyMFA(ctx, mfaMethod, code, eps)
		if err != nil {
			return nil, err
		}
	}

	return exchangeServiceTicket(ctx, ticket, eps)
}

type mobileLoginResponse struct {
	ResponseStatus struct {
		Type string `json:"type"`
	} `json:"responseStatus"`
	ServiceTicketID  string `json:"serviceTicketId"`
	CustomerMFAInfo  struct {
		MFALastMethodUsed string `json:"mfaLastMethodUsed"`
	} `json:"customerMfaInfo"`
	Error struct {
		StatusCode string `json:"status-code"`
	} `json:"error"`
}

func mobileLogin(ctx context.Context, email, password string, eps endpoints) (ticket, mfaMethod string, err error) {
	body, err := json.Marshal(map[string]any{
		"username":     email,
		"password":     password,
		"rememberMe":   true,
		"captchaToken": "",
	})
	if err != nil {
		return "", "", err
	}

	loginURL := eps.ssoBase + "/mobile/api/login?" + url.Values{
		"clientId": {eps.clientID},
		"locale":   {"en-US"},
		"service":  {eps.serviceURL},
	}.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, loginURL, strings.NewReader(string(body)))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", iosLoginUA)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", eps.ssoBase)

	res, err := decodeMobileResponse(eps.httpClient, req)
	if err != nil {
		return "", "", err
	}

	switch res.ResponseStatus.Type {
	case "SUCCESSFUL":
		return res.ServiceTicketID, "", nil
	case "MFA_REQUIRED":
		method := res.CustomerMFAInfo.MFALastMethodUsed
		if method == "" {
			method = "email"
		}
		return "", method, nil
	case "INVALID_USERNAME_PASSWORD":
		return "", "", ErrInvalidCredentials
	}
	return "", "", fmt.Errorf("mobile login: unexpected response type %q", res.ResponseStatus.Type)
}

func verifyMFA(ctx context.Context, method, code string, eps endpoints) (string, error) {
	body, err := json.Marshal(map[string]any{
		"mfaMethod":           method,
		"mfaVerificationCode": code,
		"rememberMyBrowser":   true,
		"reconsentList":       []string{},
		"mfaSetup":            false,
	})
	if err != nil {
		return "", err
	}

	verifyURL := eps.ssoBase + "/mobile/api/mfa/verifyCode?" + url.Values{
		"clientId": {eps.clientID},
		"locale":   {"en-US"},
		"service":  {eps.serviceURL},
	}.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, verifyURL, strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", iosLoginUA)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", eps.ssoBase)

	res, err := decodeMobileResponse(eps.httpClient, req)
	if err != nil {
		return "", err
	}
	if res.ResponseStatus.Type != "SUCCESSFUL" {
		return "", fmt.Errorf("MFA verification rejected: %q", res.ResponseStatus.Type)
	}
	return res.ServiceTicketID, nil
}

func decodeMobileResponse(client *http.Client, req *http.Request) (*mobileLoginResponse, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, ErrRateLimited
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var out mobileLoginResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode response (HTTP %d): %w", resp.StatusCode, err)
	}
	if out.Error.StatusCode == "429" {
		return nil, ErrRateLimited
	}
	return &out, nil
}

type diTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func exchangeServiceTicket(ctx context.Context, ticket string, eps endpoints) (*Tokens, error) {
	form := url.Values{
		"service_ticket": {ticket},
		"grant_type":     {diGrantType},
		"service_url":    {eps.serviceURL},
	}

	var lastErr error
	for _, clientID := range eps.diClientIDs {
		form.Set("client_id", clientID)
		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodPost,
			eps.diAuthBase+"/di-oauth2-service/oauth/token",
			strings.NewReader(form.Encode()),
		)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(clientID+":")))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")

		resp, err := eps.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		raw, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("read DI response: %w", err)
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			return nil, ErrRateLimited
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("DI exchange %s: HTTP %d %s", clientID, resp.StatusCode, truncate(string(raw), 200))
			continue
		}

		var body diTokenResponse
		if err := json.Unmarshal(raw, &body); err != nil {
			lastErr = fmt.Errorf("DI exchange %s: decode %w", clientID, err)
			continue
		}
		if body.AccessToken == "" {
			lastErr = fmt.Errorf("DI exchange %s: empty access_token", clientID)
			continue
		}

		resolvedClient := clientIDFromJWT(body.AccessToken)
		if resolvedClient == "" {
			resolvedClient = clientID
		}
		return &Tokens{
			DIToken:        body.AccessToken,
			DIRefreshToken: body.RefreshToken,
			DIClientID:     resolvedClient,
		}, nil
	}
	return nil, fmt.Errorf("DI exchange failed for all client IDs: %w", lastErr)
}

func clientIDFromJWT(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		ClientID string `json:"client_id"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}
	return claims.ClientID
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
