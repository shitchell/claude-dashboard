package ui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/shitchell/claude-dashboard/internal/logging"
	"github.com/shitchell/claude-dashboard/internal/session"
	"github.com/shitchell/claude-dashboard/internal/tmux"
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

	// Session selected (Enter key)
	case sessionSelectedMsg:
		return m.handleSessionSelected(msg)

	// Navigation completed
	case navigationResultMsg:
		return m.handleNavigationResult(msg)

	// Matcher refreshed
	case matcherRefreshedMsg:
		return m.handleMatcherRefreshed(msg)

	// Key presses
	case tea.KeyMsg:
		return m.handleKeyPress(msg)
	}

	return m, nil
}

// handleWindowSize processes window resize events.
func (m Model) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	logging.Debug("Window resize: %dx%d", msg.Width, msg.Height)
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
		logging.Warn("Failed to load sessions: %v", msg.Error)
		m.lastError = msg.Error
		return m, nil
	}

	logging.Info("Sessions loaded: %d total", len(msg.Sessions))
	m.sessions = msg.Sessions
	m.applyFiltersAndSort()
	m.restoreCursor()

	// Chain status update command to determine active/idle/exited status
	return m, m.updateStatusCmd(m.sessions)
}

// handleSessionsRefreshed processes refresh results.
func (m Model) handleSessionsRefreshed(msg sessionsRefreshedMsg) (tea.Model, tea.Cmd) {
	// Clear the refreshing indicator
	m.refreshing = false

	if msg.Error != nil {
		logging.Warn("Failed to refresh sessions: %v", msg.Error)
		m.lastError = msg.Error
		return m, nil
	}

	logging.Debug("Sessions refreshed: %d total", len(msg.Sessions))

	// Save current cursor position before updating
	m.saveCursor()

	m.sessions = msg.Sessions
	m.applyFiltersAndSort()
	m.restoreCursor()

	// Chain status update command to determine active/idle/exited status
	return m, m.updateStatusCmd(m.sessions)
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
	logging.Debug("Auto-refresh tick triggered")
	// Mark refresh as in progress for visual indicator
	m.refreshing = true

	// Schedule the next tick and trigger a refresh
	// Also refresh the matcher in the background for navigation
	return m, tea.Batch(
		m.tickCmd(),
		m.refreshSessionsCmd(),
		m.refreshMatcherCmd(),
	)
}

// handleError processes error messages.
func (m Model) handleError(msg errorMsg) (tea.Model, tea.Cmd) {
	logging.Warn("UI error received: %v", msg.Error)
	m.lastError = msg.Error
	return m, nil
}

// handleKeyPress processes key press events.
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	logging.Debug("Key pressed: %s", msg.String())

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
		logging.Debug("Quit key pressed")
		return m, tea.Quit

	// Navigation keys - delegate to layout for proper handling (grid vs list)
	case key.Matches(msg, m.keys.Up),
		key.Matches(msg, m.keys.Down),
		key.Matches(msg, m.keys.Left),
		key.Matches(msg, m.keys.Right),
		key.Matches(msg, m.keys.PageUp),
		key.Matches(msg, m.keys.PageDown),
		key.Matches(msg, m.keys.Home),
		key.Matches(msg, m.keys.End):
		return m.handleNavigation(msg)

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

// handleNavigation delegates navigation key handling to the current layout.
// This allows grid and list layouts to handle navigation differently.
func (m Model) handleNavigation(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.layout == nil {
		return m, nil
	}

	sessionCount := len(m.filteredSessions)
	newCursor, handled := m.layout.HandleKey(msg, sessionCount, m.cursorIndex)

	if handled {
		m.cursorIndex = newCursor
		m.saveCursor()
	}

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
	logging.Debug("Toggling view mode from %v", m.layoutType)
	m.switchLayout()
	logging.Debug("View mode changed to %v", m.layoutType)
	return m, nil
}

// enterSearchMode enters search/filter mode.
func (m Model) enterSearchMode() (tea.Model, tea.Cmd) {
	logging.Debug("Entering search mode")
	m.viewMode = ViewModeSearch
	m.searchQuery = ""
	return m, nil
}

// handleSessionSelected handles the session selection message (Enter key).
// It attempts to navigate to the tmux pane where the session is running.
func (m Model) handleSessionSelected(msg sessionSelectedMsg) (tea.Model, tea.Cmd) {
	if msg.Session == nil {
		logging.Debug("handleSessionSelected: no session selected")
		return m, nil
	}

	logging.Info("handleSessionSelected: Session=%s ID=%s TmuxPane='%s' CWD='%s' ProjectPath='%s'",
		msg.Session.Summary, msg.Session.ID, msg.Session.TmuxPane, msg.Session.CWD, msg.Session.ProjectPath)

	// If we already know the pane, navigate directly
	if msg.Session.TmuxPane != "" {
		logging.Debug("handleSessionSelected: TmuxPane already known, navigating directly to %s", msg.Session.TmuxPane)
		return m, m.navigateToPane(msg.Session, msg.Session.TmuxPane)
	}

	// Otherwise, try to find the pane via memory scanning
	logging.Debug("handleSessionSelected: TmuxPane empty, will try memory scanning")
	return m, m.findAndNavigateToSession(msg.Session)
}

