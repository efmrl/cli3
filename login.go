package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/pkg/browser"
)

// LoginCmd handles user authentication
type LoginCmd struct {
	Host     string `help:"Server host (defaults to base_host from efmrl.toml or efmrl.work)" default:""`
	Provider string `help:"Auth provider to use" default:"google" enum:"google,workos"`
}

// Run executes the login command
func (l *LoginCmd) Run() error {
	// Determine which host to use
	host := l.Host
	if host == "" {
		config, err := LoadConfig()
		if err == nil && config.BaseHost != "" {
			host = config.BaseHost
			fmt.Printf("Using base_host from efmrl.toml: %s\n", host)
		} else {
			host = DefaultBaseHost
		}
	}

	switch l.Provider {
	case "google":
		return l.loginWithGoogle(host)
	default:
		return l.loginWithWorkOS(host)
	}
}

func (l *LoginCmd) loginWithGoogle(host string) error {
	fmt.Println("Authenticating with efmrl via Google...")

	clientID := getGoogleClientID()
	clientSecret := getGoogleClientSecret()

	// Step 1: Request device code
	deviceCode, err := RequestGoogleDeviceCode(clientID)
	if err != nil {
		return fmt.Errorf("failed to initiate Google device authorization: %w", err)
	}

	// Step 2: Display instructions
	fmt.Println()
	fmt.Println("Please authenticate by visiting:")
	fmt.Printf("  %s\n", deviceCode.VerificationURL)
	fmt.Println()
	fmt.Printf("And entering code: %s\n", deviceCode.UserCode)
	fmt.Println()

	// Step 3: Auto-open browser
	fmt.Println("Opening browser automatically...")
	if err := browser.OpenURL(deviceCode.VerificationURL); err != nil {
		fmt.Fprintf(os.Stderr, "Could not open browser automatically: %v\n", err)
		fmt.Fprintf(os.Stderr, "Please visit the URL above manually.\n")
	}

	fmt.Println()
	fmt.Println("Waiting for authentication... (press Ctrl+C to cancel)")

	// Step 4: Poll for token
	pollInterval := time.Duration(deviceCode.Interval) * time.Second
	if pollInterval < 5*time.Second {
		pollInterval = 5 * time.Second
	}
	expiresAt := time.Now().Add(time.Duration(deviceCode.ExpiresIn) * time.Second)

	var tokenResp *GoogleTokenResponse
	for {
		if time.Now().After(expiresAt) {
			return fmt.Errorf("device code expired, please try again")
		}

		tokenResp, err = PollGoogleDeviceAuth(clientID, clientSecret, deviceCode.DeviceCode)
		if err != nil {
			if IsPollError(err) {
				pollErr := err.(*PollError)
				if pollErr.Type == "slow_down" {
					pollInterval += 5 * time.Second
					fmt.Fprintln(os.Stderr, "Slowing down polling...")
				}
				time.Sleep(pollInterval)
				continue
			}
			return fmt.Errorf("authentication failed: %w", err)
		}
		break
	}

	if tokenResp.IDToken == "" {
		return fmt.Errorf("Google did not return an ID token")
	}

	// Step 5: Save credentials — store id_token as the bearer token sent to our API
	globalConfig, err := LoadGlobalConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	globalConfig.SetHostCredentials(host, HostCredentials{
		AccessToken:  tokenResp.IDToken, // JWT with iss=accounts.google.com
		RefreshToken: tokenResp.RefreshToken,
		Provider:     "google",
	})

	if err := SaveGlobalConfig(globalConfig); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}

	return verifyAndPrint(host)
}

func (l *LoginCmd) loginWithWorkOS(host string) error {
	fmt.Println("Authenticating with efmrl via WorkOS...")

	clientID := getWorkOSClientID()

	// Step 1: Request device code from WorkOS
	deviceCode, err := RequestDeviceCode(clientID)
	if err != nil {
		return fmt.Errorf("failed to initiate device authorization: %w", err)
	}

	// Step 2: Display instructions
	fmt.Println()
	fmt.Println("Please authenticate by visiting:")
	fmt.Printf("  %s\n", deviceCode.VerificationURI)
	fmt.Println()
	fmt.Printf("And entering code: %s\n", deviceCode.UserCode)
	fmt.Println()

	// Step 3: Auto-open browser
	fmt.Println("Opening browser automatically...")
	verificationURL := deviceCode.VerificationURIComplete
	if verificationURL == "" {
		verificationURL = deviceCode.VerificationURI
	}
	if err := browser.OpenURL(verificationURL); err != nil {
		fmt.Fprintf(os.Stderr, "Could not open browser automatically: %v\n", err)
		fmt.Fprintf(os.Stderr, "Please visit the URL above manually.\n")
	}

	fmt.Println()
	fmt.Println("Waiting for authentication... (press Ctrl+C to cancel)")

	// Step 4: Poll for token
	pollInterval := time.Duration(deviceCode.Interval) * time.Second
	if pollInterval < 5*time.Second {
		pollInterval = 5 * time.Second
	}
	expiresAt := time.Now().Add(time.Duration(deviceCode.ExpiresIn) * time.Second)

	var tokenResp *TokenResponse
	for {
		if time.Now().After(expiresAt) {
			return fmt.Errorf("device code expired, please try again")
		}

		tokenResp, err = PollForToken(clientID, deviceCode.DeviceCode, deviceCode.Interval)
		if err != nil {
			if IsPollError(err) {
				pollErr := err.(*PollError)
				if pollErr.Type == "slow_down" {
					pollInterval += 5 * time.Second
					fmt.Fprintln(os.Stderr, "Slowing down polling...")
				}
				time.Sleep(pollInterval)
				continue
			}
			return fmt.Errorf("authentication failed: %w", err)
		}
		break
	}

	// Step 5: Save credentials
	globalConfig, err := LoadGlobalConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	globalConfig.SetHostCredentials(host, HostCredentials{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		Provider:     "workos",
	})

	if err := SaveGlobalConfig(globalConfig); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}

	return verifyAndPrint(host)
}

// verifyAndPrint confirms authentication by calling /api/session and prints the result.
func verifyAndPrint(host string) error {
	baseURL := fmt.Sprintf("https://%s", host)
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
