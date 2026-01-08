package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shitchell/claude-dashboard/internal/config"
	"github.com/shitchell/claude-dashboard/internal/session"
)

// testSessions creates a slice of test sessions.
func testSessions(count int) []*session.Session {
	sessions := make([]*session.Session, count)
	for i := 0; i < count; i++ {
		sessions[i] = &session.Session{
			ID:           "session-" + itoa(i),
			FilePath:     "/home/user/.claude/projects/test/session-" + itoa(i) + ".jsonl",
			ProjectPath:  "/home/user/code/project-" + itoa(i),
			ProjectName:  "project-" + itoa(i),
			CWD:          "/home/user/code/project-" + itoa(i),
			Summary:      "Summary for session " + itoa(i),
			Model:        "claude-sonnet-4-20250514",
			Preview:      "Preview text for session " + itoa(i),
			MessageCount: 10 + i,
			TurnCount:    5 + i,
			ModTime:      time.Now().Add(-time.Duration(i) * time.Hour),
			Status:       session.Status(i % 3),
			TmuxPane:     "",
		}
	}
	return sessions
}

// TestNewListLayout verifies ListLayout creation.
func TestNewListLayout(t *testing.T) {
	keys := DefaultKeyMap()

	t.Run("with columns", func(t *testing.T) {
		columns := []Column{&StatusColumn{}, &NameColumn{}}
		layout := NewListLayout(columns, keys)

		if layout == nil {
			t.Fatal("NewListLayout returned nil")
		}

		if len(layout.columns) != 2 {
			t.Errorf("expected 2 columns, got %d", len(layout.columns))
		}
	})

	t.Run("with nil columns uses defaults", func(t *testing.T) {
		layout := NewListLayout(nil, keys)

		if layout == nil {
			t.Fatal("NewListLayout returned nil")
		}

		if len(layout.columns) == 0 {
			t.Error("expected default columns, got empty")
		}
	})

	t.Run("with empty columns uses defaults", func(t *testing.T) {
		layout := NewListLayout([]Column{}, keys)

		if layout == nil {
			t.Fatal("NewListLayout returned nil")
		}

		if len(layout.columns) == 0 {
			t.Error("expected default columns, got empty")
		}
	})
}

// TestNewListLayoutFromConfig verifies config-based creation.
func TestNewListLayoutFromConfig(t *testing.T) {
	keys := DefaultKeyMap()

	t.Run("with config columns", func(t *testing.T) {
		cfg := &config.Config{
			Columns: []config.ColumnType{
				config.ColumnTypeStatus,
				config.ColumnTypeName,
			},
		}

		layout := NewListLayoutFromConfig(cfg, keys)

		if layout == nil {
			t.Fatal("NewListLayoutFromConfig returned nil")
		}

		if len(layout.columns) != 2 {
			t.Errorf("expected 2 columns, got %d", len(layout.columns))
		}
	})

	t.Run("with nil config uses defaults", func(t *testing.T) {
		layout := NewListLayoutFromConfig(nil, keys)

		if layout == nil {
			t.Fatal("NewListLayoutFromConfig returned nil")
		}

		if len(layout.columns) == 0 {
			t.Error("expected default columns, got empty")
		}
	})
}

// TestListLayoutName verifies the layout name.
func TestListLayoutName(t *testing.T) {
	layout := NewListLayout(nil, DefaultKeyMap())

	name := layout.Name()

	if name != "List" {
		t.Errorf("Name() = %q, want %q", name, "List")
	}
}

// TestListLayoutRenderEmpty verifies empty session list rendering.
func TestListLayoutRenderEmpty(t *testing.T) {
	layout := NewListLayout(nil, DefaultKeyMap())
	styles := NewStyles()

	rendered := layout.Render([]*session.Session{}, 0, 80, 20, &styles, nil)

	if !strings.Contains(rendered, "No sessions") {
		t.Errorf("empty render should mention 'No sessions', got %q", rendered)
	}
}

// TestListLayoutRenderSessions verifies session rendering.
func TestListLayoutRenderSessions(t *testing.T) {
	layout := NewListLayout(nil, DefaultKeyMap())
	styles := NewStyles()
	sessions := testSessions(3)

	rendered := layout.Render(sessions, 0, 100, 20, &styles, nil)

	// Should contain header
	if !strings.Contains(rendered, "Name") {
		t.Error("rendered should contain header 'Name'")
	}

	// Should contain project names
	for i := 0; i < 3; i++ {
		if !strings.Contains(rendered, "project-"+itoa(i)) {
			t.Errorf("rendered should contain project-%d", i)
		}
	}
}

