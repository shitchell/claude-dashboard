// Package ui provides the terminal user interface for claude-dashboard.
// This file defines the column system for the list view, including
// the Column interface, built-in column implementations, and width calculation.
package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mattn/go-runewidth"
	"github.com/shitchell/claude-dashboard/internal/config"
	"github.com/shitchell/claude-dashboard/internal/session"
	"github.com/shitchell/claude-dashboard/internal/util"
)

// Column width constants define minimum widths and flex weights.
const (
	// MinWidthStatus is the minimum width for the status column.
	// Status shows a single indicator character.
	MinWidthStatus = 1

	// MinWidthName is the minimum width for the name column.
	MinWidthName = 10

	// MinWidthPreview is the minimum width for the preview column.
	MinWidthPreview = 15

	// MinWidthModified is the minimum width for the modified column.
	// Shows relative time like "5m ago".
	MinWidthModified = 8

	// MinWidthTurns is the minimum width for the turns column.
	MinWidthTurns = 5

	// MinWidthMessages is the minimum width for the messages column.
	MinWidthMessages = 5

	// MinWidthModel is the minimum width for the model column.
	MinWidthModel = 10

	// MinWidthProject is the minimum width for the project column.
	MinWidthProject = 15

	// FlexWeightFixed is used for fixed-width columns that don't expand.
	FlexWeightFixed = 0

	// FlexWeightLow is for columns that expand minimally.
	FlexWeightLow = 1

	// FlexWeightMedium is for columns that expand moderately.
	FlexWeightMedium = 2

	// FlexWeightHigh is for columns that expand significantly.
	FlexWeightHigh = 3

	// ColumnSeparator is the string placed between columns.
	ColumnSeparator = " "

	// ColumnSeparatorWidth is the visual width of the separator.
	ColumnSeparatorWidth = 1
)

// Column defines the interface for a displayable column in the list view.
// Each column knows how to render itself for a given session and width.
type Column interface {
	// ID returns the column type identifier.
	ID() config.ColumnType

	// Header returns the column header text.
	Header() string

	// Render renders the column content for a session at the given width.
	// The styles parameter provides styling for the rendered content.
	Render(session *session.Session, width int, styles *Styles) string

	// MinWidth returns the minimum width required for this column.
	MinWidth() int

	// FlexWeight returns the weight for proportional sizing.
	// Columns with weight 0 are fixed-width.
	// Columns with weight > 0 share extra space proportionally.
	FlexWeight() int
}

// StatusColumn displays the session status indicator.
type StatusColumn struct{}

// ID returns the column type identifier for StatusColumn.
func (c *StatusColumn) ID() config.ColumnType {
	return config.ColumnTypeStatus
}

// Header returns the column header text for StatusColumn.
func (c *StatusColumn) Header() string {
	return " "
}

// Render renders the status indicator for a session.
func (c *StatusColumn) Render(sess *session.Session, width int, styles *Styles) string {
	if sess == nil {
		return padOrTruncate("", width)
	}

	indicator := StatusIndicator(sess.Status)
	style := styles.StatusStyle(sess.Status)

	// Render the styled indicator
	rendered := style.Render(indicator)

	// For width calculation, we use the visual width of the indicator
	indicatorWidth := runewidth.StringWidth(indicator)

	// If width is larger than the indicator, we need to pad
	if indicatorWidth < width {
		// Since the indicator is styled, we pad after rendering
		// We add plain spaces (the style only applies to the indicator)
		padding := strings.Repeat(" ", width-indicatorWidth)
		return rendered + padding
	}

	return rendered
}

// MinWidth returns the minimum width for the status column.
func (c *StatusColumn) MinWidth() int {
	return MinWidthStatus
}

// FlexWeight returns the flex weight for the status column (fixed width).
func (c *StatusColumn) FlexWeight() int {
	return FlexWeightFixed
}

// NameColumn displays the session/project name.
type NameColumn struct{}

