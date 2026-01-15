package tmux

import (
	"testing"

	"github.com/shitchell/claude-dashboard/internal/session"
)

// TestDetermineStatus tests the DetermineStatus function.
// Note: Full socket-based detection requires real processes, so we primarily
// test the no-process case here. Integration tests cover the full path.
func TestDetermineStatus(t *testing.T) {
	tests := []struct {
		name      string
		pid       int
		jsonlPath string
		expected  session.Status
	}{
		{
			name:      "no process (pid=0) returns Exited",
			pid:       0,
			jsonlPath: "",
			expected:  session.StatusExited,
		},
		{
			name:      "no process with path returns Exited",
			pid:       0,
			jsonlPath: "/home/user/.claude/projects/test/session.jsonl",
			expected:  session.StatusExited,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetermineStatus(tt.pid, tt.jsonlPath)
			if result != tt.expected {
				t.Errorf("DetermineStatus(%d, %q) = %v (%s), want %v (%s)",
					tt.pid, tt.jsonlPath, result, result.String(), tt.expected, tt.expected.String())
			}
		})
	}
}

// TestDetermineStatusFromSession tests the full integration path.
func TestDetermineStatusFromSession(t *testing.T) {
	// Test with nil session
	status := DetermineStatusFromSession(nil, nil, nil)
	if status != session.StatusExited {
		t.Errorf("DetermineStatusFromSession(nil, nil, nil) = %v, want StatusExited", status)
	}
}

// TestUpdateSessionStatus tests the UpdateSessionStatus function.
func TestUpdateSessionStatus(t *testing.T) {
	// Test with nil session
	result := UpdateSessionStatus(nil, nil, nil)
	if result != nil {
		t.Errorf("UpdateSessionStatus(nil, nil, nil) = %+v, want nil", result)
	}

	// Set TMUX env var for the test
	t.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")

	// Test with a session but no matching process
	runner := &combinedMockRunner{
		tmuxOutput:    []byte{},
		processOutput: []byte{},
		cwds:          map[int]string{},
	}

	matcher := NewMatcherWithRunners(runner, runner)
	err := matcher.Refresh()
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	sess := &session.Session{
		ID:       "test123",
		CWD:      "/home/user/project",
		Status:   session.StatusActive, // Should be changed to Exited
		TmuxPane: "%0",                 // Should be cleared
	}

	result = UpdateSessionStatus(sess, matcher, nil)
	if result == nil {
		t.Fatal("UpdateSessionStatus returned nil")
	}
	if result.Status != session.StatusExited {
		t.Errorf("Status = %v, want StatusExited", result.Status)
	}
	if result.TmuxPane != "" {
		t.Errorf("TmuxPane = %q, want empty string", result.TmuxPane)
	}
}

// stateTestMockScanner implements MemoryScannerInterface for state tests.
type stateTestMockScanner struct {
	pidToSession map[int]string
}

func (m *stateTestMockScanner) ScanAllPIDsForSessions(pids []int, sessionPaths []string) map[int]string {
	result := make(map[int]string)
	for _, pid := range pids {
		if sessionID, ok := m.pidToSession[pid]; ok {
			result[pid] = sessionID
		}
	}
	return result
}

// TestUpdateAllSessionStatuses tests batch status updates.
func TestUpdateAllSessionStatuses(t *testing.T) {
	// Set TMUX env var for the test
	t.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")

	runner := &combinedMockRunner{
		tmuxOutput: []byte(`/dev/pts/42	%0	main	0	code	0	bash
`),
		processOutput: []byte(`12346	pts/42	claude
`),
		cwds: map[int]string{
			12346: "/home/user/project1",
		},
	}

	// Use mock memory scanner to map PID 12346 to session123
	mockScanner := &stateTestMockScanner{
		pidToSession: map[int]string{
			12346: "session123",
		},
	}

	matcher := NewMatcherWithRunners(runner, runner)
	matcher.SetMemoryScanner(mockScanner)
	matcher.SetSessionFilePaths([]string{
		"/home/user/.claude/projects/-home-user/session123.jsonl",
		"/home/user/.claude/projects/-home-user/session456.jsonl",
	})

	sessions := []*session.Session{
		{
			ID:       "session123", // Has matching process via memory scan
			FilePath: "/home/user/.claude/projects/-home-user/session123.jsonl",
			CWD:      "/home/user/project1",
		},
		{
			ID:       "session456", // No matching process
			FilePath: "/home/user/.claude/projects/-home-user/session456.jsonl",
			CWD:      "/home/user/project2",
		},
		nil, // Should be handled gracefully
	}

	UpdateAllSessionStatuses(sessions, matcher, nil)

	// First session has a matching process (via memory scanning)
	// Since we can't mock socket counts, the status will be Idle or Active
	// depending on IsProcessActive(). With a fake PID (12346), /proc won't exist,
	// so CountProcessSockets returns 0, making it Idle.
	if sessions[0].Status == session.StatusExited {
		t.Error("Session 1 should not be Exited (has matching process)")
	}
	if sessions[0].TmuxPane != "%0" {
		t.Errorf("Session 1 TmuxPane = %q, want %%0", sessions[0].TmuxPane)
	}

	// Second session should be exited (no matching process)
	if sessions[1].Status != session.StatusExited {
		t.Errorf("Session 2 Status = %v, want StatusExited", sessions[1].Status)
	}
	if sessions[1].TmuxPane != "" {
		t.Errorf("Session 2 TmuxPane = %q, want empty", sessions[1].TmuxPane)
	}
}

// TestStatusString verifies the String() method works correctly.
func TestStatusString(t *testing.T) {
	tests := []struct {
		status   session.Status
		expected string
	}{
		{session.StatusExited, "exited"},
		{session.StatusIdle, "idle"},
		{session.StatusActive, "active"},
		{session.Status(99), "unknown"}, // Edge case
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.status.String()
			if result != tt.expected {
				t.Errorf("Status(%d).String() = %q, want %q", tt.status, result, tt.expected)
			}
		})
	}
}

// TestSocketDetectionConstants verifies the socket detection constants are sensible.
func TestSocketDetectionConstants(t *testing.T) {
	// SocketHeartbeatMax should be small (heartbeat uses 1 socket)
	if SocketHeartbeatMax < 1 || SocketHeartbeatMax > 2 {
		t.Errorf("SocketHeartbeatMax = %d, expected 1-2", SocketHeartbeatMax)
	}

	// MtimeStaleThreshold should be a few seconds
	if MtimeStaleThreshold.Seconds() < 1 || MtimeStaleThreshold.Seconds() > 10 {
		t.Errorf("MtimeStaleThreshold = %v, expected 1-10 seconds", MtimeStaleThreshold)
	}
}
