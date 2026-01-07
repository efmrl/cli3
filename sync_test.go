package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestComputeFileETag tests MD5 hash computation
func TestComputeFileETag(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "sync-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test 1: Empty file
	emptyFile := filepath.Join(tempDir, "empty.txt")
	if err := os.WriteFile(emptyFile, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create empty file: %v", err)
	}
	etag, err := computeFileETag(emptyFile)
	if err != nil {
		t.Errorf("computeFileETag failed for empty file: %v", err)
	}
	// MD5 of empty string
	expectedEmpty := "d41d8cd98f00b204e9800998ecf8427e"
	if etag != expectedEmpty {
		t.Errorf("Expected ETag %s for empty file, got %s", expectedEmpty, etag)
	}

	// Test 2: File with known content
	testFile := filepath.Join(tempDir, "test.txt")
	content := []byte("hello world")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	etag, err = computeFileETag(testFile)
	if err != nil {
		t.Errorf("computeFileETag failed: %v", err)
	}
	// MD5 of "hello world"
	expectedMD5 := "5eb63bbbe01eeed093cb22bb8f5acdc3"
	if etag != expectedMD5 {
		t.Errorf("Expected ETag %s, got %s", expectedMD5, etag)
	}

	// Test 3: Non-existent file
	_, err = computeFileETag(filepath.Join(tempDir, "nonexistent.txt"))
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}

// TestFormatBytes tests human-readable byte formatting
func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 bytes"},
		{1, "1 bytes"},
		{512, "512 bytes"},
		{1023, "1023 bytes"},
		{1024, "1.00 KB"},
		{1536, "1.50 KB"},
		{1024 * 1024, "1.00 MB"},
		{1024*1024 + 512*1024, "1.50 MB"},
		{1024 * 1024 * 1024, "1.00 GB"},
		{1024*1024*1024 + 512*1024*1024, "1.50 GB"},
		{5*1024*1024*1024 + 256*1024*1024, "5.25 GB"},
	}

	for _, tt := range tests {
		result := formatBytes(tt.bytes)
		if result != tt.expected {
			t.Errorf("formatBytes(%d) = %s, expected %s", tt.bytes, result, tt.expected)
		}
	}
}

// TestDetectContentType tests MIME type detection
func TestDetectContentType(t *testing.T) {
	tests := []struct {
		filename    string
		expectedCT  string
		description string
	}{
		{"/path/to/index.html", "text/html; charset=utf-8", "HTML file"},
		{"/path/to/style.css", "text/css; charset=utf-8", "CSS file"},
		{"/path/to/script.js", "text/javascript; charset=utf-8", "JavaScript file"},
		{"/path/to/data.json", "application/json", "JSON file"},
		{"/path/to/image.png", "image/png", "PNG image"},
		{"/path/to/image.jpg", "image/jpeg", "JPEG image"},
		{"/path/to/image.gif", "image/gif", "GIF image"},
		{"/path/to/document.pdf", "application/pdf", "PDF file"},
		{"/path/to/archive.zip", "application/zip", "ZIP file"},
		{"/path/to/font.woff", "font/woff", "WOFF font"},
		{"/path/to/font.woff2", "font/woff2", "WOFF2 font"},
		{"/path/to/noextension", "application/octet-stream", "No extension"},
		{"/path/to/unknown.unknownext", "application/octet-stream", "Unknown extension"},
	}

	for _, tt := range tests {
		result := detectContentType(tt.filename)
		if result != tt.expectedCT {
			t.Errorf("%s: detectContentType(%s) = %s, expected %s",
				tt.description, tt.filename, result, tt.expectedCT)
		}
	}
}

// TestCalculateTotalSize tests total size calculation
func TestCalculateTotalSize(t *testing.T) {
	tests := []struct {
		files    []LocalFile
		expected int64
	}{
		{
			files:    []LocalFile{},
			expected: 0,
		},
		{
			files: []LocalFile{
				{Path: "/file1.txt", Size: 100},
			},
			expected: 100,
		},
		{
			files: []LocalFile{
				{Path: "/file1.txt", Size: 100},
				{Path: "/file2.txt", Size: 200},
				{Path: "/file3.txt", Size: 300},
			},
			expected: 600,
		},
		{
			files: []LocalFile{
				{Path: "/large.bin", Size: 1024 * 1024 * 5}, // 5 MB
				{Path: "/small.txt", Size: 1024},            // 1 KB
			},
			expected: 1024*1024*5 + 1024,
		},
	}

	for _, tt := range tests {
		result := calculateTotalSize(tt.files)
		if result != tt.expected {
			t.Errorf("calculateTotalSize() = %d, expected %d", result, tt.expected)
		}
	}
}