// ID returns the column type identifier for NameColumn.
func (c *NameColumn) ID() config.ColumnType {
	return config.ColumnTypeName
}

// Header returns the column header text for NameColumn.
func (c *NameColumn) Header() string {
	return "Name"
}

// Render renders the session name.
func (c *NameColumn) Render(sess *session.Session, width int, styles *Styles) string {
	if sess == nil {
		return padOrTruncate("", width)
	}

	name := sess.ProjectName
	if name == "" {
		name = sess.ID
	}

	// Truncate or pad to fit width
	result := padOrTruncate(name, width)

	return styles.Name.Render(result)
}

// MinWidth returns the minimum width for the name column.
func (c *NameColumn) MinWidth() int {
	return MinWidthName
}

// FlexWeight returns the flex weight for the name column.
func (c *NameColumn) FlexWeight() int {
	return FlexWeightMedium
}

// PreviewColumn displays a preview of the last message.
type PreviewColumn struct{}

// ID returns the column type identifier for PreviewColumn.
func (c *PreviewColumn) ID() config.ColumnType {
	return config.ColumnTypePreview
}

// Header returns the column header text for PreviewColumn.
func (c *PreviewColumn) Header() string {
	return "Preview"
}

// Render renders the message preview for a session.
func (c *PreviewColumn) Render(sess *session.Session, width int, styles *Styles) string {
	if sess == nil {
		return padOrTruncate("", width)
	}

	preview := sess.Preview
	if preview == "" && sess.Summary != "" {
		preview = sess.Summary
	}

	// Clean up the preview text - remove newlines
	preview = strings.ReplaceAll(preview, "\n", " ")
	preview = strings.ReplaceAll(preview, "\r", " ")

	// Truncate or pad to fit width
	result := padOrTruncate(preview, width)

	return styles.Preview.Render(result)
}

// MinWidth returns the minimum width for the preview column.
func (c *PreviewColumn) MinWidth() int {
	return MinWidthPreview
}

// FlexWeight returns the flex weight for the preview column (expands most).
func (c *PreviewColumn) FlexWeight() int {
	return FlexWeightHigh
}

// ModifiedColumn displays the modification time.
type ModifiedColumn struct{}

// ID returns the column type identifier for ModifiedColumn.
func (c *ModifiedColumn) ID() config.ColumnType {
	return config.ColumnTypeModified
}

// Header returns the column header text for ModifiedColumn.
func (c *ModifiedColumn) Header() string {
	return "Modified"
}

// Render renders the relative modification time for a session.
func (c *ModifiedColumn) Render(sess *session.Session, width int, styles *Styles) string {
	if sess == nil {
		return padOrTruncate("", width)
	}

	// Use the util.RelativeTime function for formatting
	timeStr := util.RelativeTime(sess.ModTime)

	// Truncate or pad to fit width
	result := padOrTruncate(timeStr, width)

	return styles.Time.Render(result)
}

// MinWidth returns the minimum width for the modified column.
func (c *ModifiedColumn) MinWidth() int {
	return MinWidthModified
}

// FlexWeight returns the flex weight for the modified column (fixed width).
func (c *ModifiedColumn) FlexWeight() int {
	return FlexWeightLow
}

// TurnsColumn displays the turn count.
type TurnsColumn struct{}

// ID returns the column type identifier for TurnsColumn.
func (c *TurnsColumn) ID() config.ColumnType {
	return config.ColumnTypeTurns
}

// Header returns the column header text for TurnsColumn.
func (c *TurnsColumn) Header() string {
	return "Turns"
}

// Render renders the turn count for a session.
func (c *TurnsColumn) Render(sess *session.Session, width int, styles *Styles) string {
	if sess == nil {
		return padOrTruncate("", width)
	}

	turnStr := strconv.Itoa(sess.TurnCount)

	// Right-align numeric values
	result := padLeft(turnStr, width)

	return styles.Dim.Render(result)
}

// MinWidth returns the minimum width for the turns column.
func (c *TurnsColumn) MinWidth() int {
	return MinWidthTurns
}

