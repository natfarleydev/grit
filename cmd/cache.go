package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5"
)

// CacheEntry represents a single cached result
type CacheEntry struct {
	Args struct {
		AuthorRegex    string
		RemoteName     string
		FilenamesRegex string
		WeekToDate     bool
	}
	Paths      []string
	HeadHashes map[string]string // path -> commit hash
	Results    struct {
		Added   int64
		Deleted int64
	}
	Timestamp time.Time
}

// Cache represents the entire cache file
type Cache struct {
	Entries []CacheEntry
}

const (
	cacheFileName = ".grit-cache.json"
	maxCacheSize  = 100 // Maximum number of cache entries to keep
)

var getCachePathFn = defaultGetCachePath

// defaultGetCachePath returns the default path to the cache file in the user's home directory
func defaultGetCachePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, cacheFileName), nil
}

// getCachePath returns the path to the cache file
func getCachePath() (string, error) {
	return getCachePathFn()
}

// loadCache loads the cache from the file
func loadCache() (*Cache, error) {
	cachePath, err := getCachePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Cache{Entries: make([]CacheEntry, 0)}, nil
		}
		return nil, err
	}

	var cache Cache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}

	return &cache, nil
}

// saveCache saves the cache to the file
func saveCache(cache *Cache) error {
	// Limit cache size
	if len(cache.Entries) > maxCacheSize {
		cache.Entries = cache.Entries[len(cache.Entries)-maxCacheSize:]
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	cachePath, err := getCachePath()
	if err != nil {
		return err
	}

	return os.WriteFile(cachePath, data, 0644)
}

// findMatchingCacheEntry finds a cache entry that matches the given arguments
func findMatchingCacheEntry(cache *Cache, args []string, authorRegex, remoteName, filenamesRegex string, weekToDate bool) *CacheEntry {
	for i := len(cache.Entries) - 1; i >= 0; i-- {
		entry := &cache.Entries[i]
		if entry.Args.AuthorRegex != authorRegex ||
			entry.Args.RemoteName != remoteName ||
			entry.Args.FilenamesRegex != filenamesRegex ||
			entry.Args.WeekToDate != weekToDate ||
			len(entry.Paths) != len(args) {
			continue
		}

		// Check if paths match
		pathsMatch := true
		for j, path := range args {
			if entry.Paths[j] != path {
				pathsMatch = false
				break
			}
		}
		if !pathsMatch {
			continue
		}

		return entry
	}
	return nil
}

// isCacheValid checks if a cache entry is still valid
func isCacheValid(entry *CacheEntry, paths []string) bool {
	// Check if cache is too old (24 hours)
	if time.Since(entry.Timestamp) > 24*time.Hour {
		return false
	}

	// Check if all paths still exist and have matching HEAD hashes
	for _, path := range paths {
		repo, err := git.PlainOpen(path)
		if err != nil {
			return false
		}

		head, err := repo.Head()
		if err != nil {
			return false
		}

		cachedHash, exists := entry.HeadHashes[path]
		if !exists || cachedHash != head.Hash().String() {
			return false
		}
	}

	return true
}
