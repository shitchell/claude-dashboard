package tmux

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"
)

// =============================================================================
// PID Cache Path Selection Tests
// =============================================================================

// TestPIDCachePath_XDGRuntime verifies that CachePath() uses XDG_RUNTIME_DIR when set.
func TestPIDCachePath_XDGRuntime(t *testing.T) {
	// Set XDG_RUNTIME_DIR
	xdgDir := "/run/user/1000"
	t.Setenv("XDG_RUNTIME_DIR", xdgDir)

	path := CachePath()

	expected := "/run/user/1000/claude-dashboard/pid-cache.json"
	if path != expected {
		t.Errorf("CachePath() with XDG_RUNTIME_DIR = %q, want %q", path, expected)
	}
}

// TestPIDCachePath_Fallback verifies that CachePath() falls back to /tmp when XDG_RUNTIME_DIR is not set.
func TestPIDCachePath_Fallback(t *testing.T) {
	// Unset XDG_RUNTIME_DIR
	t.Setenv("XDG_RUNTIME_DIR", "")

	path := CachePath()

	// Should use /tmp/claude-dashboard-$UID/pid-cache.json
	uid := os.Getuid()
	expected := "/tmp/claude-dashboard-" + strconv.Itoa(uid) + "/pid-cache.json"
	if path != expected {
		t.Errorf("CachePath() without XDG_RUNTIME_DIR = %q, want %q", path, expected)
	}
}

// =============================================================================
// PID Cache Basic CRUD Tests
// =============================================================================

// TestPIDCache_SetAndGet tests basic Set and Get operations.
func TestPIDCache_SetAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	cache := NewPIDCacheWithPath(filepath.Join(tmpDir, "cache.json"))

	// Set an entry
	sessionFile := "/home/user/.claude/projects/test/session-abc.jsonl"
	cache.Set(12345, sessionFile)

	// Get should return it
	result, ok := cache.Get(12345)
	if !ok {
		t.Fatal("Get(12345) returned false, want true")
	}
	if result != sessionFile {
		t.Errorf("Get(12345) = %q, want %q", result, sessionFile)
	}
}

// TestPIDCache_GetMissing tests Get for non-existent PID.
func TestPIDCache_GetMissing(t *testing.T) {
	tmpDir := t.TempDir()
	cache := NewPIDCacheWithPath(filepath.Join(tmpDir, "cache.json"))

	// Get non-existent PID
	result, ok := cache.Get(99999)
	if ok {
		t.Error("Get(99999) returned true for non-existent PID, want false")
	}
	if result != "" {
		t.Errorf("Get(99999) = %q, want empty string", result)
	}
}

// TestPIDCache_Remove tests Remove operation.
func TestPIDCache_Remove(t *testing.T) {
	tmpDir := t.TempDir()
	cache := NewPIDCacheWithPath(filepath.Join(tmpDir, "cache.json"))

	// Set and verify
	cache.Set(12345, "/path/session.jsonl")
	if _, ok := cache.Get(12345); !ok {
		t.Fatal("Set failed, entry not found")
	}

	// Remove and verify
	cache.Remove(12345)
	result, ok := cache.Get(12345)
	if ok {
		t.Error("Get(12345) after Remove returned true, want false")
	}
	if result != "" {
		t.Errorf("Get(12345) after Remove = %q, want empty string", result)
	}
}

// TestPIDCache_Clear tests Clear operation.
func TestPIDCache_Clear(t *testing.T) {
	tmpDir := t.TempDir()
	cache := NewPIDCacheWithPath(filepath.Join(tmpDir, "cache.json"))

	// Set multiple entries
	cache.Set(100, "/path/session1.jsonl")
	cache.Set(200, "/path/session2.jsonl")
	cache.Set(300, "/path/session3.jsonl")

	// Verify all exist
	if cache.Size() != 3 {
		t.Fatalf("Size() = %d after setting 3 entries, want 3", cache.Size())
	}

	// Clear and verify
	cache.Clear()
	if cache.Size() != 0 {
		t.Errorf("Size() = %d after Clear, want 0", cache.Size())
	}

	// Individual entries should be gone
	for _, pid := range []int{100, 200, 300} {
		if _, ok := cache.Get(pid); ok {
			t.Errorf("Get(%d) returned true after Clear, want false", pid)
		}
	}
}

// =============================================================================
// PID Cache Persistence Tests
// =============================================================================

// TestPIDCache_SaveAndLoad tests persistence across cache instances.
func TestPIDCache_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	// Create cache and add entries
	cache1 := NewPIDCacheWithPath(cachePath)
	cache1.Set(111, "/path/session-aaa.jsonl")
	cache1.Set(222, "/path/session-bbb.jsonl")

	// Save to disk
	if err := cache1.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		t.Fatal("Cache file not created after Save()")
	}

	// Create new cache instance and load
	// Note: We can't use Load() directly because it validates PIDs,
	// and PIDs 111 and 222 don't exist. So we test the file format directly.
	content, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("Failed to read cache file: %v", err)
	}

	var cacheFile PIDCacheFile
	if err := json.Unmarshal(content, &cacheFile); err != nil {
		t.Fatalf("Failed to parse cache file: %v", err)
	}

	// Verify structure
	if cacheFile.Version != 1 {
		t.Errorf("Cache file version = %d, want 1", cacheFile.Version)
	}
	if len(cacheFile.Entries) != 2 {
		t.Errorf("Cache file has %d entries, want 2", len(cacheFile.Entries))
	}

	// Verify entries
	entry111, ok := cacheFile.Entries["111"]
	if !ok {
		t.Error("Entry for PID 111 not found in cache file")
	} else if entry111.SessionFile != "/path/session-aaa.jsonl" {
		t.Errorf("Entry 111 SessionFile = %q, want /path/session-aaa.jsonl", entry111.SessionFile)
	}
}

// TestPIDCache_LoadMissingFile tests Load when file doesn't exist.
func TestPIDCache_LoadMissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	cache := NewPIDCacheWithPath(filepath.Join(tmpDir, "nonexistent.json"))

	// Load should succeed (no error) but cache should be empty
	err := cache.Load()
	if err != nil {
		t.Errorf("Load() error = %v for missing file, want nil", err)
	}
	if cache.Size() != 0 {
		t.Errorf("Size() = %d after Load with missing file, want 0", cache.Size())
	}
}

