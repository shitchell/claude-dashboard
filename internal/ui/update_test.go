package ui

import (
	"errors"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shitchell/claude-dashboard/internal/session"
)

// TestUpdateWindowSize verifies window size handling.
func TestUpdateWindowSize(t *testing.T) {
	m := NewModel(ModelConfig{})

	// Initially not ready
	if m.ready {
		t.Error("expected ready=false initially")
	}

	// Send window size message
	msg := tea.WindowSizeMsg{Width: 80, Height: 24}
	newModel, cmd := m.Update(msg)

	updatedModel := newModel.(Model)
	if !updatedModel.ready {
		t.Error("expected ready=true after window size")
	}
	if updatedModel.windowWidth != 80 {
		t.Errorf("expected windowWidth=80, got %d", updatedModel.windowWidth)
	}
	if updatedModel.windowHeight != 24 {
		t.Errorf("expected windowHeight=24, got %d", updatedModel.windowHeight)
	}
	if cmd != nil {
		t.Error("expected no command after window size")
	}
}

// TestUpdateSessionsLoaded verifies session loading handling.
func TestUpdateSessionsLoaded(t *testing.T) {
	t.Run("successful load", func(t *testing.T) {
		m := NewModel(ModelConfig{})
		m.loading = true

		sessions := []*session.Session{
			{ID: "1", ProjectName: "project-a"},
			{ID: "2", ProjectName: "project-b"},
		}
		msg := sessionsLoadedMsg{Sessions: sessions, Error: nil}

		newModel, _ := m.Update(msg)
		updatedModel := newModel.(Model)

		if updatedModel.loading {
			t.Error("expected loading=false after load")
		}
		if len(updatedModel.sessions) != 2 {
			t.Errorf("expected 2 sessions, got %d", len(updatedModel.sessions))
		}
		if updatedModel.lastError != nil {
			t.Errorf("expected no error, got %v", updatedModel.lastError)
		}
	})

	t.Run("load with error", func(t *testing.T) {
		m := NewModel(ModelConfig{})
		m.loading = true

		testErr := errors.New("load failed")
		msg := sessionsLoadedMsg{Sessions: nil, Error: testErr}

		newModel, _ := m.Update(msg)
		updatedModel := newModel.(Model)

		if updatedModel.loading {
			t.Error("expected loading=false even on error")
		}
		if updatedModel.lastError != testErr {
			t.Errorf("expected error %v, got %v", testErr, updatedModel.lastError)
		}
	})
}

// TestUpdateSessionsRefreshed verifies refresh handling.
func TestUpdateSessionsRefreshed(t *testing.T) {
	t.Run("successful refresh preserves cursor", func(t *testing.T) {
		m := NewModel(ModelConfig{})
		m.sessions = []*session.Session{
			{ID: "1", ProjectName: "project-a"},
			{ID: "2", ProjectName: "project-b"},
		}
		m.filteredSessions = m.sessions
		m.cursorIndex = 1
		m.cursorSessionID = "2"

		// New order after refresh
		newSessions := []*session.Session{
			{ID: "2", ProjectName: "project-b"},
			{ID: "1", ProjectName: "project-a"},
			{ID: "3", ProjectName: "project-c"},
		}
		msg := sessionsRefreshedMsg{Sessions: newSessions, Error: nil}

		newModel, _ := m.Update(msg)
		updatedModel := newModel.(Model)

		// Cursor should be restored to session-2, now at index 0
		if updatedModel.cursorSessionID != "2" {
			t.Errorf("expected cursorSessionID=2, got %s", updatedModel.cursorSessionID)
		}
	})
}