// TestComputeSyncPlan tests sync plan computation
func TestComputeSyncPlan(t *testing.T) {
	// Test 1: Empty local and remote
	plan := computeSyncPlan([]LocalFile{}, []RemoteFile{}, false, false)
	if len(plan.ToUpload) != 0 || len(plan.ToDelete) != 0 || len(plan.Unchanged) != 0 {
		t.Errorf("Expected empty plan, got uploads=%d, deletes=%d, unchanged=%d",
			len(plan.ToUpload), len(plan.ToDelete), len(plan.Unchanged))
	}

	// Test 2: New local files (should upload)
	local := []LocalFile{
		{Path: "/index.html", ETag: "abc123"},
		{Path: "/style.css", ETag: "def456"},
	}
	plan = computeSyncPlan(local, []RemoteFile{}, false, false)
	if len(plan.ToUpload) != 2 {
		t.Errorf("Expected 2 uploads, got %d", len(plan.ToUpload))
	}
	if len(plan.ToDelete) != 0 {
		t.Errorf("Expected 0 deletes, got %d", len(plan.ToDelete))
	}

	// Test 3: Matching ETags (should be unchanged)
	remote := []RemoteFile{
		{Path: "/index.html", ETag: "abc123"},
		{Path: "/style.css", ETag: "def456"},
	}
	plan = computeSyncPlan(local, remote, false, false)
	if len(plan.ToUpload) != 0 {
		t.Errorf("Expected 0 uploads, got %d", len(plan.ToUpload))
	}
	if len(plan.Unchanged) != 2 {
		t.Errorf("Expected 2 unchanged, got %d", len(plan.Unchanged))
	}

	// Test 4: Changed ETags (should upload)
	remote = []RemoteFile{
		{Path: "/index.html", ETag: "old123"},
		{Path: "/style.css", ETag: "old456"},
	}
	plan = computeSyncPlan(local, remote, false, false)
	if len(plan.ToUpload) != 2 {
		t.Errorf("Expected 2 uploads, got %d", len(plan.ToUpload))
	}
	if len(plan.Unchanged) != 0 {
		t.Errorf("Expected 0 unchanged, got %d", len(plan.Unchanged))
	}

	// Test 5: Force flag (should upload even with matching ETags)
	remote = []RemoteFile{
		{Path: "/index.html", ETag: "abc123"},
		{Path: "/style.css", ETag: "def456"},
	}
	plan = computeSyncPlan(local, remote, true, false) // force=true
	if len(plan.ToUpload) != 2 {
		t.Errorf("Expected 2 uploads with force flag, got %d", len(plan.ToUpload))
	}
	if len(plan.Unchanged) != 0 {
		t.Errorf("Expected 0 unchanged with force flag, got %d", len(plan.Unchanged))
	}

	// Test 6: Remote files not in local (should delete with --delete flag)
	remote = []RemoteFile{
		{Path: "/index.html", ETag: "abc123"},
		{Path: "/style.css", ETag: "def456"},
		{Path: "/old.txt", ETag: "xyz789"},
	}
	plan = computeSyncPlan(local, remote, false, true) // deleteRemote=true
	if len(plan.ToDelete) != 1 {
		t.Errorf("Expected 1 delete, got %d", len(plan.ToDelete))
	}
	if plan.ToDelete[0].Path != "/old.txt" {
		t.Errorf("Expected to delete /old.txt, got %s", plan.ToDelete[0].Path)
	}

	// Test 7: Remote files not in local (should NOT delete without --delete flag)
	plan = computeSyncPlan(local, remote, false, false) // deleteRemote=false
	if len(plan.ToDelete) != 0 {
		t.Errorf("Expected 0 deletes without delete flag, got %d", len(plan.ToDelete))
	}

	// Test 8: Mixed scenario
	local = []LocalFile{
		{Path: "/index.html", ETag: "new123"},   // Changed
		{Path: "/style.css", ETag: "def456"},    // Unchanged
		{Path: "/newfile.js", ETag: "brand999"}, // New
	}
	remote = []RemoteFile{
		{Path: "/index.html", ETag: "old123"},
		{Path: "/style.css", ETag: "def456"},
		{Path: "/removed.txt", ETag: "gone000"},
	}
	plan = computeSyncPlan(local, remote, false, true)
	if len(plan.ToUpload) != 2 { // index.html (changed) + newfile.js (new)
		t.Errorf("Expected 2 uploads, got %d", len(plan.ToUpload))
	}
	if len(plan.Unchanged) != 1 { // style.css
		t.Errorf("Expected 1 unchanged, got %d", len(plan.Unchanged))
	}
	if len(plan.ToDelete) != 1 { // removed.txt
		t.Errorf("Expected 1 delete, got %d", len(plan.ToDelete))
	}
}

