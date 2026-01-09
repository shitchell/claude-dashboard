// Package ui implements the TUI components using bubbletea
// for browsing and managing Claude Code sessions.
package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shitchell/claude-dashboard/internal/session"
)

// sessionsLoadedMsg is sent when sessions have been loaded from disk.
// This is the result of async session loading operations.
type sessionsLoadedMsg struct {
	// Sessions is the list of loaded sessions.
	Sessions []*session.Session

	// Error is non-nil if loading failed.
	Error error
}

// sessionsRefreshedMsg is sent after a periodic refresh completes.
// It contains the updated session list.
type sessionsRefreshedMsg struct {
	// Sessions is the refreshed list of sessions.
	Sessions []*session.Session

	// Error is non-nil if refresh failed.
	Error error
}

// statusUpdatedMsg is sent when session status has been updated
// (e.g., after checking tmux panes for active sessions).
type statusUpdatedMsg struct {
	// Sessions is the list with updated status fields.
	Sessions []*session.Session

	// Error is non-nil if status update failed.
	Error error
}

// refreshTickMsg is sent periodically to trigger auto-refresh.
// The time indicates when the tick occurred.
type refreshTickMsg time.Time

// errorMsg wraps any error that occurs during async operations.
// This is used for errors that should be displayed to the user.
type errorMsg struct {
	// Error is the error that occurred.
	Error error

	// Context provides additional context about what operation failed.
	Context string
}

// windowSizeMsg is sent when the terminal window is resized.
// This wraps tea.WindowSizeMsg for type safety.
type windowSizeMsg tea.WindowSizeMsg

// sessionSelectedMsg is sent when the user selects a session.
type sessionSelectedMsg struct {
	// Session is the selected session.
	Session *session.Session
}

// navigationResultMsg is sent after attempting to navigate to a tmux pane.
type navigationResultMsg struct {
	// Session is the session we tried to navigate to.
	Session *session.Session

	// PaneID is the pane we navigated to, if successful.
	PaneID string

	// Error is non-nil if navigation failed.
	Error error
}

// sortChangedMsg is sent when the sort order changes.
type sortChangedMsg struct {
	// Field is the new sort field.
	Field session.SortField

	// Ascending is true for ascending order.
	Ascending bool
}

// filterChangedMsg is sent when the filter changes.
type filterChangedMsg struct {
	// Config is the new filter configuration.
	Config session.FilterConfig
}

// searchQueryMsg is sent when the search query changes.
type searchQueryMsg struct {
	// Query is the search text.
	Query string
}

// helpToggleMsg is sent when help is toggled.
type helpToggleMsg struct{}

// quitMsg is sent when the application should quit.
type quitMsg struct{}

// matcherRefreshedMsg is sent when the tmux matcher has been refreshed.
// This contains the mapping of sessions to panes for navigation.
type matcherRefreshedMsg struct {
	// Error is non-nil if the matcher refresh failed.
	Error error
}