// TestUpdateRefreshTick verifies auto-refresh tick handling.
func TestUpdateRefreshTick(t *testing.T) {
	t.Run("sets refreshing state and returns commands", func(t *testing.T) {
		m := NewModel(ModelConfig{})

		msg := refreshTickMsg(time.Now())
		newModel, cmd := m.Update(msg)
		updatedModel := newModel.(Model)

		// Should set refreshing state
		if !updatedModel.refreshing {
			t.Error("expected refreshing=true after tick")
		}

		// Should return a batch command with tick and refresh
		if cmd == nil {
			t.Error("expected command after refresh tick")
		}
	})

	t.Run("refresh completion clears refreshing state", func(t *testing.T) {
		m := NewModel(ModelConfig{})
		m.refreshing = true

		sessions := []*session.Session{{ID: "1"}}
		msg := sessionsRefreshedMsg{Sessions: sessions, Error: nil}

		newModel, _ := m.Update(msg)
		updatedModel := newModel.(Model)

		// Should clear refreshing state
		if updatedModel.refreshing {
			t.Error("expected refreshing=false after refresh completes")
		}
	})

	t.Run("refresh error clears refreshing state", func(t *testing.T) {
		m := NewModel(ModelConfig{})
		m.refreshing = true

		msg := sessionsRefreshedMsg{Sessions: nil, Error: errors.New("refresh failed")}

		newModel, _ := m.Update(msg)
		updatedModel := newModel.(Model)

		// Should clear refreshing state even on error
		if updatedModel.refreshing {
			t.Error("expected refreshing=false after refresh error")
		}
	})
}

// TestUpdateError verifies error message handling.
func TestUpdateError(t *testing.T) {
	m := NewModel(ModelConfig{})

	testErr := errors.New("test error")
	msg := errorMsg{Error: testErr, Context: "test context"}

	newModel, _ := m.Update(msg)
	updatedModel := newModel.(Model)

	if updatedModel.lastError != testErr {
		t.Errorf("expected error %v, got %v", testErr, updatedModel.lastError)
	}
}

// TestUpdateKeyQuit verifies quit key handling.
func TestUpdateKeyQuit(t *testing.T) {
	m := NewModel(ModelConfig{})
	m.ready = true

	// Test 'q' key
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	_, cmd := m.Update(msg)

	// Should return quit command
	if cmd == nil {
		t.Error("expected quit command")
	}
}

// TestUpdateKeyNavigation verifies navigation key handling.
func TestUpdateKeyNavigation(t *testing.T) {
	setupModel := func() Model {
		m := NewModel(ModelConfig{})
		m.ready = true
		m.filteredSessions = []*session.Session{
			{ID: "1"},
			{ID: "2"},
			{ID: "3"},
			{ID: "4"},
			{ID: "5"},
		}
		m.cursorIndex = 2
		m.windowHeight = 10
		return m
	}

	t.Run("down navigation", func(t *testing.T) {
		m := setupModel()

		msg := tea.KeyMsg{Type: tea.KeyDown}
		newModel, _ := m.Update(msg)
		updatedModel := newModel.(Model)

		if updatedModel.cursorIndex != 3 {
			t.Errorf("expected cursorIndex=3, got %d", updatedModel.cursorIndex)
		}
	})

	t.Run("up navigation", func(t *testing.T) {
		m := setupModel()

		msg := tea.KeyMsg{Type: tea.KeyUp}
		newModel, _ := m.Update(msg)
		updatedModel := newModel.(Model)

		if updatedModel.cursorIndex != 1 {
			t.Errorf("expected cursorIndex=1, got %d", updatedModel.cursorIndex)
		}
	})

	t.Run("vim j navigation", func(t *testing.T) {
		m := setupModel()

		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
		newModel, _ := m.Update(msg)
		updatedModel := newModel.(Model)

		if updatedModel.cursorIndex != 3 {
			t.Errorf("expected cursorIndex=3, got %d", updatedModel.cursorIndex)
		}
	})

	t.Run("vim k navigation", func(t *testing.T) {
		m := setupModel()

		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
		newModel, _ := m.Update(msg)
		updatedModel := newModel.(Model)

		if updatedModel.cursorIndex != 1 {
			t.Errorf("expected cursorIndex=1, got %d", updatedModel.cursorIndex)
		}
	})

	t.Run("home navigation", func(t *testing.T) {
		m := setupModel()

		msg := tea.KeyMsg{Type: tea.KeyHome}
		newModel, _ := m.Update(msg)
		updatedModel := newModel.(Model)

		if updatedModel.cursorIndex != 0 {
			t.Errorf("expected cursorIndex=0, got %d", updatedModel.cursorIndex)
		}
	})

	t.Run("end navigation", func(t *testing.T) {
		m := setupModel()

		msg := tea.KeyMsg{Type: tea.KeyEnd}
		newModel, _ := m.Update(msg)
		updatedModel := newModel.(Model)

		if updatedModel.cursorIndex != 4 {
			t.Errorf("expected cursorIndex=4, got %d", updatedModel.cursorIndex)
		}
	})

	t.Run("down at end stays at end", func(t *testing.T) {
		m := setupModel()
		m.cursorIndex = 4

		msg := tea.KeyMsg{Type: tea.KeyDown}
		newModel, _ := m.Update(msg)
		updatedModel := newModel.(Model)

		if updatedModel.cursorIndex != 4 {
			t.Errorf("expected cursorIndex=4, got %d", updatedModel.cursorIndex)
		}
	})

	t.Run("up at start stays at start", func(t *testing.T) {
		m := setupModel()
		m.cursorIndex = 0

		msg := tea.KeyMsg{Type: tea.KeyUp}
		newModel, _ := m.Update(msg)
		updatedModel := newModel.(Model)

		if updatedModel.cursorIndex != 0 {
			t.Errorf("expected cursorIndex=0, got %d", updatedModel.cursorIndex)
		}
	})
}

