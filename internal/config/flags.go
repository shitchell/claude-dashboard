package config

import (
	"flag"
	"strings"
	"time"
)

// Flags holds parsed CLI flag values.
// Pointer types are used to distinguish between "not set" and "set to zero/false".
// Only explicitly set flags will override configuration values.
type Flags struct {
	// ConfigPath is the path to the config file (-c, --config).
	ConfigPath string

	// Mode is the display mode (-m, --mode).
	Mode *string

	// Layout is the layout type (-l, --layout).
	Layout *string

	// Columns is the list of columns to display (--columns).
	Columns *string

	// RefreshInterval is the refresh interval (--refresh).
	RefreshInterval *time.Duration

	// RefreshEnabled enables/disables auto-refresh (--no-refresh to disable).
	RefreshEnabled *bool

	// TmuxEnabled enables/disables tmux integration (--no-tmux to disable).
	TmuxEnabled *bool

	// CacheEnabled enables/disables caching (--no-cache to disable).
	CacheEnabled *bool

	// SortField is the field to sort by (--sort).
	SortField *string

	// SortAscending sets ascending sort order (--asc).
	SortAscending *bool

	// Project filters to a specific project (--project, -p).
	Project *string

	// MaxAge filters sessions by age (--max-age).
	MaxAge *string

	// ExcludeExited excludes exited sessions (--running).
	ExcludeExited *bool

	// Version shows version and exits (--version, -v).
	Version bool

	// Help shows help and exits (--help, -h).
	Help bool
}

// ParseFlags parses CLI arguments into a Flags struct.
// Returns the flags struct and any parse error.
func ParseFlags(args []string) (*Flags, error) {
	f := &Flags{}
	fs := flag.NewFlagSet("claude-dashboard", flag.ContinueOnError)

	// Config file
	fs.StringVar(&f.ConfigPath, "c", "", "path to config file")
	fs.StringVar(&f.ConfigPath, "config", "", "path to config file")

	// Mode and layout use string variables that we check for emptiness
	var modeStr, layoutStr string
	fs.StringVar(&modeStr, "m", "", "display mode: list or grid")
	fs.StringVar(&modeStr, "mode", "", "display mode: list or grid")
	fs.StringVar(&layoutStr, "l", "", "layout type: list or grid")
	fs.StringVar(&layoutStr, "layout", "", "layout type: list or grid")

	// Columns
	var columnsStr string
	fs.StringVar(&columnsStr, "columns", "", "comma-separated column list (e.g., status,name,preview,modified)")

	// Refresh
	var refreshStr string
	fs.StringVar(&refreshStr, "refresh", "", "refresh interval (e.g., 5s, 1m)")

	// Boolean flags with negation support
	var noRefresh, noTmux, noCache bool
	fs.BoolVar(&noRefresh, "no-refresh", false, "disable auto-refresh")
	fs.BoolVar(&noTmux, "no-tmux", false, "disable tmux integration")
	fs.BoolVar(&noCache, "no-cache", false, "disable session caching")

	// Sort
	var sortField string
	var sortAsc bool
	fs.StringVar(&sortField, "sort", "", "sort field: modified, project, status, messages, turns, summary")
	fs.BoolVar(&sortAsc, "asc", false, "sort in ascending order")

	// Filter
	var project, maxAge string
	var running bool
	fs.StringVar(&project, "project", "", "filter to project (matches path or name)")
	fs.StringVar(&project, "p", "", "filter to project (matches path or name)")
	fs.StringVar(&maxAge, "max-age", "", "filter by age (e.g., 24h, 7d)")
	fs.BoolVar(&running, "running", false, "show only running sessions (exclude exited)")

	// Meta flags
	fs.BoolVar(&f.Version, "version", false, "show version and exit")
	fs.BoolVar(&f.Version, "v", false, "show version and exit")
	fs.BoolVar(&f.Help, "help", false, "show help and exit")
	fs.BoolVar(&f.Help, "h", false, "show help and exit")

	// Parse the flags
	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	// Convert parsed values to pointers only if they were explicitly set
	// We detect this by checking for non-empty strings or true booleans

	if modeStr != "" {
		f.Mode = &modeStr
	}
	if layoutStr != "" {
		f.Layout = &layoutStr
	}
	if columnsStr != "" {
		f.Columns = &columnsStr
	}
	if refreshStr != "" {
		d, err := time.ParseDuration(refreshStr)
		if err == nil {
			f.RefreshInterval = &d
		}
	}
	if noRefresh {
		enabled := false
		f.RefreshEnabled = &enabled
	}
	if noTmux {
		enabled := false
		f.TmuxEnabled = &enabled
	}
	if noCache {
		enabled := false
		f.CacheEnabled = &enabled
	}
	if sortField != "" {
		f.SortField = &sortField
	}
	if sortAsc {
		f.SortAscending = &sortAsc
	}
	if project != "" {
		f.Project = &project
	}
	if maxAge != "" {
		f.MaxAge = &maxAge
	}
	if running {
		f.ExcludeExited = &running
	}

	return f, nil
}

