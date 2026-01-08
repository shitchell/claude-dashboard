package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shitchell/claude-dashboard/internal/config"
	"github.com/shitchell/claude-dashboard/internal/constants"
	"github.com/shitchell/claude-dashboard/internal/logging"
	"github.com/shitchell/claude-dashboard/internal/session"
)

// UIMode represents the overall UI mode (dashboard or side-panel).
type UIMode int

const (
	// UIModeNormal is the standard full-screen dashboard mode.
	UIModeNormal UIMode = iota

	// UIModeSidePanel is a compact side-panel mode for narrower displays.
	UIModeSidePanel
)

// SidePanelWidthThreshold is the width below which side-panel mode is used.
const SidePanelWidthThreshold = 60

// ViewMode represents the current UI mode.
type ViewMode int

const (
	// ViewModeList displays sessions in a vertical list.
	ViewModeList ViewMode = iota

	// ViewModeGrid displays sessions in a grid layout.
	ViewModeGrid

	// ViewModeHelp displays the help screen.
	ViewModeHelp

	// ViewModeSearch displays the search input.
	ViewModeSearch
)

// Model is the main bubbletea model for the dashboard application.
// It contains all application state and is passed through the
// Init, Update, and View lifecycle.
type Model struct {
	// sessions is the list of Claude Code sessions.
	// This is the source of truth for session data.
	sessions []*session.Session

	// filteredSessions is the list after applying filters.
	// This is what gets displayed to the user.
	filteredSessions []*session.Session

	// cursorIndex is the index of the currently selected session
	// in the filteredSessions list.
	cursorIndex int

	// cursorSessionID tracks the selected session by ID to survive
	// refreshes. When sessions are refreshed, we use this to restore
	// the cursor position.
	cursorSessionID string

	// viewMode is the current UI mode (list, grid, help, search).
	viewMode ViewMode

	// uiMode is the overall UI mode (normal dashboard or side-panel).
	// This is determined by window width.
	uiMode UIMode

	// layout is the current layout implementation for rendering sessions.
	// This can be a ListLayout or GridLayout.
	layout Layout

	// layoutType tracks which layout type is currently active.
	layoutType LayoutType

	// styles contains the current visual styles.
	// Styles are adjusted based on uiMode.
	styles *Styles

	// sortConfig specifies how sessions are sorted.
	sortConfig session.SortConfig

	// filterConfig specifies which sessions are shown.
	filterConfig session.FilterConfig

	// searchQuery is the current search text.
	searchQuery string

	// windowWidth is the terminal width in characters.
	windowWidth int

	// windowHeight is the terminal height in characters.
	windowHeight int

	// loading indicates whether sessions are being loaded.
	loading bool

	// refreshing indicates whether a background refresh is in progress.
	// This is used to show a visual indicator during auto-refresh.
	refreshing bool

	// error holds the last error that occurred, if any.
	// This is displayed to the user until cleared.
	lastError error

	// showHelp indicates whether the help overlay is visible.
	showHelp bool

	// refreshInterval is the time between auto-refreshes.
	refreshInterval time.Duration

	// service is the session service for loading and refreshing.
	service *session.Service

	// config is the application configuration.
	config *config.Config

	// keys defines the key bindings.
	keys KeyMap

	// ready indicates whether the model has finished initialization.
	// This is set to true after receiving the first windowSizeMsg.
	ready bool
}

// ModelConfig contains configuration options for creating a new Model.
type ModelConfig struct {
	// Service is the session service for loading sessions.
	// Required.
	Service *session.Service

	// Config is the application configuration.
	// If nil, defaults are used.
	Config *config.Config

	// Keys is the key bindings.
	// If nil, DefaultKeyMap is used.
	Keys *KeyMap

	// RefreshInterval is the time between auto-refreshes.
	// If zero, DefaultRefreshInterval is used.
	RefreshInterval time.Duration
}

