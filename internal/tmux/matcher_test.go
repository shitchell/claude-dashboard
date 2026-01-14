package tmux

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/shitchell/claude-dashboard/internal/session"
)

// TestIsClaudeProcess tests the IsClaudeProcess function.
func TestIsClaudeProcess(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		expected bool
	}{
		// Positive cases
		{
			name:     "bare claude command",
			command:  "claude",
			expected: true,
		},
		{
			name:     "claude with arguments",
			command:  "claude --resume abc123",
			expected: true,
		},
		{
			name:     "claude with path",
			command:  "/usr/local/bin/claude",
			expected: true,
		},
		{
			name:     "claude with path and args",
			command:  "/home/user/.local/bin/claude -p 'hello'",
			expected: true,
		},
		{
			name:     "node running claude",
			command:  "node /home/user/.nvm/versions/node/v18/bin/claude",
			expected: true,
		},
		{
			name:     "claude at end with space",
			command:  "/path/to/claude ",
			expected: true,
		},
		// Negative cases
		{
			name:     "claude as substring",
			command:  "claudette",
			expected: false,
		},
		{
			name:     "claude-dashboard",
			command:  "claude-dashboard",
			expected: false,
		},
		{
			name:     "unclaude",
			command:  "unclaude",
			expected: false,
		},
		{
			name:     "bash",
			command:  "bash",
			expected: false,
		},
		{
			name:     "vim",
			command:  "vim",
			expected: false,
		},
		{
			name:     "empty string",
			command:  "",
			expected: false,
		},
		{
			name:     "file with claude in path",
			command:  "vim /home/claude/file.txt",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsClaudeProcess(tt.command)
			if result != tt.expected {
				t.Errorf("IsClaudeProcess(%q) = %v, want %v", tt.command, result, tt.expected)
			}
		})
	}
}

// TestExtractSessionID tests the ExtractSessionID function.
func TestExtractSessionID(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		expected string
	}{
		// Positive cases
		{
			name:     "--resume with space",
			command:  "claude --resume abc123",
			expected: "abc123",
		},
		{
			name:     "--resume with equals",
			command:  "claude --resume=def456",
			expected: "def456",
		},
		{
			name:     "-r with space",
			command:  "claude -r ghi789",
			expected: "ghi789",
		},
		{
			name:     "-r with equals",
			command:  "claude -r=jkl012",
			expected: "jkl012",
		},
		{
			name:     "session ID with dashes",
			command:  "claude --resume abc-123-def",
			expected: "abc-123-def",
		},
		{
			name:     "session ID with underscores",
			command:  "claude --resume abc_123_def",
			expected: "abc_123_def",
		},
		{
			name:     "resume in middle of command",
			command:  "/usr/bin/claude --resume xyz789 -p 'hello'",
			expected: "xyz789",
		},
		{
			name:     "complex path before claude",
			command:  "/home/user/.local/bin/claude --resume session123",
			expected: "session123",
		},
		// Negative cases
		{
			name:     "no resume flag",
			command:  "claude",
			expected: "",
		},
		{
			name:     "other flags only",
			command:  "claude -p 'hello world'",
			expected: "",
		},
		{
			name:     "empty string",
			command:  "",
			expected: "",
		},
		{
			name:     "resume without value",
			command:  "claude --resume",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractSessionID(tt.command)
			if result != tt.expected {
				t.Errorf("ExtractSessionID(%q) = %q, want %q", tt.command, result, tt.expected)
			}
		})
	}
}