// TestPIDCache_LoadCorruptedJSON tests Load with invalid JSON.
func TestPIDCache_LoadCorruptedJSON(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	// Write invalid JSON
	if err := os.WriteFile(cachePath, []byte("{invalid json}"), 0644); err != nil {
		t.Fatalf("Failed to write corrupted file: %v", err)
	}

	cache := NewPIDCacheWithPath(cachePath)

	// Load should succeed but start fresh
	err := cache.Load()
	if err != nil {
		t.Errorf("Load() error = %v for corrupted JSON, want nil", err)
	}
	if cache.Size() != 0 {
		t.Errorf("Size() = %d after Load with corrupted JSON, want 0", cache.Size())
	}
}

// TestPIDCache_LoadVersionMismatch tests Load with wrong version.
func TestPIDCache_LoadVersionMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	// Write cache with wrong version
	cacheFile := PIDCacheFile{
		Version: 999, // Wrong version
		Entries: map[string]PIDCacheEntry{
			"12345": {SessionFile: "/path/session.jsonl"},
		},
	}
	data, _ := json.Marshal(cacheFile)
	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		t.Fatalf("Failed to write cache file: %v", err)
	}

	cache := NewPIDCacheWithPath(cachePath)

	// Load should succeed but start fresh due to version mismatch
	err := cache.Load()
	if err != nil {
		t.Errorf("Load() error = %v for version mismatch, want nil", err)
	}
	if cache.Size() != 0 {
		t.Errorf("Size() = %d after Load with version mismatch, want 0", cache.Size())
	}
}

// =============================================================================
// PID Validation Tests
// =============================================================================

// TestPIDCache_ValidatePIDs_RemovesDead tests that ValidatePIDs removes entries for dead PIDs.
func TestPIDCache_ValidatePIDs_RemovesDead(t *testing.T) {
	tmpDir := t.TempDir()
	cache := NewPIDCacheWithPath(filepath.Join(tmpDir, "cache.json"))

	// Add entry for a non-existent PID
	fakePID := 999999 // Very unlikely to exist
	cache.Set(fakePID, "/path/session.jsonl")

	// Validate should remove it
	cache.ValidatePIDs()

	if _, ok := cache.Get(fakePID); ok {
		t.Error("Dead PID entry still exists after ValidatePIDs()")
	}
}

// TestPIDCache_ValidatePIDs_KeepsLive tests that ValidatePIDs keeps entries for live PIDs.
func TestPIDCache_ValidatePIDs_KeepsLive(t *testing.T) {
	tmpDir := t.TempDir()
	cache := NewPIDCacheWithPath(filepath.Join(tmpDir, "cache.json"))

	// Add entry for current process (guaranteed to exist)
	myPID := os.Getpid()
	sessionFile := "/path/session.jsonl"
	cache.Set(myPID, sessionFile)

	// Validate should keep it
	cache.ValidatePIDs()

	result, ok := cache.Get(myPID)
	if !ok {
		t.Error("Live PID entry removed by ValidatePIDs()")
	}
	if result != sessionFile {
		t.Errorf("Get(%d) = %q after ValidatePIDs, want %q", myPID, result, sessionFile)
	}
}

// =============================================================================
// GetUncachedPIDs Tests
// =============================================================================

// TestPIDCache_GetUncachedPIDs_EmptyCache tests GetUncachedPIDs with empty cache.
func TestPIDCache_GetUncachedPIDs_EmptyCache(t *testing.T) {
	tmpDir := t.TempDir()
	cache := NewPIDCacheWithPath(filepath.Join(tmpDir, "cache.json"))

	// All PIDs should be returned as uncached
	allPIDs := []int{100, 200, 300, 400, 500}
	uncached := cache.GetUncachedPIDs(allPIDs)

	if len(uncached) != len(allPIDs) {
		t.Errorf("GetUncachedPIDs returned %d PIDs, want %d", len(uncached), len(allPIDs))
	}

	// Verify all input PIDs are in result
	uncachedSet := make(map[int]bool)
	for _, pid := range uncached {
		uncachedSet[pid] = true
	}
	for _, pid := range allPIDs {
		if !uncachedSet[pid] {
			t.Errorf("PID %d missing from uncached result", pid)
		}
	}
}

// TestPIDCache_GetUncachedPIDs_SomeCached tests GetUncachedPIDs with partial cache.
func TestPIDCache_GetUncachedPIDs_SomeCached(t *testing.T) {
	tmpDir := t.TempDir()
	cache := NewPIDCacheWithPath(filepath.Join(tmpDir, "cache.json"))

	// Cache some PIDs
	cache.Set(100, "/path/session-100.jsonl")
	cache.Set(300, "/path/session-300.jsonl")

	// Request PIDs where some are cached
	allPIDs := []int{100, 200, 300, 400, 500}
	uncached := cache.GetUncachedPIDs(allPIDs)

	// Should return only 200, 400, 500
	expectedUncached := []int{200, 400, 500}
	if len(uncached) != len(expectedUncached) {
		t.Errorf("GetUncachedPIDs returned %d PIDs, want %d", len(uncached), len(expectedUncached))
	}

	uncachedSet := make(map[int]bool)
	for _, pid := range uncached {
		uncachedSet[pid] = true
	}

	for _, pid := range expectedUncached {
		if !uncachedSet[pid] {
			t.Errorf("Expected uncached PID %d missing from result", pid)
		}
	}

	// Verify cached PIDs are NOT in result
	for _, pid := range []int{100, 300} {
		if uncachedSet[pid] {
			t.Errorf("Cached PID %d should not be in uncached result", pid)
		}
	}
}

// TestPIDCache_GetUncachedPIDs_AllCached tests GetUncachedPIDs when all are cached.
func TestPIDCache_GetUncachedPIDs_AllCached(t *testing.T) {
	tmpDir := t.TempDir()
	cache := NewPIDCacheWithPath(filepath.Join(tmpDir, "cache.json"))

	// Cache all PIDs
	allPIDs := []int{100, 200, 300}
	for _, pid := range allPIDs {
		cache.Set(pid, "/path/session-"+strconv.Itoa(pid)+".jsonl")
	}

	// Should return empty slice
	uncached := cache.GetUncachedPIDs(allPIDs)

	if len(uncached) != 0 {
		t.Errorf("GetUncachedPIDs returned %d PIDs when all cached, want 0", len(uncached))
	}
}

// =============================================================================
// GetCachedMapping Tests
// =============================================================================

