package tmux

import (
	"time"

	"github.com/shitchell/claude-dashboard/internal/session"
)

// StatusInactivityThreshold is the duration after which a session
// is considered idle if no recent activity is detected.
// A session waiting for user input with no recent file activity
// is considered idle.
const StatusInactivityThreshold = 30 * time.Second

// DetermineStatus determines the status of a Claude session based on
// the last entry in its JSONL file and whether a matching process exists.
//
// Status determination logic:
//
// 1. StatusExited: No Claude process is running for this session.
//    This is the default when hasProcess is false.
//
// 2. StatusActive: A Claude process is running AND:
//    - The last entry is an assistant message (Claude is responding), OR
//    - The last entry is a user tool result (tool execution in progress), OR
//    - The last entry is within the inactivity threshold
//
// 3. StatusIdle: A Claude process is running AND:
//    - The last entry is a regular user message (waiting for assistant), OR
//    - The last entry is a result message (session completed but pane still open), OR
//    - The last entry is stale (older than inactivity threshold)
//
// Parameters:
//   - lastEntry: The last entry from the session's JSONL file. Can be nil
//     if the file couldn't be parsed.
//   - hasProcess: Whether a Claude process is running for this session.
//
// Returns the determined status.
func DetermineStatus(lastEntry *session.LastEntry, hasProcess bool) session.Status {
	// No process running = exited
	if !hasProcess {
		return session.StatusExited
	}

	// If we couldn't parse the last entry, assume idle
	// (process is running but we don't know what it's doing)
	if lastEntry == nil {
		return session.StatusIdle
	}

	// Check if session has a completion result
	if lastEntry.IsComplete {
		// Session completed but pane still open = idle
		return session.StatusIdle
	}

	// Determine status based on message type
	switch lastEntry.Type {
	case session.MessageTypeAssistant:
		// Claude is responding = active
		return session.StatusActive

	case session.MessageTypeUser:
		if lastEntry.IsToolResult {
			// Tool execution in progress = active
			return session.StatusActive
		}
		// Regular user message = waiting for assistant = idle
		return session.StatusIdle

	case session.MessageTypeSystem:
		// System messages like init or api_error
		// Check if it's recent activity
		if isRecentActivity(lastEntry.Timestamp) {
			return session.StatusActive
		}
		return session.StatusIdle

	case session.MessageTypeResult:
		// Session has a result = completed = idle
		return session.StatusIdle

	case session.MessageTypeSummary:
		// Summary messages don't indicate activity state
		// Check recency as a fallback
		if isRecentActivity(lastEntry.Timestamp) {
			return session.StatusActive
		}
		return session.StatusIdle

	default:
		// Unknown message type - use recency heuristic
		if isRecentActivity(lastEntry.Timestamp) {
			return session.StatusActive
		}
		return session.StatusIdle
	}
}

// isRecentActivity returns true if the timestamp is within the
// inactivity threshold from now.
func isRecentActivity(timestamp time.Time) bool {
	if timestamp.IsZero() {
		return false
	}
	return time.Since(timestamp) < StatusInactivityThreshold
}

// DetermineStatusFromSession is a convenience function that determines
// the status of a session using the matcher and parser.
//
// It:
// 1. Checks if a Claude process is running for the session
// 2. Parses the last entry from the session's JSONL file
// 3. Determines the appropriate status
//
// This is the main entry point for status determination in the UI layer.
func DetermineStatusFromSession(
	sess *session.Session,
	matcher *Matcher,
	parser *session.Parser,
) session.Status {
	if sess == nil {
		return session.StatusExited
	}

	// Check if process is running
	hasProcess := matcher.HasRunningProcess(sess)

	// Parse last entry (ignore errors - will treat as nil)
	var lastEntry *session.LastEntry
	if parser != nil && sess.FilePath != "" {
		entry, err := parser.ParseLastEntry(sess.FilePath)
		if err == nil {
			lastEntry = entry
		}
	}

	return DetermineStatus(lastEntry, hasProcess)
}

// UpdateSessionStatus updates the Status and TmuxPane fields of a session
// based on current process state.
//
// This modifies the session in place and returns it for convenience.
func UpdateSessionStatus(
	sess *session.Session,
	matcher *Matcher,
	parser *session.Parser,
) *session.Session {
	if sess == nil {
		return nil
	}

	// Determine status
	sess.Status = DetermineStatusFromSession(sess, matcher, parser)

	// Find the pane if process is running
	if sess.Status != session.StatusExited {
		pane := matcher.MatchSessionToPane(sess)
		if pane != nil {
			sess.TmuxPane = pane.ID
		}
	} else {
		sess.TmuxPane = ""
	}

	return sess
}

// UpdateAllSessionStatuses updates the status of multiple sessions.
// This is more efficient than calling UpdateSessionStatus individually
// because the matcher's state is only refreshed once.
func UpdateAllSessionStatuses(
	sessions []*session.Session,
	matcher *Matcher,
	parser *session.Parser,
) {
	// Refresh matcher state once
	if err := matcher.Refresh(); err != nil {
		// If refresh fails, mark all sessions as exited
		// (we can't determine their actual status)
		for _, sess := range sessions {
			if sess != nil {
				sess.Status = session.StatusExited
				sess.TmuxPane = ""
			}
		}
		return
	}

	// Update each session
	for _, sess := range sessions {
		UpdateSessionStatus(sess, matcher, parser)
	}
}
