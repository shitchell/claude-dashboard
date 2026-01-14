package tmux

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/shitchell/claude-dashboard/internal/constants"
	"github.com/shitchell/claude-dashboard/internal/logging"
)

// PIDCacheEntry represents a cached PID-to-session mapping.
type PIDCacheEntry struct {
	// SessionFile is the full path to the session JSONL file.
	SessionFile string `json:"session_file"`

	// CachedAt is when this entry was cached.
	CachedAt time.Time `json:"cached_at"`
}

// PIDCacheFile represents the on-disk cache file format.
type PIDCacheFile struct {
	// Version is the cache format version. When the format changes,
	// this is incremented to force a cache rebuild.
	Version int `json:"version"`

	// Entries maps PID (as string for JSON) to cache entries.
	// Using string keys because JSON requires string keys.
	Entries map[string]PIDCacheEntry `json:"entries"`
}

// PIDCache provides thread-safe caching of PID-to-session file mappings.
// It stores the mapping in an ephemeral location (XDG_RUNTIME_DIR or /tmp)
// so it's cleared on reboot.
type PIDCache struct {
	mu sync.RWMutex

	// path is the absolute path to the cache file.
	path string

	// version is the expected cache version.
	version int

	// entries holds the in-memory cache data (PID -> entry).
	entries map[int]PIDCacheEntry

	// dirty indicates whether the cache has unsaved changes.
	dirty bool
}

// NewPIDCache creates a new PIDCache at the default location.
// The cache is not loaded automatically; call Load() to read existing data.
func NewPIDCache() *PIDCache {
	return &PIDCache{
		path:    CachePath(),
		version: constants.PIDCacheVersion,
		entries: make(map[int]PIDCacheEntry),
	}
}

// NewPIDCacheWithPath creates a new PIDCache at a custom path.
// This is primarily used for testing.
func NewPIDCacheWithPath(path string) *PIDCache {
	return &PIDCache{
		path:    path,
		version: constants.PIDCacheVersion,
		entries: make(map[int]PIDCacheEntry),
	}
}

// CachePath returns the appropriate cache file path based on environment.
// Primary: $XDG_RUNTIME_DIR/claude-dashboard/pid-cache.json
// Fallback: /tmp/claude-dashboard-$UID/pid-cache.json
func CachePath() string {
	// Try XDG_RUNTIME_DIR first (ephemeral, per-user, typically tmpfs)
	if xdgRuntime := os.Getenv("XDG_RUNTIME_DIR"); xdgRuntime != "" {
		return filepath.Join(xdgRuntime, constants.PIDCacheRuntimeDir, constants.PIDCacheFileName)
	}

	// Fallback to /tmp with UID suffix for security
	uid := os.Getuid()
	return filepath.Join(fmt.Sprintf("/tmp/claude-dashboard-%d", uid), constants.PIDCacheFileName)
}

// Path returns the cache file path.
func (c *PIDCache) Path() string {
	return c.path
}

// Load reads the cache from disk with flock.
// If the file doesn't exist or the version doesn't match, starts fresh.
// After loading, validates that cached PIDs still exist.
func (c *PIDCache) Load() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	logging.Debug("PIDCache: Loading from %s", c.path)

	// Reset to empty state
	c.entries = make(map[int]PIDCacheEntry)
	c.dirty = false

	// Try to open the file
	file, err := os.Open(c.path)
	if err != nil {
		if os.IsNotExist(err) {
			logging.Debug("PIDCache: File does not exist, starting fresh")
			return nil
		}
		logging.Warn("PIDCache: Error opening file, starting fresh: %v", err)
		return nil
	}
	defer file.Close()

	// Read and parse
	var cacheFile PIDCacheFile
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&cacheFile); err != nil {
		logging.Warn("PIDCache: Error decoding file, starting fresh: %v", err)
		return nil
	}

	// Version check
	if cacheFile.Version != c.version {
		logging.Info("PIDCache: Version mismatch (got %d, expected %d), starting fresh",
			cacheFile.Version, c.version)
		return nil
	}

	// Convert string keys to int keys
	for pidStr, entry := range cacheFile.Entries {
		var pid int
		if _, err := fmt.Sscanf(pidStr, "%d", &pid); err != nil {
			logging.Warn("PIDCache: Invalid PID key %q, skipping", pidStr)
			continue
		}
		c.entries[pid] = entry
	}

	logging.Info("PIDCache: Loaded %d entries", len(c.entries))

	// Validate PIDs (remove entries for dead processes)
	c.validatePIDsLocked()

	return nil
}