// TestScanLocalFiles tests directory scanning
func TestScanLocalFiles(t *testing.T) {
	// Create a temporary directory structure
	tempDir, err := os.MkdirTemp("", "sync-scan-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files
	files := map[string]string{
		"index.html":        "<!DOCTYPE html>",
		"style.css":         "body { margin: 0; }",
		"subdir/page.html":  "<html></html>",
		".hidden.txt":       "should be ignored",
		".git/config":       "should be ignored",
		"subdir/.DS_Store":  "should be ignored",
	}

	for path, content := range files {
		fullPath := filepath.Join(tempDir, path)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", path, err)
		}
	}

	// Scan the directory
	scanned, err := scanLocalFiles(tempDir)
	if err != nil {
		t.Fatalf("scanLocalFiles failed: %v", err)
	}

	// Should have 3 files (hidden files and .git ignored)
	if len(scanned) != 3 {
		t.Errorf("Expected 3 files, got %d", len(scanned))
		for _, f := range scanned {
			t.Logf("  Found: %s", f.Path)
		}
	}

	// Build a map for easier checking
	foundPaths := make(map[string]bool)
	for _, f := range scanned {
		foundPaths[f.Path] = true

		// Verify ETag is computed
		if f.ETag == "" {
			t.Errorf("File %s has empty ETag", f.Path)
		}

		// Verify Size is set
		if f.Size == 0 {
			t.Errorf("File %s has zero size", f.Path)
		}

		// Verify ContentType is set
		if f.ContentType == "" {
			t.Errorf("File %s has empty ContentType", f.Path)
		}

		// Verify AbsPath is absolute
		if !filepath.IsAbs(f.AbsPath) {
			t.Errorf("File %s has non-absolute AbsPath: %s", f.Path, f.AbsPath)
		}
	}

	// Check expected files are present
	expectedPaths := []string{"/index.html", "/style.css", "/subdir/page.html"}
	for _, expected := range expectedPaths {
		if !foundPaths[expected] {
			t.Errorf("Expected to find %s, but it was not scanned", expected)
		}
	}

	// Check hidden files are NOT present
	hiddenPaths := []string{"/.hidden.txt", "/.git/config", "/subdir/.DS_Store"}
	for _, hidden := range hiddenPaths {
		if foundPaths[hidden] {
			t.Errorf("Found hidden file %s, should have been skipped", hidden)
		}
	}
}

// TestValidateQuota tests quota validation
func TestValidateQuota(t *testing.T) {
	// Test 1: Under quota
	localFiles := []LocalFile{
		{Path: "/file1.txt", Size: 1024 * 1024},      // 1 MB
		{Path: "/file2.txt", Size: 2 * 1024 * 1024},  // 2 MB
	}
	quota := &QuotaInfo{
		MaxSpace: 10 * 1024 * 1024, // 10 MB
	}
	err := validateQuota(localFiles, quota)
	if err != nil {
		t.Errorf("Expected no error for files under quota, got: %v", err)
	}

	// Test 2: Exactly at quota
	quota = &QuotaInfo{
		MaxSpace: 3 * 1024 * 1024, // 3 MB (exact match)
	}
	err = validateQuota(localFiles, quota)
	if err != nil {
		t.Errorf("Expected no error for files at quota limit, got: %v", err)
	}

	// Test 3: Over quota
	quota = &QuotaInfo{
		MaxSpace: 2 * 1024 * 1024, // 2 MB (less than 3 MB total)
	}
	err = validateQuota(localFiles, quota)
	if err == nil {
		t.Error("Expected error for files over quota, got nil")
	}

	// Test 4: Empty files
	err = validateQuota([]LocalFile{}, quota)
	if err != nil {
		t.Errorf("Expected no error for empty file list, got: %v", err)
	}
}
