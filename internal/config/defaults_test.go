package config

import (
	"testing"
	"time"

	"github.com/shitchell/claude-dashboard/internal/constants"
	"github.com/shitchell/claude-dashboard/internal/tmux"
)

func TestDefaults(t *testing.T) {
	cfg := Defaults()

	// Mode and Layout
	if cfg.Mode != DefaultMode {
		t.Errorf("Mode = %v, want %v", cfg.Mode, DefaultMode)
	}
	if cfg.Layout != DefaultLayout {
		t.Errorf("Layout = %v, want %v", cfg.Layout, DefaultLayout)
	}
	if cfg.ModeString != "list" {
		t.Errorf("ModeString = %q, want %q", cfg.ModeString, "list")
	}
	if cfg.LayoutString != "list" {
		t.Errorf("LayoutString = %q, want %q", cfg.LayoutString, "list")
	}

	// Columns
	if len(cfg.Columns) != len(DefaultColumns) {
		t.Errorf("len(Columns) = %d, want %d", len(cfg.Columns), len(DefaultColumns))
	}
	for i, col := range cfg.Columns {
		if col != DefaultColumns[i] {
			t.Errorf("Columns[%d] = %v, want %v", i, col, DefaultColumns[i])
		}
	}

	// Widths
	if cfg.Widths.Name != constants.DefaultColumnWidthName {
		t.Errorf("Widths.Name = %d, want %d", cfg.Widths.Name, constants.DefaultColumnWidthName)
	}
	if cfg.Widths.Preview != constants.DefaultColumnWidthPreview {
		t.Errorf("Widths.Preview = %d, want %d", cfg.Widths.Preview, constants.DefaultColumnWidthPreview)
	}
	if cfg.Widths.Modified != constants.DefaultColumnWidthModified {
		t.Errorf("Widths.Modified = %d, want %d", cfg.Widths.Modified, constants.DefaultColumnWidthModified)
	}
	if cfg.Widths.Status != DefaultStatusColumnWidth {
		t.Errorf("Widths.Status = %d, want %d", cfg.Widths.Status, DefaultStatusColumnWidth)
	}

	// Side Panel
	if cfg.SidePanel.Enabled != DefaultSidePanelEnabled {
		t.Errorf("SidePanel.Enabled = %v, want %v", cfg.SidePanel.Enabled, DefaultSidePanelEnabled)
	}
	if cfg.SidePanel.MinWidth != constants.MinSidePanelWidth {
		t.Errorf("SidePanel.MinWidth = %d, want %d", cfg.SidePanel.MinWidth, constants.MinSidePanelWidth)
	}

	// Grid
	if cfg.Grid.CardWidth != DefaultGridCardWidth {
		t.Errorf("Grid.CardWidth = %d, want %d", cfg.Grid.CardWidth, DefaultGridCardWidth)
	}
	if cfg.Grid.CardHeight != DefaultGridCardHeight {
		t.Errorf("Grid.CardHeight = %d, want %d", cfg.Grid.CardHeight, DefaultGridCardHeight)
	}
	if cfg.Grid.Gap != DefaultGridGap {
		t.Errorf("Grid.Gap = %d, want %d", cfg.Grid.Gap, DefaultGridGap)
	}

	// Indicators
	if cfg.Indicators.Active != DefaultIndicatorActive {
		t.Errorf("Indicators.Active = %q, want %q", cfg.Indicators.Active, DefaultIndicatorActive)
	}
	if cfg.Indicators.Idle != DefaultIndicatorIdle {
		t.Errorf("Indicators.Idle = %q, want %q", cfg.Indicators.Idle, DefaultIndicatorIdle)
	}
	if cfg.Indicators.Exited != DefaultIndicatorExited {
		t.Errorf("Indicators.Exited = %q, want %q", cfg.Indicators.Exited, DefaultIndicatorExited)
	}

	// Sort
	if cfg.Sort.Field != DefaultSortField {
		t.Errorf("Sort.Field = %q, want %q", cfg.Sort.Field, DefaultSortField)
	}
	if cfg.Sort.Ascending != DefaultSortAscending {
		t.Errorf("Sort.Ascending = %v, want %v", cfg.Sort.Ascending, DefaultSortAscending)
	}

	// Filter (should be empty)
	if cfg.Filter.MaxAge != "" {
		t.Errorf("Filter.MaxAge = %q, want empty", cfg.Filter.MaxAge)
	}
	if cfg.Filter.Project != "" {
		t.Errorf("Filter.Project = %q, want empty", cfg.Filter.Project)
	}
	if cfg.Filter.ExcludeExited {
		t.Error("Filter.ExcludeExited should be false by default")
	}

	// Tmux
	if cfg.Tmux.Enabled != DefaultTmuxEnabled {
		t.Errorf("Tmux.Enabled = %v, want %v", cfg.Tmux.Enabled, DefaultTmuxEnabled)
	}
	if cfg.Tmux.HighlightRunning != DefaultTmuxHighlightRunning {
		t.Errorf("Tmux.HighlightRunning = %v, want %v", cfg.Tmux.HighlightRunning, DefaultTmuxHighlightRunning)
	}
	if cfg.Tmux.NavigationMethod != tmux.NavigationMethodDefault {
		t.Errorf("Tmux.NavigationMethod = %q, want %q", cfg.Tmux.NavigationMethod, tmux.NavigationMethodDefault)
	}

	// Cache
	if cfg.Cache.Enabled != DefaultCacheEnabled {
		t.Errorf("Cache.Enabled = %v, want %v", cfg.Cache.Enabled, DefaultCacheEnabled)
	}
	if cfg.Cache.Dir != constants.CacheDir {
		t.Errorf("Cache.Dir = %q, want %q", cfg.Cache.Dir, constants.CacheDir)
	}
	if cfg.Cache.Filename != constants.CacheFileName {
		t.Errorf("Cache.Filename = %q, want %q", cfg.Cache.Filename, constants.CacheFileName)
	}

	// Refresh
	if cfg.Refresh.Enabled != DefaultRefreshEnabled {
		t.Errorf("Refresh.Enabled = %v, want %v", cfg.Refresh.Enabled, DefaultRefreshEnabled)
	}
	if cfg.Refresh.Interval != constants.DefaultRefreshInterval {
		t.Errorf("Refresh.Interval = %v, want %v", cfg.Refresh.Interval, constants.DefaultRefreshInterval)
	}
	if cfg.Refresh.IntervalSeconds != int(constants.DefaultRefreshInterval/time.Second) {
		t.Errorf("Refresh.IntervalSeconds = %d, want %d",
			cfg.Refresh.IntervalSeconds, int(constants.DefaultRefreshInterval/time.Second))
	}

	// Sessions
	if cfg.Sessions.ProjectsDir != constants.ClaudeProjectsDir {
		t.Errorf("Sessions.ProjectsDir = %q, want %q", cfg.Sessions.ProjectsDir, constants.ClaudeProjectsDir)
	}
	if cfg.Sessions.MaxNameLength != constants.MaxNameLength {
		t.Errorf("Sessions.MaxNameLength = %d, want %d", cfg.Sessions.MaxNameLength, constants.MaxNameLength)
	}
	if cfg.Sessions.MaxPreviewLength != constants.MaxPreviewLength {
		t.Errorf("Sessions.MaxPreviewLength = %d, want %d", cfg.Sessions.MaxPreviewLength, constants.MaxPreviewLength)
	}
}

