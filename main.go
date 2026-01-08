// Package main provides the entry point for claude-dashboard,
// a TUI application for browsing and managing Claude Code sessions.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shitchell/claude-dashboard/internal/config"
	"github.com/shitchell/claude-dashboard/internal/constants"
	"github.com/shitchell/claude-dashboard/internal/logging"
	"github.com/shitchell/claude-dashboard/internal/session"
	"github.com/shitchell/claude-dashboard/internal/ui"
)

// Exit codes for the application.
const (
	// ExitCodeSuccess indicates successful execution.
	ExitCodeSuccess = 0

	// ExitCodeConfigError indicates a configuration error.
	ExitCodeConfigError = 1

	// ExitCodeRuntimeError indicates a runtime error.
	ExitCodeRuntimeError = 2
)

func main() {
	exitCode := run(os.Args[1:])
	if exitCode != ExitCodeSuccess {
		os.Exit(exitCode)
	}
}

// run is the main entry point, separated for testability.
// It parses flags, loads configuration, and starts the TUI.
// Returns an exit code.
func run(args []string) int {
	// Initialize logging early (writes to ~/.cache/claude-dashboard/logs/)
	if err := logging.Init(nil); err != nil {
		fmt.Fprintf(os.Stderr, "%s: warning: failed to initialize logging: %v\n", constants.AppName, err)
		// Continue without logging - not fatal
	}
	defer logging.Close()

	logging.Info("Starting %s version %s", constants.AppName, constants.Version)

	// Parse command line flags
	flags, err := config.ParseFlags(args)
	if err != nil {
		// ParseFlags returns errors for unknown flags.
		// The flag package prints the error, so we just exit.
		return ExitCodeConfigError
	}

	// Handle --version flag
	if flags.Version {
		fmt.Printf("%s version %s\n", constants.AppName, constants.Version)
		return ExitCodeSuccess
	}

	// Handle --help flag
	// Note: The flag package already handles -h/--help by printing usage,
	// but we set the Help flag for explicit handling if needed.
	if flags.Help {
		printUsage()
		return ExitCodeSuccess
	}

	// Load configuration with priority: defaults <- file <- flags
	cfg, err := config.Load(flags.ConfigPath, flags)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: config error: %v\n", constants.AppName, err)
		return ExitCodeConfigError
	}

	// Create the session service with configuration from app config
	svcConfig := &session.ServiceConfig{
		ProjectsDir: cfg.Sessions.ProjectsDir,
		CacheDir:    cfg.Cache.Dir,
	}
	svc, err := session.NewService(svcConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: failed to create session service: %v\n", constants.AppName, err)
		return ExitCodeRuntimeError
	}

	// Create the UI model
	modelCfg := ui.ModelConfig{
		Service: svc,
		Config:  cfg,
	}
	if cfg.Refresh.Enabled {
		modelCfg.RefreshInterval = cfg.Refresh.Interval
	}
	model := ui.NewModel(modelCfg)

	// Create and run the bubbletea program
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: error: %v\n", constants.AppName, err)
		return ExitCodeRuntimeError
	}

	return ExitCodeSuccess
}

// printUsage prints the help message.
func printUsage() {
	fmt.Printf(`%s - A TUI for browsing Claude Code sessions

Usage:
  %s [options]

Options:
  -c, --config PATH      Path to config file
  -m, --mode MODE        Display mode: list or grid
  -l, --layout LAYOUT    Layout type: list or grid
      --columns COLS     Comma-separated column list
      --refresh DURATION Refresh interval (e.g., 5s, 1m)
      --no-refresh       Disable auto-refresh
      --no-tmux          Disable tmux integration
      --no-cache         Disable session caching
      --sort FIELD       Sort by: modified, project, status, messages, turns, summary
      --asc              Sort in ascending order
  -p, --project NAME     Filter to project (matches path or name)
      --max-age DURATION Filter by age (e.g., 24h, 7d)
      --running          Show only running sessions (exclude exited)
  -v, --version          Show version and exit
  -h, --help             Show this help

Configuration:
  Config file is searched in: ~/.config/claude-dashboard/config.yaml

Examples:
  %s                     # Start with defaults
  %s -m grid             # Start in grid mode
  %s --sort project      # Sort by project name
  %s -p myproject        # Filter to sessions in myproject
  %s --max-age 24h       # Show sessions from last 24 hours
`,
		constants.AppName,
		constants.AppName,
		constants.AppName,
		constants.AppName,
		constants.AppName,
		constants.AppName,
		constants.AppName,
	)
}
