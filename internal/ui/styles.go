// Package ui provides the terminal user interface for claude-dashboard.
// This file defines all visual styles using the Lip Gloss library.
package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/shitchell/claude-dashboard/internal/session"
)

// Theme color constants define the color palette for the UI.
// Using ANSI 256-color codes for maximum terminal compatibility.
const (
	// Primary colors
	ColorPrimary   = lipgloss.Color("62")  // Purple - main accent color
	ColorSecondary = lipgloss.Color("45")  // Cyan - secondary accent
	ColorWhite     = lipgloss.Color("15")  // Bright white
	ColorBlack     = lipgloss.Color("0")   // Black
	ColorGray      = lipgloss.Color("240") // Dark gray - dimmed text

	// Status colors
	ColorActive = lipgloss.Color("46")  // Green - active/running sessions
	ColorIdle   = lipgloss.Color("226") // Yellow - idle sessions
	ColorExited = lipgloss.Color("240") // Gray - exited sessions

	// Feedback colors
	ColorError   = lipgloss.Color("196") // Red - errors
	ColorWarning = lipgloss.Color("214") // Orange - warnings
	ColorSuccess = lipgloss.Color("46")  // Green - success

	// Selection colors
	ColorSelection       = lipgloss.Color("240") // Gray background for selection
	ColorSelectionBorder = lipgloss.Color("62")  // Purple border for selected items
)

// Layout constants define spacing and sizing for UI elements.
const (
	// Padding values
	PaddingHorizontal = 1
	PaddingVertical   = 0

	// Border styles
	BorderNone   = 0
	BorderNormal = 1
	BorderThick  = 2

	// List item prefix widths
	ListItemPrefixWidth = 2 // "  " or "> "
)

// Status indicators are the characters shown for each session status.
const (
	IndicatorActive = "*"
	IndicatorIdle   = "~"
	IndicatorExited = " "
)

// Styles contains all Lip Gloss styles used in the dashboard UI.
// This struct is created once and passed through the application,
// allowing for consistent styling and potential theme switching.
type Styles struct {
	// Container styles - for major UI regions
	App    lipgloss.Style // Overall application container
	Header lipgloss.Style // Top header bar
	Footer lipgloss.Style // Bottom footer bar
	List   lipgloss.Style // Session list container

	// Item styles - for list/grid items
	ItemNormal   lipgloss.Style // Unselected item
	ItemSelected lipgloss.Style // Currently selected item

	// Status styles - for session status indicators
	StatusActive lipgloss.Style // Active (processing) session
	StatusIdle   lipgloss.Style // Idle (waiting) session
	StatusExited lipgloss.Style // Exited (not running) session

	// Content styles - for individual content elements
	Title     lipgloss.Style // Application title
	Name      lipgloss.Style // Session/project name
	Preview   lipgloss.Style // Message preview text
	Time      lipgloss.Style // Timestamp display
	Indicator lipgloss.Style // Status indicator character

	// Grid-specific styles
	Cell         lipgloss.Style // Grid cell/card
	CellSelected lipgloss.Style // Selected grid cell/card

	// Feedback styles
	Error   lipgloss.Style // Error messages
	Warning lipgloss.Style // Warning messages
	Success lipgloss.Style // Success messages

	// Input styles
	Search       lipgloss.Style // Search mode indicator
	SearchPrompt lipgloss.Style // Search prompt text
	SearchInput  lipgloss.Style // Search input field

	// Help styles
	Help        lipgloss.Style // Help text
	HelpKey     lipgloss.Style // Key binding
	HelpDesc    lipgloss.Style // Help description
	HelpSection lipgloss.Style // Help section header

	// Dim style for less important text
	Dim lipgloss.Style // Dimmed/muted text
}