// TestListLayoutRenderSelection verifies selected item highlighting.
func TestListLayoutRenderSelection(t *testing.T) {
	layout := NewListLayout(nil, DefaultKeyMap())
	styles := NewStyles()
	sessions := testSessions(3)

	// Select second item
	rendered := layout.Render(sessions, 1, 100, 20, &styles, nil)

	// Should contain selection prefix somewhere
	if !strings.Contains(rendered, ListSelectionPrefix) {
		t.Error("rendered should contain selection prefix")
	}
}

// TestListLayoutRenderScrolling verifies scrolling behavior.
func TestListLayoutRenderScrolling(t *testing.T) {
	layout := NewListLayout(nil, DefaultKeyMap())
	styles := NewStyles()
	sessions := testSessions(50)

	// Small height to force scrolling
	height := 5

	// Cursor at position 40 should scroll
	rendered := layout.Render(sessions, 40, 100, height, &styles, nil)

	// Should contain the selected session
	if !strings.Contains(rendered, "project-40") {
		t.Error("rendered should contain selected session project-40")
	}

	// First session should not be visible
	if strings.Contains(rendered, "project-0") {
		t.Error("project-0 should not be visible when scrolled down")
	}
}

// TestListLayoutRenderScrollIndicator verifies scroll indicator rendering.
func TestListLayoutRenderScrollIndicator(t *testing.T) {
	layout := NewListLayout(nil, DefaultKeyMap())
	styles := NewStyles()
	sessions := testSessions(50)

	// Small height to force scrolling
	rendered := layout.Render(sessions, 25, 100, 10, &styles, nil)

	// Should contain scroll indicator
	if !strings.Contains(rendered, "/50") {
		t.Error("rendered should contain scroll indicator with total count")
	}
}

// TestListLayoutHandleKeyUp verifies up navigation.
func TestListLayoutHandleKeyUp(t *testing.T) {
	layout := NewListLayout(nil, DefaultKeyMap())

	tests := []struct {
		name           string
		sessionCount   int
		cursor         int
		expectedCursor int
		expectedHandle bool
	}{
		{"move up from middle", 10, 5, 4, true},
		{"at top stays", 10, 0, 0, true},
		{"empty list", 0, 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tea.KeyMsg{Type: tea.KeyUp}
			newCursor, handled := layout.HandleKey(msg, tt.sessionCount, tt.cursor)

			if newCursor != tt.expectedCursor {
				t.Errorf("HandleKey(Up) cursor = %d, want %d", newCursor, tt.expectedCursor)
			}
			if handled != tt.expectedHandle {
				t.Errorf("HandleKey(Up) handled = %v, want %v", handled, tt.expectedHandle)
			}
		})
	}
}

// TestListLayoutHandleKeyDown verifies down navigation.
func TestListLayoutHandleKeyDown(t *testing.T) {
	layout := NewListLayout(nil, DefaultKeyMap())

	tests := []struct {
		name           string
		sessionCount   int
		cursor         int
		expectedCursor int
		expectedHandle bool
	}{
		{"move down from middle", 10, 5, 6, true},
		{"at bottom stays", 10, 9, 9, true},
		{"empty list", 0, 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tea.KeyMsg{Type: tea.KeyDown}
			newCursor, handled := layout.HandleKey(msg, tt.sessionCount, tt.cursor)

			if newCursor != tt.expectedCursor {
				t.Errorf("HandleKey(Down) cursor = %d, want %d", newCursor, tt.expectedCursor)
			}
			if handled != tt.expectedHandle {
				t.Errorf("HandleKey(Down) handled = %v, want %v", handled, tt.expectedHandle)
			}
		})
	}
}

