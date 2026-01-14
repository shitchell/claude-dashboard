package tmux

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// =============================================================================
// SessionWatcher Creation Tests
// =============================================================================

// TestSessionWatcher_New tests that NewSessionWatcher creates successfully.
func TestSessionWatcher_New(t *testing.T) {
	callbackCalled := false
	watcher, err := NewSessionWatcher(func(path string) {
		callbackCalled = true
	})
	if err != nil {
		t.Fatalf("NewSessionWatcher() error = %v", err)
	}
	defer watcher.Stop()

	if watcher == nil {
		t.Fatal("NewSessionWatcher() returned nil")
	}

	// Callback should not have been called yet
	if callbackCalled {
		t.Error("Callback should not be called on creation")
	}
}

// =============================================================================
// Directory Watching Tests
// =============================================================================

// TestSessionWatcher_WatchDirectory tests adding a directory to the watch list.
func TestSessionWatcher_WatchDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	watcher, err := NewSessionWatcher(func(path string) {})
	if err != nil {
		t.Fatalf("NewSessionWatcher() error = %v", err)
	}
	defer watcher.Stop()

	// Watch a valid directory
	if err := watcher.WatchDirectory(tmpDir); err != nil {
		t.Errorf("WatchDirectory() error = %v", err)
	}

	// Verify it's in the watched list
	dirs := watcher.WatchedDirs()
	found := false
	for _, dir := range dirs {
		if dir == tmpDir {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Directory %s not in WatchedDirs()", tmpDir)
	}
}

// TestSessionWatcher_WatchNonExistent tests watching a non-existent directory.
func TestSessionWatcher_WatchNonExistent(t *testing.T) {
	watcher, err := NewSessionWatcher(func(path string) {})
	if err != nil {
		t.Fatalf("NewSessionWatcher() error = %v", err)
	}
	defer watcher.Stop()

	// Watch non-existent directory should fail
	err = watcher.WatchDirectory("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("WatchDirectory on non-existent path should return error")
	}
}

// TestSessionWatcher_WatchDirectoryIdempotent tests that double-watching is safe.
func TestSessionWatcher_WatchDirectoryIdempotent(t *testing.T) {
	tmpDir := t.TempDir()

	watcher, err := NewSessionWatcher(func(path string) {})
	if err != nil {
		t.Fatalf("NewSessionWatcher() error = %v", err)
	}
	defer watcher.Stop()

	// Watch same directory twice
	if err := watcher.WatchDirectory(tmpDir); err != nil {
		t.Errorf("First WatchDirectory() error = %v", err)
	}
	if err := watcher.WatchDirectory(tmpDir); err != nil {
		t.Errorf("Second WatchDirectory() error = %v", err)
	}

	// Should only appear once in watched list
	dirs := watcher.WatchedDirs()
	count := 0
	for _, dir := range dirs {
		if dir == tmpDir {
			count++
		}
	}
	if count != 1 {
		t.Errorf("Directory appears %d times in WatchedDirs(), want 1", count)
	}
}

// TestSessionWatcher_UnwatchDirectory tests removing a directory from watch list.
func TestSessionWatcher_UnwatchDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	watcher, err := NewSessionWatcher(func(path string) {})
	if err != nil {
		t.Fatalf("NewSessionWatcher() error = %v", err)
	}
	defer watcher.Stop()

	// Watch then unwatch
	watcher.WatchDirectory(tmpDir)
	if err := watcher.UnwatchDirectory(tmpDir); err != nil {
		t.Errorf("UnwatchDirectory() error = %v", err)
	}

	// Should no longer be in list
	dirs := watcher.WatchedDirs()
	for _, dir := range dirs {
		if dir == tmpDir {
			t.Error("Directory still in WatchedDirs() after UnwatchDirectory()")
		}
	}
}

// =============================================================================
// Event Handling Tests
// =============================================================================