// NewStyles creates a new Styles struct with the default theme.
// This is the primary way to obtain styled elements for the UI.
func NewStyles() Styles {
	return Styles{
		// Container styles
		App: lipgloss.NewStyle(),
		Header: lipgloss.NewStyle().
			Padding(PaddingVertical, PaddingHorizontal),
		Footer: lipgloss.NewStyle().
			Padding(PaddingVertical, PaddingHorizontal),
		List: lipgloss.NewStyle(),

		// Item styles
		ItemNormal: lipgloss.NewStyle().
			Padding(PaddingVertical, PaddingHorizontal),
		ItemSelected: lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorWhite).
			Background(ColorSelection).
			Padding(PaddingVertical, PaddingHorizontal),

		// Status styles
		StatusActive: lipgloss.NewStyle().
			Foreground(ColorActive).
			Bold(true),
		StatusIdle: lipgloss.NewStyle().
			Foreground(ColorIdle),
		StatusExited: lipgloss.NewStyle().
			Foreground(ColorExited),

		// Content styles
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorWhite).
			Background(ColorPrimary).
			Padding(PaddingVertical, PaddingHorizontal),
		Name: lipgloss.NewStyle().
			Bold(true),
		Preview: lipgloss.NewStyle().
			Foreground(ColorGray),
		Time: lipgloss.NewStyle().
			Foreground(ColorGray),
		Indicator: lipgloss.NewStyle(),

		// Grid-specific styles
		Cell: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorGray).
			Padding(PaddingVertical, PaddingHorizontal),
		CellSelected: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorSelectionBorder).
			Bold(true).
			Padding(PaddingVertical, PaddingHorizontal),

		// Feedback styles
		Error: lipgloss.NewStyle().
			Foreground(ColorError).
			Bold(true),
		Warning: lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true),
		Success: lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true),

		// Input styles
		Search: lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true),
		SearchPrompt: lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true),
		SearchInput: lipgloss.NewStyle().
			Foreground(ColorWhite),

		// Help styles
		Help: lipgloss.NewStyle().
			Foreground(ColorGray),
		HelpKey: lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true),
		HelpDesc: lipgloss.NewStyle().
			Foreground(ColorGray),
		HelpSection: lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorWhite),

		// Dim style
		Dim: lipgloss.NewStyle().
			Foreground(ColorGray),
	}
}

// NewSidePanelStyles creates styles optimized for side panel display.
// Side panel mode uses more compact styling to fit in narrower widths.
func NewSidePanelStyles() Styles {
	s := NewStyles()

	// Override styles for narrower display
	s.Cell = s.Cell.
		Border(lipgloss.NormalBorder()).
		Padding(0, 0)
	s.CellSelected = s.CellSelected.
		Border(lipgloss.NormalBorder()).
		Padding(0, 0)

	// More compact item styles
	s.ItemNormal = s.ItemNormal.Padding(0, 0)
	s.ItemSelected = s.ItemSelected.Padding(0, 0)

	return s
}

// StatusStyle returns the appropriate style for a given session status.
// This provides a convenient way to get status-specific styling.
func (s Styles) StatusStyle(status session.Status) lipgloss.Style {
	switch status {
	case session.StatusActive:
		return s.StatusActive
	case session.StatusIdle:
		return s.StatusIdle
	default:
		return s.StatusExited
	}
}

// StatusIndicator returns the indicator character for a given status.
func StatusIndicator(status session.Status) string {
	switch status {
	case session.StatusActive:
		return IndicatorActive
	case session.StatusIdle:
		return IndicatorIdle
	default:
		return IndicatorExited
	}
}

// RenderStatusIndicator renders a styled status indicator for a session.
func (s Styles) RenderStatusIndicator(status session.Status) string {
	indicator := StatusIndicator(status)
	style := s.StatusStyle(status)
	return style.Render(indicator)
}

// WithWidth returns a copy of the style with the specified width.
// This is useful for creating fixed-width columns.
func WithWidth(style lipgloss.Style, width int) lipgloss.Style {
	return style.Width(width)
}

// WithMaxWidth returns a copy of the style with the specified max width.
// Text will be truncated if it exceeds this width.
func WithMaxWidth(style lipgloss.Style, width int) lipgloss.Style {
	return style.MaxWidth(width)
}

// DefaultStyles is a package-level instance of the default styles.
// This can be used when dependency injection is not practical.
var DefaultStyles = NewStyles()
