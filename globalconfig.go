package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const GlobalConfigDir = ".config/efmrl3"
const GlobalConfigFileName = "credentials.toml"

// GlobalConfig stores credentials for multiple hosts
type GlobalConfig struct {
	Hosts map[string]HostCredentials `toml:"host"`
}

// HostCredentials stores authentication credentials for a specific host
type HostCredentials struct {
	AccessToken  string `toml:"access_token,omitempty"`
	RefreshToken string `toml:"refresh_token,omitempty"`
}

// GetGlobalConfigPath returns the path to the global config file
func GetGlobalConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("error getting home directory: %w", err)
	}
	return filepath.Join(homeDir, GlobalConfigDir, GlobalConfigFileName), nil
}

// LoadGlobalConfig loads the global config file
func LoadGlobalConfig() (*GlobalConfig, error) {
	configPath, err := GetGlobalConfigPath()
	if err != nil {
		return nil, err
	}

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Return empty config if file doesn't exist
		return &GlobalConfig{
			Hosts: make(map[string]HostCredentials),
		}, nil
	}

	var config GlobalConfig
	if _, err := toml.DecodeFile(configPath, &config); err != nil {
		return nil, fmt.Errorf("error parsing %s: %w", configPath, err)
	}

	// Initialize map if nil
	if config.Hosts == nil {
		config.Hosts = make(map[string]HostCredentials)
	}

	return &config, nil
}

// SaveGlobalConfig saves the global config file with secure permissions
func SaveGlobalConfig(config *GlobalConfig) error {
	configPath, err := GetGlobalConfigPath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("error creating config directory: %w", err)
	}

	// Create or truncate the file
	file, err := os.OpenFile(configPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("error creating config file: %w", err)
	}
	defer file.Close()

	// Encode to TOML
	encoder := toml.NewEncoder(file)
	if err := encoder.Encode(config); err != nil {
		return fmt.Errorf("error writing config file: %w", err)
	}

	return nil
}

// GetHostCredentials retrieves credentials for a specific host
func (gc *GlobalConfig) GetHostCredentials(host string) (HostCredentials, bool) {
	creds, ok := gc.Hosts[host]
	return creds, ok
}

// SetHostCredentials sets credentials for a specific host
func (gc *GlobalConfig) SetHostCredentials(host string, creds HostCredentials) {
	if gc.Hosts == nil {
		gc.Hosts = make(map[string]HostCredentials)
	}
	gc.Hosts[host] = creds
}

// DeleteHostCredentials removes credentials for a specific host
func (gc *GlobalConfig) DeleteHostCredentials(host string) {
	delete(gc.Hosts, host)
}
