package tmux

import (
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
12346	pts/43	claude --resume session123
`),
		cwds: map[int]string{
			12345: "/home/user",
			12346: "/home/user/project",
		},
	}

	matcher := NewMatcherWithRunners(runner, runner)
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
12346	pts/43	claude --resume session123
12347	pts/44	claude
`),
		cwds: map[int]string{
			12345: "/home/user",
			12346: "/home/user/project1",
			12347: "/home/user/project2",
		},
	}

	matcher := NewMatcherWithRunners(runner, runner)
	err := matcher.Refresh()
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	// Test matching by session ID
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

	// Test matching by CWD when session ID doesn't match
	sess2 := &session.Session{
		ID:  "nonexistent",
		CWD: "/home/user/project2",
	}
	pane2 := matcher.MatchSessionToPane(sess2)
	if pane2 == nil {
		t.Error("MatchSessionToPane by CWD returned nil")
	} else if pane2.ID != "%2" {
		t.Errorf("MatchSessionToPane by CWD returned pane %q, want %%2", pane2.ID)
	}

	// Test no match
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
		processOutput: []byte(`12346	pts/42	claude --resume session123
`),
		cwds: map[int]string{
			12346: "/home/user/project",
		},
	}

	matcher := NewMatcherWithRunners(runner, runner)
	err := matcher.Refresh()
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	// Test session with matching session ID
	sess1 := &session.Session{
		ID:  "session123",
		CWD: "/home/user/project",
	}
	if !matcher.HasRunningProcess(sess1) {
		t.Error("HasRunningProcess should return true for matching session ID")
	}

	// Test session with matching CWD only
	sess2 := &session.Session{
		ID:  "other",
		CWD: "/home/user/project",
	}
	if !matcher.HasRunningProcess(sess2) {
		t.Error("HasRunningProcess should return true for matching CWD")
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
		processOutput: []byte(`12346	pts/42	claude --resume session123
12347	pts/42	claude --resume session456
`),
		cwds: map[int]string{},
	}

	matcher := NewMatcherWithRunners(runner, runner)
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

	matcher := NewMatcherWithRunners(runner, runner)
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

// TestMatchByMostRecent tests the matchByMostRecent function.
func TestMatchByMostRecent(t *testing.T) {
	// Test with empty slice
	result := matchByMostRecent([]ClaudeProcess{})
	if result != nil {
		t.Errorf("matchByMostRecent([]) = %+v, want nil", result)
	}

	// Test with single process
	single := []ClaudeProcess{
		{Process: Process{PID: 100}},
	}
	result = matchByMostRecent(single)
	if result == nil || result.PID != 100 {
		t.Errorf("matchByMostRecent(single) = %+v, want PID 100", result)
	}

	// Test with multiple processes
	multiple := []ClaudeProcess{
		{Process: Process{PID: 100}},
		{Process: Process{PID: 300}},
		{Process: Process{PID: 200}},
	}
	result = matchByMostRecent(multiple)
	if result == nil || result.PID != 300 {
		t.Errorf("matchByMostRecent(multiple) = %+v, want PID 300", result)
	}
}
