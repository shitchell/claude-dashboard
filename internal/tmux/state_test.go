package tmux

import (
	"testing"
	"time"

	"github.com/shitchell/claude-dashboard/internal/session"
)

// TestDetermineStatus tests the DetermineStatus function exhaustively.
func TestDetermineStatus(t *testing.T) {
	// Use a fixed "now" for consistent testing
	now := time.Now()
	recentTimestamp := now.Add(-10 * time.Second)
	staleTimestamp := now.Add(-5 * time.Minute)

	tests := []struct {
		name       string
		lastEntry  *session.LastEntry
		hasProcess bool
		expected   session.Status
	}{
		// ========== No Process Cases (StatusExited) ==========
		{
			name:       "no process - nil entry",
			lastEntry:  nil,
			hasProcess: false,
			expected:   session.StatusExited,
		},
		{
			name: "no process - assistant message",
			lastEntry: &session.LastEntry{
				Type:      session.MessageTypeAssistant,
				Timestamp: recentTimestamp,
			},
			hasProcess: false,
			expected:   session.StatusExited,
		},
		{
			name: "no process - completed result",
			lastEntry: &session.LastEntry{
				Type:       session.MessageTypeResult,
				IsComplete: true,
			},
			hasProcess: false,
			expected:   session.StatusExited,
		},

		// ========== Process Running, nil Entry (StatusIdle) ==========
		{
			name:       "process running - nil entry",
			lastEntry:  nil,
			hasProcess: true,
			expected:   session.StatusIdle,
		},

		// ========== Process Running, Completed Session (StatusIdle) ==========
		{
			name: "process running - completed session",
			lastEntry: &session.LastEntry{
				Type:       session.MessageTypeResult,
				IsComplete: true,
				Subtype:    "success",
			},
			hasProcess: true,
			expected:   session.StatusIdle,
		},
		{
			name: "process running - cancelled session",
			lastEntry: &session.LastEntry{
				Type:       session.MessageTypeResult,
				IsComplete: true,
				Subtype:    "cancelled",
			},
			hasProcess: true,
			expected:   session.StatusIdle,
		},
		{
			name: "process running - error session",
			lastEntry: &session.LastEntry{
				Type:       session.MessageTypeResult,
				IsComplete: true,
				Subtype:    "error",
			},
			hasProcess: true,
			expected:   session.StatusIdle,
		},

		// ========== Process Running, Assistant Message (StatusActive) ==========
		{
			name: "process running - assistant message recent",
			lastEntry: &session.LastEntry{
				Type:      session.MessageTypeAssistant,
				Timestamp: recentTimestamp,
				Preview:   "I'll help you with that...",
			},
			hasProcess: true,
			expected:   session.StatusActive,
		},
		{
			name: "process running - assistant message stale",
			lastEntry: &session.LastEntry{
				Type:      session.MessageTypeAssistant,
				Timestamp: staleTimestamp,
			},
			hasProcess: true,
			expected:   session.StatusActive, // Assistant message always means active if process running
		},
		{
			name: "process running - assistant message no timestamp",
			lastEntry: &session.LastEntry{
				Type: session.MessageTypeAssistant,
			},
			hasProcess: true,
			expected:   session.StatusActive,
		},

		// ========== Process Running, User Message (StatusActive or StatusIdle) ==========
		{
			name: "process running - user tool result",
			lastEntry: &session.LastEntry{
				Type:         session.MessageTypeUser,
				Timestamp:    recentTimestamp,
				IsToolResult: true,
			},
			hasProcess: true,
			expected:   session.StatusActive, // Tool result means tool execution in progress
		},
		{
			name: "process running - regular user message",
			lastEntry: &session.LastEntry{
				Type:         session.MessageTypeUser,
				Timestamp:    recentTimestamp,
				IsToolResult: false,
				Preview:      "Please help me with...",
			},
			hasProcess: true,
			expected:   session.StatusIdle, // User message = waiting for assistant
		},
		{
			name: "process running - stale user message",
			lastEntry: &session.LastEntry{
				Type:         session.MessageTypeUser,
				Timestamp:    staleTimestamp,
				IsToolResult: false,
			},
			hasProcess: true,
			expected:   session.StatusIdle,
		},

		// ========== Process Running, System Message (depends on recency) ==========
		{
			name: "process running - recent system init",
			lastEntry: &session.LastEntry{
				Type:      session.MessageTypeSystem,
				Subtype:   "init",
				Timestamp: recentTimestamp,
			},
			hasProcess: true,
			expected:   session.StatusActive,
		},
		{
			name: "process running - stale system init",
			lastEntry: &session.LastEntry{
				Type:      session.MessageTypeSystem,
				Subtype:   "init",
				Timestamp: staleTimestamp,
			},
			hasProcess: true,
			expected:   session.StatusIdle,
		},
		{
			name: "process running - recent system api_error",
			lastEntry: &session.LastEntry{
				Type:      session.MessageTypeSystem,
				Subtype:   "api_error",
				Timestamp: recentTimestamp,
			},
			hasProcess: true,
			expected:   session.StatusActive,
		},
		{
			name: "process running - stale system api_error",
			lastEntry: &session.LastEntry{
				Type:      session.MessageTypeSystem,
				Subtype:   "api_error",
				Timestamp: staleTimestamp,
			},
			hasProcess: true,
			expected:   session.StatusIdle,
		},

		// ========== Process Running, Result Message (StatusIdle) ==========
		{
			name: "process running - result message not marked complete",
			lastEntry: &session.LastEntry{
				Type:       session.MessageTypeResult,
				Subtype:    "success",
				IsComplete: false, // edge case
			},
			hasProcess: true,
			expected:   session.StatusIdle, // Result type always means idle
		},

		// ========== Process Running, Summary Message (depends on recency) ==========
		{
			name: "process running - recent summary",
			lastEntry: &session.LastEntry{
				Type:      session.MessageTypeSummary,
				Timestamp: recentTimestamp,
				Preview:   "Fixed the bug in...",
			},
			hasProcess: true,
			expected:   session.StatusActive,
		},
		{
			name: "process running - stale summary",
			lastEntry: &session.LastEntry{
				Type:      session.MessageTypeSummary,
				Timestamp: staleTimestamp,
				Preview:   "Fixed the bug in...",
			},
			hasProcess: true,
			expected:   session.StatusIdle,
		},
		{
			name: "process running - summary no timestamp",
			lastEntry: &session.LastEntry{
				Type:    session.MessageTypeSummary,
				Preview: "Fixed the bug in...",
			},
			hasProcess: true,
			expected:   session.StatusIdle, // No timestamp = can't determine recency = idle
		},

		// ========== Edge Cases ==========
		{
			name: "unknown message type - recent",
			lastEntry: &session.LastEntry{
				Type:      "unknown",
				Timestamp: recentTimestamp,
			},
			hasProcess: true,
			expected:   session.StatusActive,
		},
		{
			name: "unknown message type - stale",
			lastEntry: &session.LastEntry{
				Type:      "unknown",
				Timestamp: staleTimestamp,
			},
			hasProcess: true,
			expected:   session.StatusIdle,
		},
		{
			name: "file history snapshot - recent",
			lastEntry: &session.LastEntry{
				Type:      session.MessageTypeFileHistorySnapshot,
				Timestamp: recentTimestamp,
			},
			hasProcess: true,
			expected:   session.StatusActive,
		},
		{
			name: "queue operation - stale",
			lastEntry: &session.LastEntry{
				Type:      session.MessageTypeQueueOperation,
				Timestamp: staleTimestamp,
			},
			hasProcess: true,
			expected:   session.StatusIdle,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetermineStatus(tt.lastEntry, tt.hasProcess)
			if result != tt.expected {
				t.Errorf("DetermineStatus() = %v (%s), want %v (%s)",
					result, result.String(), tt.expected, tt.expected.String())
			}
		})
	}
}