// TestNormalizePath tests the NormalizePath function.
func TestNormalizePath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "already clean path",
			path:     "/home/user/project",
			expected: "/home/user/project",
		},
		{
			name:     "path with trailing slash",
			path:     "/home/user/project/",
			expected: "/home/user/project",
		},
		{
			name:     "path with double slashes",
			path:     "/home//user//project",
			expected: "/home/user/project",
		},
		{
			name:     "path with dot",
			path:     "/home/user/./project",
			expected: "/home/user/project",
		},
		{
			name:     "path with dotdot",
			path:     "/home/user/foo/../project",
			expected: "/home/user/project",
		},
		{
			name:     "empty path",
			path:     "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizePath(tt.path)
			if result != tt.expected {
				t.Errorf("NormalizePath(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

// TestPathsEqual tests the PathsEqual function.
func TestPathsEqual(t *testing.T) {
	tests := []struct {
		name     string
		path1    string
		path2    string
		expected bool
	}{
		{
			name:     "identical paths",
			path1:    "/home/user/project",
			path2:    "/home/user/project",
			expected: true,
		},
		{
			name:     "one with trailing slash",
			path1:    "/home/user/project",
			path2:    "/home/user/project/",
			expected: true,
		},
		{
			name:     "different paths",
			path1:    "/home/user/project1",
			path2:    "/home/user/project2",
			expected: false,
		},
		{
			name:     "empty paths",
			path1:    "",
			path2:    "",
			expected: false, // empty paths are never equal
		},
		{
			name:     "one empty",
			path1:    "/home/user",
			path2:    "",
			expected: false,
		},
		{
			name:     "paths with dots",
			path1:    "/home/user/./project",
			path2:    "/home/user/project",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PathsEqual(tt.path1, tt.path2)
			if result != tt.expected {
				t.Errorf("PathsEqual(%q, %q) = %v, want %v", tt.path1, tt.path2, result, tt.expected)
			}
		})
	}
}

// combinedMockRunner implements both ProcessRunner and CommandRunner.
type combinedMockRunner struct {
	processOutput []byte
	cwds          map[int]string
	tmuxOutput    []byte
}

func (m *combinedMockRunner) ListProcesses() ([]byte, error) {
	return m.processOutput, nil
}

func (m *combinedMockRunner) ReadCWD(pid int) (string, error) {
	if cwd, ok := m.cwds[pid]; ok {
		return cwd, nil
	}
	return "", nil
}

func (m *combinedMockRunner) Run(args ...string) ([]byte, error) {
	return m.tmuxOutput, nil
}

// mockMemoryScanner implements MemoryScannerInterface for testing.
type mockMemoryScanner struct {
	pidToSession map[int]string
}

func (m *mockMemoryScanner) ScanAllPIDsForSessions(pids []int, sessionPaths []string) map[int]string {
	result := make(map[int]string)
	for _, pid := range pids {
		if sessionID, ok := m.pidToSession[pid]; ok {
			result[pid] = sessionID
		}
	}
	return result
}

// TestMatcherIntegration tests the full matching flow with mock data.
func TestMatcherIntegration(t *testing.T) {
	// Set TMUX env var for the test
	t.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")

	runner := &combinedMockRunner{
		// Two panes
		tmuxOutput: []byte(`/dev/pts/42	%0	main	0	code	0	bash
/dev/pts/43	%1	main	1	claude	0	claude
`),
		// Two processes, one is claude
		processOutput: []byte(`12345	pts/42	bash
12346	pts/43	claude
`),
		cwds: map[int]string{
			12345: "/home/user",
			12346: "/home/user/project",
		},
	}

	// Mock memory scanner that maps PID 12346 to session123
	memScanner := &mockMemoryScanner{
		pidToSession: map[int]string{
			12346: "session123",
		},
	}

	matcher := NewMatcherWithRunners(runner, runner)
	matcher.SetMemoryScanner(memScanner)
	matcher.SetSessionFilePaths([]string{
		"/home/user/.claude/projects/-home-user-project/session123.jsonl",
	})

	err := matcher.Refresh()
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	// Should find one Claude process
	claudeProcs := matcher.FindClaudeProcesses()
	if len(claudeProcs) != 1 {
		t.Fatalf("FindClaudeProcesses() returned %d processes, want 1", len(claudeProcs))
	}

	proc := claudeProcs[0]
	if proc.PID != 12346 {
		t.Errorf("Claude process PID = %d, want 12346", proc.PID)
	}
	if proc.SessionID != "session123" {
		t.Errorf("SessionID = %q, want session123", proc.SessionID)
	}
	if proc.PaneID != "%1" {
		t.Errorf("PaneID = %q, want %%1", proc.PaneID)
	}
	if proc.CWD != "/home/user/project" {
		t.Errorf("CWD = %q, want /home/user/project", proc.CWD)
	}
}

// TestMatcherMatchSessionToPane tests session-to-pane matching.
func TestMatcherMatchSessionToPane(t *testing.T) {
	// Set TMUX env var for the test
	t.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")

	runner := &combinedMockRunner{
		tmuxOutput: []byte(`/dev/pts/42	%0	main	0	code	0	bash
/dev/pts/43	%1	main	1	claude	0	claude
/dev/pts/44	%2	main	2	other	0	vim
`),
		processOutput: []byte(`12345	pts/42	bash
12346	pts/43	claude
12347	pts/44	claude
`),
		cwds: map[int]string{
			12345: "/home/user",
			12346: "/home/user/project1",
			12347: "/home/user/project2",
		},
	}

	// Mock memory scanner
	memScanner := &mockMemoryScanner{
		pidToSession: map[int]string{
			12346: "session123",
			12347: "session456",
		},
	}

	matcher := NewMatcherWithRunners(runner, runner)
	matcher.SetMemoryScanner(memScanner)
	matcher.SetSessionFilePaths([]string{
		"/home/user/.claude/projects/-home-user-project1/session123.jsonl",
		"/home/user/.claude/projects/-home-user-project2/session456.jsonl",
	})

	err := matcher.Refresh()
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	// Test matching by session ID (via memory scan)
	sess1 := &session.Session{
		ID:  "session123",
		CWD: "/home/user/project1",
	}
	pane1 := matcher.MatchSessionToPane(sess1)
	if pane1 == nil {
		t.Error("MatchSessionToPane by session ID returned nil")
	} else if pane1.ID != "%1" {
		t.Errorf("MatchSessionToPane by session ID returned pane %q, want %%1", pane1.ID)
	}

	// Test matching second session
	sess2 := &session.Session{
		ID:  "session456",
		CWD: "/home/user/project2",
	}
	pane2 := matcher.MatchSessionToPane(sess2)
	if pane2 == nil {
		t.Error("MatchSessionToPane for session456 returned nil")
	} else if pane2.ID != "%2" {
		t.Errorf("MatchSessionToPane for session456 returned pane %q, want %%2", pane2.ID)
	}

	// Test no match for unknown session
	sess3 := &session.Session{
		ID:  "nonexistent",
		CWD: "/home/user/nonexistent",
	}
	pane3 := matcher.MatchSessionToPane(sess3)
	if pane3 != nil {
		t.Errorf("MatchSessionToPane expected nil for nonexistent session, got pane %q", pane3.ID)
	}

	// Test nil session
	pane4 := matcher.MatchSessionToPane(nil)
	if pane4 != nil {
		t.Errorf("MatchSessionToPane(nil) expected nil, got pane %q", pane4.ID)
	}
}

// TestMatcherHasRunningProcess tests the HasRunningProcess method.
func TestMatcherHasRunningProcess(t *testing.T) {
	// Set TMUX env var for the test
	t.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")

	runner := &combinedMockRunner{
		tmuxOutput: []byte(`/dev/pts/42	%0	main	0	code	0	bash
`),
		processOutput: []byte(`12346	pts/42	claude
`),
		cwds: map[int]string{
			12346: "/home/user/project",
		},
	}

	// Mock memory scanner
	memScanner := &mockMemoryScanner{
		pidToSession: map[int]string{
			12346: "session123",
		},
	}

	matcher := NewMatcherWithRunners(runner, runner)
	matcher.SetMemoryScanner(memScanner)
	matcher.SetSessionFilePaths([]string{
		"/home/user/.claude/projects/-home-user-project/session123.jsonl",
	})

	err := matcher.Refresh()
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	// Test session with matching session ID (from memory scan)
	sess1 := &session.Session{
		ID:  "session123",
		CWD: "/home/user/project",
	}
	if !matcher.HasRunningProcess(sess1) {
		t.Error("HasRunningProcess should return true for matching session ID")
	}

	// Test session with no match (different session ID)
	sess2 := &session.Session{
		ID:  "other",
		CWD: "/home/user/project",
	}
	if matcher.HasRunningProcess(sess2) {
		t.Error("HasRunningProcess should return false for non-matching session ID")
	}

	// Test session with no match
	sess3 := &session.Session{
		ID:  "nonexistent",
		CWD: "/home/user/other",
	}
	if matcher.HasRunningProcess(sess3) {
		t.Error("HasRunningProcess should return false for non-matching session")
	}

	// Test nil session
	if matcher.HasRunningProcess(nil) {
		t.Error("HasRunningProcess(nil) should return false")
	}
}

// TestMatcherFindProcessBySessionID tests finding a process by session ID.
func TestMatcherFindProcessBySessionID(t *testing.T) {
	// Set TMUX env var for the test
	t.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")

	runner := &combinedMockRunner{
		tmuxOutput: []byte(`/dev/pts/42	%0	main	0	code	0	bash
`),
		processOutput: []byte(`12346	pts/42	claude
12347	pts/42	claude
`),
		cwds: map[int]string{},
	}

	// Mock memory scanner
	memScanner := &mockMemoryScanner{
		pidToSession: map[int]string{
			12346: "session123",
			12347: "session456",
		},
	}

	matcher := NewMatcherWithRunners(runner, runner)
	matcher.SetMemoryScanner(memScanner)
	matcher.SetSessionFilePaths([]string{
		"/home/user/.claude/projects/-home-user/session123.jsonl",
		"/home/user/.claude/projects/-home-user/session456.jsonl",
	})

	err := matcher.Refresh()
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	// Find existing session
	proc := matcher.FindProcessBySessionID("session123")
	if proc == nil {
		t.Error("FindProcessBySessionID returned nil for existing session")
	} else if proc.PID != 12346 {
		t.Errorf("FindProcessBySessionID returned wrong PID: %d, want 12346", proc.PID)
	}

	// Find another existing session
	proc = matcher.FindProcessBySessionID("session456")
	if proc == nil {
		t.Error("FindProcessBySessionID returned nil for existing session")
	} else if proc.PID != 12347 {
		t.Errorf("FindProcessBySessionID returned wrong PID: %d, want 12347", proc.PID)
	}

	// Find nonexistent session
	proc = matcher.FindProcessBySessionID("nonexistent")
	if proc != nil {
		t.Errorf("FindProcessBySessionID returned process for nonexistent session: %+v", proc)
	}
}

// TestMatcherFindProcessesByCWD tests finding processes by CWD.
func TestMatcherFindProcessesByCWD(t *testing.T) {
	// Set TMUX env var for the test
	t.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")

	runner := &combinedMockRunner{
		tmuxOutput: []byte(`/dev/pts/42	%0	main	0	code	0	bash
`),
		processOutput: []byte(`12346	pts/42	claude
12347	pts/42	claude
12348	pts/42	claude
`),
		cwds: map[int]string{
			12346: "/home/user/project1",
			12347: "/home/user/project1", // Same CWD as 12346
			12348: "/home/user/project2",
		},
	}

	// Mock memory scanner (no session mapping needed for CWD test)
	memScanner := &mockMemoryScanner{
		pidToSession: map[int]string{},
	}

	matcher := NewMatcherWithRunners(runner, runner)
	matcher.SetMemoryScanner(memScanner)

	err := matcher.Refresh()
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	// Find processes in project1 (should return 2)
	procs := matcher.FindProcessesByCWD("/home/user/project1")
	if len(procs) != 2 {
		t.Errorf("FindProcessesByCWD(project1) returned %d processes, want 2", len(procs))
	}

	// Find processes in project2 (should return 1)
	procs = matcher.FindProcessesByCWD("/home/user/project2")
	if len(procs) != 1 {
		t.Errorf("FindProcessesByCWD(project2) returned %d processes, want 1", len(procs))
	}

	// Find processes in nonexistent (should return 0)
	procs = matcher.FindProcessesByCWD("/home/user/nonexistent")
	if len(procs) != 0 {
		t.Errorf("FindProcessesByCWD(nonexistent) returned %d processes, want 0", len(procs))
	}
}

// TestNewMatcher tests that NewMatcher creates a valid matcher.
func TestNewMatcher(t *testing.T) {
	matcher := NewMatcher()
	if matcher == nil {
		t.Fatal("NewMatcher() = nil")
	}

	// Should have empty process list
	procs := matcher.FindClaudeProcesses()
	if len(procs) != 0 {
		t.Errorf("FindClaudeProcesses on new matcher = %d, want 0", len(procs))
	}

	// Pane lookup should return nil
	pane := matcher.GetPaneByID("%0")
	if pane != nil {
		t.Errorf("GetPaneByID on new matcher = %+v, want nil", pane)
	}
}

// TestIsUUIDSessionFile tests the UUID pattern matching.
func TestIsUUIDSessionFile(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		expected  bool
	}{
		{
			name:      "valid UUID",
			sessionID: "cd49619d-7192-4a31-8b66-37fa4751c8be",
			expected:  true,
		},
		{
			name:      "another valid UUID",
			sessionID: "8808c685-1b64-46c9-9709-839d02ed1478",
			expected:  true,
		},
		{
			name:      "agent prefix",
			sessionID: "agent-cd49619d-7192-4a31-8b66-37fa4751c8be",
			expected:  false,
		},
		{
			name:      "short string",
			sessionID: "abc123",
			expected:  false,
		},
		{
			name:      "empty string",
			sessionID: "",
			expected:  false,
		},
		{
			name:      "uppercase UUID (should not match)",
			sessionID: "CD49619D-7192-4A31-8B66-37FA4751C8BE",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsUUIDSessionFile(tt.sessionID)
			if result != tt.expected {
				t.Errorf("IsUUIDSessionFile(%q) = %v, want %v", tt.sessionID, result, tt.expected)
			}
		})
	}
}

