package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shitchell/claude-dashboard/internal/config"
	"github.com/shitchell/claude-dashboard/internal/session"
)

// TestNewModel verifies that NewModel creates a properly initialized model.
func TestNewModel(t *testing.T) {
	t.Run("creates model with default values", func(t *testing.T) {
		cfg := ModelConfig{
			Service: nil, // Nil service is allowed for testing
		}
		m := NewModel(cfg)

		// Verify initial state
		if m.cursorIndex != 0 {
			t.Errorf("expected cursorIndex=0, got %d", m.cursorIndex)
		}
		if !m.loading {
			t.Error("expected loading=true initially")
		}
		if m.ready {
			t.Error("expected ready=false initially")
		}
		if m.showHelp {
			t.Error("expected showHelp=false initially")
		}
		if m.viewMode != ViewModeList {
			t.Errorf("expected viewMode=ViewModeList, got %v", m.viewMode)
		}
	})

	t.Run("uses custom refresh interval", func(t *testing.T) {
		customInterval := 10 * time.Second
		cfg := ModelConfig{
			RefreshInterval: customInterval,
		}
		m := NewModel(cfg)

		if m.refreshInterval != customInterval {
			t.Errorf("expected refreshInterval=%v, got %v", customInterval, m.refreshInterval)
		}
	})
}

// TestInit verifies that Init returns the correct commands.
func TestInit(t *testing.T) {
	cfg := ModelConfig{
		Service: nil,
	}
	m := NewModel(cfg)

	cmd := m.Init()

	// Init should return a batch command (for loading and tick)
	if cmd == nil {
		t.Error("Init() should return a non-nil command")
	}
}

// TestSelectedSession verifies cursor-based session selection.
func TestSelectedSession(t *testing.T) {
	t.Run("returns nil when no sessions", func(t *testing.T) {
		m := NewModel(ModelConfig{})
		m.filteredSessions = nil

		if m.SelectedSession() != nil {
			t.Error("expected nil when no sessions")
		}
	})

	t.Run("returns correct session at cursor", func(t *testing.T) {
		m := NewModel(ModelConfig{})
		m.filteredSessions = []*session.Session{
			{ID: "session-1"},
			{ID: "session-2"},
			{ID: "session-3"},
		}
		m.cursorIndex = 1

		selected := m.SelectedSession()
		if selected == nil {
			t.Fatal("expected non-nil session")
		}
		if selected.ID != "session-2" {
			t.Errorf("expected session-2, got %s", selected.ID)
		}
	})

	t.Run("returns nil when cursor out of bounds", func(t *testing.T) {
		m := NewModel(ModelConfig{})
		m.filteredSessions = []*session.Session{
			{ID: "session-1"},
		}
		m.cursorIndex = 5

		if m.SelectedSession() != nil {
			t.Error("expected nil when cursor out of bounds")
		}
	})
}

// TestApplyFiltersAndSort verifies filter and sort application.
func TestApplyFiltersAndSort(t *testing.T) {
	t.Run("applies search filter", func(t *testing.T) {
		m := NewModel(ModelConfig{})
		m.sessions = []*session.Session{
			{ID: "1", ProjectName: "project-alpha", Summary: "Test summary"},
			{ID: "2", ProjectName: "project-beta", Summary: "Another summary"},
			{ID: "3", ProjectName: "project-gamma", Summary: "Third summary"},
		}
		m.filterConfig.SearchText = "alpha"

		m.applyFiltersAndSort()

		if len(m.filteredSessions) != 1 {
			t.Errorf("expected 1 filtered session, got %d", len(m.filteredSessions))
		}
		if m.filteredSessions[0].ID != "1" {
			t.Errorf("expected session 1, got %s", m.filteredSessions[0].ID)
		}
	})

	t.Run("applies sorting", func(t *testing.T) {
		m := NewModel(ModelConfig{})
		now := time.Now()
		m.sessions = []*session.Session{
			{ID: "1", ProjectName: "charlie", ModTime: now},
			{ID: "2", ProjectName: "alpha", ModTime: now.Add(-time.Hour)},
			{ID: "3", ProjectName: "bravo", ModTime: now.Add(-2 * time.Hour)},
		}
		m.sortConfig.Field = session.SortByProjectName
		m.sortConfig.Ascending = true

		m.applyFiltersAndSort()

		if len(m.filteredSessions) != 3 {
			t.Fatalf("expected 3 sessions, got %d", len(m.filteredSessions))
		}
		// After sorting by project name ascending: alpha, bravo, charlie
		if m.filteredSessions[0].ProjectName != "alpha" {
			t.Errorf("expected first session to be alpha, got %s", m.filteredSessions[0].ProjectName)
		}
		if m.filteredSessions[1].ProjectName != "bravo" {
			t.Errorf("expected second session to be bravo, got %s", m.filteredSessions[1].ProjectName)
		}
		if m.filteredSessions[2].ProjectName != "charlie" {
			t.Errorf("expected third session to be charlie, got %s", m.filteredSessions[2].ProjectName)
		}
	})
}