// Save writes the cache to disk atomically with flock.
func (c *PIDCache) Save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.dirty {
		logging.Debug("PIDCache: Not dirty, skipping save")
		return nil
	}

	logging.Debug("PIDCache: Saving %d entries to %s", len(c.entries), c.path)

	// Create directory if needed
	dir := filepath.Dir(c.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logging.Warn("PIDCache: Failed to create directory %s: %v", dir, err)
		return err
	}

	// Convert int keys to string keys for JSON
	entries := make(map[string]PIDCacheEntry)
	for pid, entry := range c.entries {
		entries[fmt.Sprintf("%d", pid)] = entry
	}

	cacheFile := PIDCacheFile{
		Version: c.version,
		Entries: entries,
	}

	data, err := json.MarshalIndent(cacheFile, "", "  ")
	if err != nil {
		return err
	}

	// Write to temp file
	tempPath := c.path + ".tmp"
	tempFile, err := os.OpenFile(tempPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		logging.Warn("PIDCache: Failed to create temp file: %v", err)
		return err
	}

	// Acquire exclusive flock for writing
	if err := syscall.Flock(int(tempFile.Fd()), syscall.LOCK_EX); err != nil {
		tempFile.Close()
		os.Remove(tempPath)
		logging.Warn("PIDCache: Failed to acquire flock: %v", err)
		return err
	}

	_, writeErr := tempFile.Write(data)
	closeErr := tempFile.Close() // This also releases the flock

	if writeErr != nil {
		os.Remove(tempPath)
		logging.Warn("PIDCache: Failed to write data: %v", writeErr)
		return writeErr
	}
	if closeErr != nil {
		os.Remove(tempPath)
		logging.Warn("PIDCache: Failed to close temp file: %v", closeErr)
		return closeErr
	}

	// Atomic rename
	if err := os.Rename(tempPath, c.path); err != nil {
		os.Remove(tempPath)
		logging.Warn("PIDCache: Failed to rename temp file: %v", err)
		return err
	}

	logging.Info("PIDCache: Saved %d entries to %s", len(c.entries), c.path)
	return nil
}

// Get retrieves the cached session file for a PID.
// Returns (sessionFile, true) if found, ("", false) otherwise.
func (c *PIDCache) Get(pid int) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[pid]
	if !ok {
		return "", false
	}
	return entry.SessionFile, true
}

// Set stores the session file for a PID.
func (c *PIDCache) Set(pid int, sessionFile string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[pid] = PIDCacheEntry{
		SessionFile: sessionFile,
		CachedAt:    time.Now(),
	}
	c.dirty = true
	logging.Debug("PIDCache: Set PID %d -> %s", pid, sessionFile)
}

// Remove removes an entry for a PID.
func (c *PIDCache) Remove(pid int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.entries[pid]; ok {
		delete(c.entries, pid)
		c.dirty = true
		logging.Debug("PIDCache: Removed PID %d", pid)
	}
}

// ValidatePIDs removes entries for PIDs that no longer exist.
// This is called automatically during Load() and can be called manually.
func (c *PIDCache) ValidatePIDs() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.validatePIDsLocked()
}

// validatePIDsLocked removes entries for dead PIDs. Caller must hold the lock.
func (c *PIDCache) validatePIDsLocked() {
	var removed []int
	for pid := range c.entries {
		if !pidExists(pid) {
			removed = append(removed, pid)
		}
	}

	for _, pid := range removed {
		delete(c.entries, pid)
		c.dirty = true
	}

	if len(removed) > 0 {
		logging.Info("PIDCache: Removed %d stale entries for dead PIDs", len(removed))
	}
}

// pidExists checks if a process with the given PID exists.
// This is done by checking /proc/<pid> on Linux.
func pidExists(pid int) bool {
	procPath := fmt.Sprintf("/proc/%d", pid)
	_, err := os.Stat(procPath)
	return err == nil
}

// GetUncachedPIDs returns PIDs from the input slice that are not in the cache.
func (c *PIDCache) GetUncachedPIDs(allPIDs []int) []int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var uncached []int
	for _, pid := range allPIDs {
		if _, ok := c.entries[pid]; !ok {
			uncached = append(uncached, pid)
		}
	}
	return uncached
}

// GetCachedMapping returns a copy of all cached PID -> sessionID mappings.
// The session ID is extracted from the session file path.
func (c *PIDCache) GetCachedMapping() map[int]string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[int]string)
	for pid, entry := range c.entries {
		// Extract session ID from path (filename without .jsonl)
		sessionID := extractSessionIDFromPath(entry.SessionFile)
		result[pid] = sessionID
	}
	return result
}

// extractSessionIDFromPath extracts the session ID from a session file path.
// Example: "/home/user/.claude/projects/foo/abc123.jsonl" -> "abc123"
func extractSessionIDFromPath(path string) string {
	base := filepath.Base(path)
	// Remove .jsonl extension
	if len(base) > 6 && base[len(base)-6:] == ".jsonl" {
		return base[:len(base)-6]
	}
	return base
}

// Clear removes all entries from the cache.
func (c *PIDCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[int]PIDCacheEntry)
	c.dirty = true
	logging.Debug("PIDCache: Cleared all entries")
}

// Size returns the number of entries in the cache.
func (c *PIDCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// IsDirty returns whether the cache has unsaved changes.
func (c *PIDCache) IsDirty() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.dirty
}

// MarkClean clears the dirty flag (used after Save).
func (c *PIDCache) MarkClean() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.dirty = false
}

// GetAllSessionDirs returns unique session directories from cached entries.
// This is used by the SessionWatcher to know which directories to watch.
func (c *PIDCache) GetAllSessionDirs() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	dirSet := make(map[string]struct{})
	for _, entry := range c.entries {
		dir := filepath.Dir(entry.SessionFile)
		dirSet[dir] = struct{}{}
	}

	dirs := make([]string, 0, len(dirSet))
	for dir := range dirSet {
		dirs = append(dirs, dir)
	}
	return dirs
}