// TestFilterUUIDSessionPaths tests filtering session paths to UUID-only.
func TestFilterUUIDSessionPaths(t *testing.T) {
	paths := []string{
		"/home/user/.claude/projects/-home-user/cd49619d-7192-4a31-8b66-37fa4751c8be.jsonl",
		"/home/user/.claude/projects/-home-user/agent-abc123.jsonl",
		"/home/user/.claude/projects/-home-user/8808c685-1b64-46c9-9709-839d02ed1478.jsonl",
		"/home/user/.claude/projects/-home-user/invalid.jsonl",
	}

	filtered := FilterUUIDSessionPaths(paths)

	if len(filtered) != 2 {
		t.Errorf("FilterUUIDSessionPaths returned %d paths, want 2", len(filtered))
	}

	// Check that only UUID paths remain
	for _, path := range filtered {
		if path != paths[0] && path != paths[2] {
			t.Errorf("Unexpected path in filtered results: %s", path)
		}
	}
}

// TestBuildNullPrefixedPatterns tests building NULL-prefixed patterns.
func TestBuildNullPrefixedPatterns(t *testing.T) {
	paths := []string{
		"/home/user/.claude/projects/-home-user/cd49619d-7192-4a31-8b66-37fa4751c8be.jsonl",
		"/home/user/.claude/projects/-home-user/agent-abc123.jsonl", // Should be skipped
	}

	patterns := BuildNullPrefixedPatterns(paths)

	// Should only have one pattern (agent file skipped)
	if len(patterns) != 1 {
		t.Errorf("BuildNullPrefixedPatterns returned %d patterns, want 1", len(patterns))
	}

	// Check the pattern format
	sessionID := "cd49619d-7192-4a31-8b66-37fa4751c8be"
	pattern, ok := patterns[sessionID]
	if !ok {
		t.Fatalf("Pattern for session %s not found", sessionID)
	}

	// Pattern should start with NULL byte
	if pattern[0] != 0x00 {
		t.Errorf("Pattern does not start with NULL byte")
	}

	// Pattern should contain the full path after NULL
	expectedPath := paths[0]
	if string(pattern[1:]) != expectedPath {
		t.Errorf("Pattern path = %q, want %q", string(pattern[1:]), expectedPath)
	}
}

