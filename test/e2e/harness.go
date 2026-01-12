// Package e2e provides end-to-end testing infrastructure for claude-dashboard.
// E2E tests require tmux and are gated by the e2e build tag.
package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/GianlucaP106/gotmux/gotmux"
	"github.com/stretchr/testify/require"
)

const (
	// DefaultTimeout is the default timeout for E2E test operations.
	DefaultTimeout = 30 * time.Second

	// TestSessionPrefix is the prefix for test tmux sessions.
	TestSessionPrefix = "claude-dashboard-e2e-"
)

// TestHarness provides utilities for E2E testing.
type TestHarness struct {
	t            *testing.T
	tmux         *gotmux.Tmux
	session      *gotmux.Session
	testDir      string
	binaryPath   string
	cleanupFuncs []func()
}

// SkipIfNoTmux skips the test if tmux is not available.
func SkipIfNoTmux(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not available, skipping E2E test")
	}
}

// NewTestHarness creates a new test harness for E2E testing.
// It automatically skips the test if tmux is unavailable.
func NewTestHarness(t *testing.T) *TestHarness {
	t.Helper()
	SkipIfNoTmux(t)

	// Create tmux client
	tmuxClient, err := gotmux.DefaultTmux()
	if err != nil {
		t.Fatalf("Failed to create tmux client: %v", err)
	}

	// Create a temp directory for test fixtures
	testDir := t.TempDir()

	harness := &TestHarness{
		t:            t,
		tmux:         tmuxClient,
		testDir:      testDir,
		cleanupFuncs: make([]func(), 0),
	}

	// Register cleanup
	t.Cleanup(func() {
		harness.Cleanup()
	})

	return harness
}

// CreateSession creates a new tmux session for testing.
func (h *TestHarness) CreateSession(name string) *gotmux.Session {
	h.t.Helper()

	sessionName := fmt.Sprintf("%s%s-%d", TestSessionPrefix, name, time.Now().UnixNano())

	session, err := h.tmux.NewSession(&gotmux.SessionOptions{
		Name:           sessionName,
		StartDirectory: h.testDir,
	})
	if err != nil {
		h.t.Fatalf("Failed to create tmux session %q: %v", sessionName, err)
	}

	h.session = session
	h.addCleanup(func() {
		session.Kill()
	})

	return session
}

// GetSession returns the current test session.
func (h *TestHarness) GetSession() *gotmux.Session {
	return h.session
}

// GetTestDir returns the temporary test directory.
func (h *TestHarness) GetTestDir() string {
	return h.testDir
}

// SetBinaryPath sets the path to the claude-dashboard binary.
func (h *TestHarness) SetBinaryPath(path string) {
	h.binaryPath = path
}

// GetBinaryPath returns the path to the claude-dashboard binary.
// If not set, it attempts to find it in common locations.
func (h *TestHarness) GetBinaryPath() string {
	if h.binaryPath != "" {
		return h.binaryPath
	}

	// Try to find the binary
	candidates := []string{
		"./claude-dashboard",
		"../claude-dashboard",
		"../../claude-dashboard",
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			absPath, _ := filepath.Abs(candidate)
			h.binaryPath = absPath
			return absPath
		}
	}

	// Try to build it
	h.t.Log("Binary not found, attempting to build...")
	cmd := exec.Command("go", "build", "-o", filepath.Join(h.testDir, "claude-dashboard"), ".")
	if err := cmd.Run(); err != nil {
		h.t.Fatalf("Failed to build binary: %v", err)
	}

	h.binaryPath = filepath.Join(h.testDir, "claude-dashboard")
	return h.binaryPath
}

// CreateTestProjectsDir creates a mock .claude/projects directory structure.
func (h *TestHarness) CreateTestProjectsDir() string {
	h.t.Helper()

	projectsDir := filepath.Join(h.testDir, ".claude", "projects")
	if err := os.MkdirAll(projectsDir, 0755); err != nil {
		h.t.Fatalf("Failed to create projects dir: %v", err)
	}

	return projectsDir
}

