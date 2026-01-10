package helpers

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TmuxTestSession represents a test tmux session.
type TmuxTestSession struct {
	t           *testing.T
	Name        string
	WindowID    string
	PaneID      string
	StartDir    string
	cleanupFunc func()
}

// SkipIfNoTmux skips the test if tmux is not available.
func SkipIfNoTmux(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not available, skipping test")
	}
}

// SkipIfNotInTmux skips the test if not running inside a tmux session.
func SkipIfNotInTmux(t *testing.T) {
	t.Helper()
	if !IsInsideTmux() {
		t.Skip("Not running inside tmux, skipping test")
	}
}

// IsInsideTmux returns true if running inside a tmux session.
func IsInsideTmux() bool {
	cmd := exec.Command("printenv", "TMUX")
	output, err := cmd.Output()
	return err == nil && len(strings.TrimSpace(string(output))) > 0
}

// CreateTmuxTestSession creates a new tmux session for testing.
// The session is automatically cleaned up when the test completes.
func CreateTmuxTestSession(t *testing.T, name string) *TmuxTestSession {
	t.Helper()
	SkipIfNoTmux(t)

	// Make the session name unique to avoid conflicts
	uniqueName := fmt.Sprintf("test-%s-%d", name, time.Now().UnixNano())

	// Create a detached session
	cmd := exec.Command("tmux", "new-session", "-d", "-s", uniqueName, "-P", "-F", "#{session_name}")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to create tmux session %q: %v", uniqueName, err)
	}

	sessionName := strings.TrimSpace(string(output))

	// Get the window and pane IDs
	cmd = exec.Command("tmux", "list-panes", "-t", sessionName, "-F", "#{window_id}:#{pane_id}")
	output, err = cmd.Output()
	if err != nil {
		// Cleanup on error
		exec.Command("tmux", "kill-session", "-t", sessionName).Run()
		t.Fatalf("Failed to get pane info: %v", err)
	}

	parts := strings.Split(strings.TrimSpace(string(output)), ":")
	windowID := ""
	paneID := ""
	if len(parts) >= 2 {
		windowID = parts[0]
		paneID = parts[1]
	}

	session := &TmuxTestSession{
		t:        t,
		Name:     sessionName,
		WindowID: windowID,
		PaneID:   paneID,
		cleanupFunc: func() {
			exec.Command("tmux", "kill-session", "-t", sessionName).Run()
		},
	}

	// Register cleanup
	t.Cleanup(session.cleanupFunc)

	return session
}

// Kill terminates the test session.
func (s *TmuxTestSession) Kill() {
	if s.cleanupFunc != nil {
		s.cleanupFunc()
		s.cleanupFunc = nil
	}
}

// SendKeys sends keys to the session's pane.
func (s *TmuxTestSession) SendKeys(keys string) error {
	cmd := exec.Command("tmux", "send-keys", "-t", s.PaneID, keys)
	return cmd.Run()
}

// SendKeysEnter sends keys followed by Enter.
func (s *TmuxTestSession) SendKeysEnter(keys string) error {
	cmd := exec.Command("tmux", "send-keys", "-t", s.PaneID, keys, "Enter")
	return cmd.Run()
}

// CapturePane captures the current pane content.
func (s *TmuxTestSession) CapturePane() (string, error) {
	cmd := exec.Command("tmux", "capture-pane", "-t", s.PaneID, "-p")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// WaitForContent waits for specific content to appear in the pane.
func (s *TmuxTestSession) WaitForContent(content string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		output, err := s.CapturePane()
		if err == nil && strings.Contains(output, content) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// CreateNewPane creates a new pane and returns its ID.
func (s *TmuxTestSession) CreateNewPane() (string, error) {
	cmd := exec.Command("tmux", "split-window", "-t", s.Name, "-P", "-F", "#{pane_id}")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// RunCommand runs a command in the pane and waits for completion.
// This is useful for setup commands.
func (s *TmuxTestSession) RunCommand(command string) error {
	// Send the command
	if err := s.SendKeysEnter(command); err != nil {
		return err
	}

	// Wait a moment for the command to start
	time.Sleep(100 * time.Millisecond)
	return nil
}

// GetPaneProcesses returns the PIDs of processes running in the pane.
func (s *TmuxTestSession) GetPaneProcesses() ([]string, error) {
	cmd := exec.Command("tmux", "list-panes", "-t", s.PaneID, "-F", "#{pane_pid}")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var pids []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line != "" {
			pids = append(pids, line)
		}
	}
	return pids, nil
}

// MockTmuxRunner provides a mock implementation for tmux command testing.
type MockTmuxRunner struct {
	Calls     [][]string
	Responses map[string]MockResponse
	Default   MockResponse
}

// MockResponse represents a mock response from tmux.
type MockResponse struct {
	Output []byte
	Error  error
}

// NewMockTmuxRunner creates a new mock runner.
func NewMockTmuxRunner() *MockTmuxRunner {
	return &MockTmuxRunner{
		Calls:     make([][]string, 0),
		Responses: make(map[string]MockResponse),
	}
}

// Run implements the CommandRunner interface.
func (m *MockTmuxRunner) Run(args ...string) ([]byte, error) {
	m.Calls = append(m.Calls, args)

	// Look for a matching response
	key := strings.Join(args, " ")
	if resp, ok := m.Responses[key]; ok {
		return resp.Output, resp.Error
	}

	// Check for prefix matches (e.g., "list-panes" matches "list-panes -t %0")
	for pattern, resp := range m.Responses {
		if len(args) > 0 && args[0] == pattern {
			return resp.Output, resp.Error
		}
	}

	return m.Default.Output, m.Default.Error
}

// SetResponse sets a specific response for a command pattern.
func (m *MockTmuxRunner) SetResponse(command string, output []byte, err error) {
	m.Responses[command] = MockResponse{Output: output, Error: err}
}

// SetDefaultResponse sets the default response for unmatched commands.
func (m *MockTmuxRunner) SetDefaultResponse(output []byte, err error) {
	m.Default = MockResponse{Output: output, Error: err}
}

// GetCallCount returns how many times Run was called.
func (m *MockTmuxRunner) GetCallCount() int {
	return len(m.Calls)
}

// GetLastCall returns the arguments from the last call.
func (m *MockTmuxRunner) GetLastCall() []string {
	if len(m.Calls) == 0 {
		return nil
	}
	return m.Calls[len(m.Calls)-1]
}

// Reset clears all recorded calls.
func (m *MockTmuxRunner) Reset() {
	m.Calls = make([][]string, 0)
}
