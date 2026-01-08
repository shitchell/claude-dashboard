// Package config handles loading and managing application configuration.
// Configuration can come from defaults, YAML files, or CLI flags, with
// merge priority: defaults <- file <- flags (flags win).
package config

import (
	"time"

	"github.com/shitchell/claude-dashboard/internal/session"
)

// DisplayMode represents the UI display mode.
type DisplayMode int

const (
	// DisplayModeList shows sessions in a vertical list.
	DisplayModeList DisplayMode = iota

	// DisplayModeGrid shows sessions in a grid layout.
	DisplayModeGrid
)

// String returns the string representation of the display mode.
func (m DisplayMode) String() string {
	switch m {
	case DisplayModeList:
		return "list"
	case DisplayModeGrid:
		return "grid"
	default:
		return "list"
	}
}

// ParseDisplayMode converts a string to a DisplayMode.
// Returns DisplayModeList for unrecognized values.
func ParseDisplayMode(s string) DisplayMode {
	switch s {
	case "grid":
		return DisplayModeGrid
	default:
		return DisplayModeList
	}
}

// LayoutType represents the layout style within a display mode.
type LayoutType int

const (
	// LayoutTypeList is the standard list layout.
	LayoutTypeList LayoutType = iota

	// LayoutTypeGrid is the grid/card layout.
	LayoutTypeGrid
)

// String returns the string representation of the layout type.
func (l LayoutType) String() string {
	switch l {
	case LayoutTypeList:
		return "list"
	case LayoutTypeGrid:
		return "grid"
	default:
		return "list"
	}
}

// ParseLayoutType converts a string to a LayoutType.
// Returns LayoutTypeList for unrecognized values.
func ParseLayoutType(s string) LayoutType {
	switch s {
	case "grid":
		return LayoutTypeGrid
	default:
		return LayoutTypeList
	}
}

// ColumnType represents a column that can be displayed in list view.
type ColumnType int

const (
	// ColumnTypeName shows the project/session name.
	ColumnTypeName ColumnType = iota

	// ColumnTypePreview shows a preview of the last message.
	ColumnTypePreview

	// ColumnTypeModified shows the modification time.
	ColumnTypeModified

	// ColumnTypeStatus shows the session status (active/idle/exited).
	ColumnTypeStatus

	// ColumnTypeProject shows the project path.
	ColumnTypeProject

	// ColumnTypeMessages shows the message count.
	ColumnTypeMessages

	// ColumnTypeTurns shows the turn count.
	ColumnTypeTurns

	// ColumnTypeModel shows the Claude model used.
	ColumnTypeModel
)

// String returns the string representation of the column type.
func (c ColumnType) String() string {
	switch c {
	case ColumnTypeName:
		return "name"
	case ColumnTypePreview:
		return "preview"
	case ColumnTypeModified:
		return "modified"
	case ColumnTypeStatus:
		return "status"
	case ColumnTypeProject:
		return "project"
	case ColumnTypeMessages:
		return "messages"
	case ColumnTypeTurns:
		return "turns"
	case ColumnTypeModel:
		return "model"
	default:
		return "name"
	}
}

// ParseColumnType converts a string to a ColumnType.
// Returns ColumnTypeName for unrecognized values.
func ParseColumnType(s string) ColumnType {
	switch s {
	case "name":
		return ColumnTypeName
	case "preview":
		return ColumnTypePreview
	case "modified":
		return ColumnTypeModified
	case "status":
		return ColumnTypeStatus
	case "project":
		return ColumnTypeProject
	case "messages":
		return ColumnTypeMessages
	case "turns":
		return ColumnTypeTurns
	case "model":
		return ColumnTypeModel
	default:
		return ColumnTypeName
	}
}

// ColumnWidths defines the width for each column type.
type ColumnWidths struct {
	// Name is the width of the name column.
	Name int `yaml:"name"`

	// Preview is the width of the preview column.
	Preview int `yaml:"preview"`

	// Modified is the width of the modified column.
	Modified int `yaml:"modified"`

	// Status is the width of the status column.
	Status int `yaml:"status"`

	// Project is the width of the project column.
	Project int `yaml:"project"`

	// Messages is the width of the messages column.
	Messages int `yaml:"messages"`

	// Turns is the width of the turns column.
	Turns int `yaml:"turns"`

	// Model is the width of the model column.
	Model int `yaml:"model"`
}