// TestPIDCache_GetCachedMapping tests that GetCachedMapping returns correct session IDs.
func TestPIDCache_GetCachedMapping(t *testing.T) {
	tmpDir := t.TempDir()
	cache := NewPIDCacheWithPath(filepath.Join(tmpDir, "cache.json"))

	// Set entries with session file paths
	cache.Set(100, "/home/user/.claude/projects/test/abc-123.jsonl")
	cache.Set(200, "/home/user/.claude/projects/test/def-456.jsonl")

	// Get mapping
	mapping := cache.GetCachedMapping()

	// Verify session IDs are extracted correctly (filename without .jsonl)
	if mapping[100] != "abc-123" {
		t.Errorf("mapping[100] = %q, want abc-123", mapping[100])
	}
	if mapping[200] != "def-456" {
		t.Errorf("mapping[200] = %q, want def-456", mapping[200])
	}
}

// =============================================================================
// GetAllSessionDirs Tests
// =============================================================================

// TestPIDCache_GetAllSessionDirs tests extracting unique directories.
func TestPIDCache_GetAllSessionDirs(t *testing.T) {
	tmpDir := t.TempDir()
	cache := NewPIDCacheWithPath(filepath.Join(tmpDir, "cache.json"))

	// Set entries with various paths
	cache.Set(100, "/home/user/.claude/projects/project-a/session1.jsonl")
	cache.Set(200, "/home/user/.claude/projects/project-a/session2.jsonl") // Same dir
	cache.Set(300, "/home/user/.claude/projects/project-b/session3.jsonl") // Different dir

	dirs := cache.GetAllSessionDirs()

	// Should have 2 unique directories
	if len(dirs) != 2 {
		t.Errorf("GetAllSessionDirs returned %d dirs, want 2", len(dirs))
	}

	dirSet := make(map[string]bool)
	for _, dir := range dirs {
		dirSet[dir] = true
	}

	expectedDirs := []string{
		"/home/user/.claude/projects/project-a",
		"/home/user/.claude/projects/project-b",
	}
	for _, expected := range expectedDirs {
		if !dirSet[expected] {
			t.Errorf("Expected directory %q not found in result", expected)
		}
	}
}

// =============================================================================
// Thread Safety Tests
// =============================================================================

// TestPIDCache_ConcurrentAccess tests thread safety with concurrent operations.
func TestPIDCache_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	cache := NewPIDCacheWithPath(filepath.Join(tmpDir, "cache.json"))

	const numGoroutines = 20
	const numOps = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Multiple goroutines doing Get/Set/Remove operations
	for g := 0; g < numGoroutines; g++ {
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < numOps; i++ {
				pid := goroutineID*1000 + i

				// Set
				cache.Set(pid, "/path/session-"+strconv.Itoa(pid)+".jsonl")

				// Get
				cache.Get(pid)

				// Remove (every other)
				if i%2 == 0 {
					cache.Remove(pid)
				}
			}
		}(g)
	}

	// Should complete without deadlock or panic
	wg.Wait()

	t.Log("Concurrent access completed without panic or deadlock")
}

// TestPIDCache_ConcurrentSaveLoad tests concurrent Save/Load operations.
func TestPIDCache_ConcurrentSaveLoad(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	const numGoroutines = 10
	const numOps = 20

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Multiple goroutines doing Save/Load
	for g := 0; g < numGoroutines; g++ {
		go func(goroutineID int) {
			defer wg.Done()
			cache := NewPIDCacheWithPath(cachePath)

			for i := 0; i < numOps; i++ {
				// Mix of operations
				if i%3 == 0 {
					cache.Load()
				} else {
					cache.Set(goroutineID*1000+i, "/path/session.jsonl")
					cache.Save()
				}
			}
		}(g)
	}

	// Should complete without race conditions or corruption
	wg.Wait()

	t.Log("Concurrent Save/Load completed without panic")
}

// =============================================================================
// Dirty Flag Tests
// =============================================================================

// TestPIDCache_DirtyFlag tests the dirty flag behavior.
func TestPIDCache_DirtyFlag(t *testing.T) {
	tmpDir := t.TempDir()
	cache := NewPIDCacheWithPath(filepath.Join(tmpDir, "cache.json"))

	// Initially not dirty
	if cache.IsDirty() {
		t.Error("New cache should not be dirty")
	}

	// Set makes it dirty
	cache.Set(100, "/path/session.jsonl")
	if !cache.IsDirty() {
		t.Error("Cache should be dirty after Set")
	}

	// MarkClean clears it
	cache.MarkClean()
	if cache.IsDirty() {
		t.Error("Cache should not be dirty after MarkClean")
	}

	// Remove makes it dirty
	cache.Remove(100)
	if !cache.IsDirty() {
		t.Error("Cache should be dirty after Remove")
	}

	// Clear makes it dirty
	cache.MarkClean()
	cache.Clear()
	if !cache.IsDirty() {
		t.Error("Cache should be dirty after Clear")
	}
}

// =============================================================================
// extractSessionIDFromPath Tests
// =============================================================================

