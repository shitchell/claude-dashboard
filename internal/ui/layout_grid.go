// Package ui provides the terminal user interface for claude-dashboard.
// This file implements the GridLayout for displaying sessions in a grid/card format.
package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/shitchell/claude-dashboard/internal/config"
	"github.com/shitchell/claude-dashboard/internal/session"
	"github.com/shitchell/claude-dashboard/internal/util"
)

// Grid layout constants define cell sizing and spacing.
const (
	// GridMinCellWidth is the minimum width for a grid cell.
	GridMinCellWidth = 30

	// GridMaxCellWidth is the maximum width for a grid cell.
	GridMaxCellWidth = 50

	// GridPreferredCellWidth is the preferred cell width when space allows.
	GridPreferredCellWidth = 35

	// GridCellHeight is the fixed height of each grid cell (in lines).
	// Line 1: Status + Name
	// Line 2: Preview
	// Line 3: Time (right-aligned)
	GridCellHeight = 3

	// GridCellPaddingH is the horizontal padding inside each cell.
	GridCellPaddingH = 1

	// GridCellPaddingV is the vertical padding inside each cell.
	GridCellPaddingV = 0

	// GridCellGap is the space between cells.
	GridCellGap = 1

	// GridBorderWidth is the width taken by cell borders (left + right).
	GridBorderWidth = 2

	// GridMinColumns is the minimum number of columns in the grid.
	GridMinColumns = 1

	// GridScrollIndicatorHeight is the height of the scroll indicator row.
	GridScrollIndicatorHeight = 1
)

// GridLayout displays sessions in a grid/card format.
// Sessions are arranged in rows and columns, with each cell showing
// status, name, preview, and time.
type GridLayout struct {
	// scrollOffset tracks the first visible row for scrolling.
	scrollOffset int

	// lastWidth tracks the last known terminal width.
	// Used to detect when dimensions need recalculation.
	lastWidth int

	// cachedColumns stores the calculated column count.
	cachedColumns int

	// cachedCellWidth stores the calculated cell width.
	cachedCellWidth int

	// keys defines the key bindings for navigation.
	keys KeyMap
}

// NewGridLayout creates a new GridLayout with the specified key bindings.
func NewGridLayout(keys KeyMap) *GridLayout {
	return &GridLayout{
		scrollOffset:    0,
		lastWidth:       0,
		cachedColumns:   0,
		cachedCellWidth: 0,
		keys:            keys,
	}
}

// Name returns the human-readable name of this layout.
func (g *GridLayout) Name() string {
	return "Grid"
}

// Render renders the sessions to a string for display.
func (g *GridLayout) Render(sessions []*session.Session, cursor int, width, height int, styles *Styles, cfg *config.Config) string {
	// Handle empty session list
	if len(sessions) == 0 {
		return g.renderEmpty(styles)
	}

	// Recalculate grid dimensions if width changed
	if g.lastWidth != width {
		g.cachedColumns, g.cachedCellWidth = g.calculateGridDimensions(width)
		g.lastWidth = width
	}

	cols := g.cachedColumns
	cellWidth := g.cachedCellWidth

	// Calculate number of rows needed
	totalRows := (len(sessions) + cols - 1) / cols

	// Calculate visible rows based on available height
	visibleRows := g.visibleRowCount(height)
	if visibleRows < 1 {
		visibleRows = 1
	}

	// Update scroll offset based on cursor position
	cursorRow := cursor / cols
	g.updateScrollOffset(cursorRow, visibleRows, totalRows)

	// Build the output
	var lines []string

	// Render visible rows
	startRow := g.scrollOffset
	endRow := startRow + visibleRows
	if endRow > totalRows {
		endRow = totalRows
	}

	for row := startRow; row < endRow; row++ {
		rowLines := g.renderRow(sessions, row, cols, cellWidth, cursor, styles)
		lines = append(lines, rowLines...)

		// Add gap between rows (except after last row)
		if row < endRow-1 {
			lines = append(lines, "")
		}
	}

	// Add scroll indicator if needed
	if g.needsScrollIndicator(totalRows, visibleRows) {
		indicator := g.renderScrollIndicator(cursorRow, totalRows, visibleRows, styles)
		lines = append(lines, indicator)
	}

	return strings.Join(lines, "\n")
}

