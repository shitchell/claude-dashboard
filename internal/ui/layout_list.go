// Package ui provides the terminal user interface for claude-dashboard.
// This file implements the ListLayout for displaying sessions in a vertical list.
package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/shitchell/claude-dashboard/internal/config"
	"github.com/shitchell/claude-dashboard/internal/session"
)

// ListLayout constants define layout-specific settings.
const (
	// ListHeaderHeight is the number of lines used by the header row.
	ListHeaderHeight = 1

	// ListMinHeight is the minimum height for the list view.
	ListMinHeight = 3

	// ListItemHeight is the height of each list item (1 line).
	ListItemHeight = 1

	// ListSelectionPrefix is the prefix shown for selected items.
	ListSelectionPrefix = "> "

	// ListNormalPrefix is the prefix shown for non-selected items.
	ListNormalPrefix = "  "

	// ListPrefixWidth is the width of the selection prefix.
	ListPrefixWidth = 2
)

// ListLayout displays sessions in a vertical list format.
// Each session occupies one row, with columns showing different fields.
type ListLayout struct {
	// columns holds the Column implementations for rendering.
	columns []Column

	// widths holds the calculated width for each column.
	// These are recalculated when the terminal width changes.
	widths []int

	// scrollOffset tracks the first visible row for scrolling.
	scrollOffset int

	// lastWidth tracks the last known terminal width.
	// Used to detect when widths need recalculation.
	lastWidth int

	// keys defines the key bindings for navigation.
	keys KeyMap
}

// NewListLayout creates a new ListLayout with the specified columns.
// If columns is nil or empty, default columns are used.
func NewListLayout(columns []Column, keys KeyMap) *ListLayout {
	if len(columns) == 0 {
		// Use default columns: Status, Name, Preview, Modified
		columns = GetColumns([]config.ColumnType{
			config.ColumnTypeStatus,
			config.ColumnTypeName,
			config.ColumnTypePreview,
			config.ColumnTypeModified,
		})
	}

	return &ListLayout{
		columns:      columns,
		widths:       nil,
		scrollOffset: 0,
		lastWidth:    0,
		keys:         keys,
	}
}

// NewListLayoutFromConfig creates a ListLayout using configuration.
func NewListLayoutFromConfig(cfg *config.Config, keys KeyMap) *ListLayout {
	var columns []Column

	if cfg != nil && len(cfg.Columns) > 0 {
		columns = GetColumns(cfg.Columns)
	}

	return NewListLayout(columns, keys)
}

// Name returns the human-readable name of this layout.
func (l *ListLayout) Name() string {
	return "List"
}

// Render renders the sessions to a string for display.
func (l *ListLayout) Render(sessions []*session.Session, cursor int, width, height int, styles *Styles, cfg *config.Config) string {
	// Handle empty session list
	if len(sessions) == 0 {
		return l.renderEmpty(styles)
	}

	// Recalculate column widths if terminal width changed
	contentWidth := width - ListPrefixWidth
	if contentWidth < 0 {
		contentWidth = 0
	}
	if l.lastWidth != contentWidth || len(l.widths) != len(l.columns) {
		l.widths = CalculateWidths(l.columns, contentWidth)
		l.lastWidth = contentWidth
	}

	// Calculate visible range
	visibleHeight := l.visibleRowCount(height)
	l.updateScrollOffset(cursor, visibleHeight, len(sessions))

	// Build the output
	var lines []string

	// Render header row
	header := l.renderHeader(styles)
	lines = append(lines, header)

	// Render visible session rows
	endIdx := l.scrollOffset + visibleHeight
	if endIdx > len(sessions) {
		endIdx = len(sessions)
	}

	for i := l.scrollOffset; i < endIdx; i++ {
		sess := sessions[i]
		isSelected := (i == cursor)
		row := l.renderRow(sess, isSelected, styles)
		lines = append(lines, row)
	}

	// Add scroll indicator if needed
	if l.needsScrollIndicator(len(sessions), visibleHeight) {
		indicator := l.renderScrollIndicator(cursor, len(sessions), visibleHeight, styles)
		lines = append(lines, indicator)
	}

	return strings.Join(lines, "\n")
}

// HandleKey processes a key message for navigation within the list.
// Returns the new cursor position and whether the key was handled.
func (l *ListLayout) HandleKey(msg tea.KeyMsg, sessionCount int, cursor int) (newCursor int, handled bool) {
	if sessionCount == 0 {
		return 0, false
	}

	switch {
	// Move up
	case key.Matches(msg, l.keys.Up):
		if cursor > 0 {
			return cursor - 1, true
		}
		return cursor, true

	// Move down
	case key.Matches(msg, l.keys.Down):
		if cursor < sessionCount-1 {
			return cursor + 1, true
		}
		return cursor, true

	// Page up
	case key.Matches(msg, l.keys.PageUp):
		newCursor = cursor - l.pageSize()
		if newCursor < 0 {
			newCursor = 0
		}
		return newCursor, true

	// Page down
	case key.Matches(msg, l.keys.PageDown):
		newCursor = cursor + l.pageSize()
		if newCursor >= sessionCount {
			newCursor = sessionCount - 1
		}
		return newCursor, true

	// Go to first
	case key.Matches(msg, l.keys.Home):
		return 0, true

	// Go to last
	case key.Matches(msg, l.keys.End):
		return sessionCount - 1, true
	}

	return cursor, false
}

