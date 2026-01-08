package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestCacheHit verifies that Get returns cached metadata when mtime matches.
func TestCacheHit(t *testing.T) {
	// Create a temporary cache file
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	cache := NewCache(cachePath)

	// Create test metadata
	testTime := time.Now().Truncate(time.Second)
	testMeta := &SessionMetadata{
		ID:           "test-session-123",
		FilePath:     "/path/to/session.jsonl",
		ProjectPath:  "/home/user/project",
		ProjectName:  "project",
		CWD:          "/home/user/project",
		Summary:      "Test session summary",
		Model:        "claude-sonnet-4-20250514",
		Preview:      "This is a preview",
		MessageCount: 10,
		TurnCount:    5,
		ModTime:      testTime,
	}

	// Set the entry
	cache.Set(testMeta.FilePath, testTime, testMeta)

	// Get with matching mtime should return the metadata
	result := cache.Get(testMeta.FilePath, testTime)
	if result == nil {
		t.Fatal("expected cache hit, got nil")
	}

	// Verify all fields match
	if result.ID != testMeta.ID {
		t.Errorf("ID mismatch: got %q, want %q", result.ID, testMeta.ID)
	}
	if result.FilePath != testMeta.FilePath {
		t.Errorf("FilePath mismatch: got %q, want %q", result.FilePath, testMeta.FilePath)
	}
	if result.ProjectPath != testMeta.ProjectPath {
		t.Errorf("ProjectPath mismatch: got %q, want %q", result.ProjectPath, testMeta.ProjectPath)
	}
	if result.Summary != testMeta.Summary {
		t.Errorf("Summary mismatch: got %q, want %q", result.Summary, testMeta.Summary)
	}
	if result.Model != testMeta.Model {
		t.Errorf("Model mismatch: got %q, want %q", result.Model, testMeta.Model)
	}
	if result.MessageCount != testMeta.MessageCount {
		t.Errorf("MessageCount mismatch: got %d, want %d", result.MessageCount, testMeta.MessageCount)
	}
	if result.TurnCount != testMeta.TurnCount {
		t.Errorf("TurnCount mismatch: got %d, want %d", result.TurnCount, testMeta.TurnCount)
	}
}

// TestCacheMissMtimeDiffers verifies that Get returns nil when mtime differs.
func TestCacheMissMtimeDiffers(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	cache := NewCache(cachePath)

	// Create test metadata
	cachedTime := time.Now().Truncate(time.Second)
	testMeta := &SessionMetadata{
		ID:           "test-session-123",
		FilePath:     "/path/to/session.jsonl",
		ProjectPath:  "/home/user/project",
		ProjectName:  "project",
		CWD:          "/home/user/project",
		Summary:      "Test session summary",
		Model:        "claude-sonnet-4-20250514",
		MessageCount: 10,
		TurnCount:    5,
		ModTime:      cachedTime,
	}

	// Set the entry with cached time
	cache.Set(testMeta.FilePath, cachedTime, testMeta)

	// Get with different mtime should return nil
	newerTime := cachedTime.Add(time.Hour)
	result := cache.Get(testMeta.FilePath, newerTime)
	if result != nil {
		t.Errorf("expected cache miss for different mtime, got %+v", result)
	}

	// Get with older mtime should also return nil
	olderTime := cachedTime.Add(-time.Hour)
	result = cache.Get(testMeta.FilePath, olderTime)
	if result != nil {
		t.Errorf("expected cache miss for older mtime, got %+v", result)
	}
}

// TestCacheMissPathNotInCache verifies that Get returns nil for unknown paths.
func TestCacheMissPathNotInCache(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	cache := NewCache(cachePath)

	// Create and set one entry
	testTime := time.Now().Truncate(time.Second)
	testMeta := &SessionMetadata{
		ID:       "test-session-123",
		FilePath: "/path/to/session.jsonl",
	}
	cache.Set(testMeta.FilePath, testTime, testMeta)

	// Get for a different path should return nil
	result := cache.Get("/different/path/session.jsonl", testTime)
	if result != nil {
		t.Errorf("expected nil for unknown path, got %+v", result)
	}

	// Get for empty cache
	emptyCache := NewCache(cachePath)
	result = emptyCache.Get("/any/path.jsonl", testTime)
	if result != nil {
		t.Errorf("expected nil from empty cache, got %+v", result)
	}
}

