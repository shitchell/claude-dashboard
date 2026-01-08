package config

import (
	"os"
	"path/filepath"
	"time"

	"github.com/shitchell/claude-dashboard/internal/constants"
	"github.com/shitchell/claude-dashboard/internal/tmux"
)

// Default values for configuration.
// These constants define the baseline configuration used when no
// config file exists or when specific values are not specified.
const (
	// DefaultMode is the default display mode.
	DefaultMode = DisplayModeList

	// DefaultLayout is the default layout type.
	DefaultLayout = LayoutTypeList

	// DefaultSidePanelEnabled is whether side panel is enabled by default.
	DefaultSidePanelEnabled = true

	// DefaultSidePanelWidth is the default side panel width (0 = auto).
	DefaultSidePanelWidth = 0

	// DefaultGridCardWidth is the default width of grid cards.
	DefaultGridCardWidth = 40

	// DefaultGridCardHeight is the default height of grid cards.
	DefaultGridCardHeight = 8

	// DefaultGridGap is the default spacing between grid cards.
	DefaultGridGap = 1

	// DefaultIndicatorActive is the default indicator for active sessions.
	DefaultIndicatorActive = "●"

	// DefaultIndicatorIdle is the default indicator for idle sessions.
	DefaultIndicatorIdle = "○"

	// DefaultIndicatorExited is the default indicator for exited sessions.
	DefaultIndicatorExited = "·"

	// DefaultSortField is the default sort field.
	DefaultSortField = "modified"

	// DefaultSortAscending is the default sort direction.
	DefaultSortAscending = false

	// DefaultTmuxEnabled is whether tmux integration is enabled by default.
	DefaultTmuxEnabled = true

	// DefaultTmuxHighlightRunning is whether to highlight running sessions.
	DefaultTmuxHighlightRunning = true

	// DefaultCacheEnabled is whether caching is enabled by default.
	DefaultCacheEnabled = true

	// DefaultRefreshEnabled is whether auto-refresh is enabled by default.
	DefaultRefreshEnabled = true

	// DefaultStatusColumnWidth is the default width for the status column.
	DefaultStatusColumnWidth = 8

	// DefaultProjectColumnWidth is the default width for the project column.
	DefaultProjectColumnWidth = 30

	// DefaultMessagesColumnWidth is the default width for the messages column.
	DefaultMessagesColumnWidth = 8

	// DefaultTurnsColumnWidth is the default width for the turns column.
	DefaultTurnsColumnWidth = 6

	// DefaultModelColumnWidth is the default width for the model column.
	DefaultModelColumnWidth = 25
)

// DefaultColumns is the default list of columns to display.
var DefaultColumns = []ColumnType{
	ColumnTypeStatus,
	ColumnTypeName,
	ColumnTypePreview,
	ColumnTypeModified,
}

// DefaultColumnStrings is the default list of column names as strings.
var DefaultColumnStrings = []string{
	"status",
	"name",
	"preview",
	"modified",
}

// Defaults returns a Config with all default values set.
// This provides the baseline configuration that is overridden by
// config file values and then by CLI flags.
func Defaults() Config {
	// Create a copy of the default columns slice to avoid mutation
	columns := make([]ColumnType, len(DefaultColumns))
	copy(columns, DefaultColumns)

	columnStrings := make([]string, len(DefaultColumnStrings))
	copy(columnStrings, DefaultColumnStrings)

	// Resolve home directory for absolute paths
	homeDir, _ := os.UserHomeDir()
	projectsDir := constants.ClaudeProjectsDir
	cacheDir := constants.CacheDir
	if homeDir != "" {
		projectsDir = filepath.Join(homeDir, constants.ClaudeProjectsDir)
		cacheDir = filepath.Join(homeDir, constants.CacheDir)
	}

	return Config{
		Mode:           DefaultMode,
		ModeString:     DefaultMode.String(),
		Layout:         DefaultLayout,
		LayoutString:   DefaultLayout.String(),
		Columns:        columns,
		ColumnsStrings: columnStrings,
		Widths: ColumnWidths{
			Name:     constants.DefaultColumnWidthName,
			Preview:  constants.DefaultColumnWidthPreview,
			Modified: constants.DefaultColumnWidthModified,
			Status:   DefaultStatusColumnWidth,
			Project:  DefaultProjectColumnWidth,
			Messages: DefaultMessagesColumnWidth,
			Turns:    DefaultTurnsColumnWidth,
			Model:    DefaultModelColumnWidth,
		},
		SidePanel: SidePanelConfig{
			Enabled:  DefaultSidePanelEnabled,
			MinWidth: constants.MinSidePanelWidth,
			Width:    DefaultSidePanelWidth,
		},
		Grid: GridConfig{
			CardWidth:  DefaultGridCardWidth,
			CardHeight: DefaultGridCardHeight,
			Gap:        DefaultGridGap,
		},
		Indicators: IndicatorConfig{
			Active: DefaultIndicatorActive,
			Idle:   DefaultIndicatorIdle,
			Exited: DefaultIndicatorExited,
		},
		Sort: SortConfig{
			Field:     DefaultSortField,
			Ascending: DefaultSortAscending,
		},
		Filter: FilterConfig{
			// All filters empty/off by default
		},
		Tmux: TmuxConfig{
			Enabled:          DefaultTmuxEnabled,
			HighlightRunning: DefaultTmuxHighlightRunning,
			NavigationMethod: tmux.NavigationMethodDefault,
		},
		Cache: CacheConfig{
			Enabled:  DefaultCacheEnabled,
			Dir:      cacheDir,
			Filename: constants.CacheFileName,
		},
		Refresh: RefreshConfig{
			Enabled:         DefaultRefreshEnabled,
			Interval:        constants.DefaultRefreshInterval,
			IntervalSeconds: int(constants.DefaultRefreshInterval / time.Second),
		},
		Sessions: SessionsConfig{
			ProjectsDir:      projectsDir,
			MaxNameLength:    constants.MaxNameLength,
			MaxPreviewLength: constants.MaxPreviewLength,
		},
	}
}
