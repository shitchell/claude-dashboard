package ui

import (
	"strings"
	"testing"

	"github.com/shitchell/claude-dashboard/internal/session"
)

// testErr is a simple error type for testing.
type testErr struct {
	msg string
}

func (e testErr) Error() string {
	return e.msg
}

// TestViewNotReady verifies View shows loading when not ready.
func TestViewNotReady(t *testing.T) {
	m := NewModel(ModelConfig{})
	m.ready = false

	view := m.View()

	if view != "Loading..." {
		t.Errorf("expected 'Loading...', got %q", view)
	}
}

// TestViewHeader verifies header rendering.
func TestViewHeader(t *testing.T) {
	m := NewModel(ModelConfig{})
	m.ready = true
	m.sessions = []*session.Session{
		{ID: "1"},
		{ID: "2"},
	}
	m.filteredSessions = m.sessions

	header := m.viewHeader()

	if !strings.Contains(header, "Claude Dashboard") {
		t.Error("expected header to contain title")
	}
	if !strings.Contains(header, "2 sessions") {
		t.Error("expected header to show session count")
	}
}

// TestViewHeaderFiltered verifies header shows filtered count.
func TestViewHeaderFiltered(t *testing.T) {
	m := NewModel(ModelConfig{})
	m.ready = true
	m.sessions = []*session.Session{
		{ID: "1"},
		{ID: "2"},
		{ID: "3"},
	}
	m.filteredSessions = []*session.Session{
		{ID: "1"},
	}

	header := m.viewHeader()

	if !strings.Contains(header, "3 sessions") {
		t.Error("expected header to show total session count")
	}
	if !strings.Contains(header, "1 shown") {
		t.Error("expected header to show filtered count")
	}
}

// TestViewListEmpty verifies empty list message.
func TestViewListEmpty(t *testing.T) {
	t.Run("no sessions found", func(t *testing.T) {
		m := NewModel(ModelConfig{})
		m.ready = true
		m.loading = false
		m.sessions = nil
		m.filteredSessions = nil

		list := m.viewList()

		if !strings.Contains(list, "No sessions found") {
			t.Errorf("expected 'No sessions found', got %q", list)
		}
	})

	t.Run("no sessions match filter", func(t *testing.T) {
		m := NewModel(ModelConfig{})
		m.ready = true
		m.loading = false
		m.sessions = []*session.Session{{ID: "1"}}
		m.filteredSessions = nil

		list := m.viewList()

		if !strings.Contains(list, "No sessions match") {
			t.Errorf("expected filter message, got %q", list)
		}
	})

	t.Run("loading state", func(t *testing.T) {
		m := NewModel(ModelConfig{})
		m.ready = true
		m.loading = true
		m.filteredSessions = nil

		list := m.viewList()

		if !strings.Contains(list, "Loading") {
			t.Errorf("expected loading message, got %q", list)
		}
	})
}

// TestViewListWithSessions verifies session list rendering.
func TestViewListWithSessions(t *testing.T) {
	m := NewModel(ModelConfig{})
	m.ready = true
	m.windowWidth = 80
	m.windowHeight = 24
	m.filteredSessions = []*session.Session{
		{ID: "1", ProjectName: "project-alpha", Summary: "First session"},
		{ID: "2", ProjectName: "project-beta", Summary: "Second session"},
	}
	m.cursorIndex = 0

	list := m.viewList()

	// Check that sessions are rendered
	if !strings.Contains(list, "project-alpha") {
		t.Error("expected list to contain first project name")
	}
	if !strings.Contains(list, "project-beta") {
		t.Error("expected list to contain second project name")
	}
	// Check cursor indicator
	if !strings.Contains(list, ">") {
		t.Error("expected list to contain cursor indicator")
	}
}

// TestViewSessionLine verifies individual session line rendering.
func TestViewSessionLine(t *testing.T) {
	m := NewModel(ModelConfig{})
	m.windowWidth = 100

	tests := []struct {
		name     string
		session  *session.Session
		selected bool
		contains []string
	}{
		{
			name: "selected session",
			session: &session.Session{
				ProjectName: "my-project",
				Summary:     "Test summary",
				Status:      session.StatusIdle,
			},
			selected: true,
			contains: []string{"my-project", ">"},
		},
		{
			name: "unselected session",
			session: &session.Session{
				ProjectName: "other-project",
				Summary:     "Other summary",
				Status:      session.StatusExited,
			},
			selected: false,
			contains: []string{"other-project"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			line := m.renderSessionLine(tt.session, tt.selected)

			for _, expected := range tt.contains {
				if !strings.Contains(line, expected) {
					t.Errorf("expected line to contain %q, got %q", expected, line)
				}
			}
		})
	}
}

