package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/shitchell/claude-dashboard/internal/session"
)

// Style constants for the UI.
// These define colors, borders, and other visual properties.
var (
	// titleStyle is used for the application title.
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")). // White
			Background(lipgloss.Color("62")). // Purple
			Padding(0, 1)

	// selectedStyle highlights the currently selected item.
	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).  // White
			Background(lipgloss.Color("240")). // Gray
			Padding(0, 1)

	// normalStyle is for unselected items.
	normalStyle = lipgloss.NewStyle().
			Padding(0, 1)

	// statusActiveStyle is for active sessions.
	statusActiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("46")). // Green
				Bold(true)

	// statusIdleStyle is for idle sessions.
	statusIdleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("226")) // Yellow

	// statusExitedStyle is for exited sessions.
	statusExitedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240")) // Dark gray

	// errorStyle is for error messages.
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")). // Red
			Bold(true)

	// helpStyle is for help text.
	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")) // Dark gray

	// searchStyle is for search mode indicator.
	searchStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("45")). // Cyan
			Bold(true)

	// dimStyle is for less important text.
	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")) // Dark gray
)

// Status indicator characters.
const (
	indicatorActive = "*"
	indicatorIdle   = "~"
	indicatorExited = " "
)

// View renders the model to a string for display.
// This is called after every Update to refresh the screen.
func (m Model) View() string {
	// If not ready (no window size yet), show loading message
	if !m.ready {
		return "Loading..."
	}

	var content string

	// Build the view based on current mode
	switch {
	case m.showHelp:
		content = m.viewHelp()
	case m.viewMode == ViewModeSearch:
		content = m.viewList() + "\n" + m.viewSearchBar()
	case m.viewMode == ViewModeGrid:
		// Grid view placeholder - will be implemented in a later phase
		content = m.viewList()
	default:
		content = m.viewList()
	}

	// Build the full view with header and footer
	return m.viewHeader() + "\n" + content + "\n" + m.viewFooter()
}

// viewHeader renders the header bar.
func (m Model) viewHeader() string {
	title := titleStyle.Render("Claude Dashboard")

	// Show session count
	countStr := fmt.Sprintf(" [%d sessions", len(m.sessions))
	if len(m.filteredSessions) != len(m.sessions) {
		countStr += fmt.Sprintf(", %d shown", len(m.filteredSessions))
	}
	countStr += "]"
	count := dimStyle.Render(countStr)

	// Show sort indicator
	sortStr := fmt.Sprintf(" | Sort: %s", m.sortFieldName())
	if m.sortConfig.Ascending {
		sortStr += " (asc)"
	} else {
		sortStr += " (desc)"
	}
	sort := dimStyle.Render(sortStr)

	// Combine header elements
	header := title + count + sort

	// Show loading indicator
	if m.loading {
		header += dimStyle.Render(" | Loading...")
	}

	return header
}

// sortFieldName returns a human-readable name for the current sort field.
func (m Model) sortFieldName() string {
	switch m.sortConfig.Field {
	case session.SortByModTime:
		return "Modified"
	case session.SortByProjectName:
		return "Project"
	case session.SortByStatus:
		return "Status"
	case session.SortByMessageCount:
		return "Messages"
	case session.SortByTurnCount:
		return "Turns"
	case session.SortBySummary:
		return "Summary"
	default:
		return "Modified"
	}
}