// TestUpdateKeyRefresh verifies refresh key handling.
func TestUpdateKeyRefresh(t *testing.T) {
	m := NewModel(ModelConfig{})
	m.ready = true

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}
	_, cmd := m.Update(msg)

	if cmd == nil {
		t.Error("expected refresh command")
	}
}

// TestUpdateKeySort verifies sort key handling.
func TestUpdateKeySort(t *testing.T) {
	m := NewModel(ModelConfig{})
	m.ready = true
	m.filteredSessions = []*session.Session{{ID: "1"}}
	initialSort := m.sortConfig.Field

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
	newModel, _ := m.Update(msg)
	updatedModel := newModel.(Model)

	if updatedModel.sortConfig.Field == initialSort {
		t.Error("expected sort field to change")
	}
}

// TestUpdateKeyHelp verifies help toggle handling.
func TestUpdateKeyHelp(t *testing.T) {
	m := NewModel(ModelConfig{})
	m.ready = true

	// Toggle help on
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}}
	newModel, _ := m.Update(msg)
	updatedModel := newModel.(Model)

	if !updatedModel.showHelp {
		t.Error("expected showHelp=true after '?'")
	}

	// Toggle help off
	newModel2, _ := updatedModel.Update(msg)
	updatedModel2 := newModel2.(Model)

	if updatedModel2.showHelp {
		t.Error("expected showHelp=false after second '?'")
	}
}

// TestUpdateKeyEnter verifies enter key handling.
func TestUpdateKeyEnter(t *testing.T) {
	m := NewModel(ModelConfig{})
	m.ready = true
	m.filteredSessions = []*session.Session{
		{ID: "test-session"},
	}
	m.cursorIndex = 0

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	_, cmd := m.Update(msg)

	if cmd == nil {
		t.Error("expected command after enter")
	}
}

// TestUpdateKeyEscape verifies escape key handling.
func TestUpdateKeyEscape(t *testing.T) {
	t.Run("clears error", func(t *testing.T) {
		m := NewModel(ModelConfig{})
		m.ready = true
		m.lastError = errors.New("test error")

		msg := tea.KeyMsg{Type: tea.KeyEscape}
		newModel, _ := m.Update(msg)
		updatedModel := newModel.(Model)

		if updatedModel.lastError != nil {
			t.Error("expected error to be cleared")
		}
	})

	t.Run("closes help", func(t *testing.T) {
		m := NewModel(ModelConfig{})
		m.ready = true
		m.showHelp = true

		msg := tea.KeyMsg{Type: tea.KeyEscape}
		newModel, _ := m.Update(msg)
		updatedModel := newModel.(Model)

		if updatedModel.showHelp {
			t.Error("expected help to be closed")
		}
	})
}