// FlexWeight returns the flex weight for the turns column (fixed width).
func (c *TurnsColumn) FlexWeight() int {
	return FlexWeightFixed
}

// MessagesColumn displays the message count.
type MessagesColumn struct{}

// ID returns the column type identifier for MessagesColumn.
func (c *MessagesColumn) ID() config.ColumnType {
	return config.ColumnTypeMessages
}

// Header returns the column header text for MessagesColumn.
func (c *MessagesColumn) Header() string {
	return "Msgs"
}

// Render renders the message count for a session.
func (c *MessagesColumn) Render(sess *session.Session, width int, styles *Styles) string {
	if sess == nil {
		return padOrTruncate("", width)
	}

	msgStr := strconv.Itoa(sess.MessageCount)

	// Right-align numeric values
	result := padLeft(msgStr, width)

	return styles.Dim.Render(result)
}

// MinWidth returns the minimum width for the messages column.
func (c *MessagesColumn) MinWidth() int {
	return MinWidthMessages
}

// FlexWeight returns the flex weight for the messages column (fixed width).
func (c *MessagesColumn) FlexWeight() int {
	return FlexWeightFixed
}

// ModelColumn displays the Claude model used.
type ModelColumn struct{}

// ID returns the column type identifier for ModelColumn.
func (c *ModelColumn) ID() config.ColumnType {
	return config.ColumnTypeModel
}

// Header returns the column header text for ModelColumn.
func (c *ModelColumn) Header() string {
	return "Model"
}

// Render renders the model name for a session.
func (c *ModelColumn) Render(sess *session.Session, width int, styles *Styles) string {
	if sess == nil {
		return padOrTruncate("", width)
	}

	model := sess.Model
	if model == "" {
		model = "-"
	}

	// Truncate or pad to fit width
	result := padOrTruncate(model, width)

	return styles.Dim.Render(result)
}

// MinWidth returns the minimum width for the model column.
func (c *ModelColumn) MinWidth() int {
	return MinWidthModel
}

// FlexWeight returns the flex weight for the model column.
func (c *ModelColumn) FlexWeight() int {
	return FlexWeightLow
}

// ProjectColumn displays the project path.
type ProjectColumn struct{}

// ID returns the column type identifier for ProjectColumn.
func (c *ProjectColumn) ID() config.ColumnType {
	return config.ColumnTypeProject
}

// Header returns the column header text for ProjectColumn.
func (c *ProjectColumn) Header() string {
	return "Project"
}

// Render renders the project path for a session.
func (c *ProjectColumn) Render(sess *session.Session, width int, styles *Styles) string {
	if sess == nil {
		return padOrTruncate("", width)
	}

	project := sess.ProjectPath
	if project == "" {
		project = sess.CWD
	}

	// Truncate or pad to fit width
	result := padOrTruncate(project, width)

	return styles.Dim.Render(result)
}

// MinWidth returns the minimum width for the project column.
func (c *ProjectColumn) MinWidth() int {
	return MinWidthProject
}

// FlexWeight returns the flex weight for the project column.
func (c *ProjectColumn) FlexWeight() int {
	return FlexWeightMedium
}

// DefaultColumns is a registry of all built-in column implementations.
// Use GetColumn to retrieve a column by type.
var DefaultColumns = map[config.ColumnType]Column{
	config.ColumnTypeStatus:   &StatusColumn{},
	config.ColumnTypeName:     &NameColumn{},
	config.ColumnTypePreview:  &PreviewColumn{},
	config.ColumnTypeModified: &ModifiedColumn{},
	config.ColumnTypeTurns:    &TurnsColumn{},
	config.ColumnTypeMessages: &MessagesColumn{},
	config.ColumnTypeModel:    &ModelColumn{},
	config.ColumnTypeProject:  &ProjectColumn{},
}

// GetColumn returns the Column implementation for a given type.
// Returns nil if the column type is not registered.
func GetColumn(colType config.ColumnType) Column {
	return DefaultColumns[colType]
}