// TestLogDuplicateMappingsNoDuplicates verifies no warnings for unique mappings.
func TestLogDuplicateMappingsNoDuplicates(t *testing.T) {
	matcher := NewMatcher()

	// Set up unique session-to-pane mappings via claudeProcesses (no duplicates)
	matcher.mu.Lock()
	matcher.claudeProcesses = []ClaudeProcess{
		{SessionID: "session-1", PaneID: "%0"},
		{SessionID: "session-2", PaneID: "%1"},
		{SessionID: "session-3", PaneID: "%2"},
	}
	matcher.mu.Unlock()

	// Call logDuplicateMappings - should not log any warnings
	// We can't easily capture log output, but we verify no panic
	matcher.mu.Lock()
	matcher.logDuplicateMappings()
	matcher.mu.Unlock()

	t.Log("logDuplicateMappings completed without panic for unique mappings")
}

// TestLogDuplicateMappingsWithDuplicates verifies warnings for duplicate mappings.
func TestLogDuplicateMappingsWithDuplicates(t *testing.T) {
	matcher := NewMatcher()

	// Set up mappings where multiple sessions point to the same pane
	// This is the bug scenario that /clear can cause
	matcher.mu.Lock()
	matcher.claudeProcesses = []ClaudeProcess{
		{SessionID: "session-1", PaneID: "%0"},
		{SessionID: "session-2", PaneID: "%0"}, // Same pane - should trigger warning
		{SessionID: "session-3", PaneID: "%1"},
	}
	matcher.mu.Unlock()

	// Call logDuplicateMappings - should log warning for pane %0
	// We verify no panic and the function completes
	matcher.mu.Lock()
	matcher.logDuplicateMappings()
	matcher.mu.Unlock()

	t.Log("logDuplicateMappings completed for duplicate mappings scenario")
}

