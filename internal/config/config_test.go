package config

import (
	"testing"
	"time"

	"github.com/shitchell/claude-dashboard/internal/session"
)

func TestDisplayModeString(t *testing.T) {
	tests := []struct {
		mode     DisplayMode
		expected string
	}{
		{DisplayModeList, "list"},
		{DisplayModeGrid, "grid"},
		{DisplayMode(99), "list"}, // Unknown defaults to list
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.mode.String(); got != tt.expected {
				t.Errorf("DisplayMode.String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestParseDisplayMode(t *testing.T) {
	tests := []struct {
		input    string
		expected DisplayMode
	}{
		{"list", DisplayModeList},
		{"grid", DisplayModeGrid},
		{"unknown", DisplayModeList},
		{"", DisplayModeList},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ParseDisplayMode(tt.input); got != tt.expected {
				t.Errorf("ParseDisplayMode(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestLayoutTypeString(t *testing.T) {
	tests := []struct {
		layout   LayoutType
		expected string
	}{
		{LayoutTypeList, "list"},
		{LayoutTypeGrid, "grid"},
		{LayoutType(99), "list"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.layout.String(); got != tt.expected {
				t.Errorf("LayoutType.String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestParseLayoutType(t *testing.T) {
	tests := []struct {
		input    string
		expected LayoutType
	}{
		{"list", LayoutTypeList},
		{"grid", LayoutTypeGrid},
		{"unknown", LayoutTypeList},
		{"", LayoutTypeList},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ParseLayoutType(tt.input); got != tt.expected {
				t.Errorf("ParseLayoutType(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestColumnTypeString(t *testing.T) {
	tests := []struct {
		col      ColumnType
		expected string
	}{
		{ColumnTypeName, "name"},
		{ColumnTypePreview, "preview"},
		{ColumnTypeModified, "modified"},
		{ColumnTypeStatus, "status"},
		{ColumnTypeProject, "project"},
		{ColumnTypeMessages, "messages"},
		{ColumnTypeTurns, "turns"},
		{ColumnTypeModel, "model"},
		{ColumnType(99), "name"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.col.String(); got != tt.expected {
				t.Errorf("ColumnType.String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestParseColumnType(t *testing.T) {
	tests := []struct {
		input    string
		expected ColumnType
	}{
		{"name", ColumnTypeName},
		{"preview", ColumnTypePreview},
		{"modified", ColumnTypeModified},
		{"status", ColumnTypeStatus},
		{"project", ColumnTypeProject},
		{"messages", ColumnTypeMessages},
		{"turns", ColumnTypeTurns},
		{"model", ColumnTypeModel},
		{"unknown", ColumnTypeName},
		{"", ColumnTypeName},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ParseColumnType(tt.input); got != tt.expected {
				t.Errorf("ParseColumnType(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestSortConfigToSessionSortConfig(t *testing.T) {
	tests := []struct {
		name      string
		cfg       SortConfig
		wantField session.SortField
		wantAsc   bool
	}{
		{
			name:      "modified",
			cfg:       SortConfig{Field: "modified", Ascending: false},
			wantField: session.SortByModTime,
			wantAsc:   false,
		},
		{
			name:      "mod_time alias",
			cfg:       SortConfig{Field: "mod_time", Ascending: true},
			wantField: session.SortByModTime,
			wantAsc:   true,
		},
		{
			name:      "project",
			cfg:       SortConfig{Field: "project", Ascending: true},
			wantField: session.SortByProjectName,
			wantAsc:   true,
		},
		{
			name:      "project_name alias",
			cfg:       SortConfig{Field: "project_name", Ascending: false},
			wantField: session.SortByProjectName,
			wantAsc:   false,
		},
		{
			name:      "status",
			cfg:       SortConfig{Field: "status", Ascending: false},
			wantField: session.SortByStatus,
			wantAsc:   false,
		},
		{
			name:      "messages",
			cfg:       SortConfig{Field: "messages", Ascending: true},
			wantField: session.SortByMessageCount,
			wantAsc:   true,
		},
		{
			name:      "turns",
			cfg:       SortConfig{Field: "turns", Ascending: false},
			wantField: session.SortByTurnCount,
			wantAsc:   false,
		},
		{
			name:      "summary",
			cfg:       SortConfig{Field: "summary", Ascending: true},
			wantField: session.SortBySummary,
			wantAsc:   true,
		},
		{
			name:      "unknown defaults to modified",
			cfg:       SortConfig{Field: "unknown", Ascending: false},
			wantField: session.SortByModTime,
			wantAsc:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.ToSessionSortConfig()
			if got.Field != tt.wantField {
				t.Errorf("Field = %v, want %v", got.Field, tt.wantField)
			}
			if got.Ascending != tt.wantAsc {
				t.Errorf("Ascending = %v, want %v", got.Ascending, tt.wantAsc)
			}
		})
	}
}

func TestFilterConfigToSessionFilterConfig(t *testing.T) {
	tests := []struct {
		name         string
		cfg          FilterConfig
		wantMaxAge   time.Duration
		wantMinAge   time.Duration
		wantProject  string
		wantExcluded bool
	}{
		{
			name: "basic filter",
			cfg: FilterConfig{
				Project:       "myproject",
				ExcludeExited: true,
			},
			wantProject:  "myproject",
			wantExcluded: true,
		},
		{
			name: "with durations",
			cfg: FilterConfig{
				MaxAge: "24h",
				MinAge: "1h",
			},
			wantMaxAge: 24 * time.Hour,
			wantMinAge: 1 * time.Hour,
		},
		{
			name: "with days",
			cfg: FilterConfig{
				MaxAge: "7d",
			},
			wantMaxAge: 7 * 24 * time.Hour,
		},
		{
			name: "invalid duration is ignored",
			cfg: FilterConfig{
				MaxAge: "invalid",
			},
			wantMaxAge: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.ToSessionFilterConfig()
			if got.MaxAge != tt.wantMaxAge {
				t.Errorf("MaxAge = %v, want %v", got.MaxAge, tt.wantMaxAge)
			}
			if got.MinAge != tt.wantMinAge {
				t.Errorf("MinAge = %v, want %v", got.MinAge, tt.wantMinAge)
			}
			if got.Project != tt.wantProject {
				t.Errorf("Project = %q, want %q", got.Project, tt.wantProject)
			}
			if got.ExcludeExited != tt.wantExcluded {
				t.Errorf("ExcludeExited = %v, want %v", got.ExcludeExited, tt.wantExcluded)
			}
		})
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{"1h", time.Hour, false},
		{"30m", 30 * time.Minute, false},
		{"1d", 24 * time.Hour, false},
		{"7d", 7 * 24 * time.Hour, false},
		{"30d", 30 * 24 * time.Hour, false},
		{"5s", 5 * time.Second, false},
		{"invalid", 0, true},
		{"1x", 0, true},
		{"xd", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseDuration(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDuration(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.expected {
				t.Errorf("parseDuration(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestConfigSyncFromStrings(t *testing.T) {
	cfg := Config{
		ModeString:      "grid",
		LayoutString:    "list",
		ColumnsStrings:  []string{"name", "status", "modified"},
		Refresh:         RefreshConfig{IntervalSeconds: 10},
	}

	cfg.syncFromStrings()

	if cfg.Mode != DisplayModeGrid {
		t.Errorf("Mode = %v, want %v", cfg.Mode, DisplayModeGrid)
	}
	if cfg.Layout != LayoutTypeList {
		t.Errorf("Layout = %v, want %v", cfg.Layout, LayoutTypeList)
	}
	if len(cfg.Columns) != 3 {
		t.Errorf("len(Columns) = %d, want 3", len(cfg.Columns))
	} else {
		if cfg.Columns[0] != ColumnTypeName {
			t.Errorf("Columns[0] = %v, want %v", cfg.Columns[0], ColumnTypeName)
		}
		if cfg.Columns[1] != ColumnTypeStatus {
			t.Errorf("Columns[1] = %v, want %v", cfg.Columns[1], ColumnTypeStatus)
		}
		if cfg.Columns[2] != ColumnTypeModified {
			t.Errorf("Columns[2] = %v, want %v", cfg.Columns[2], ColumnTypeModified)
		}
	}
	if cfg.Refresh.Interval != 10*time.Second {
		t.Errorf("Refresh.Interval = %v, want %v", cfg.Refresh.Interval, 10*time.Second)
	}
}

func TestConfigSyncToStrings(t *testing.T) {
	cfg := Config{
		Mode:    DisplayModeGrid,
		Layout:  LayoutTypeList,
		Columns: []ColumnType{ColumnTypeName, ColumnTypePreview},
		Refresh: RefreshConfig{Interval: 15 * time.Second},
	}

	cfg.syncToStrings()

	if cfg.ModeString != "grid" {
		t.Errorf("ModeString = %q, want %q", cfg.ModeString, "grid")
	}
	if cfg.LayoutString != "list" {
		t.Errorf("LayoutString = %q, want %q", cfg.LayoutString, "list")
	}
	if len(cfg.ColumnsStrings) != 2 {
		t.Errorf("len(ColumnsStrings) = %d, want 2", len(cfg.ColumnsStrings))
	} else {
		if cfg.ColumnsStrings[0] != "name" {
			t.Errorf("ColumnsStrings[0] = %q, want %q", cfg.ColumnsStrings[0], "name")
		}
		if cfg.ColumnsStrings[1] != "preview" {
			t.Errorf("ColumnsStrings[1] = %q, want %q", cfg.ColumnsStrings[1], "preview")
		}
	}
	if cfg.Refresh.IntervalSeconds != 15 {
		t.Errorf("Refresh.IntervalSeconds = %d, want 15", cfg.Refresh.IntervalSeconds)
	}
}
