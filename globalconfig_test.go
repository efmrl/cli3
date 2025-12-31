package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGlobalConfig(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "efmrl3-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Override the home directory for testing
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	// Test loading non-existent config (should return empty)
	config, err := LoadGlobalConfig()
	if err != nil {
		t.Fatalf("LoadGlobalConfig failed: %v", err)
	}
	if len(config.Hosts) != 0 {
		t.Errorf("Expected empty hosts, got %d hosts", len(config.Hosts))
	}

	// Test setting credentials for multiple hosts
	config.SetHostCredentials("efmrl.com", HostCredentials{
		AccessToken:  "token-efmrl-com",
		RefreshToken: "refresh-efmrl-com",
	})
	config.SetHostCredentials("efmrl.work", HostCredentials{
		AccessToken:  "token-efmrl-work",
		RefreshToken: "refresh-efmrl-work",
	})

	// Test saving config
	if err := SaveGlobalConfig(config); err != nil {
		t.Fatalf("SaveGlobalConfig failed: %v", err)
	}

	// Verify file exists with correct permissions
	configPath, _ := GetGlobalConfigPath()
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("Config file not found: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("Expected permissions 0600, got %o", info.Mode().Perm())
	}

	// Test loading saved config
	loadedConfig, err := LoadGlobalConfig()
	if err != nil {
		t.Fatalf("LoadGlobalConfig failed: %v", err)
	}

	// Verify credentials for efmrl.com
	creds, ok := loadedConfig.GetHostCredentials("efmrl.com")
	if !ok {
		t.Error("Expected to find credentials for efmrl.com")
	}
	if creds.AccessToken != "token-efmrl-com" {
		t.Errorf("Expected access token 'token-efmrl-com', got '%s'", creds.AccessToken)
	}
	if creds.RefreshToken != "refresh-efmrl-com" {
		t.Errorf("Expected refresh token 'refresh-efmrl-com', got '%s'", creds.RefreshToken)
	}

	// Verify credentials for efmrl.work
	creds, ok = loadedConfig.GetHostCredentials("efmrl.work")
	if !ok {
		t.Error("Expected to find credentials for efmrl.work")
	}
	if creds.AccessToken != "token-efmrl-work" {
		t.Errorf("Expected access token 'token-efmrl-work', got '%s'", creds.AccessToken)
	}

	// Test deleting credentials
	loadedConfig.DeleteHostCredentials("efmrl.com")
	if _, ok := loadedConfig.GetHostCredentials("efmrl.com"); ok {
		t.Error("Expected credentials for efmrl.com to be deleted")
	}
	if _, ok := loadedConfig.GetHostCredentials("efmrl.work"); !ok {
		t.Error("Expected credentials for efmrl.work to still exist")
	}

	// Test GetGlobalConfigPath
	expectedPath := filepath.Join(tempDir, ".config", "efmrl3", "credentials.toml")
	actualPath, err := GetGlobalConfigPath()
	if err != nil {
		t.Fatalf("GetGlobalConfigPath failed: %v", err)
	}
	if actualPath != expectedPath {
		t.Errorf("Expected path '%s', got '%s'", expectedPath, actualPath)
	}
}