// TestIsRecentActivity tests the isRecentActivity helper function.
func TestIsRecentActivity(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		timestamp time.Time
		expected  bool
	}{
		{
			name:      "very recent (5 seconds ago)",
			timestamp: now.Add(-5 * time.Second),
			expected:  true,
		},
		{
			name:      "recent (25 seconds ago)",
			timestamp: now.Add(-25 * time.Second),
			expected:  true,
		},
		{
			name:      "at threshold (30 seconds ago)",
			timestamp: now.Add(-30 * time.Second),
			expected:  false, // >= threshold is NOT recent
		},
		{
			name:      "stale (1 minute ago)",
			timestamp: now.Add(-1 * time.Minute),
			expected:  false,
		},
		{
			name:      "very stale (5 minutes ago)",
			timestamp: now.Add(-5 * time.Minute),
			expected:  false,
		},
		{
			name:      "zero timestamp",
			timestamp: time.Time{},
			expected:  false,
		},
		{
			name:      "future timestamp",
			timestamp: now.Add(5 * time.Second),
			expected:  true, // Future timestamps are considered "recent"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRecentActivity(tt.timestamp)
			if result != tt.expected {
				t.Errorf("isRecentActivity(%v) = %v, want %v", tt.timestamp, result, tt.expected)
			}
		})
	}
}

// TestStatusInactivityThreshold verifies the threshold constant.
func TestStatusInactivityThreshold(t *testing.T) {
	// The threshold should be 30 seconds
	expectedThreshold := 30 * time.Second
	if StatusInactivityThreshold != expectedThreshold {
		t.Errorf("StatusInactivityThreshold = %v, want %v", StatusInactivityThreshold, expectedThreshold)
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
			ID:  "session123", // Has matching process via memory scan
			CWD: "/home/user/project1",
		},
		{
			ID:  "session456", // No matching process
			CWD: "/home/user/project2",
		},
		nil, // Should be handled gracefully
	}

	UpdateAllSessionStatuses(sessions, matcher, nil)

	// First session should have process (matched via memory scanning)
	if sessions[0].Status == session.StatusExited {
		t.Error("Session 1 should not be Exited (has matching process)")
	}
	if sessions[0].TmuxPane != "%0" {
		t.Errorf("Session 1 TmuxPane = %q, want %%0", sessions[0].TmuxPane)
	}

	// Second session should be exited
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
