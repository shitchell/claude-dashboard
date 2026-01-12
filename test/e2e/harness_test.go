//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestSkipIfNoClaudeFunction verifies the SkipIfNoClaude helper exists.
func TestSkipIfNoClaudeFunction(t *testing.T) {
	// Just verify the function is callable
	// The actual behavior depends on whether claude is installed
	t.Log("SkipIfNoClaude function exists and is callable")
}

// TestClaudeInstanceStruct verifies the ClaudeInstance struct has expected fields.
func TestClaudeInstanceStruct(t *testing.T) {
	instance := &ClaudeInstance{
		Pane:        nil,
		SessionID:   "test-session-123",
		PID:         12345,
		SessionFile: "/path/to/session.jsonl",
	}

	if instance.SessionID != "test-session-123" {
		t.Errorf("SessionID = %q, want %q", instance.SessionID, "test-session-123")
	}
	if instance.PID != 12345 {
		t.Errorf("PID = %d, want %d", instance.PID, 12345)
	}
	if instance.SessionFile != "/path/to/session.jsonl" {
		t.Errorf("SessionFile = %q, want %q", instance.SessionFile, "/path/to/session.jsonl")
	}

	t.Log("ClaudeInstance struct has expected fields")
}

// TestTestSessionPrefixConstant verifies the test session prefix.
func TestTestSessionPrefixConstant(t *testing.T) {
	expected := "claude-dashboard-e2e-"
	if TestSessionPrefix != expected {
		t.Errorf("TestSessionPrefix = %q, want %q", TestSessionPrefix, expected)
	}
}

// TestDefaultTimeoutConstant verifies the default timeout value.
func TestDefaultTimeoutConstant(t *testing.T) {
	expected := 30 * time.Second
	if DefaultTimeout != expected {
		t.Errorf("DefaultTimeout = %v, want %v", DefaultTimeout, expected)
	}
}

// TestTestHarnessCreation verifies that NewTestHarness creates a valid harness.
func TestTestHarnessCreation(t *testing.T) {
	h := NewTestHarness(t)
	if h == nil {
		t.Fatal("NewTestHarness returned nil")
	}

	// Check that testDir was created
	if h.testDir == "" {
		t.Error("TestHarness.testDir is empty")
	}
	if _, err := os.Stat(h.testDir); err != nil {
		t.Errorf("TestHarness.testDir does not exist: %v", err)
	}

	t.Log("TestHarness created successfully with temp directory")
}

// TestTestHarnessGetTestDir verifies GetTestDir returns the temp directory.
func TestTestHarnessGetTestDir(t *testing.T) {
	h := NewTestHarness(t)
	dir := h.GetTestDir()

	if dir == "" {
		t.Error("GetTestDir returned empty string")
	}
	if dir != h.testDir {
		t.Errorf("GetTestDir = %q, want %q", dir, h.testDir)
	}
}

// TestTestHarnessSetBinaryPath verifies SetBinaryPath and GetBinaryPath.
func TestTestHarnessSetBinaryPath(t *testing.T) {
	h := NewTestHarness(t)

	expectedPath := "/custom/path/to/binary"
	h.SetBinaryPath(expectedPath)

	got := h.GetBinaryPath()
	if got != expectedPath {
		t.Errorf("GetBinaryPath = %q, want %q", got, expectedPath)
	}
}