// TestCachePruning verifies that Prune removes entries not in validPaths.
func TestCachePruning(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	cache := NewCache(cachePath)

	testTime := time.Now().Truncate(time.Second)

	// Add multiple entries
	paths := []string{
		"/path/to/session1.jsonl",
		"/path/to/session2.jsonl",
		"/path/to/session3.jsonl",
		"/path/to/session4.jsonl",
	}

	for i, path := range paths {
		meta := &SessionMetadata{
			ID:       "session-" + string(rune('1'+i)),
			FilePath: path,
		}
		cache.Set(path, testTime, meta)
	}

	// Verify all 4 entries exist
	if cache.Size() != 4 {
		t.Fatalf("expected 4 entries, got %d", cache.Size())
	}

	// Prune - keep only session1 and session3
	validPaths := map[string]struct{}{
		"/path/to/session1.jsonl": {},
		"/path/to/session3.jsonl": {},
	}
	cache.Prune(validPaths)

	// Verify only 2 entries remain
	if cache.Size() != 2 {
		t.Errorf("expected 2 entries after prune, got %d", cache.Size())
	}

	// Verify correct entries remain
	if cache.Get(paths[0], testTime) == nil {
		t.Error("session1 should still be cached")
	}
	if cache.Get(paths[1], testTime) != nil {
		t.Error("session2 should have been pruned")
	}
	if cache.Get(paths[2], testTime) == nil {
		t.Error("session3 should still be cached")
	}
	if cache.Get(paths[3], testTime) != nil {
		t.Error("session4 should have been pruned")
	}

	// Verify cache is marked dirty after prune
	if !cache.IsDirty() {
		t.Error("cache should be dirty after prune")
	}
}

// TestCacheAtomicWrite verifies that atomic file writes prevent corruption.
// This test simulates what happens during a save operation to ensure
// the cache file is never left in a corrupted state.
func TestCacheAtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	cache := NewCache(cachePath)

	testTime := time.Now().Truncate(time.Second)
	testMeta := &SessionMetadata{
		ID:           "test-session",
		FilePath:     "/path/to/session.jsonl",
		ProjectPath:  "/home/user/project",
		ProjectName:  "project",
		CWD:          "/home/user/project",
		Summary:      "Test summary",
		Model:        "claude-sonnet-4-20250514",
		MessageCount: 42,
		TurnCount:    21,
		ModTime:      testTime,
	}

	cache.Set(testMeta.FilePath, testTime, testMeta)

	// Save should create the cache file
	if err := cache.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify the temp file doesn't exist (was renamed)
	tempPath := cachePath + ".tmp"
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Error("temp file should not exist after successful save")
	}

	// Verify the cache file exists and is valid JSON
	data, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("failed to read cache file: %v", err)
	}

	var cacheFile CacheFile
	if err := json.Unmarshal(data, &cacheFile); err != nil {
		t.Fatalf("cache file is not valid JSON: %v", err)
	}

	// Verify content
	if cacheFile.Version != 1 {
		t.Errorf("version mismatch: got %d, want 1", cacheFile.Version)
	}

	entry, exists := cacheFile.Entries[testMeta.FilePath]
	if !exists {
		t.Fatal("entry not found in saved cache")
	}

	if entry.Metadata.ID != testMeta.ID {
		t.Errorf("saved ID mismatch: got %q, want %q", entry.Metadata.ID, testMeta.ID)
	}
	if entry.Metadata.MessageCount != testMeta.MessageCount {
		t.Errorf("saved MessageCount mismatch: got %d, want %d", entry.Metadata.MessageCount, testMeta.MessageCount)
	}
}

// TestCacheLoadAndSave verifies that cache data survives a load/save cycle.
func TestCacheLoadAndSave(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	// Create and populate first cache instance
	cache1 := NewCache(cachePath)
	testTime := time.Now().Truncate(time.Second)

	entries := []SessionMetadata{
		{ID: "session-1", FilePath: "/path/1.jsonl", Summary: "First session"},
		{ID: "session-2", FilePath: "/path/2.jsonl", Summary: "Second session"},
		{ID: "session-3", FilePath: "/path/3.jsonl", Summary: "Third session"},
	}

	for _, meta := range entries {
		m := meta // Create a copy for the pointer
		cache1.Set(m.FilePath, testTime, &m)
	}

	if err := cache1.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Create new cache instance and load
	cache2 := NewCache(cachePath)
	if err := cache2.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify all entries were loaded
	if cache2.Size() != len(entries) {
		t.Errorf("size mismatch after load: got %d, want %d", cache2.Size(), len(entries))
	}

	for _, meta := range entries {
		result := cache2.Get(meta.FilePath, testTime)
		if result == nil {
			t.Errorf("entry for %s not found after load", meta.FilePath)
			continue
		}
		if result.ID != meta.ID {
			t.Errorf("ID mismatch for %s: got %q, want %q", meta.FilePath, result.ID, meta.ID)
		}
		if result.Summary != meta.Summary {
			t.Errorf("Summary mismatch for %s: got %q, want %q", meta.FilePath, result.Summary, meta.Summary)
		}
	}
}

// TestCacheLoadMissingFile verifies Load handles missing cache file gracefully.
func TestCacheLoadMissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "nonexistent", "cache.json")

	cache := NewCache(cachePath)
	if err := cache.Load(); err != nil {
		t.Errorf("Load should not error on missing file: %v", err)
	}

	if cache.Size() != 0 {
		t.Errorf("cache should be empty after loading missing file, got %d entries", cache.Size())
	}
}