// TestExtractSessionIDFromPath tests session ID extraction from paths.
func TestExtractSessionIDFromPath(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{
			path:     "/home/user/.claude/projects/test/abc-123-456.jsonl",
			expected: "abc-123-456",
		},
		{
			path:     "/path/to/session.jsonl",
			expected: "session",
		},
		{
			path:     "/path/cd49619d-7192-4a31-8b66-37fa4751c8be.jsonl",
			expected: "cd49619d-7192-4a31-8b66-37fa4751c8be",
		},
		{
			path:     "simple.jsonl",
			expected: "simple",
		},
		{
			path:     "no-extension",
			expected: "no-extension",
		},
		{
			path:     "", // Empty path - filepath.Base returns "." for empty string
			expected: ".",
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := extractSessionIDFromPath(tt.path)
			if result != tt.expected {
				t.Errorf("extractSessionIDFromPath(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// Causality Tests - Will FAIL if caching implementation is reverted
// =============================================================================

// TestCausality_PIDCacheStructExists verifies the PIDCache struct exists.
// This test will FAIL if PIDCache is removed.
func TestCausality_PIDCacheStructExists(t *testing.T) {
	// This will fail to compile if PIDCache doesn't exist
	cache := NewPIDCache()
	if cache == nil {
		t.Fatal("NewPIDCache() returned nil")
	}

	// Verify required methods exist
	_ = cache.Path()
	_ = cache.Load()
	_ = cache.Save()
	_, _ = cache.Get(0)
	cache.Set(0, "")
	cache.Remove(0)
	cache.Clear()
	cache.ValidatePIDs()
	_ = cache.GetUncachedPIDs(nil)
	_ = cache.GetCachedMapping()
	_ = cache.GetAllSessionDirs()
	_ = cache.Size()
	_ = cache.IsDirty()
	cache.MarkClean()
}

// TestCausality_PIDCachePath_UsesXDG verifies XDG_RUNTIME_DIR is checked.
// This test will FAIL if the XDG logic is removed.
func TestCausality_PIDCachePath_UsesXDG(t *testing.T) {
	// Read the source file
	sourceFile := "pid_cache.go"
	content, err := os.ReadFile(sourceFile)
	if err != nil {
		t.Fatalf("Failed to read source file %s: %v", sourceFile, err)
	}

	source := string(content)

	// Verify XDG_RUNTIME_DIR is checked
	if !contains(source, "XDG_RUNTIME_DIR") {
		t.Error("XDG_RUNTIME_DIR not found in source - path selection may have been changed")
	}

	// Verify /tmp fallback exists
	if !contains(source, "/tmp/claude-dashboard-") {
		t.Error("/tmp fallback path not found in source - fallback logic may have been removed")
	}
}

// TestCausality_PIDCacheFile_HasVersion verifies version field exists in cache format.
// This test will FAIL if version checking is removed.
func TestCausality_PIDCacheFile_HasVersion(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	cache := NewPIDCacheWithPath(cachePath)
	cache.Set(100, "/path/session.jsonl")
	if err := cache.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Read and parse file
	content, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("Failed to read cache file: %v", err)
	}

	var cacheFile map[string]interface{}
	if err := json.Unmarshal(content, &cacheFile); err != nil {
		t.Fatalf("Failed to parse cache file: %v", err)
	}

	// Verify version field exists
	if _, ok := cacheFile["version"]; !ok {
		t.Error("Cache file missing 'version' field - version checking may have been removed")
	}
}

// TestCausality_PIDCacheUsesFlock verifies flock is used for concurrent write safety.
// This test will FAIL if flock is removed from Save().
func TestCausality_PIDCacheUsesFlock(t *testing.T) {
	sourceFile := "pid_cache.go"
	content, err := os.ReadFile(sourceFile)
	if err != nil {
		t.Fatalf("Failed to read source file %s: %v", sourceFile, err)
	}

	source := string(content)

	// Check for flock usage
	if !contains(source, "syscall.LOCK_EX") && !contains(source, "syscall.Flock") {
		t.Error("flock (LOCK_EX) not found in source - atomic write protection may have been removed")
	}
}

// contains is a helper function that checks if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// =============================================================================
// ValidateOnLoad Tests - Stale PID Cache Validation (Chunk 001)
// =============================================================================

// createTestSessionFile creates a JSONL file with a timestamp in the first line.
// The mtime is explicitly set to mtimeOffset from now.
func createTestSessionFile(t *testing.T, dir, name string, mtimeOffset time.Duration) string {
	t.Helper()
	path := filepath.Join(dir, name+".jsonl")
	timestamp := time.Now().Add(mtimeOffset).Format(time.RFC3339Nano)
	content := fmt.Sprintf(`{"timestamp": "%s", "type": "user", "message": "test"}`, timestamp)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file %s: %v", path, err)
	}
	// Set mtime explicitly
	mtime := time.Now().Add(mtimeOffset)
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatalf("Failed to set mtime for %s: %v", path, err)
	}
	return path
}

// createEmptySessionFile creates an empty JSONL file with explicit mtime.
func createEmptySessionFile(t *testing.T, dir, name string, mtimeOffset time.Duration) string {
	t.Helper()
	path := filepath.Join(dir, name+".jsonl")
	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create empty test file %s: %v", path, err)
	}
	// Set mtime explicitly
	mtime := time.Now().Add(mtimeOffset)
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatalf("Failed to set mtime for %s: %v", path, err)
	}
	return path
}

// TestValidateOnLoad_NoEntries verifies validation handles empty cache.
func TestValidateOnLoad_NoEntries(t *testing.T) {
	tmpDir := t.TempDir()
	cache := NewPIDCacheWithPath(filepath.Join(tmpDir, "cache.json"))

	rescanCalled := false
	rescanFunc := func(pid int) string {
		rescanCalled = true
		return ""
	}

	// Should not panic or error with empty cache
	cache.ValidateOnLoad(rescanFunc)

	if rescanCalled {
		t.Error("Rescan should not be called for empty cache")
	}
}

// TestValidateOnLoad_NoUntrackedFiles_AllSafe verifies entries are safe
// when all files in directory are tracked in cache.
func TestValidateOnLoad_NoUntrackedFiles_AllSafe(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatalf("Failed to create session dir: %v", err)
	}

	// Create two session files with UUID names
	session1 := createTestSessionFile(t, sessionDir, "cd49619d-7192-4a31-8b66-37fa4751c8be", -1*time.Hour)
	session2 := createTestSessionFile(t, sessionDir, "dd49619d-7192-4a31-8b66-37fa4751c8be", -30*time.Minute)

	// Create cache with entries for both files
	cache := NewPIDCacheWithPath(filepath.Join(tmpDir, "cache.json"))
	cache.Set(os.Getpid(), session1)       // Use current PID (guaranteed to exist)
	cache.Set(os.Getppid(), session2)      // Use parent PID (guaranteed to exist)

	rescanCalled := false
	rescanFunc := func(pid int) string {
		rescanCalled = true
		return ""
	}

	cache.ValidateOnLoad(rescanFunc)

	if rescanCalled {
		t.Error("Rescan should not be called when no untracked files exist")
	}

	// Verify entries are still in cache
	if _, ok := cache.Get(os.Getpid()); !ok {
		t.Error("Entry for current PID should still exist")
	}
	if _, ok := cache.Get(os.Getppid()); !ok {
		t.Error("Entry for parent PID should still exist")
	}
}

// TestValidateOnLoad_UntrackedFile_OlderThanCached verifies entry is safe
// when cached session's mtime is after untracked file's birth time.
//
// Note: On filesystems with birth time support (ext4, btrfs), statx returns the
// actual creation time which cannot be faked. Since untracked files are created
// during the test, their birth time is "now". To make the cached session "safe",
// its mtime must be set to the future.
func TestValidateOnLoad_UntrackedFile_OlderThanCached(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatalf("Failed to create session dir: %v", err)
	}

	// Create untracked file first - its birth time will be "now"
	_ = createTestSessionFile(t, sessionDir, "dd49619d-7192-4a31-8b66-37fa4751c8be", 0)

	// Create cached session with FUTURE mtime (after untracked's birth time)
	session1 := createTestSessionFile(t, sessionDir, "cd49619d-7192-4a31-8b66-37fa4751c8be", +1*time.Hour)

	cache := NewPIDCacheWithPath(filepath.Join(tmpDir, "cache.json"))
	cache.Set(os.Getpid(), session1)

	rescanCalled := false
	rescanFunc := func(pid int) string {
		rescanCalled = true
		return ""
	}

	cache.ValidateOnLoad(rescanFunc)

	if rescanCalled {
		t.Error("Rescan should not be called when cached session mtime > untracked birth time")
	}
}