// NewModel creates a new Model with the given configuration.
// The service is required; other options have sensible defaults.
func NewModel(cfg ModelConfig) Model {
	// Use defaults for optional fields
	keys := DefaultKeyMap()
	if cfg.Keys != nil {
		keys = *cfg.Keys
	}

	refreshInterval := constants.DefaultRefreshInterval
	if cfg.RefreshInterval > 0 {
		refreshInterval = cfg.RefreshInterval
	}

	// Determine initial view mode and layout type from config
	viewMode := ViewModeList
	layoutType := LayoutTypeList
	if cfg.Config != nil && cfg.Config.Mode == config.DisplayModeGrid {
		viewMode = ViewModeGrid
		layoutType = LayoutTypeGrid
	}

	// Get sort config from app config
	sortConfig := session.SortConfig{
		Field:     session.SortByModTime,
		Ascending: false,
	}
	if cfg.Config != nil {
		sortConfig = cfg.Config.Sort.ToSessionSortConfig()
	}

	// Get filter config from app config
	filterConfig := session.FilterConfig{}
	if cfg.Config != nil {
		filterConfig = cfg.Config.Filter.ToSessionFilterConfig()
	}

	// Initialize styles (normal mode by default)
	styles := NewStyles()

	// Initialize layout based on config
	var layout Layout
	if layoutType == LayoutTypeGrid {
		layout = NewGridLayout(keys)
	} else {
		layout = NewListLayoutFromConfig(cfg.Config, keys)
	}

	logging.Info("UI model initialized: layout=%s, refreshInterval=%v", layoutType, refreshInterval)
	return Model{
		sessions:         nil,
		filteredSessions: nil,
		cursorIndex:      0,
		cursorSessionID:  "",
		viewMode:         viewMode,
		uiMode:           UIModeNormal,
		layout:           layout,
		layoutType:       layoutType,
		styles:           &styles,
		sortConfig:       sortConfig,
		filterConfig:     filterConfig,
		searchQuery:      "",
		windowWidth:      0,
		windowHeight:     0,
		loading:          true,
		refreshing:       false,
		lastError:        nil,
		showHelp:         false,
		refreshInterval:  refreshInterval,
		service:          cfg.Service,
		config:           cfg.Config,
		keys:             keys,
		ready:            false,
	}
}

// Init initializes the model and returns the initial command.
// This is called once when the program starts.
func (m Model) Init() tea.Cmd {
	logging.Debug("UI Init called, starting session load and refresh ticker")
	return tea.Batch(
		// Load sessions asynchronously
		m.loadSessionsCmd(),
		// Start the refresh ticker
		m.tickCmd(),
	)
}

// loadSessionsCmd returns a command that loads sessions from disk.
func (m Model) loadSessionsCmd() tea.Cmd {
	return func() tea.Msg {
		if m.service == nil {
			return sessionsLoadedMsg{
				Sessions: nil,
				Error:    nil,
			}
		}

		sessions, err := m.service.LoadAll()
		return sessionsLoadedMsg{
			Sessions: sessions,
			Error:    err,
		}
	}
}

// refreshSessionsCmd returns a command that refreshes the session list.
func (m Model) refreshSessionsCmd() tea.Cmd {
	return func() tea.Msg {
		if m.service == nil {
			return sessionsRefreshedMsg{
				Sessions: nil,
				Error:    nil,
			}
		}

		sessions, err := m.service.Refresh(m.sessions)
		return sessionsRefreshedMsg{
			Sessions: sessions,
			Error:    err,
		}
	}
}

// tickCmd returns a command that sends a tick after the refresh interval.
// Returns nil if auto-refresh is disabled or the interval is zero.
func (m Model) tickCmd() tea.Cmd {
	// Check if auto-refresh is enabled via config
	if m.config != nil && !m.config.Refresh.Enabled {
		return nil
	}

	// Check if refresh interval is valid
	if m.refreshInterval == 0 {
		return nil
	}

	return tea.Tick(m.refreshInterval, func(t time.Time) tea.Msg {
		return refreshTickMsg(t)
	})
}

// SelectedSession returns the currently selected session, or nil if none.
func (m Model) SelectedSession() *session.Session {
	if m.cursorIndex < 0 || m.cursorIndex >= len(m.filteredSessions) {
		return nil
	}
	return m.filteredSessions[m.cursorIndex]
}

// SessionCount returns the total number of sessions (before filtering).
func (m Model) SessionCount() int {
	return len(m.sessions)
}

// FilteredSessionCount returns the number of sessions after filtering.
func (m Model) FilteredSessionCount() int {
	return len(m.filteredSessions)
}

// IsLoading returns true if sessions are currently being loaded.
func (m Model) IsLoading() bool {
	return m.loading
}

