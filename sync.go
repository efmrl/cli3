package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// SyncCmd synchronizes local files with the remote efmrl site
type SyncCmd struct {
	DryRun bool `help:"Show what would be synced without making changes" short:"n"`
	Force  bool `help:"Force upload all files, ignoring ETags" short:"f"`
	Delete bool `help:"Delete remote files not present locally" default:"true" negatable:""`
}

// RemoteFile represents a file on the server
type RemoteFile struct {
	Path     string `json:"path"`
	ETag     string `json:"etag"`
	Size     int64  `json:"size"`
	Uploaded string `json:"uploaded"`
}

// LocalFile represents a file on the local filesystem
type LocalFile struct {
	Path        string // Relative path with leading slash (e.g., "/index.html")
	AbsPath     string // Absolute filesystem path
	ETag        string // MD5 hex hash
	Size        int64
	ContentType string
}

// SyncPlan describes what operations will be performed
type SyncPlan struct {
	ToUpload  []LocalFile
	ToDelete  []RemoteFile
	Unchanged []string
}

// QuotaInfo represents quota information for an efmrl
type QuotaInfo struct {
	CurrentSpace   int64 `json:"currentSpace"`
	MaxSpace       int64 `json:"maxSpace"`
	AvailableSpace int64 `json:"availableSpace"`
}

func (s *SyncCmd) Run() error {
	// 1. Load configuration
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if config.Site.SiteID == "" {
		return fmt.Errorf("no site_id configured (run 'efmrl3 config --id <site-id>')")
	}

	// Determine the directory to sync
	syncDir := config.Site.Dir
	if syncDir == "" {
		syncDir = "." // Default to current directory
	}

	// Convert to absolute path
	absDir, err := filepath.Abs(syncDir)
	if err != nil {
		return fmt.Errorf("failed to resolve directory path: %w", err)
	}

	// Verify directory exists
	if info, err := os.Stat(absDir); err != nil || !info.IsDir() {
		return fmt.Errorf("sync directory does not exist: %s", syncDir)
	}

	fmt.Printf("Syncing directory: %s\n", absDir)
	fmt.Printf("Site ID: %s\n", config.Site.SiteID)
	fmt.Println()

	// 2. Scan local files
	fmt.Println("Scanning local files...")
	localFiles, err := scanLocalFiles(absDir)
	if err != nil {
		return fmt.Errorf("failed to scan local files: %w", err)
	}
	fmt.Printf("Found %d local file(s)\n\n", len(localFiles))

	// 3. Check quota before syncing
	fmt.Println("Checking quota...")
	baseURL := fmt.Sprintf("https://%s", config.GetBaseHost())
	apiClient, err := NewAPIClient(baseURL)
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	quota, err := fetchQuota(apiClient, config.Site.SiteID)
	if err != nil {
		return fmt.Errorf("failed to fetch quota: %w", err)
	}

	if err := validateQuota(localFiles, quota); err != nil {
		return err
	}
	fmt.Printf("Quota check passed (local: %s, quota: %s)\n\n",
		formatBytes(calculateTotalSize(localFiles)),
		formatBytes(quota.MaxSpace))

	// 4. Fetch remote file list
	fmt.Println("Fetching remote file list...")
	remoteFiles, err := fetchRemoteFiles(apiClient, config.Site.SiteID)
	if err != nil {
		return fmt.Errorf("failed to fetch remote files: %w", err)
	}
	fmt.Printf("Found %d remote file(s)\n\n", len(remoteFiles))

	// 5. Compute sync plan
	plan := computeSyncPlan(localFiles, remoteFiles, s.Force, s.Delete)

	// 6. Display plan
	fmt.Println("Sync Plan")
	fmt.Println("=========")
	if len(plan.ToUpload) > 0 {
		fmt.Printf("Files to upload: %d\n", len(plan.ToUpload))
		for _, f := range plan.ToUpload {
			fmt.Printf("  + %s\n", f.Path)
		}
		fmt.Println()
	}

	if len(plan.ToDelete) > 0 {
		fmt.Printf("Files to delete: %d\n", len(plan.ToDelete))
		for _, f := range plan.ToDelete {
			fmt.Printf("  - %s\n", f.Path)
		}
		fmt.Println()
	}

	if len(plan.Unchanged) > 0 {
		fmt.Printf("Files unchanged: %d\n", len(plan.Unchanged))
	}

	if len(plan.ToUpload) == 0 && len(plan.ToDelete) == 0 {
		fmt.Println("✓ Everything is up to date")
		return nil
	}

	// 7. Execute plan (or exit if dry-run)
	if s.DryRun {
		fmt.Println("\n--dry-run mode: no changes made")
		return nil
	}

	fmt.Println()
	return executeSyncPlan(apiClient, config.Site.SiteID, plan)
}

