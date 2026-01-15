// Package constants defines all magic numbers and configurable values
// used throughout the claude-dashboard application.
package constants

import "time"

// Application metadata.
const (
	// Version is the current application version.
	// This is displayed with the --version flag.
	Version = "0.1.0"

	// AppName is the application name used in output and error messages.
	AppName = "claude-dashboard"
)

// Parsing constants control how session data is parsed and displayed.
const (
	// MaxPreviewLength is the maximum number of characters to show
	// in session preview text.
	MaxPreviewLength = 80

	// MaxNameLength is the maximum number of characters for
	// session names in the UI.
	MaxNameLength = 40
)

// Cache constants control session caching behavior.
const (
	// CacheVersion is incremented when the cache format changes,
	// forcing a cache rebuild.
	CacheVersion = 1

	// CacheFileName is the name of the JSON cache file.
	CacheFileName = "sessions.json"
)

// PID cache constants control PID-to-session caching behavior.
const (
	// PIDCacheVersion is incremented when the PID cache format changes,
	// forcing a cache rebuild.
	PIDCacheVersion = 1

	// PIDCacheFileName is the name of the PID cache file.
	PIDCacheFileName = "pid-cache.json"

	// PIDCacheRuntimeDir is the subdirectory name in XDG_RUNTIME_DIR.
	PIDCacheRuntimeDir = "claude-dashboard"
)

// Refresh constants control how often session data is refreshed.
const (
	// DefaultRefreshInterval is the default time between automatic
	// session list refreshes.
	DefaultRefreshInterval = 5 * time.Second

	// MinRefreshInterval is the minimum allowed refresh interval
	// to prevent excessive disk reads.
	MinRefreshInterval = 1 * time.Second
)

// UI constants control layout and display defaults.
const (
	// MinSidePanelWidth is the minimum width in characters for
	// the side panel in grid view.
	MinSidePanelWidth = 20

	// DefaultColumnWidthName is the default width for the session
	// name column in list view.
	DefaultColumnWidthName = 20

	// DefaultColumnWidthPreview is the default width for the session
	// preview column in list view.
	DefaultColumnWidthPreview = 40

	// DefaultColumnWidthModified is the default width for the
	// modification time column in list view.
	DefaultColumnWidthModified = 10
)

// Path constants define default filesystem locations.
const (
	// ClaudeProjectsDir is the relative path from home directory
	// to Claude Code's project sessions directory.
	ClaudeProjectsDir = ".claude/projects"

	// ConfigDir is the relative path from home directory
	// to the configuration directory.
	ConfigDir = ".config/claude-dashboard"

	// CacheDir is the relative path from home directory
	// to the cache directory.
	CacheDir = ".cache/claude-dashboard"
)