// IsRefreshing returns true if a background refresh is in progress.
func (m Model) IsRefreshing() bool {
	return m.refreshing
}

// LastError returns the last error that occurred, or nil.
func (m Model) LastError() error {
	return m.lastError
}

// applyFiltersAndSort applies the current filter and sort configuration
// to the sessions list and updates filteredSessions.
func (m *Model) applyFiltersAndSort() {
	// Apply filters
	m.filteredSessions = session.ApplyFilters(m.sessions, m.filterConfig)

	// Apply sorting
	session.ApplySorting(m.filteredSessions, m.sortConfig)
}

// restoreCursor attempts to restore the cursor to the session with
// the saved cursorSessionID. If not found, keeps cursor in bounds.
func (m *Model) restoreCursor() {
	// If we have a saved session ID, try to find it
	if m.cursorSessionID != "" {
		for i, sess := range m.filteredSessions {
			if sess.ID == m.cursorSessionID {
				m.cursorIndex = i
				return
			}
		}
	}

	// Session not found or no saved ID - keep cursor in bounds
	m.keepCursorInBounds()
}

// keepCursorInBounds ensures the cursor index is valid.
func (m *Model) keepCursorInBounds() {
	if len(m.filteredSessions) == 0 {
		m.cursorIndex = 0
		m.cursorSessionID = ""
		return
	}

	if m.cursorIndex < 0 {
		m.cursorIndex = 0
	}
	if m.cursorIndex >= len(m.filteredSessions) {
		m.cursorIndex = len(m.filteredSessions) - 1
	}

	// Update the saved session ID
	m.cursorSessionID = m.filteredSessions[m.cursorIndex].ID
}

// saveCursor saves the current cursor position by session ID.
func (m *Model) saveCursor() {
	if m.cursorIndex >= 0 && m.cursorIndex < len(m.filteredSessions) {
		m.cursorSessionID = m.filteredSessions[m.cursorIndex].ID
	}
}

// updateUIMode updates the UI mode based on window width.
// This switches between normal dashboard mode and side-panel mode.
func (m *Model) updateUIMode() {
	newMode := UIModeNormal
	if m.windowWidth < SidePanelWidthThreshold {
		newMode = UIModeSidePanel
	}

	// Only update if mode changed
	if m.uiMode != newMode {
		m.uiMode = newMode

		// Update styles based on mode
		if newMode == UIModeSidePanel {
			styles := NewSidePanelStyles()
			m.styles = &styles
		} else {
			styles := NewStyles()
			m.styles = &styles
		}
	}
}

// switchLayout switches between list and grid layouts.
// This is called when the user presses the toggle key.
func (m *Model) switchLayout() {
	if m.layoutType == LayoutTypeList {
		m.layoutType = LayoutTypeGrid
		m.layout = NewGridLayout(m.keys)
		m.viewMode = ViewModeGrid
	} else {
		m.layoutType = LayoutTypeList
		m.layout = NewListLayoutFromConfig(m.config, m.keys)
		m.viewMode = ViewModeList
	}
}

// SetLayout sets the layout to a specific type.
func (m *Model) SetLayout(layoutType LayoutType) {
	if m.layoutType == layoutType {
		return // No change needed
	}

	m.layoutType = layoutType
	if layoutType == LayoutTypeGrid {
		m.layout = NewGridLayout(m.keys)
		m.viewMode = ViewModeGrid
	} else {
		m.layout = NewListLayoutFromConfig(m.config, m.keys)
		m.viewMode = ViewModeList
	}
}

// GetLayout returns the current layout.
func (m *Model) GetLayout() Layout {
	return m.layout
}

// GetLayoutType returns the current layout type.
func (m *Model) GetLayoutType() LayoutType {
	return m.layoutType
}

// GetUIMode returns the current UI mode.
func (m *Model) GetUIMode() UIMode {
	return m.uiMode
}

// contentHeight returns the available height for the main content area.
// This subtracts the header and footer heights from the total window height.
func (m *Model) contentHeight() int {
	// Header: 1 line
	// Footer: 1 line
	// We need space for separators too
	const headerLines = 1
	const footerLines = 1

	available := m.windowHeight - headerLines - footerLines
	if available < 1 {
		available = 1
	}
	return available
}

// contentWidth returns the available width for the main content area.
func (m *Model) contentWidth() int {
	return m.windowWidth
}