// GetColumns returns a slice of Column implementations for the given types.
// Unknown column types are skipped.
func GetColumns(types []config.ColumnType) []Column {
	columns := make([]Column, 0, len(types))
	for _, colType := range types {
		if col := GetColumn(colType); col != nil {
			columns = append(columns, col)
		}
	}
	return columns
}

// CalculateWidths calculates the width for each column given total available width.
// Fixed columns (FlexWeight=0) get their MinWidth.
// Flex columns share remaining space proportionally based on their weights.
// Returns a slice of widths corresponding to each column.
func CalculateWidths(columns []Column, totalWidth int) []int {
	if len(columns) == 0 {
		return nil
	}

	widths := make([]int, len(columns))

	// Account for separators between columns
	separatorSpace := ColumnSeparatorWidth * (len(columns) - 1)
	availableWidth := totalWidth - separatorSpace
	if availableWidth < 0 {
		availableWidth = 0
	}

	// First pass: allocate minimum widths and calculate flex total
	totalMinWidth := 0
	totalFlexWeight := 0

	for i, col := range columns {
		minWidth := col.MinWidth()
		widths[i] = minWidth
		totalMinWidth += minWidth
		totalFlexWeight += col.FlexWeight()
	}

	// Calculate extra space to distribute
	extraSpace := availableWidth - totalMinWidth
	if extraSpace < 0 {
		// Not enough space - scale down proportionally
		return scaleDownWidths(columns, availableWidth)
	}

	// If no flex columns, distribute evenly to last column or return as-is
	if totalFlexWeight == 0 {
		// No flex columns, just return minimum widths
		// Any extra space is unused
		return widths
	}

	// Second pass: distribute extra space according to flex weights
	spacePerWeight := float64(extraSpace) / float64(totalFlexWeight)
	distributed := 0

	for i, col := range columns {
		flexWeight := col.FlexWeight()
		if flexWeight > 0 {
			extra := int(float64(flexWeight) * spacePerWeight)
			widths[i] += extra
			distributed += extra
		}
	}

	// Handle rounding error by adding remainder to first flex column
	remainder := extraSpace - distributed
	if remainder > 0 {
		for i, col := range columns {
			if col.FlexWeight() > 0 {
				widths[i] += remainder
				break
			}
		}
	}

	return widths
}

// scaleDownWidths scales column widths down when total width is insufficient.
// It attempts to preserve minimum widths for fixed columns first.
func scaleDownWidths(columns []Column, availableWidth int) []int {
	widths := make([]int, len(columns))

	if availableWidth <= 0 || len(columns) == 0 {
		return widths
	}

	// Calculate total minimum width
	totalMinWidth := 0
	for _, col := range columns {
		totalMinWidth += col.MinWidth()
	}

	if totalMinWidth == 0 {
		// All columns have 0 min width, distribute evenly
		perColumn := availableWidth / len(columns)
		for i := range columns {
			widths[i] = perColumn
		}
		return widths
	}

	// Scale proportionally
	scale := float64(availableWidth) / float64(totalMinWidth)
	distributed := 0

	for i, col := range columns {
		width := int(float64(col.MinWidth()) * scale)
		if width < 1 {
			width = 1
		}
		widths[i] = width
		distributed += width
	}

	// Adjust for rounding errors
	diff := availableWidth - distributed
	if diff != 0 && len(columns) > 0 {
		// Add/subtract from the last column
		widths[len(columns)-1] += diff
		if widths[len(columns)-1] < 1 {
			widths[len(columns)-1] = 1
		}
	}

	return widths
}

// padOrTruncate adjusts a string to exactly the specified width.
// Uses runewidth for accurate Unicode/emoji width handling.
// If the string is longer, it's truncated with an ellipsis.
// If shorter, it's padded with spaces on the right.
func padOrTruncate(s string, width int) string {
	if width <= 0 {
		return ""
	}

	currentWidth := runewidth.StringWidth(s)

	if currentWidth == width {
		return s
	}

	if currentWidth < width {
		// Pad with spaces
		return s + strings.Repeat(" ", width-currentWidth)
	}

	// Truncate with ellipsis
	return truncateWithEllipsis(s, width)
}

