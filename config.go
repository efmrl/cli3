package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const ConfigFileName = "efmrl.toml"
const DefaultBaseHost = "tempemail.app"

type Config struct {
	BaseHost string     `toml:"base_host,omitempty"`
	Site     SiteConfig `toml:"site"`
}

type SiteConfig struct {
	SiteID string `toml:"site_id"`
	Dir    string `toml:"dir,omitempty"`
}

// LoadConfig loads the efmrl.toml config file from the current directory
func LoadConfig() (*Config, error) {
	configPath := filepath.Join(".", ConfigFileName)

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("no %s file found in current directory", ConfigFileName)
	}

	var config Config
	if _, err := toml.DecodeFile(configPath, &config); err != nil {
		return nil, fmt.Errorf("error parsing %s: %w", ConfigFileName, err)
	}

	return &config, nil
}

// LoadConfigOrDefault loads the config file, or returns a default config if it doesn't exist
func LoadConfigOrDefault() (*Config, error) {
	config, err := LoadConfig()
	if err != nil {
		// Return default config
		return &Config{
			BaseHost: DefaultBaseHost,
			Site:     SiteConfig{},
		}, nil
	}
	return config, nil
}

// SaveConfig saves the config to the efmrl.toml file in the current directory
func SaveConfig(config *Config) error {
	configPath := filepath.Join(".", ConfigFileName)

	file, err := os.Create(configPath)
	if err != nil {
		return fmt.Errorf("error creating %s: %w", ConfigFileName, err)
	}
	defer file.Close()

	encoder := toml.NewEncoder(file)
	if err := encoder.Encode(config); err != nil {
		return fmt.Errorf("error writing %s: %w", ConfigFileName, err)
	}

	return nil
}

// GetBaseHost returns the base host from config, or the default if not set
func (c *Config) GetBaseHost() string {
	if c.BaseHost == "" {
		return DefaultBaseHost
	}
	return c.BaseHost
}

type ConfigCmd struct {
	ID       string `help:"Set the site ID"`
	Dir      string `help:"Set the directory to sync"`
	BaseHost string `hidden:"" help:"Set the base host for the efmrl server"`
}

func (c *ConfigCmd) Run() error {
	// Load existing config or create default
	config, err := LoadConfigOrDefault()
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	// Track if any changes were made
	changed := false

	// Update ID if provided
	if c.ID != "" {
		config.Site.SiteID = c.ID
		changed = true
	}

	// Update Dir if provided
	if c.Dir != "" {
		config.Site.Dir = c.Dir
		changed = true
	}

	// Update BaseHost if provided
	if c.BaseHost != "" {
		config.BaseHost = c.BaseHost
		changed = true
	}

	// If no flags were provided, just display current config
	if !changed {
		fmt.Println("Current Configuration")
		fmt.Println("=====================")
		fmt.Printf("Site ID:   %s\n", config.Site.SiteID)
		fmt.Printf("Dir:       %s\n", config.Site.Dir)
		fmt.Printf("Base Host: %s\n", config.GetBaseHost())
		fmt.Printf("\nConfig file: %s\n", ConfigFileName)
		return nil
	}

	// Save the updated config
	if err := SaveConfig(config); err != nil {
		return err
	}

	fmt.Printf("Configuration saved to %s\n", ConfigFileName)
	if c.ID != "" {
		fmt.Printf("  Site ID set to: %s\n", c.ID)
	}
	if c.Dir != "" {
		fmt.Printf("  Dir set to: %s\n", c.Dir)
	}
	if c.BaseHost != "" {
		fmt.Printf("  Base host set to: %s\n", c.BaseHost)
	}

	return nil
}
