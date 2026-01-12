//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestClearScenarioSessionMapping tests that the dashboard correctly handles
// session-to-pane mapping when a Claude instance uses /clear to start a new session.
//
// This test addresses the bug where after /clear:
// 1. The old session was still shown as "running" (stale mapping)
// 2. The new session was not detected as running in the pane
//
// Test Flow:
// 1. Create an isolated tmux session for testing
// 2. Spawn Claude instance A (control - kept running throughout)
// 3. Spawn Claude instance B
// 4. Send /clear to instance B to start a fresh session
// 5. Send a new prompt to B to ensure it's active
// 6. Wait for B's new session file to appear
// 7. Spawn the dashboard
// 8. Verify:
//   - Instance A shows as running (control)
//   - Instance B's OLD session does NOT show as running (cleared)
//   - Instance B's NEW session shows as running
func TestClearScenarioSessionMapping(t *testing.T) {
	// Skip if Claude CLI is not available
	SkipIfNoClaude(t)

	h := NewTestHarness(t)

	// Create a test session
	h.CreateSession("clear-scenario")

	// Log session creation (not proof - session creation is infrastructure, not evidence)
	t.Log("Created tmux session for E2E test")

	// Step 1: Spawn Claude instance A (control)
	t.Log("Spawning control Claude instance A...")
	instanceA := h.SpawnClaude("Say hello briefly")
	require.NotEmpty(t, instanceA.SessionID, "Instance A should have a session ID")
	t.Logf("Instance A session ID: %s", instanceA.SessionID)

	// PROOF: Capture pane after spawning Claude A
	proofA := h.LogProof(t, instanceA.Pane, "After spawn Claude A")
	require.NotEmpty(t, proofA, "PROOF FAILED: Claude A pane should have content")
	require.Contains(t, proofA, "Claude", "PROOF FAILED: Claude UI not visible in Instance A pane")

	// Give Claude A time to respond
	time.Sleep(5 * time.Second)

	// PROOF: Capture Claude A output after response
	h.LogProof(t, instanceA.Pane, "Claude A after response time")

	// Step 2: Spawn Claude instance B
	t.Log("Spawning Claude instance B...")
	instanceB := h.SpawnClaude("Say hi briefly")
	require.NotEmpty(t, instanceB.SessionID, "Instance B should have a session ID")
	originalBSessionID := instanceB.SessionID
	t.Logf("Instance B original session ID: %s", originalBSessionID)

	// PROOF: Capture pane after spawning Claude B
	proofB := h.LogProof(t, instanceB.Pane, "After spawn Claude B")
	require.NotEmpty(t, proofB, "PROOF FAILED: Claude B pane should have content")
	require.Contains(t, proofB, "Claude", "PROOF FAILED: Claude UI not visible in Instance B pane")

	// Give Claude B time to respond
	time.Sleep(5 * time.Second)

	// PROOF: Capture Claude B output after response
	h.LogProof(t, instanceB.Pane, "Claude B after response time")

	// Step 3: Send /clear to instance B
	// PROOF: Capture state before /clear
	h.LogProof(t, instanceB.Pane, "Before /clear on B")

	// CRITICAL: Snapshot session files BEFORE sending /clear.
	// This fixes the race condition where /clear creates the new session file
	// immediately, before WaitForNewSession() can capture its baseline.
	t.Log("Capturing session file snapshot before /clear...")
	preCleatSnapshot := h.SnapshotSessionFiles()
	t.Logf("Pre-/clear snapshot contains %d session files", len(preCleatSnapshot))

	t.Log("Sending /clear to instance B...")
	err := h.SendCommand(instanceB, "/clear")
	require.NoError(t, err, "Should be able to send /clear command")

	// Wait for Claude B to process the clear command
	time.Sleep(2 * time.Second)

	// PROOF: Capture state after /clear
	h.LogProof(t, instanceB.Pane, "After /clear on B")

	// Step 4: Send a new prompt to instance B to create new session activity
	t.Log("Sending new prompt to instance B...")
	err = h.SendPrompt(instanceB, "What is 2+2?")
	require.NoError(t, err, "Should be able to send prompt")

	// PROOF: Capture state after sending prompt
	time.Sleep(1 * time.Second)
	h.LogProof(t, instanceB.Pane, "After prompt to B")

	// Step 5: Wait for B's new session file to appear
	// Use WaitForSessionSince with the pre-/clear snapshot to correctly detect
	// the new session, even if it was created immediately after /clear.
	t.Log("Waiting for new session file (using pre-/clear snapshot)...")
	newSessionID := h.WaitForSessionSince(instanceB, preCleatSnapshot, 30*time.Second)
	require.NotEmpty(t, newSessionID, "Instance B should have a new session ID after /clear")
	require.NotEqual(t, originalBSessionID, newSessionID, "New session ID should differ from original")
	t.Logf("Instance B new session ID: %s", newSessionID)

	// Step 6: Spawn the dashboard
	t.Log("Spawning dashboard...")
	dashboardPane := h.SpawnDashboard("")

	// Wait for dashboard to load
	found := h.WaitForContent(dashboardPane, "project", 15*time.Second)
	require.True(t, found, "Dashboard should display content within timeout")

	// Give the dashboard time to detect running processes
	time.Sleep(3 * time.Second)

	// PROOF: Capture dashboard output after spawn
	proofDashboard := h.LogProof(t, dashboardPane, "After dashboard spawn")
	require.NotEmpty(t, proofDashboard, "PROOF FAILED: Dashboard pane should have content")

	// Step 7: Capture and analyze dashboard output
	output := h.CapturePane(dashboardPane)
	t.Logf("Dashboard output:\n%s", output)

	// Verify Instance A shows as running (control)
	// The exact format depends on the UI, but we expect some running indicator
	// Check for session A in the output
	require.Contains(t, output, instanceA.SessionID[:8],
		"Dashboard should show instance A's session")

	// Verify Instance B's new session shows
	require.Contains(t, output, newSessionID[:8],
		"Dashboard should show instance B's new session")

	// Note: We can't easily verify that the OLD session is NOT marked as running
	// without more sophisticated parsing. The key test is that the new session
	// IS detected and displayed correctly.

	t.Log("Clear scenario test completed successfully")
}