// truncateWithEllipsis truncates a string to fit within width, adding an ellipsis.
// Uses runewidth for accurate Unicode width handling.
func truncateWithEllipsis(s string, width int) string {
	if width <= 0 {
		return ""
	}

	const ellipsis = "..."
	ellipsisWidth := runewidth.StringWidth(ellipsis)

	// If width is too small for ellipsis, just truncate
	if width <= ellipsisWidth {
		return runewidth.Truncate(s, width, "")
	}

	// Truncate to make room for ellipsis
	truncated := runewidth.Truncate(s, width-ellipsisWidth, "")
	return truncated + ellipsis
}

// padLeft right-aligns a string within the given width.
// Uses runewidth for accurate Unicode width handling.
func padLeft(s string, width int) string {
	if width <= 0 {
		return ""
	}

	currentWidth := runewidth.StringWidth(s)

	if currentWidth >= width {
		return runewidth.Truncate(s, width, "")
	}

	return strings.Repeat(" ", width-currentWidth) + s
}

// RenderRow renders a complete row for a session using the given columns and widths.
// The styles parameter provides styling for each column.
func RenderRow(sess *session.Session, columns []Column, widths []int, styles *Styles) string {
	if len(columns) == 0 || len(widths) == 0 {
		return ""
	}

	// Ensure widths array matches columns
	if len(widths) < len(columns) {
		// Extend widths with minimum widths
		extended := make([]int, len(columns))
		copy(extended, widths)
		for i := len(widths); i < len(columns); i++ {
			extended[i] = columns[i].MinWidth()
		}
		widths = extended
	}

	parts := make([]string, len(columns))
	for i, col := range columns {
		parts[i] = col.Render(sess, widths[i], styles)
	}

	return strings.Join(parts, ColumnSeparator)
}

// RenderHeader renders the header row for the given columns and widths.
func RenderHeader(columns []Column, widths []int, styles *Styles) string {
	if len(columns) == 0 || len(widths) == 0 {
		return ""
	}

	// Ensure widths array matches columns
	if len(widths) < len(columns) {
		extended := make([]int, len(columns))
		copy(extended, widths)
		for i := len(widths); i < len(columns); i++ {
			extended[i] = columns[i].MinWidth()
		}
		widths = extended
	}

	parts := make([]string, len(columns))
	headerStyle := styles.Name.Bold(true)

	for i, col := range columns {
		header := col.Header()
		padded := padOrTruncate(header, widths[i])
		parts[i] = headerStyle.Render(padded)
	}

	return strings.Join(parts, ColumnSeparator)
}

// TotalMinWidth calculates the total minimum width required for all columns.
// This includes space for separators between columns.
func TotalMinWidth(columns []Column) int {
	if len(columns) == 0 {
		return 0
	}

	total := 0
	for _, col := range columns {
		total += col.MinWidth()
	}

	// Add separator space
	total += ColumnSeparatorWidth * (len(columns) - 1)

	return total
}

// Verify interface implementation at compile time.
var (
	_ Column = (*StatusColumn)(nil)
	_ Column = (*NameColumn)(nil)
	_ Column = (*PreviewColumn)(nil)
	_ Column = (*ModifiedColumn)(nil)
	_ Column = (*TurnsColumn)(nil)
	_ Column = (*MessagesColumn)(nil)
	_ Column = (*ModelColumn)(nil)
	_ Column = (*ProjectColumn)(nil)
)

// String returns a string representation of a Column for debugging.
func ColumnString(col Column) string {
	if col == nil {
		return "<nil>"
	}
	return fmt.Sprintf("Column{ID: %s, Header: %q, MinWidth: %d, FlexWeight: %d}",
		col.ID(), col.Header(), col.MinWidth(), col.FlexWeight())
}
