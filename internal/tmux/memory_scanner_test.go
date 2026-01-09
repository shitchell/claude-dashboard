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

// TestBuildSessionPatterns tests pattern generation from session paths.
func TestBuildSessionPatterns(t *testing.T) {
	tests := []struct {
		name           string
		sessionPaths   []string
		wantPatterns   map[string]string // sessionID -> expected pattern string
		wantEmptyIDs   []string          // session IDs we don't expect
	}{
		{
			name: "single session",
			sessionPaths: []string{
				"/home/user/.claude/projects/-home-user-myproject/abc123.jsonl",
			},
			wantPatterns: map[string]string{
				"abc123": "-home-user-myproject/abc123",
			},
		},
		{
			name: "multiple sessions same project",
			sessionPaths: []string{
				"/home/user/.claude/projects/-home-user/sess1.jsonl",
				"/home/user/.claude/projects/-home-user/sess2.jsonl",
			},
			wantPatterns: map[string]string{
				"sess1": "-home-user/sess1",
				"sess2": "-home-user/sess2",
			},
		},
		{
			name: "different projects",
			sessionPaths: []string{
				"/home/user/.claude/projects/-home-user-proj1/abc.jsonl",
				"/home/user/.claude/projects/-home-user-proj2/def.jsonl",
			},
			wantPatterns: map[string]string{
				"abc": "-home-user-proj1/abc",
				"def": "-home-user-proj2/def",
			},
		},
		{
			name: "uuid-style session IDs",
			sessionPaths: []string{
				"/home/guy/.claude/projects/-home-guy/b6ce5659-450d-4026-a3d8-94f8c45ddc37.jsonl",
			},
			wantPatterns: map[string]string{
				"b6ce5659-450d-4026-a3d8-94f8c45ddc37": "-home-guy/b6ce5659-450d-4026-a3d8-94f8c45ddc37",
			},
		},
		{
			name:         "empty input",
			sessionPaths: []string{},
			wantPatterns: map[string]string{},
		},
		{
			name: "invalid path (too short)",
			sessionPaths: []string{
				"abc.jsonl",
			},
			// Should still produce something, even if not ideal
			wantPatterns: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patterns := BuildSessionPatterns(tt.sessionPaths)

			// Check expected patterns
			for sessionID, wantPattern := range tt.wantPatterns {
				gotPattern, ok := patterns[sessionID]
				if !ok {
					t.Errorf("Missing pattern for session %q", sessionID)
					continue
				}
				if string(gotPattern) != wantPattern {
					t.Errorf("Pattern for %q = %q, want %q", sessionID, string(gotPattern), wantPattern)
				}
			}

			// Check we don't have unexpected patterns
			for sessionID := range patterns {
				if _, ok := tt.wantPatterns[sessionID]; !ok {
					// Only fail if it's in wantEmptyIDs
					for _, emptyID := range tt.wantEmptyIDs {
						if sessionID == emptyID {
							t.Errorf("Unexpected pattern for session %q", sessionID)
						}
					}
				}
			}
		})
	}
}

// TestFindBestMatch tests finding the pattern with highest count.
func TestFindBestMatch(t *testing.T) {
	tests := []struct {
		name        string
		counts      map[string]int
		wantPattern string
		wantCount   int
	}{
		{
			name:        "empty map",
			counts:      map[string]int{},
			wantPattern: "",
			wantCount:   0,
		},
		{
			name: "single entry",
			counts: map[string]int{
				"pattern1": 5,
			},
			wantPattern: "pattern1",
			wantCount:   5,
		},
		{
			name: "multiple entries - clear winner",
			counts: map[string]int{
				"pattern1": 5,
				"pattern2": 10,
				"pattern3": 3,
			},
			wantPattern: "pattern2",
			wantCount:   10,
		},
		{
			name: "all zeros",
			counts: map[string]int{
				"pattern1": 0,
				"pattern2": 0,
			},
			wantPattern: "",
			wantCount:   0,
		},
		{
			name: "some zeros",
			counts: map[string]int{
				"pattern1": 0,
				"pattern2": 7,
				"pattern3": 0,
			},
			wantPattern: "pattern2",
			wantCount:   7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPattern, gotCount := FindBestMatch(tt.counts)

			if gotPattern != tt.wantPattern {
				t.Errorf("FindBestMatch() pattern = %q, want %q", gotPattern, tt.wantPattern)
			}
			if gotCount != tt.wantCount {
				t.Errorf("FindBestMatch() count = %d, want %d", gotCount, tt.wantCount)
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

// mockMemoryScanner implements MemoryScannerInterface for testing.
type mockMemoryScanner struct {
	pidToSession map[int]string
	pidToCounts  map[int]int
}

func (m *mockMemoryScanner) MatchPIDToSession(pid int, sessionPatterns map[string][]byte) (string, int) {
	sessionID := m.pidToSession[pid]
	count := m.pidToCounts[pid]
	return sessionID, count
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

	mockScanner := &mockMemoryScanner{
		pidToSession: map[int]string{
			12346: "session-abc",
			12347: "session-def",
		},
		pidToCounts: map[int]int{
			12346: 50,
			12347: 30,
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

	mockScanner := &mockMemoryScanner{
		pidToSession: map[int]string{
			12346: "session-xyz",
		},
		pidToCounts: map[int]int{
			12346: 100,
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
