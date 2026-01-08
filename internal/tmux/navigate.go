package tmux

import (
	"errors"
	"os/exec"
	"strings"
)

// Navigation method constants define how the Navigator switches to panes.
const (
	// NavigationMethodSelectPane uses tmux select-pane to switch directly.
	// This is the most common method and works within the same session.
	NavigationMethodSelectPane = "select-pane"

	// NavigationMethodSwitchClient uses tmux switch-client for cross-session
	// navigation. Useful when the target pane is in a different tmux session.
	NavigationMethodSwitchClient = "switch-client"

	// NavigationMethodDefault is the default navigation method.
	NavigationMethodDefault = NavigationMethodSelectPane
)

// Navigation error constants.
var (
	// ErrPaneNotFound is returned when the target pane ID does not exist.
	ErrPaneNotFound = errors.New("pane not found")

	// ErrNavigationFailed is returned when the tmux navigation command fails.
	ErrNavigationFailed = errors.New("navigation failed")

	// ErrClaudeNotFound is returned when the claude executable cannot be found.
	ErrClaudeNotFound = errors.New("claude executable not found")
)

// ClaudeExecutable is the name of the Claude CLI executable.
// This is used when resuming sessions in new panes.
const ClaudeExecutable = "claude"

// Navigator handles navigation to tmux panes and creating new panes
// for Claude sessions.
type Navigator struct {
	// method is the navigation method to use (select-pane or switch-client).
	method string

	// runner is the command runner for executing tmux commands.
	// If nil, the default runner is used.
	runner CommandRunner
}

// NewNavigator creates a new Navigator with the specified navigation method.
// If method is empty, NavigationMethodDefault is used.
func NewNavigator(method string) *Navigator {
	if method == "" {
		method = NavigationMethodDefault
	}
	return &Navigator{
		method: method,
	}
}

// NewNavigatorWithRunner creates a new Navigator with a custom command runner.
// This is primarily used for testing with mock tmux output.
func NewNavigatorWithRunner(method string, runner CommandRunner) *Navigator {
	if method == "" {
		method = NavigationMethodDefault
	}
	return &Navigator{
		method: method,
		runner: runner,
	}
}

// GoToPane switches focus to the specified tmux pane.
// The paneID should be in tmux format (e.g., "%0", "%5").
//
// The navigation method determines how the switch is performed:
//   - select-pane: Direct pane selection within current session
//   - switch-client: Client-level switch for cross-session navigation
//
// Returns ErrNotInTmux if not running inside tmux,
// ErrPaneNotFound if the pane doesn't exist,
// or ErrNavigationFailed if the tmux command fails.
func (n *Navigator) GoToPane(paneID string) error {
	if paneID == "" {
		return ErrPaneNotFound
	}

	var args []string
	switch n.method {
	case NavigationMethodSwitchClient:
		// switch-client switches the entire client to the target pane's window
		args = []string{"switch-client", "-t", paneID}
	default:
		// select-pane is the default - switches focus to the target pane
		args = []string{"select-pane", "-t", paneID}
	}

	var output []byte
	var err error
	if n.runner != nil {
		output, err = runTmuxWithRunner(n.runner, args...)
	} else {
		output, err = runTmux(args...)
	}

	if err != nil {
		// Preserve ErrNotInTmux and ErrTmuxNotFound errors
		if errors.Is(err, ErrNotInTmux) || errors.Is(err, ErrTmuxNotFound) {
			return err
		}

		// Check if error indicates pane not found
		// tmux returns "can't find pane" or similar messages
		outputStr := string(output)
		if strings.Contains(outputStr, "can't find") ||
			strings.Contains(outputStr, "no such") ||
			strings.Contains(outputStr, "not found") {
			return ErrPaneNotFound
		}
		return ErrNavigationFailed
	}

	return nil
}

// ResumeInNewPane creates a new tmux pane and starts a Claude session
// with the --resume flag for the given session ID.
//
// Parameters:
//   - sessionID: The Claude session ID to resume (required)
//   - cwd: The working directory for the new pane (optional; uses current if empty)
//
// The new pane is created by splitting the current pane and running
// the claude command with --resume in the specified directory.
//
// Returns ErrNotInTmux if not running inside tmux,
// ErrClaudeNotFound if the claude executable is not in PATH,
// or an error if the tmux command fails.
func (n *Navigator) ResumeInNewPane(sessionID, cwd string) error {
	if sessionID == "" {
		return errors.New("session ID is required")
	}

	// Build the claude command to run in the new pane
	claudeCmd := ClaudeExecutable + " --resume " + sessionID

	// Build tmux split-window command
	// -v: Split vertically (new pane below current)
	// -c: Start in the specified directory
	// Command to execute in the new pane
	var args []string
	if cwd != "" {
		args = []string{"split-window", "-v", "-c", cwd, claudeCmd}
	} else {
		args = []string{"split-window", "-v", claudeCmd}
	}

	var err error
	if n.runner != nil {
		_, err = runTmuxWithRunner(n.runner, args...)
	} else {
		_, err = runTmux(args...)
	}

	if err != nil {
		return err
	}

	return nil
}

// BuildGoToPaneCommand returns the tmux command arguments that would be
// used to navigate to the specified pane. This is useful for testing
// and for building command strings to display to users.
func (n *Navigator) BuildGoToPaneCommand(paneID string) []string {
	switch n.method {
	case NavigationMethodSwitchClient:
		return []string{"tmux", "switch-client", "-t", paneID}
	default:
		return []string{"tmux", "select-pane", "-t", paneID}
	}
}

// BuildResumeCommand returns the tmux command arguments that would be
// used to create a new pane and resume a session. This is useful for
// testing and for building command strings to display to users.
func (n *Navigator) BuildResumeCommand(sessionID, cwd string) []string {
	claudeCmd := ClaudeExecutable + " --resume " + sessionID

	if cwd != "" {
		return []string{"tmux", "split-window", "-v", "-c", cwd, claudeCmd}
	}
	return []string{"tmux", "split-window", "-v", claudeCmd}
}

// IsAvailable checks whether tmux navigation is available.
// This returns true if:
//   - We are running inside a tmux session (TMUX env var is set)
//   - The tmux executable is available in PATH
//
// This is a quick check that can be used to determine whether
// navigation features should be enabled in the UI.
func (n *Navigator) IsAvailable() bool {
	// Check if we're running inside tmux
	if !InTmux() {
		return false
	}

	// Check if tmux is available
	if _, err := exec.LookPath("tmux"); err != nil {
		return false
	}

	return true
}

// IsClaudeAvailable checks whether the Claude CLI is available in PATH.
// This is used to determine if session resume functionality is available.
func IsClaudeAvailable() bool {
	_, err := exec.LookPath(ClaudeExecutable)
	return err == nil
}

// Method returns the navigation method being used by this Navigator.
func (n *Navigator) Method() string {
	return n.method
}