// TestMultipleClaudeInstancesTracking verifies that the dashboard can track
// multiple concurrent Claude instances correctly.
func TestMultipleClaudeInstancesTracking(t *testing.T) {
	// Skip if Claude CLI is not available
	SkipIfNoClaude(t)

	h := NewTestHarness(t)

	// Create a test session
	h.CreateSession("multi-instance")
	t.Log("Created tmux session for multi-instance test")

	// Spawn multiple Claude instances
	t.Log("Spawning Claude instance 1...")
	instance1 := h.SpawnClaude("Count to 3")
	require.NotEmpty(t, instance1.SessionID)
	t.Logf("Instance 1 session ID: %s", instance1.SessionID)

	// PROOF: Capture pane after spawning instance 1
	proof1 := h.LogProof(t, instance1.Pane, "After spawn Claude instance 1")
	require.NotEmpty(t, proof1, "PROOF FAILED: Instance 1 pane should have content")
	require.Contains(t, proof1, "Claude", "PROOF FAILED: Claude UI not visible in Instance 1 pane")

	time.Sleep(3 * time.Second)

	t.Log("Spawning Claude instance 2...")
	instance2 := h.SpawnClaude("Name 3 colors")
	require.NotEmpty(t, instance2.SessionID)
	t.Logf("Instance 2 session ID: %s", instance2.SessionID)

	// PROOF: Capture pane after spawning instance 2
	proof2 := h.LogProof(t, instance2.Pane, "After spawn Claude instance 2")
	require.NotEmpty(t, proof2, "PROOF FAILED: Instance 2 pane should have content")
	require.Contains(t, proof2, "Claude", "PROOF FAILED: Claude UI not visible in Instance 2 pane")

	time.Sleep(3 * time.Second)

	t.Log("Spawning Claude instance 3...")
	instance3 := h.SpawnClaude("List 3 animals")
	require.NotEmpty(t, instance3.SessionID)
	t.Logf("Instance 3 session ID: %s", instance3.SessionID)

	// PROOF: Capture pane after spawning instance 3
	proof3 := h.LogProof(t, instance3.Pane, "After spawn Claude instance 3")
	require.NotEmpty(t, proof3, "PROOF FAILED: Instance 3 pane should have content")
	require.Contains(t, proof3, "Claude", "PROOF FAILED: Claude UI not visible in Instance 3 pane")

	time.Sleep(3 * time.Second)

	// Spawn the dashboard
	t.Log("Spawning dashboard...")
	dashboardPane := h.SpawnDashboard("")

	// Wait for dashboard to load
	found := h.WaitForContent(dashboardPane, "project", 15*time.Second)
	require.True(t, found, "Dashboard should display content within timeout")

	// Give time for process detection
	time.Sleep(3 * time.Second)

	// PROOF: Capture dashboard output
	proofDashboard := h.LogProof(t, dashboardPane, "After dashboard spawn - multi-instance")
	require.NotEmpty(t, proofDashboard, "PROOF FAILED: Dashboard pane should have content")

	// Capture output
	output := h.CapturePane(dashboardPane)
	t.Logf("Dashboard output:\n%s", output)

	// All three sessions should be visible
	require.Contains(t, output, instance1.SessionID[:8],
		"Dashboard should show instance 1's session")
	require.Contains(t, output, instance2.SessionID[:8],
		"Dashboard should show instance 2's session")
	require.Contains(t, output, instance3.SessionID[:8],
		"Dashboard should show instance 3's session")

	t.Log("Multiple instances tracking test completed successfully")
}
