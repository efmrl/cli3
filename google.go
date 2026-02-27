package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

// PollError represents a non-fatal polling error during device authorization.
type PollError struct{ Type string }

func (e *PollError) Error() string { return e.Type }

// IsPollError checks if an error is a non-fatal polling error.
func IsPollError(err error) bool { _, ok := err.(*PollError); return ok }

// googleDeviceClientID and googleDeviceClientSecret are the "TV and Limited Input"
// OAuth credentials for the CLI device flow. They are safe to embed in the binary;
// Google's own documentation permits this for installed/device-flow clients.
const (
	googleDeviceClientID     = "384561155891-j89kklto18vvps5ar0a5fnh2mvol394o.apps.googleusercontent.com"
	googleDeviceClientSecret = "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"
	googleDeviceCodeURL      = "https://oauth2.googleapis.com/device/code"
	googleTokenURL           = "https://oauth2.googleapis.com/token"
)

// GoogleDeviceCodeResponse is the response from Google's device authorization endpoint.
type GoogleDeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURL string `json:"verification_url"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// GoogleTokenResponse is the response from Google's token endpoint.
// We store IDToken as the bearer token sent to our API — it's a signed JWT
// with iss=https://accounts.google.com, which the server can validate.
type GoogleTokenResponse struct {
	AccessToken  string `json:"access_token"`  // opaque; used only for refresh
	IDToken      string `json:"id_token"`      // JWT; used as bearer to our API
	RefreshToken string `json:"refresh_token"` // may be absent on refresh
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// GoogleTokenError is an error response from Google's token endpoint.
type GoogleTokenError struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// getGoogleClientID returns the Google device client ID, overridable via env.
func getGoogleClientID() string {
	if id := os.Getenv("GOOGLE_DEVICE_CLIENT_ID"); id != "" {
		return id
	}
	return googleDeviceClientID
}

// getGoogleClientSecret returns the Google device client secret, overridable via env.
func getGoogleClientSecret() string {
	if s := os.Getenv("GOOGLE_DEVICE_CLIENT_SECRET"); s != "" {
		return s
	}
	return googleDeviceClientSecret
}

// RequestGoogleDeviceCode initiates the Google Device Authorization Grant (RFC 8628).
func RequestGoogleDeviceCode(clientID string) (*GoogleDeviceCodeResponse, error) {
	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("scope", "openid email profile")

	req, err := http.NewRequest("POST", googleDeviceCodeURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Google API error (%d): %s", resp.StatusCode, string(body))
	}

	var result GoogleDeviceCodeResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

// PollGoogleDeviceAuth polls Google's token endpoint until the user approves the device.
// Returns the same PollError types as the WorkOS poller so login.go can reuse the same
// polling loop logic.
func PollGoogleDeviceAuth(clientID, clientSecret, deviceCode string) (*GoogleTokenResponse, error) {
	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("device_code", deviceCode)
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")

	req, err := http.NewRequest("POST", googleTokenURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == http.StatusOK {
		var tokenResp GoogleTokenResponse
		if err := json.Unmarshal(body, &tokenResp); err != nil {
			return nil, fmt.Errorf("failed to parse token response: %w", err)
		}
		return &tokenResp, nil
	}

	var tokenErr GoogleTokenError
	if err := json.Unmarshal(body, &tokenErr); err != nil {
		return nil, fmt.Errorf("Google API error (%d): %s", resp.StatusCode, string(body))
	}

	switch tokenErr.Error {
	case "authorization_pending":
		return nil, &PollError{Type: "authorization_pending"}
	case "slow_down":
		return nil, &PollError{Type: "slow_down"}
	case "expired_token":
		return nil, fmt.Errorf("device code expired, please try again")
	case "access_denied":
		return nil, fmt.Errorf("user denied authorization")
	default:
		return nil, fmt.Errorf("Google error: %s - %s", tokenErr.Error, tokenErr.ErrorDescription)
	}
}

// RefreshGoogleToken exchanges a refresh token for a new id_token (and access_token).
// Google does not always return a new refresh_token; the caller should keep the old one
// if the response omits it.
func RefreshGoogleToken(clientID, clientSecret, refreshToken string) (*GoogleTokenResponse, error) {
	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("refresh_token", refreshToken)
	data.Set("grant_type", "refresh_token")

	req, err := http.NewRequest("POST", googleTokenURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var tokenErr GoogleTokenError
		if err := json.Unmarshal(body, &tokenErr); err != nil {
			return nil, fmt.Errorf("Google API error (%d): %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("failed to refresh token: %s - %s", tokenErr.Error, tokenErr.ErrorDescription)
	}

	var tokenResp GoogleTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	return &tokenResp, nil
}