// HandleKey processes a key message for navigation within the grid.
// Returns the new cursor position and whether the key was handled.
func (g *GridLayout) HandleKey(msg tea.KeyMsg, sessionCount int, cursor int) (newCursor int, handled bool) {
	if sessionCount == 0 {
		return 0, false
	}

	cols := g.cachedColumns
	if cols < 1 {
		cols = 1
	}

	// Calculate current row and column
	currentRow := cursor / cols
	currentCol := cursor % cols
	totalRows := (sessionCount + cols - 1) / cols

	switch {
	// Move up
	case key.Matches(msg, g.keys.Up):
		if currentRow > 0 {
			// Move to same column in previous row
			newPos := cursor - cols
			if newPos >= 0 {
				return newPos, true
			}
		}
		return cursor, true

	// Move down
	case key.Matches(msg, g.keys.Down):
		if currentRow < totalRows-1 {
			// Move to same column in next row
			newPos := cursor + cols
			// Clamp to last session if going beyond
			if newPos >= sessionCount {
				newPos = sessionCount - 1
			}
			return newPos, true
		}
		return cursor, true

	// Move left
	case key.Matches(msg, g.keys.Left):
		if currentCol > 0 {
			return cursor - 1, true
		}
		return cursor, true

	// Move right
	case key.Matches(msg, g.keys.Right):
		if currentCol < cols-1 && cursor < sessionCount-1 {
			return cursor + 1, true
		}
		return cursor, true

	// Page up
	case key.Matches(msg, g.keys.PageUp):
		// Move up by visible rows worth of items
		pageSize := g.pageSize(cols)
		newCursor = cursor - pageSize
		if newCursor < 0 {
			newCursor = 0
		}
		return newCursor, true

	// Page down
	case key.Matches(msg, g.keys.PageDown):
		// Move down by visible rows worth of items
		pageSize := g.pageSize(cols)
		newCursor = cursor + pageSize
		if newCursor >= sessionCount {
			newCursor = sessionCount - 1
		}
		return newCursor, true

	// Go to first
	case key.Matches(msg, g.keys.Home):
		return 0, true

	// Go to last
	case key.Matches(msg, g.keys.End):
		return sessionCount - 1, true
	}

	return cursor, false
}

// calculateGridDimensions calculates the optimal column count and cell width.
// Returns (columns, cellWidth).
func (g *GridLayout) calculateGridDimensions(width int) (cols int, cellWidth int) {
	// Account for total cell width including borders and gaps
	totalCellWidth := GridPreferredCellWidth + GridBorderWidth

	// Calculate how many columns can fit
	// First column needs no gap, each additional column needs a gap
	if width <= 0 {
		return GridMinColumns, GridMinCellWidth
	}

	// Try to fit preferred width columns
	// Available = width - (cols-1)*gap
	// cols * totalCellWidth <= Available
	// cols * totalCellWidth <= width - (cols-1)*gap
	// cols * totalCellWidth + (cols-1)*gap <= width
	// cols * (totalCellWidth + gap) - gap <= width
	// cols <= (width + gap) / (totalCellWidth + gap)
	cols = (width + GridCellGap) / (totalCellWidth + GridCellGap)

	if cols < GridMinColumns {
		cols = GridMinColumns
	}

	// Calculate actual cell width to use
	// Available width for cells after gaps
	gapSpace := (cols - 1) * GridCellGap
	availableForCells := width - gapSpace

	// Distribute evenly, accounting for borders
	cellWidth = (availableForCells / cols) - GridBorderWidth

	// Clamp cell width to min/max
	if cellWidth < GridMinCellWidth {
		cellWidth = GridMinCellWidth
	}
	if cellWidth > GridMaxCellWidth {
		cellWidth = GridMaxCellWidth
	}

	return cols, cellWidth
}

