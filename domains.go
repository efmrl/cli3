package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// DomainsCmd manages domains for an efmrl
type DomainsCmd struct {
	List   DomainsListCmd   `cmd:"" help:"List all domains"`
	Add    DomainsAddCmd    `cmd:"" help:"Add one or more domains"`
	Remove DomainsRemoveCmd `cmd:"" help:"Remove one or more domains"`
}

// DomainsListCmd lists all domains for the configured efmrl
type DomainsListCmd struct{}

func (d *DomainsListCmd) Run() error {
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if config.Site.SiteID == "" {
		return fmt.Errorf("no site_id configured")
	}

	// Create API client
	baseURL := fmt.Sprintf("https://%s", config.GetBaseHost())
	apiClient, err := NewAPIClient(baseURL)
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	// Fetch domains
	resp, err := apiClient.Get(fmt.Sprintf("/admin/efmrls/%s/domains", config.Site.SiteID))
	if err != nil {
		return fmt.Errorf("failed to fetch domains: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Domains []struct {
			ID       int    `json:"id"`
			EfmrlID  string `json:"efmrl_id"`
			Domain   string `json:"domain"`
		} `json:"domains"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(result.Domains) == 0 {
		fmt.Println("No domains configured")
		return nil
	}

	fmt.Printf("Domains (%d):\n", len(result.Domains))
	for _, domain := range result.Domains {
		fmt.Printf("  %s\n", domain.Domain)
	}

	return nil
}

// DomainsAddCmd adds one or more domains
type DomainsAddCmd struct {
	Domains []string `arg:"" name:"domain" help:"Domain(s) to add" required:""`
}

func (d *DomainsAddCmd) Run() error {
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if config.Site.SiteID == "" {
		return fmt.Errorf("no site_id configured")
	}

	// Create API client
	baseURL := fmt.Sprintf("https://%s", config.GetBaseHost())
	apiClient, err := NewAPIClient(baseURL)
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	// Add each domain
	for _, domain := range d.Domains {
		fmt.Printf("Adding %s... ", domain)

		body := map[string]string{"domain": domain}
		resp, err := apiClient.Post(fmt.Sprintf("/admin/efmrls/%s/domains", config.Site.SiteID), body)
		if err != nil {
			fmt.Printf("FAILED\n")
			return fmt.Errorf("failed to add domain %s: %w", domain, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			fmt.Printf("FAILED\n")
			return fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(respBody))
		}

		fmt.Printf("OK\n")
	}

	fmt.Printf("\n✓ Added %d domain(s)\n", len(d.Domains))
	return nil
}

// DomainsRemoveCmd removes one or more domains
type DomainsRemoveCmd struct {
	Domains []string `arg:"" name:"domain" help:"Domain(s) to remove" required:""`
}

func (d *DomainsRemoveCmd) Run() error {
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if config.Site.SiteID == "" {
		return fmt.Errorf("no site_id configured")
	}

	// Create API client
	baseURL := fmt.Sprintf("https://%s", config.GetBaseHost())
	apiClient, err := NewAPIClient(baseURL)
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	// First, fetch all domains to find their IDs
	resp, err := apiClient.Get(fmt.Sprintf("/admin/efmrls/%s/domains", config.Site.SiteID))
	if err != nil {
		return fmt.Errorf("failed to fetch domains: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	var listResult struct {
		Domains []struct {
			ID     int    `json:"id"`
			Domain string `json:"domain"`
		} `json:"domains"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&listResult); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Build a map of domain name to ID
	domainMap := make(map[string]int)
	for _, d := range listResult.Domains {
		domainMap[d.Domain] = d.ID
	}

	// Remove each domain
	for _, domain := range d.Domains {
		fmt.Printf("Removing %s... ", domain)

		domainID, ok := domainMap[domain]
		if !ok {
			fmt.Printf("NOT FOUND\n")
			continue
		}

		resp, err := apiClient.Delete(fmt.Sprintf("/admin/efmrls/%s/domains/%d", config.Site.SiteID, domainID))
		if err != nil {
			fmt.Printf("FAILED\n")
			return fmt.Errorf("failed to remove domain %s: %w", domain, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			fmt.Printf("FAILED\n")
			return fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(respBody))
		}

		fmt.Printf("OK\n")
	}

	fmt.Printf("\n✓ Removed %d domain(s)\n", len(d.Domains))
	return nil
}