// CreateTestSession creates a test session file in the projects directory.
func (h *TestHarness) CreateTestSession(projectsDir, projectPath, sessionID, model, summary string) string {
	h.t.Helper()

	// Encode project path
	encodedPath := strings.ReplaceAll(projectPath, "/", "-")
	projectDir := filepath.Join(projectsDir, encodedPath)

	if err := os.MkdirAll(projectDir, 0755); err != nil {
		h.t.Fatalf("Failed to create project dir: %v", err)
	}

	// Create session file
	sessionFile := filepath.Join(projectDir, sessionID+".jsonl")
	content := fmt.Sprintf(`{"type":"system","subtype":"init","uuid":"init","timestamp":"%s","sessionId":"%s","model":"%s","cwd":"%s"}
{"type":"user","uuid":"u1","parentUuid":null,"timestamp":"%s","message":{"role":"user","content":"Hello"}}
{"type":"assistant","uuid":"a1","parentUuid":"u1","timestamp":"%s","message":{"id":"msg_1","type":"message","model":"%s","role":"assistant","content":[{"type":"text","text":"Hello!"}]}}
{"type":"summary","summary":"%s","leafUuid":"a1"}
`,
		time.Now().Format(time.RFC3339),
		sessionID,
		model,
		projectPath,
		time.Now().Format(time.RFC3339),
		time.Now().Format(time.RFC3339),
		model,
		summary,
	)

	if err := os.WriteFile(sessionFile, []byte(content), 0644); err != nil {
		h.t.Fatalf("Failed to create session file: %v", err)
	}

	return sessionFile
}

// RunDashboard starts the dashboard in a pane and returns the pane.
func (h *TestHarness) RunDashboard(projectsDir string, args ...string) *gotmux.Pane {
	h.t.Helper()

	if h.session == nil {
		h.CreateSession("dashboard")
	}

	window, err := h.session.GetWindowByIndex(0)
	if err != nil {
		h.t.Fatalf("Failed to get window: %v", err)
	}

	pane, err := window.GetPaneByIndex(0)
	if err != nil {
		h.t.Fatalf("Failed to get pane: %v", err)
	}

	// Build command
	binary := h.GetBinaryPath()
	cmd := binary
	if projectsDir != "" {
		cmd = fmt.Sprintf("CLAUDE_PROJECTS_DIR=%s %s", projectsDir, binary)
	}
	for _, arg := range args {
		cmd += " " + arg
	}

	// Run the dashboard (append Enter to execute)
	if err := pane.SendKeys(cmd + " Enter"); err != nil {
		h.t.Fatalf("Failed to send command: %v", err)
	}

	return pane
}

// WaitForContent waits for specific content to appear in a pane.
func (h *TestHarness) WaitForContent(pane *gotmux.Pane, content string, timeout time.Duration) bool {
	h.t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			output, err := pane.Capture()
			if err != nil {
				continue
			}
			if strings.Contains(output, content) {
				return true
			}
		}
	}
}

// SendKeys sends keys to a pane.
func (h *TestHarness) SendKeys(pane *gotmux.Pane, keys string) error {
	return pane.SendKeys(keys)
}

// SendKeysEnter sends keys followed by Enter.
func (h *TestHarness) SendKeysEnter(pane *gotmux.Pane, keys string) error {
	return pane.SendKeys(keys + " Enter")
}

// CapturePane captures the current pane content.
func (h *TestHarness) CapturePane(pane *gotmux.Pane) string {
	h.t.Helper()

	output, err := pane.Capture()
	if err != nil {
		h.t.Logf("Warning: Failed to capture pane: %v", err)
		return ""
	}
	return output
}

// LogProof captures and logs pane output as observable proof of test activity.
// Returns the captured output for further assertions.
func (h *TestHarness) LogProof(t *testing.T, pane *gotmux.Pane, step string) string {
	t.Helper()
	output := h.CapturePane(pane)
	t.Logf("PROOF [%s]:\n%s", step, output)
	return output
}

// addCleanup adds a cleanup function to be called on harness cleanup.
func (h *TestHarness) addCleanup(f func()) {
	h.cleanupFuncs = append(h.cleanupFuncs, f)
}

// Cleanup runs all cleanup functions in reverse order.
func (h *TestHarness) Cleanup() {
	for i := len(h.cleanupFuncs) - 1; i >= 0; i-- {
		h.cleanupFuncs[i]()
	}
	h.cleanupFuncs = nil
}