// visibleRowCount returns the number of rows that can be displayed.
// This accounts for header and leaves room for scroll indicator if needed.
func (l *ListLayout) visibleRowCount(totalHeight int) int {
	// Subtract header height
	available := totalHeight - ListHeaderHeight

	// Ensure minimum visibility
	if available < 1 {
		return 1
	}

	return available
}

// updateScrollOffset adjusts the scroll position to keep the cursor visible.
func (l *ListLayout) updateScrollOffset(cursor, visibleCount, totalCount int) {
	// If cursor is above the visible area, scroll up
	if cursor < l.scrollOffset {
		l.scrollOffset = cursor
	}

	// If cursor is below the visible area, scroll down
	if cursor >= l.scrollOffset+visibleCount {
		l.scrollOffset = cursor - visibleCount + 1
	}

	// Clamp scroll offset to valid range
	maxOffset := totalCount - visibleCount
	if maxOffset < 0 {
		maxOffset = 0
	}
	if l.scrollOffset > maxOffset {
		l.scrollOffset = maxOffset
	}
	if l.scrollOffset < 0 {
		l.scrollOffset = 0
	}
}

// pageSize returns the number of items to move for page up/down.
// Uses a default if visible row count is not yet known.
const defaultPageSize = 10

func (l *ListLayout) pageSize() int {
	// Could be enhanced to use actual visible row count
	return defaultPageSize
}

// renderEmpty renders the empty state message.
func (l *ListLayout) renderEmpty(styles *Styles) string {
	msg := "  No sessions to display"
	return styles.Dim.Render(msg)
}

// renderHeader renders the column header row.
func (l *ListLayout) renderHeader(styles *Styles) string {
	// Add prefix spacing to align with content
	prefix := strings.Repeat(" ", ListPrefixWidth)

	// Render the header using the column system
	header := RenderHeader(l.columns, l.widths, styles)

	return prefix + header
}

// renderRow renders a single session row.
func (l *ListLayout) renderRow(sess *session.Session, selected bool, styles *Styles) string {
	// Build the row content using the column system
	content := RenderRow(sess, l.columns, l.widths, styles)

	// Add selection prefix and styling
	if selected {
		prefix := styles.ItemSelected.Render(ListSelectionPrefix)
		// Apply selection background to the entire row content
		styledContent := styles.ItemSelected.Render(content)
		return prefix + styledContent
	}

	prefix := ListNormalPrefix
	return prefix + content
}

// needsScrollIndicator returns true if a scroll indicator should be shown.
func (l *ListLayout) needsScrollIndicator(totalCount, visibleCount int) bool {
	return totalCount > visibleCount
}

// renderScrollIndicator renders the scroll position indicator.
func (l *ListLayout) renderScrollIndicator(cursor, totalCount, visibleCount int, styles *Styles) string {
	// Calculate scroll position as percentage
	position := cursor + 1
	indicator := strings.Builder{}

	// Add prefix spacing
	indicator.WriteString(strings.Repeat(" ", ListPrefixWidth))

	// Show position and total
	posStr := strings.Builder{}
	posStr.WriteString("[")

	// Show scroll bar visualization
	barWidth := 10
	scrollPos := 0
	if totalCount > 1 {
		scrollPos = (cursor * (barWidth - 1)) / (totalCount - 1)
	}

	for i := 0; i < barWidth; i++ {
		if i == scrollPos {
			posStr.WriteString("#")
		} else {
			posStr.WriteString("-")
		}
	}

	posStr.WriteString("] ")

	// Add position text
	posStr.WriteString(formatPosition(position, totalCount))

	indicator.WriteString(styles.Dim.Render(posStr.String()))

	return indicator.String()
}

// formatPosition formats the position indicator (e.g., "5/20").
func formatPosition(position, total int) string {
	return itoa(position) + "/" + itoa(total)
}

// itoa converts an integer to a string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}

	negative := false
	if n < 0 {
		negative = true
		n = -n
	}

	// Build digits in reverse
	digits := make([]byte, 0, 20)
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}

	// Reverse and add sign
	result := make([]byte, 0, len(digits)+1)
	if negative {
		result = append(result, '-')
	}
	for i := len(digits) - 1; i >= 0; i-- {
		result = append(result, digits[i])
	}

	return string(result)
}

// SetColumns updates the columns used for rendering.
// This forces a width recalculation on the next render.
func (l *ListLayout) SetColumns(columns []Column) {
	l.columns = columns
	l.widths = nil
	l.lastWidth = 0
}

// GetColumns returns the current columns.
func (l *ListLayout) GetColumns() []Column {
	return l.columns
}

// GetScrollOffset returns the current scroll offset.
func (l *ListLayout) GetScrollOffset() int {
	return l.scrollOffset
}

// ResetScroll resets the scroll position to the top.
func (l *ListLayout) ResetScroll() {
	l.scrollOffset = 0
}

// Verify interface implementation at compile time.
var _ Layout = (*ListLayout)(nil)