// TestRestoreCursor verifies cursor restoration after refresh.
func TestRestoreCursor(t *testing.T) {
	t.Run("restores cursor by session ID", func(t *testing.T) {
		m := NewModel(ModelConfig{})
		m.filteredSessions = []*session.Session{
			{ID: "session-1"},
			{ID: "session-2"},
			{ID: "session-3"},
		}
		m.cursorSessionID = "session-2"
		m.cursorIndex = 0 // Wrong position

		m.restoreCursor()

		if m.cursorIndex != 1 {
			t.Errorf("expected cursor at index 1, got %d", m.cursorIndex)
		}
	})

	t.Run("keeps cursor in bounds when session not found", func(t *testing.T) {
		m := NewModel(ModelConfig{})
		m.filteredSessions = []*session.Session{
			{ID: "session-1"},
			{ID: "session-2"},
		}
		m.cursorSessionID = "session-deleted"
		m.cursorIndex = 10

		m.restoreCursor()

		if m.cursorIndex != 1 {
			t.Errorf("expected cursor at last valid index (1), got %d", m.cursorIndex)
		}
	})

	t.Run("handles empty session list", func(t *testing.T) {
		m := NewModel(ModelConfig{})
		m.filteredSessions = nil
		m.cursorIndex = 5

		m.restoreCursor()

		if m.cursorIndex != 0 {
			t.Errorf("expected cursor at 0 for empty list, got %d", m.cursorIndex)
		}
	})
}

// TestKeepCursorInBounds verifies cursor boundary enforcement.
func TestKeepCursorInBounds(t *testing.T) {
	tests := []struct {
		name            string
		sessions        []*session.Session
		cursorIndex     int
		expectedIndex   int
		expectedID      string
	}{
		{
			name:          "cursor negative",
			sessions:      []*session.Session{{ID: "s1"}, {ID: "s2"}},
			cursorIndex:   -1,
			expectedIndex: 0,
			expectedID:    "s1",
		},
		{
			name:          "cursor beyond end",
			sessions:      []*session.Session{{ID: "s1"}, {ID: "s2"}},
			cursorIndex:   10,
			expectedIndex: 1,
			expectedID:    "s2",
		},
		{
			name:          "cursor valid",
			sessions:      []*session.Session{{ID: "s1"}, {ID: "s2"}},
			cursorIndex:   0,
			expectedIndex: 0,
			expectedID:    "s1",
		},
		{
			name:          "empty list",
			sessions:      nil,
			cursorIndex:   5,
			expectedIndex: 0,
			expectedID:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewModel(ModelConfig{})
			m.filteredSessions = tt.sessions
			m.cursorIndex = tt.cursorIndex

			m.keepCursorInBounds()

			if m.cursorIndex != tt.expectedIndex {
				t.Errorf("expected cursorIndex=%d, got %d", tt.expectedIndex, m.cursorIndex)
			}
			if m.cursorSessionID != tt.expectedID {
				t.Errorf("expected cursorSessionID=%q, got %q", tt.expectedID, m.cursorSessionID)
			}
		})
	}
}

// TestSessionCount verifies session counting methods.
func TestSessionCount(t *testing.T) {
	m := NewModel(ModelConfig{})
	m.sessions = []*session.Session{
		{ID: "1"},
		{ID: "2"},
		{ID: "3"},
	}
	m.filteredSessions = []*session.Session{
		{ID: "1"},
		{ID: "2"},
	}

	if m.SessionCount() != 3 {
		t.Errorf("expected SessionCount()=3, got %d", m.SessionCount())
	}
	if m.FilteredSessionCount() != 2 {
		t.Errorf("expected FilteredSessionCount()=2, got %d", m.FilteredSessionCount())
	}
}