// TestCacheLoadCorruptedFile verifies Load handles corrupted cache file.
func TestCacheLoadCorruptedFile(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	// Write corrupted JSON
	if err := os.WriteFile(cachePath, []byte("not valid json{{{"), 0644); err != nil {
		t.Fatalf("failed to write corrupted cache file: %v", err)
	}

	cache := NewCache(cachePath)
	if err := cache.Load(); err != nil {
		t.Errorf("Load should not error on corrupted file: %v", err)
	}

	// Cache should be empty (reset) after loading corrupted file
	if cache.Size() != 0 {
		t.Errorf("cache should be empty after loading corrupted file, got %d entries", cache.Size())
	}
}

// TestCacheLoadVersionMismatch verifies cache is reset on version mismatch.
func TestCacheLoadVersionMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	// Write cache file with old version
	oldCache := CacheFile{
		Version: 0, // Old version
		Entries: map[string]CacheEntry{
			"/path/session.jsonl": {
				Mtime:    time.Now().Unix(),
				Metadata: SessionMetadata{ID: "old-session"},
			},
		},
	}

	data, err := json.Marshal(oldCache)
	if err != nil {
		t.Fatalf("failed to marshal old cache: %v", err)
	}
	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		t.Fatalf("failed to write old cache file: %v", err)
	}

	cache := NewCache(cachePath)
	if err := cache.Load(); err != nil {
		t.Errorf("Load should not error on version mismatch: %v", err)
	}

	// Cache should be empty due to version mismatch
	if cache.Size() != 0 {
		t.Errorf("cache should be empty after version mismatch, got %d entries", cache.Size())
	}
}

// TestCacheSaveCreatesDirectory verifies Save creates parent directory.
func TestCacheSaveCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	// Use a nested path that doesn't exist
	cachePath := filepath.Join(tmpDir, "nested", "dir", "cache.json")

	cache := NewCache(cachePath)
	testTime := time.Now().Truncate(time.Second)
	meta := &SessionMetadata{ID: "test"}
	cache.Set("/test.jsonl", testTime, meta)

	if err := cache.Save(); err != nil {
		t.Fatalf("Save failed to create directory: %v", err)
	}

	// Verify the directory was created
	dir := filepath.Dir(cachePath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("Save should have created parent directories")
	}
}

// TestCacheConcurrentAccess verifies thread safety of cache operations.
func TestCacheConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	cache := NewCache(cachePath)

	var wg sync.WaitGroup
	numGoroutines := 100

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			testTime := time.Now().Truncate(time.Second)
			meta := &SessionMetadata{
				ID:       "session",
				FilePath: "/path/session.jsonl",
			}
			cache.Set(meta.FilePath, testTime, meta)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			testTime := time.Now().Truncate(time.Second)
			cache.Get("/path/session.jsonl", testTime)
		}(i)
	}

	// Concurrent size checks
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			cache.Size()
		}(i)
	}

	wg.Wait()

	// Cache should still be in a consistent state
	// (exactly 1 entry, since all writes are to the same path)
	if cache.Size() != 1 {
		t.Errorf("expected 1 entry after concurrent access, got %d", cache.Size())
	}
}

// TestCacheEmptyPrune verifies Prune with empty validPaths clears cache.
func TestCacheEmptyPrune(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	cache := NewCache(cachePath)
	testTime := time.Now().Truncate(time.Second)

	// Add entries
	cache.Set("/path/1.jsonl", testTime, &SessionMetadata{ID: "1"})
	cache.Set("/path/2.jsonl", testTime, &SessionMetadata{ID: "2"})

	// Prune with empty validPaths should remove all entries
	cache.Prune(make(map[string]struct{}))

	if cache.Size() != 0 {
		t.Errorf("expected 0 entries after empty prune, got %d", cache.Size())
	}
}

// TestCacheDirtyFlag verifies the dirty flag is set correctly.
func TestCacheDirtyFlag(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	cache := NewCache(cachePath)

	// New cache is not dirty
	if cache.IsDirty() {
		t.Error("new cache should not be dirty")
	}

	// After Set, cache should be dirty
	testTime := time.Now().Truncate(time.Second)
	cache.Set("/path/session.jsonl", testTime, &SessionMetadata{ID: "test"})
	if !cache.IsDirty() {
		t.Error("cache should be dirty after Set")
	}
}

// TestCachePath verifies Path() returns the configured path.
func TestCachePath(t *testing.T) {
	expectedPath := "/some/path/cache.json"
	cache := NewCache(expectedPath)

	if cache.Path() != expectedPath {
		t.Errorf("Path() mismatch: got %q, want %q", cache.Path(), expectedPath)
	}
}