// TestUpdateKeyToggleView verifies view toggle handling.
func TestUpdateKeyToggleView(t *testing.T) {
	m := NewModel(ModelConfig{})
	m.ready = true
	m.viewMode = ViewModeList

	msg := tea.KeyMsg{Type: tea.KeyTab}
	newModel, _ := m.Update(msg)
	updatedModel := newModel.(Model)

	if updatedModel.viewMode != ViewModeGrid {
		t.Error("expected viewMode=ViewModeGrid after tab")
	}

	// Toggle back
	newModel2, _ := updatedModel.Update(msg)
	updatedModel2 := newModel2.(Model)

	if updatedModel2.viewMode != ViewModeList {
		t.Error("expected viewMode=ViewModeList after second tab")
	}
}

// TestUpdateKeySearch verifies search mode handling.
func TestUpdateKeySearch(t *testing.T) {
	t.Run("enters search mode", func(t *testing.T) {
		m := NewModel(ModelConfig{})
		m.ready = true

		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}
		newModel, _ := m.Update(msg)
		updatedModel := newModel.(Model)

		if updatedModel.viewMode != ViewModeSearch {
			t.Error("expected viewMode=ViewModeSearch after '/'")
		}
	})

	t.Run("escape exits search mode", func(t *testing.T) {
		m := NewModel(ModelConfig{})
		m.ready = true
		m.viewMode = ViewModeSearch
		m.searchQuery = "test"

		msg := tea.KeyMsg{Type: tea.KeyEscape}
		newModel, _ := m.Update(msg)
		updatedModel := newModel.(Model)

		if updatedModel.viewMode != ViewModeList {
			t.Error("expected viewMode=ViewModeList after escape in search")
		}
		if updatedModel.searchQuery != "" {
			t.Error("expected searchQuery to be cleared")
		}
	})

	t.Run("enter confirms search", func(t *testing.T) {
		m := NewModel(ModelConfig{})
		m.ready = true
		m.viewMode = ViewModeSearch
		m.searchQuery = "test"

		msg := tea.KeyMsg{Type: tea.KeyEnter}
		newModel, _ := m.Update(msg)
		updatedModel := newModel.(Model)

		if updatedModel.viewMode != ViewModeList {
			t.Error("expected viewMode=ViewModeList after enter in search")
		}
		if updatedModel.searchQuery != "test" {
			t.Error("expected searchQuery to be preserved")
		}
	})

	t.Run("typing updates search query", func(t *testing.T) {
		m := NewModel(ModelConfig{})
		m.ready = true
		m.viewMode = ViewModeSearch
		m.filteredSessions = []*session.Session{{ID: "1"}}

		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
		newModel, _ := m.Update(msg)
		updatedModel := newModel.(Model)

		if updatedModel.searchQuery != "a" {
			t.Errorf("expected searchQuery='a', got %q", updatedModel.searchQuery)
		}
	})

	t.Run("backspace deletes character", func(t *testing.T) {
		m := NewModel(ModelConfig{})
		m.ready = true
		m.viewMode = ViewModeSearch
		m.searchQuery = "test"
		m.filteredSessions = []*session.Session{{ID: "1"}}

		msg := tea.KeyMsg{Type: tea.KeyBackspace}
		newModel, _ := m.Update(msg)
		updatedModel := newModel.(Model)

		if updatedModel.searchQuery != "tes" {
			t.Errorf("expected searchQuery='tes', got %q", updatedModel.searchQuery)
		}
	})
}

// TestCycleSort verifies sort cycling.
func TestCycleSort(t *testing.T) {
	m := NewModel(ModelConfig{})
	m.filteredSessions = []*session.Session{{ID: "1"}}

	// Default is ModTime descending
	if m.sortConfig.Field != session.SortByModTime {
		t.Fatalf("expected initial sort=ModTime, got %v", m.sortConfig.Field)
	}

	// Cycle: ModTime -> ProjectName
	newModel, _ := m.cycleSort()
	m = newModel.(Model)
	if m.sortConfig.Field != session.SortByProjectName {
		t.Errorf("expected ProjectName after first cycle, got %v", m.sortConfig.Field)
	}
	if !m.sortConfig.Ascending {
		t.Error("expected ascending=true for ProjectName")
	}

	// Cycle: ProjectName -> Status
	newModel, _ = m.cycleSort()
	m = newModel.(Model)
	if m.sortConfig.Field != session.SortByStatus {
		t.Errorf("expected Status after second cycle, got %v", m.sortConfig.Field)
	}

	// Cycle: Status -> ModTime
	newModel, _ = m.cycleSort()
	m = newModel.(Model)
	if m.sortConfig.Field != session.SortByModTime {
		t.Errorf("expected ModTime after third cycle, got %v", m.sortConfig.Field)
	}
}

