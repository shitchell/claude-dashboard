package tmux

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
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