// TestListLayoutHandleKeyPageUp verifies page up navigation.
func TestListLayoutHandleKeyPageUp(t *testing.T) {
	layout := NewListLayout(nil, DefaultKeyMap())

	tests := []struct {
		name           string
		sessionCount   int
		cursor         int
		expectedCursor int
	}{
		{"page up from middle", 50, 25, 15},
		{"page up clamps to start", 50, 5, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tea.KeyMsg{Type: tea.KeyPgUp}
			newCursor, handled := layout.HandleKey(msg, tt.sessionCount, tt.cursor)

			if newCursor != tt.expectedCursor {
				t.Errorf("HandleKey(PageUp) cursor = %d, want %d", newCursor, tt.expectedCursor)
			}
			if !handled {
				t.Error("HandleKey(PageUp) should be handled")
			}
		})
	}
}

// TestListLayoutHandleKeyPageDown verifies page down navigation.
func TestListLayoutHandleKeyPageDown(t *testing.T) {
	layout := NewListLayout(nil, DefaultKeyMap())

	tests := []struct {
		name           string
		sessionCount   int
		cursor         int
		expectedCursor int
	}{
		{"page down from middle", 50, 25, 35},
		{"page down clamps to end", 50, 45, 49},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tea.KeyMsg{Type: tea.KeyPgDown}
			newCursor, handled := layout.HandleKey(msg, tt.sessionCount, tt.cursor)

			if newCursor != tt.expectedCursor {
				t.Errorf("HandleKey(PageDown) cursor = %d, want %d", newCursor, tt.expectedCursor)
			}
			if !handled {
				t.Error("HandleKey(PageDown) should be handled")
			}
		})
	}
}

// TestListLayoutHandleKeyHome verifies home navigation.
func TestListLayoutHandleKeyHome(t *testing.T) {
	layout := NewListLayout(nil, DefaultKeyMap())

	msg := tea.KeyMsg{Type: tea.KeyHome}
	newCursor, handled := layout.HandleKey(msg, 50, 25)

	if newCursor != 0 {
		t.Errorf("HandleKey(Home) cursor = %d, want 0", newCursor)
	}
	if !handled {
		t.Error("HandleKey(Home) should be handled")
	}
}

// TestListLayoutHandleKeyEnd verifies end navigation.
func TestListLayoutHandleKeyEnd(t *testing.T) {
	layout := NewListLayout(nil, DefaultKeyMap())

	msg := tea.KeyMsg{Type: tea.KeyEnd}
	newCursor, handled := layout.HandleKey(msg, 50, 25)

	if newCursor != 49 {
		t.Errorf("HandleKey(End) cursor = %d, want 49", newCursor)
	}
	if !handled {
		t.Error("HandleKey(End) should be handled")
	}
}

// TestListLayoutHandleKeyUnhandled verifies unhandled keys.
func TestListLayoutHandleKeyUnhandled(t *testing.T) {
	layout := NewListLayout(nil, DefaultKeyMap())

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}
	newCursor, handled := layout.HandleKey(msg, 10, 5)

	if handled {
		t.Error("random key should not be handled")
	}
	if newCursor != 5 {
		t.Errorf("cursor should be unchanged, got %d", newCursor)
	}
}

// TestListLayoutSetColumns verifies column updating.
func TestListLayoutSetColumns(t *testing.T) {
	layout := NewListLayout(nil, DefaultKeyMap())

	newColumns := []Column{&StatusColumn{}, &NameColumn{}}
	layout.SetColumns(newColumns)

	if len(layout.GetColumns()) != 2 {
		t.Errorf("expected 2 columns after SetColumns, got %d", len(layout.GetColumns()))
	}
}

// TestListLayoutGetScrollOffset verifies scroll offset access.
func TestListLayoutGetScrollOffset(t *testing.T) {
	layout := NewListLayout(nil, DefaultKeyMap())
	styles := NewStyles()
	sessions := testSessions(50)

	// Initial offset should be 0
	if layout.GetScrollOffset() != 0 {
		t.Errorf("initial scroll offset = %d, want 0", layout.GetScrollOffset())
	}

	// Render with cursor at position 40 to trigger scroll
	layout.Render(sessions, 40, 100, 5, &styles, nil)

	// Scroll offset should have changed
	if layout.GetScrollOffset() == 0 {
		t.Error("scroll offset should have changed after rendering with cursor at 40")
	}
}

// TestListLayoutResetScroll verifies scroll reset.
func TestListLayoutResetScroll(t *testing.T) {
	layout := NewListLayout(nil, DefaultKeyMap())
	styles := NewStyles()
	sessions := testSessions(50)

	// Trigger scroll
	layout.Render(sessions, 40, 100, 5, &styles, nil)

	// Reset scroll
	layout.ResetScroll()

	if layout.GetScrollOffset() != 0 {
		t.Errorf("scroll offset after reset = %d, want 0", layout.GetScrollOffset())
	}
}

