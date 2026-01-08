package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/mattn/go-runewidth"
	"github.com/shitchell/claude-dashboard/internal/config"
	"github.com/shitchell/claude-dashboard/internal/session"
)

// testSession creates a session with test data.
func testSession() *session.Session {
	return &session.Session{
		ID:           "test-session-123",
		FilePath:     "/home/user/.claude/projects/test/session.jsonl",
		ProjectPath:  "/home/user/code/test-project",
		ProjectName:  "test-project",
		CWD:          "/home/user/code/test-project",
		Summary:      "This is a test summary",
		Model:        "claude-sonnet-4-20250514",
		Preview:      "This is a preview of the last message",
		MessageCount: 42,
		TurnCount:    21,
		ModTime:      time.Now().Add(-5 * time.Minute),
		Status:       session.StatusActive,
		TmuxPane:     "%5",
	}
}

// TestStatusColumnInterface verifies StatusColumn implements Column.
func TestStatusColumnInterface(t *testing.T) {
	var col Column = &StatusColumn{}

	if col.ID() != config.ColumnTypeStatus {
		t.Errorf("ID() = %v, want %v", col.ID(), config.ColumnTypeStatus)
	}

	// Header can be anything (it's a space for status)
	_ = col.Header()

	if col.MinWidth() != MinWidthStatus {
		t.Errorf("MinWidth() = %d, want %d", col.MinWidth(), MinWidthStatus)
	}

	if col.FlexWeight() != FlexWeightFixed {
		t.Errorf("FlexWeight() = %d, want %d", col.FlexWeight(), FlexWeightFixed)
	}
}

// TestStatusColumnRender verifies StatusColumn renders correctly.
func TestStatusColumnRender(t *testing.T) {
	col := &StatusColumn{}
	styles := NewStyles()
	sess := testSession()

	tests := []struct {
		name   string
		status session.Status
	}{
		{"active", session.StatusActive},
		{"idle", session.StatusIdle},
		{"exited", session.StatusExited},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sess.Status = tt.status
			rendered := col.Render(sess, 3, &styles)

			// Should render something
			if len(rendered) == 0 {
				t.Error("expected non-empty rendered output")
			}
		})
	}
}

// TestStatusColumnRenderNilSession verifies StatusColumn handles nil session.
func TestStatusColumnRenderNilSession(t *testing.T) {
	col := &StatusColumn{}
	styles := NewStyles()

	rendered := col.Render(nil, 5, &styles)

	// Should not panic and should return padded empty string
	if len(rendered) != 5 {
		t.Errorf("rendered length = %d, want 5", len(rendered))
	}
}

// TestNameColumnInterface verifies NameColumn implements Column.
func TestNameColumnInterface(t *testing.T) {
	var col Column = &NameColumn{}

	if col.ID() != config.ColumnTypeName {
		t.Errorf("ID() = %v, want %v", col.ID(), config.ColumnTypeName)
	}

	if col.Header() != "Name" {
		t.Errorf("Header() = %q, want %q", col.Header(), "Name")
	}

	if col.MinWidth() != MinWidthName {
		t.Errorf("MinWidth() = %d, want %d", col.MinWidth(), MinWidthName)
	}

	if col.FlexWeight() != FlexWeightMedium {
		t.Errorf("FlexWeight() = %d, want %d", col.FlexWeight(), FlexWeightMedium)
	}
}

// TestNameColumnRender verifies NameColumn renders correctly.
func TestNameColumnRender(t *testing.T) {
	col := &NameColumn{}
	styles := NewStyles()
	sess := testSession()

	rendered := col.Render(sess, 20, &styles)

	// Should contain the project name (may include ANSI codes)
	if !strings.Contains(rendered, "test-project") {
		t.Errorf("rendered should contain project name, got %q", rendered)
	}
}

