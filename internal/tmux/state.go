package tmux

import (
	"github.com/shitchell/claude-dashboard/internal/logging"
	"github.com/shitchell/claude-dashboard/internal/session"
)

// DetermineStatus determines the status of a Claude session based on
// socket activity and process state.
//
// Status determination logic (Linux):
//   - Uses /proc/<pid>/fd/ to count open sockets
//   - Active: 2+ sockets, OR 1 socket with recent JSONL mtime
//   - Idle: 0 sockets, OR 1 socket with stale JSONL mtime (background heartbeat)
//
// Status determination logic (non-Linux fallback):
//   - Falls back to JSONL-based heuristics
//
// Parameters:
//   - pid: The process ID of the Claude process (0 if no process)
//   - jsonlPath: Path to the session's JSONL file
//
// Returns the determined status.
func DetermineStatus(pid int, jsonlPath string) session.Status {
	// No process running = exited
	if pid == 0 {
		return session.StatusExited
	}

	// Use socket-based detection (Linux) or fallback (other platforms)
	if IsProcessActive(pid, jsonlPath) {
		return session.StatusActive
	}
	return session.StatusIdle
}

// DetermineStatusFromSession is a convenience function that determines
// the status of a session using the matcher.
//
// It:
// 1. Gets the PID of the Claude process for the session
// 2. Uses socket-based detection to determine if active or idle
//
// This is the main entry point for status determination in the UI layer.
func DetermineStatusFromSession(
	sess *session.Session,
	matcher *Matcher,
	parser *session.Parser, // kept for API compatibility, unused
) session.Status {
	if sess == nil {
		return session.StatusExited
	}

	// Get the PID for this session
	pid := matcher.GetProcessPID(sess)

	return DetermineStatus(pid, sess.FilePath)
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
	logging.Info("UpdateAllSessionStatuses: Updating status for %d sessions", len(sessions))

	// Refresh matcher state once (this does memory scanning which is slow)
	logging.Info("UpdateAllSessionStatuses: Starting matcher refresh...")
	if err := matcher.Refresh(); err != nil {
		// If refresh fails, mark all sessions as exited
		// (we can't determine their actual status)
		logging.Warn("Matcher refresh failed, marking all sessions as exited: %v", err)
		for _, sess := range sessions {
			if sess != nil {
				sess.Status = session.StatusExited
				sess.TmuxPane = ""
			}
		}
		return
	}
	logging.Info("UpdateAllSessionStatuses: Matcher refresh complete")

	// Update each session
	activeCount := 0
	idleCount := 0
	exitedCount := 0
	for _, sess := range sessions {
		UpdateSessionStatus(sess, matcher, parser)
		if sess != nil {
			switch sess.Status {
			case session.StatusActive:
				activeCount++
			case session.StatusIdle:
				idleCount++
			case session.StatusExited:
				exitedCount++
			}
		}
	}
	logging.Debug("Session status update complete: %d active, %d idle, %d exited",
		activeCount, idleCount, exitedCount)
}
