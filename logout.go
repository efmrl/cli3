package main

import (
	"fmt"
)

// LogoutCmd handles clearing authentication credentials
type LogoutCmd struct {
	Host string `help:"Server host (defaults to base_host from efmrl.toml or tempemail.app)" default:""`
	All  bool   `help:"Remove credentials for all hosts" default:"false"`
}

// Run executes the logout command
func (l *LogoutCmd) Run() error {
	// Determine which host to use
	host := l.Host
	if host == "" && !l.All {
		// Try to load efmrl.toml from current directory
		config, err := LoadConfig()
		if err == nil && config.BaseHost != "" {
			host = config.BaseHost
		} else {
			host = DefaultBaseHost
		}
	}

	// Load global config
	config, err := LoadGlobalConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if l.All {
		// Remove all credentials
		if len(config.Hosts) == 0 {
			fmt.Println("No credentials to remove")
			return nil
		}

		count := len(config.Hosts)
		config.Hosts = make(map[string]HostCredentials)

		if err := SaveGlobalConfig(config); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Printf("✓ Removed credentials for %d host(s)\n", count)
		return nil
	}

	// Remove credentials for specific host
	_, ok := config.GetHostCredentials(host)
	if !ok {
		fmt.Printf("No credentials found for %s\n", host)
		return nil
	}

	config.DeleteHostCredentials(host)

	if err := SaveGlobalConfig(config); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("✓ Logged out from %s\n", host)
	return nil
}