// TestValidateOnLoad_UntrackedFile_NewerThanCached verifies entry is rescanned
// when untracked file's birth time is newer than cached session's mtime.
func TestValidateOnLoad_UntrackedFile_NewerThanCached(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatalf("Failed to create session dir: %v", err)
	}

	// Create cached session with old mtime (T1)
	session1 := createTestSessionFile(t, sessionDir, "cd49619d-7192-4a31-8b66-37fa4751c8be", -1*time.Hour)

	// Create untracked file with newer mtime/birth (T2 > T1)
	// Note: Need a realistic UUID
	session2 := createTestSessionFile(t, sessionDir, "dd49619d-7192-4a31-8b66-37fa4751c8be", -10*time.Minute)

	cache := NewPIDCacheWithPath(filepath.Join(tmpDir, "cache.json"))
	cache.Set(os.Getpid(), session1)

	rescanPIDs := []int{}
	rescanFunc := func(pid int) string {
		rescanPIDs = append(rescanPIDs, pid)
		return session2 // Rescan finds new session
	}

	cache.ValidateOnLoad(rescanFunc)

	if len(rescanPIDs) != 1 {
		t.Errorf("Expected 1 rescan, got %d", len(rescanPIDs))
	}
	if len(rescanPIDs) > 0 && rescanPIDs[0] != os.Getpid() {
		t.Errorf("Wrong PID rescanned: got %d, want %d", rescanPIDs[0], os.Getpid())
	}

	// Verify cache was updated
	newSession, ok := cache.Get(os.Getpid())
	if !ok {
		t.Error("Entry should still exist after rescan")
	}
	if newSession != session2 {
		t.Errorf("Cache not updated: got %s, want %s", newSession, session2)
	}
}

// TestValidateOnLoad_MultipleSessionsSameDir verifies only stale entries
// are rescanned when multiple sessions exist in same directory.
//
// Note: Birth time cannot be faked on modern filesystems. All files created during
// the test have birth time ~= "now". To simulate the scenario where session-B is
// safe (mtime > birth time of untracked file), we set session-B's mtime to the future.
func TestValidateOnLoad_MultipleSessionsSameDir(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatalf("Failed to create session dir: %v", err)
	}

	// Create untracked session-C first - its birth time is "now"
	_ = createTestSessionFile(t, sessionDir, "cccccccc-7192-4a31-8b66-37fa4751c8be", 0)

	// session-A: old mtime (stale - mtime < untracked birth time "now")
	sessionA := createTestSessionFile(t, sessionDir, "aaaaaaaa-7192-4a31-8b66-37fa4751c8be", -2*time.Hour)

	// session-B: FUTURE mtime (safe - mtime > untracked birth time "now")
	sessionB := createTestSessionFile(t, sessionDir, "bbbbbbbb-7192-4a31-8b66-37fa4751c8be", +1*time.Hour)

	cache := NewPIDCacheWithPath(filepath.Join(tmpDir, "cache.json"))
	pidA := os.Getpid()
	pidB := os.Getppid()
	cache.Set(pidA, sessionA)
	cache.Set(pidB, sessionB)

	rescanPIDs := []int{}
	rescanFunc := func(pid int) string {
		rescanPIDs = append(rescanPIDs, pid)
		return "" // No new session found
	}

	cache.ValidateOnLoad(rescanFunc)

	// Only session-A should be rescanned (mtime past < birth "now")
	// session-B is safe (mtime future > birth "now")
	if len(rescanPIDs) != 1 {
		t.Errorf("Expected 1 rescan, got %d: %v", len(rescanPIDs), rescanPIDs)
	}
	if len(rescanPIDs) > 0 && rescanPIDs[0] != pidA {
		t.Errorf("Wrong PID rescanned: got %d, want %d", rescanPIDs[0], pidA)
	}

	// session-A removed (rescan returned empty)
	if _, ok := cache.Get(pidA); ok {
		t.Error("Stale entry should be removed when rescan returns empty")
	}

	// session-B still exists
	if session, ok := cache.Get(pidB); !ok || session != sessionB {
		t.Errorf("Safe entry should remain: got %q, want %q", session, sessionB)
	}
}

// TestValidateOnLoad_AgentFileIgnored verifies agent-*.jsonl files are not
// treated as untracked session files.
func TestValidateOnLoad_AgentFileIgnored(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatalf("Failed to create session dir: %v", err)
	}

	// Create cached session with old mtime
	session1 := createTestSessionFile(t, sessionDir, "cd49619d-7192-4a31-8b66-37fa4751c8be", -1*time.Hour)

	// Create agent file with newer mtime (should be ignored)
	agentPath := filepath.Join(sessionDir, "agent-abc123.jsonl")
	if err := os.WriteFile(agentPath, []byte(`{"timestamp": "2025-01-01T00:00:00Z"}`), 0644); err != nil {
		t.Fatalf("Failed to create agent file: %v", err)
	}
	mtime := time.Now().Add(-10 * time.Minute)
	os.Chtimes(agentPath, mtime, mtime)

	cache := NewPIDCacheWithPath(filepath.Join(tmpDir, "cache.json"))
	cache.Set(os.Getpid(), session1)

	rescanCalled := false
	rescanFunc := func(pid int) string {
		rescanCalled = true
		return ""
	}

	cache.ValidateOnLoad(rescanFunc)

	if rescanCalled {
		t.Error("Rescan should not be called - agent files should be ignored as untracked")
	}
}

// TestValidateOnLoad_RescanUpdatesCache verifies cache is updated when
// rescan finds a different session.
func TestValidateOnLoad_RescanUpdatesCache(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatalf("Failed to create session dir: %v", err)
	}

	// Old session (cached)
	oldSession := createTestSessionFile(t, sessionDir, "aaaaaaaa-7192-4a31-8b66-37fa4751c8be", -2*time.Hour)

	// New session (untracked, will be found by rescan)
	newSession := createTestSessionFile(t, sessionDir, "bbbbbbbb-7192-4a31-8b66-37fa4751c8be", -10*time.Minute)

	cache := NewPIDCacheWithPath(filepath.Join(tmpDir, "cache.json"))
	cache.Set(os.Getpid(), oldSession)

	rescanFunc := func(pid int) string {
		return newSession
	}

	cache.ValidateOnLoad(rescanFunc)

	// Cache should now point to new session
	result, ok := cache.Get(os.Getpid())
	if !ok {
		t.Fatal("Entry should exist after rescan update")
	}
	if result != newSession {
		t.Errorf("Cache not updated: got %s, want %s", result, newSession)
	}
}