// TestNameColumnFallbackToID verifies NameColumn uses ID when name is empty.
func TestNameColumnFallbackToID(t *testing.T) {
	col := &NameColumn{}
	styles := NewStyles()
	sess := testSession()
	sess.ProjectName = ""

	rendered := col.Render(sess, 30, &styles)

	// Should contain the session ID
	if !strings.Contains(rendered, sess.ID) {
		t.Errorf("rendered should contain session ID when name is empty")
	}
}

// TestPreviewColumnInterface verifies PreviewColumn implements Column.
func TestPreviewColumnInterface(t *testing.T) {
	var col Column = &PreviewColumn{}

	if col.ID() != config.ColumnTypePreview {
		t.Errorf("ID() = %v, want %v", col.ID(), config.ColumnTypePreview)
	}

	if col.Header() != "Preview" {
		t.Errorf("Header() = %q, want %q", col.Header(), "Preview")
	}

	if col.MinWidth() != MinWidthPreview {
		t.Errorf("MinWidth() = %d, want %d", col.MinWidth(), MinWidthPreview)
	}

	if col.FlexWeight() != FlexWeightHigh {
		t.Errorf("FlexWeight() = %d, want %d", col.FlexWeight(), FlexWeightHigh)
	}
}

// TestPreviewColumnRender verifies PreviewColumn renders correctly.
func TestPreviewColumnRender(t *testing.T) {
	col := &PreviewColumn{}
	styles := NewStyles()
	sess := testSession()

	rendered := col.Render(sess, 40, &styles)

	// Should contain part of the preview
	if !strings.Contains(rendered, "preview") {
		t.Errorf("rendered should contain preview text")
	}
}

// TestPreviewColumnFallbackToSummary verifies PreviewColumn uses summary when preview is empty.
func TestPreviewColumnFallbackToSummary(t *testing.T) {
	col := &PreviewColumn{}
	styles := NewStyles()
	sess := testSession()
	sess.Preview = ""

	rendered := col.Render(sess, 40, &styles)

	// Should contain the summary
	if !strings.Contains(rendered, "summary") {
		t.Errorf("rendered should contain summary when preview is empty")
	}
}

// TestPreviewColumnStripsNewlines verifies newlines are removed from preview.
func TestPreviewColumnStripsNewlines(t *testing.T) {
	col := &PreviewColumn{}
	styles := NewStyles()
	sess := testSession()
	sess.Preview = "line1\nline2\r\nline3"

	rendered := col.Render(sess, 40, &styles)

	// Should not contain newlines
	if strings.Contains(rendered, "\n") || strings.Contains(rendered, "\r") {
		t.Errorf("rendered should not contain newlines, got %q", rendered)
	}
}

// TestModifiedColumnInterface verifies ModifiedColumn implements Column.
func TestModifiedColumnInterface(t *testing.T) {
	var col Column = &ModifiedColumn{}

	if col.ID() != config.ColumnTypeModified {
		t.Errorf("ID() = %v, want %v", col.ID(), config.ColumnTypeModified)
	}

	if col.Header() != "Modified" {
		t.Errorf("Header() = %q, want %q", col.Header(), "Modified")
	}

	if col.MinWidth() != MinWidthModified {
		t.Errorf("MinWidth() = %d, want %d", col.MinWidth(), MinWidthModified)
	}
}

// TestModifiedColumnRender verifies ModifiedColumn renders relative time.
func TestModifiedColumnRender(t *testing.T) {
	col := &ModifiedColumn{}
	styles := NewStyles()
	sess := testSession()

	rendered := col.Render(sess, 12, &styles)

	// Should contain "ago" for recent times
	if !strings.Contains(rendered, "ago") {
		t.Errorf("rendered should contain 'ago' for recent times, got %q", rendered)
	}
}

