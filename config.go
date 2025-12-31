package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const ConfigFileName = "efmrl.toml"

type Config struct {
	Site SiteConfig `toml:"site"`
}

type SiteConfig struct {
	Name   string `toml:"name"`
	SiteID string `toml:"site_id"`
	Domain string `toml:"domain"`
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