// TestSessionWatcher_OnCreateJSONL tests callback is invoked for new .jsonl files.
func TestSessionWatcher_OnCreateJSONL(t *testing.T) {
	tmpDir := t.TempDir()

	var callbackPath string
	var callbackMu sync.Mutex
	callbackCalled := make(chan struct{})

	watcher, err := NewSessionWatcher(func(path string) {
		callbackMu.Lock()
		callbackPath = path
		callbackMu.Unlock()
		close(callbackCalled)
	})
	if err != nil {
		t.Fatalf("NewSessionWatcher() error = %v", err)
	}
	defer watcher.Stop()

	// Watch the directory
	if err := watcher.WatchDirectory(tmpDir); err != nil {
		t.Fatalf("WatchDirectory() error = %v", err)
	}

	// Start the watcher
	watcher.Start()

	// Create a UUID-patterned .jsonl file (should trigger callback)
	testFile := filepath.Join(tmpDir, "cd49619d-7192-4a31-8b66-37fa4751c8be.jsonl")
	if err := os.WriteFile(testFile, []byte("{}"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Wait for callback or timeout
	select {
	case <-callbackCalled:
		callbackMu.Lock()
		defer callbackMu.Unlock()
		if callbackPath != testFile {
			t.Errorf("Callback received path %q, want %q", callbackPath, testFile)
		}
	case <-time.After(2 * time.Second):
		t.Error("Callback not invoked within timeout for .jsonl file creation")
	}
}

// TestSessionWatcher_IgnoresNonJSONL tests callback is NOT invoked for non-.jsonl files.
func TestSessionWatcher_IgnoresNonJSONL(t *testing.T) {
	tmpDir := t.TempDir()

	var callbackCount int32

	watcher, err := NewSessionWatcher(func(path string) {
		atomic.AddInt32(&callbackCount, 1)
	})
	if err != nil {
		t.Fatalf("NewSessionWatcher() error = %v", err)
	}
	defer watcher.Stop()

	// Watch and start
	watcher.WatchDirectory(tmpDir)
	watcher.Start()

	// Create non-.jsonl files
	nonJsonlFiles := []string{
		"test.txt",
		"session.json",
		"cd49619d-7192-4a31-8b66-37fa4751c8be.log",
	}

	for _, filename := range nonJsonlFiles {
		testFile := filepath.Join(tmpDir, filename)
		if err := os.WriteFile(testFile, []byte("data"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	// Wait a bit for any potential callbacks
	time.Sleep(500 * time.Millisecond)

	count := atomic.LoadInt32(&callbackCount)
	if count != 0 {
		t.Errorf("Callback invoked %d times for non-.jsonl files, want 0", count)
	}
}

// TestSessionWatcher_IgnoresNonUUID tests callback ignores non-UUID .jsonl files.
func TestSessionWatcher_IgnoresNonUUID(t *testing.T) {
	tmpDir := t.TempDir()

	var callbackCount int32

	watcher, err := NewSessionWatcher(func(path string) {
		atomic.AddInt32(&callbackCount, 1)
	})
	if err != nil {
		t.Fatalf("NewSessionWatcher() error = %v", err)
	}
	defer watcher.Stop()

	// Watch and start
	watcher.WatchDirectory(tmpDir)
	watcher.Start()

	// Create non-UUID .jsonl files (should be ignored)
	nonUUIDFiles := []string{
		"agent-abc123.jsonl",      // agent prefix
		"short.jsonl",             // not UUID format
		"test-session.jsonl",      // not UUID format
		"UPPER-CASE-UUID.jsonl",   // uppercase (not valid)
	}

	for _, filename := range nonUUIDFiles {
		testFile := filepath.Join(tmpDir, filename)
		if err := os.WriteFile(testFile, []byte("{}"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	// Wait a bit for any potential callbacks
	time.Sleep(500 * time.Millisecond)

	count := atomic.LoadInt32(&callbackCount)
	if count != 0 {
		t.Errorf("Callback invoked %d times for non-UUID .jsonl files, want 0", count)
	}
}

// TestSessionWatcher_IgnoresDelete tests callback is NOT invoked for DELETE events.
func TestSessionWatcher_IgnoresDelete(t *testing.T) {
	tmpDir := t.TempDir()

	// Pre-create a file to delete
	testFile := filepath.Join(tmpDir, "cd49619d-7192-4a31-8b66-37fa4751c8be.jsonl")
	if err := os.WriteFile(testFile, []byte("{}"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	var callbackCount int32

	watcher, err := NewSessionWatcher(func(path string) {
		atomic.AddInt32(&callbackCount, 1)
	})
	if err != nil {
		t.Fatalf("NewSessionWatcher() error = %v", err)
	}
	defer watcher.Stop()

	// Watch and start
	watcher.WatchDirectory(tmpDir)
	watcher.Start()

	// Wait a bit for watcher to stabilize
	time.Sleep(100 * time.Millisecond)

	// Delete the file
	if err := os.Remove(testFile); err != nil {
		t.Fatalf("Failed to remove test file: %v", err)
	}

	// Wait a bit for any potential callbacks
	time.Sleep(500 * time.Millisecond)

	count := atomic.LoadInt32(&callbackCount)
	if count != 0 {
		t.Errorf("Callback invoked %d times for DELETE event, want 0", count)
	}
}

// =============================================================================
// Lifecycle Tests
// =============================================================================

// TestSessionWatcher_Stop tests that Stop closes watcher properly.
func TestSessionWatcher_Stop(t *testing.T) {
	watcher, err := NewSessionWatcher(func(path string) {})
	if err != nil {
		t.Fatalf("NewSessionWatcher() error = %v", err)
	}

	watcher.Start()

	// Stop should complete without error
	err = watcher.Stop()
	if err != nil {
		t.Errorf("Stop() error = %v", err)
	}

	// IsRunning should be false
	if watcher.IsRunning() {
		t.Error("IsRunning() = true after Stop(), want false")
	}
}

// TestSessionWatcher_StopIdempotent tests multiple Stop calls are safe.
func TestSessionWatcher_StopIdempotent(t *testing.T) {
	watcher, err := NewSessionWatcher(func(path string) {})
	if err != nil {
		t.Fatalf("NewSessionWatcher() error = %v", err)
	}

	watcher.Start()

	// Multiple stops should not panic
	watcher.Stop()
	watcher.Stop()
	watcher.Stop()

	t.Log("Multiple Stop() calls completed without panic")
}

// TestSessionWatcher_StartIdempotent tests multiple Start calls are safe.
func TestSessionWatcher_StartIdempotent(t *testing.T) {
	watcher, err := NewSessionWatcher(func(path string) {})
	if err != nil {
		t.Fatalf("NewSessionWatcher() error = %v", err)
	}
	defer watcher.Stop()

	// Multiple starts should not create multiple goroutines
	watcher.Start()
	watcher.Start()
	watcher.Start()

	if !watcher.IsRunning() {
		t.Error("IsRunning() = false after Start()")
	}

	t.Log("Multiple Start() calls completed without panic")
}

// =============================================================================
// WatchDirectories Tests
// =============================================================================

// TestSessionWatcher_WatchDirectories tests bulk directory watching.
func TestSessionWatcher_WatchDirectories(t *testing.T) {
	// Create multiple temp directories
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()
	tmpDir3 := t.TempDir()

	watcher, err := NewSessionWatcher(func(path string) {})
	if err != nil {
		t.Fatalf("NewSessionWatcher() error = %v", err)
	}
	defer watcher.Stop()

	// Watch all directories at once
	watcher.WatchDirectories([]string{tmpDir1, tmpDir2, tmpDir3})

	// All should be in watched list
	dirs := watcher.WatchedDirs()
	if len(dirs) != 3 {
		t.Errorf("WatchedDirs() has %d entries, want 3", len(dirs))
	}
}

// TestSessionWatcher_SyncWatchedDirs tests directory synchronization.
func TestSessionWatcher_SyncWatchedDirs(t *testing.T) {
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()
	tmpDir3 := t.TempDir()

	watcher, err := NewSessionWatcher(func(path string) {})
	if err != nil {
		t.Fatalf("NewSessionWatcher() error = %v", err)
	}
	defer watcher.Stop()

	// Initially watch dir1 and dir2
	watcher.WatchDirectories([]string{tmpDir1, tmpDir2})

	// Sync to dir2 and dir3 (removes dir1, adds dir3)
	watcher.SyncWatchedDirs([]string{tmpDir2, tmpDir3})

	dirs := watcher.WatchedDirs()
	dirSet := make(map[string]bool)
	for _, dir := range dirs {
		dirSet[dir] = true
	}

	// dir1 should be removed
	if dirSet[tmpDir1] {
		t.Errorf("dir1 should be removed after SyncWatchedDirs")
	}

	// dir2 should remain
	if !dirSet[tmpDir2] {
		t.Errorf("dir2 should remain after SyncWatchedDirs")
	}

	// dir3 should be added
	if !dirSet[tmpDir3] {
		t.Errorf("dir3 should be added after SyncWatchedDirs")
	}
}

// =============================================================================
// Error Handler Tests
// =============================================================================

// TestSessionWatcher_SetErrorHandler tests setting an error handler.
func TestSessionWatcher_SetErrorHandler(t *testing.T) {
	watcher, err := NewSessionWatcher(func(path string) {})
	if err != nil {
		t.Fatalf("NewSessionWatcher() error = %v", err)
	}
	defer watcher.Stop()

	watcher.SetErrorHandler(func(err error) {
		// Error handler set - we can't easily trigger an error to verify it's called
		t.Logf("Error handler invoked: %v", err)
	})

	// We can't easily trigger an error, but verify the setter doesn't panic
	t.Log("SetErrorHandler completed without panic")
}

// =============================================================================
// Causality Tests - Will FAIL if SessionWatcher implementation is reverted
// =============================================================================

// TestCausality_SessionWatcherStructExists verifies the SessionWatcher struct exists.
// This test will FAIL if SessionWatcher is removed.
func TestCausality_SessionWatcherStructExists(t *testing.T) {
	// This will fail to compile if SessionWatcher doesn't exist
	watcher, err := NewSessionWatcher(func(path string) {})
	if err != nil {
		t.Fatalf("NewSessionWatcher() error = %v", err)
	}
	defer watcher.Stop()

	// Verify required methods exist
	_ = watcher.WatchDirectory("")
	_ = watcher.UnwatchDirectory("")
	_ = watcher.WatchedDirs()
	watcher.WatchDirectories(nil)
	watcher.SyncWatchedDirs(nil)
	watcher.Start()
	_ = watcher.Stop()
	_ = watcher.IsRunning()
	watcher.SetErrorHandler(nil)
}

// TestCausality_SessionWatcherUsesfsnotify verifies fsnotify is used.
// This test will FAIL if fsnotify is removed.
func TestCausality_SessionWatcherUsesfsnotify(t *testing.T) {
	sourceFile := "session_watcher.go"
	content, err := os.ReadFile(sourceFile)
	if err != nil {
		t.Fatalf("Failed to read source file %s: %v", sourceFile, err)
	}

	source := string(content)

	// Verify fsnotify import
	if !strings.Contains(source, "fsnotify") {
		t.Error("fsnotify not found in source - watcher may have been changed to polling")
	}

	// Verify CREATE event check
	if !strings.Contains(source, "fsnotify.Create") {
		t.Error("fsnotify.Create not found in source - CREATE event handling may have been removed")
	}
}

// TestCausality_SessionWatcherFiltersUUID verifies UUID filtering is implemented.
// This test will FAIL if UUID filtering is removed.
func TestCausality_SessionWatcherFiltersUUID(t *testing.T) {
	sourceFile := "session_watcher.go"
	content, err := os.ReadFile(sourceFile)
	if err != nil {
		t.Fatalf("Failed to read source file %s: %v", sourceFile, err)
	}

	source := string(content)

	// Verify UUID check is present
	if !strings.Contains(source, "IsUUIDSessionFile") {
		t.Error("IsUUIDSessionFile not found in source - UUID filtering may have been removed")
	}
}

// TestCausality_SessionWatcherFiltersJSONL verifies .jsonl filtering is implemented.
// This test will FAIL if .jsonl filtering is removed.
func TestCausality_SessionWatcherFiltersJSONL(t *testing.T) {
	sourceFile := "session_watcher.go"
	content, err := os.ReadFile(sourceFile)
	if err != nil {
		t.Fatalf("Failed to read source file %s: %v", sourceFile, err)
	}

	source := string(content)

	// Verify .jsonl suffix check
	if !strings.Contains(source, ".jsonl") {
		t.Error(".jsonl suffix check not found in source - file extension filtering may have been removed")
	}
}