// Apply applies flag values to a Config.
// Only flags that were explicitly set override config values.
func (f *Flags) Apply(cfg *Config) {
	if f == nil {
		return
	}

	// Mode
	if f.Mode != nil {
		cfg.Mode = ParseDisplayMode(*f.Mode)
		cfg.ModeString = *f.Mode
	}

	// Layout
	if f.Layout != nil {
		cfg.Layout = ParseLayoutType(*f.Layout)
		cfg.LayoutString = *f.Layout
	}

	// Columns
	if f.Columns != nil {
		parts := strings.Split(*f.Columns, ",")
		cfg.ColumnsStrings = make([]string, 0, len(parts))
		cfg.Columns = make([]ColumnType, 0, len(parts))
		for _, part := range parts {
			col := strings.TrimSpace(part)
			if col != "" {
				cfg.ColumnsStrings = append(cfg.ColumnsStrings, col)
				cfg.Columns = append(cfg.Columns, ParseColumnType(col))
			}
		}
	}

	// Refresh
	if f.RefreshInterval != nil {
		cfg.Refresh.Interval = *f.RefreshInterval
		cfg.Refresh.IntervalSeconds = int(f.RefreshInterval.Seconds())
	}
	if f.RefreshEnabled != nil {
		cfg.Refresh.Enabled = *f.RefreshEnabled
	}

	// Tmux
	if f.TmuxEnabled != nil {
		cfg.Tmux.Enabled = *f.TmuxEnabled
	}

	// Cache
	if f.CacheEnabled != nil {
		cfg.Cache.Enabled = *f.CacheEnabled
	}

	// Sort
	if f.SortField != nil {
		cfg.Sort.Field = *f.SortField
	}
	if f.SortAscending != nil {
		cfg.Sort.Ascending = *f.SortAscending
	}

	// Filter
	if f.Project != nil {
		cfg.Filter.Project = *f.Project
	}
	if f.MaxAge != nil {
		cfg.Filter.MaxAge = *f.MaxAge
	}
	if f.ExcludeExited != nil {
		cfg.Filter.ExcludeExited = *f.ExcludeExited
	}
}

// IsEmpty returns true if no flags were set (besides config path).
func (f *Flags) IsEmpty() bool {
	return f.Mode == nil &&
		f.Layout == nil &&
		f.Columns == nil &&
		f.RefreshInterval == nil &&
		f.RefreshEnabled == nil &&
		f.TmuxEnabled == nil &&
		f.CacheEnabled == nil &&
		f.SortField == nil &&
		f.SortAscending == nil &&
		f.Project == nil &&
		f.MaxAge == nil &&
		f.ExcludeExited == nil
}

// HasConfigPath returns true if a config path was specified.
func (f *Flags) HasConfigPath() bool {
	return f.ConfigPath != ""
}