// SidePanelConfig configures the side panel in grid view.
type SidePanelConfig struct {
	// Enabled determines whether the side panel is shown.
	Enabled bool `yaml:"enabled"`

	// MinWidth is the minimum width of the side panel.
	MinWidth int `yaml:"min_width"`

	// Width is the desired width of the side panel.
	// Zero means auto-size based on content.
	Width int `yaml:"width"`
}

// GridConfig configures the grid layout.
type GridConfig struct {
	// CardWidth is the width of each card in the grid.
	CardWidth int `yaml:"card_width"`

	// CardHeight is the height of each card in the grid.
	CardHeight int `yaml:"card_height"`

	// Gap is the spacing between cards.
	Gap int `yaml:"gap"`
}

// IndicatorConfig configures status indicators.
type IndicatorConfig struct {
	// Active is the indicator for active sessions.
	Active string `yaml:"active"`

	// Idle is the indicator for idle sessions.
	Idle string `yaml:"idle"`

	// Exited is the indicator for exited sessions.
	Exited string `yaml:"exited"`
}

// SortConfig configures session sorting.
// This embeds session.SortConfig to reuse existing sorting logic.
type SortConfig struct {
	// Field is the field to sort by.
	// Valid values: "modified", "project", "status", "messages", "turns", "summary"
	Field string `yaml:"field"`

	// Ascending determines sort direction.
	// When false (default), sorts in descending order.
	Ascending bool `yaml:"ascending"`
}

// ToSessionSortConfig converts to the session package's SortConfig.
func (s SortConfig) ToSessionSortConfig() session.SortConfig {
	var field session.SortField
	switch s.Field {
	case "modified", "mod_time":
		field = session.SortByModTime
	case "project", "project_name":
		field = session.SortByProjectName
	case "status":
		field = session.SortByStatus
	case "messages", "message_count":
		field = session.SortByMessageCount
	case "turns", "turn_count":
		field = session.SortByTurnCount
	case "summary":
		field = session.SortBySummary
	default:
		field = session.SortByModTime
	}
	return session.SortConfig{
		Field:     field,
		Ascending: s.Ascending,
	}
}

// FilterConfig configures session filtering.
// This provides YAML-friendly configuration that can be converted
// to session.FilterConfig.
type FilterConfig struct {
	// MaxAge filters out sessions older than this duration.
	// Format: "24h", "7d", "30d", etc.
	MaxAge string `yaml:"max_age"`

	// MinAge filters out sessions newer than this duration.
	MinAge string `yaml:"min_age"`

	// Project filters to sessions matching this project path or name.
	Project string `yaml:"project"`

	// ExcludeExited filters out sessions that are not running in tmux.
	ExcludeExited bool `yaml:"exclude_exited"`

	// SearchText filters to sessions containing this text.
	SearchText string `yaml:"search_text"`

	// Model filters to sessions using this model.
	Model string `yaml:"model"`
}

// ToSessionFilterConfig converts to the session package's FilterConfig.
// Duration strings are parsed; invalid durations default to zero.
func (f FilterConfig) ToSessionFilterConfig() session.FilterConfig {
	cfg := session.FilterConfig{
		Project:       f.Project,
		ExcludeExited: f.ExcludeExited,
		SearchText:    f.SearchText,
		Model:         f.Model,
	}

	if f.MaxAge != "" {
		if d, err := parseDuration(f.MaxAge); err == nil {
			cfg.MaxAge = d
		}
	}
	if f.MinAge != "" {
		if d, err := parseDuration(f.MinAge); err == nil {
			cfg.MinAge = d
		}
	}

	return cfg
}

