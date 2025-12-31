package main

import (
	"fmt"
)

// LogoutCmd handles clearing authentication credentials
type LogoutCmd struct {
	Host string `help:"Server host" default:"efmrl.samf.workers.dev"`
	All  bool   `help:"Remove credentials for all hosts" default:"false"`
}

// Run executes the logout command
func (l *LogoutCmd) Run() error {
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
	_, ok := config.GetHostCredentials(l.Host)
	if !ok {
		fmt.Printf("No credentials found for %s\n", l.Host)
		return nil
	}

	config.DeleteHostCredentials(l.Host)

	if err := SaveGlobalConfig(config); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("✓ Logged out from %s\n", l.Host)
	return nil
}
