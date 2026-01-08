package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shitchell/claude-dashboard/internal/session"
)

// testGridSessions creates a slice of test sessions for grid testing.
func testGridSessions(count int) []*session.Session {
	sessions := make([]*session.Session, count)
	for i := 0; i < count; i++ {
		sessions[i] = &session.Session{
			ID:           "grid-session-" + itoa(i),
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

// TestNewGridLayout verifies GridLayout creation.
func TestNewGridLayout(t *testing.T) {
	keys := DefaultKeyMap()
	layout := NewGridLayout(keys)

	if layout == nil {
		t.Fatal("NewGridLayout returned nil")
	}

	if layout.scrollOffset != 0 {
		t.Errorf("initial scrollOffset = %d, want 0", layout.scrollOffset)
	}
}

// TestGridLayoutName verifies the layout name.
func TestGridLayoutName(t *testing.T) {
	layout := NewGridLayout(DefaultKeyMap())

	name := layout.Name()

	if name != "Grid" {
		t.Errorf("Name() = %q, want %q", name, "Grid")
	}
}

// TestGridLayoutRenderEmpty verifies empty session list rendering.
func TestGridLayoutRenderEmpty(t *testing.T) {
	layout := NewGridLayout(DefaultKeyMap())
	styles := NewStyles()

	rendered := layout.Render([]*session.Session{}, 0, 120, 20, &styles, nil)

	if !strings.Contains(rendered, "No sessions") {
		t.Errorf("empty render should mention 'No sessions', got %q", rendered)
	}
}

// TestGridLayoutRenderSessions verifies session rendering.
func TestGridLayoutRenderSessions(t *testing.T) {
	layout := NewGridLayout(DefaultKeyMap())
	styles := NewStyles()
	sessions := testGridSessions(6)

	rendered := layout.Render(sessions, 0, 120, 30, &styles, nil)

	// Should contain project names
	for i := 0; i < 6; i++ {
		if !strings.Contains(rendered, "project-"+itoa(i)) {
			t.Errorf("rendered should contain project-%d", i)
		}
	}
}

// TestGridLayoutRenderSelection verifies selected cell highlighting.
func TestGridLayoutRenderSelection(t *testing.T) {
	layout := NewGridLayout(DefaultKeyMap())
	styles := NewStyles()
	sessions := testGridSessions(6)

	// Select second item
	rendered := layout.Render(sessions, 1, 120, 30, &styles, nil)

	// Should contain the selected session
	if !strings.Contains(rendered, "project-1") {
		t.Error("rendered should contain selected project-1")
	}
}

// TestCalculateGridDimensions verifies grid dimension calculation.
func TestCalculateGridDimensions(t *testing.T) {
	layout := NewGridLayout(DefaultKeyMap())

	tests := []struct {
		name         string
		width        int
		minCols      int
		maxCols      int
		minCellWidth int
		maxCellWidth int
	}{
		{
			name:         "wide terminal - 3 columns",
			width:        120,
			minCols:      3,
			maxCols:      4,
			minCellWidth: GridMinCellWidth,
			maxCellWidth: GridMaxCellWidth,
		},
		{
			name:         "medium terminal - 2 columns",
			width:        80,
			minCols:      2,
			maxCols:      2,
			minCellWidth: GridMinCellWidth,
			maxCellWidth: GridMaxCellWidth,
		},
		{
			name:         "narrow terminal - 1 column",
			width:        40,
			minCols:      1,
			maxCols:      1,
			minCellWidth: GridMinCellWidth,
			maxCellWidth: GridMaxCellWidth,
		},
		{
			name:         "zero width",
			width:        0,
			minCols:      1,
			maxCols:      1,
			minCellWidth: GridMinCellWidth,
			maxCellWidth: GridMaxCellWidth,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cols, cellWidth := layout.calculateGridDimensions(tt.width)

			if cols < tt.minCols || cols > tt.maxCols {
				t.Errorf("calculateGridDimensions(%d) cols = %d, want between %d and %d",
					tt.width, cols, tt.minCols, tt.maxCols)
			}

			if cellWidth < tt.minCellWidth || cellWidth > tt.maxCellWidth {
				t.Errorf("calculateGridDimensions(%d) cellWidth = %d, want between %d and %d",
					tt.width, cellWidth, tt.minCellWidth, tt.maxCellWidth)
			}
		})
	}
}

// TestGridLayoutHandleKeyUp verifies up navigation.
func TestGridLayoutHandleKeyUp(t *testing.T) {
	layout := NewGridLayout(DefaultKeyMap())
	styles := NewStyles()
	sessions := testGridSessions(9)

	// Render to set up cached dimensions (3 columns at width 120)
	layout.Render(sessions, 0, 120, 30, &styles, nil)

	tests := []struct {
		name           string
		sessionCount   int
		cursor         int
		expectedCursor int
		expectedHandle bool
	}{
		{"move up from middle row", 9, 4, 1, true},  // col 1, row 1 -> col 1, row 0
		{"at top row stays", 9, 1, 1, true},         // already at top
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

// TestGridLayoutHandleKeyDown verifies down navigation.
func TestGridLayoutHandleKeyDown(t *testing.T) {
	layout := NewGridLayout(DefaultKeyMap())
	styles := NewStyles()
	sessions := testGridSessions(9)

	// Render to set up cached dimensions (3 columns at width 120)
	layout.Render(sessions, 0, 120, 30, &styles, nil)

	tests := []struct {
		name           string
		sessionCount   int
		cursor         int
		expectedCursor int
		expectedHandle bool
	}{
		{"move down from first row", 9, 1, 4, true},  // col 1, row 0 -> col 1, row 1
		{"at bottom row stays", 9, 7, 7, true},       // already at bottom
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

// TestGridLayoutHandleKeyLeft verifies left navigation.
func TestGridLayoutHandleKeyLeft(t *testing.T) {
	layout := NewGridLayout(DefaultKeyMap())
	styles := NewStyles()
	sessions := testGridSessions(9)

	// Render to set up cached dimensions (3 columns at width 120)
	layout.Render(sessions, 0, 120, 30, &styles, nil)

	tests := []struct {
		name           string
		sessionCount   int
		cursor         int
		expectedCursor int
		expectedHandle bool
	}{
		{"move left in row", 9, 1, 0, true},
		{"at left edge stays", 9, 0, 0, true},
		{"at left edge second row", 9, 3, 3, true},
		{"empty list", 0, 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tea.KeyMsg{Type: tea.KeyLeft}
			newCursor, handled := layout.HandleKey(msg, tt.sessionCount, tt.cursor)

			if newCursor != tt.expectedCursor {
				t.Errorf("HandleKey(Left) cursor = %d, want %d", newCursor, tt.expectedCursor)
			}
			if handled != tt.expectedHandle {
				t.Errorf("HandleKey(Left) handled = %v, want %v", handled, tt.expectedHandle)
			}
		})
	}
}

// TestGridLayoutHandleKeyRight verifies right navigation.
func TestGridLayoutHandleKeyRight(t *testing.T) {
	layout := NewGridLayout(DefaultKeyMap())
	styles := NewStyles()
	sessions := testGridSessions(9)

	// Render to set up cached dimensions (3 columns at width 120)
	layout.Render(sessions, 0, 120, 30, &styles, nil)

	tests := []struct {
		name           string
		sessionCount   int
		cursor         int
		expectedCursor int
		expectedHandle bool
	}{
		{"move right in row", 9, 0, 1, true},
		{"at right edge stays", 9, 2, 2, true},
		{"at last item stays", 9, 8, 8, true},
		{"empty list", 0, 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tea.KeyMsg{Type: tea.KeyRight}
			newCursor, handled := layout.HandleKey(msg, tt.sessionCount, tt.cursor)

			if newCursor != tt.expectedCursor {
				t.Errorf("HandleKey(Right) cursor = %d, want %d", newCursor, tt.expectedCursor)
			}
			if handled != tt.expectedHandle {
				t.Errorf("HandleKey(Right) handled = %v, want %v", handled, tt.expectedHandle)
			}
		})
	}
}

// TestGridLayoutHandleKeyPageUp verifies page up navigation.
func TestGridLayoutHandleKeyPageUp(t *testing.T) {
	layout := NewGridLayout(DefaultKeyMap())
	styles := NewStyles()
	sessions := testGridSessions(30)

	// Render to set up cached dimensions (3 columns at width 120)
	layout.Render(sessions, 0, 120, 30, &styles, nil)

	msg := tea.KeyMsg{Type: tea.KeyPgUp}
	// From position 15, should move up by page (3 rows * 3 cols = 9)
	newCursor, handled := layout.HandleKey(msg, 30, 15)

	if newCursor != 6 { // 15 - 9 = 6
		t.Errorf("HandleKey(PageUp) cursor = %d, want 6", newCursor)
	}
	if !handled {
		t.Error("HandleKey(PageUp) should be handled")
	}
}

// TestGridLayoutHandleKeyPageDown verifies page down navigation.
func TestGridLayoutHandleKeyPageDown(t *testing.T) {
	layout := NewGridLayout(DefaultKeyMap())
	styles := NewStyles()
	sessions := testGridSessions(30)

	// Render to set up cached dimensions (3 columns at width 120)
	layout.Render(sessions, 0, 120, 30, &styles, nil)

	msg := tea.KeyMsg{Type: tea.KeyPgDown}
	// From position 5, should move down by page (3 rows * 3 cols = 9)
	newCursor, handled := layout.HandleKey(msg, 30, 5)

	if newCursor != 14 { // 5 + 9 = 14
		t.Errorf("HandleKey(PageDown) cursor = %d, want 14", newCursor)
	}
	if !handled {
		t.Error("HandleKey(PageDown) should be handled")
	}
}

// TestGridLayoutHandleKeyHome verifies home navigation.
func TestGridLayoutHandleKeyHome(t *testing.T) {
	layout := NewGridLayout(DefaultKeyMap())

	msg := tea.KeyMsg{Type: tea.KeyHome}
	newCursor, handled := layout.HandleKey(msg, 30, 15)

	if newCursor != 0 {
		t.Errorf("HandleKey(Home) cursor = %d, want 0", newCursor)
	}
	if !handled {
		t.Error("HandleKey(Home) should be handled")
	}
}

// TestGridLayoutHandleKeyEnd verifies end navigation.
func TestGridLayoutHandleKeyEnd(t *testing.T) {
	layout := NewGridLayout(DefaultKeyMap())

	msg := tea.KeyMsg{Type: tea.KeyEnd}
	newCursor, handled := layout.HandleKey(msg, 30, 15)

	if newCursor != 29 {
		t.Errorf("HandleKey(End) cursor = %d, want 29", newCursor)
	}
	if !handled {
		t.Error("HandleKey(End) should be handled")
	}
}

// TestGridLayoutHandleKeyUnhandled verifies unhandled keys.
func TestGridLayoutHandleKeyUnhandled(t *testing.T) {
	layout := NewGridLayout(DefaultKeyMap())

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}
	newCursor, handled := layout.HandleKey(msg, 10, 5)

	if handled {
		t.Error("random key should not be handled")
	}
	if newCursor != 5 {
		t.Errorf("cursor should be unchanged, got %d", newCursor)
	}
}

// TestGridLayoutRenderScrolling verifies scrolling behavior.
func TestGridLayoutRenderScrolling(t *testing.T) {
	layout := NewGridLayout(DefaultKeyMap())
	styles := NewStyles()
	sessions := testGridSessions(30)

	// Small height to force scrolling
	height := 10

	// Cursor at position 20 should scroll
	rendered := layout.Render(sessions, 20, 120, height, &styles, nil)

	// Should contain the selected session
	if !strings.Contains(rendered, "project-20") {
		t.Error("rendered should contain selected session project-20")
	}
}

// TestGridLayoutRenderScrollIndicator verifies scroll indicator rendering.
func TestGridLayoutRenderScrollIndicator(t *testing.T) {
	layout := NewGridLayout(DefaultKeyMap())
	styles := NewStyles()
	sessions := testGridSessions(30)

	// Small height to force scrolling
	rendered := layout.Render(sessions, 15, 120, 10, &styles, nil)

	// Should contain scroll indicator with "Row"
	if !strings.Contains(rendered, "Row") {
		t.Error("rendered should contain scroll indicator with 'Row'")
	}
}

// TestGridLayoutVisibleRowCount verifies visible row calculation.
func TestGridLayoutVisibleRowCount(t *testing.T) {
	layout := NewGridLayout(DefaultKeyMap())

	tests := []struct {
		name        string
		totalHeight int
		minExpected int
	}{
		{"normal height", 30, 2},
		{"small height", 10, 1},
		{"very small height", 1, 1},
		{"zero height", 0, 1},
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

// TestGridLayoutScrollOffsetBounds verifies scroll offset stays in bounds.
func TestGridLayoutScrollOffsetBounds(t *testing.T) {
	layout := NewGridLayout(DefaultKeyMap())
	styles := NewStyles()
	sessions := testGridSessions(30)

	tests := []struct {
		name   string
		cursor int
	}{
		{"cursor at start", 0},
		{"cursor at end", 29},
		{"cursor in middle", 15},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout.ResetScroll()
			layout.Render(sessions, tt.cursor, 120, 20, &styles, nil)

			offset := layout.GetScrollOffset()
			if offset < 0 {
				t.Errorf("scroll offset = %d, should be >= 0", offset)
			}
		})
	}
}

// TestGridLayoutGetScrollOffset verifies scroll offset access.
func TestGridLayoutGetScrollOffset(t *testing.T) {
	layout := NewGridLayout(DefaultKeyMap())
	styles := NewStyles()
	sessions := testGridSessions(30)

	// Initial offset should be 0
	if layout.GetScrollOffset() != 0 {
		t.Errorf("initial scroll offset = %d, want 0", layout.GetScrollOffset())
	}

	// Render with cursor at position 20 to trigger scroll (assuming small height)
	layout.Render(sessions, 20, 120, 10, &styles, nil)

	// Scroll offset should have changed
	if layout.GetScrollOffset() == 0 {
		t.Error("scroll offset should have changed after rendering with cursor at 20")
	}
}

// TestGridLayoutResetScroll verifies scroll reset.
func TestGridLayoutResetScroll(t *testing.T) {
	layout := NewGridLayout(DefaultKeyMap())
	styles := NewStyles()
	sessions := testGridSessions(30)

	// Trigger scroll
	layout.Render(sessions, 20, 120, 10, &styles, nil)

	// Reset scroll
	layout.ResetScroll()

	if layout.GetScrollOffset() != 0 {
		t.Errorf("scroll offset after reset = %d, want 0", layout.GetScrollOffset())
	}
}

// TestGridLayoutGetCachedDimensions verifies cached dimension access.
func TestGridLayoutGetCachedDimensions(t *testing.T) {
	layout := NewGridLayout(DefaultKeyMap())
	styles := NewStyles()
	sessions := testGridSessions(6)

	// Before render, dimensions should be 0
	cols, cellWidth := layout.GetCachedDimensions()
	if cols != 0 || cellWidth != 0 {
		t.Errorf("initial dimensions = (%d, %d), want (0, 0)", cols, cellWidth)
	}

	// After render, dimensions should be set
	layout.Render(sessions, 0, 120, 30, &styles, nil)
	cols, cellWidth = layout.GetCachedDimensions()

	if cols < 1 {
		t.Errorf("after render cols = %d, want >= 1", cols)
	}
	if cellWidth < GridMinCellWidth || cellWidth > GridMaxCellWidth {
		t.Errorf("after render cellWidth = %d, want between %d and %d",
			cellWidth, GridMinCellWidth, GridMaxCellWidth)
	}
}

// TestGridLayoutInterfaceCompliance verifies Layout interface compliance.
func TestGridLayoutInterfaceCompliance(t *testing.T) {
	var _ Layout = (*GridLayout)(nil)
}

// TestGridLayoutNarrowWidth verifies behavior with very narrow terminal.
func TestGridLayoutNarrowWidth(t *testing.T) {
	layout := NewGridLayout(DefaultKeyMap())
	styles := NewStyles()
	sessions := testGridSessions(6)

	// Very narrow width
	rendered := layout.Render(sessions, 0, 20, 30, &styles, nil)

	// Should not panic and should return something
	if len(rendered) == 0 {
		t.Error("render with narrow width should produce output")
	}
}

// TestGridLayoutZeroHeight verifies behavior with zero height.
func TestGridLayoutZeroHeight(t *testing.T) {
	layout := NewGridLayout(DefaultKeyMap())
	styles := NewStyles()
	sessions := testGridSessions(6)

	// Zero height
	rendered := layout.Render(sessions, 0, 120, 0, &styles, nil)

	// Should not panic and should return something
	if len(rendered) == 0 {
		t.Error("render with zero height should produce output")
	}
}

// TestGridLayoutPartialLastRow verifies handling of incomplete last rows.
func TestGridLayoutPartialLastRow(t *testing.T) {
	layout := NewGridLayout(DefaultKeyMap())
	styles := NewStyles()

	// 7 sessions with 3 columns = 2 full rows + 1 partial row
	sessions := testGridSessions(7)

	rendered := layout.Render(sessions, 6, 120, 30, &styles, nil)

	// Should contain all sessions
	for i := 0; i < 7; i++ {
		if !strings.Contains(rendered, "project-"+itoa(i)) {
			t.Errorf("rendered should contain project-%d", i)
		}
	}
}

// TestGridLayoutDownNavigationLastRow verifies down navigation to partial last row.
func TestGridLayoutDownNavigationLastRow(t *testing.T) {
	layout := NewGridLayout(DefaultKeyMap())
	styles := NewStyles()

	// 7 sessions with 3 columns = positions 0-2, 3-5, 6
	sessions := testGridSessions(7)
	layout.Render(sessions, 0, 120, 30, &styles, nil)

	// From position 4 (middle of second row), going down should go to last item (6)
	msg := tea.KeyMsg{Type: tea.KeyDown}
	newCursor, handled := layout.HandleKey(msg, 7, 4)

	// Should clamp to last session
	if newCursor != 6 {
		t.Errorf("HandleKey(Down) from 4 with 7 sessions = %d, want 6", newCursor)
	}
	if !handled {
		t.Error("HandleKey(Down) should be handled")
	}
}

// TestGridLayoutWidthRecalculation verifies grid dimensions are recalculated on width change.
func TestGridLayoutWidthRecalculation(t *testing.T) {
	layout := NewGridLayout(DefaultKeyMap())
	styles := NewStyles()
	sessions := testGridSessions(9)

	// Render at width 120
	layout.Render(sessions, 0, 120, 30, &styles, nil)
	cols120, _ := layout.GetCachedDimensions()

	// Render at width 80
	layout.Render(sessions, 0, 80, 30, &styles, nil)
	cols80, _ := layout.GetCachedDimensions()

	// Columns should be different (or at least recalculated)
	// With 120 width we expect ~3 cols, with 80 width we expect ~2 cols
	if cols120 <= cols80 && cols120 >= 3 && cols80 >= 3 {
		// Both have 3+ columns, which is unexpected for width 80
		t.Errorf("column counts should differ: 120px=%d, 80px=%d", cols120, cols80)
	}
}

// TestGridLayout2DCursorMath verifies 2D cursor position calculations.
func TestGridLayout2DCursorMath(t *testing.T) {
	layout := NewGridLayout(DefaultKeyMap())
	styles := NewStyles()
	sessions := testGridSessions(12)

	// Render to set up 3 columns
	layout.Render(sessions, 0, 120, 30, &styles, nil)
	cols, _ := layout.GetCachedDimensions()

	if cols != 3 {
		t.Skipf("test expects 3 columns but got %d", cols)
	}

	// Test cursor positions
	// Row 0: positions 0, 1, 2
	// Row 1: positions 3, 4, 5
	// Row 2: positions 6, 7, 8
	// Row 3: positions 9, 10, 11

	tests := []struct {
		name      string
		keyType   tea.KeyType
		start     int
		expected  int
	}{
		{"right from 0", tea.KeyRight, 0, 1},
		{"right from 1", tea.KeyRight, 1, 2},
		{"right at edge 2", tea.KeyRight, 2, 2},
		{"left from 2", tea.KeyLeft, 2, 1},
		{"left from 1", tea.KeyLeft, 1, 0},
		{"left at edge 0", tea.KeyLeft, 0, 0},
		{"down from 0", tea.KeyDown, 0, 3},
		{"down from 4", tea.KeyDown, 4, 7},
		{"up from 7", tea.KeyUp, 7, 4},
		{"up from 3", tea.KeyUp, 3, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tea.KeyMsg{Type: tt.keyType}
			newCursor, _ := layout.HandleKey(msg, 12, tt.start)

			if newCursor != tt.expected {
				t.Errorf("from %d, got %d, want %d", tt.start, newCursor, tt.expected)
			}
		})
	}
}

// TestGridLayoutEmptyCell verifies empty cell rendering for partial rows.
func TestGridLayoutEmptyCell(t *testing.T) {
	layout := NewGridLayout(DefaultKeyMap())
	styles := NewStyles()

	// 4 sessions with 3 columns = 1 full row + 1 item in second row
	sessions := testGridSessions(4)

	// This should not panic and should render properly
	rendered := layout.Render(sessions, 0, 120, 30, &styles, nil)

	if len(rendered) == 0 {
		t.Error("render should produce output")
	}
}

// TestGridLayoutJoinCellsHorizontally verifies horizontal cell joining.
func TestGridLayoutJoinCellsHorizontally(t *testing.T) {
	layout := NewGridLayout(DefaultKeyMap())

	// Test with cells of different line counts
	cells := []string{
		"line1\nline2\nline3",
		"a\nb",
		"x\ny\nz",
	}

	result := layout.joinCellsHorizontally(cells)

	if len(result) != 3 {
		t.Errorf("joinCellsHorizontally should return 3 lines, got %d", len(result))
	}

	// Each line should contain parts from each cell
	if !strings.Contains(result[0], "line1") {
		t.Error("first line should contain 'line1'")
	}
	if !strings.Contains(result[0], "a") {
		t.Error("first line should contain 'a'")
	}
	if !strings.Contains(result[0], "x") {
		t.Error("first line should contain 'x'")
	}
}

// TestGridLayoutJoinCellsEmpty verifies empty cell array handling.
func TestGridLayoutJoinCellsEmpty(t *testing.T) {
	layout := NewGridLayout(DefaultKeyMap())

	result := layout.joinCellsHorizontally([]string{})

	if result != nil {
		t.Errorf("joinCellsHorizontally([]) should return nil, got %v", result)
	}
}
