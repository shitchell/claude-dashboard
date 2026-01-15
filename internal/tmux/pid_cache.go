package tmux

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/shitchell/claude-dashboard/internal/constants"
	"github.com/shitchell/claude-dashboard/internal/logging"
	"golang.org/x/sys/unix"
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

// ValidateOnLoad checks cached entries for staleness by comparing session file
// modification times against birth times of untracked files in the same directory.
// This detects stale mappings caused by /clear while the dashboard was closed.
//
// The algorithm:
//  1. Group entries by session file's parent directory
//  2. For each directory, find untracked .jsonl files (UUID-patterned, not agent-*)
//  3. Get birth time of untracked files
//  4. If session mtime <= newest untracked birth time, rescan the PID
//  5. Update cache with new session files from rescan
//
// The rescanFunc callback performs memory scanning for a single PID and returns
// the new session file path if found, or "" if no match.
func (c *PIDCache) ValidateOnLoad(rescanFunc func(pid int) string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.entries) == 0 {
		logging.Debug("PIDCache.ValidateOnLoad: No entries to validate")
		return
	}

	logging.Info("PIDCache.ValidateOnLoad: Validating %d entries for staleness", len(c.entries))

	// Group entries by directory
	dirToEntries := make(map[string][]int) // dir -> list of PIDs
	for pid, entry := range c.entries {
		dir := filepath.Dir(entry.SessionFile)
		dirToEntries[dir] = append(dirToEntries[dir], pid)
	}

	logging.Debug("PIDCache.ValidateOnLoad: %d unique directories", len(dirToEntries))

	var rescannedCount, updatedCount int

	for dir, pids := range dirToEntries {
		// Get all session files in this directory
		allFiles, err := listSessionFilesInDir(dir)
		if err != nil {
			logging.Warn("PIDCache.ValidateOnLoad: Failed to list files in %s: %v", dir, err)
			continue
		}

		// Build set of tracked files (files in cache)
		trackedFiles := make(map[string]struct{})
		for _, pid := range pids {
			entry := c.entries[pid]
			trackedFiles[entry.SessionFile] = struct{}{}
		}

		// Find untracked files
		var untrackedFiles []string
		for _, f := range allFiles {
			if _, tracked := trackedFiles[f]; !tracked {
				untrackedFiles = append(untrackedFiles, f)
			}
		}

		if len(untrackedFiles) == 0 {
			logging.Debug("PIDCache.ValidateOnLoad: %s - no untracked files", dir)
			continue
		}

		logging.Debug("PIDCache.ValidateOnLoad: %s - %d untracked files", dir, len(untrackedFiles))

		// Get newest birth time among untracked files
		var newestBirthTime time.Time
		for _, f := range untrackedFiles {
			birthTime, err := getBirthTime(f)
			if err != nil {
				logging.Debug("PIDCache.ValidateOnLoad: Failed to get birth time for %s: %v", f, err)
				continue
			}
			if birthTime.After(newestBirthTime) {
				newestBirthTime = birthTime
			}
		}

		if newestBirthTime.IsZero() {
			logging.Debug("PIDCache.ValidateOnLoad: %s - could not determine birth times", dir)
			continue
		}

		logging.Debug("PIDCache.ValidateOnLoad: %s - newest untracked birth time: %v", dir, newestBirthTime)

		// Check each cached entry in this directory
		for _, pid := range pids {
			entry := c.entries[pid]

			// Get mtime of the cached session file
			info, err := os.Stat(entry.SessionFile)
			if err != nil {
				logging.Debug("PIDCache.ValidateOnLoad: Cannot stat %s: %v", entry.SessionFile, err)
				// File doesn't exist - mark for rescan
				rescannedCount++
				newSessionFile := rescanFunc(pid)
				if newSessionFile != "" {
					c.entries[pid] = PIDCacheEntry{
						SessionFile: newSessionFile,
						CachedAt:    time.Now(),
					}
					c.dirty = true
					updatedCount++
					logging.Info("PIDCache.ValidateOnLoad: PID %d rescanned: %s -> %s",
						pid, entry.SessionFile, newSessionFile)
				} else {
					// Rescan found nothing - remove entry
					delete(c.entries, pid)
					c.dirty = true
					logging.Info("PIDCache.ValidateOnLoad: PID %d removed (no session found)", pid)
				}
				continue
			}

			sessionMtime := info.ModTime()

			// If session mtime <= newest untracked birth time, the cached session
			// might be stale (a newer session was created after this one was last modified)
			if !sessionMtime.After(newestBirthTime) {
				logging.Info("PIDCache.ValidateOnLoad: PID %d may be stale (mtime %v <= birth %v), rescanning",
					pid, sessionMtime, newestBirthTime)

				rescannedCount++
				newSessionFile := rescanFunc(pid)
				if newSessionFile != "" && newSessionFile != entry.SessionFile {
					c.entries[pid] = PIDCacheEntry{
						SessionFile: newSessionFile,
						CachedAt:    time.Now(),
					}
					c.dirty = true
					updatedCount++
					logging.Info("PIDCache.ValidateOnLoad: PID %d updated: %s -> %s",
						pid, entry.SessionFile, newSessionFile)
				} else if newSessionFile == "" {
					// Rescan found nothing - remove entry
					delete(c.entries, pid)
					c.dirty = true
					logging.Info("PIDCache.ValidateOnLoad: PID %d removed (no session found on rescan)", pid)
				} else {
					logging.Debug("PIDCache.ValidateOnLoad: PID %d unchanged after rescan", pid)
				}
			}
		}
	}

	logging.Info("PIDCache.ValidateOnLoad: Rescanned %d PIDs, updated %d entries", rescannedCount, updatedCount)
}

