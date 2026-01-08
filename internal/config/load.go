package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/shitchell/claude-dashboard/internal/constants"
	"github.com/shitchell/claude-dashboard/internal/tmux"
)

// Configuration file name constants.
const (
	// ConfigFileName is the default config file name.
	ConfigFileName = "config.yaml"

	// ConfigFileNameAlt is an alternative config file name.
	ConfigFileNameAlt = "config.yml"
)

// Validation error constants define specific validation failure types.
var (
	// ErrInvalidRefreshInterval indicates the refresh interval is too small.
	ErrInvalidRefreshInterval = errors.New("refresh interval must be at least 1 second")

	// ErrInvalidColumnWidth indicates a column width is invalid.
	ErrInvalidColumnWidth = errors.New("column width must be positive")

	// ErrInvalidSidePanelWidth indicates the side panel width is too small.
	ErrInvalidSidePanelWidth = errors.New("side panel min_width must be at least 1")

	// ErrInvalidGridCardSize indicates grid card dimensions are invalid.
	ErrInvalidGridCardSize = errors.New("grid card dimensions must be positive")

	// ErrInvalidNavigationMethod indicates an unknown navigation method.
	ErrInvalidNavigationMethod = errors.New("navigation method must be 'select-pane' or 'switch-client'")

	// ErrEmptyProjectsDir indicates the projects directory is empty.
	ErrEmptyProjectsDir = errors.New("sessions.projects_dir cannot be empty")

	// ErrInvalidMaxLength indicates a max length value is invalid.
	ErrInvalidMaxLength = errors.New("max length must be positive")
)

// ValidationError wraps a validation error with context about what failed.
type ValidationError struct {
	Field string
	Err   error
}

// Error returns the error message with field context.
func (e *ValidationError) Error() string {
	return fmt.Sprintf("config validation error for %s: %v", e.Field, e.Err)
}

// Unwrap returns the underlying error.
func (e *ValidationError) Unwrap() error {
	return e.Err
}

// Load loads configuration with priority: defaults <- file <- flags.
// If configPath is empty, it searches standard locations.
// If flags is nil, no flag overrides are applied.
//
// The function:
// 1. Starts with default values
// 2. Merges in values from config file (if found)
// 3. Applies CLI flag overrides
// 4. Validates the final configuration
//
// Returns the merged config and any error encountered during loading or validation.
func Load(configPath string, flags *Flags) (*Config, error) {
	// Start with defaults
	cfg := Defaults()

	// Load config file if it exists
	filePath := configPath
	if filePath == "" {
		filePath = findConfigFile()
	}

	if filePath != "" {
		if err := loadFile(filePath, &cfg); err != nil {
			return nil, fmt.Errorf("loading config file: %w", err)
		}
	}

	// Apply CLI flags if provided
	if flags != nil {
		flags.Apply(&cfg)
	}

	// Validate the final configuration
	if err := validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// findConfigFile searches for a config file in standard locations.
// Returns the path to the first config file found, or empty string if none.
//
// Search order:
// 1. ~/.config/claude-dashboard/config.yaml
// 2. ~/.config/claude-dashboard/config.yml
func findConfigFile() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	// Check both .yaml and .yml extensions
	configDir := filepath.Join(home, constants.ConfigDir)
	candidates := []string{
		filepath.Join(configDir, ConfigFileName),
		filepath.Join(configDir, ConfigFileNameAlt),
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// loadFile loads configuration from a YAML file and merges it into cfg.
// Only values that are explicitly set in the file override the existing config.
func loadFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading config file: %w", err)
	}

	return loadFromYAMLData(data, cfg)
}

// loadFromYAMLData parses YAML data and merges it into cfg.
// It uses a raw map to detect which fields were actually set in the YAML.
func loadFromYAMLData(data []byte, cfg *Config) error {
	// First, parse into a raw map to detect which fields are set
	var rawMap map[string]interface{}
	if err := yaml.Unmarshal(data, &rawMap); err != nil {
		return fmt.Errorf("parsing config file: %w", err)
	}

	// Parse into struct
	var fileCfg Config
	if err := yaml.Unmarshal(data, &fileCfg); err != nil {
		return fmt.Errorf("parsing config file: %w", err)
	}

	// Sync string fields to typed fields
	fileCfg.syncFromStrings()

	// Merge file values into config, using rawMap to detect what was set
	mergeConfigWithRaw(cfg, &fileCfg, rawMap)

	return nil
}

