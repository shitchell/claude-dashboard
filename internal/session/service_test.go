package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// createTestSession creates a minimal valid JSONL session file.
func createTestSession(t *testing.T, dir, filename, model, cwd string, modTime time.Time) string {
	t.Helper()
	path := filepath.Join(dir, filename)
	content := `{"type":"system","subtype":"init","uuid":"init","timestamp":"2025-01-01T00:00:00Z","sessionId":"test","model":"` + model + `","cwd":"` + cwd + `"}
{"type":"user","uuid":"u1","parentUuid":null,"message":{"role":"user","content":"Hello"}}
{"type":"assistant","uuid":"a1","parentUuid":"u1","message":{"id":"msg_1","type":"message","model":"` + model + `","role":"assistant","content":[{"type":"text","text":"Hi there!"}]}}
{"type":"summary","summary":"Test session summary"}`

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Set the modification time
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("Failed to set mod time: %v", err)
	}

	return path
}

func TestNewService(t *testing.T) {
	t.Run("with nil config uses defaults", func(t *testing.T) {
		// Create a temp dir for cache
		tmpDir := t.TempDir()
		config := &ServiceConfig{
			ProjectsDir: tmpDir,
			CacheDir:    tmpDir,
		}

		svc, err := NewService(config)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		if svc.scanner == nil {
			t.Error("scanner should not be nil")
		}
		if svc.parser == nil {
			t.Error("parser should not be nil")
		}
		if svc.cache == nil {
			t.Error("cache should not be nil")
		}
	})

	t.Run("with custom config", func(t *testing.T) {
		tmpDir := t.TempDir()
		projectsDir := filepath.Join(tmpDir, "projects")
		cacheDir := filepath.Join(tmpDir, "cache")

		if err := os.MkdirAll(projectsDir, 0755); err != nil {
			t.Fatalf("Failed to create projects dir: %v", err)
		}

		config := &ServiceConfig{
			ProjectsDir: projectsDir,
			CacheDir:    cacheDir,
		}

		svc, err := NewService(config)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		if svc.projectsDir != projectsDir {
			t.Errorf("projectsDir = %q, want %q", svc.projectsDir, projectsDir)
		}
		if svc.cacheDir != cacheDir {
			t.Errorf("cacheDir = %q, want %q", svc.cacheDir, cacheDir)
		}
	})
}

// TestIntegrationScanParseCacheReturn is an integration test that verifies
// the full pipeline: scan -> parse -> cache -> return sessions.
func TestIntegrationScanParseCacheReturn(t *testing.T) {
	// Create temporary directory structure
	tmpDir := t.TempDir()
	projectsDir := filepath.Join(tmpDir, "projects")
	cacheDir := filepath.Join(tmpDir, "cache")

	// Create two project directories
	project1Dir := filepath.Join(projectsDir, "-home-user-project1")
	project2Dir := filepath.Join(projectsDir, "-home-user-project2")
	if err := os.MkdirAll(project1Dir, 0755); err != nil {
		t.Fatalf("Failed to create project1 dir: %v", err)
	}
	if err := os.MkdirAll(project2Dir, 0755); err != nil {
		t.Fatalf("Failed to create project2 dir: %v", err)
	}

	// Create session files with different timestamps
	now := time.Now().Truncate(time.Second)
	oneHourAgo := now.Add(-1 * time.Hour)
	twoDaysAgo := now.Add(-48 * time.Hour)

	createTestSession(t, project1Dir, "session1.jsonl", "claude-opus-4-5-20251101", "/home/user/project1", now)
	createTestSession(t, project1Dir, "session2.jsonl", "claude-sonnet-4-20250514", "/home/user/project1", oneHourAgo)
	createTestSession(t, project2Dir, "session3.jsonl", "claude-opus-4-5-20251101", "/home/user/project2", twoDaysAgo)

	// Also create an agent file that should be excluded
	createTestSession(t, project1Dir, "agent-sub1.jsonl", "claude-opus-4-5-20251101", "/home/user/project1", now)

	// Create the service
	config := &ServiceConfig{
		ProjectsDir: projectsDir,
		CacheDir:    cacheDir,
	}

	svc, err := NewService(config)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	// Phase 1: Initial LoadAll
	sessions, err := svc.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}

	// Should have 3 sessions (excluding agent file)
	if len(sessions) != 3 {
		t.Errorf("LoadAll() returned %d sessions, want 3", len(sessions))
	}

	// Verify sessions are sorted by mod time (newest first)
	if len(sessions) >= 3 {
		if !sessions[0].ModTime.After(sessions[1].ModTime) || !sessions[1].ModTime.After(sessions[2].ModTime) {
			t.Errorf("Sessions not sorted by mod time: %v, %v, %v",
				sessions[0].ModTime, sessions[1].ModTime, sessions[2].ModTime)
		}
	}

	// Verify cache was populated
	if svc.cache.Size() != 3 {
		t.Errorf("Cache size = %d, want 3", svc.cache.Size())
	}

	// Phase 2: Verify cache is used on second load
	sessions2, err := svc.LoadAll()
	if err != nil {
		t.Fatalf("Second LoadAll() error = %v", err)
	}

	if len(sessions2) != 3 {
		t.Errorf("Second LoadAll() returned %d sessions, want 3", len(sessions2))
	}

	// Phase 3: Test Refresh with unchanged files
	sessions3, err := svc.Refresh(sessions2)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	if len(sessions3) != 3 {
		t.Errorf("Refresh() returned %d sessions, want 3", len(sessions3))
	}

	// Phase 4: Add a new session and verify Refresh picks it up
	newSessionTime := now.Add(1 * time.Hour)
	createTestSession(t, project1Dir, "session4.jsonl", "claude-sonnet-4-20250514", "/home/user/project1", newSessionTime)

	sessions4, err := svc.Refresh(sessions3)
	if err != nil {
		t.Fatalf("Refresh() after new file error = %v", err)
	}

	if len(sessions4) != 4 {
		t.Errorf("Refresh() after new file returned %d sessions, want 4", len(sessions4))
	}

	// Phase 5: Delete a session and verify Refresh removes it
	if err := os.Remove(filepath.Join(project2Dir, "session3.jsonl")); err != nil {
		t.Fatalf("Failed to remove file: %v", err)
	}

	sessions5, err := svc.Refresh(sessions4)
	if err != nil {
		t.Fatalf("Refresh() after delete error = %v", err)
	}

	if len(sessions5) != 3 {
		t.Errorf("Refresh() after delete returned %d sessions, want 3", len(sessions5))
	}
}

func TestLoadAllEmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	projectsDir := filepath.Join(tmpDir, "projects")
	cacheDir := filepath.Join(tmpDir, "cache")

	if err := os.MkdirAll(projectsDir, 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}

	config := &ServiceConfig{
		ProjectsDir: projectsDir,
		CacheDir:    cacheDir,
	}

	svc, err := NewService(config)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	sessions, err := svc.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}

	if len(sessions) != 0 {
		t.Errorf("LoadAll() returned %d sessions, want 0", len(sessions))
	}
}

func TestRefreshWithModifiedFile(t *testing.T) {
	tmpDir := t.TempDir()
	projectsDir := filepath.Join(tmpDir, "projects")
	cacheDir := filepath.Join(tmpDir, "cache")

	projectDir := filepath.Join(projectsDir, "-home-user-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}

	now := time.Now().Truncate(time.Second)
	sessionPath := createTestSession(t, projectDir, "session.jsonl", "claude-opus-4-5-20251101", "/home/user/project", now)

	config := &ServiceConfig{
		ProjectsDir: projectsDir,
		CacheDir:    cacheDir,
	}

	svc, err := NewService(config)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	// Initial load
	sessions, err := svc.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}

	originalSummary := sessions[0].Summary

	// Modify the file (change content and mtime)
	newContent := `{"type":"system","subtype":"init","uuid":"init","timestamp":"2025-01-01T00:00:00Z","sessionId":"test","model":"claude-opus-4-5-20251101","cwd":"/home/user/project"}
{"type":"user","uuid":"u1","parentUuid":null,"message":{"role":"user","content":"Different message"}}
{"type":"assistant","uuid":"a1","parentUuid":"u1","message":{"id":"msg_1","type":"message","model":"claude-opus-4-5-20251101","role":"assistant","content":[{"type":"text","text":"Different response!"}]}}
{"type":"summary","summary":"Modified session summary"}`

	if err := os.WriteFile(sessionPath, []byte(newContent), 0644); err != nil {
		t.Fatalf("Failed to modify file: %v", err)
	}

	// Change mtime to trigger re-parse
	newTime := now.Add(1 * time.Hour)
	if err := os.Chtimes(sessionPath, newTime, newTime); err != nil {
		t.Fatalf("Failed to change mtime: %v", err)
	}

	// Refresh should detect the change
	sessions2, err := svc.Refresh(sessions)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	if len(sessions2) != 1 {
		t.Fatalf("Refresh() returned %d sessions, want 1", len(sessions2))
	}

	if sessions2[0].Summary == originalSummary {
		t.Error("Refresh() did not re-parse modified file")
	}

	if sessions2[0].Summary != "Modified session summary" {
		t.Errorf("Summary = %q, want %q", sessions2[0].Summary, "Modified session summary")
	}
}
