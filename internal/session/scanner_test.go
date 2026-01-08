package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewScanner(t *testing.T) {
	t.Run("with explicit path", func(t *testing.T) {
		scanner, err := NewScanner("/custom/path")
		if err != nil {
			t.Fatalf("NewScanner() error = %v", err)
		}
		if scanner.ProjectsDir != "/custom/path" {
			t.Errorf("ProjectsDir = %q, want %q", scanner.ProjectsDir, "/custom/path")
		}
	})

	t.Run("with empty path uses default", func(t *testing.T) {
		scanner, err := NewScanner("")
		if err != nil {
			t.Fatalf("NewScanner() error = %v", err)
		}
		homeDir, _ := os.UserHomeDir()
		expected := filepath.Join(homeDir, ".claude/projects")
		if scanner.ProjectsDir != expected {
			t.Errorf("ProjectsDir = %q, want %q", scanner.ProjectsDir, expected)
		}
	})
}

func TestDefaultProjectsDir(t *testing.T) {
	dir, err := DefaultProjectsDir()
	if err != nil {
		t.Fatalf("DefaultProjectsDir() error = %v", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home dir: %v", err)
	}

	expected := filepath.Join(homeDir, ".claude/projects")
	if dir != expected {
		t.Errorf("DefaultProjectsDir() = %q, want %q", dir, expected)
	}
}

func TestScanEmptyDirectory(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()

	scanner, err := NewScanner(tmpDir)
	if err != nil {
		t.Fatalf("NewScanner() error = %v", err)
	}

	results, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Scan() returned %d results, want 0", len(results))
	}
}

func TestScanNonexistentDirectory(t *testing.T) {
	scanner, err := NewScanner("/nonexistent/path/that/does/not/exist")
	if err != nil {
		t.Fatalf("NewScanner() error = %v", err)
	}

	results, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Scan() error = %v, want nil", err)
	}

	if len(results) != 0 {
		t.Errorf("Scan() returned %d results, want 0", len(results))
	}
}