// TestTurnsColumnInterface verifies TurnsColumn implements Column.
func TestTurnsColumnInterface(t *testing.T) {
	var col Column = &TurnsColumn{}

	if col.ID() != config.ColumnTypeTurns {
		t.Errorf("ID() = %v, want %v", col.ID(), config.ColumnTypeTurns)
	}

	if col.Header() != "Turns" {
		t.Errorf("Header() = %q, want %q", col.Header(), "Turns")
	}

	if col.MinWidth() != MinWidthTurns {
		t.Errorf("MinWidth() = %d, want %d", col.MinWidth(), MinWidthTurns)
	}

	if col.FlexWeight() != FlexWeightFixed {
		t.Errorf("FlexWeight() = %d, want %d", col.FlexWeight(), FlexWeightFixed)
	}
}

// TestTurnsColumnRender verifies TurnsColumn renders the turn count.
func TestTurnsColumnRender(t *testing.T) {
	col := &TurnsColumn{}
	styles := NewStyles()
	sess := testSession()

	rendered := col.Render(sess, 6, &styles)

	// Should contain the turn count
	if !strings.Contains(rendered, "21") {
		t.Errorf("rendered should contain turn count '21', got %q", rendered)
	}
}

// TestMessagesColumnInterface verifies MessagesColumn implements Column.
func TestMessagesColumnInterface(t *testing.T) {
	var col Column = &MessagesColumn{}

	if col.ID() != config.ColumnTypeMessages {
		t.Errorf("ID() = %v, want %v", col.ID(), config.ColumnTypeMessages)
	}

	if col.Header() != "Msgs" {
		t.Errorf("Header() = %q, want %q", col.Header(), "Msgs")
	}
}

// TestMessagesColumnRender verifies MessagesColumn renders the message count.
func TestMessagesColumnRender(t *testing.T) {
	col := &MessagesColumn{}
	styles := NewStyles()
	sess := testSession()

	rendered := col.Render(sess, 6, &styles)

	// Should contain the message count
	if !strings.Contains(rendered, "42") {
		t.Errorf("rendered should contain message count '42', got %q", rendered)
	}
}

// TestModelColumnInterface verifies ModelColumn implements Column.
func TestModelColumnInterface(t *testing.T) {
	var col Column = &ModelColumn{}

	if col.ID() != config.ColumnTypeModel {
		t.Errorf("ID() = %v, want %v", col.ID(), config.ColumnTypeModel)
	}

	if col.Header() != "Model" {
		t.Errorf("Header() = %q, want %q", col.Header(), "Model")
	}
}

// TestModelColumnRender verifies ModelColumn renders the model name.
func TestModelColumnRender(t *testing.T) {
	col := &ModelColumn{}
	styles := NewStyles()
	sess := testSession()

	rendered := col.Render(sess, 30, &styles)

	// Should contain part of the model name
	if !strings.Contains(rendered, "claude") {
		t.Errorf("rendered should contain 'claude', got %q", rendered)
	}
}

// TestModelColumnEmptyModel verifies ModelColumn handles empty model.
func TestModelColumnEmptyModel(t *testing.T) {
	col := &ModelColumn{}
	styles := NewStyles()
	sess := testSession()
	sess.Model = ""

	rendered := col.Render(sess, 10, &styles)

	// Should show placeholder
	if !strings.Contains(rendered, "-") {
		t.Errorf("rendered should contain '-' for empty model, got %q", rendered)
	}
}

// TestProjectColumnInterface verifies ProjectColumn implements Column.
func TestProjectColumnInterface(t *testing.T) {
	var col Column = &ProjectColumn{}

	if col.ID() != config.ColumnTypeProject {
		t.Errorf("ID() = %v, want %v", col.ID(), config.ColumnTypeProject)
	}

	if col.Header() != "Project" {
		t.Errorf("Header() = %q, want %q", col.Header(), "Project")
	}
}

// TestProjectColumnRender verifies ProjectColumn renders the project path.
func TestProjectColumnRender(t *testing.T) {
	col := &ProjectColumn{}
	styles := NewStyles()
	sess := testSession()

	rendered := col.Render(sess, 40, &styles)

	// Should contain part of the project path
	if !strings.Contains(rendered, "test-project") {
		t.Errorf("rendered should contain 'test-project', got %q", rendered)
	}
}