// renderRow renders a single row of grid cells.
// Returns the lines that make up this row.
func (g *GridLayout) renderRow(sessions []*session.Session, row, cols, cellWidth int, cursor int, styles *Styles) []string {
	// Collect cells for this row
	var cells []string

	startIdx := row * cols
	for col := 0; col < cols; col++ {
		idx := startIdx + col
		if idx < len(sessions) {
			isSelected := (idx == cursor)
			cell := g.renderCell(sessions[idx], isSelected, cellWidth, styles)
			cells = append(cells, cell)
		} else {
			// Empty cell placeholder
			cell := g.renderEmptyCell(cellWidth, styles)
			cells = append(cells, cell)
		}
	}

	// Join cells horizontally with gap
	return g.joinCellsHorizontally(cells)
}

// renderCell renders a single grid cell for a session.
func (g *GridLayout) renderCell(sess *session.Session, selected bool, width int, styles *Styles) string {
	// Calculate inner content width (accounting for padding and borders)
	innerWidth := width - (GridCellPaddingH * 2)
	if innerWidth < 1 {
		innerWidth = 1
	}

	// Build cell content lines
	var contentLines []string

	// Line 1: Status indicator + Name
	statusIndicator := StatusIndicator(sess.Status)
	statusStyle := styles.StatusStyle(sess.Status)
	styledIndicator := statusStyle.Render(statusIndicator)

	// Name gets remaining width after status indicator and space
	nameWidth := innerWidth - runewidth.StringWidth(statusIndicator) - 1
	if nameWidth < 1 {
		nameWidth = 1
	}
	name := sess.ProjectName
	if name == "" {
		name = sess.ID
	}
	truncatedName := padOrTruncate(name, nameWidth)

	// Apply name styling
	if selected {
		truncatedName = styles.Name.Bold(true).Render(truncatedName)
	} else {
		truncatedName = styles.Name.Render(truncatedName)
	}

	line1 := styledIndicator + " " + truncatedName
	contentLines = append(contentLines, line1)

	// Line 2: Preview (truncated to fit)
	preview := sess.Preview
	if preview == "" && sess.Summary != "" {
		preview = sess.Summary
	}
	// Clean up preview text
	preview = strings.ReplaceAll(preview, "\n", " ")
	preview = strings.ReplaceAll(preview, "\r", " ")
	truncatedPreview := padOrTruncate(preview, innerWidth)
	styledPreview := styles.Preview.Render(truncatedPreview)
	contentLines = append(contentLines, styledPreview)

	// Line 3: Time (right-aligned)
	timeStr := util.RelativeTime(sess.ModTime)
	paddedTime := padLeft(timeStr, innerWidth)
	styledTime := styles.Time.Render(paddedTime)
	contentLines = append(contentLines, styledTime)

	// Join content lines
	content := strings.Join(contentLines, "\n")

	// Apply cell styling with border
	var cellStyle lipgloss.Style
	if selected {
		cellStyle = styles.CellSelected.
			Width(width).
			Height(GridCellHeight)
	} else {
		cellStyle = styles.Cell.
			Width(width).
			Height(GridCellHeight)
	}

	return cellStyle.Render(content)
}

// renderEmptyCell renders a placeholder for empty grid positions.
func (g *GridLayout) renderEmptyCell(width int, styles *Styles) string {
	// Create empty content lines
	var contentLines []string
	innerWidth := width - (GridCellPaddingH * 2)
	if innerWidth < 1 {
		innerWidth = 1
	}

	emptyLine := strings.Repeat(" ", innerWidth)
	for i := 0; i < GridCellHeight; i++ {
		contentLines = append(contentLines, emptyLine)
	}

	content := strings.Join(contentLines, "\n")

	// Use transparent/minimal styling for empty cells
	emptyStyle := lipgloss.NewStyle().
		Width(width).
		Height(GridCellHeight)

	return emptyStyle.Render(content)
}

// joinCellsHorizontally joins rendered cells side by side.
// Each cell may have multiple lines, so this joins corresponding lines.
func (g *GridLayout) joinCellsHorizontally(cells []string) []string {
	if len(cells) == 0 {
		return nil
	}

	// Split each cell into lines
	cellLines := make([][]string, len(cells))
	maxLines := 0

	for i, cell := range cells {
		cellLines[i] = strings.Split(cell, "\n")
		if len(cellLines[i]) > maxLines {
			maxLines = len(cellLines[i])
		}
	}

	// Join corresponding lines from each cell
	result := make([]string, maxLines)
	gap := strings.Repeat(" ", GridCellGap)

	for lineIdx := 0; lineIdx < maxLines; lineIdx++ {
		var lineParts []string
		for cellIdx := range cells {
			if lineIdx < len(cellLines[cellIdx]) {
				lineParts = append(lineParts, cellLines[cellIdx][lineIdx])
			}
		}
		result[lineIdx] = strings.Join(lineParts, gap)
	}

	return result
}