func TestScanWithSessions(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()

	// Create project directories with JSONL files
	projectDir1 := filepath.Join(tmpDir, "-home-user-project1")
	projectDir2 := filepath.Join(tmpDir, "-home-user-project2")

	if err := os.MkdirAll(projectDir1, 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}
	if err := os.MkdirAll(projectDir2, 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}

	// Create session files
	session1 := filepath.Join(projectDir1, "session-uuid-1.jsonl")
	session2 := filepath.Join(projectDir1, "session-uuid-2.jsonl")
	session3 := filepath.Join(projectDir2, "session-uuid-3.jsonl")
	agent1 := filepath.Join(projectDir1, "agent-abc123.jsonl")

	for _, f := range []string{session1, session2, session3, agent1} {
		if err := os.WriteFile(f, []byte(`{"type":"test"}`), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", f, err)
		}
	}

	// Also create a non-JSONL file that should be ignored
	txtFile := filepath.Join(projectDir1, "notes.txt")
	if err := os.WriteFile(txtFile, []byte("notes"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	scanner, err := NewScanner(tmpDir)
	if err != nil {
		t.Fatalf("NewScanner() error = %v", err)
	}

	results, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if len(results) != 4 {
		t.Errorf("Scan() returned %d results, want 4", len(results))
	}

	// Check that we found the expected files
	foundSessions := make(map[string]bool)
	foundAgents := make(map[string]bool)
	for _, r := range results {
		if r.IsSubAgent {
			foundAgents[r.AgentID] = true
		} else {
			foundSessions[r.SessionID] = true
		}
	}

	expectedSessions := []string{"session-uuid-1", "session-uuid-2", "session-uuid-3"}
	for _, s := range expectedSessions {
		if !foundSessions[s] {
			t.Errorf("Did not find session %q", s)
		}
	}

	if !foundAgents["abc123"] {
		t.Errorf("Did not find agent abc123")
	}
}

func TestScanMainSessionsOnly(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "-home-user-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}

	// Create regular session and agent files
	session := filepath.Join(projectDir, "main-session.jsonl")
	agent := filepath.Join(projectDir, "agent-sub1.jsonl")

	for _, f := range []string{session, agent} {
		if err := os.WriteFile(f, []byte(`{"type":"test"}`), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	scanner, err := NewScanner(tmpDir)
	if err != nil {
		t.Fatalf("NewScanner() error = %v", err)
	}

	results, err := scanner.ScanMainSessionsOnly()
	if err != nil {
		t.Fatalf("ScanMainSessionsOnly() error = %v", err)
	}

	if len(results) != 1 {
		t.Errorf("ScanMainSessionsOnly() returned %d results, want 1", len(results))
	}

	if results[0].IsSubAgent {
		t.Error("ScanMainSessionsOnly() returned a sub-agent session")
	}
}

func TestScanResultFields(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "-home-user-myproject")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}

	sessionFile := filepath.Join(projectDir, "abc123-def456.jsonl")
	if err := os.WriteFile(sessionFile, []byte(`{"type":"test"}`), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	scanner, err := NewScanner(tmpDir)
	if err != nil {
		t.Fatalf("NewScanner() error = %v", err)
	}

	results, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Scan() returned %d results, want 1", len(results))
	}

	result := results[0]

	if result.FilePath != sessionFile {
		t.Errorf("FilePath = %q, want %q", result.FilePath, sessionFile)
	}

	if result.ProjectDir != "-home-user-myproject" {
		t.Errorf("ProjectDir = %q, want %q", result.ProjectDir, "-home-user-myproject")
	}

	if result.SessionID != "abc123-def456" {
		t.Errorf("SessionID = %q, want %q", result.SessionID, "abc123-def456")
	}

	if result.IsSubAgent {
		t.Error("IsSubAgent = true, want false")
	}

	if result.ModTime == 0 {
		t.Error("ModTime = 0, want non-zero")
	}
}

func TestScanAgentFileFields(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "-home-user-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}

	agentFile := filepath.Join(projectDir, "agent-xyz789.jsonl")
	if err := os.WriteFile(agentFile, []byte(`{"type":"test"}`), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	scanner, err := NewScanner(tmpDir)
	if err != nil {
		t.Fatalf("NewScanner() error = %v", err)
	}

	results, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Scan() returned %d results, want 1", len(results))
	}

	result := results[0]

	if !result.IsSubAgent {
		t.Error("IsSubAgent = false, want true")
	}

	if result.AgentID != "xyz789" {
		t.Errorf("AgentID = %q, want %q", result.AgentID, "xyz789")
	}

	if result.SessionID != "xyz789" {
		t.Errorf("SessionID = %q, want %q", result.SessionID, "xyz789")
	}
}

// TestScanProjectsPathIsFile verifies scanner handles path being a file.
func TestScanProjectsPathIsFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "notadir")

	// Create a file instead of a directory
	if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	scanner, err := NewScanner(filePath)
	if err != nil {
		t.Fatalf("NewScanner() error = %v", err)
	}

	results, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Scan() error = %v, want nil", err)
	}

	if len(results) != 0 {
		t.Errorf("Scan() returned %d results, want 0", len(results))
	}
}

// TestScanWithFileInProjectsDir verifies scanner skips files in projects dir.
func TestScanWithFileInProjectsDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file directly in the projects directory (should be skipped)
	filePath := filepath.Join(tmpDir, "somefile.txt")
	if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	// Also create a valid project dir with a session
	projectDir := filepath.Join(tmpDir, "-home-user-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}

	sessionFile := filepath.Join(projectDir, "session.jsonl")
	if err := os.WriteFile(sessionFile, []byte(`{"type":"test"}`), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	scanner, err := NewScanner(tmpDir)
	if err != nil {
		t.Fatalf("NewScanner() error = %v", err)
	}

	results, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	// Should only find the session file, not somefile.txt
	if len(results) != 1 {
		t.Errorf("Scan() returned %d results, want 1", len(results))
	}
}

// TestScanWithSubdirInProject verifies scanner skips subdirectories in project dirs.
func TestScanWithSubdirInProject(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "-home-user-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}

	// Create a subdirectory inside the project dir (should be skipped)
	subDir := filepath.Join(projectDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	// Create a session file in the subdir (should be ignored - scanner only goes 2 levels)
	nestedSession := filepath.Join(subDir, "nested.jsonl")
	if err := os.WriteFile(nestedSession, []byte(`{"type":"test"}`), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	// Create a session at the correct level
	sessionFile := filepath.Join(projectDir, "session.jsonl")
	if err := os.WriteFile(sessionFile, []byte(`{"type":"test"}`), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	scanner, err := NewScanner(tmpDir)
	if err != nil {
		t.Fatalf("NewScanner() error = %v", err)
	}

	results, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	// Should only find session.jsonl, not nested.jsonl
	if len(results) != 1 {
		t.Errorf("Scan() returned %d results, want 1", len(results))
	}
	if len(results) == 1 && results[0].SessionID != "session" {
		t.Errorf("SessionID = %q, want %q", results[0].SessionID, "session")
	}
}
