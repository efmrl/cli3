package main

import (
	"encoding/json"
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

	// Fetch efmrl details from server if logged in and we have a site ID
	var efmrlName string
	var efmrlDomains []string
	var efmrlNotFound bool
	if loggedIn && config.Site.SiteID != "" {
		baseURL := fmt.Sprintf("https://%s", baseHost)
		apiClient, err := NewAPIClient(baseURL)
		if err == nil {
			// Fetch efmrl details (name, etc.)
			resp, err := apiClient.Get(fmt.Sprintf("/admin/efmrls/%s", config.Site.SiteID))
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == 200 {
					var efmrlResp struct {
						Efmrl struct {
							Name string `json:"name"`
						} `json:"efmrl"`
					}
					if err := json.NewDecoder(resp.Body).Decode(&efmrlResp); err == nil {
						efmrlName = efmrlResp.Efmrl.Name
					}
				} else if resp.StatusCode == 404 {
					efmrlNotFound = true
				}
			}

			// Fetch domains separately (only if efmrl was found)
			if !efmrlNotFound {
				resp2, err := apiClient.Get(fmt.Sprintf("/admin/efmrls/%s/domains", config.Site.SiteID))
				if err == nil {
					defer resp2.Body.Close()
					if resp2.StatusCode == 200 {
						var domainsResp struct {
							Domains []struct {
								Domain string `json:"domain"`
							} `json:"domains"`
						}
						if err := json.NewDecoder(resp2.Body).Decode(&domainsResp); err == nil {
							for _, d := range domainsResp.Domains {
								efmrlDomains = append(efmrlDomains, d.Domain)
							}
						}
					}
				}
			}
		}
	}

	fmt.Println("Site Status")
	fmt.Println("===========")
	if efmrlNotFound {
		fmt.Fprintf(os.Stderr, "\nWARNING: Efmrl with this ID was not found or you no longer have access.\n")
		fmt.Fprintf(os.Stderr, "         It may have been deleted or you may have been removed from the pod.\n\n")
	}
	if efmrlName != "" {
		fmt.Printf("Name:      %s\n", efmrlName)
	}
	fmt.Printf("Site ID:   %s\n", config.Site.SiteID)
	if len(efmrlDomains) > 0 {
		if len(efmrlDomains) == 1 {
			fmt.Printf("Domain:    %s\n", efmrlDomains[0])
		} else {
			fmt.Printf("Domains:   %s\n", efmrlDomains[0])
			for _, domain := range efmrlDomains[1:] {
				fmt.Printf("           %s\n", domain)
			}
		}
	}
	fmt.Printf("Dir:       %s\n", config.Site.Dir)
	fmt.Printf("Base Host: %s\n", baseHost)
	fmt.Printf("Logged in: %v\n", loggedIn)

	return nil
}