// mergeConfig merges non-zero values from src into dst.
// Zero values in src do not override values in dst.
func mergeConfig(dst, src *Config) {
	// Mode/Layout (check string version since enum zero is valid)
	if src.ModeString != "" {
		dst.Mode = src.Mode
		dst.ModeString = src.ModeString
	}
	if src.LayoutString != "" {
		dst.Layout = src.Layout
		dst.LayoutString = src.LayoutString
	}

	// Columns (only override if explicitly set)
	if len(src.ColumnsStrings) > 0 {
		dst.Columns = src.Columns
		dst.ColumnsStrings = src.ColumnsStrings
	}

	// Column widths (only override non-zero values)
	if src.Widths.Name > 0 {
		dst.Widths.Name = src.Widths.Name
	}
	if src.Widths.Preview > 0 {
		dst.Widths.Preview = src.Widths.Preview
	}
	if src.Widths.Modified > 0 {
		dst.Widths.Modified = src.Widths.Modified
	}
	if src.Widths.Status > 0 {
		dst.Widths.Status = src.Widths.Status
	}
	if src.Widths.Project > 0 {
		dst.Widths.Project = src.Widths.Project
	}
	if src.Widths.Messages > 0 {
		dst.Widths.Messages = src.Widths.Messages
	}
	if src.Widths.Turns > 0 {
		dst.Widths.Turns = src.Widths.Turns
	}
	if src.Widths.Model > 0 {
		dst.Widths.Model = src.Widths.Model
	}

	// SidePanel (merge individual fields)
	// Note: booleans are tricky - we need explicit tracking for "set to false"
	// For now, we merge if any value suggests the section was configured
	if src.SidePanel.MinWidth > 0 {
		dst.SidePanel.MinWidth = src.SidePanel.MinWidth
	}
	if src.SidePanel.Width > 0 {
		dst.SidePanel.Width = src.SidePanel.Width
	}
	// For booleans, we check if the entire struct appears configured
	// This is a limitation - explicit false vs unset can't be distinguished
	// The flags layer allows explicit control via pointers

	// Grid
	if src.Grid.CardWidth > 0 {
		dst.Grid.CardWidth = src.Grid.CardWidth
	}
	if src.Grid.CardHeight > 0 {
		dst.Grid.CardHeight = src.Grid.CardHeight
	}
	if src.Grid.Gap > 0 {
		dst.Grid.Gap = src.Grid.Gap
	}

	// Indicators
	if src.Indicators.Active != "" {
		dst.Indicators.Active = src.Indicators.Active
	}
	if src.Indicators.Idle != "" {
		dst.Indicators.Idle = src.Indicators.Idle
	}
	if src.Indicators.Exited != "" {
		dst.Indicators.Exited = src.Indicators.Exited
	}

	// Sort
	if src.Sort.Field != "" {
		dst.Sort.Field = src.Sort.Field
	}
	// Ascending is a boolean - same limitation as above

	// Filter (merge non-empty strings)
	if src.Filter.MaxAge != "" {
		dst.Filter.MaxAge = src.Filter.MaxAge
	}
	if src.Filter.MinAge != "" {
		dst.Filter.MinAge = src.Filter.MinAge
	}
	if src.Filter.Project != "" {
		dst.Filter.Project = src.Filter.Project
	}
	if src.Filter.SearchText != "" {
		dst.Filter.SearchText = src.Filter.SearchText
	}
	if src.Filter.Model != "" {
		dst.Filter.Model = src.Filter.Model
	}
	// ExcludeExited is a boolean - same limitation

	// Tmux
	if src.Tmux.NavigationMethod != "" {
		dst.Tmux.NavigationMethod = src.Tmux.NavigationMethod
	}
	// Enabled and HighlightRunning are booleans - same limitation

	// Cache
	if src.Cache.Dir != "" {
		dst.Cache.Dir = src.Cache.Dir
	}
	if src.Cache.Filename != "" {
		dst.Cache.Filename = src.Cache.Filename
	}
	// Enabled is a boolean - same limitation

	// Refresh
	if src.Refresh.IntervalSeconds > 0 {
		dst.Refresh.IntervalSeconds = src.Refresh.IntervalSeconds
		dst.Refresh.Interval = src.Refresh.Interval
	}
	// Enabled is a boolean - same limitation

	// Sessions
	if src.Sessions.ProjectsDir != "" {
		dst.Sessions.ProjectsDir = src.Sessions.ProjectsDir
	}
	if src.Sessions.MaxNameLength > 0 {
		dst.Sessions.MaxNameLength = src.Sessions.MaxNameLength
	}
	if src.Sessions.MaxPreviewLength > 0 {
		dst.Sessions.MaxPreviewLength = src.Sessions.MaxPreviewLength
	}
}