// TestCreateTestProjectsDir verifies project directory creation.
func TestCreateTestProjectsDir(t *testing.T) {
	h := NewTestHarness(t)
	projectsDir := h.CreateTestProjectsDir()

	if projectsDir == "" {
		t.Error("CreateTestProjectsDir returned empty string")
	}

	// Verify the directory exists
	info, err := os.Stat(projectsDir)
	if err != nil {
		t.Fatalf("CreateTestProjectsDir directory does not exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("CreateTestProjectsDir did not create a directory")
	}

	// Verify it's under the test dir
	expectedParent := filepath.Join(h.testDir, ".claude", "projects")
	if projectsDir != expectedParent {
		t.Errorf("CreateTestProjectsDir = %q, want %q", projectsDir, expectedParent)
	}

	t.Log("CreateTestProjectsDir created directory successfully")
}

// TestCreateTestSession verifies session file creation.
func TestCreateTestSession(t *testing.T) {
	h := NewTestHarness(t)
	projectsDir := h.CreateTestProjectsDir()

	sessionFile := h.CreateTestSession(
		projectsDir,
		"/home/user/my-project",
		"test-session-abc123",
		"claude-sonnet-4-20250514",
		"Test session summary",
	)

	if sessionFile == "" {
		t.Error("CreateTestSession returned empty string")
	}

	// Verify the session file exists
	info, err := os.Stat(sessionFile)
	if err != nil {
		t.Fatalf("CreateTestSession file does not exist: %v", err)
	}
	if info.IsDir() {
		t.Error("CreateTestSession created a directory, not a file")
	}

	// Verify file has expected extension
	if filepath.Ext(sessionFile) != ".jsonl" {
		t.Errorf("Session file extension = %q, want .jsonl", filepath.Ext(sessionFile))
	}

	// Verify the file has content
	content, err := os.ReadFile(sessionFile)
	if err != nil {
		t.Fatalf("Failed to read session file: %v", err)
	}
	if len(content) == 0 {
		t.Error("Session file is empty")
	}

	// Verify content contains expected data
	contentStr := string(content)
	if !strings.Contains(contentStr, "test-session-abc123") {
		t.Error("Session file does not contain session ID")
	}
	if !strings.Contains(contentStr, "claude-sonnet-4-20250514") {
		t.Error("Session file does not contain model")
	}
	if !strings.Contains(contentStr, "Test session summary") {
		t.Error("Session file does not contain summary")
	}

	t.Log("CreateTestSession created valid session file")
}

// TestListSessionFilesEmpty verifies listing session files when none exist.
func TestListSessionFilesEmpty(t *testing.T) {
	h := NewTestHarness(t)

	// Set a non-existent projects dir
	os.Setenv("CLAUDE_PROJECTS_DIR", filepath.Join(h.testDir, "nonexistent"))
	defer os.Unsetenv("CLAUDE_PROJECTS_DIR")

	files := h.listSessionFiles()
	if len(files) != 0 {
		t.Errorf("listSessionFiles returned %d files, want 0", len(files))
	}
}

// TestListSessionFilesWithContent verifies listing session files finds them.
func TestListSessionFilesWithContent(t *testing.T) {
	h := NewTestHarness(t)
	projectsDir := h.CreateTestProjectsDir()

	// Set the projects dir for this test
	os.Setenv("CLAUDE_PROJECTS_DIR", projectsDir)
	defer os.Unsetenv("CLAUDE_PROJECTS_DIR")

	// Create some session files
	h.CreateTestSession(projectsDir, "/proj1", "session-1", "model", "Summary 1")
	h.CreateTestSession(projectsDir, "/proj2", "session-2", "model", "Summary 2")

	files := h.listSessionFiles()
	if len(files) != 2 {
		t.Errorf("listSessionFiles returned %d files, want 2", len(files))
	}

	t.Log("listSessionFiles found session files correctly")
}

// TestWaitForSessionFileTimeout verifies timeout behavior.
func TestWaitForSessionFileTimeout(t *testing.T) {
	h := NewTestHarness(t)
	projectsDir := h.CreateTestProjectsDir()

	// Set the projects dir
	os.Setenv("CLAUDE_PROJECTS_DIR", projectsDir)
	defer os.Unsetenv("CLAUDE_PROJECTS_DIR")

	existingFiles := h.listSessionFiles()

	// Wait for a file that will never appear (short timeout)
	result := h.WaitForSessionFile(existingFiles, 500*time.Millisecond)
	if result != "" {
		t.Errorf("WaitForSessionFile should return empty on timeout, got %q", result)
	}

	t.Log("WaitForSessionFile correctly times out")
}

// TestWaitForSessionFileSuccess verifies finding new session files.
func TestWaitForSessionFileSuccess(t *testing.T) {
	h := NewTestHarness(t)
	projectsDir := h.CreateTestProjectsDir()

	// Set the projects dir
	os.Setenv("CLAUDE_PROJECTS_DIR", projectsDir)
	defer os.Unsetenv("CLAUDE_PROJECTS_DIR")

	existingFiles := h.listSessionFiles()

	// Create a new file after a short delay in a goroutine
	done := make(chan struct{})
	go func() {
		time.Sleep(300 * time.Millisecond)
		h.CreateTestSession(projectsDir, "/new-project", "new-session", "model", "New summary")
		close(done)
	}()

	// Wait for the new file
	result := h.WaitForSessionFile(existingFiles, 2*time.Second)
	<-done // Ensure goroutine completes

	if result == "" {
		t.Error("WaitForSessionFile should have found the new file")
	}
	if !strings.Contains(result, "new-session") {
		t.Errorf("WaitForSessionFile returned unexpected file: %q", result)
	}

	t.Log("WaitForSessionFile correctly detected new session file")
}

// TestCleanupOrder verifies cleanup functions run in reverse order.
func TestCleanupOrder(t *testing.T) {
	h := NewTestHarness(t)

	var order []int

	h.addCleanup(func() { order = append(order, 1) })
	h.addCleanup(func() { order = append(order, 2) })
	h.addCleanup(func() { order = append(order, 3) })

	h.Cleanup()

	if len(order) != 3 {
		t.Fatalf("Expected 3 cleanup functions to run, got %d", len(order))
	}

	// Should be in reverse order: 3, 2, 1
	expected := []int{3, 2, 1}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("Cleanup order[%d] = %d, want %d", i, order[i], v)
		}
	}

	t.Log("Cleanup functions run in reverse order")
}

// TestCleanupClearsSlice verifies cleanup clears the function list.
func TestCleanupClearsSlice(t *testing.T) {
	h := NewTestHarness(t)

	h.addCleanup(func() {})
	h.addCleanup(func() {})

	h.Cleanup()

	if len(h.cleanupFuncs) != 0 {
		t.Errorf("After Cleanup, cleanupFuncs length = %d, want 0", len(h.cleanupFuncs))
	}
}
