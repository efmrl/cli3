package main

import (
	"fmt"
	"os"
)

type StatusCmd struct{}

func (s *StatusCmd) Run() error {
	config, err := LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
		fmt.Fprintf(os.Stderr, "Please navigate to a directory containing an %s file.\n", ConfigFileName)
		fmt.Fprintf(os.Stderr, "If this is your first time, run 'efmrl3 config' to set up initial configuration.\n")
		return fmt.Errorf("config file not found")
	}

	// Check login status
	baseHost := config.GetBaseHost()
	globalConfig, err := LoadGlobalConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not load credentials: %v\n", err)
	}

	var loggedIn bool
	if globalConfig != nil {
		_, loggedIn = globalConfig.GetHostCredentials(baseHost)
	}

	fmt.Println("Site Status")
	fmt.Println("===========")
	fmt.Printf("Name:      %s\n", config.Site.Name)
	fmt.Printf("Site ID:   %s\n", config.Site.SiteID)
	fmt.Printf("Domain:    %s\n", config.Site.Domain)
	fmt.Printf("Base Host: %s\n", baseHost)
	fmt.Printf("Logged in: %v\n", loggedIn)

	return nil
}
