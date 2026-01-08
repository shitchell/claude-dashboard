package tmux

import (
	"errors"
	"os"
	"testing"
)

// mockRunner is a test implementation of CommandRunner that returns
// predetermined output or errors.
type mockRunner struct {
	output []byte
	err    error
	calls  [][]string // Record of all calls made
}

func (m *mockRunner) Run(args ...string) ([]byte, error) {
	m.calls = append(m.calls, args)
	return m.output, m.err
}

// TestInTmux tests the InTmux function.
func TestInTmux(t *testing.T) {
	// Save original TMUX value
	originalTMUX := os.Getenv("TMUX")
	defer os.Setenv("TMUX", originalTMUX)

	tests := []struct {
		name     string
		tmuxEnv  string
		expected bool
	}{
		{
			name:     "TMUX set",
			tmuxEnv:  "/tmp/tmux-1000/default,12345,0",
			expected: true,
		},
		{
			name:     "TMUX empty",
			tmuxEnv:  "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.tmuxEnv == "" {
				os.Unsetenv("TMUX")
			} else {
				os.Setenv("TMUX", tt.tmuxEnv)
			}

			result := InTmux()
			if result != tt.expected {
				t.Errorf("InTmux() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestRunTmuxNotInTmux tests that runTmux returns ErrNotInTmux
// when the TMUX environment variable is not set.
func TestRunTmuxNotInTmux(t *testing.T) {
	// Save original TMUX value
	originalTMUX := os.Getenv("TMUX")
	defer os.Setenv("TMUX", originalTMUX)

	// Unset TMUX
	os.Unsetenv("TMUX")

	_, err := runTmux("list-panes")
	if !errors.Is(err, ErrNotInTmux) {
		t.Errorf("runTmux() error = %v, want ErrNotInTmux", err)
	}
}

// TestRunTmuxWithRunnerNotInTmux tests that runTmuxWithRunner returns
// ErrNotInTmux when the TMUX environment variable is not set.
func TestRunTmuxWithRunnerNotInTmux(t *testing.T) {
	// Save original TMUX value
	originalTMUX := os.Getenv("TMUX")
	defer os.Setenv("TMUX", originalTMUX)

	// Unset TMUX
	os.Unsetenv("TMUX")

	runner := &mockRunner{output: []byte("test")}
	_, err := runTmuxWithRunner(runner, "list-panes")
	if !errors.Is(err, ErrNotInTmux) {
		t.Errorf("runTmuxWithRunner() error = %v, want ErrNotInTmux", err)
	}

	// Runner should not have been called
	if len(runner.calls) != 0 {
		t.Errorf("runner was called %d times, want 0", len(runner.calls))
	}
}

// TestRunTmuxWithRunner tests the runTmuxWithRunner function with a mock.
func TestRunTmuxWithRunner(t *testing.T) {
	// Save original TMUX value
	originalTMUX := os.Getenv("TMUX")
	defer os.Setenv("TMUX", originalTMUX)

	// Set TMUX to simulate being inside tmux
	os.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")

	expectedOutput := []byte("test output")
	runner := &mockRunner{output: expectedOutput}

	output, err := runTmuxWithRunner(runner, "list-panes", "-a")
	if err != nil {
		t.Errorf("runTmuxWithRunner() error = %v", err)
	}

	if string(output) != string(expectedOutput) {
		t.Errorf("runTmuxWithRunner() output = %q, want %q", output, expectedOutput)
	}

	// Verify the runner was called with correct args
	if len(runner.calls) != 1 {
		t.Fatalf("runner was called %d times, want 1", len(runner.calls))
	}
	if len(runner.calls[0]) != 2 || runner.calls[0][0] != "list-panes" || runner.calls[0][1] != "-a" {
		t.Errorf("runner was called with %v, want [list-panes -a]", runner.calls[0])
	}
}

// TestRunTmuxWithRunnerError tests that errors from the runner are propagated.
func TestRunTmuxWithRunnerError(t *testing.T) {
	// Save original TMUX value
	originalTMUX := os.Getenv("TMUX")
	defer os.Setenv("TMUX", originalTMUX)

	// Set TMUX to simulate being inside tmux
	os.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")

	expectedErr := errors.New("tmux command failed")
	runner := &mockRunner{err: expectedErr}

	_, err := runTmuxWithRunner(runner, "list-panes")
	if err != expectedErr {
		t.Errorf("runTmuxWithRunner() error = %v, want %v", err, expectedErr)
	}
}

// TestPaneMapDiscoverWithMock tests the full Discover flow with mock output.
func TestPaneMapDiscoverWithMock(t *testing.T) {
	// Save original TMUX value
	originalTMUX := os.Getenv("TMUX")
	defer os.Setenv("TMUX", originalTMUX)

	// Set TMUX to simulate being inside tmux
	os.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")

	mockOutput := []byte(`/dev/pts/42	%0	dev	0	code	0	bash
/dev/pts/43	%1	dev	1	logs	0	tail
/dev/pts/44	%2	work	0	main	0	shell
`)

	runner := &mockRunner{output: mockOutput}
	pm := NewPaneMapWithRunner(runner)

	err := pm.Discover()
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	// Verify runner was called with correct format
	if len(runner.calls) != 1 {
		t.Fatalf("runner was called %d times, want 1", len(runner.calls))
	}
	expectedArgs := []string{"list-panes", "-a", "-F", TmuxListPanesFormat}
	if len(runner.calls[0]) != len(expectedArgs) {
		t.Errorf("runner was called with %d args, want %d", len(runner.calls[0]), len(expectedArgs))
	}
	for i, arg := range expectedArgs {
		if i < len(runner.calls[0]) && runner.calls[0][i] != arg {
			t.Errorf("runner.calls[0][%d] = %q, want %q", i, runner.calls[0][i], arg)
		}
	}

	// Verify panes were parsed correctly
	if pm.Count() != 3 {
		t.Errorf("Count() = %d, want 3", pm.Count())
	}

	// Test retrieval
	pane := pm.GetByTTY("pts/42")
	if pane == nil {
		t.Fatal("GetByTTY(pts/42) = nil")
	}
	if pane.SessionName != "dev" {
		t.Errorf("pane.SessionName = %q, want 'dev'", pane.SessionName)
	}

	pane = pm.GetByID("%2")
	if pane == nil {
		t.Fatal("GetByID(%2) = nil")
	}
	if pane.SessionName != "work" {
		t.Errorf("pane.SessionName = %q, want 'work'", pane.SessionName)
	}
}

// TestPaneMapDiscoverNotInTmux tests that Discover returns ErrNotInTmux
// when not running inside tmux.
func TestPaneMapDiscoverNotInTmux(t *testing.T) {
	// Save original TMUX value
	originalTMUX := os.Getenv("TMUX")
	defer os.Setenv("TMUX", originalTMUX)

	// Unset TMUX
	os.Unsetenv("TMUX")

	pm := NewPaneMap()
	err := pm.Discover()
	if !errors.Is(err, ErrNotInTmux) {
		t.Errorf("Discover() error = %v, want ErrNotInTmux", err)
	}
}

// TestPaneMapDiscoverWithRunnerNotInTmux tests that Discover with a custom
// runner still checks for TMUX environment variable.
func TestPaneMapDiscoverWithRunnerNotInTmux(t *testing.T) {
	// Save original TMUX value
	originalTMUX := os.Getenv("TMUX")
	defer os.Setenv("TMUX", originalTMUX)

	// Unset TMUX
	os.Unsetenv("TMUX")

	runner := &mockRunner{output: []byte("should not reach here")}
	pm := NewPaneMapWithRunner(runner)

	err := pm.Discover()
	if !errors.Is(err, ErrNotInTmux) {
		t.Errorf("Discover() error = %v, want ErrNotInTmux", err)
	}

	// Runner should not have been called
	if len(runner.calls) != 0 {
		t.Errorf("runner was called %d times, want 0", len(runner.calls))
	}
}

// TestPaneMapDiscoverError tests that errors from tmux are propagated.
func TestPaneMapDiscoverError(t *testing.T) {
	// Save original TMUX value
	originalTMUX := os.Getenv("TMUX")
	defer os.Setenv("TMUX", originalTMUX)

	// Set TMUX to simulate being inside tmux
	os.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")

	expectedErr := errors.New("tmux server not running")
	runner := &mockRunner{err: expectedErr}
	pm := NewPaneMapWithRunner(runner)

	err := pm.Discover()
	if err != expectedErr {
		t.Errorf("Discover() error = %v, want %v", err, expectedErr)
	}
}