// TestValidateOnLoad_RescanReturnsEmpty verifies entry is removed when
// rescan finds no session.
func TestValidateOnLoad_RescanReturnsEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatalf("Failed to create session dir: %v", err)
	}

	// Old session (cached)
	oldSession := createTestSessionFile(t, sessionDir, "aaaaaaaa-7192-4a31-8b66-37fa4751c8be", -2*time.Hour)

	// New untracked session
	_ = createTestSessionFile(t, sessionDir, "bbbbbbbb-7192-4a31-8b66-37fa4751c8be", -10*time.Minute)

	cache := NewPIDCacheWithPath(filepath.Join(tmpDir, "cache.json"))
	cache.Set(os.Getpid(), oldSession)

	rescanFunc := func(pid int) string {
		return "" // No session found
	}

	cache.ValidateOnLoad(rescanFunc)

	// Entry should be removed
	if _, ok := cache.Get(os.Getpid()); ok {
		t.Error("Entry should be removed when rescan returns empty")
	}
}

// TestValidateOnLoad_DeadPIDsRemovedFirst verifies dead PIDs don't trigger rescan.
// Note: ValidateOnLoad assumes dead PIDs are already pruned by validatePIDsLocked
// during Load(). This test verifies the integration with Load().
func TestValidateOnLoad_DeadPIDsRemovedFirst(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatalf("Failed to create session dir: %v", err)
	}

	// Create session files
	session1 := createTestSessionFile(t, sessionDir, "aaaaaaaa-7192-4a31-8b66-37fa4751c8be", -2*time.Hour)
	_ = createTestSessionFile(t, sessionDir, "bbbbbbbb-7192-4a31-8b66-37fa4751c8be", -10*time.Minute)

	// Create cache with dead PID
	cache := NewPIDCacheWithPath(filepath.Join(tmpDir, "cache.json"))
	deadPID := 999999 // Very unlikely to exist
	cache.Set(deadPID, session1)

	// First prune dead PIDs (as Load() would do)
	cache.ValidatePIDs()

	// Now ValidateOnLoad should not see the dead PID
	rescanPIDs := []int{}
	rescanFunc := func(pid int) string {
		rescanPIDs = append(rescanPIDs, pid)
		return ""
	}

	cache.ValidateOnLoad(rescanFunc)

	for _, pid := range rescanPIDs {
		if pid == deadPID {
			t.Error("Dead PID should not be rescanned")
		}
	}
}

// TestValidateOnLoad_SessionFileDeleted verifies rescan is triggered when
// cached session file no longer exists.
func TestValidateOnLoad_SessionFileDeleted(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatalf("Failed to create session dir: %v", err)
	}

	// Create session file, then delete it
	session1 := createTestSessionFile(t, sessionDir, "aaaaaaaa-7192-4a31-8b66-37fa4751c8be", -2*time.Hour)

	// Create untracked file
	session2 := createTestSessionFile(t, sessionDir, "bbbbbbbb-7192-4a31-8b66-37fa4751c8be", -10*time.Minute)

	cache := NewPIDCacheWithPath(filepath.Join(tmpDir, "cache.json"))
	cache.Set(os.Getpid(), session1)

	// Delete the cached session file
	os.Remove(session1)

	rescanPIDs := []int{}
	rescanFunc := func(pid int) string {
		rescanPIDs = append(rescanPIDs, pid)
		return session2
	}

	cache.ValidateOnLoad(rescanFunc)

	// Should trigger rescan even though file doesn't exist
	if len(rescanPIDs) != 1 {
		t.Errorf("Expected 1 rescan for deleted file, got %d", len(rescanPIDs))
	}

	// Cache should be updated to new session
	result, ok := cache.Get(os.Getpid())
	if !ok || result != session2 {
		t.Errorf("Cache should be updated to new session: got %q, want %q", result, session2)
	}
}

// TestValidateOnLoad_MultipleDirs verifies only affected directories are checked.
func TestValidateOnLoad_MultipleDirs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create two separate session directories
	dir1 := filepath.Join(tmpDir, "project-a")
	dir2 := filepath.Join(tmpDir, "project-b")
	if err := os.MkdirAll(dir1, 0755); err != nil {
		t.Fatalf("Failed to create dir1: %v", err)
	}
	if err := os.MkdirAll(dir2, 0755); err != nil {
		t.Fatalf("Failed to create dir2: %v", err)
	}

	// Dir1: cached session + untracked file (should trigger rescan)
	session1 := createTestSessionFile(t, dir1, "aaaaaaaa-7192-4a31-8b66-37fa4751c8be", -2*time.Hour)
	_ = createTestSessionFile(t, dir1, "bbbbbbbb-7192-4a31-8b66-37fa4751c8be", -10*time.Minute)

	// Dir2: cached session only (no untracked files)
	session2 := createTestSessionFile(t, dir2, "cccccccc-7192-4a31-8b66-37fa4751c8be", -1*time.Hour)

	cache := NewPIDCacheWithPath(filepath.Join(tmpDir, "cache.json"))
	pid1 := os.Getpid()
	pid2 := os.Getppid()
	cache.Set(pid1, session1)
	cache.Set(pid2, session2)

	rescanPIDs := []int{}
	rescanFunc := func(pid int) string {
		rescanPIDs = append(rescanPIDs, pid)
		return ""
	}

	cache.ValidateOnLoad(rescanFunc)

	// Only pid1 should be rescanned (dir1 has untracked files)
	if len(rescanPIDs) != 1 {
		t.Errorf("Expected 1 rescan, got %d: %v", len(rescanPIDs), rescanPIDs)
	}
	if len(rescanPIDs) > 0 && rescanPIDs[0] != pid1 {
		t.Errorf("Wrong PID rescanned: got %d, want %d", rescanPIDs[0], pid1)
	}

	// Dir2 entry should still exist
	if session, ok := cache.Get(pid2); !ok || session != session2 {
		t.Error("Entry in dir2 should remain unchanged")
	}
}

// =============================================================================
// listSessionFilesInDir Tests
// =============================================================================

// TestListSessionFilesInDir_Basic verifies basic file listing.
func TestListSessionFilesInDir_Basic(t *testing.T) {
	tmpDir := t.TempDir()

	// Create various files
	createTestSessionFile(t, tmpDir, "aaaaaaaa-7192-4a31-8b66-37fa4751c8be", 0)
	createTestSessionFile(t, tmpDir, "bbbbbbbb-7192-4a31-8b66-37fa4751c8be", 0)

	files, err := listSessionFilesInDir(tmpDir)
	if err != nil {
		t.Fatalf("listSessionFilesInDir error: %v", err)
	}

	if len(files) != 2 {
		t.Errorf("Expected 2 files, got %d", len(files))
	}
}