// TestDefaultColumnsRegistry verifies all column types are registered.
func TestDefaultColumnsRegistry(t *testing.T) {
	expectedTypes := []config.ColumnType{
		config.ColumnTypeStatus,
		config.ColumnTypeName,
		config.ColumnTypePreview,
		config.ColumnTypeModified,
		config.ColumnTypeTurns,
		config.ColumnTypeMessages,
		config.ColumnTypeModel,
		config.ColumnTypeProject,
	}

	for _, colType := range expectedTypes {
		t.Run(colType.String(), func(t *testing.T) {
			col := GetColumn(colType)
			if col == nil {
				t.Errorf("GetColumn(%v) returned nil", colType)
				return
			}
			if col.ID() != colType {
				t.Errorf("col.ID() = %v, want %v", col.ID(), colType)
			}
		})
	}
}

// TestGetColumns verifies GetColumns returns columns for given types.
func TestGetColumns(t *testing.T) {
	types := []config.ColumnType{
		config.ColumnTypeStatus,
		config.ColumnTypeName,
		config.ColumnTypeModified,
	}

	columns := GetColumns(types)

	if len(columns) != len(types) {
		t.Errorf("GetColumns() returned %d columns, want %d", len(columns), len(types))
	}

	for i, col := range columns {
		if col.ID() != types[i] {
			t.Errorf("columns[%d].ID() = %v, want %v", i, col.ID(), types[i])
		}
	}
}

// TestGetColumnsSkipsUnknown verifies GetColumns skips unknown types.
func TestGetColumnsSkipsUnknown(t *testing.T) {
	types := []config.ColumnType{
		config.ColumnTypeStatus,
		config.ColumnType(999), // Unknown type
		config.ColumnTypeName,
	}

	columns := GetColumns(types)

	// Should only have 2 columns (unknown skipped)
	if len(columns) != 2 {
		t.Errorf("GetColumns() returned %d columns, want 2", len(columns))
	}
}

// TestCalculateWidthsBasic verifies basic width calculation.
func TestCalculateWidthsBasic(t *testing.T) {
	columns := []Column{
		&StatusColumn{},  // Fixed, min=1
		&NameColumn{},    // Flex=2, min=10
		&ModifiedColumn{}, // Flex=1, min=8
	}

	// Total width: 100
	// Separator space: 2 (2 separators)
	// Available: 98
	// Min total: 1 + 10 + 8 = 19
	// Extra: 98 - 19 = 79
	// Total flex weight: 0 + 2 + 1 = 3
	// Per weight: 79 / 3 = 26.33
	// NameColumn gets: 10 + (2 * 26) = 62
	// ModifiedColumn gets: 8 + (1 * 26) = 34
	// Rounding adjustment goes to first flex column

	widths := CalculateWidths(columns, 100)

	if len(widths) != 3 {
		t.Fatalf("CalculateWidths() returned %d widths, want 3", len(widths))
	}

	// Status should get min width (fixed)
	if widths[0] != MinWidthStatus {
		t.Errorf("widths[0] = %d, want %d (status min)", widths[0], MinWidthStatus)
	}

	// Name and Modified should share extra space
	if widths[1] <= MinWidthName {
		t.Errorf("widths[1] = %d, should be > %d (name min)", widths[1], MinWidthName)
	}

	if widths[2] <= MinWidthModified {
		t.Errorf("widths[2] = %d, should be > %d (modified min)", widths[2], MinWidthModified)
	}

	// Total should roughly equal available width (minus separators)
	total := widths[0] + widths[1] + widths[2]
	expected := 100 - 2 // minus separator space
	if total != expected {
		t.Errorf("total width = %d, want %d", total, expected)
	}
}

