// Package tmux provides integration with tmux for detecting
// panes running Claude Code sessions.
package tmux

import (
	"errors"
	"os"
	"os/exec"

	"github.com/shitchell/claude-dashboard/internal/logging"
)

// ErrNotInTmux is returned when tmux commands are attempted
// but the TMUX environment variable is not set, indicating
// we are not running inside a tmux session.
var ErrNotInTmux = errors.New("not running inside tmux")

// ErrTmuxNotFound is returned when the tmux executable
// cannot be found in the system PATH.
var ErrTmuxNotFound = errors.New("tmux executable not found")

// CommandRunner is an interface for executing tmux commands.
// This allows for dependency injection in tests.
type CommandRunner interface {
	// Run executes a tmux command with the given arguments
	// and returns the combined stdout output.
	Run(args ...string) ([]byte, error)
}

// defaultRunner is the production implementation that executes
// actual tmux commands.
type defaultRunner struct{}

// Run executes a tmux command with the given arguments.
func (r *defaultRunner) Run(args ...string) ([]byte, error) {
	cmd := exec.Command("tmux", args...)
	return cmd.Output()
}

// runTmux executes a tmux command with the given arguments.
// It first checks that we are running inside tmux (TMUX env var set)
// and that the tmux executable exists.
//
// This is the low-level execution function used by higher-level
// pane discovery functions.
func runTmux(args ...string) ([]byte, error) {
	// Check if we're running inside tmux
	if os.Getenv("TMUX") == "" {
		logging.Debug("tmux: not running inside tmux (TMUX env var not set)")
		return nil, ErrNotInTmux
	}

	// Check if tmux is available
	if _, err := exec.LookPath("tmux"); err != nil {
		logging.Warn("tmux: executable not found in PATH")
		return nil, ErrTmuxNotFound
	}

	runner := &defaultRunner{}
	output, err := runner.Run(args...)
	if err != nil {
		logging.Debug("tmux command failed: tmux %v: %v", args, err)
	}
	return output, err
}

// runTmuxWithRunner executes a tmux command using the provided runner.
// This variant is used for testing with mock runners.
func runTmuxWithRunner(runner CommandRunner, args ...string) ([]byte, error) {
	// Check if we're running inside tmux
	if os.Getenv("TMUX") == "" {
		return nil, ErrNotInTmux
	}

	return runner.Run(args...)
}

// InTmux returns true if we are currently running inside a tmux session.
// This is determined by checking if the TMUX environment variable is set.
func InTmux() bool {
	return os.Getenv("TMUX") != ""
}