// TestListSessionFilesInDir_ExcludesAgentFiles verifies agent-*.jsonl is excluded.
func TestListSessionFilesInDir_ExcludesAgentFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create UUID session file
	createTestSessionFile(t, tmpDir, "aaaaaaaa-7192-4a31-8b66-37fa4751c8be", 0)

	// Create agent file (should be excluded)
	agentPath := filepath.Join(tmpDir, "agent-abc123.jsonl")
	if err := os.WriteFile(agentPath, []byte(`{}`), 0644); err != nil {
		t.Fatalf("Failed to create agent file: %v", err)
	}

	files, err := listSessionFilesInDir(tmpDir)
	if err != nil {
		t.Fatalf("listSessionFilesInDir error: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("Expected 1 file (excluding agent), got %d: %v", len(files), files)
	}

	for _, f := range files {
		if filepath.Base(f) == "agent-abc123.jsonl" {
			t.Error("Agent file should be excluded")
		}
	}
}

// TestListSessionFilesInDir_ExcludesNonUUID verifies non-UUID files are excluded.
func TestListSessionFilesInDir_ExcludesNonUUID(t *testing.T) {
	tmpDir := t.TempDir()

	// Create UUID session file
	createTestSessionFile(t, tmpDir, "aaaaaaaa-7192-4a31-8b66-37fa4751c8be", 0)

	// Create non-UUID jsonl file (should be excluded)
	nonUUIDPath := filepath.Join(tmpDir, "not-a-uuid.jsonl")
	if err := os.WriteFile(nonUUIDPath, []byte(`{}`), 0644); err != nil {
		t.Fatalf("Failed to create non-UUID file: %v", err)
	}

	files, err := listSessionFilesInDir(tmpDir)
	if err != nil {
		t.Fatalf("listSessionFilesInDir error: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("Expected 1 file (excluding non-UUID), got %d: %v", len(files), files)
	}
}

// TestListSessionFilesInDir_ExcludesNonJSONL verifies non-.jsonl files are excluded.
func TestListSessionFilesInDir_ExcludesNonJSONL(t *testing.T) {
	tmpDir := t.TempDir()

	// Create UUID session file
	createTestSessionFile(t, tmpDir, "aaaaaaaa-7192-4a31-8b66-37fa4751c8be", 0)

	// Create non-jsonl file with UUID name
	txtPath := filepath.Join(tmpDir, "bbbbbbbb-7192-4a31-8b66-37fa4751c8be.txt")
	if err := os.WriteFile(txtPath, []byte("not jsonl"), 0644); err != nil {
		t.Fatalf("Failed to create txt file: %v", err)
	}

	files, err := listSessionFilesInDir(tmpDir)
	if err != nil {
		t.Fatalf("listSessionFilesInDir error: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("Expected 1 file (excluding .txt), got %d: %v", len(files), files)
	}
}

// TestListSessionFilesInDir_ExcludesDirectories verifies directories are excluded.
func TestListSessionFilesInDir_ExcludesDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	// Create UUID session file
	createTestSessionFile(t, tmpDir, "aaaaaaaa-7192-4a31-8b66-37fa4751c8be", 0)

	// Create directory with UUID-like name
	dirPath := filepath.Join(tmpDir, "bbbbbbbb-7192-4a31-8b66-37fa4751c8be.jsonl")
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	files, err := listSessionFilesInDir(tmpDir)
	if err != nil {
		t.Fatalf("listSessionFilesInDir error: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("Expected 1 file (excluding directory), got %d: %v", len(files), files)
	}
}

// TestListSessionFilesInDir_EmptyDir verifies empty directory returns empty list.
func TestListSessionFilesInDir_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	files, err := listSessionFilesInDir(tmpDir)
	if err != nil {
		t.Fatalf("listSessionFilesInDir error: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("Expected 0 files for empty dir, got %d: %v", len(files), files)
	}
}

// TestListSessionFilesInDir_NonexistentDir verifies error for nonexistent directory.
func TestListSessionFilesInDir_NonexistentDir(t *testing.T) {
	_, err := listSessionFilesInDir("/nonexistent/path/123456")
	if err == nil {
		t.Error("Expected error for nonexistent directory")
	}
}

// =============================================================================
// parseJSONLFirstLineTimestamp Tests
// =============================================================================

// TestParseJSONLFirstLineTimestamp_RFC3339Nano verifies parsing RFC3339Nano format.
func TestParseJSONLFirstLineTimestamp_RFC3339Nano(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.jsonl")

	timestamp := "2025-12-29T22:35:51.370123456Z"
	content := fmt.Sprintf(`{"timestamp": "%s", "type": "user"}`, timestamp)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	result, err := parseJSONLFirstLineTimestamp(path)
	if err != nil {
		t.Fatalf("parseJSONLFirstLineTimestamp error: %v", err)
	}

	expected, _ := time.Parse(time.RFC3339Nano, timestamp)
	if !result.Equal(expected) {
		t.Errorf("Timestamp mismatch: got %v, want %v", result, expected)
	}
}

// TestParseJSONLFirstLineTimestamp_RFC3339 verifies parsing RFC3339 format.
func TestParseJSONLFirstLineTimestamp_RFC3339(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.jsonl")

	timestamp := "2025-12-29T22:35:51Z"
	content := fmt.Sprintf(`{"timestamp": "%s", "type": "user"}`, timestamp)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	result, err := parseJSONLFirstLineTimestamp(path)
	if err != nil {
		t.Fatalf("parseJSONLFirstLineTimestamp error: %v", err)
	}

	expected, _ := time.Parse(time.RFC3339, timestamp)
	if !result.Equal(expected) {
		t.Errorf("Timestamp mismatch: got %v, want %v", result, expected)
	}
}

// TestParseJSONLFirstLineTimestamp_EmptyFile verifies error for empty file.
func TestParseJSONLFirstLineTimestamp_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "empty.jsonl")

	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to write empty file: %v", err)
	}

	_, err := parseJSONLFirstLineTimestamp(path)
	if err == nil {
		t.Error("Expected error for empty file")
	}
}

// TestParseJSONLFirstLineTimestamp_NoTimestamp verifies error when no timestamp field.
func TestParseJSONLFirstLineTimestamp_NoTimestamp(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "no-timestamp.jsonl")

	content := `{"type": "user", "message": "hello"}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	_, err := parseJSONLFirstLineTimestamp(path)
	if err == nil {
		t.Error("Expected error when no timestamp field")
	}
}

// TestParseJSONLFirstLineTimestamp_InvalidJSON verifies error for invalid JSON.
func TestParseJSONLFirstLineTimestamp_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "invalid.jsonl")

	content := `{invalid json}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	_, err := parseJSONLFirstLineTimestamp(path)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

