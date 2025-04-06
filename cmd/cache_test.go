package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// testCachePath is used to override the cache path during testing
var testCachePath string

// getCachePathForTest returns the test cache path if set, otherwise delegates to getCachePath
func getCachePathForTest() (string, error) {
	if testCachePath != "" {
		return testCachePath, nil
	}
	return getCachePath()
}

func TestCacheOperations(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "grit-cache-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Override the cache path for testing
	originalGetCachePath := getCachePathFn
	getCachePathFn = func() (string, error) {
		return filepath.Join(tempDir, cacheFileName), nil
	}
	defer func() { getCachePathFn = originalGetCachePath }()

	// Test creating and loading an empty cache
	cache, err := loadCache()
	if err != nil {
		t.Fatalf("Failed to load empty cache: %v", err)
	}
	if len(cache.Entries) != 0 {
		t.Errorf("Expected empty cache, got %d entries", len(cache.Entries))
	}

	// Test saving and loading a cache entry
	entry := CacheEntry{
		Args: struct {
			AuthorRegex    string
			RemoteName     string
			FilenamesRegex string
			WeekToDate     bool
		}{
			AuthorRegex:    "test",
			RemoteName:     "origin",
			FilenamesRegex: ".*",
			WeekToDate:     true,
		},
		Paths:      []string{"./"},
		HeadHashes: map[string]string{"./": "abc123"},
		Results: struct {
			Added   int64
			Deleted int64
		}{
			Added:   100,
			Deleted: 50,
		},
		Timestamp: time.Now(),
	}

	cache.Entries = append(cache.Entries, entry)
	if err := saveCache(cache); err != nil {
		t.Fatalf("Failed to save cache: %v", err)
	}

	// Load the cache again and verify the entry
	cache, err = loadCache()
	if err != nil {
		t.Fatalf("Failed to load cache: %v", err)
	}
	if len(cache.Entries) != 1 {
		t.Errorf("Expected 1 cache entry, got %d", len(cache.Entries))
	}

	// Test finding matching cache entry
	foundEntry := findMatchingCacheEntry(cache, []string{"./"}, "test", "origin", ".*", true)
	if foundEntry == nil {
		t.Error("Failed to find matching cache entry")
	}
	if foundEntry.Results.Added != 100 || foundEntry.Results.Deleted != 50 {
		t.Errorf("Found entry has incorrect results: +%d/-%d", foundEntry.Results.Added, foundEntry.Results.Deleted)
	}

	// Test cache size limit
	for i := 0; i < maxCacheSize+10; i++ {
		cache.Entries = append(cache.Entries, entry)
	}
	if err := saveCache(cache); err != nil {
		t.Fatalf("Failed to save cache with size limit: %v", err)
	}
	cache, err = loadCache()
	if err != nil {
		t.Fatalf("Failed to load cache with size limit: %v", err)
	}
	if len(cache.Entries) != maxCacheSize {
		t.Errorf("Expected cache size to be limited to %d, got %d", maxCacheSize, len(cache.Entries))
	}
}

func TestCacheValidity(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "grit-cache-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Initialize a git repository
	repo, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to initialize git repository: %v", err)
	}

	// Create a test file and commit it
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	w, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Failed to get worktree: %v", err)
	}

	_, err = w.Add("test.txt")
	if err != nil {
		t.Fatalf("Failed to add test file: %v", err)
	}

	commit, err := w.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test Author",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	entry := CacheEntry{
		Paths:      []string{tempDir},
		HeadHashes: map[string]string{tempDir: commit.String()},
		Timestamp:  time.Now(),
	}

	// Test valid cache
	if !isCacheValid(&entry, []string{tempDir}) {
		t.Error("Valid cache entry was considered invalid")
	}

	// Test expired cache
	oldEntry := entry
	oldEntry.Timestamp = time.Now().Add(-25 * time.Hour)
	if isCacheValid(&oldEntry, []string{tempDir}) {
		t.Error("Expired cache entry was considered valid")
	}

	// Test invalid path
	if isCacheValid(&entry, []string{"nonexistent"}) {
		t.Error("Cache entry with invalid path was considered valid")
	}

	// Test invalid hash
	invalidEntry := entry
	invalidEntry.HeadHashes[tempDir] = "invalid-hash"
	if isCacheValid(&invalidEntry, []string{tempDir}) {
		t.Error("Cache entry with invalid hash was considered valid")
	}
}