// handleNavigationResult handles the result of a navigation attempt.
func (m Model) handleNavigationResult(msg navigationResultMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		logging.Warn("Navigation failed: %v", msg.Error)
		// Could show an error to the user here
		return m, nil
	}

	logging.Info("Successfully navigated to pane %s for session %s",
		msg.PaneID, msg.Session.ID)
	return m, nil
}

// handleMatcherRefreshed handles the result of a matcher refresh.
func (m Model) handleMatcherRefreshed(msg matcherRefreshedMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		logging.Warn("Matcher refresh failed: %v", msg.Error)
		// Don't treat this as a fatal error - navigation can still work
		// by falling back to on-demand scanning
		return m, nil
	}

	logging.Debug("Matcher refresh completed successfully")
	return m, nil
}

// navigateToPane returns a command that navigates to the specified pane.
func (m Model) navigateToPane(sess *session.Session, paneID string) tea.Cmd {
	return func() tea.Msg {
		logging.Debug("Navigating to pane %s for session %s", paneID, sess.ID)

		// Get navigation method from config
		method := tmux.NavigationMethodDefault
		if m.config != nil {
			method = m.config.Tmux.NavigationMethod
		}

		// Use navigator factory to create navigator (enables testing with mocks)
		navigator := m.navigatorFactory(method)
		err := navigator.GoToPane(paneID)
		return navigationResultMsg{
			Session: sess,
			PaneID:  paneID,
			Error:   err,
		}
	}
}

// findAndNavigateToSession returns a command that finds the pane via the
// cached matcher and navigates to it. This should be instant if the matcher
// has been refreshed in the background.
func (m Model) findAndNavigateToSession(sess *session.Session) tea.Cmd {
	return func() tea.Msg {
		logging.Info("findAndNavigateToSession: Starting for session %s (ID=%s)", sess.Summary, sess.ID)

		// Try to use the cached matcher first (instant lookup)
		if m.matcher != nil {
			logging.Debug("findAndNavigateToSession: Using cached matcher")
			pane := m.matcher.MatchSessionToPane(sess)
			if pane != nil {
				logging.Info("findAndNavigateToSession: Found pane %s via cached matcher, navigating...", pane.ID)

				// Navigate to the pane
				method := tmux.NavigationMethodDefault
				if m.config != nil {
					method = m.config.Tmux.NavigationMethod
				}

				navigator := m.navigatorFactory(method)
				err := navigator.GoToPane(pane.ID)
				if err != nil {
					logging.Warn("findAndNavigateToSession: GoToPane failed: %v", err)
				} else {
					logging.Info("findAndNavigateToSession: Successfully navigated to pane %s", pane.ID)
				}
				return navigationResultMsg{
					Session: sess,
					PaneID:  pane.ID,
					Error:   err,
				}
			}
			logging.Debug("findAndNavigateToSession: Cached matcher found no pane, trying fresh scan")
		}

		// Fall back to a fresh matcher if cached one doesn't have the mapping
		// This handles newly started Claude sessions that weren't in the last refresh
		logging.Info("findAndNavigateToSession: Falling back to fresh matcher scan")
		matcher := tmux.NewMatcher()

		// Get session file paths for memory scanning
		if m.service != nil {
			paths := m.service.GetSessionFilePaths()
			logging.Info("findAndNavigateToSession: Got %d session file paths from service", len(paths))
			matcher.SetSessionFilePaths(paths)
		} else {
			logging.Warn("findAndNavigateToSession: No service available, cannot get session file paths")
		}

		// Refresh to discover processes and scan memory
		logging.Debug("findAndNavigateToSession: Calling matcher.Refresh()")
		if err := matcher.Refresh(); err != nil {
			logging.Warn("findAndNavigateToSession: Matcher refresh failed: %v", err)
			return navigationResultMsg{
				Session: sess,
				Error:   err,
			}
		}

		// Try to find the pane
		pane := matcher.MatchSessionToPane(sess)
		if pane == nil {
			logging.Info("findAndNavigateToSession: No pane found for session %s (ID=%s, CWD=%s, ProjectPath=%s)",
				sess.Summary, sess.ID, sess.CWD, sess.ProjectPath)
			return navigationResultMsg{
				Session: sess,
				Error:   nil, // Not an error, just no pane found
			}
		}

		logging.Info("findAndNavigateToSession: Found pane %s via fresh scan, navigating...", pane.ID)

		// Navigate to the pane
		method := tmux.NavigationMethodDefault
		if m.config != nil {
			method = m.config.Tmux.NavigationMethod
		}

		navigator := m.navigatorFactory(method)
		err := navigator.GoToPane(pane.ID)
		if err != nil {
			logging.Warn("findAndNavigateToSession: GoToPane failed: %v", err)
		} else {
			logging.Info("findAndNavigateToSession: Successfully navigated to pane %s", pane.ID)
		}
		return navigationResultMsg{
			Session: sess,
			PaneID:  pane.ID,
			Error:   err,
		}
	}
}
