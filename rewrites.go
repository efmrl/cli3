package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// RewritesCmd manages rewrites for an efmrl
type RewritesCmd struct {
	List   RewritesListCmd   `cmd:"" help:"List all rewrites"`
	Add    RewritesAddCmd    `cmd:"" help:"Add one or more rewrites"`
	Remove RewritesRemoveCmd `cmd:"" help:"Remove one or more rewrites"`
}

// RewritesListCmd lists all rewrites for the configured efmrl
type RewritesListCmd struct{}

func (r *RewritesListCmd) Run() error {
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

	// Fetch rewrites
	resp, err := apiClient.Get(fmt.Sprintf("/admin/efmrls/%s/rewrites", config.Site.SiteID))
	if err != nil {
		return fmt.Errorf("failed to fetch rewrites: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Rewrites []struct {
			ID       int    `json:"id"`
			Filename string `json:"filename"`
		} `json:"rewrites"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(result.Rewrites) == 0 {
		fmt.Println("No rewrites configured")
		return nil
	}

	fmt.Printf("Rewrites (%d):\n", len(result.Rewrites))
	for _, rewrite := range result.Rewrites {
		fmt.Printf("  %s\n", rewrite.Filename)
	}

	return nil
}

// RewritesAddCmd adds one or more rewrites
type RewritesAddCmd struct {
	Filenames []string `arg:"" name:"filename" help:"Filename(s) to add" required:""`
}

func (r *RewritesAddCmd) Run() error {
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

	// Add each rewrite
	for _, filename := range r.Filenames {
		fmt.Printf("Adding %s... ", filename)

		body := map[string]string{"filename": filename}
		resp, err := apiClient.Post(fmt.Sprintf("/admin/efmrls/%s/rewrites", config.Site.SiteID), body)
		if err != nil {
			fmt.Printf("FAILED\n")
			return fmt.Errorf("failed to add rewrite %s: %w", filename, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			fmt.Printf("FAILED\n")
			return fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(respBody))
		}

		fmt.Printf("OK\n")
	}

	fmt.Printf("\n✓ Added %d rewrite(s)\n", len(r.Filenames))
	return nil
}

// RewritesRemoveCmd removes one or more rewrites
type RewritesRemoveCmd struct {
	Filenames []string `arg:"" name:"filename" help:"Filename(s) to remove" required:""`
}

func (r *RewritesRemoveCmd) Run() error {
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

	// First, fetch all rewrites to find their IDs
	resp, err := apiClient.Get(fmt.Sprintf("/admin/efmrls/%s/rewrites", config.Site.SiteID))
	if err != nil {
		return fmt.Errorf("failed to fetch rewrites: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	var listResult struct {
		Rewrites []struct {
			ID       int    `json:"id"`
			Filename string `json:"filename"`
		} `json:"rewrites"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&listResult); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Build a map of filename to ID
	rewriteMap := make(map[string]int)
	for _, r := range listResult.Rewrites {
		rewriteMap[r.Filename] = r.ID
	}

	// Remove each rewrite
	for _, filename := range r.Filenames {
		fmt.Printf("Removing %s... ", filename)

		rewriteID, ok := rewriteMap[filename]
		if !ok {
			fmt.Printf("NOT FOUND\n")
			continue
		}

		resp, err := apiClient.Delete(fmt.Sprintf("/admin/efmrls/%s/rewrites/%d", config.Site.SiteID, rewriteID))
		if err != nil {
			fmt.Printf("FAILED\n")
			return fmt.Errorf("failed to remove rewrite %s: %w", filename, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			fmt.Printf("FAILED\n")
			return fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(respBody))
		}

		fmt.Printf("OK\n")
	}

	fmt.Printf("\n✓ Removed %d rewrite(s)\n", len(r.Filenames))
	return nil
}
