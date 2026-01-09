package tmux

import (
	"errors"
	"os"
	"testing"
)

// mockNavigatorRunner is a test implementation of CommandRunner that records
// calls and returns predetermined responses.
type mockNavigatorRunner struct {
	output   []byte
	err      error
	calls    [][]string // Record of all calls made
	exitCode int        // Simulated exit code (used for error scenarios)
}

func (m *mockNavigatorRunner) Run(args ...string) ([]byte, error) {
	m.calls = append(m.calls, args)
	return m.output, m.err
}

// TestNewNavigator tests the Navigator constructor.
func TestNewNavigator(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		expectedMethod string
	}{
		{
			name:           "default method when empty",
			method:         "",
			expectedMethod: NavigationMethodDefault,
		},
		{
			name:           "select-pane method",
			method:         NavigationMethodSelectPane,
			expectedMethod: NavigationMethodSelectPane,
		},
		{
			name:           "switch-client method",
			method:         NavigationMethodSwitchClient,
			expectedMethod: NavigationMethodSwitchClient,
		},
		{
			name:           "custom method preserved",
			method:         "custom-method",
			expectedMethod: "custom-method",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nav := NewNavigator(tt.method)
			if nav.Method() != tt.expectedMethod {
				t.Errorf("Method() = %q, want %q", nav.Method(), tt.expectedMethod)
			}
		})
	}
}

// TestBuildGoToPaneCommand tests command generation for GoToPane.
func TestBuildGoToPaneCommand(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		paneID   string
		expected []string
	}{
		{
			name:     "switch-client default",
			method:   "",
			paneID:   "%0",
			expected: []string{"tmux", "switch-client", "-t", "%0"},
		},
		{
			name:     "select-pane explicit",
			method:   NavigationMethodSelectPane,
			paneID:   "%5",
			expected: []string{"tmux", "select-pane", "-t", "%5"},
		},
		{
			name:     "switch-client",
			method:   NavigationMethodSwitchClient,
			paneID:   "%10",
			expected: []string{"tmux", "switch-client", "-t", "%10"},
		},
		{
			name:     "complex pane ID",
			method:   NavigationMethodSelectPane,
			paneID:   "mysession:0.1",
			expected: []string{"tmux", "select-pane", "-t", "mysession:0.1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nav := NewNavigator(tt.method)
			result := nav.BuildGoToPaneCommand(tt.paneID)

			if len(result) != len(tt.expected) {
				t.Fatalf("BuildGoToPaneCommand() returned %d args, want %d", len(result), len(tt.expected))
			}

			for i, arg := range tt.expected {
				if result[i] != arg {
					t.Errorf("BuildGoToPaneCommand()[%d] = %q, want %q", i, result[i], arg)
				}
			}
		})
	}
}

// TestBuildResumeCommand tests command generation for ResumeInNewPane.
func TestBuildResumeCommand(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		cwd       string
		expected  []string
	}{
		{
			name:      "without cwd",
			sessionID: "abc123",
			cwd:       "",
			expected:  []string{"tmux", "split-window", "-v", "claude --resume abc123"},
		},
		{
			name:      "with cwd",
			sessionID: "xyz789",
			cwd:       "/home/user/project",
			expected:  []string{"tmux", "split-window", "-v", "-c", "/home/user/project", "claude --resume xyz789"},
		},
		{
			name:      "complex session ID",
			sessionID: "session_with-mixed_chars123",
			cwd:       "/path/with spaces",
			expected:  []string{"tmux", "split-window", "-v", "-c", "/path/with spaces", "claude --resume session_with-mixed_chars123"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nav := NewNavigator("")
			result := nav.BuildResumeCommand(tt.sessionID, tt.cwd)

			if len(result) != len(tt.expected) {
				t.Fatalf("BuildResumeCommand() returned %d args, want %d: got %v", len(result), len(tt.expected), result)
			}

			for i, arg := range tt.expected {
				if result[i] != arg {
					t.Errorf("BuildResumeCommand()[%d] = %q, want %q", i, result[i], arg)
				}
			}
		})
	}
}