// mergeConfigWithRaw merges values from src into dst, using rawMap to detect
// which fields were explicitly set in the YAML. This allows proper handling
// of boolean fields that may be explicitly set to false.
func mergeConfigWithRaw(dst, src *Config, rawMap map[string]interface{}) {
	// Mode/Layout (check string version since enum zero is valid)
	if src.ModeString != "" {
		dst.Mode = src.Mode
		dst.ModeString = src.ModeString
	}
	if src.LayoutString != "" {
		dst.Layout = src.Layout
		dst.LayoutString = src.LayoutString
	}

	// Columns (only override if explicitly set)
	if len(src.ColumnsStrings) > 0 {
		dst.Columns = src.Columns
		dst.ColumnsStrings = src.ColumnsStrings
	}

	// Widths
	if widths, ok := rawMap["widths"].(map[string]interface{}); ok {
		if _, ok := widths["name"]; ok {
			dst.Widths.Name = src.Widths.Name
		}
		if _, ok := widths["preview"]; ok {
			dst.Widths.Preview = src.Widths.Preview
		}
		if _, ok := widths["modified"]; ok {
			dst.Widths.Modified = src.Widths.Modified
		}
		if _, ok := widths["status"]; ok {
			dst.Widths.Status = src.Widths.Status
		}
		if _, ok := widths["project"]; ok {
			dst.Widths.Project = src.Widths.Project
		}
		if _, ok := widths["messages"]; ok {
			dst.Widths.Messages = src.Widths.Messages
		}
		if _, ok := widths["turns"]; ok {
			dst.Widths.Turns = src.Widths.Turns
		}
		if _, ok := widths["model"]; ok {
			dst.Widths.Model = src.Widths.Model
		}
	}

	// SidePanel
	if sidePanel, ok := rawMap["side_panel"].(map[string]interface{}); ok {
		if _, ok := sidePanel["enabled"]; ok {
			dst.SidePanel.Enabled = src.SidePanel.Enabled
		}
		if _, ok := sidePanel["min_width"]; ok {
			dst.SidePanel.MinWidth = src.SidePanel.MinWidth
		}
		if _, ok := sidePanel["width"]; ok {
			dst.SidePanel.Width = src.SidePanel.Width
		}
	}

	// Grid
	if grid, ok := rawMap["grid"].(map[string]interface{}); ok {
		if _, ok := grid["card_width"]; ok {
			dst.Grid.CardWidth = src.Grid.CardWidth
		}
		if _, ok := grid["card_height"]; ok {
			dst.Grid.CardHeight = src.Grid.CardHeight
		}
		if _, ok := grid["gap"]; ok {
			dst.Grid.Gap = src.Grid.Gap
		}
	}

	// Indicators
	if indicators, ok := rawMap["indicators"].(map[string]interface{}); ok {
		if _, ok := indicators["active"]; ok {
			dst.Indicators.Active = src.Indicators.Active
		}
		if _, ok := indicators["idle"]; ok {
			dst.Indicators.Idle = src.Indicators.Idle
		}
		if _, ok := indicators["exited"]; ok {
			dst.Indicators.Exited = src.Indicators.Exited
		}
	}

	// Sort
	if sortMap, ok := rawMap["sort"].(map[string]interface{}); ok {
		if _, ok := sortMap["field"]; ok {
			dst.Sort.Field = src.Sort.Field
		}
		if _, ok := sortMap["ascending"]; ok {
			dst.Sort.Ascending = src.Sort.Ascending
		}
	}

	// Filter
	if filter, ok := rawMap["filter"].(map[string]interface{}); ok {
		if _, ok := filter["max_age"]; ok {
			dst.Filter.MaxAge = src.Filter.MaxAge
		}
		if _, ok := filter["min_age"]; ok {
			dst.Filter.MinAge = src.Filter.MinAge
		}
		if _, ok := filter["project"]; ok {
			dst.Filter.Project = src.Filter.Project
		}
		if _, ok := filter["exclude_exited"]; ok {
			dst.Filter.ExcludeExited = src.Filter.ExcludeExited
		}
		if _, ok := filter["search_text"]; ok {
			dst.Filter.SearchText = src.Filter.SearchText
		}
		if _, ok := filter["model"]; ok {
			dst.Filter.Model = src.Filter.Model
		}
	}

	// Tmux
	if tmuxMap, ok := rawMap["tmux"].(map[string]interface{}); ok {
		if _, ok := tmuxMap["enabled"]; ok {
			dst.Tmux.Enabled = src.Tmux.Enabled
		}
		if _, ok := tmuxMap["highlight_running"]; ok {
			dst.Tmux.HighlightRunning = src.Tmux.HighlightRunning
		}
		if _, ok := tmuxMap["navigation_method"]; ok {
			dst.Tmux.NavigationMethod = src.Tmux.NavigationMethod
		}
	}

	// Cache
	if cache, ok := rawMap["cache"].(map[string]interface{}); ok {
		if _, ok := cache["enabled"]; ok {
			dst.Cache.Enabled = src.Cache.Enabled
		}
		if _, ok := cache["dir"]; ok {
			dst.Cache.Dir = src.Cache.Dir
		}
		if _, ok := cache["filename"]; ok {
			dst.Cache.Filename = src.Cache.Filename
		}
	}

	// Refresh
	if refresh, ok := rawMap["refresh"].(map[string]interface{}); ok {
		if _, ok := refresh["enabled"]; ok {
			dst.Refresh.Enabled = src.Refresh.Enabled
		}
		if _, ok := refresh["interval"]; ok {
			dst.Refresh.IntervalSeconds = src.Refresh.IntervalSeconds
			dst.Refresh.Interval = src.Refresh.Interval
		}
	}

	// Sessions
	if sessions, ok := rawMap["sessions"].(map[string]interface{}); ok {
		if _, ok := sessions["projects_dir"]; ok {
			dst.Sessions.ProjectsDir = src.Sessions.ProjectsDir
		}
		if _, ok := sessions["max_name_length"]; ok {
			dst.Sessions.MaxNameLength = src.Sessions.MaxNameLength
		}
		if _, ok := sessions["max_preview_length"]; ok {
			dst.Sessions.MaxPreviewLength = src.Sessions.MaxPreviewLength
		}
	}
}