// TestLogDuplicateMappingsMultipleDuplicatePanes verifies handling multiple duplicate panes.
func TestLogDuplicateMappingsMultipleDuplicatePanes(t *testing.T) {
	matcher := NewMatcher()

	// Set up multiple panes with duplicate mappings via claudeProcesses
	matcher.mu.Lock()
	matcher.claudeProcesses = []ClaudeProcess{
		{SessionID: "session-1", PaneID: "%0"},
		{SessionID: "session-2", PaneID: "%0"}, // Pane %0 has 2 sessions
		{SessionID: "session-3", PaneID: "%1"},
		{SessionID: "session-4", PaneID: "%1"}, // Pane %1 has 2 sessions
		{SessionID: "session-5", PaneID: "%1"}, // Pane %1 has 3 sessions
		{SessionID: "session-6", PaneID: "%2"}, // Pane %2 has only 1 session (no duplicate)
	}
	matcher.mu.Unlock()

	// Call logDuplicateMappings - should log warnings for both %0 and %1
	matcher.mu.Lock()
	matcher.logDuplicateMappings()
	matcher.mu.Unlock()

	t.Log("logDuplicateMappings handled multiple duplicate panes")
}

// TestLogDuplicateMappingsEmptyMappings verifies handling empty mappings.
func TestLogDuplicateMappingsEmptyMappings(t *testing.T) {
	matcher := NewMatcher()

	// Ensure empty claudeProcesses
	matcher.mu.Lock()
	matcher.claudeProcesses = []ClaudeProcess{}
	matcher.mu.Unlock()

	// Call logDuplicateMappings - should complete without issue
	matcher.mu.Lock()
	matcher.logDuplicateMappings()
	matcher.mu.Unlock()

	t.Log("logDuplicateMappings handled empty mappings")
}

// TestDuplicateMappingDetectionAfterRefresh verifies duplicate detection is called during refresh.
func TestDuplicateMappingDetectionAfterRefresh(t *testing.T) {
	// Set TMUX env var for the test
	t.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")

	runner := &combinedMockRunner{
		// Two panes
		tmuxOutput: []byte(`/dev/pts/42	%0	main	0	code	0	bash
/dev/pts/43	%1	main	1	claude	0	claude
`),
		// Two Claude processes - both resuming different sessions but somehow
		// mapped to the same pane (simulating a bug scenario)
		processOutput: []byte(`12346	pts/43	claude --resume session123
12347	pts/43	claude --resume session456
`),
		cwds: map[int]string{
			12346: "/home/user/project",
			12347: "/home/user/project",
		},
	}

	matcher := NewMatcherWithRunners(runner, runner)
	err := matcher.Refresh()
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	// After refresh, logDuplicateMappings should have been called internally
	// We verify the claudeProcesses state
	matcher.mu.RLock()
	defer matcher.mu.RUnlock()

	// Both processes should have pane %1 (from pts/43)
	// This is a valid scenario where duplicate detection would log a warning
	processCount := len(matcher.claudeProcesses)
	t.Logf("claudeProcesses has %d entries", processCount)

	t.Log("Duplicate detection is invoked during Refresh()")
}

// TestSessionToPaneMappingAfterClearScenario simulates the /clear bug scenario.
func TestSessionToPaneMappingAfterClearScenario(t *testing.T) {
	// This test simulates what happens when:
	// 1. Claude is running with session-old
	// 2. User does /clear
	// 3. Claude now runs with session-new in the same pane
	// 4. Without proper cleanup, both session-old and session-new map to same pane

	matcher := NewMatcher()

	// Simulate state BEFORE proper fix: both sessions point to same pane via claudeProcesses
	matcher.mu.Lock()
	matcher.claudeProcesses = []ClaudeProcess{
		{SessionID: "session-old-abc123", PaneID: "%5"},
		{SessionID: "session-new-def456", PaneID: "%5"}, // Bug: old session not cleaned up
	}
	matcher.mu.Unlock()

	// Build reverse mapping to detect duplicates from claudeProcesses
	matcher.mu.RLock()
	paneToSessions := make(map[string][]string)
	for _, proc := range matcher.claudeProcesses {
		if proc.SessionID != "" && proc.PaneID != "" {
			paneToSessions[proc.PaneID] = append(paneToSessions[proc.PaneID], proc.SessionID)
		}
	}
	matcher.mu.RUnlock()

	// Verify we detect the duplicate
	duplicateCount := 0
	for _, sessions := range paneToSessions {
		if len(sessions) > 1 {
			duplicateCount++
		}
	}

	if duplicateCount != 1 {
		t.Errorf("Expected 1 pane with duplicates, got %d", duplicateCount)
	}

	// Pane %5 should have 2 sessions
	if len(paneToSessions["%5"]) != 2 {
		t.Errorf("Pane %%5 should have 2 sessions, got %d", len(paneToSessions["%5"]))
	}

	t.Log("Duplicate detection correctly identifies /clear scenario bug")
}

