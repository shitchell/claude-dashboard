package ui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/shitchell/claude-dashboard/internal/session"
)

// Update handles messages and returns the updated model and any commands.
// This is the core of the bubbletea update loop.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	// Window size changes
	case tea.WindowSizeMsg:
		return m.handleWindowSize(msg)

	// Session loading results
	case sessionsLoadedMsg:
		return m.handleSessionsLoaded(msg)

	// Session refresh results
	case sessionsRefreshedMsg:
		return m.handleSessionsRefreshed(msg)

	// Status update results
	case statusUpdatedMsg:
		return m.handleStatusUpdated(msg)

	// Auto-refresh tick
	case refreshTickMsg:
		return m.handleRefreshTick(msg)

	// Error messages
	case errorMsg:
		return m.handleError(msg)

	// Key presses
	case tea.KeyMsg:
		return m.handleKeyPress(msg)
	}

	return m, nil
}

// handleWindowSize processes window resize events.
func (m Model) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.windowWidth = msg.Width
	m.windowHeight = msg.Height
	m.ready = true

	// Update UI mode based on window width
	m.updateUIMode()

	return m, nil
}

// handleSessionsLoaded processes the initial session load results.
func (m Model) handleSessionsLoaded(msg sessionsLoadedMsg) (tea.Model, tea.Cmd) {
	m.loading = false

	if msg.Error != nil {
		m.lastError = msg.Error
		return m, nil
	}

	m.sessions = msg.Sessions
	m.applyFiltersAndSort()
	m.restoreCursor()

	return m, nil
}

// handleSessionsRefreshed processes refresh results.
func (m Model) handleSessionsRefreshed(msg sessionsRefreshedMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		m.lastError = msg.Error
		return m, nil
	}

	// Save current cursor position before updating
	m.saveCursor()

	m.sessions = msg.Sessions
	m.applyFiltersAndSort()
	m.restoreCursor()

	return m, nil
}

// handleStatusUpdated processes status update results.
func (m Model) handleStatusUpdated(msg statusUpdatedMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		m.lastError = msg.Error
		return m, nil
	}

	m.sessions = msg.Sessions
	m.applyFiltersAndSort()
	m.restoreCursor()

	return m, nil
}

// handleRefreshTick processes the auto-refresh tick.
func (m Model) handleRefreshTick(_ refreshTickMsg) (tea.Model, tea.Cmd) {
	// Schedule the next tick and trigger a refresh
	return m, tea.Batch(
		m.tickCmd(),
		m.refreshSessionsCmd(),
	)
}

// handleError processes error messages.
func (m Model) handleError(msg errorMsg) (tea.Model, tea.Cmd) {
	m.lastError = msg.Error
	return m, nil
}

// handleKeyPress processes key press events.
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle help mode separately - most keys close help
	if m.showHelp {
		return m.handleKeyPressInHelp(msg)
	}

	// Handle search mode separately
	if m.viewMode == ViewModeSearch {
		return m.handleKeyPressInSearch(msg)
	}

	// Normal mode key handling
	switch {
	// Quit
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	// Navigation: Up
	case key.Matches(msg, m.keys.Up):
		return m.moveCursorUp()

	// Navigation: Down
	case key.Matches(msg, m.keys.Down):
		return m.moveCursorDown()

	// Navigation: Page Up
	case key.Matches(msg, m.keys.PageUp):
		return m.moveCursorPageUp()

	// Navigation: Page Down
	case key.Matches(msg, m.keys.PageDown):
		return m.moveCursorPageDown()

	// Navigation: Home
	case key.Matches(msg, m.keys.Home):
		return m.moveCursorToStart()

	// Navigation: End
	case key.Matches(msg, m.keys.End):
		return m.moveCursorToEnd()

	// Select session
	case key.Matches(msg, m.keys.Enter):
		return m.selectSession()

	// Refresh
	case key.Matches(msg, m.keys.Refresh):
		return m, m.refreshSessionsCmd()

	// Toggle sort
	case key.Matches(msg, m.keys.Sort):
		return m.cycleSort()

	// Toggle help
	case key.Matches(msg, m.keys.Help):
		return m.toggleHelp()

	// Toggle view mode
	case key.Matches(msg, m.keys.ToggleView):
		return m.toggleViewMode()

	// Enter search mode
	case key.Matches(msg, m.keys.Search):
		return m.enterSearchMode()

	// Escape - clear error or do nothing
	case key.Matches(msg, m.keys.Escape):
		m.lastError = nil
		return m, nil
	}

	return m, nil
}

// handleKeyPressInHelp handles keys when help is visible.
func (m Model) handleKeyPressInHelp(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Most keys close help
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Help), key.Matches(msg, m.keys.Escape):
		m.showHelp = false
		return m, nil
	default:
		// Any other key closes help
		m.showHelp = false
		return m, nil
	}
}