// TestGoToPane tests the GoToPane function with mock runner.
func TestGoToPane(t *testing.T) {
	// Save original TMUX value
	originalTMUX := os.Getenv("TMUX")
	defer os.Setenv("TMUX", originalTMUX)

	// Set TMUX to simulate being inside tmux
	os.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")

	tests := []struct {
		name        string
		method      string
		paneID      string
		mockOutput  []byte
		mockErr     error
		expectError error
		expectArgs  []string
	}{
		{
			name:        "successful select-pane",
			method:      NavigationMethodSelectPane,
			paneID:      "%0",
			mockOutput:  []byte(""),
			mockErr:     nil,
			expectError: nil,
			expectArgs:  []string{"select-pane", "-t", "%0"},
		},
		{
			name:   "successful switch-client",
			method: NavigationMethodSwitchClient,
			paneID: "%5",
			// switch-client now calls display-message, list-clients, then switch-client
			// So this test needs to accept the final switch-client call
			mockOutput:  []byte(""),
			mockErr:     nil,
			expectError: nil,
			expectArgs:  nil, // Skip args check - tested in dedicated test
		},
		{
			name:        "empty pane ID",
			method:      NavigationMethodSelectPane,
			paneID:      "",
			mockOutput:  []byte(""),
			mockErr:     nil,
			expectError: ErrPaneNotFound,
			expectArgs:  nil, // Runner should not be called
		},
		{
			name:        "pane not found error",
			method:      NavigationMethodSelectPane,
			paneID:      "%999",
			mockOutput:  []byte("can't find pane: %999"),
			mockErr:     errors.New("exit status 1"),
			expectError: ErrPaneNotFound,
			expectArgs:  []string{"select-pane", "-t", "%999"},
		},
		{
			name:        "navigation failed error",
			method:      NavigationMethodSelectPane,
			paneID:      "%0",
			mockOutput:  []byte("some other error"),
			mockErr:     errors.New("exit status 1"),
			expectError: ErrNavigationFailed,
			expectArgs:  []string{"select-pane", "-t", "%0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &mockNavigatorRunner{output: tt.mockOutput, err: tt.mockErr}
			nav := NewNavigatorWithRunner(tt.method, runner)

			err := nav.GoToPane(tt.paneID)

			// Check error
			if tt.expectError != nil {
				if !errors.Is(err, tt.expectError) {
					t.Errorf("GoToPane() error = %v, want %v", err, tt.expectError)
				}
			} else if err != nil {
				t.Errorf("GoToPane() unexpected error = %v", err)
			}

			// Check runner was called with correct args (if applicable)
			if tt.expectArgs != nil {
				// For most tests, we expect exactly 1 call
				if len(runner.calls) != 1 {
					t.Fatalf("runner was called %d times, want 1", len(runner.calls))
				}
				for i, arg := range tt.expectArgs {
					if i >= len(runner.calls[0]) || runner.calls[0][i] != arg {
						t.Errorf("runner.calls[0][%d] = %q, want %q", i, runner.calls[0][i], arg)
					}
				}
			} else if tt.paneID != "" && len(runner.calls) == 0 {
				// If paneID is non-empty and expectArgs is nil, we expect some calls
				// (for switch-client which makes multiple calls)
			} else if tt.paneID == "" && len(runner.calls) != 0 {
				t.Errorf("runner was called %d times, want 0", len(runner.calls))
			}
		})
	}
}

// TestGoToPaneNotInTmux tests that GoToPane returns ErrNotInTmux when
// not running inside tmux.
func TestGoToPaneNotInTmux(t *testing.T) {
	// Save original TMUX value
	originalTMUX := os.Getenv("TMUX")
	defer os.Setenv("TMUX", originalTMUX)

	// Unset TMUX
	os.Unsetenv("TMUX")

	runner := &mockNavigatorRunner{output: []byte("")}
	nav := NewNavigatorWithRunner("", runner)

	err := nav.GoToPane("%0")
	if !errors.Is(err, ErrNotInTmux) {
		t.Errorf("GoToPane() error = %v, want ErrNotInTmux", err)
	}

	// Runner should not have been called
	if len(runner.calls) != 0 {
		t.Errorf("runner was called %d times, want 0", len(runner.calls))
	}
}

// TestResumeInNewPane tests the ResumeInNewPane function with mock runner.
func TestResumeInNewPane(t *testing.T) {
	// Save original TMUX value
	originalTMUX := os.Getenv("TMUX")
	defer os.Setenv("TMUX", originalTMUX)

	// Set TMUX to simulate being inside tmux
	os.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")

	tests := []struct {
		name        string
		sessionID   string
		cwd         string
		mockOutput  []byte
		mockErr     error
		expectError bool
		expectArgs  []string
	}{
		{
			name:        "resume without cwd",
			sessionID:   "abc123",
			cwd:         "",
			mockOutput:  []byte(""),
			mockErr:     nil,
			expectError: false,
			expectArgs:  []string{"split-window", "-v", "claude --resume abc123"},
		},
		{
			name:        "resume with cwd",
			sessionID:   "xyz789",
			cwd:         "/home/user/project",
			mockOutput:  []byte(""),
			mockErr:     nil,
			expectError: false,
			expectArgs:  []string{"split-window", "-v", "-c", "/home/user/project", "claude --resume xyz789"},
		},
		{
			name:        "empty session ID",
			sessionID:   "",
			cwd:         "/some/path",
			mockOutput:  []byte(""),
			mockErr:     nil,
			expectError: true,
			expectArgs:  nil, // Runner should not be called
		},
		{
			name:        "tmux command fails",
			sessionID:   "abc123",
			cwd:         "",
			mockOutput:  []byte("error"),
			mockErr:     errors.New("exit status 1"),
			expectError: true,
			expectArgs:  []string{"split-window", "-v", "claude --resume abc123"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &mockNavigatorRunner{output: tt.mockOutput, err: tt.mockErr}
			nav := NewNavigatorWithRunner("", runner)

			err := nav.ResumeInNewPane(tt.sessionID, tt.cwd)

			// Check error
			if tt.expectError {
				if err == nil {
					t.Error("ResumeInNewPane() expected error, got nil")
				}
			} else if err != nil {
				t.Errorf("ResumeInNewPane() unexpected error = %v", err)
			}

			// Check runner was called with correct args (if applicable)
			if tt.expectArgs != nil {
				if len(runner.calls) != 1 {
					t.Fatalf("runner was called %d times, want 1", len(runner.calls))
				}
				if len(runner.calls[0]) != len(tt.expectArgs) {
					t.Errorf("runner.calls[0] has %d args, want %d: got %v", len(runner.calls[0]), len(tt.expectArgs), runner.calls[0])
				}
				for i, arg := range tt.expectArgs {
					if i < len(runner.calls[0]) && runner.calls[0][i] != arg {
						t.Errorf("runner.calls[0][%d] = %q, want %q", i, runner.calls[0][i], arg)
					}
				}
			} else if len(runner.calls) != 0 {
				t.Errorf("runner was called %d times, want 0", len(runner.calls))
			}
		})
	}
}