// validate checks the configuration for invalid values.
// Returns a ValidationError if any value is invalid.
func validate(cfg *Config) error {
	// Validate refresh interval
	if cfg.Refresh.Interval > 0 && cfg.Refresh.Interval < constants.MinRefreshInterval {
		return &ValidationError{
			Field: "refresh.interval",
			Err:   ErrInvalidRefreshInterval,
		}
	}

	// Validate column widths
	if cfg.Widths.Name < 0 {
		return &ValidationError{Field: "widths.name", Err: ErrInvalidColumnWidth}
	}
	if cfg.Widths.Preview < 0 {
		return &ValidationError{Field: "widths.preview", Err: ErrInvalidColumnWidth}
	}
	if cfg.Widths.Modified < 0 {
		return &ValidationError{Field: "widths.modified", Err: ErrInvalidColumnWidth}
	}
	if cfg.Widths.Status < 0 {
		return &ValidationError{Field: "widths.status", Err: ErrInvalidColumnWidth}
	}
	if cfg.Widths.Project < 0 {
		return &ValidationError{Field: "widths.project", Err: ErrInvalidColumnWidth}
	}
	if cfg.Widths.Messages < 0 {
		return &ValidationError{Field: "widths.messages", Err: ErrInvalidColumnWidth}
	}
	if cfg.Widths.Turns < 0 {
		return &ValidationError{Field: "widths.turns", Err: ErrInvalidColumnWidth}
	}
	if cfg.Widths.Model < 0 {
		return &ValidationError{Field: "widths.model", Err: ErrInvalidColumnWidth}
	}

	// Validate side panel
	if cfg.SidePanel.MinWidth < 0 {
		return &ValidationError{Field: "side_panel.min_width", Err: ErrInvalidSidePanelWidth}
	}

	// Validate grid card dimensions
	if cfg.Grid.CardWidth < 0 {
		return &ValidationError{Field: "grid.card_width", Err: ErrInvalidGridCardSize}
	}
	if cfg.Grid.CardHeight < 0 {
		return &ValidationError{Field: "grid.card_height", Err: ErrInvalidGridCardSize}
	}
	if cfg.Grid.Gap < 0 {
		return &ValidationError{Field: "grid.gap", Err: ErrInvalidGridCardSize}
	}

	// Validate navigation method
	if cfg.Tmux.NavigationMethod != "" &&
		cfg.Tmux.NavigationMethod != tmux.NavigationMethodSelectPane &&
		cfg.Tmux.NavigationMethod != tmux.NavigationMethodSwitchClient {
		return &ValidationError{
			Field: "tmux.navigation_method",
			Err:   ErrInvalidNavigationMethod,
		}
	}

	// Validate sessions config
	if cfg.Sessions.ProjectsDir == "" {
		return &ValidationError{Field: "sessions.projects_dir", Err: ErrEmptyProjectsDir}
	}
	if cfg.Sessions.MaxNameLength < 0 {
		return &ValidationError{Field: "sessions.max_name_length", Err: ErrInvalidMaxLength}
	}
	if cfg.Sessions.MaxPreviewLength < 0 {
		return &ValidationError{Field: "sessions.max_preview_length", Err: ErrInvalidMaxLength}
	}

	return nil
}

// LoadFromYAML parses YAML data into a Config.
// This is useful for testing or loading config from non-file sources.
func LoadFromYAML(data []byte) (*Config, error) {
	cfg := Defaults()

	if err := loadFromYAMLData(data, &cfg); err != nil {
		return nil, err
	}

	if err := validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// ConfigPath returns the path to the user's config file.
// If the config file doesn't exist, returns the default path where
// it would be created.
func ConfigPath() string {
	if path := findConfigFile(); path != "" {
		return path
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, constants.ConfigDir, ConfigFileName)
}