// ClaudeInstance represents a spawned Claude Code process.
type ClaudeInstance struct {
	// Pane is the tmux pane where Claude is running.
	Pane *gotmux.Pane

	// SessionID is the Claude session ID detected from the session file after spawn.
	SessionID string

	// PID is the process ID of the Claude process.
	PID int

	// SessionFile is the path to the session's .jsonl file.
	SessionFile string
}

// SkipIfNoClaude skips the test if the claude CLI is not available.
func SkipIfNoClaude(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("claude CLI not available, skipping E2E test")
	}
}

// SpawnClaude spawns a Claude Code instance with the given prompt in a new window.
// It waits for Claude to start and detects the session ID from the session file.
// Returns a ClaudeInstance with pane, session ID, and PID.
func (h *TestHarness) SpawnClaude(prompt string) *ClaudeInstance {
	h.t.Helper()

	if h.session == nil {
		h.CreateSession("claude")
	}

	// Create a new window for the Claude instance
	window, err := h.session.NewWindow(&gotmux.NewWindowOptions{
		WindowName:     fmt.Sprintf("claude-%d", time.Now().UnixNano()),
		StartDirectory: h.testDir,
	})
	if err != nil {
		h.t.Fatalf("Failed to create window for Claude: %v", err)
	}

	pane, err := window.GetPaneByIndex(0)
	if err != nil {
		h.t.Fatalf("Failed to get pane for Claude: %v", err)
	}

	// Record session files before spawning
	existingFiles := h.listSessionFiles()

	// Build the claude command with prompt
	// Use -p for initial prompt (not --resume since this is a new session)
	cmd := fmt.Sprintf("claude -p %q", prompt)

	// Send the command to start Claude
	if err := pane.SendKeys(cmd + " Enter"); err != nil {
		h.t.Fatalf("Failed to send claude command: %v", err)
	}

	// Wait for a new session file to appear
	sessionFile := h.WaitForSessionFile(existingFiles, 30*time.Second)
	if sessionFile == "" {
		h.t.Fatalf("No new session file detected after spawning Claude")
	}

	// Extract session ID from the file path
	// Session files are named <session-id>.jsonl
	sessionID := strings.TrimSuffix(filepath.Base(sessionFile), ".jsonl")

	// Try to get the PID of the claude process
	pid := h.getClaudePID(pane)

	instance := &ClaudeInstance{
		Pane:        pane,
		SessionID:   sessionID,
		PID:         pid,
		SessionFile: sessionFile,
	}

	// Add cleanup to kill the Claude process when test ends
	h.addCleanup(func() {
		// Send Ctrl+C to stop Claude gracefully
		pane.SendKeys("C-c")
		time.Sleep(100 * time.Millisecond)
	})

	return instance
}

// listSessionFiles returns all .jsonl session files in the test's project directory.
func (h *TestHarness) listSessionFiles() []string {
	h.t.Helper()

	var files []string

	// Get the claude projects directory (either from env or default)
	projectsDir := os.Getenv("CLAUDE_PROJECTS_DIR")
	if projectsDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return files
		}
		projectsDir = filepath.Join(homeDir, ".claude", "projects")
	}

	// Walk through all project directories looking for .jsonl files
	filepath.Walk(projectsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(path, ".jsonl") {
			files = append(files, path)
		}
		return nil
	})

	return files
}

// WaitForSessionFile waits for a new session file to appear that wasn't in existingFiles.
// Returns the path to the new session file, or empty string if timeout.
func (h *TestHarness) WaitForSessionFile(existingFiles []string, timeout time.Duration) string {
	h.t.Helper()

	// Create a set of existing files for fast lookup
	existing := make(map[string]bool)
	for _, f := range existingFiles {
		existing[f] = true
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ""
		case <-ticker.C:
			currentFiles := h.listSessionFiles()
			for _, f := range currentFiles {
				if !existing[f] {
					// Found a new file - wait a moment for it to be written
					time.Sleep(100 * time.Millisecond)
					return f
				}
			}
		}
	}
}