// TestHelpModeKeys verifies key handling in help mode.
func TestHelpModeKeys(t *testing.T) {
	t.Run("quit works in help", func(t *testing.T) {
		m := NewModel(ModelConfig{})
		m.ready = true
		m.showHelp = true

		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
		_, cmd := m.Update(msg)

		if cmd == nil {
			t.Error("expected quit command in help mode")
		}
	})

	t.Run("other keys close help", func(t *testing.T) {
		m := NewModel(ModelConfig{})
		m.ready = true
		m.showHelp = true

		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}
		newModel, _ := m.Update(msg)
		updatedModel := newModel.(Model)

		if updatedModel.showHelp {
			t.Error("expected help to close on other keys")
		}
	})
}

// TestStatusUpdateDoesNotCauseRefiltering verifies that status updates
// change session status indicators but do NOT re-filter the visible session list.
// This is a SUCCESS test - it tests the expected behavior AFTER the fix.
func TestStatusUpdateDoesNotCauseRefiltering(t *testing.T) {
	t.Run("status update preserves filtered session count", func(t *testing.T) {
		m := NewModel(ModelConfig{})
		m.ready = true

		// Create sessions with StatusActive (not exited)
		sessions := []*session.Session{
			{ID: "sess-1", ProjectName: "project-a", Status: session.StatusActive},
			{ID: "sess-2", ProjectName: "project-b", Status: session.StatusActive},
			{ID: "sess-3", ProjectName: "project-c", Status: session.StatusIdle},
		}

		// Enable ExcludeExited filter
		m.filterConfig.ExcludeExited = true

		// Set sessions and apply filters - all 3 should be visible
		m.sessions = sessions
		m.applyFiltersAndSort()

		if len(m.filteredSessions) != 3 {
			t.Fatalf("precondition failed: expected 3 filtered sessions, got %d", len(m.filteredSessions))
		}

		countBefore := len(m.filteredSessions)

		// Create status update message - sessions remain Active/Idle
		updatedSessions := []*session.Session{
			{ID: "sess-1", ProjectName: "project-a", Status: session.StatusIdle}, // Active -> Idle
			{ID: "sess-2", ProjectName: "project-b", Status: session.StatusActive},
			{ID: "sess-3", ProjectName: "project-c", Status: session.StatusIdle},
		}
		msg := statusUpdatedMsg{Sessions: updatedSessions, Error: nil}

		// Apply status update
		newModel, _ := m.Update(msg)
		updatedModel := newModel.(Model)

		countAfter := len(updatedModel.filteredSessions)

		// Status update should NOT change the count of visible sessions
		if countAfter != countBefore {
			t.Errorf("status update changed filtered session count: before=%d, after=%d",
				countBefore, countAfter)
		}

		// Verify status indicators DID change
		var foundSess1 *session.Session
		for _, s := range updatedModel.filteredSessions {
			if s.ID == "sess-1" {
				foundSess1 = s
				break
			}
		}
		if foundSess1 == nil {
			t.Fatal("sess-1 not found in filtered sessions")
		}
		if foundSess1.Status != session.StatusIdle {
			t.Errorf("expected sess-1 status to change to Idle, got %v", foundSess1.Status)
		}
	})

	t.Run("status update does not filter out sessions that become idle", func(t *testing.T) {
		m := NewModel(ModelConfig{})
		m.ready = true

		// All sessions start as Active
		sessions := []*session.Session{
			{ID: "sess-1", ProjectName: "project-a", Status: session.StatusActive},
			{ID: "sess-2", ProjectName: "project-b", Status: session.StatusActive},
		}

		m.filterConfig.ExcludeExited = true
		m.sessions = sessions
		m.applyFiltersAndSort()

		// Both sessions should be visible
		if len(m.filteredSessions) != 2 {
			t.Fatalf("precondition failed: expected 2 filtered sessions, got %d", len(m.filteredSessions))
		}

		// Status update changes both to Idle (still not Exited)
		updatedSessions := []*session.Session{
			{ID: "sess-1", ProjectName: "project-a", Status: session.StatusIdle},
			{ID: "sess-2", ProjectName: "project-b", Status: session.StatusIdle},
		}
		msg := statusUpdatedMsg{Sessions: updatedSessions, Error: nil}

		newModel, _ := m.Update(msg)
		updatedModel := newModel.(Model)

		// Both sessions should still be visible (Idle != Exited)
		if len(updatedModel.filteredSessions) != 2 {
			t.Errorf("expected 2 sessions to remain visible after status update, got %d",
				len(updatedModel.filteredSessions))
		}
	})
}