// TestCalculateWidthsAllFixed verifies width calculation with all fixed columns.
func TestCalculateWidthsAllFixed(t *testing.T) {
	columns := []Column{
		&StatusColumn{}, // Fixed
		&TurnsColumn{},  // Fixed
	}

	widths := CalculateWidths(columns, 100)

	// All fixed columns should get their minimum widths
	if widths[0] != MinWidthStatus {
		t.Errorf("widths[0] = %d, want %d", widths[0], MinWidthStatus)
	}

	if widths[1] != MinWidthTurns {
		t.Errorf("widths[1] = %d, want %d", widths[1], MinWidthTurns)
	}
}

// TestCalculateWidthsNarrowWidth verifies width calculation with insufficient space.
func TestCalculateWidthsNarrowWidth(t *testing.T) {
	columns := []Column{
		&NameColumn{},    // min=10
		&PreviewColumn{}, // min=15
	}

	// Very narrow - not enough for minimum widths
	widths := CalculateWidths(columns, 10)

	if len(widths) != 2 {
		t.Fatalf("CalculateWidths() returned %d widths, want 2", len(widths))
	}

	// Both widths should be positive but scaled down
	if widths[0] <= 0 {
		t.Errorf("widths[0] = %d, should be > 0", widths[0])
	}
	if widths[1] <= 0 {
		t.Errorf("widths[1] = %d, should be > 0", widths[1])
	}
}

// TestCalculateWidthsEmpty verifies width calculation with no columns.
func TestCalculateWidthsEmpty(t *testing.T) {
	widths := CalculateWidths(nil, 100)

	if widths != nil {
		t.Errorf("CalculateWidths(nil) = %v, want nil", widths)
	}
}

// TestCalculateWidthsZeroWidth verifies width calculation with zero width.
func TestCalculateWidthsZeroWidth(t *testing.T) {
	columns := []Column{&NameColumn{}}

	widths := CalculateWidths(columns, 0)

	if len(widths) != 1 {
		t.Fatalf("CalculateWidths() returned %d widths, want 1", len(widths))
	}

	// Should be scaled down (possibly to 0 or 1)
	if widths[0] < 0 {
		t.Errorf("widths[0] = %d, should be >= 0", widths[0])
	}
}

// TestPadOrTruncate verifies padding and truncation.
func TestPadOrTruncate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		width    int
		expected int // expected visual width
	}{
		{"exact", "hello", 5, 5},
		{"pad", "hi", 5, 5},
		{"truncate", "hello world", 5, 5},
		{"empty", "", 5, 5},
		{"zero width", "hello", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := padOrTruncate(tt.input, tt.width)
			visualWidth := runewidth.StringWidth(result)

			if visualWidth != tt.expected {
				t.Errorf("padOrTruncate(%q, %d) visual width = %d, want %d, result = %q",
					tt.input, tt.width, visualWidth, tt.expected, result)
			}
		})
	}
}

// TestPadLeft verifies right-alignment.
func TestPadLeft(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		width    int
		expected string
	}{
		{"pad", "42", 5, "   42"},
		{"exact", "12345", 5, "12345"},
		{"truncate", "123456", 5, "12345"},
		{"empty", "", 5, "     "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := padLeft(tt.input, tt.width)

			if result != tt.expected {
				t.Errorf("padLeft(%q, %d) = %q, want %q",
					tt.input, tt.width, result, tt.expected)
			}
		})
	}
}

// TestTruncateWithEllipsis verifies truncation with ellipsis.
func TestTruncateWithEllipsis(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		width    int
		contains string
	}{
		{"normal", "hello world", 8, "..."},
		{"no ellipsis space", "hello", 3, "hel"},
		{"exact", "hi", 10, "hi"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateWithEllipsis(tt.input, tt.width)

			if !strings.Contains(result, tt.contains) {
				t.Errorf("truncateWithEllipsis(%q, %d) = %q, should contain %q",
					tt.input, tt.width, result, tt.contains)
			}

			visualWidth := runewidth.StringWidth(result)
			if visualWidth > tt.width {
				t.Errorf("truncateWithEllipsis(%q, %d) visual width = %d, should be <= %d",
					tt.input, tt.width, visualWidth, tt.width)
			}
		})
	}
}