// scanLocalFiles walks the directory tree and computes ETags for all files
func scanLocalFiles(rootDir string) ([]LocalFile, error) {
	var files []LocalFile

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Skip hidden files and directories (starting with .)
		relPath, err := filepath.Rel(rootDir, path)
		if err != nil {
			return err
		}

		// Check if any component of the path starts with .
		parts := strings.Split(relPath, string(filepath.Separator))
		for _, part := range parts {
			if strings.HasPrefix(part, ".") {
				// If it's a directory, skip the entire subtree
				if info.IsDir() {
					return filepath.SkipDir
				}
				// If it's a file, skip just this file
				return nil
			}
		}

		// Compute ETag (MD5 hash)
		etag, err := computeFileETag(path)
		if err != nil {
			return fmt.Errorf("failed to compute ETag for %s: %w", relPath, err)
		}

		// Convert to URL path (with leading slash, forward slashes)
		urlPath := "/" + filepath.ToSlash(relPath)

		// Detect content type
		contentType := detectContentType(path)

		files = append(files, LocalFile{
			Path:        urlPath,
			AbsPath:     path,
			ETag:        etag,
			Size:        info.Size(),
			ContentType: contentType,
		})

		return nil
	})

	return files, err
}

// computeFileETag computes the MD5 hash of a file (matching R2's ETag format)
func computeFileETag(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	// Return unquoted hex string (matching R2's etag field)
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// detectContentType determines the MIME type of a file based on extension
func detectContentType(path string) string {
	ext := filepath.Ext(path)

	// Try Go's built-in MIME type detection first
	if mimeType := mime.TypeByExtension(ext); mimeType != "" {
		return mimeType
	}

	// Fallback to application/octet-stream
	return "application/octet-stream"
}

// fetchRemoteFiles retrieves the list of files from the server
func fetchRemoteFiles(client *APIClient, siteID string) ([]RemoteFile, error) {
	resp, err := client.Get(fmt.Sprintf("/admin/efmrls/%s/files", siteID))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Files []RemoteFile `json:"files"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result.Files, nil
}

// fetchQuota retrieves quota information from the server
func fetchQuota(client *APIClient, siteID string) (*QuotaInfo, error) {
	resp, err := client.Get(fmt.Sprintf("/admin/efmrls/%s/quota", siteID))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	var quota QuotaInfo
	if err := json.NewDecoder(resp.Body).Decode(&quota); err != nil {
		return nil, fmt.Errorf("failed to parse quota response: %w", err)
	}

	return &quota, nil
}

// validateQuota checks if the local files will fit within the efmrl's quota
func validateQuota(localFiles []LocalFile, quota *QuotaInfo) error {
	// Calculate total size of local files
	var totalLocalSize int64
	for _, lf := range localFiles {
		totalLocalSize += lf.Size
	}

	// Check if total local size exceeds max quota
	if totalLocalSize > quota.MaxSpace {
		return fmt.Errorf(
			"local directory size (%s) exceeds efmrl quota (%s)",
			formatBytes(totalLocalSize),
			formatBytes(quota.MaxSpace),
		)
	}

	return nil
}

// formatBytes formats a byte count as a human-readable string
func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d bytes", bytes)
	}
}

// calculateTotalSize calculates the total size of all local files
func calculateTotalSize(files []LocalFile) int64 {
	var total int64
	for _, f := range files {
		total += f.Size
	}
	return total
}

// computeSyncPlan determines which files need to be uploaded or deleted
func computeSyncPlan(local []LocalFile, remote []RemoteFile, force bool, deleteRemote bool) SyncPlan {
	plan := SyncPlan{
		ToUpload:  []LocalFile{},
		ToDelete:  []RemoteFile{},
		Unchanged: []string{},
	}

	// Build a map of remote files for quick lookup
	remoteMap := make(map[string]RemoteFile)
	for _, rf := range remote {
		remoteMap[rf.Path] = rf
	}

	// Check each local file
	for _, lf := range local {
		rf, existsRemote := remoteMap[lf.Path]

		if !existsRemote || force || lf.ETag != rf.ETag {
			// File doesn't exist remotely, or --force flag, or ETags differ
			plan.ToUpload = append(plan.ToUpload, lf)
		} else {
			// File exists and ETags match
			plan.Unchanged = append(plan.Unchanged, lf.Path)
		}

		// Remove from remote map (we've processed it)
		delete(remoteMap, lf.Path)
	}

	// Remaining remote files should be deleted (if --delete flag is set)
	if deleteRemote {
		for _, rf := range remoteMap {
			plan.ToDelete = append(plan.ToDelete, rf)
		}
	}

	return plan
}

// executeSyncPlan performs the delete and upload operations
func executeSyncPlan(client *APIClient, siteID string, plan SyncPlan) error {
	totalOps := len(plan.ToUpload) + len(plan.ToDelete)
	currentOp := 0

	// Delete files first to free up space
	for _, rf := range plan.ToDelete {
		currentOp++
		fmt.Printf("[%d/%d] Deleting %s... ", currentOp, totalOps, rf.Path)

		if err := deleteFile(client, siteID, rf.Path); err != nil {
			fmt.Printf("FAILED\n")
			return fmt.Errorf("failed to delete %s: %w", rf.Path, err)
		}

		fmt.Printf("OK\n")
	}

	// Upload files after deletes complete
	for _, lf := range plan.ToUpload {
		currentOp++
		fmt.Printf("[%d/%d] Uploading %s... ", currentOp, totalOps, lf.Path)

		if err := uploadFile(client, siteID, lf); err != nil {
			fmt.Printf("FAILED\n")
			return fmt.Errorf("failed to upload %s: %w", lf.Path, err)
		}

		fmt.Printf("OK\n")
	}

	fmt.Println("\n✓ Sync complete")
	return nil
}

// uploadFile uploads a single file to the server
func uploadFile(client *APIClient, siteID string, file LocalFile) error {
	// Open the file
	f, err := os.Open(file.AbsPath)
	if err != nil {
		return err
	}
	defer f.Close()

	// Create the request
	url := fmt.Sprintf("%s/admin/efmrls/%s/files%s", client.BaseURL, siteID, file.Path)
	req, err := http.NewRequest("PUT", url, f)
	if err != nil {
		return err
	}

	// Set Content-Type
	req.Header.Set("Content-Type", file.ContentType)

	// Get access token
	accessToken, err := client.getAccessToken()
	if err != nil {
		return err
	}

	// Add Authorization header
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	// Send request
	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Handle 401 with token refresh (similar to APIClient.doRequest)
	if resp.StatusCode == http.StatusUnauthorized {
		// Try to refresh token
		if err := client.refreshTokenIfNeeded(); err != nil {
			return fmt.Errorf("failed to refresh credentials: %w", err)
		}

		// Retry with new token
		accessToken, err = client.getAccessToken()
		if err != nil {
			return err
		}

		// Reopen file (previous one was consumed)
		f.Close()
		f, err = os.Open(file.AbsPath)
		if err != nil {
			return err
		}
		defer f.Close()

		req.Body = f
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))

		resp, err = httpClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// deleteFile deletes a single file from the server
func deleteFile(client *APIClient, siteID string, path string) error {
	url := fmt.Sprintf("/admin/efmrls/%s/files%s", siteID, path)
	resp, err := client.Delete(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
