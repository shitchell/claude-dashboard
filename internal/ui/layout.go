// Package ui provides the terminal user interface for claude-dashboard.
// This file defines the Layout interface for session display layouts.
package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/shitchell/claude-dashboard/internal/config"
	"github.com/shitchell/claude-dashboard/internal/session"
)

// Layout defines the interface for session display layouts.
// Layouts are responsible for rendering sessions and handling navigation.
// Different layouts (list, grid) implement this interface to provide
// different visual presentations of the same session data.
type Layout interface {
	// Render renders the sessions to a string for display.
	// Parameters:
	//   - sessions: the sessions to display
	//   - cursor: the index of the currently selected session
	//   - width: the available width in characters
	//   - height: the available height in lines
	//   - styles: the styling configuration
	//   - cfg: the application configuration
	// Returns the rendered string for display.
	Render(sessions []*session.Session, cursor int, width, height int, styles *Styles, cfg *config.Config) string

	// HandleKey processes a key message for navigation.
	// Returns the new cursor position and whether the key was handled.
	// If handled is false, the key should be passed to other handlers.
	HandleKey(msg tea.KeyMsg, sessionCount int, cursor int) (newCursor int, handled bool)

	// Name returns the human-readable name of this layout.
	// Used for display in the UI (e.g., "List", "Grid").
	Name() string
}

// LayoutType represents the type of layout.
type LayoutType int

const (
	// LayoutTypeList is the standard vertical list layout.
	LayoutTypeList LayoutType = iota

	// LayoutTypeGrid is the grid/card layout.
	LayoutTypeGrid
)

// String returns the string representation of the layout type.
func (l LayoutType) String() string {
	switch l {
	case LayoutTypeList:
		return "list"
	case LayoutTypeGrid:
		return "grid"
	default:
		return "list"
	}
}
