package ui

import (
	"fmt"
	"strings"

	"github.com/shitchell/claude-dashboard/internal/session"
)

// View rendering constants define layout spacing.
const (
	// HeaderLines is the number of lines used by the header.
	HeaderLines = 1

	// FooterLines is the number of lines used by the footer.
	FooterLines = 1

	// SearchBarLines is the number of lines used by the search bar.
	SearchBarLines = 1
)

// View renders the model to a string for display.
// This is called after every Update to refresh the screen.
// The view function implements the state hierarchy:
//   1. If not ready (no window size), show initial loading
//   2. If loading (sessions not yet loaded), show loading state
//   3. If error occurred, show error state
//   4. If no sessions, show empty state
//   5. Otherwise, show normal session list
func (m Model) View() string {
	// If not ready (no window size yet), show loading message
	if !m.ready {
		return m.renderLoading()
	}

	// If loading sessions (initial load), show loading state
	if m.loading {
		return m.renderFullView(m.renderLoadingContent())
	}

	// If there's an error, show error state
	if m.lastError != nil {
		return m.renderFullView(m.renderErrorContent())
	}

	// If no sessions found, show empty state
	if len(m.sessions) == 0 {
		return m.renderFullView(m.renderEmptyContent())
	}

	// Build the view based on current mode
	if m.showHelp {
		// Help overlay replaces the entire view
		return m.renderFullView(m.viewHelp())
	}

	// Render main content using the layout system
	var content string
	if m.viewMode == ViewModeSearch {
		// In search mode, render layout content with search bar
		content = m.renderLayoutContent()
		content += "\n" + m.viewSearchBar()
	} else {
		// Normal rendering through the layout system
		content = m.renderLayoutContent()
	}

	// Build the full view with header and footer
	return m.renderFullView(content)
}

// renderFullView assembles the header, content, and footer into a complete view.
func (m Model) renderFullView(content string) string {
	var parts []string

	// Header
	parts = append(parts, m.viewHeader())

	// Content
	parts = append(parts, content)

	// Footer
	parts = append(parts, m.viewFooter())

	return strings.Join(parts, "\n")
}

// renderLayoutContent renders the main session content using the current layout.
func (m Model) renderLayoutContent() string {
	// If no layout is set, fall back to legacy list view
	if m.layout == nil {
		return m.viewList()
	}

	// Calculate available height for the layout
	// Subtract header, footer, and any additional UI elements
	availableHeight := m.contentHeight()

	// Handle search mode: reduce height for search bar
	if m.viewMode == ViewModeSearch {
		availableHeight -= SearchBarLines
	}

	// Get the styles to use (already adjusted for UI mode)
	styles := m.styles
	if styles == nil {
		styles = &DefaultStyles
	}

	// Render through the layout
	return m.layout.Render(
		m.filteredSessions,
		m.cursorIndex,
		m.contentWidth(),
		availableHeight,
		styles,
		m.config,
	)
}

// viewHeader renders the header bar.
func (m Model) viewHeader() string {
	styles := m.getStyles()
	title := styles.Title.Render("Claude Dashboard")

	// Show session count
	countStr := fmt.Sprintf(" [%d sessions", len(m.sessions))
	if len(m.filteredSessions) != len(m.sessions) {
		countStr += fmt.Sprintf(", %d shown", len(m.filteredSessions))
	}
	countStr += "]"
	count := styles.Dim.Render(countStr)

	// Show sort indicator
	sortStr := fmt.Sprintf(" | Sort: %s", m.sortFieldName())
	if m.sortConfig.Ascending {
		sortStr += " (asc)"
	} else {
		sortStr += " (desc)"
	}
	sort := styles.Dim.Render(sortStr)

	// Show layout indicator
	layoutStr := fmt.Sprintf(" | %s", m.layout.Name())
	layout := styles.Dim.Render(layoutStr)

	// Combine header elements
	header := title + count + sort + layout

	// Show loading indicator
	if m.loading {
		header += styles.Dim.Render(" | Loading...")
	}

	// In side-panel mode, truncate header if needed
	if m.uiMode == UIModeSidePanel && m.windowWidth > 0 {
		// Simplified header for narrow displays
		header = title + count
		if m.loading {
			header += styles.Dim.Render(" ...")
		}
	}

	return header
}