func TestDefaultsColumnsAreCopied(t *testing.T) {
	// Get two defaults and verify they have independent column slices
	cfg1 := Defaults()
	cfg2 := Defaults()

	// Modify cfg1's columns
	cfg1.Columns[0] = ColumnTypeModel

	// cfg2 should be unaffected
	if cfg2.Columns[0] == ColumnTypeModel {
		t.Error("Default columns should be copied, not shared")
	}
}

func TestDefaultColumnsContainExpectedTypes(t *testing.T) {
	// Verify the default columns contain the expected types
	expected := []ColumnType{
		ColumnTypeStatus,
		ColumnTypeName,
		ColumnTypePreview,
		ColumnTypeModified,
	}

	if len(DefaultColumns) != len(expected) {
		t.Fatalf("DefaultColumns length = %d, want %d", len(DefaultColumns), len(expected))
	}

	for i, col := range DefaultColumns {
		if col != expected[i] {
			t.Errorf("DefaultColumns[%d] = %v, want %v", i, col, expected[i])
		}
	}
}

func TestDefaultColumnStringsMatchTypes(t *testing.T) {
	if len(DefaultColumnStrings) != len(DefaultColumns) {
		t.Fatalf("DefaultColumnStrings length (%d) != DefaultColumns length (%d)",
			len(DefaultColumnStrings), len(DefaultColumns))
	}

	for i, str := range DefaultColumnStrings {
		expected := DefaultColumns[i].String()
		if str != expected {
			t.Errorf("DefaultColumnStrings[%d] = %q, want %q", i, str, expected)
		}
	}
}