// TestBug015_FlickerOnRefreshWithExcludeExitedFilter explicitly detects the
// BUG-015 flicker issue: sessions temporarily disappear during refresh when
// ExcludeExited filter is active because new sessions have default StatusExited.
//
// This is a BUG DETECTION test - it should FAIL if the bug is present.
func TestBug015_FlickerOnRefreshWithExcludeExitedFilter(t *testing.T) {
	t.Run("sessions do not disappear during refresh with ExcludeExited filter", func(t *testing.T) {
		m := NewModel(ModelConfig{})
		m.ready = true

		// Start with sessions that have Active status (not exited)
		initialSessions := []*session.Session{
			{ID: "sess-1", ProjectName: "project-a", Status: session.StatusActive},
			{ID: "sess-2", ProjectName: "project-b", Status: session.StatusIdle},
			{ID: "sess-3", ProjectName: "project-c", Status: session.StatusActive},
		}

		// Enable ExcludeExited filter - this is the critical precondition
		m.filterConfig.ExcludeExited = true

		// Set up initial state
		m.sessions = initialSessions
		m.applyFiltersAndSort()

		countBefore := len(m.filteredSessions)
		if countBefore != 3 {
			t.Fatalf("precondition failed: expected 3 visible sessions, got %d", countBefore)
		}

		// Simulate sessionsRefreshedMsg with NEW session objects
		// These have DEFAULT StatusExited (the bug trigger!)
		// In real usage, the Service.Refresh() creates new Session objects
		// that have Status=StatusExited (zero value) before status determination
		refreshedSessions := []*session.Session{
			{ID: "sess-1", ProjectName: "project-a", Status: session.StatusExited}, // Default!
			{ID: "sess-2", ProjectName: "project-b", Status: session.StatusExited}, // Default!
			{ID: "sess-3", ProjectName: "project-c", Status: session.StatusExited}, // Default!
		}
		msg := sessionsRefreshedMsg{Sessions: refreshedSessions, Error: nil}

		// Apply the refresh message
		newModel, _ := m.Update(msg)
		updatedModel := newModel.(Model)

		countAfter := len(updatedModel.filteredSessions)

		// THE BUG: If ExcludeExited filter sees the default StatusExited,
		// it will filter out ALL sessions, causing the count to drop to 0
		if countAfter < countBefore {
			t.Errorf("BUG-015: Session count dropped during refresh! "+
				"Expected %d sessions, got %d. "+
				"Sessions temporarily filtered out due to default StatusExited before status determination. "+
				"This causes UI flicker where the first row disappears momentarily.",
				countBefore, countAfter)
		}
	})

	t.Run("single session does not disappear during refresh", func(t *testing.T) {
		// This tests the exact scenario from the bug report:
		// "first row temporarily disappears (~100-200ms)"
		m := NewModel(ModelConfig{})
		m.ready = true

		// Single session with Active status
		initialSessions := []*session.Session{
			{ID: "active-session", ProjectName: "my-project", Status: session.StatusActive},
		}

		m.filterConfig.ExcludeExited = true
		m.sessions = initialSessions
		m.applyFiltersAndSort()

		if len(m.filteredSessions) != 1 {
			t.Fatalf("precondition failed: expected 1 visible session, got %d", len(m.filteredSessions))
		}

		// Refresh returns session with default StatusExited
		refreshedSessions := []*session.Session{
			{ID: "active-session", ProjectName: "my-project", Status: session.StatusExited},
		}
		msg := sessionsRefreshedMsg{Sessions: refreshedSessions, Error: nil}

		newModel, _ := m.Update(msg)
		updatedModel := newModel.(Model)

		// The session should NOT disappear during refresh
		if len(updatedModel.filteredSessions) != 1 {
			t.Errorf("BUG-015: Session disappeared during refresh! "+
				"Expected 1 session, got %d. "+
				"The ExcludeExited filter removed the session because it had default StatusExited. "+
				"This causes the 'first row temporarily disappears' flicker bug.",
				len(updatedModel.filteredSessions))
		}
	})

	t.Run("session count transitions match bug report pattern", func(t *testing.T) {
		// Tests the exact pattern from bug report: "11 -> 10 -> 11"
		// (though we use smaller numbers for test simplicity)
		m := NewModel(ModelConfig{})
		m.ready = true

		// Create 3 sessions with various active statuses
		initialSessions := []*session.Session{
			{ID: "s1", ProjectName: "p1", Status: session.StatusActive},
			{ID: "s2", ProjectName: "p2", Status: session.StatusIdle},
			{ID: "s3", ProjectName: "p3", Status: session.StatusActive},
		}

		m.filterConfig.ExcludeExited = true
		m.sessions = initialSessions
		m.applyFiltersAndSort()

		countInitial := len(m.filteredSessions)
		if countInitial != 3 {
			t.Fatalf("precondition: expected 3 sessions, got %d", countInitial)
		}

		// STEP 1: Refresh arrives with default StatusExited for all
		refreshedSessions := []*session.Session{
			{ID: "s1", ProjectName: "p1", Status: session.StatusExited},
			{ID: "s2", ProjectName: "p2", Status: session.StatusExited},
			{ID: "s3", ProjectName: "p3", Status: session.StatusExited},
		}
		refreshMsg := sessionsRefreshedMsg{Sessions: refreshedSessions, Error: nil}

		newModel, _ := m.Update(refreshMsg)
		m2 := newModel.(Model)
		countAfterRefresh := len(m2.filteredSessions)

		// STEP 2: Status update arrives with correct statuses
		statusUpdatedSessions := []*session.Session{
			{ID: "s1", ProjectName: "p1", Status: session.StatusActive},
			{ID: "s2", ProjectName: "p2", Status: session.StatusIdle},
			{ID: "s3", ProjectName: "p3", Status: session.StatusActive},
		}
		statusMsg := statusUpdatedMsg{Sessions: statusUpdatedSessions, Error: nil}

		newModel2, _ := m2.Update(statusMsg)
		m3 := newModel2.(Model)
		countAfterStatus := len(m3.filteredSessions)

		// Detect the flicker pattern
		if countAfterRefresh < countInitial && countAfterStatus == countInitial {
			// This is exactly the "3 -> 0 -> 3" pattern (analogous to "11 -> 10 -> 11")
			t.Errorf("BUG-015: Detected flicker pattern! "+
				"Count went %d -> %d -> %d. "+
				"Sessions temporarily disappeared after refresh (count dropped to %d) "+
				"then reappeared after status update (count restored to %d). "+
				"This is the root cause of the UI flicker bug.",
				countInitial, countAfterRefresh, countAfterStatus,
				countAfterRefresh, countAfterStatus)
		}

		// Even if the above didn't catch it, verify no intermediate drop
		if countAfterRefresh != countInitial {
			t.Errorf("BUG-015: Session count changed during refresh! "+
				"Initial: %d, After refresh: %d. "+
				"Sessions should remain visible during refresh cycle.",
				countInitial, countAfterRefresh)
		}
	})
}