// visibleRowCount returns the number of grid rows that can be displayed.
func (g *GridLayout) visibleRowCount(totalHeight int) int {
	// Each row takes GridCellHeight lines plus borders (2) plus gap (1)
	// Except the last row doesn't need a trailing gap
	rowHeight := GridCellHeight + GridBorderWidth + GridCellGap

	if totalHeight <= 0 {
		return 1
	}

	// Account for scroll indicator if present
	available := totalHeight - GridScrollIndicatorHeight
	if available <= 0 {
		available = totalHeight
	}

	// Calculate how many complete rows fit
	// First row: rowHeight - GridCellGap (no leading gap)
	// Each additional row: rowHeight
	firstRowHeight := rowHeight - GridCellGap
	if available < firstRowHeight {
		return 1
	}

	remaining := available - firstRowHeight
	additionalRows := remaining / rowHeight

	return 1 + additionalRows
}

// updateScrollOffset adjusts the scroll position to keep the cursor row visible.
func (g *GridLayout) updateScrollOffset(cursorRow, visibleRows, totalRows int) {
	// If cursor row is above the visible area, scroll up
	if cursorRow < g.scrollOffset {
		g.scrollOffset = cursorRow
	}

	// If cursor row is below the visible area, scroll down
	if cursorRow >= g.scrollOffset+visibleRows {
		g.scrollOffset = cursorRow - visibleRows + 1
	}

	// Clamp scroll offset to valid range
	maxOffset := totalRows - visibleRows
	if maxOffset < 0 {
		maxOffset = 0
	}
	if g.scrollOffset > maxOffset {
		g.scrollOffset = maxOffset
	}
	if g.scrollOffset < 0 {
		g.scrollOffset = 0
	}
}

// pageSize returns the number of items to move for page up/down.
const gridDefaultPageRows = 3

func (g *GridLayout) pageSize(cols int) int {
	// Move by multiple rows
	return gridDefaultPageRows * cols
}

// renderEmpty renders the empty state message.
func (g *GridLayout) renderEmpty(styles *Styles) string {
	msg := "  No sessions to display"
	return styles.Dim.Render(msg)
}

// needsScrollIndicator returns true if a scroll indicator should be shown.
func (g *GridLayout) needsScrollIndicator(totalRows, visibleRows int) bool {
	return totalRows > visibleRows
}

// renderScrollIndicator renders the scroll position indicator.
func (g *GridLayout) renderScrollIndicator(cursorRow, totalRows, visibleRows int, styles *Styles) string {
	indicator := strings.Builder{}

	// Show position and total rows
	posStr := strings.Builder{}
	posStr.WriteString("[")

	// Show scroll bar visualization
	barWidth := 10
	scrollPos := 0
	if totalRows > 1 {
		scrollPos = (cursorRow * (barWidth - 1)) / (totalRows - 1)
	}

	for i := 0; i < barWidth; i++ {
		if i == scrollPos {
			posStr.WriteString("#")
		} else {
			posStr.WriteString("-")
		}
	}

	posStr.WriteString("] Row ")
	posStr.WriteString(itoa(cursorRow + 1))
	posStr.WriteString("/")
	posStr.WriteString(itoa(totalRows))

	indicator.WriteString(styles.Dim.Render(posStr.String()))

	return indicator.String()
}

// GetScrollOffset returns the current scroll offset.
func (g *GridLayout) GetScrollOffset() int {
	return g.scrollOffset
}

// ResetScroll resets the scroll position to the top.
func (g *GridLayout) ResetScroll() {
	g.scrollOffset = 0
}

// GetCachedDimensions returns the cached grid dimensions.
// Returns (columns, cellWidth).
func (g *GridLayout) GetCachedDimensions() (int, int) {
	return g.cachedColumns, g.cachedCellWidth
}

// Verify interface implementation at compile time.
var _ Layout = (*GridLayout)(nil)