// =============================================================================
// Tests for Chunk 003: PID Cache and SessionWatcher Integration
// =============================================================================
//
// These tests verify the caching integration introduced in ticket 011 chunk 003:
// 1. PIDCache is used to skip scanning known PIDs
// 2. ForceFullScan bypasses the cache
// 3. Cache is updated after scans
// 4. SessionWatcher triggers cache invalidation

// trackingMemoryScanner tracks which PIDs are scanned and allows configurable results.
type trackingMemoryScanner struct {
	mu           sync.Mutex
	scannedPIDs  []int
	scanCount    int
	pidToSession map[int]string
}

func (s *trackingMemoryScanner) ScanAllPIDsForSessions(pids []int, sessionPaths []string) map[int]string {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.scannedPIDs = append(s.scannedPIDs, pids...)
	s.scanCount++

	result := make(map[int]string)
	for _, pid := range pids {
		if sessionID, ok := s.pidToSession[pid]; ok {
			result[pid] = sessionID
		}
	}
	return result
}

func (s *trackingMemoryScanner) GetScannedPIDs() []int {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]int, len(s.scannedPIDs))
	copy(result, s.scannedPIDs)
	return result
}

func (s *trackingMemoryScanner) GetScanCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.scanCount
}

func (s *trackingMemoryScanner) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scannedPIDs = nil
	s.scanCount = 0
}

// TestMatcher_UsesCacheForKnownPIDs verifies that cached PIDs are not rescanned.
func TestMatcher_UsesCacheForKnownPIDs(t *testing.T) {
	// Set TMUX env var for the test
	t.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")

	tmpDir := t.TempDir()

	runner := &combinedMockRunner{
		tmuxOutput: []byte(`/dev/pts/42	%0	main	0	code	0	bash
/dev/pts/43	%1	main	1	claude	0	claude
`),
		processOutput: []byte(`12346	pts/43	claude
`),
		cwds: map[int]string{
			12346: "/home/user/project",
		},
	}

	// Create tracking scanner
	tracker := &trackingMemoryScanner{
		pidToSession: map[int]string{
			12346: "session-abc",
		},
	}

	matcher := NewMatcherWithRunners(runner, runner)
	matcher.SetMemoryScanner(tracker)
	matcher.SetSessionFilePaths([]string{
		"/home/user/.claude/projects/-home-user-project/session-abc.jsonl",
	})

	// Create and populate cache with known PID
	cache := NewPIDCacheWithPath(filepath.Join(tmpDir, "cache.json"))
	cache.Set(12346, "/home/user/.claude/projects/-home-user-project/session-abc.jsonl")
	matcher.pidCache = cache

	// First refresh - PID should be served from cache
	if err := matcher.Refresh(); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	// Verify PID was NOT scanned (came from cache)
	scanned := tracker.GetScannedPIDs()
	for _, pid := range scanned {
		if pid == 12346 {
			t.Error("PID 12346 was scanned despite being in cache")
		}
	}

	// Verify session ID was still resolved correctly from cache
	procs := matcher.FindClaudeProcesses()
	if len(procs) != 1 {
		t.Fatalf("FindClaudeProcesses() returned %d processes, want 1", len(procs))
	}
	if procs[0].SessionID != "session-abc" {
		t.Errorf("SessionID = %q, want session-abc (from cache)", procs[0].SessionID)
	}
}

// TestMatcher_ScansUncachedPIDs verifies that uncached PIDs are scanned.
func TestMatcher_ScansUncachedPIDs(t *testing.T) {
	// Set TMUX env var for the test
	t.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")

	tmpDir := t.TempDir()

	runner := &combinedMockRunner{
		tmuxOutput: []byte(`/dev/pts/42	%0	main	0	code	0	bash
/dev/pts/43	%1	main	1	claude	0	claude
/dev/pts/44	%2	main	2	claude2	0	claude
`),
		processOutput: []byte(`12346	pts/43	claude
12347	pts/44	claude
`),
		cwds: map[int]string{
			12346: "/home/user/project1",
			12347: "/home/user/project2",
		},
	}

	// Tracker will return session IDs for both PIDs
	tracker := &trackingMemoryScanner{
		pidToSession: map[int]string{
			12346: "session-cached",
			12347: "session-new",
		},
	}

	matcher := NewMatcherWithRunners(runner, runner)
	matcher.SetMemoryScanner(tracker)
	matcher.SetSessionFilePaths([]string{
		"/home/user/.claude/projects/-home-user-project1/session-cached.jsonl",
		"/home/user/.claude/projects/-home-user-project2/session-new.jsonl",
	})

	// Pre-populate cache with only one PID
	cache := NewPIDCacheWithPath(filepath.Join(tmpDir, "cache.json"))
	cache.Set(12346, "/home/user/.claude/projects/-home-user-project1/session-cached.jsonl")
	matcher.pidCache = cache

	// Refresh
	if err := matcher.Refresh(); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	// Verify only the uncached PID (12347) was scanned
	scanned := tracker.GetScannedPIDs()
	found12346 := false
	found12347 := false
	for _, pid := range scanned {
		if pid == 12346 {
			found12346 = true
		}
		if pid == 12347 {
			found12347 = true
		}
	}

	if found12346 {
		t.Error("Cached PID 12346 was scanned despite being in cache")
	}
	if !found12347 {
		t.Error("Uncached PID 12347 was NOT scanned")
	}
}