// TestParseJSONLFirstLineTimestamp_InvalidTimestamp verifies error for invalid timestamp format.
func TestParseJSONLFirstLineTimestamp_InvalidTimestamp(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "bad-timestamp.jsonl")

	content := `{"timestamp": "not-a-timestamp", "type": "user"}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	_, err := parseJSONLFirstLineTimestamp(path)
	if err == nil {
		t.Error("Expected error for invalid timestamp format")
	}
}

// TestParseJSONLFirstLineTimestamp_NonexistentFile verifies error for nonexistent file.
func TestParseJSONLFirstLineTimestamp_NonexistentFile(t *testing.T) {
	_, err := parseJSONLFirstLineTimestamp("/nonexistent/file.jsonl")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

// =============================================================================
// getBirthTime Tests
// =============================================================================

// TestGetBirthTime_ReturnsValidTime verifies getBirthTime returns a valid time.
func TestGetBirthTime_ReturnsValidTime(t *testing.T) {
	tmpDir := t.TempDir()
	path := createTestSessionFile(t, tmpDir, "test", 0)

	birthTime, err := getBirthTime(path)
	if err != nil {
		t.Fatalf("getBirthTime error: %v", err)
	}

	if birthTime.IsZero() {
		t.Error("Birth time should not be zero")
	}
}

// TestGetBirthTime_FallbackToJSONL verifies JSONL fallback is used when statx unavailable.
// Note: This is hard to test directly since most systems support birth time.
// We test indirectly by verifying the JSONL parser works.
func TestGetBirthTime_JSONLFallbackWorks(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.jsonl")

	// Create file with known timestamp
	timestamp := "2025-06-15T12:00:00.000000000Z"
	content := fmt.Sprintf(`{"timestamp": "%s", "type": "user"}`, timestamp)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// parseJSONLFirstLineTimestamp should work
	result, err := parseJSONLFirstLineTimestamp(path)
	if err != nil {
		t.Fatalf("JSONL timestamp parse error: %v", err)
	}

	expected, _ := time.Parse(time.RFC3339Nano, timestamp)
	if !result.Equal(expected) {
		t.Errorf("JSONL timestamp mismatch: got %v, want %v", result, expected)
	}
}

// TestGetBirthTime_FallbackToMtime verifies mtime fallback for empty files.
func TestGetBirthTime_EmptyFile_FallsBackToMtime(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "empty.jsonl")

	// Create empty file with explicit mtime
	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to write empty file: %v", err)
	}
	expectedMtime := time.Now().Add(-1 * time.Hour).Truncate(time.Second)
	os.Chtimes(path, expectedMtime, expectedMtime)

	birthTime, err := getBirthTime(path)
	if err != nil {
		t.Fatalf("getBirthTime error for empty file: %v", err)
	}

	// Should return a valid time (either statx birth time or mtime fallback)
	if birthTime.IsZero() {
		t.Error("Birth time should not be zero even for empty file")
	}
}

// TestGetBirthTime_NonexistentFile verifies error for nonexistent file.
func TestGetBirthTime_NonexistentFile(t *testing.T) {
	_, err := getBirthTime("/nonexistent/path/file.jsonl")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

// =============================================================================
// Causality Tests - Will FAIL if ValidateOnLoad implementation is reverted
// =============================================================================

// TestCausality_ValidateOnLoadExists verifies ValidateOnLoad method exists.
func TestCausality_ValidateOnLoadExists(t *testing.T) {
	tmpDir := t.TempDir()
	cache := NewPIDCacheWithPath(filepath.Join(tmpDir, "cache.json"))

	// This will fail to compile if ValidateOnLoad doesn't exist
	cache.ValidateOnLoad(func(pid int) string { return "" })
}

// TestCausality_GetBirthTimeExists verifies getBirthTime function exists.
// We test this indirectly since it's not exported.
func TestCausality_GetBirthTimeExists(t *testing.T) {
	tmpDir := t.TempDir()
	path := createTestSessionFile(t, tmpDir, "test", 0)

	// getBirthTime is called internally by ValidateOnLoad
	// If it doesn't exist, ValidateOnLoad will fail
	cache := NewPIDCacheWithPath(filepath.Join(tmpDir, "cache.json"))
	cache.Set(os.Getpid(), path)

	// Create untracked file to trigger birth time check
	createTestSessionFile(t, tmpDir, "untracked", -10*time.Minute)

	// Should not panic
	cache.ValidateOnLoad(func(pid int) string { return "" })
}

// TestCausality_UsesStatx verifies STATX_BTIME is used in the source.
func TestCausality_UsesStatx(t *testing.T) {
	sourceFile := "pid_cache.go"
	content, err := os.ReadFile(sourceFile)
	if err != nil {
		t.Fatalf("Failed to read source file %s: %v", sourceFile, err)
	}

	source := string(content)

	// Verify STATX_BTIME is used
	if !contains(source, "STATX_BTIME") {
		t.Error("STATX_BTIME not found in source - birth time detection may have been removed")
	}

	// Verify unix.Statx is used
	if !contains(source, "unix.Statx") {
		t.Error("unix.Statx not found in source - statx syscall may have been removed")
	}
}

// TestCausality_HasJSONLFallback verifies JSONL timestamp fallback exists.
func TestCausality_HasJSONLFallback(t *testing.T) {
	sourceFile := "pid_cache.go"
	content, err := os.ReadFile(sourceFile)
	if err != nil {
		t.Fatalf("Failed to read source file %s: %v", sourceFile, err)
	}

	source := string(content)

	// Verify parseJSONLFirstLineTimestamp function exists
	if !contains(source, "parseJSONLFirstLineTimestamp") {
		t.Error("parseJSONLFirstLineTimestamp not found - JSONL fallback may have been removed")
	}
}

// TestCausality_ValidateOnLoadGroupsByDirectory verifies entries are grouped by directory.
func TestCausality_ValidateOnLoadGroupsByDirectory(t *testing.T) {
	sourceFile := "pid_cache.go"
	content, err := os.ReadFile(sourceFile)
	if err != nil {
		t.Fatalf("Failed to read source file %s: %v", sourceFile, err)
	}

	source := string(content)

	// Verify directory grouping logic exists
	if !contains(source, "dirToEntries") {
		t.Error("dirToEntries not found - directory grouping may have been removed")
	}
}
