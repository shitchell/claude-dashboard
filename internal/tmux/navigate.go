package tmux

import (
	"errors"
	"os/exec"
	"strings"

	"github.com/shitchell/claude-dashboard/internal/logging"
)

// PaneNavigator is an interface for navigating to tmux panes.
// This allows for dependency injection in tests and enables
// mocking navigation behavior without actually calling tmux.
type PaneNavigator interface {
	// GoToPane switches focus to the specified tmux pane.
	// The paneID should be in tmux format (e.g., "%0", "%5").
	GoToPane(paneID string) error

	// IsAvailable returns true if tmux navigation is available.
	IsAvailable() bool

	// Method returns the navigation method being used.
	Method() string
}

// Navigation method constants define how the Navigator switches to panes.
const (
	// NavigationMethodSelectPane uses tmux select-pane to switch directly.
	// This is the most common method and works within the same session.
	NavigationMethodSelectPane = "select-pane"

	// NavigationMethodSwitchClient uses tmux switch-client for cross-session
	// navigation. Useful when the target pane is in a different tmux session.
	NavigationMethodSwitchClient = "switch-client"

	// NavigationMethodDefault is the default navigation method.
	// switch-client is the default because it works for both same-session
	// and cross-session navigation. select-pane only works within the
	// same tmux session.
	NavigationMethodDefault = NavigationMethodSwitchClient
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

// Ensure Navigator implements PaneNavigator interface at compile time.
var _ PaneNavigator = (*Navigator)(nil)

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
// For switch-client, we find the client attached to the current session
// and switch it to the target pane's session/window.
//
// Returns ErrNotInTmux if not running inside tmux,
// ErrPaneNotFound if the pane doesn't exist,
// or ErrNavigationFailed if the tmux command fails.
func (n *Navigator) GoToPane(paneID string) error {
	logging.Debug("Navigating to tmux pane: %s (method=%s)", paneID, n.method)

	if paneID == "" {
		logging.Debug("GoToPane called with empty pane ID")
		return ErrPaneNotFound
	}

	var args []string
	switch n.method {
	case NavigationMethodSwitchClient:
		// switch-client switches a client to the target pane's session/window
		// We need to find the client attached to the current session
		client := n.findCurrentClient()
		if client != "" {
			logging.Debug("Found current client: %s", client)
			args = []string{"switch-client", "-c", client, "-t", paneID}
		} else {
			// Fall back to basic switch-client without -c
			logging.Debug("No client found, using basic switch-client")
			args = []string{"switch-client", "-t", paneID}
		}
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
			logging.Debug("tmux: pane not found: %s", paneID)
			return ErrPaneNotFound
		}
		logging.Warn("tmux: navigation failed to pane %s: %v", paneID, err)
		return ErrNavigationFailed
	}

	logging.Info("Successfully navigated to pane %s", paneID)
	return nil
}

// findCurrentClient finds the tmux client attached to the current session.
// Returns the client name (e.g., "/dev/pts/0") or empty string if not found.
func (n *Navigator) findCurrentClient() string {
	// First, get the current session name from our pane
	var sessionOutput []byte
	var err error
	if n.runner != nil {
		sessionOutput, err = runTmuxWithRunner(n.runner, "display-message", "-p", "#{session_name}")
	} else {
		sessionOutput, err = runTmux("display-message", "-p", "#{session_name}")
	}
	if err != nil {
		logging.Debug("Failed to get current session name: %v", err)
		return ""
	}
	currentSession := strings.TrimSpace(string(sessionOutput))
	logging.Debug("Current session: %s", currentSession)

	// Now list clients and find one attached to this session
	var clientsOutput []byte
	if n.runner != nil {
		clientsOutput, err = runTmuxWithRunner(n.runner, "list-clients", "-F", "#{client_name} #{session_name}")
	} else {
		clientsOutput, err = runTmux("list-clients", "-F", "#{client_name} #{session_name}")
	}
	if err != nil {
		logging.Debug("Failed to list clients: %v", err)
		return ""
	}

	// Parse output to find a client attached to current session
	lines := strings.Split(strings.TrimSpace(string(clientsOutput)), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 {
			clientName := parts[0]
			sessionName := parts[1]
			if sessionName == currentSession {
				logging.Debug("Found client %s attached to session %s", clientName, sessionName)
				return clientName
			}
		}
	}

	logging.Debug("No client found attached to session %s", currentSession)
	return ""
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
	logging.Debug("Resuming session %s in new pane (cwd=%s)", sessionID, cwd)

	if sessionID == "" {
		logging.Debug("ResumeInNewPane called with empty session ID")
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
		logging.Warn("tmux: failed to resume session %s in new pane: %v", sessionID, err)
		return err
	}

	logging.Info("tmux: resumed session %s in new pane", sessionID)
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

// Method returns the navigation method being used by this Navigator.
func (n *Navigator) Method() string {
	return n.method
}