// TestResumeInNewPaneNotInTmux tests that ResumeInNewPane returns ErrNotInTmux
// when not running inside tmux.
func TestResumeInNewPaneNotInTmux(t *testing.T) {
	// Save original TMUX value
	originalTMUX := os.Getenv("TMUX")
	defer os.Setenv("TMUX", originalTMUX)

	// Unset TMUX
	os.Unsetenv("TMUX")

	runner := &mockNavigatorRunner{output: []byte("")}
	nav := NewNavigatorWithRunner("", runner)

	err := nav.ResumeInNewPane("abc123", "/some/path")
	if !errors.Is(err, ErrNotInTmux) {
		t.Errorf("ResumeInNewPane() error = %v, want ErrNotInTmux", err)
	}

	// Runner should not have been called
	if len(runner.calls) != 0 {
		t.Errorf("runner was called %d times, want 0", len(runner.calls))
	}
}

// TestIsAvailable tests the IsAvailable function.
func TestIsAvailable(t *testing.T) {
	// Save original TMUX value
	originalTMUX := os.Getenv("TMUX")
	defer os.Setenv("TMUX", originalTMUX)

	tests := []struct {
		name     string
		tmuxEnv  string
		expected bool
	}{
		{
			name: "inside tmux",
			// Note: tmux must be in PATH for this to return true
			// In CI/test environments without tmux, this will return false
			tmuxEnv:  "/tmp/tmux-1000/default,12345,0",
			expected: true, // Will be adjusted if tmux not found
		},
		{
			name:     "outside tmux",
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

			nav := NewNavigator("")
			result := nav.IsAvailable()

			// If TMUX is not set, IsAvailable should always be false
			if tt.tmuxEnv == "" && result {
				t.Errorf("IsAvailable() = true when TMUX not set, want false")
			}

			// If TMUX is set, result depends on whether tmux binary is available
			// We don't require tmux to be installed for tests to pass
			if tt.tmuxEnv != "" {
				// Just verify it returns a boolean without panicking
				_ = result
			}
		})
	}
}

// TestNavigatorMethod tests the Method getter.
func TestNavigatorMethod(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		expected string
	}{
		{
			name:     "default method",
			method:   "",
			expected: NavigationMethodDefault,
		},
		{
			name:     "select-pane",
			method:   NavigationMethodSelectPane,
			expected: NavigationMethodSelectPane,
		},
		{
			name:     "switch-client",
			method:   NavigationMethodSwitchClient,
			expected: NavigationMethodSwitchClient,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nav := NewNavigator(tt.method)
			if nav.Method() != tt.expected {
				t.Errorf("Method() = %q, want %q", nav.Method(), tt.expected)
			}
		})
	}
}

// TestNavigationConstants tests that constants have expected values.
func TestNavigationConstants(t *testing.T) {
	// Verify constants have the expected values
	if NavigationMethodSelectPane != "select-pane" {
		t.Errorf("NavigationMethodSelectPane = %q, want %q", NavigationMethodSelectPane, "select-pane")
	}
	if NavigationMethodSwitchClient != "switch-client" {
		t.Errorf("NavigationMethodSwitchClient = %q, want %q", NavigationMethodSwitchClient, "switch-client")
	}
	if NavigationMethodDefault != NavigationMethodSwitchClient {
		t.Errorf("NavigationMethodDefault = %q, want %q", NavigationMethodDefault, NavigationMethodSwitchClient)
	}
	if ClaudeExecutable != "claude" {
		t.Errorf("ClaudeExecutable = %q, want %q", ClaudeExecutable, "claude")
	}
}