// viewList renders the session list view.
func (m Model) viewList() string {
	if len(m.filteredSessions) == 0 {
		if m.loading {
			return "\n  Loading sessions..."
		}
		if len(m.sessions) == 0 {
			return "\n  No sessions found."
		}
		return "\n  No sessions match the current filter."
	}

	var lines []string

	// Calculate visible range based on window height
	visibleHeight := m.windowHeight - headerFooterLines - 2 // Extra padding
	if visibleHeight < 1 {
		visibleHeight = len(m.filteredSessions)
	}

	// Calculate scroll position to keep cursor visible
	startIdx := 0
	if m.cursorIndex >= visibleHeight {
		startIdx = m.cursorIndex - visibleHeight + 1
	}
	endIdx := startIdx + visibleHeight
	if endIdx > len(m.filteredSessions) {
		endIdx = len(m.filteredSessions)
	}

	// Render visible sessions
	for i := startIdx; i < endIdx; i++ {
		sess := m.filteredSessions[i]
		line := m.renderSessionLine(sess, i == m.cursorIndex)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// renderSessionLine renders a single session line.
func (m Model) renderSessionLine(sess *session.Session, selected bool) string {
	// Build the session line content
	var parts []string

	// Status indicator
	indicator := m.renderStatusIndicator(sess.Status)
	parts = append(parts, indicator)

	// Project name (truncated)
	projectName := truncate(sess.ProjectName, 25)
	parts = append(parts, projectName)

	// Summary (truncated to fit remaining width)
	summaryWidth := m.windowWidth - 35 // Leave room for other columns
	if summaryWidth < 10 {
		summaryWidth = 10
	}
	summary := truncate(sess.Summary, summaryWidth)
	parts = append(parts, dimStyle.Render(summary))

	// Join parts with separators
	lineContent := strings.Join(parts, " | ")

	// Apply selection style
	if selected {
		return selectedStyle.Render("> " + lineContent)
	}
	return normalStyle.Render("  " + lineContent)
}

// renderStatusIndicator returns a styled status indicator.
func (m Model) renderStatusIndicator(status session.Status) string {
	switch status {
	case session.StatusActive:
		return statusActiveStyle.Render(indicatorActive)
	case session.StatusIdle:
		return statusIdleStyle.Render(indicatorIdle)
	default:
		return statusExitedStyle.Render(indicatorExited)
	}
}

// viewFooter renders the footer bar with help hints.
func (m Model) viewFooter() string {
	// Show error if present
	if m.lastError != nil {
		return errorStyle.Render("Error: " + m.lastError.Error())
	}

	// Show help hints
	hints := []string{
		"j/k: navigate",
		"enter: select",
		"s: sort",
		"r: refresh",
		"/: search",
		"?: help",
		"q: quit",
	}
	return helpStyle.Render(strings.Join(hints, " | "))
}

// viewSearchBar renders the search input bar.
func (m Model) viewSearchBar() string {
	prompt := searchStyle.Render("Search: ")
	query := m.searchQuery
	cursor := "_"
	return prompt + query + cursor
}

// viewHelp renders the help overlay.
func (m Model) viewHelp() string {
	var lines []string

	lines = append(lines, titleStyle.Render("Help"))
	lines = append(lines, "")

	// Navigation section
	lines = append(lines, "Navigation:")
	lines = append(lines, "  j/k, up/down     Move cursor up/down")
	lines = append(lines, "  g, home          Go to first session")
	lines = append(lines, "  G, end           Go to last session")
	lines = append(lines, "  ctrl+u/d, pgup   Page up/down")
	lines = append(lines, "")

	// Actions section
	lines = append(lines, "Actions:")
	lines = append(lines, "  enter            Select session (navigate to pane)")
	lines = append(lines, "  r                Refresh session list")
	lines = append(lines, "  s                Cycle sort order")
	lines = append(lines, "  /                Search sessions")
	lines = append(lines, "  tab              Toggle list/grid view")
	lines = append(lines, "")

	// Exit section
	lines = append(lines, "Exit:")
	lines = append(lines, "  q, ctrl+c        Quit")
	lines = append(lines, "  esc              Close help/cancel search")
	lines = append(lines, "")

	// Legend section
	lines = append(lines, "Status Legend:")
	lines = append(lines, fmt.Sprintf("  %s  Active (processing)", statusActiveStyle.Render(indicatorActive)))
	lines = append(lines, fmt.Sprintf("  %s  Idle (waiting for input)", statusIdleStyle.Render(indicatorIdle)))
	lines = append(lines, fmt.Sprintf("  %s  Exited (not running)", statusExitedStyle.Render("-")))
	lines = append(lines, "")

	lines = append(lines, helpStyle.Render("Press any key to close help"))

	return strings.Join(lines, "\n")
}

// truncate shortens a string to maxLen characters, adding "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
