// Package session provides types and functions for working with
// Claude Code sessions, including parsing JSONL files and caching.
package session

import (
	"time"
)

// Status represents the current state of a Claude Code session.
// Sessions can be exited (no tmux pane), idle (pane exists but waiting),
// or active (currently processing).
type Status int

const (
	// StatusExited indicates the session is not running in any tmux pane.
	StatusExited Status = iota

	// StatusIdle indicates the session is running but waiting for input.
	StatusIdle

	// StatusActive indicates the session is currently processing.
	StatusActive
)

// String returns a human-readable representation of the status.
func (s Status) String() string {
	switch s {
	case StatusExited:
		return "exited"
	case StatusIdle:
		return "idle"
	case StatusActive:
		return "active"
	default:
		return "unknown"
	}
}

// Session represents a Claude Code session with all associated metadata.
// This includes both cached data (parsed from JSONL) and runtime data
// (status from tmux).
type Session struct {
	// ID is the unique session identifier extracted from the JSONL filename.
	ID string

	// FilePath is the absolute path to the session's JSONL file.
	FilePath string

	// ProjectPath is the decoded project path from the .claude/projects
	// directory structure. This is URL-decoded from the directory name.
	ProjectPath string

	// ProjectName is the base name of the project, extracted from ProjectPath.
	// For example, "/home/user/code/myproject" would yield "myproject".
	ProjectName string

	// CWD is the current working directory from the session's init message.
	// This may differ from ProjectPath if the user changed directories.
	CWD string

	// Summary is a brief description of the session, typically the first
	// user message or a generated summary from the JSONL.
	Summary string

	// Model is the Claude model used in this session (e.g., "claude-sonnet-4-20250514").
	Model string

	// Preview is a truncated preview of the last message in the session,
	// useful for showing recent activity in the UI.
	Preview string

	// MessageCount is the total number of messages in the session.
	MessageCount int

	// TurnCount is the number of conversation turns (user-assistant pairs).
	TurnCount int

	// ModTime is the last modification time of the JSONL file.
	ModTime time.Time

	// Status indicates whether the session is exited, idle, or active.
	// This is determined at runtime by checking tmux panes.
	Status Status

	// TmuxPane is the tmux pane identifier (e.g., "%5") where this session
	// is running, or empty string if not running in any pane.
	TmuxPane string
}

// SessionMetadata contains the cacheable portion of session data.
// This excludes runtime-only fields like Status and TmuxPane which
// must be determined fresh on each scan.
type SessionMetadata struct {
	// ID is the unique session identifier.
	ID string `json:"id"`

	// FilePath is the absolute path to the session's JSONL file.
	FilePath string `json:"file_path"`

	// ProjectPath is the decoded project path.
	ProjectPath string `json:"project_path"`

	// ProjectName is the base name of the project.
	ProjectName string `json:"project_name"`

	// CWD is the current working directory from the session.
	CWD string `json:"cwd"`

	// Summary is the session summary.
	Summary string `json:"summary"`

	// Model is the Claude model used.
	Model string `json:"model"`

	// Preview is the truncated last message preview.
	Preview string `json:"preview"`

	// MessageCount is the total message count.
	MessageCount int `json:"message_count"`

	// TurnCount is the conversation turn count.
	TurnCount int `json:"turn_count"`

	// ModTime is the file modification time.
	ModTime time.Time `json:"mod_time"`
}

// ToMetadata extracts the cacheable metadata from a Session.
func (s *Session) ToMetadata() SessionMetadata {
	return SessionMetadata{
		ID:           s.ID,
		FilePath:     s.FilePath,
		ProjectPath:  s.ProjectPath,
		ProjectName:  s.ProjectName,
		CWD:          s.CWD,
		Summary:      s.Summary,
		Model:        s.Model,
		Preview:      s.Preview,
		MessageCount: s.MessageCount,
		TurnCount:    s.TurnCount,
		ModTime:      s.ModTime,
	}
}

// FromMetadata creates a Session from cached metadata.
// The Status and TmuxPane fields will be set to their zero values
// and must be populated separately.
func FromMetadata(m SessionMetadata) Session {
	return Session{
		ID:           m.ID,
		FilePath:     m.FilePath,
		ProjectPath:  m.ProjectPath,
		ProjectName:  m.ProjectName,
		CWD:          m.CWD,
		Summary:      m.Summary,
		Model:        m.Model,
		Preview:      m.Preview,
		MessageCount: m.MessageCount,
		TurnCount:    m.TurnCount,
		ModTime:      m.ModTime,
		Status:       StatusExited,
		TmuxPane:     "",
	}
}