// WaitForNewSession waits for a Claude instance to have a different session ID
// than the one it started with. This is useful for detecting /clear operations.
// Returns the new session ID, or empty string if timeout.
func (h *TestHarness) WaitForNewSession(instance *ClaudeInstance, timeout time.Duration) string {
	h.t.Helper()

	originalID := instance.SessionID
	existingFiles := h.listSessionFiles()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ""
		case <-ticker.C:
			currentFiles := h.listSessionFiles()
			for _, f := range currentFiles {
				// Check if this is a new file
				isNew := true
				for _, ef := range existingFiles {
					if f == ef {
						isNew = false
						break
					}
				}
				if isNew {
					sessionID := strings.TrimSuffix(filepath.Base(f), ".jsonl")
					if sessionID != originalID {
						// Update the instance with the new session info
						instance.SessionID = sessionID
						instance.SessionFile = f
						return sessionID
					}
				}
			}
		}
	}
}

// getClaudePID attempts to get the PID of the claude process in the given pane.
func (h *TestHarness) getClaudePID(pane *gotmux.Pane) int {
	// Use pgrep to find claude processes in this TTY
	// This is a best-effort attempt
	cmd := exec.Command("pgrep", "-f", "claude")
	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	// Return the first PID found (simplistic approach)
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) > 0 {
		var pid int
		fmt.Sscanf(lines[0], "%d", &pid)
		return pid
	}
	return 0
}

// SpawnDashboard starts the dashboard in a new window and returns the pane.
// This is similar to RunDashboard but creates a new window instead of using window 0.
func (h *TestHarness) SpawnDashboard(projectsDir string, args ...string) *gotmux.Pane {
	h.t.Helper()

	if h.session == nil {
		h.CreateSession("dashboard")
	}

	// Create a new window for the dashboard
	window, err := h.session.NewWindow(&gotmux.NewWindowOptions{
		WindowName:     "dashboard",
		StartDirectory: h.testDir,
	})
	if err != nil {
		h.t.Fatalf("Failed to create window for dashboard: %v", err)
	}

	pane, err := window.GetPaneByIndex(0)
	if err != nil {
		h.t.Fatalf("Failed to get pane for dashboard: %v", err)
	}

	// Build command
	binary := h.GetBinaryPath()
	cmd := binary
	if projectsDir != "" {
		cmd = fmt.Sprintf("CLAUDE_PROJECTS_DIR=%s %s", projectsDir, binary)
	}
	for _, arg := range args {
		cmd += " " + arg
	}

	// Run the dashboard
	if err := pane.SendKeys(cmd + " Enter"); err != nil {
		h.t.Fatalf("Failed to send dashboard command: %v", err)
	}

	return pane
}

// AssertSessionStatus verifies that a session appears in the dashboard output
// with the expected status indicator.
func (h *TestHarness) AssertSessionStatus(t *testing.T, pane *gotmux.Pane, sessionID string, expectedStatus string, timeout time.Duration) {
	t.Helper()

	// Build a regex pattern to match the session with its status
	// Status indicators are typically shown near the session ID
	pattern := regexp.MustCompile(fmt.Sprintf(`%s.*%s|%s.*%s`,
		regexp.QuoteMeta(sessionID[:8]), regexp.QuoteMeta(expectedStatus),
		regexp.QuoteMeta(expectedStatus), regexp.QuoteMeta(sessionID[:8])))

	require.Eventually(t, func() bool {
		output, err := pane.Capture()
		if err != nil {
			return false
		}
		return pattern.MatchString(output)
	}, timeout, 200*time.Millisecond,
		"Session %s should show status %q within %v", sessionID, expectedStatus, timeout)
}

// SendCommand sends a slash command (like /clear) to a Claude instance.
func (h *TestHarness) SendCommand(instance *ClaudeInstance, command string) error {
	h.t.Helper()

	// Send the command followed by Enter
	return instance.Pane.SendKeys(command + " Enter")
}

// SendPrompt sends a prompt to a Claude instance.
func (h *TestHarness) SendPrompt(instance *ClaudeInstance, prompt string) error {
	h.t.Helper()

	// Send the prompt followed by Enter
	return instance.Pane.SendKeys(prompt + " Enter")
}