// handleKeyPressInSearch handles keys in search mode.
func (m Model) handleKeyPressInSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Escape):
		// Cancel search
		m.viewMode = ViewModeList
		m.searchQuery = ""
		m.filterConfig.SearchText = ""
		m.applyFiltersAndSort()
		m.restoreCursor()
		return m, nil

	case key.Matches(msg, m.keys.Enter):
		// Confirm search and return to list
		m.viewMode = ViewModeList
		return m, nil

	default:
		// Handle text input for search
		// In a full implementation, this would update a text input component
		// For now, just handle basic character input
		if msg.Type == tea.KeyRunes {
			m.searchQuery += string(msg.Runes)
			m.filterConfig.SearchText = m.searchQuery
			m.applyFiltersAndSort()
			m.keepCursorInBounds()
		} else if msg.Type == tea.KeyBackspace && len(m.searchQuery) > 0 {
			m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
			m.filterConfig.SearchText = m.searchQuery
			m.applyFiltersAndSort()
			m.keepCursorInBounds()
		}
		return m, nil
	}
}

// moveCursorUp moves the cursor up one position.
func (m Model) moveCursorUp() (tea.Model, tea.Cmd) {
	if m.cursorIndex > 0 {
		m.cursorIndex--
		m.saveCursor()
	}
	return m, nil
}

// moveCursorDown moves the cursor down one position.
func (m Model) moveCursorDown() (tea.Model, tea.Cmd) {
	if m.cursorIndex < len(m.filteredSessions)-1 {
		m.cursorIndex++
		m.saveCursor()
	}
	return m, nil
}

// pageSize returns the number of items visible on one page.
// This is calculated based on window height minus header/footer.
const headerFooterLines = 4

func (m Model) pageSize() int {
	size := m.windowHeight - headerFooterLines
	if size < 1 {
		size = 1
	}
	return size
}

// moveCursorPageUp moves the cursor up one page.
func (m Model) moveCursorPageUp() (tea.Model, tea.Cmd) {
	m.cursorIndex -= m.pageSize()
	if m.cursorIndex < 0 {
		m.cursorIndex = 0
	}
	m.saveCursor()
	return m, nil
}

// moveCursorPageDown moves the cursor down one page.
func (m Model) moveCursorPageDown() (tea.Model, tea.Cmd) {
	m.cursorIndex += m.pageSize()
	if m.cursorIndex >= len(m.filteredSessions) {
		m.cursorIndex = len(m.filteredSessions) - 1
	}
	if m.cursorIndex < 0 {
		m.cursorIndex = 0
	}
	m.saveCursor()
	return m, nil
}

// moveCursorToStart moves the cursor to the first item.
func (m Model) moveCursorToStart() (tea.Model, tea.Cmd) {
	m.cursorIndex = 0
	m.saveCursor()
	return m, nil
}

// moveCursorToEnd moves the cursor to the last item.
func (m Model) moveCursorToEnd() (tea.Model, tea.Cmd) {
	if len(m.filteredSessions) > 0 {
		m.cursorIndex = len(m.filteredSessions) - 1
	}
	m.saveCursor()
	return m, nil
}

// selectSession handles session selection (Enter key).
func (m Model) selectSession() (tea.Model, tea.Cmd) {
	selectedSession := m.SelectedSession()
	if selectedSession == nil {
		return m, nil
	}

	// For now, just return - navigation to tmux pane will be added in a later phase
	return m, func() tea.Msg {
		return sessionSelectedMsg{Session: selectedSession}
	}
}

// cycleSort cycles through available sort fields.
func (m Model) cycleSort() (tea.Model, tea.Cmd) {
	// Cycle through sort fields: ModTime -> ProjectName -> Status -> ModTime
	switch m.sortConfig.Field {
	case session.SortByModTime:
		m.sortConfig.Field = session.SortByProjectName
		m.sortConfig.Ascending = true
	case session.SortByProjectName:
		m.sortConfig.Field = session.SortByStatus
		m.sortConfig.Ascending = false
	case session.SortByStatus:
		m.sortConfig.Field = session.SortByModTime
		m.sortConfig.Ascending = false
	default:
		m.sortConfig.Field = session.SortByModTime
		m.sortConfig.Ascending = false
	}

	m.saveCursor()
	m.applyFiltersAndSort()
	m.restoreCursor()

	return m, nil
}

// toggleHelp toggles the help overlay.
func (m Model) toggleHelp() (tea.Model, tea.Cmd) {
	m.showHelp = !m.showHelp
	return m, nil
}

// toggleViewMode toggles between list and grid view.
func (m Model) toggleViewMode() (tea.Model, tea.Cmd) {
	// Use the switchLayout method to toggle layout and view mode together
	m.switchLayout()
	return m, nil
}

// enterSearchMode enters search/filter mode.
func (m Model) enterSearchMode() (tea.Model, tea.Cmd) {
	m.viewMode = ViewModeSearch
	m.searchQuery = ""
	return m, nil
}
