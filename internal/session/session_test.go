package session

import (
	"testing"
	"time"
)

func TestStatusString(t *testing.T) {
	tests := []struct {
		name     string
		status   Status
		expected string
	}{
		{
			name:     "StatusExited returns exited",
			status:   StatusExited,
			expected: "exited",
		},
		{
			name:     "StatusIdle returns idle",
			status:   StatusIdle,
			expected: "idle",
		},
		{
			name:     "StatusActive returns active",
			status:   StatusActive,
			expected: "active",
		},
		{
			name:     "Unknown status returns unknown",
			status:   Status(99),
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.status.String()
			if result != tt.expected {
				t.Errorf("Status.String() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestStatusIotaValues(t *testing.T) {
	// Verify the iota values are as expected for stable serialization
	if StatusExited != 0 {
		t.Errorf("StatusExited = %d, want 0", StatusExited)
	}
	if StatusIdle != 1 {
		t.Errorf("StatusIdle = %d, want 1", StatusIdle)
	}
	if StatusActive != 2 {
		t.Errorf("StatusActive = %d, want 2", StatusActive)
	}
}

func TestSessionToMetadata(t *testing.T) {
	modTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	session := Session{
		ID:           "abc123",
		FilePath:     "/home/user/.claude/projects/test/abc123.jsonl",
		ProjectPath:  "/home/user/code/myproject",
		ProjectName:  "myproject",
		CWD:          "/home/user/code/myproject/src",
		Summary:      "Working on feature X",
		Model:        "claude-sonnet-4-20250514",
		Preview:      "Let me help you with that...",
		MessageCount: 42,
		TurnCount:    21,
		ModTime:      modTime,
		Status:       StatusActive,
		TmuxPane:     "%5",
	}

	metadata := session.ToMetadata()

	// Verify all cacheable fields are copied
	if metadata.ID != session.ID {
		t.Errorf("ID = %q, want %q", metadata.ID, session.ID)
	}
	if metadata.FilePath != session.FilePath {
		t.Errorf("FilePath = %q, want %q", metadata.FilePath, session.FilePath)
	}
	if metadata.ProjectPath != session.ProjectPath {
		t.Errorf("ProjectPath = %q, want %q", metadata.ProjectPath, session.ProjectPath)
	}
	if metadata.ProjectName != session.ProjectName {
		t.Errorf("ProjectName = %q, want %q", metadata.ProjectName, session.ProjectName)
	}
	if metadata.CWD != session.CWD {
		t.Errorf("CWD = %q, want %q", metadata.CWD, session.CWD)
	}
	if metadata.Summary != session.Summary {
		t.Errorf("Summary = %q, want %q", metadata.Summary, session.Summary)
	}
	if metadata.Model != session.Model {
		t.Errorf("Model = %q, want %q", metadata.Model, session.Model)
	}
	if metadata.Preview != session.Preview {
		t.Errorf("Preview = %q, want %q", metadata.Preview, session.Preview)
	}
	if metadata.MessageCount != session.MessageCount {
		t.Errorf("MessageCount = %d, want %d", metadata.MessageCount, session.MessageCount)
	}
	if metadata.TurnCount != session.TurnCount {
		t.Errorf("TurnCount = %d, want %d", metadata.TurnCount, session.TurnCount)
	}
	if !metadata.ModTime.Equal(session.ModTime) {
		t.Errorf("ModTime = %v, want %v", metadata.ModTime, session.ModTime)
	}
}

func TestFromMetadata(t *testing.T) {
	modTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	metadata := SessionMetadata{
		ID:           "xyz789",
		FilePath:     "/home/user/.claude/projects/test/xyz789.jsonl",
		ProjectPath:  "/home/user/code/another",
		ProjectName:  "another",
		CWD:          "/home/user/code/another",
		Summary:      "Bug fixing session",
		Model:        "claude-opus-4-20250514",
		Preview:      "I found the issue...",
		MessageCount: 100,
		TurnCount:    50,
		ModTime:      modTime,
	}

	session := FromMetadata(metadata)

	// Verify all fields are copied from metadata
	if session.ID != metadata.ID {
		t.Errorf("ID = %q, want %q", session.ID, metadata.ID)
	}
	if session.FilePath != metadata.FilePath {
		t.Errorf("FilePath = %q, want %q", session.FilePath, metadata.FilePath)
	}
	if session.ProjectPath != metadata.ProjectPath {
		t.Errorf("ProjectPath = %q, want %q", session.ProjectPath, metadata.ProjectPath)
	}
	if session.ProjectName != metadata.ProjectName {
		t.Errorf("ProjectName = %q, want %q", session.ProjectName, metadata.ProjectName)
	}
	if session.CWD != metadata.CWD {
		t.Errorf("CWD = %q, want %q", session.CWD, metadata.CWD)
	}
	if session.Summary != metadata.Summary {
		t.Errorf("Summary = %q, want %q", session.Summary, metadata.Summary)
	}
	if session.Model != metadata.Model {
		t.Errorf("Model = %q, want %q", session.Model, metadata.Model)
	}
	if session.Preview != metadata.Preview {
		t.Errorf("Preview = %q, want %q", session.Preview, metadata.Preview)
	}
	if session.MessageCount != metadata.MessageCount {
		t.Errorf("MessageCount = %d, want %d", session.MessageCount, metadata.MessageCount)
	}
	if session.TurnCount != metadata.TurnCount {
		t.Errorf("TurnCount = %d, want %d", session.TurnCount, metadata.TurnCount)
	}
	if !session.ModTime.Equal(metadata.ModTime) {
		t.Errorf("ModTime = %v, want %v", session.ModTime, metadata.ModTime)
	}

	// Verify runtime fields are set to zero values
	if session.Status != StatusExited {
		t.Errorf("Status = %v, want StatusExited", session.Status)
	}
	if session.TmuxPane != "" {
		t.Errorf("TmuxPane = %q, want empty string", session.TmuxPane)
	}
}

func TestMetadataRoundTrip(t *testing.T) {
	modTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	original := Session{
		ID:           "roundtrip",
		FilePath:     "/home/user/.claude/projects/test/roundtrip.jsonl",
		ProjectPath:  "/home/user/code/project",
		ProjectName:  "project",
		CWD:          "/home/user/code/project",
		Summary:      "Test session",
		Model:        "claude-sonnet-4-20250514",
		Preview:      "Preview text",
		MessageCount: 10,
		TurnCount:    5,
		ModTime:      modTime,
		Status:       StatusActive,
		TmuxPane:     "%10",
	}

	// Convert to metadata and back
	metadata := original.ToMetadata()
	restored := FromMetadata(metadata)

	// All cacheable fields should match
	if restored.ID != original.ID {
		t.Errorf("Round-trip ID mismatch: got %q, want %q", restored.ID, original.ID)
	}
	if restored.ProjectPath != original.ProjectPath {
		t.Errorf("Round-trip ProjectPath mismatch: got %q, want %q", restored.ProjectPath, original.ProjectPath)
	}
	if restored.MessageCount != original.MessageCount {
		t.Errorf("Round-trip MessageCount mismatch: got %d, want %d", restored.MessageCount, original.MessageCount)
	}

	// Runtime fields should be reset
	if restored.Status != StatusExited {
		t.Errorf("Round-trip Status should be StatusExited, got %v", restored.Status)
	}
	if restored.TmuxPane != "" {
		t.Errorf("Round-trip TmuxPane should be empty, got %q", restored.TmuxPane)
	}
}