// parseDuration parses a duration string that supports days (d).
// Standard Go durations ("1h", "30m") are supported, plus "1d" for days.
func parseDuration(s string) (time.Duration, error) {
	// Check for day suffix
	if len(s) > 0 && s[len(s)-1] == 'd' {
		// Parse the numeric part
		numStr := s[:len(s)-1]
		var days int
		for _, c := range numStr {
			if c < '0' || c > '9' {
				return 0, &time.ParseError{Layout: "duration", Value: s}
			}
			days = days*10 + int(c-'0')
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	// Fall back to standard time.ParseDuration
	return time.ParseDuration(s)
}

// TmuxConfig configures tmux integration.
type TmuxConfig struct {
	// Enabled determines whether tmux integration is active.
	Enabled bool `yaml:"enabled"`

	// HighlightRunning determines whether to visually distinguish
	// sessions that are running in tmux panes.
	HighlightRunning bool `yaml:"highlight_running"`

	// NavigationMethod is the method used for pane navigation.
	// Valid values: "select-pane", "switch-client"
	NavigationMethod string `yaml:"navigation_method"`
}

// CacheConfig configures session caching.
type CacheConfig struct {
	// Enabled determines whether caching is active.
	Enabled bool `yaml:"enabled"`

	// Dir is the cache directory path (relative to home).
	Dir string `yaml:"dir"`

	// Filename is the cache file name.
	Filename string `yaml:"filename"`
}

// RefreshConfig configures auto-refresh behavior.
type RefreshConfig struct {
	// Enabled determines whether auto-refresh is active.
	Enabled bool `yaml:"enabled"`

	// Interval is the refresh interval.
	Interval time.Duration `yaml:"-"`

	// IntervalSeconds is the interval in seconds for YAML serialization.
	IntervalSeconds int `yaml:"interval"`
}

// SessionsConfig configures session discovery and display.
type SessionsConfig struct {
	// ProjectsDir is the path to Claude projects directory (relative to home).
	ProjectsDir string `yaml:"projects_dir"`

	// MaxNameLength is the maximum characters for session name display.
	MaxNameLength int `yaml:"max_name_length"`

	// MaxPreviewLength is the maximum characters for preview text.
	MaxPreviewLength int `yaml:"max_preview_length"`
}

// Config is the main configuration struct containing all application settings.
// Configuration is loaded with priority: defaults <- file <- flags.
type Config struct {
	// Mode is the display mode (list or grid).
	Mode DisplayMode `yaml:"-"`

	// ModeString is the mode as a string for YAML serialization.
	ModeString string `yaml:"mode"`

	// Layout is the layout type within the display mode.
	Layout LayoutType `yaml:"-"`

	// LayoutString is the layout as a string for YAML serialization.
	LayoutString string `yaml:"layout"`

	// Columns is the list of columns to display in list view.
	Columns []ColumnType `yaml:"-"`

	// ColumnsStrings is the column list as strings for YAML serialization.
	ColumnsStrings []string `yaml:"columns"`

	// Widths defines the width for each column type.
	Widths ColumnWidths `yaml:"widths"`

	// SidePanel configures the side panel in grid view.
	SidePanel SidePanelConfig `yaml:"side_panel"`

	// Grid configures the grid layout.
	Grid GridConfig `yaml:"grid"`

	// Indicators configures status indicators.
	Indicators IndicatorConfig `yaml:"indicators"`

	// Sort configures session sorting.
	Sort SortConfig `yaml:"sort"`

	// Filter configures session filtering.
	Filter FilterConfig `yaml:"filter"`

	// Tmux configures tmux integration.
	Tmux TmuxConfig `yaml:"tmux"`

	// Cache configures session caching.
	Cache CacheConfig `yaml:"cache"`

	// Refresh configures auto-refresh behavior.
	Refresh RefreshConfig `yaml:"refresh"`

	// Sessions configures session discovery and display.
	Sessions SessionsConfig `yaml:"sessions"`
}

// syncFromStrings converts string representations to typed values.
// This is called after loading from YAML to populate the typed fields.
func (c *Config) syncFromStrings() {
	// Convert mode string to enum
	c.Mode = ParseDisplayMode(c.ModeString)

	// Convert layout string to enum
	c.Layout = ParseLayoutType(c.LayoutString)

	// Convert column strings to enums
	if len(c.ColumnsStrings) > 0 {
		c.Columns = make([]ColumnType, len(c.ColumnsStrings))
		for i, s := range c.ColumnsStrings {
			c.Columns[i] = ParseColumnType(s)
		}
	}

	// Convert refresh interval seconds to duration
	if c.Refresh.IntervalSeconds > 0 {
		c.Refresh.Interval = time.Duration(c.Refresh.IntervalSeconds) * time.Second
	}
}

// syncToStrings converts typed values to their string representations.
// This is called before saving to YAML.
func (c *Config) syncToStrings() {
	// Convert mode enum to string
	c.ModeString = c.Mode.String()

	// Convert layout enum to string
	c.LayoutString = c.Layout.String()

	// Convert column enums to strings
	if len(c.Columns) > 0 {
		c.ColumnsStrings = make([]string, len(c.Columns))
		for i, col := range c.Columns {
			c.ColumnsStrings[i] = col.String()
		}
	}

	// Convert refresh duration to seconds
	c.Refresh.IntervalSeconds = int(c.Refresh.Interval.Seconds())
}