// TestListLayoutVisibleRowCount verifies visible row calculation.
func TestListLayoutVisibleRowCount(t *testing.T) {
	layout := NewListLayout(nil, DefaultKeyMap())

	tests := []struct {
		name         string
		totalHeight  int
		minExpected  int
	}{
		{"normal height", 20, 10},
		{"small height", 5, 1},
		{"very small height", 1, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := layout.visibleRowCount(tt.totalHeight)
			if count < tt.minExpected {
				t.Errorf("visibleRowCount(%d) = %d, want >= %d", tt.totalHeight, count, tt.minExpected)
			}
		})
	}
}

// TestListLayoutScrollOffsetBounds verifies scroll offset stays in bounds.
func TestListLayoutScrollOffsetBounds(t *testing.T) {
	layout := NewListLayout(nil, DefaultKeyMap())
	styles := NewStyles()
	sessions := testSessions(10)

	tests := []struct {
		name   string
		cursor int
	}{
		{"cursor at start", 0},
		{"cursor at end", 9},
		{"cursor in middle", 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout.ResetScroll()
			layout.Render(sessions, tt.cursor, 100, 20, &styles, nil)

			offset := layout.GetScrollOffset()
			if offset < 0 {
				t.Errorf("scroll offset = %d, should be >= 0", offset)
			}
		})
	}
}

// TestListLayoutInterfaceCompliance verifies Layout interface compliance.
func TestListLayoutInterfaceCompliance(t *testing.T) {
	var _ Layout = (*ListLayout)(nil)
}

// TestItoa verifies the internal itoa function.
func TestItoa(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{42, "42"},
		{-1, "-1"},
		{-42, "-42"},
		{12345, "12345"},
		{-12345, "-12345"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := itoa(tt.input)
			if result != tt.expected {
				t.Errorf("itoa(%d) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestFormatPosition verifies position formatting.
func TestFormatPosition(t *testing.T) {
	tests := []struct {
		position int
		total    int
		expected string
	}{
		{1, 10, "1/10"},
		{5, 20, "5/20"},
		{100, 100, "100/100"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatPosition(tt.position, tt.total)
			if result != tt.expected {
				t.Errorf("formatPosition(%d, %d) = %q, want %q", tt.position, tt.total, result, tt.expected)
			}
		})
	}
}

// TestListLayoutWidthRecalculation verifies column widths are recalculated on width change.
func TestListLayoutWidthRecalculation(t *testing.T) {
	layout := NewListLayout(nil, DefaultKeyMap())
	styles := NewStyles()
	sessions := testSessions(3)

	// Render at width 80
	layout.Render(sessions, 0, 80, 20, &styles, nil)
	widths80 := make([]int, len(layout.widths))
	copy(widths80, layout.widths)

	// Render at width 120
	layout.Render(sessions, 0, 120, 20, &styles, nil)
	widths120 := layout.widths

	// Widths should be different
	if len(widths80) != len(widths120) {
		t.Error("width slices should have same length")
		return
	}

	different := false
	for i := range widths80 {
		if widths80[i] != widths120[i] {
			different = true
			break
		}
	}

	if !different {
		t.Error("column widths should change when terminal width changes")
	}
}

// TestListLayoutNarrowWidth verifies behavior with very narrow terminal.
func TestListLayoutNarrowWidth(t *testing.T) {
	layout := NewListLayout(nil, DefaultKeyMap())
	styles := NewStyles()
	sessions := testSessions(3)

	// Very narrow width
	rendered := layout.Render(sessions, 0, 20, 20, &styles, nil)

	// Should not panic and should return something
	if len(rendered) == 0 {
		t.Error("render with narrow width should produce output")
	}
}

// TestListLayoutZeroHeight verifies behavior with zero height.
func TestListLayoutZeroHeight(t *testing.T) {
	layout := NewListLayout(nil, DefaultKeyMap())
	styles := NewStyles()
	sessions := testSessions(3)

	// Zero height
	rendered := layout.Render(sessions, 0, 80, 0, &styles, nil)

	// Should not panic and should return something
	if len(rendered) == 0 {
		t.Error("render with zero height should produce output")
	}
}