// TestViewStatusIndicator verifies status indicator rendering.
func TestViewStatusIndicator(t *testing.T) {
	styles := DefaultStyles

	tests := []struct {
		status   session.Status
		expected string
	}{
		{session.StatusActive, IndicatorActive},
		{session.StatusIdle, IndicatorIdle},
		{session.StatusExited, IndicatorExited},
	}

	for _, tt := range tests {
		t.Run(tt.status.String(), func(t *testing.T) {
			indicator := styles.RenderStatusIndicator(tt.status)

			if !strings.Contains(indicator, tt.expected) {
				t.Errorf("expected indicator to contain %q, got %q", tt.expected, indicator)
			}
		})
	}
}

// TestViewFooter verifies footer rendering.
func TestViewFooter(t *testing.T) {
	t.Run("normal footer", func(t *testing.T) {
		m := NewModel(ModelConfig{})
		m.ready = true

		footer := m.viewFooter()

		// Should contain help hints
		if !strings.Contains(footer, "quit") {
			t.Error("expected footer to contain 'quit' hint")
		}
		if !strings.Contains(footer, "help") {
			t.Error("expected footer to contain 'help' hint")
		}
	})

	t.Run("footer with error", func(t *testing.T) {
		m := NewModel(ModelConfig{})
		m.ready = true

		// Use errors package to create test error
		m.lastError = testErr{"test error message"}

		footer := m.viewFooter()

		if !strings.Contains(footer, "Error") {
			t.Error("expected footer to show error")
		}
		if !strings.Contains(footer, "test error message") {
			t.Error("expected footer to show error message")
		}
	})
}

// TestViewSearchBar verifies search bar rendering.
func TestViewSearchBar(t *testing.T) {
	m := NewModel(ModelConfig{})
	m.searchQuery = "test query"

	bar := m.viewSearchBar()

	if !strings.Contains(bar, "Search") {
		t.Error("expected search bar to contain 'Search'")
	}
	if !strings.Contains(bar, "test query") {
		t.Error("expected search bar to contain query")
	}
	if !strings.Contains(bar, "_") {
		t.Error("expected search bar to contain cursor")
	}
}

// TestViewHelp verifies help view rendering.
func TestViewHelp(t *testing.T) {
	m := NewModel(ModelConfig{})

	help := m.viewHelp()

	// Should contain section headers
	if !strings.Contains(help, "Navigation") {
		t.Error("expected help to contain Navigation section")
	}
	if !strings.Contains(help, "Actions") {
		t.Error("expected help to contain Actions section")
	}
	if !strings.Contains(help, "Exit") {
		t.Error("expected help to contain Exit section")
	}

	// Should contain key bindings
	if !strings.Contains(help, "enter") {
		t.Error("expected help to contain enter key")
	}
	// Help shows "q, ctrl+c" on the Quit line
	if !strings.Contains(help, "ctrl+c") {
		t.Error("expected help to contain quit keys")
	}
}

// TestViewFullOutput verifies complete View output.
func TestViewFullOutput(t *testing.T) {
	m := NewModel(ModelConfig{})
	m.ready = true
	m.windowWidth = 80
	m.windowHeight = 24
	m.sessions = []*session.Session{
		{ID: "1", ProjectName: "test-project", Summary: "Test session"},
	}
	m.filteredSessions = m.sessions

	view := m.View()

	// Should contain header, content, and footer
	if !strings.Contains(view, "Claude Dashboard") {
		t.Error("expected view to contain header")
	}
	if !strings.Contains(view, "test-project") {
		t.Error("expected view to contain session")
	}
}

// TestViewHelpMode verifies help mode view.
func TestViewHelpMode(t *testing.T) {
	m := NewModel(ModelConfig{})
	m.ready = true
	m.showHelp = true
	m.windowWidth = 80
	m.windowHeight = 24

	view := m.View()

	if !strings.Contains(view, "Help") {
		t.Error("expected view to show help overlay")
	}
}

// TestViewSearchMode verifies search mode view.
func TestViewSearchMode(t *testing.T) {
	m := NewModel(ModelConfig{})
	m.ready = true
	m.viewMode = ViewModeSearch
	m.windowWidth = 80
	m.windowHeight = 24
	m.filteredSessions = []*session.Session{{ID: "1"}}

	view := m.View()

	if !strings.Contains(view, "Search") {
		t.Error("expected view to show search bar")
	}
}

// TestTruncate verifies string truncation.
func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is a long string", 10, "this is..."},
		{"abc", 3, "abc"},
		{"abcd", 3, "abc"}, // When maxLen <= 3, no ellipsis
		{"ab", 1, "a"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := truncate(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
			}
		})
	}
}

// TestSortFieldName verifies sort field name conversion.
func TestSortFieldName(t *testing.T) {
	m := NewModel(ModelConfig{})

	tests := []struct {
		field    session.SortField
		expected string
	}{
		{session.SortByModTime, "Modified"},
		{session.SortByProjectName, "Project"},
		{session.SortByStatus, "Status"},
		{session.SortByMessageCount, "Messages"},
		{session.SortByTurnCount, "Turns"},
		{session.SortBySummary, "Summary"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			m.sortConfig.Field = tt.field
			result := m.sortFieldName()
			if result != tt.expected {
				t.Errorf("sortFieldName() = %q, want %q", result, tt.expected)
			}
		})
	}
}
