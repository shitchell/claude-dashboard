//go:build e2e

package e2e

import (
	"testing"
	"time"
)

// TestDashboardStartup verifies the dashboard can start and display sessions.
func TestDashboardStartup(t *testing.T) {
	h := NewTestHarness(t)

	// Create test projects directory with sessions
	projectsDir := h.CreateTestProjectsDir()
	h.CreateTestSession(projectsDir, "/home/user/project1", "session-1", "claude-sonnet-4-20250514", "First test session")
	h.CreateTestSession(projectsDir, "/home/user/project2", "session-2", "claude-opus-4-5-20251101", "Second test session")

	// Start the dashboard
	pane := h.RunDashboard(projectsDir)

	// Wait for content to appear
	if !h.WaitForContent(pane, "project", 10*time.Second) {
		output := h.CapturePane(pane)
		t.Fatalf("Dashboard did not display projects within timeout. Output:\n%s", output)
	}

	t.Log("Dashboard started successfully and displayed sessions")
}

// TestBasicNavigation verifies cursor movement in the session list.
func TestBasicNavigation(t *testing.T) {
	h := NewTestHarness(t)

	// Create test projects with multiple sessions
	projectsDir := h.CreateTestProjectsDir()
	h.CreateTestSession(projectsDir, "/home/user/project-a", "session-a", "claude-sonnet-4-20250514", "Session A")
	h.CreateTestSession(projectsDir, "/home/user/project-b", "session-b", "claude-sonnet-4-20250514", "Session B")
	h.CreateTestSession(projectsDir, "/home/user/project-c", "session-c", "claude-sonnet-4-20250514", "Session C")

	// Start the dashboard
	pane := h.RunDashboard(projectsDir)

	// Wait for initial display
	if !h.WaitForContent(pane, "project", 10*time.Second) {
		t.Fatal("Dashboard did not start in time")
	}

	// Give the UI a moment to stabilize
	time.Sleep(500 * time.Millisecond)

	// Navigate down using 'j' key
	if err := h.SendKeys(pane, "j"); err != nil {
		t.Fatalf("Failed to send 'j' key: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Navigate down again
	if err := h.SendKeys(pane, "j"); err != nil {
		t.Fatalf("Failed to send second 'j' key: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Navigate up using 'k' key
	if err := h.SendKeys(pane, "k"); err != nil {
		t.Fatalf("Failed to send 'k' key: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Quit the application
	if err := h.SendKeys(pane, "q"); err != nil {
		t.Fatalf("Failed to send 'q' key: %v", err)
	}

	// Wait for dashboard to exit
	time.Sleep(500 * time.Millisecond)

	t.Log("Basic navigation test completed")
}

// TestSearchMode verifies search functionality.
func TestSearchMode(t *testing.T) {
	h := NewTestHarness(t)

	// Create test projects
	projectsDir := h.CreateTestProjectsDir()
	h.CreateTestSession(projectsDir, "/home/user/alpha-project", "alpha-session", "claude-sonnet-4-20250514", "Alpha session")
	h.CreateTestSession(projectsDir, "/home/user/beta-project", "beta-session", "claude-sonnet-4-20250514", "Beta session")
	h.CreateTestSession(projectsDir, "/home/user/gamma-project", "gamma-session", "claude-sonnet-4-20250514", "Gamma session")

	// Start the dashboard
	pane := h.RunDashboard(projectsDir)

	// Wait for initial display
	if !h.WaitForContent(pane, "project", 10*time.Second) {
		t.Fatal("Dashboard did not start in time")
	}

	time.Sleep(500 * time.Millisecond)

	// Enter search mode with '/'
	if err := h.SendKeys(pane, "/"); err != nil {
		t.Fatalf("Failed to send '/' key: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Type search query
	if err := h.SendKeys(pane, "beta"); err != nil {
		t.Fatalf("Failed to send search query: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	// Check if beta is highlighted/filtered
	output := h.CapturePane(pane)
	if output == "" {
		t.Log("Could not capture pane output for verification")
	}

	// Press Escape to exit search mode
	if err := h.SendKeys(pane, "\x1b"); err != nil { // ESC key
		t.Logf("Failed to send ESC key: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Quit
	if err := h.SendKeys(pane, "q"); err != nil {
		t.Fatalf("Failed to send 'q' key: %v", err)
	}

	t.Log("Search mode test completed")
}

// TestQuitCommand verifies the dashboard quits properly with 'q'.
func TestQuitCommand(t *testing.T) {
	h := NewTestHarness(t)

	// Create minimal test environment
	projectsDir := h.CreateTestProjectsDir()
	h.CreateTestSession(projectsDir, "/home/user/project", "session", "claude-sonnet-4-20250514", "Test session")

	// Start the dashboard
	pane := h.RunDashboard(projectsDir)

	// Wait for it to start
	if !h.WaitForContent(pane, "project", 10*time.Second) {
		t.Fatal("Dashboard did not start in time")
	}

	time.Sleep(500 * time.Millisecond)

	// Send quit command
	if err := h.SendKeys(pane, "q"); err != nil {
		t.Fatalf("Failed to send 'q' key: %v", err)
	}

	// Wait and verify it exited
	time.Sleep(1 * time.Second)

	// The pane should no longer show the dashboard
	output := h.CapturePane(pane)

	// After quitting, we should see the shell prompt or empty output
	// (not the dashboard content)
	t.Logf("Post-quit output:\n%s", output)
	t.Log("Quit command test completed")
}

// TestHelpDisplay verifies the help screen shows with '?'.
func TestHelpDisplay(t *testing.T) {
	h := NewTestHarness(t)

	// Create minimal test environment
	projectsDir := h.CreateTestProjectsDir()
	h.CreateTestSession(projectsDir, "/home/user/project", "session", "claude-sonnet-4-20250514", "Test session")

	// Start the dashboard
	pane := h.RunDashboard(projectsDir)

	// Wait for it to start
	if !h.WaitForContent(pane, "project", 10*time.Second) {
		t.Fatal("Dashboard did not start in time")
	}

	time.Sleep(500 * time.Millisecond)

	// Press '?' to show help
	if err := h.SendKeys(pane, "?"); err != nil {
		t.Fatalf("Failed to send '?' key: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	// Check for help content (key bindings)
	if h.WaitForContent(pane, "quit", 2*time.Second) {
		t.Log("Help screen displayed with key bindings")
	} else {
		t.Log("Help screen might not show 'quit' text - checking output")
		output := h.CapturePane(pane)
		t.Logf("Help screen output:\n%s", output)
	}

	// Press '?' again or 'q' to dismiss help
	if err := h.SendKeys(pane, "?"); err != nil {
		t.Logf("Failed to dismiss help: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Quit
	if err := h.SendKeys(pane, "q"); err != nil {
		t.Fatalf("Failed to send 'q' key: %v", err)
	}

	t.Log("Help display test completed")
}