// getBirthTime returns the birth (creation) time of a file.
// It first tries the statx syscall with STATX_BTIME, which is the most accurate.
// If that fails (e.g., filesystem doesn't support birth time), it falls back to
// parsing the timestamp from the first line of the JSONL file.
// If that also fails, it falls back to the file's mtime.
func getBirthTime(path string) (time.Time, error) {
	// Try statx with STATX_BTIME first
	var stat unix.Statx_t
	err := unix.Statx(unix.AT_FDCWD, path, 0, unix.STATX_BTIME, &stat)
	if err == nil && (stat.Mask&unix.STATX_BTIME) != 0 {
		birthTime := time.Unix(stat.Btime.Sec, int64(stat.Btime.Nsec))
		logging.Debug("getBirthTime: %s - statx birth time: %v", path, birthTime)
		return birthTime, nil
	}

	logging.Debug("getBirthTime: %s - statx BTIME not available, trying JSONL fallback", path)

	// Fall back to JSONL first-line timestamp
	jsonlTime, err := parseJSONLFirstLineTimestamp(path)
	if err == nil {
		logging.Debug("getBirthTime: %s - JSONL timestamp: %v", path, jsonlTime)
		return jsonlTime, nil
	}

	logging.Debug("getBirthTime: %s - JSONL fallback failed (%v), using mtime", path, err)

	// Final fallback: use mtime
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, fmt.Errorf("cannot stat file: %w", err)
	}
	return info.ModTime(), nil
}

// parseJSONLFirstLineTimestamp reads the first line of a JSONL file and extracts
// the timestamp field. This is used as a fallback when statx birth time is unavailable.
//
// Expected format: {"timestamp": "2025-12-29T22:35:51.370Z", "type": "...", ...}
func parseJSONLFirstLineTimestamp(path string) (time.Time, error) {
	file, err := os.Open(path)
	if err != nil {
		return time.Time{}, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return time.Time{}, fmt.Errorf("failed to read first line: %w", err)
		}
		return time.Time{}, fmt.Errorf("file is empty")
	}

	line := scanner.Text()

	// Parse the JSON to extract timestamp
	var entry struct {
		Timestamp string `json:"timestamp"`
	}
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		return time.Time{}, fmt.Errorf("failed to parse JSON: %w", err)
	}

	if entry.Timestamp == "" {
		return time.Time{}, fmt.Errorf("no timestamp field in first line")
	}

	// Parse ISO8601 timestamp
	t, err := time.Parse(time.RFC3339Nano, entry.Timestamp)
	if err != nil {
		// Try without nanoseconds
		t, err = time.Parse(time.RFC3339, entry.Timestamp)
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to parse timestamp %q: %w", entry.Timestamp, err)
		}
	}

	return t, nil
}

// listSessionFilesInDir returns all UUID-patterned .jsonl files in a directory,
// excluding agent-*.jsonl files.
func listSessionFilesInDir(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".jsonl") {
			continue
		}

		// Extract session ID (filename without .jsonl)
		sessionID := strings.TrimSuffix(name, ".jsonl")

		// Skip non-UUID session files (like agent-*.jsonl)
		if !IsUUIDSessionFile(sessionID) {
			continue
		}

		files = append(files, filepath.Join(dir, name))
	}

	return files, nil
}