// getStyles returns the current styles, falling back to defaults if nil.
func (m Model) getStyles() *Styles {
	if m.styles != nil {
		return m.styles
	}
	return &DefaultStyles
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
// This is the legacy list view used as a fallback when no layout is set.
func (m Model) viewList() string {
	styles := m.getStyles()

	if len(m.filteredSessions) == 0 {
		if m.loading {
			return "\n  " + styles.Dim.Render("Loading sessions...")
		}
		if len(m.sessions) == 0 {
			return "\n  " + styles.Dim.Render("No sessions found.")
		}
		return "\n  " + styles.Dim.Render("No sessions match the current filter.")
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
// This is the legacy renderer used as a fallback when no layout is set.
func (m Model) renderSessionLine(sess *session.Session, selected bool) string {
	styles := m.getStyles()

	// Build the session line content
	var parts []string

	// Status indicator
	indicator := styles.RenderStatusIndicator(sess.Status)
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
	parts = append(parts, styles.Dim.Render(summary))

	// Join parts with separators
	lineContent := strings.Join(parts, " | ")

	// Apply selection style
	if selected {
		return styles.ItemSelected.Render("> " + lineContent)
	}
	return styles.ItemNormal.Render("  " + lineContent)
}

// viewFooter renders the footer bar with help hints.
func (m Model) viewFooter() string {
	styles := m.getStyles()

	// Show error if present
	if m.lastError != nil {
		return styles.Error.Render("Error: " + m.lastError.Error())
	}

	// Build help hints based on current mode
	var hints []string

	if m.viewMode == ViewModeSearch {
		// Search mode hints
		hints = []string{
			"type: search",
			"enter: confirm",
			"esc: cancel",
		}
	} else {
		// Normal mode hints - adjust for available width
		if m.uiMode == UIModeSidePanel {
			// Compact hints for side-panel mode
			hints = []string{
				"j/k",
				"enter",
				"?",
				"q",
			}
		} else {
			// Full hints for normal mode
			hints = []string{
				"j/k: navigate",
				"enter: select",
				"s: sort",
				"r: refresh",
				"/: search",
				"tab: " + m.alternateLayoutName(),
				"?: help",
				"q: quit",
			}
		}
	}

	return styles.Help.Render(strings.Join(hints, " | "))
}

// alternateLayoutName returns the name of the layout that will be activated
// when the user presses the toggle key.
func (m Model) alternateLayoutName() string {
	if m.layoutType == LayoutTypeList {
		return "grid"
	}
	return "list"
}

// viewSearchBar renders the search input bar.
func (m Model) viewSearchBar() string {
	styles := m.getStyles()
	prompt := styles.SearchPrompt.Render("Search: ")
	query := m.searchQuery
	cursor := styles.SearchInput.Render("_")
	return prompt + query + cursor
}

// viewHelp renders the help overlay.
func (m Model) viewHelp() string {
	styles := m.getStyles()
	var lines []string

	// Title
	lines = append(lines, styles.Title.Render(" Help "))
	lines = append(lines, "")

	// Build help entries with consistent formatting
	type helpEntry struct {
		key  string
		desc string
	}

	// Navigation section
	lines = append(lines, styles.HelpSection.Render("Navigation:"))
	navEntries := []helpEntry{
		{"j/k, up/down", "Move cursor up/down"},
		{"h/l, left/right", "Move cursor left/right (grid)"},
		{"g, home", "Go to first session"},
		{"G, end", "Go to last session"},
		{"ctrl+u/d, pgup/pgdn", "Page up/down"},
	}
	for _, e := range navEntries {
		lines = append(lines, m.formatHelpEntry(e.key, e.desc, styles))
	}
	lines = append(lines, "")

	// Actions section
	lines = append(lines, styles.HelpSection.Render("Actions:"))
	actionEntries := []helpEntry{
		{"enter", "Select session (navigate to pane)"},
		{"r", "Refresh session list"},
		{"s", "Cycle sort order"},
		{"/", "Search sessions"},
		{"tab", "Toggle list/grid view"},
	}
	for _, e := range actionEntries {
		lines = append(lines, m.formatHelpEntry(e.key, e.desc, styles))
	}
	lines = append(lines, "")

	// Exit section
	lines = append(lines, styles.HelpSection.Render("Exit:"))
	exitEntries := []helpEntry{
		{"q, ctrl+c", "Quit"},
		{"esc", "Close help/cancel search"},
	}
	for _, e := range exitEntries {
		lines = append(lines, m.formatHelpEntry(e.key, e.desc, styles))
	}
	lines = append(lines, "")

	// Status Legend section
	lines = append(lines, styles.HelpSection.Render("Status Legend:"))
	lines = append(lines, fmt.Sprintf("  %s  Active (processing)", styles.StatusActive.Render(IndicatorActive)))
	lines = append(lines, fmt.Sprintf("  %s  Idle (waiting for input)", styles.StatusIdle.Render(IndicatorIdle)))
	lines = append(lines, fmt.Sprintf("  %s  Exited (not running)", styles.StatusExited.Render("-")))
	lines = append(lines, "")

	// Current mode info
	lines = append(lines, styles.HelpSection.Render("Current Mode:"))
	lines = append(lines, fmt.Sprintf("  Layout: %s", m.layout.Name()))
	uiModeStr := "Normal"
	if m.uiMode == UIModeSidePanel {
		uiModeStr = "Side Panel"
	}
	lines = append(lines, fmt.Sprintf("  UI Mode: %s", uiModeStr))
	lines = append(lines, "")

	// Close hint
	lines = append(lines, styles.Help.Render("Press any key to close help"))

	return strings.Join(lines, "\n")
}

// formatHelpEntry formats a single help entry with consistent spacing.
func (m Model) formatHelpEntry(key, desc string, styles *Styles) string {
	// Use a fixed width for the key column
	const keyWidth = 22
	paddedKey := key
	if len(key) < keyWidth {
		paddedKey = key + strings.Repeat(" ", keyWidth-len(key))
	}

	return "  " + styles.HelpKey.Render(paddedKey) + styles.HelpDesc.Render(desc)
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

// =============================================================================
// State Rendering Functions
// =============================================================================

// Loading state constants for consistent appearance.
const (
	// LoadingSpinner is the character shown during loading.
	LoadingSpinner = "*"

	// LoadingMessage is the text shown during initial loading.
	LoadingMessage = "Loading sessions..."
)

// renderLoading renders the initial loading state before window size is known.
// This is shown briefly at startup before the first window size message.
func (m Model) renderLoading() string {
	return fmt.Sprintf("\n  %s %s\n", LoadingSpinner, LoadingMessage)
}

// renderLoadingContent renders the loading state content for the main view.
// This is shown while sessions are being loaded from disk.
func (m Model) renderLoadingContent() string {
	styles := m.getStyles()

	var lines []string
	lines = append(lines, "")
	lines = append(lines, "")

	// Center the loading message based on available height
	padding := m.contentHeight() / 3
	for i := 0; i < padding; i++ {
		lines = append(lines, "")
	}

	// Loading indicator with spinner
	loadingLine := fmt.Sprintf("  %s %s", LoadingSpinner, LoadingMessage)
	lines = append(lines, styles.Dim.Render(loadingLine))
	lines = append(lines, "")
	lines = append(lines, styles.Dim.Render("  Scanning for Claude Code sessions..."))

	return strings.Join(lines, "\n")
}

// renderErrorContent renders the error state content for the main view.
// This is shown when a fatal error occurs during session loading.
func (m Model) renderErrorContent() string {
	styles := m.getStyles()

	var lines []string
	lines = append(lines, "")
	lines = append(lines, "")

	// Center the error message
	padding := m.contentHeight() / 4
	for i := 0; i < padding; i++ {
		lines = append(lines, "")
	}

	// Error icon and title
	lines = append(lines, styles.Error.Render("  Error Loading Sessions"))
	lines = append(lines, "")

	// Error message
	if m.lastError != nil {
		errMsg := m.lastError.Error()
		// Wrap long error messages
		if len(errMsg) > m.windowWidth-6 && m.windowWidth > 20 {
			errMsg = errMsg[:m.windowWidth-9] + "..."
		}
		lines = append(lines, styles.Dim.Render(fmt.Sprintf("  %s", errMsg)))
	}

	lines = append(lines, "")
	lines = append(lines, "")

	// Recovery hints
	lines = append(lines, styles.Dim.Render("  Possible solutions:"))
	lines = append(lines, styles.Dim.Render("    - Check that ~/.claude/projects exists"))
	lines = append(lines, styles.Dim.Render("    - Verify file permissions"))
	lines = append(lines, styles.Dim.Render("    - Press 'r' to retry loading"))
	lines = append(lines, styles.Dim.Render("    - Press 'q' to quit"))
	lines = append(lines, "")
	lines = append(lines, styles.Help.Render("  Press Escape to dismiss this error"))

	return strings.Join(lines, "\n")
}

// renderEmptyContent renders the empty state content for the main view.
// This is shown when no sessions are found in the projects directory.
func (m Model) renderEmptyContent() string {
	styles := m.getStyles()

	var lines []string
	lines = append(lines, "")
	lines = append(lines, "")

	// Center the message
	padding := m.contentHeight() / 4
	for i := 0; i < padding; i++ {
		lines = append(lines, "")
	}

	// Empty state icon and title
	lines = append(lines, styles.Title.Render("  No Sessions Found"))
	lines = append(lines, "")

	// Explanation
	lines = append(lines, styles.Dim.Render("  No Claude Code sessions were found in the projects directory."))
	lines = append(lines, "")

	// Getting started hints
	lines = append(lines, styles.Dim.Render("  To get started:"))
	lines = append(lines, styles.Dim.Render("    1. Open a terminal in your project directory"))
	lines = append(lines, styles.Dim.Render("    2. Run: claude"))
	lines = append(lines, styles.Dim.Render("    3. Start a conversation with Claude"))
	lines = append(lines, "")
	lines = append(lines, styles.Dim.Render("  Sessions will appear here automatically."))
	lines = append(lines, "")
	lines = append(lines, styles.Help.Render("  Press 'r' to refresh | 'q' to quit"))

	return strings.Join(lines, "\n")
}