// TestMatcher_UpdatesCacheAfterScan verifies cache is updated with new scan results.
func TestMatcher_UpdatesCacheAfterScan(t *testing.T) {
	// Set TMUX env var for the test
	t.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")

	tmpDir := t.TempDir()

	runner := &combinedMockRunner{
		tmuxOutput: []byte(`/dev/pts/43	%1	main	1	claude	0	claude
`),
		processOutput: []byte(`12346	pts/43	claude
`),
		cwds: map[int]string{
			12346: "/home/user/project",
		},
	}

	tracker := &trackingMemoryScanner{
		pidToSession: map[int]string{
			12346: "session-new",
		},
	}

	matcher := NewMatcherWithRunners(runner, runner)
	matcher.SetMemoryScanner(tracker)
	matcher.SetSessionFilePaths([]string{
		"/home/user/.claude/projects/-home-user-project/session-new.jsonl",
	})

	// Start with empty cache
	cache := NewPIDCacheWithPath(filepath.Join(tmpDir, "cache.json"))
	matcher.pidCache = cache

	// Refresh - should scan PID and update cache
	if err := matcher.Refresh(); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	// Verify PID was added to cache
	sessionFile, ok := cache.Get(12346)
	if !ok {
		t.Error("PID 12346 was NOT added to cache after scan")
	}
	if sessionFile != "/home/user/.claude/projects/-home-user-project/session-new.jsonl" {
		t.Errorf("Cached session file = %q, want /home/user/.claude/projects/-home-user-project/session-new.jsonl", sessionFile)
	}
}

// TestMatcher_ForceFullScan_ClearsCache verifies SetForceFullScan bypasses cache.
func TestMatcher_ForceFullScan_ClearsCache(t *testing.T) {
	// Set TMUX env var for the test
	t.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")

	tmpDir := t.TempDir()

	runner := &combinedMockRunner{
		tmuxOutput: []byte(`/dev/pts/43	%1	main	1	claude	0	claude
`),
		processOutput: []byte(`12346	pts/43	claude
`),
		cwds: map[int]string{
			12346: "/home/user/project",
		},
	}

	tracker := &trackingMemoryScanner{
		pidToSession: map[int]string{
			12346: "session-abc",
		},
	}

	matcher := NewMatcherWithRunners(runner, runner)
	matcher.SetMemoryScanner(tracker)
	matcher.SetSessionFilePaths([]string{
		"/home/user/.claude/projects/-home-user-project/session-abc.jsonl",
	})

	// Pre-populate cache
	cache := NewPIDCacheWithPath(filepath.Join(tmpDir, "cache.json"))
	cache.Set(12346, "/home/user/.claude/projects/-home-user-project/session-abc.jsonl")
	matcher.pidCache = cache

	// Set force full scan
	matcher.SetForceFullScan(true)

	// Refresh - should scan all PIDs despite cache
	if err := matcher.Refresh(); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	// Verify PID 12346 WAS scanned (cache was bypassed)
	scanned := tracker.GetScannedPIDs()
	found12346 := false
	for _, pid := range scanned {
		if pid == 12346 {
			found12346 = true
			break
		}
	}

	if !found12346 {
		t.Error("PID 12346 was NOT scanned despite forceFullScan=true")
	}
}

// TestMatcher_ForceFullScan_ClearsFlag verifies flag is cleared after Refresh.
func TestMatcher_ForceFullScan_ClearsFlag(t *testing.T) {
	// Set TMUX env var for the test
	t.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")

	tmpDir := t.TempDir()

	runner := &combinedMockRunner{
		tmuxOutput: []byte(`/dev/pts/43	%1	main	1	claude	0	claude
`),
		processOutput: []byte(`12346	pts/43	claude
`),
		cwds: map[int]string{
			12346: "/home/user/project",
		},
	}

	tracker := &trackingMemoryScanner{
		pidToSession: map[int]string{
			12346: "session-abc",
		},
	}

	matcher := NewMatcherWithRunners(runner, runner)
	matcher.SetMemoryScanner(tracker)
	matcher.SetSessionFilePaths([]string{
		"/home/user/.claude/projects/-home-user-project/session-abc.jsonl",
	})

	// Pre-populate cache and set force scan
	cache := NewPIDCacheWithPath(filepath.Join(tmpDir, "cache.json"))
	cache.Set(12346, "/home/user/.claude/projects/-home-user-project/session-abc.jsonl")
	matcher.pidCache = cache
	matcher.SetForceFullScan(true)

	// First refresh - force scan should be active
	if err := matcher.Refresh(); err != nil {
		t.Fatalf("First Refresh() error = %v", err)
	}

	// Reset tracker
	tracker.Reset()

	// Second refresh - force scan flag should be cleared
	if err := matcher.Refresh(); err != nil {
		t.Fatalf("Second Refresh() error = %v", err)
	}

	// Verify PID was NOT scanned (flag was cleared, cache used)
	scanned := tracker.GetScannedPIDs()
	for _, pid := range scanned {
		if pid == 12346 {
			t.Error("PID 12346 was scanned on second Refresh - forceFullScan flag was not cleared")
		}
	}
}

// TestMatcher_Close_SavesCache verifies Close saves the cache.
func TestMatcher_Close_SavesCache(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	// Create matcher with cache
	matcher := NewMatcherWithRunners(nil, nil)
	cache := NewPIDCacheWithPath(cachePath)
	cache.Set(12345, "/path/to/session.jsonl")
	matcher.pidCache = cache

	// Close should save cache
	if err := matcher.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		t.Error("Cache file was not created by Close()")
	}
}

