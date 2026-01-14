package tmux

import (
	"testing"

	"github.com/shitchell/claude-dashboard/internal/session"
)

// TestParseMapLine tests parsing of /proc/<pid>/maps lines.
func TestParseMapLine(t *testing.T) {
	tests := []struct {
		name        string
		line        string
		wantStart   uint64
		wantEnd     uint64
		wantPerms   string
		wantPath    string
		expectError bool
	}{
		{
			name:      "regular mapped file",
			line:      "00400000-00452000 r-xp 00000000 08:02 173521 /usr/bin/dbus-daemon",
			wantStart: 0x00400000,
			wantEnd:   0x00452000,
			wantPerms: "r-xp",
			wantPath:  "/usr/bin/dbus-daemon",
		},
		{
			name:      "heap region",
			line:      "55f9e8a00000-55f9e8b21000 rw-p 00000000 00:00 0 [heap]",
			wantStart: 0x55f9e8a00000,
			wantEnd:   0x55f9e8b21000,
			wantPerms: "rw-p",
			wantPath:  "[heap]",
		},
		{
			name:      "stack region",
			line:      "7ffce8000000-7ffce8021000 rw-p 00000000 00:00 0 [stack]",
			wantStart: 0x7ffce8000000,
			wantEnd:   0x7ffce8021000,
			wantPerms: "rw-p",
			wantPath:  "[stack]",
		},
		{
			name:      "anonymous region (no path)",
			line:      "7f1234000000-7f1234100000 rw-p 00000000 00:00 0",
			wantStart: 0x7f1234000000,
			wantEnd:   0x7f1234100000,
			wantPerms: "rw-p",
			wantPath:  "",
		},
		{
			name:      "non-readable region",
			line:      "00400000-00452000 --xp 00000000 08:02 173521 /usr/bin/prog",
			wantStart: 0x00400000,
			wantEnd:   0x00452000,
			wantPerms: "--xp",
			wantPath:  "/usr/bin/prog",
		},
		{
			name:        "invalid - missing fields",
			line:        "00400000-00452000",
			expectError: true,
		},
		{
			name:        "invalid - bad address format",
			line:        "badaddr r-xp 00000000 08:02 173521 /usr/bin/prog",
			expectError: true,
		},
		{
			name:        "empty line",
			line:        "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			region, err := parseMapLine(tt.line)

			if tt.expectError {
				if err == nil {
					t.Errorf("parseMapLine(%q) expected error, got nil", tt.line)
				}
				return
			}

			if err != nil {
				t.Errorf("parseMapLine(%q) error = %v", tt.line, err)
				return
			}

			if region.Start != tt.wantStart {
				t.Errorf("Start = %x, want %x", region.Start, tt.wantStart)
			}
			if region.End != tt.wantEnd {
				t.Errorf("End = %x, want %x", region.End, tt.wantEnd)
			}
			if region.Perms != tt.wantPerms {
				t.Errorf("Perms = %q, want %q", region.Perms, tt.wantPerms)
			}
			if region.Path != tt.wantPath {
				t.Errorf("Path = %q, want %q", region.Path, tt.wantPath)
			}
		})
	}
}

// TestMemoryScannerScanForPatternsEmpty tests scanning with no patterns.
func TestMemoryScannerScanForPatternsEmpty(t *testing.T) {
	scanner := NewMemoryScanner(1) // PID 1 (init) should be readable
	results := scanner.ScanForPatterns(nil)

	if results == nil {
		t.Error("ScanForPatterns(nil) returned nil, want empty map")
	}
	if len(results) != 0 {
		t.Errorf("ScanForPatterns(nil) returned %d entries, want 0", len(results))
	}
}

// legacyMockMemoryScanner is a test helper for the old interface.
// Note: The production code now uses the new ScanAllPIDsForSessions interface.
type legacyMockMemoryScanner struct {
	pidToSession map[int]string
}

func (m *legacyMockMemoryScanner) ScanAllPIDsForSessions(pids []int, sessionPaths []string) map[int]string {
	result := make(map[int]string)
	for _, pid := range pids {
		if sessionID, ok := m.pidToSession[pid]; ok {
			result[pid] = sessionID
		}
	}
	return result
}

// TestMatcherWithMockMemoryScanner tests the matcher integration with mock memory scanning.
func TestMatcherWithMockMemoryScanner(t *testing.T) {
	// Set TMUX env var for the test
	t.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")

	runner := &combinedMockRunner{
		tmuxOutput: []byte(`/dev/pts/42	%0	main	0	code	0	bash
/dev/pts/43	%1	main	1	claude	0	claude
/dev/pts/44	%2	main	2	other	0	claude
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

	mockScanner := &legacyMockMemoryScanner{
		pidToSession: map[int]string{
			12346: "session-abc",
			12347: "session-def",
		},
	}

	matcher := NewMatcherWithRunners(runner, runner)
	matcher.SetMemoryScanner(mockScanner)
	matcher.SetSessionFilePaths([]string{
		"/home/user/.claude/projects/-home-user/session-abc.jsonl",
		"/home/user/.claude/projects/-home-user/session-def.jsonl",
	})

	err := matcher.Refresh()
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	// Check that memory scanning was used to populate session IDs
	procs := matcher.FindClaudeProcesses()
	if len(procs) != 2 {
		t.Fatalf("FindClaudeProcesses() returned %d processes, want 2", len(procs))
	}

	// Find process 12346
	var proc12346 *ClaudeProcess
	for i := range procs {
		if procs[i].PID == 12346 {
			proc12346 = &procs[i]
			break
		}
	}
	if proc12346 == nil {
		t.Fatal("Process 12346 not found")
	}
	if proc12346.SessionID != "session-abc" {
		t.Errorf("Process 12346 SessionID = %q, want session-abc", proc12346.SessionID)
	}

	// Check PID to session mapping
	pidMapping := matcher.GetPIDToSessionID()
	if pidMapping[12346] != "session-abc" {
		t.Errorf("PIDToSessionID[12346] = %q, want session-abc", pidMapping[12346])
	}
	if pidMapping[12347] != "session-def" {
		t.Errorf("PIDToSessionID[12347] = %q, want session-def", pidMapping[12347])
	}
}

// TestMatcherMatchSessionToPaneWithMemoryScan tests that memory scan results
// take precedence in session-to-pane matching.
func TestMatcherMatchSessionToPaneWithMemoryScan(t *testing.T) {
	// Set TMUX env var for the test
	t.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")

	runner := &combinedMockRunner{
		tmuxOutput: []byte(`/dev/pts/43	%1	main	1	claude	0	claude
`),
		processOutput: []byte(`12346	pts/43	claude
`),
		cwds: map[int]string{
			12346: "/home/user/project",
		},
	}

	mockScanner := &legacyMockMemoryScanner{
		pidToSession: map[int]string{
			12346: "session-xyz",
		},
	}

	matcher := NewMatcherWithRunners(runner, runner)
	matcher.SetMemoryScanner(mockScanner)
	matcher.SetSessionFilePaths([]string{
		"/home/user/.claude/projects/-home-user/session-xyz.jsonl",
	})

	err := matcher.Refresh()
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	// Now test MatchSessionToPane with a session matching the memory scan result
	sess := &session.Session{
		ID:  "session-xyz",
		CWD: "/home/user/project",
	}

	pane := matcher.MatchSessionToPane(sess)
	if pane == nil {
		t.Fatal("MatchSessionToPane() returned nil")
	}
	if pane.ID != "%1" {
		t.Errorf("MatchSessionToPane() returned pane %q, want %%1", pane.ID)
	}
}