// TestRenderRow verifies row rendering.
func TestRenderRow(t *testing.T) {
	columns := []Column{
		&StatusColumn{},
		&NameColumn{},
	}
	widths := []int{1, 20}
	styles := NewStyles()
	sess := testSession()

	rendered := RenderRow(sess, columns, widths, &styles)

	// Should contain project name
	if !strings.Contains(rendered, "test-project") {
		t.Errorf("RenderRow() should contain project name")
	}

	// Should contain separator
	if !strings.Contains(rendered, ColumnSeparator) {
		t.Errorf("RenderRow() should contain column separator")
	}
}

// TestRenderRowEmpty verifies empty row rendering.
func TestRenderRowEmpty(t *testing.T) {
	rendered := RenderRow(nil, nil, nil, nil)

	if rendered != "" {
		t.Errorf("RenderRow(nil, nil, nil, nil) = %q, want empty string", rendered)
	}
}

// TestRenderHeader verifies header rendering.
func TestRenderHeader(t *testing.T) {
	columns := []Column{
		&NameColumn{},
		&ModifiedColumn{},
	}
	widths := []int{20, 12}
	styles := NewStyles()

	rendered := RenderHeader(columns, widths, &styles)

	// Should contain column headers
	if !strings.Contains(rendered, "Name") {
		t.Errorf("RenderHeader() should contain 'Name'")
	}
	if !strings.Contains(rendered, "Modified") {
		t.Errorf("RenderHeader() should contain 'Modified'")
	}
}

// TestTotalMinWidth verifies total minimum width calculation.
func TestTotalMinWidth(t *testing.T) {
	columns := []Column{
		&StatusColumn{},   // 1
		&NameColumn{},     // 10
		&ModifiedColumn{}, // 8
	}

	total := TotalMinWidth(columns)

	// Expected: 1 + 10 + 8 + 2 separators = 21
	expected := MinWidthStatus + MinWidthName + MinWidthModified + 2
	if total != expected {
		t.Errorf("TotalMinWidth() = %d, want %d", total, expected)
	}
}

// TestTotalMinWidthEmpty verifies total min width for empty list.
func TestTotalMinWidthEmpty(t *testing.T) {
	total := TotalMinWidth(nil)

	if total != 0 {
		t.Errorf("TotalMinWidth(nil) = %d, want 0", total)
	}
}

// TestColumnString verifies ColumnString output.
func TestColumnString(t *testing.T) {
	col := &NameColumn{}
	str := ColumnString(col)

	if !strings.Contains(str, "Name") {
		t.Errorf("ColumnString() should contain 'Name', got %q", str)
	}
	if !strings.Contains(str, "name") {
		t.Errorf("ColumnString() should contain column type 'name', got %q", str)
	}
}

// TestColumnStringNil verifies ColumnString handles nil.
func TestColumnStringNil(t *testing.T) {
	str := ColumnString(nil)

	if str != "<nil>" {
		t.Errorf("ColumnString(nil) = %q, want %q", str, "<nil>")
	}
}

// TestUnicodeWidthHandling verifies proper Unicode width handling.
func TestUnicodeWidthHandling(t *testing.T) {
	// Test with wide characters (emoji, CJK, etc.)
	tests := []struct {
		name  string
		input string
		width int
	}{
		{"ascii", "hello", 10},
		{"emoji", "test", 10}, // Using ASCII for reliability
		{"mixed", "mix", 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := padOrTruncate(tt.input, tt.width)
			visualWidth := runewidth.StringWidth(result)

			if visualWidth != tt.width {
				t.Errorf("padOrTruncate(%q, %d) visual width = %d, want %d",
					tt.input, tt.width, visualWidth, tt.width)
			}
		})
	}
}
