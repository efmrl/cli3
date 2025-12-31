package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const workosAPIBase = "https://api.workos.com"

// DeviceCodeResponse represents the response from WorkOS device authorization endpoint
type DeviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// TokenResponse represents the response from WorkOS token endpoint
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// TokenError represents an error response from WorkOS token endpoint
type TokenError struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// RequestDeviceCode initiates the OAuth 2.0 Device Authorization flow
// Returns the device code, user code, and verification URI
func RequestDeviceCode(clientID string) (*DeviceCodeResponse, error) {
	endpoint := fmt.Sprintf("%s/user_management/authorize/device", workosAPIBase)

	// Prepare request body
	data := url.Values{}
	data.Set("client_id", clientID)

	req, err := http.NewRequest("POST", endpoint, bytes.NewBufferString(data.Encode()))
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
		return nil, fmt.Errorf("WorkOS API error (%d): %s", resp.StatusCode, string(body))
	}

	var deviceCodeResp DeviceCodeResponse
	if err := json.Unmarshal(body, &deviceCodeResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &deviceCodeResp, nil
}

// PollForToken polls the WorkOS token endpoint until the user approves the device
// Returns the access token and refresh token, or an error
func PollForToken(clientID, deviceCode string, interval int) (*TokenResponse, error) {
	endpoint := fmt.Sprintf("%s/user_management/authenticate", workosAPIBase)

	// Prepare request body
	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	data.Set("device_code", deviceCode)

	req, err := http.NewRequest("POST", endpoint, bytes.NewBufferString(data.Encode()))
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

	// Success case
	if resp.StatusCode == http.StatusOK {
		var tokenResp TokenResponse
		if err := json.Unmarshal(body, &tokenResp); err != nil {
			return nil, fmt.Errorf("failed to parse token response: %w", err)
		}
		return &tokenResp, nil
	}

	// Error cases
	var tokenErr TokenError
	if err := json.Unmarshal(body, &tokenErr); err != nil {
		return nil, fmt.Errorf("WorkOS API error (%d): %s", resp.StatusCode, string(body))
	}

	// Handle specific error codes
	switch tokenErr.Error {
	case "authorization_pending":
		// User hasn't approved yet, caller should continue polling
		return nil, &PollError{Type: "authorization_pending"}
	case "slow_down":
		// We're polling too fast, caller should increase interval
		return nil, &PollError{Type: "slow_down"}
	case "expired_token":
		return nil, fmt.Errorf("device code expired, please try again")
	case "access_denied":
		return nil, fmt.Errorf("user denied authorization")
	default:
		return nil, fmt.Errorf("WorkOS error: %s - %s", tokenErr.Error, tokenErr.ErrorDescription)
	}
}

// PollError represents a non-fatal polling error
type PollError struct {
	Type string
}

func (e *PollError) Error() string {
	return e.Type
}

// IsPollError checks if an error is a non-fatal polling error
func IsPollError(err error) bool {
	_, ok := err.(*PollError)
	return ok
}

// RefreshAccessToken exchanges a refresh token for a new access token
func RefreshAccessToken(clientID, refreshToken string) (*TokenResponse, error) {
	endpoint := fmt.Sprintf("%s/user_management/authenticate", workosAPIBase)

	// Prepare request body
	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)

	req, err := http.NewRequest("POST", endpoint, bytes.NewBufferString(data.Encode()))
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
		var tokenErr TokenError
		if err := json.Unmarshal(body, &tokenErr); err != nil {
			return nil, fmt.Errorf("WorkOS API error (%d): %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("failed to refresh token: %s - %s", tokenErr.Error, tokenErr.ErrorDescription)
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	return &tokenResp, nil
}
