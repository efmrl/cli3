package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// APIClient handles authenticated API requests to the efmrl server
type APIClient struct {
	BaseURL       string
	host          string
	refreshFailed bool // true after a failed token refresh; prevents repeated attempts
}

// AuthFailed reports whether a token refresh was attempted and failed.
func (c *APIClient) AuthFailed() bool {
	return c.refreshFailed
}

// NewAPIClient creates a new API client for the specified base URL
func NewAPIClient(baseURL string) (*APIClient, error) {
	// Extract host from baseURL for credential lookups
	// baseURL format: https://efmrl.samf.workers.dev or http://localhost:8787
	host := baseURL
	if len(host) > 8 && host[:8] == "https://" {
		host = host[8:]
	} else if len(host) > 7 && host[:7] == "http://" {
		host = host[7:]
	}

	return &APIClient{
		BaseURL: baseURL,
		host:    host,
	}, nil
}

// getAccessToken retrieves the access token from global config
func (c *APIClient) getAccessToken() (string, error) {
	config, err := LoadGlobalConfig()
	if err != nil {
		return "", fmt.Errorf("failed to load credentials: %w", err)
	}

	creds, ok := config.GetHostCredentials(c.host)
	if !ok || creds.AccessToken == "" {
		return "", fmt.Errorf("not logged in to %s (run 'efmrl3 login' first)", c.host)
	}

	return creds.AccessToken, nil
}

// refreshTokenIfNeeded attempts to refresh the access token using the refresh token.
// It branches on creds.Provider to use the correct refresh mechanism.
func (c *APIClient) refreshTokenIfNeeded() error {
	config, err := LoadGlobalConfig()
	if err != nil {
		return fmt.Errorf("failed to load credentials: %w", err)
	}

	creds, ok := config.GetHostCredentials(c.host)
	if !ok || creds.RefreshToken == "" {
		return fmt.Errorf("no refresh token available (run 'efmrl3 login' again)")
	}

	var newCreds HostCredentials

	switch creds.Provider {
	case "google":
		clientID := getGoogleClientID()
		clientSecret := getGoogleClientSecret()
		tokenResp, err := RefreshGoogleToken(clientID, clientSecret, creds.RefreshToken)
		if err != nil {
			return fmt.Errorf("failed to refresh Google token: %w", err)
		}
		// Google may not return a new refresh_token; keep the old one if absent
		newRefreshToken := tokenResp.RefreshToken
		if newRefreshToken == "" {
			newRefreshToken = creds.RefreshToken
		}
		newCreds = HostCredentials{
			AccessToken:  tokenResp.IDToken,
			RefreshToken: newRefreshToken,
			Provider:     "google",
		}
	default: // "workos" or legacy entries without a provider
		clientID := getWorkOSClientID()
		tokenResp, err := RefreshAccessToken(clientID, creds.RefreshToken)
		if err != nil {
			return fmt.Errorf("failed to refresh token: %w", err)
		}
		newCreds = HostCredentials{
			AccessToken:  tokenResp.AccessToken,
			RefreshToken: tokenResp.RefreshToken,
			Provider:     "workos",
		}
	}

	config.SetHostCredentials(c.host, newCreds)
	if err := SaveGlobalConfig(config); err != nil {
		return fmt.Errorf("failed to save refreshed credentials: %w", err)
	}

	return nil
}

// doRequest performs an HTTP request with authentication
func (c *APIClient) doRequest(method, path string, body interface{}) (*http.Response, error) {
	url := c.BaseURL + path

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Get access token
	accessToken, err := c.getAccessToken()
	if err != nil {
		return nil, err
	}

	// Add Authorization header
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	// If we get 401, try refreshing the token and retry once
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()

		if c.refreshFailed {
			return nil, fmt.Errorf("session expired — run 'efmrl3 login' to re-authenticate")
		}

		fmt.Fprintln(os.Stderr, "Access token expired, refreshing...")

		if err := c.refreshTokenIfNeeded(); err != nil {
			c.refreshFailed = true
			return nil, fmt.Errorf("session expired — run 'efmrl3 login' to re-authenticate")
		}

		// Retry the request with the new token
		accessToken, err = c.getAccessToken()
		if err != nil {
			return nil, err
		}

		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))

		resp, err = client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("retry request failed: %w", err)
		}
	}

	return resp, nil
}

// Get performs a GET request
func (c *APIClient) Get(path string) (*http.Response, error) {
	return c.doRequest("GET", path, nil)
}

// Post performs a POST request
func (c *APIClient) Post(path string, body interface{}) (*http.Response, error) {
	return c.doRequest("POST", path, body)
}

// Patch performs a PATCH request
func (c *APIClient) Patch(path string, body interface{}) (*http.Response, error) {
	return c.doRequest("PATCH", path, body)
}

// Delete performs a DELETE request
func (c *APIClient) Delete(path string) (*http.Response, error) {
	return c.doRequest("DELETE", path, nil)
}

// getWorkOSClientID returns the WorkOS client ID from environment or default
func getWorkOSClientID() string {
	clientID := os.Getenv("WORKOS_CLIENT_ID")
	if clientID == "" {
		// This is a public identifier (OAuth client ID), safe to include in the CLI
		clientID = "client_01KCJEFZQVSPSKHT1QAS19R0X1"
	}
	return clientID
}
