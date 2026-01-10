// Package e2e provides end-to-end testing infrastructure for claude-dashboard.
// E2E tests require tmux and are opt-in via CLAUDE_E2E_TESTS=1 environment variable.
package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/GianlucaP106/gotmux/gotmux"
)

const (
	// E2EEnvVar is the environment variable that enables E2E tests.
	E2EEnvVar = "CLAUDE_E2E_TESTS"

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

// SkipIfE2EDisabled skips the test if E2E tests are not enabled.
func SkipIfE2EDisabled(t *testing.T) {
	t.Helper()
	if os.Getenv(E2EEnvVar) != "1" {
		t.Skipf("E2E tests disabled (set %s=1 to enable)", E2EEnvVar)
	}
}

// SkipIfNoTmux skips the test if tmux is not available.
func SkipIfNoTmux(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not available, skipping E2E test")
	}
}

// NewTestHarness creates a new test harness for E2E testing.
// It automatically skips the test if E2E is disabled or tmux is unavailable.
func NewTestHarness(t *testing.T) *TestHarness {
	t.Helper()
	SkipIfE2EDisabled(t)
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
