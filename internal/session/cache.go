package session

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/shitchell/claude-dashboard/internal/constants"
	"github.com/shitchell/claude-dashboard/internal/logging"
)

// CacheEntry represents a single cached session metadata entry.
// It stores both the file's modification time (for invalidation)
// and the parsed metadata.
type CacheEntry struct {
	// Mtime is the Unix timestamp of the file's modification time
	// when this entry was cached. Used for cache invalidation.
	Mtime int64 `json:"mtime"`

	// Metadata is the cached session metadata.
	Metadata SessionMetadata `json:"metadata"`
}

// CacheFile represents the on-disk cache file format.
// The version field ensures cache compatibility across upgrades.
type CacheFile struct {
	// Version is the cache format version. When the cache format
	// changes, this is incremented to force a cache rebuild.
	Version int `json:"version"`

	// Entries maps file paths to their cached data.
	// The key is the absolute path to the JSONL file.
	Entries map[string]CacheEntry `json:"entries"`
}

// Cache provides thread-safe caching of session metadata with
// mtime-based invalidation and atomic persistence.
type Cache struct {
	// path is the absolute path to the cache file.
	path string

	// version is the expected cache version.
	version int

	// entries holds the in-memory cache data.
	entries map[string]CacheEntry

	// mu protects all cache operations.
	mu sync.RWMutex

	// dirty indicates whether the cache has unsaved changes.
	dirty bool
}

// NewCache creates a new Cache instance that will store data at the
// given path. The cache is not loaded automatically; call Load()
// to read existing cache data from disk.
func NewCache(path string) *Cache {
	return &Cache{
		path:    path,
		version: constants.CacheVersion,
		entries: make(map[string]CacheEntry),
	}
}

// Load reads the cache from disk. If the file doesn't exist or the
// cache version doesn't match, the cache is reset to empty.
// Returns nil if the file doesn't exist (empty cache is valid).
func (c *Cache) Load() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Reset to empty state
	c.entries = make(map[string]CacheEntry)
	c.dirty = false

	file, err := os.Open(c.path)
	if err != nil {
		if os.IsNotExist(err) {
			// No cache file yet - this is normal on first run
			logging.Debug("Cache file does not exist, starting fresh: %s", c.path)
			return nil
		}
		logging.Warn("Error reading cache file, starting fresh: %s: %v", c.path, err)
		return nil // Start fresh on read error
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		logging.Warn("Error reading cache file contents, starting fresh: %s: %v", c.path, err)
		return nil // Start fresh on read error
	}

	if len(data) == 0 {
		// Empty file - treat as empty cache
		return nil
	}

	var cacheFile CacheFile
	if err := json.Unmarshal(data, &cacheFile); err != nil {
		// Corrupted cache - reset to empty
		logging.Warn("Cache file corrupted (invalid JSON), starting fresh: %s", c.path)
		return nil
	}

	// Version mismatch - reset to empty (will rebuild)
	if cacheFile.Version != c.version {
		logging.Info("Cache version mismatch (got %d, expected %d), rebuilding cache", cacheFile.Version, c.version)
		return nil
	}

	// Load entries
	if cacheFile.Entries != nil {
		c.entries = cacheFile.Entries
	}

	return nil
}

// Save writes the cache to disk atomically. It writes to a temporary
// file first, then renames to ensure the cache file is never left in
// a corrupted state on crash.
func (c *Cache) Save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Create the cache directory if it doesn't exist
	dir := filepath.Dir(c.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logging.Warn("Failed to create cache directory: %s: %v", dir, err)
		return err
	}

	cacheFile := CacheFile{
		Version: c.version,
		Entries: c.entries,
	}

	data, err := json.MarshalIndent(cacheFile, "", "  ")
	if err != nil {
		return err
	}

	// Write to temporary file first for atomic update
	tempPath := c.path + ".tmp"
	tempFile, err := os.Create(tempPath)
	if err != nil {
		logging.Warn("Failed to create cache temp file: %s: %v", tempPath, err)
		return err
	}

	_, writeErr := tempFile.Write(data)
	closeErr := tempFile.Close()

	// Check for write errors
	if writeErr != nil {
		// Clean up temp file on error
		os.Remove(tempPath)
		logging.Warn("Failed to write cache data: %v", writeErr)
		return writeErr
	}
	if closeErr != nil {
		os.Remove(tempPath)
		logging.Warn("Failed to close cache temp file: %v", closeErr)
		return closeErr
	}

	// Atomic rename
	if err := os.Rename(tempPath, c.path); err != nil {
		os.Remove(tempPath)
		logging.Warn("Failed to rename cache temp file: %v", err)
		return err
	}

	return nil
}

// Get retrieves cached metadata for a file if the cache entry exists
// and the mtime matches. Returns nil if the cache entry is missing
// or stale (mtime differs).
func (c *Cache) Get(path string, mtime time.Time) *SessionMetadata {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[path]
	if !exists {
		return nil
	}

	// Check if mtime matches (cache invalidation)
	// Truncate to seconds because JSONL stores Unix timestamp
	cachedTime := time.Unix(entry.Mtime, 0)
	currentTime := mtime.Truncate(time.Second)

	if !cachedTime.Equal(currentTime) {
		return nil
	}

	// Return a copy to prevent mutation
	metadata := entry.Metadata
	return &metadata
}

// Set stores metadata for a file with its modification time.
// This overwrites any existing entry for the path.
func (c *Cache) Set(path string, mtime time.Time, meta *SessionMetadata) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[path] = CacheEntry{
		Mtime:    mtime.Truncate(time.Second).Unix(),
		Metadata: *meta,
	}
	c.dirty = true
}

// Prune removes cache entries for files that no longer exist.
// The validPaths map should contain all currently valid file paths.
// Entries not in validPaths are removed from the cache.
func (c *Cache) Prune(validPaths map[string]struct{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for path := range c.entries {
		if _, valid := validPaths[path]; !valid {
			delete(c.entries, path)
			c.dirty = true
		}
	}
}

// IsDirty returns whether the cache has unsaved changes.
func (c *Cache) IsDirty() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.dirty
}

// Size returns the number of entries in the cache.
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// Path returns the cache file path.
func (c *Cache) Path() string {
	return c.path
}
