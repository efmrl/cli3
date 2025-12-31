package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/pkg/browser"
)

// LoginCmd handles user authentication via WorkOS device flow
type LoginCmd struct {
	Host string `help:"Server host" default:"efmrl.samf.workers.dev"`
}

// Run executes the login command
func (l *LoginCmd) Run() error {
	fmt.Println("Authenticating with efmrl...")

	// Get WorkOS client ID
	clientID := getWorkOSClientID()

	// Step 1: Request device code from WorkOS
	deviceCode, err := RequestDeviceCode(clientID)
	if err != nil {
		return fmt.Errorf("failed to initiate device authorization: %w", err)
	}

	// Step 2: Display instructions to user
	fmt.Println()
	fmt.Println("Please authenticate by visiting:")
	fmt.Printf("  %s\n", deviceCode.VerificationURI)
	fmt.Println()
	fmt.Printf("And entering code: %s\n", deviceCode.UserCode)
	fmt.Println()

	// Step 3: Auto-open browser
	if deviceCode.VerificationURIComplete != "" {
		fmt.Println("Opening browser automatically...")
		if err := browser.OpenURL(deviceCode.VerificationURIComplete); err != nil {
			fmt.Fprintf(os.Stderr, "Could not open browser automatically: %v\n", err)
			fmt.Fprintf(os.Stderr, "Please visit the URL above manually.\n")
		}
	} else {
		// Fallback to opening base verification URI
		fmt.Println("Opening browser automatically...")
		if err := browser.OpenURL(deviceCode.VerificationURI); err != nil {
			fmt.Fprintf(os.Stderr, "Could not open browser automatically: %v\n", err)
			fmt.Fprintf(os.Stderr, "Please visit the URL above manually.\n")
		}
	}

	fmt.Println()
	fmt.Println("Waiting for authentication... (press Ctrl+C to cancel)")

	// Step 4: Poll for token
	pollInterval := time.Duration(deviceCode.Interval) * time.Second
	if pollInterval < 5*time.Second {
		pollInterval = 5 * time.Second // Minimum interval
	}

	expiresAt := time.Now().Add(time.Duration(deviceCode.ExpiresIn) * time.Second)

	var tokenResp *TokenResponse
	for {
		// Check if device code has expired
		if time.Now().After(expiresAt) {
			return fmt.Errorf("device code expired, please try again")
		}

		// Poll for token
		tokenResp, err = PollForToken(clientID, deviceCode.DeviceCode, deviceCode.Interval)
		if err != nil {
			if IsPollError(err) {
				pollErr := err.(*PollError)
				if pollErr.Type == "slow_down" {
					// Increase polling interval
					pollInterval = pollInterval + 5*time.Second
					fmt.Fprintln(os.Stderr, "Slowing down polling...")
				}
				// Continue polling for authorization_pending and slow_down
				time.Sleep(pollInterval)
				continue
			}
			// Fatal error
			return fmt.Errorf("authentication failed: %w", err)
		}

		// Success!
		break
	}

	// Step 5: Save tokens to global config
	config, err := LoadGlobalConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	config.SetHostCredentials(l.Host, HostCredentials{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
	})

	if err := SaveGlobalConfig(config); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}

	// Step 6: Verify authentication by calling /api/session
	baseURL := fmt.Sprintf("https://%s", l.Host)
	apiClient, err := NewAPIClient(baseURL)
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	resp, err := apiClient.Get("/api/session")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to verify authentication: %v\n", err)
		fmt.Println("✓ Credentials saved, but could not verify with server")
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Fprintf(os.Stderr, "Warning: Server returned status %d\n", resp.StatusCode)
		fmt.Println("✓ Credentials saved, but could not verify with server")
		return nil
	}

	// Parse the session response to get user info
	var sessionResp struct {
		Authenticated bool `json:"authenticated"`
		User          *struct {
			Email string `json:"email"`
		} `json:"user"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&sessionResp); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to parse session response: %v\n", err)
		fmt.Println("✓ Successfully authenticated")
		return nil
	}

	if sessionResp.Authenticated && sessionResp.User != nil {
		fmt.Printf("✓ Successfully authenticated as %s\n", sessionResp.User.Email)
	} else {
		fmt.Println("✓ Successfully authenticated")
	}

	return nil
}