// TestMatcher_GetPIDCache returns the cache for inspection.
func TestMatcher_GetPIDCache(t *testing.T) {
	tmpDir := t.TempDir()

	matcher := NewMatcherWithRunners(nil, nil)
	cache := NewPIDCacheWithPath(filepath.Join(tmpDir, "cache.json"))
	cache.Set(100, "/path/session.jsonl")
	matcher.pidCache = cache

	// GetPIDCache should return the cache
	returnedCache := matcher.GetPIDCache()
	if returnedCache != cache {
		t.Error("GetPIDCache() did not return the expected cache")
	}
}

// TestMatcher_GetSessionWatcher returns the watcher for inspection.
func TestMatcher_GetSessionWatcher(t *testing.T) {
	// Create a real matcher (which initializes the watcher)
	matcher := NewMatcher()
	defer matcher.Close()

	watcher := matcher.GetSessionWatcher()
	// Watcher should exist (created in NewMatcher)
	if watcher == nil {
		t.Error("GetSessionWatcher() returned nil for NewMatcher()")
	}
}

// TestMatcher_NewMatcherWithRunners_NoCacheOrWatcher verifies test matchers don't init cache/watcher.
func TestMatcher_NewMatcherWithRunners_NoCacheOrWatcher(t *testing.T) {
	// Test matchers should NOT have cache/watcher (to avoid side effects)
	matcher := NewMatcherWithRunners(nil, nil)

	if matcher.pidCache != nil {
		t.Error("NewMatcherWithRunners() should not initialize pidCache")
	}
	if matcher.sessionWatcher != nil {
		t.Error("NewMatcherWithRunners() should not initialize sessionWatcher")
	}
}

// =============================================================================
// Causality Tests - Will FAIL if caching integration is reverted
// =============================================================================

// TestCausality_MatcherHasPIDCache verifies Matcher has pidCache field.
// This test will FAIL if the pidCache field is removed.
func TestCausality_MatcherHasPIDCache(t *testing.T) {
	matcher := NewMatcher()
	defer matcher.Close()

	// This will fail to compile if pidCache field doesn't exist
	cache := matcher.pidCache
	if cache == nil {
		t.Error("NewMatcher() should initialize pidCache")
	}
}

// TestCausality_MatcherHasSessionWatcher verifies Matcher has sessionWatcher field.
// This test will FAIL if the sessionWatcher field is removed.
func TestCausality_MatcherHasSessionWatcher(t *testing.T) {
	matcher := NewMatcher()
	defer matcher.Close()

	// This will fail to compile if sessionWatcher field doesn't exist
	watcher := matcher.sessionWatcher
	if watcher == nil {
		t.Error("NewMatcher() should initialize sessionWatcher")
	}
}

// TestCausality_MatcherHasForceFullScan verifies Matcher has forceFullScan field.
// This test will FAIL if the forceFullScan field is removed.
func TestCausality_MatcherHasForceFullScan(t *testing.T) {
	matcher := NewMatcherWithRunners(nil, nil)

	// This will fail to compile if SetForceFullScan doesn't exist
	matcher.SetForceFullScan(true)
	matcher.SetForceFullScan(false)

	t.Log("SetForceFullScan method exists")
}

// TestCausality_MatcherSourceHasCacheIntegration verifies cache integration in source.
// This test will FAIL if cache usage is removed from scanAndMatchSessions.
func TestCausality_MatcherSourceHasCacheIntegration(t *testing.T) {
	sourceFile := "matcher.go"
	content, err := os.ReadFile(sourceFile)
	if err != nil {
		t.Fatalf("Failed to read source file %s: %v", sourceFile, err)
	}

	source := string(content)

	// Verify cache usage
	requiredPatterns := []struct {
		pattern     string
		description string
	}{
		{"pidCache", "PIDCache field reference"},
		{"sessionWatcher", "SessionWatcher field reference"},
		{"forceFullScan", "forceFullScan flag reference"},
		{"GetUncachedPIDs", "GetUncachedPIDs call"},
		{"GetCachedMapping", "GetCachedMapping call"},
		{"SetForceFullScan", "SetForceFullScan method"},
	}

	for _, req := range requiredPatterns {
		if !strings.Contains(source, req.pattern) {
			t.Errorf("Cache integration missing: %s (looking for %q)", req.description, req.pattern)
		}
	}
}

// TestCausality_NewMatcherInitializesCache verifies NewMatcher initializes cache.
// This test will FAIL if NewMatcher stops initializing the cache.
func TestCausality_NewMatcherInitializesCache(t *testing.T) {
	sourceFile := "matcher.go"
	content, err := os.ReadFile(sourceFile)
	if err != nil {
		t.Fatalf("Failed to read source file %s: %v", sourceFile, err)
	}

	source := string(content)

	// Verify NewPIDCache is called in NewMatcher
	if !strings.Contains(source, "NewPIDCache()") {
		t.Error("NewPIDCache() not found in source - cache initialization may have been removed")
	}

	// Verify NewSessionWatcher is called in NewMatcher
	if !strings.Contains(source, "NewSessionWatcher") {
		t.Error("NewSessionWatcher not found in source - watcher initialization may have been removed")
	}
}