// TestLoadSessionsCmd verifies the session loading command.
func TestLoadSessionsCmd(t *testing.T) {
	t.Run("returns empty result when service is nil", func(t *testing.T) {
		m := NewModel(ModelConfig{Service: nil})
		cmd := m.loadSessionsCmd()

		// Execute the command
		msg := cmd()

		// Should return a sessionsLoadedMsg
		loadedMsg, ok := msg.(sessionsLoadedMsg)
		if !ok {
			t.Fatalf("expected sessionsLoadedMsg, got %T", msg)
		}
		if loadedMsg.Error != nil {
			t.Errorf("expected no error, got %v", loadedMsg.Error)
		}
		if loadedMsg.Sessions != nil {
			t.Errorf("expected nil sessions, got %v", loadedMsg.Sessions)
		}
	})
}

// TestTickCmd verifies the tick command with various configurations.
func TestTickCmd(t *testing.T) {
	t.Run("returns command when enabled with valid interval", func(t *testing.T) {
		m := NewModel(ModelConfig{
			RefreshInterval: 100 * time.Millisecond,
		})

		cmd := m.tickCmd()
		if cmd == nil {
			t.Error("tickCmd() should return a non-nil command")
		}
	})

	t.Run("returns nil when interval is zero", func(t *testing.T) {
		m := NewModel(ModelConfig{
			RefreshInterval: 0,
		})
		// Manually set to zero since NewModel uses default
		m.refreshInterval = 0

		cmd := m.tickCmd()
		if cmd != nil {
			t.Error("tickCmd() should return nil when interval is zero")
		}
	})

	t.Run("returns nil when auto-refresh disabled in config", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Refresh.Enabled = false
		cfg.Refresh.Interval = 5 * time.Second

		m := NewModel(ModelConfig{
			Config:          cfg,
			RefreshInterval: 5 * time.Second,
		})

		cmd := m.tickCmd()
		if cmd != nil {
			t.Error("tickCmd() should return nil when auto-refresh is disabled")
		}
	})

	t.Run("returns command when auto-refresh enabled in config", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Refresh.Enabled = true
		cfg.Refresh.Interval = 5 * time.Second

		m := NewModel(ModelConfig{
			Config:          cfg,
			RefreshInterval: 5 * time.Second,
		})

		cmd := m.tickCmd()
		if cmd == nil {
			t.Error("tickCmd() should return a command when auto-refresh is enabled")
		}
	})

	t.Run("returns command when config is nil (defaults to enabled)", func(t *testing.T) {
		m := NewModel(ModelConfig{
			Config:          nil,
			RefreshInterval: 100 * time.Millisecond,
		})

		cmd := m.tickCmd()
		if cmd == nil {
			t.Error("tickCmd() should return a command when config is nil")
		}
	})
}

// TestIsLoading verifies the loading state accessor.
func TestIsLoading(t *testing.T) {
	m := NewModel(ModelConfig{})

	// Initially loading
	if !m.IsLoading() {
		t.Error("expected IsLoading()=true initially")
	}

	// After setting loading to false
	m.loading = false
	if m.IsLoading() {
		t.Error("expected IsLoading()=false after setting loading=false")
	}
}

// TestIsRefreshing verifies the refreshing state accessor.
func TestIsRefreshing(t *testing.T) {
	m := NewModel(ModelConfig{})

	// Initially not refreshing
	if m.IsRefreshing() {
		t.Error("expected IsRefreshing()=false initially")
	}

	// After setting refreshing to true
	m.refreshing = true
	if !m.IsRefreshing() {
		t.Error("expected IsRefreshing()=true after setting refreshing=true")
	}
}

// TestLastError verifies the error accessor.
func TestLastError(t *testing.T) {
	m := NewModel(ModelConfig{})

	// Initially no error
	if m.LastError() != nil {
		t.Error("expected LastError()=nil initially")
	}

	// After setting error
	testErr := tea.ErrProgramKilled
	m.lastError = testErr
	if m.LastError() != testErr {
		t.Errorf("expected LastError()=%v, got %v", testErr, m.LastError())
	}
}
