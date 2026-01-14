package tmux

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/shitchell/claude-dashboard/internal/logging"
	"github.com/shitchell/claude-dashboard/internal/session"
)

// ClaudeProcessRegex matches Claude Code commands in process listings.
// It matches both the bare "claude" command and common variants like
// "/path/to/claude", "node claude", etc.
var ClaudeProcessRegex = regexp.MustCompile(`(?:^|/)claude(?:\s|$)`)

// ResumeSessionIDRegex extracts the session ID from --resume arguments.
// Matches patterns like: --resume abc123, --resume=abc123, -r abc123, -r=abc123
var ResumeSessionIDRegex = regexp.MustCompile(`(?:--resume[=\s]|-r[=\s])([a-zA-Z0-9_-]+)`)

// Matcher coordinates process discovery and session-to-pane matching.
// It uses memory scanning as the primary method to map PIDs to sessions.
//
// The simplified flow:
//  1. Discover tmux panes (paneID -> Pane with TTY)
//  2. Discover Claude processes (PID -> Process with TTY)
//  3. Memory scan PIDs to find session IDs (PID -> sessionID)
//  4. Derive session-to-pane mapping on-the-fly via PID -> TTY -> pane
type Matcher struct {
	mu sync.RWMutex

	// refreshing indicates if a refresh is currently in progress.
	// This prevents concurrent refreshes which cause race conditions.
	refreshing bool

	// paneMap maps TTYs to tmux panes.
	paneMap *PaneMap

	// processList contains all running processes.
	processList *ProcessList

	// claudeProcesses contains only Claude-related processes.
	claudeProcesses []ClaudeProcess

	// pidToSessionID maps PIDs to session IDs (from memory scanning).
	// This is the primary source for session-to-pane matching.
	pidToSessionID map[int]string

	// sessionFilePaths holds the paths to all known session files.
	// Used for memory scanning to match PIDs to sessions.
	sessionFilePaths []string

	// processRunner is an optional custom runner for process commands.
	processRunner ProcessRunner

	// tmuxRunner is an optional custom runner for tmux commands.
	tmuxRunner CommandRunner

	// memoryScanner is an optional custom scanner for testing.
	// If nil, real memory scanning is used.
	memoryScanner MemoryScannerInterface
}

// NewMatcher creates a new Matcher with default runners.
func NewMatcher() *Matcher {
	return &Matcher{
		paneMap:          NewPaneMap(),
		processList:      NewProcessList(),
		claudeProcesses:  make([]ClaudeProcess, 0),
		pidToSessionID:   make(map[int]string),
		sessionFilePaths: make([]string, 0),
	}
}

// NewMatcherWithRunners creates a new Matcher with custom runners.
// This is primarily used for testing with mock output.
func NewMatcherWithRunners(processRunner ProcessRunner, tmuxRunner CommandRunner) *Matcher {
	return &Matcher{
		paneMap:          NewPaneMapWithRunner(tmuxRunner),
		processList:      NewProcessListWithRunner(processRunner),
		claudeProcesses:  make([]ClaudeProcess, 0),
		pidToSessionID:   make(map[int]string),
		sessionFilePaths: make([]string, 0),
		processRunner:    processRunner,
		tmuxRunner:       tmuxRunner,
	}
}

// MemoryScannerInterface allows for mocking memory scanning in tests.
type MemoryScannerInterface interface {
	// ScanAllPIDsForSessions returns PID -> sessionID mapping.
	ScanAllPIDsForSessions(pids []int, sessionPaths []string) map[int]string
}

// SetMemoryScanner sets a custom memory scanner (for testing).
func (m *Matcher) SetMemoryScanner(scanner MemoryScannerInterface) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.memoryScanner = scanner
}

// SetSessionFilePaths sets the session file paths used for memory scanning.
// This should be called before Refresh() with paths from the session scanner.
func (m *Matcher) SetSessionFilePaths(paths []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessionFilePaths = make([]string, len(paths))
	copy(m.sessionFilePaths, paths)
	logging.Debug("Set %d session file paths for memory scanning", len(paths))
}

// Refresh updates the matcher's state by querying tmux and process lists.
// This should be called periodically to keep the matcher up-to-date.
//
// The refresh process:
// 1. Discovers all tmux panes
// 2. Lists all running processes
// 3. Identifies Claude processes
// 4. Memory scans Claude PIDs to find session IDs
//
// Returns an error if either tmux or process discovery fails.
// Returns nil immediately if a refresh is already in progress.
func (m *Matcher) Refresh() error {
	// Check if refresh is already in progress
	m.mu.Lock()
	if m.refreshing {
		m.mu.Unlock()
		logging.Info("Matcher.Refresh: Skipping - refresh already in progress")
		return nil
	}
	m.refreshing = true
	m.mu.Unlock()

	// Ensure we clear the flag when done
	defer func() {
		m.mu.Lock()
		m.refreshing = false
		m.mu.Unlock()
	}()

	logging.Info("Matcher.Refresh: Starting refresh...")

	// Discover tmux panes
	logging.Debug("Matcher.Refresh: Discovering tmux panes...")
	if err := m.paneMap.Discover(); err != nil {
		// If not in tmux, we can still continue without pane info
		if err != ErrNotInTmux {
			logging.Warn("Matcher.Refresh: Failed to discover tmux panes: %v", err)
			return err
		}
		logging.Warn("Matcher.Refresh: Not running in tmux session, skipping pane discovery")
	} else {
		allPanes := m.paneMap.All()
		logging.Info("Matcher.Refresh: Discovered %d tmux panes", len(allPanes))
		for _, pane := range allPanes {
			logging.Debug("Matcher.Refresh:   Pane ID=%s TTY=%s", pane.ID, pane.TTY)
		}
	}

	// Discover all processes
	logging.Debug("Matcher.Refresh: Discovering processes...")
	if err := m.processList.Discover(); err != nil {
		logging.Warn("Matcher.Refresh: Failed to discover processes: %v", err)
		return err
	}
	allProcs := m.processList.All()
	logging.Info("Matcher.Refresh: Discovered %d total processes", len(allProcs))

	// Find Claude processes and match them to sessions via memory scanning
	logging.Debug("Matcher.Refresh: Finding and matching Claude processes...")
	m.findAndMatchClaudeProcesses()

	logging.Info("Matcher.Refresh: Complete. pidToSessionID=%d, claudeProcesses=%d",
		len(m.pidToSessionID), len(m.claudeProcesses))
	return nil
}

// findAndMatchClaudeProcesses identifies Claude processes and matches them to sessions.
func (m *Matcher) findAndMatchClaudeProcesses() {
	logging.Info("findAndMatchClaudeProcesses: ENTERING")
	m.mu.Lock()
	logging.Info("findAndMatchClaudeProcesses: GOT LOCK")
	defer m.mu.Unlock()

	// Clear previous state
	m.claudeProcesses = make([]ClaudeProcess, 0)
	m.pidToSessionID = make(map[int]string)

	runner := m.processRunner
	if runner == nil {
		runner = &defaultProcessRunner{}
	}

	allProcs := m.processList.All()
	logging.Debug("findAndMatchClaudeProcesses: Scanning %d processes for Claude commands", len(allProcs))

	// First pass: identify Claude processes and collect PIDs
	var claudePIDs []int
	for _, proc := range allProcs {
		if !IsClaudeProcess(proc.Command) {
			continue
		}

		logging.Debug("findAndMatchClaudeProcesses: Found Claude process PID=%d Command=%s TTY=%s",
			proc.PID, proc.Command, proc.TTY)

		claudeProc := ClaudeProcess{
			Process: proc,
		}

		// Try to get CWD for this process (useful for debugging)
		if cwd, err := runner.ReadCWD(proc.PID); err == nil {
			claudeProc.CWD = cwd
			logging.Debug("findAndMatchClaudeProcesses: PID=%d CWD=%s", proc.PID, cwd)
		}

		// Try to find the pane for this process via TTY
		pane := m.paneMap.GetByTTY(proc.TTY)
		if pane != nil {
			claudeProc.PaneID = pane.ID
			logging.Debug("findAndMatchClaudeProcesses: PID=%d matched to pane %s via TTY %s",
				proc.PID, pane.ID, proc.TTY)
		} else {
			logging.Debug("findAndMatchClaudeProcesses: PID=%d no pane found for TTY %s", proc.PID, proc.TTY)
		}

		m.claudeProcesses = append(m.claudeProcesses, claudeProc)
		claudePIDs = append(claudePIDs, proc.PID)
	}

	logging.Info("findAndMatchClaudeProcesses: Found %d Claude processes, %d session paths available",
		len(m.claudeProcesses), len(m.sessionFilePaths))

	// Second pass: memory scanning to match PIDs to sessions
	if len(m.sessionFilePaths) > 0 && len(claudePIDs) > 0 {
		logging.Info("findAndMatchClaudeProcesses: Starting memory scan for %d PIDs", len(claudePIDs))
		m.scanAndMatchSessions(claudePIDs)
		logging.Info("findAndMatchClaudeProcesses: Memory scan complete, pidToSessionID=%d", len(m.pidToSessionID))
	} else {
		logging.Info("findAndMatchClaudeProcesses: SKIPPING memory scan (sessionPaths=%d, claudePIDs=%d)",
			len(m.sessionFilePaths), len(claudePIDs))
	}

	// Third pass: update ClaudeProcess structs with session IDs from memory scan
	for i := range m.claudeProcesses {
		proc := &m.claudeProcesses[i]
		if sessionID, ok := m.pidToSessionID[proc.PID]; ok {
			proc.SessionID = sessionID
			logging.Debug("PID %d: session ID from memory scan: %s", proc.PID, sessionID)
		}
	}

	// Log any duplicate mappings (same pane mapped to multiple sessions)
	m.logDuplicateMappings()
}

// logDuplicateMappings logs a warning when multiple sessions are mapped to the same pane.
// This indicates a potential bug in the session-to-pane matching logic, typically caused
// by stale session data after operations like /clear that create new sessions.
func (m *Matcher) logDuplicateMappings() {
	// Build reverse mapping: pane ID -> list of session IDs from claudeProcesses
	paneToSessions := make(map[string][]string)
	for _, proc := range m.claudeProcesses {
		if proc.SessionID != "" && proc.PaneID != "" {
			paneToSessions[proc.PaneID] = append(paneToSessions[proc.PaneID], proc.SessionID)
		}
	}

	// Log any panes with multiple sessions
	for paneID, sessions := range paneToSessions {
		if len(sessions) > 1 {
			logging.Warn("BUG DETECTED: Pane %s mapped to multiple sessions: %v", paneID, sessions)
		}
	}
}

// scanAndMatchSessions performs memory scanning to match PIDs to session IDs.
// This is called with the mutex already held.
func (m *Matcher) scanAndMatchSessions(pids []int) {
	logging.Info("scanAndMatchSessions: Starting memory scan for %d PIDs with %d session paths",
		len(pids), len(m.sessionFilePaths))

	var pidToSession map[int]string

	if m.memoryScanner != nil {
		// Use mock scanner for testing
		pidToSession = m.memoryScanner.ScanAllPIDsForSessions(pids, m.sessionFilePaths)
	} else {
		// Use real memory scanning with NULL-prefix approach
		pidToSession = ScanAllPIDsForSessions(pids, m.sessionFilePaths)
	}

	// Store results
	for pid, sessionID := range pidToSession {
		m.pidToSessionID[pid] = sessionID
		logging.Info("scanAndMatchSessions: PID %d -> session %s", pid, sessionID)
	}

	logging.Info("scanAndMatchSessions: Complete. Matched %d PIDs to sessions", len(m.pidToSessionID))
}

// FindClaudeProcesses returns all detected Claude processes.
func (m *Matcher) FindClaudeProcesses() []ClaudeProcess {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]ClaudeProcess, len(m.claudeProcesses))
	copy(result, m.claudeProcesses)
	return result
}

// MatchSessionToPane finds the tmux pane where a session is running.
// Returns nil if no matching pane is found.
//
// Matching strategy:
// 1. Find PID that owns this session (from memory scanning)
// 2. Find pane via PID's TTY
//
// This is a direct lookup: sessionID -> PID -> TTY -> pane
func (m *Matcher) MatchSessionToPane(sess *session.Session) *Pane {
	if sess == nil {
		logging.Debug("MatchSessionToPane: session is nil")
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	logging.Debug("MatchSessionToPane: Looking for session ID=%s", sess.ID)

	// Find the PID that owns this session
	for pid, sessionID := range m.pidToSessionID {
		if sessionID == sess.ID {
			// Found the PID, now find the pane
			for _, proc := range m.claudeProcesses {
				if proc.PID == pid && proc.PaneID != "" {
					logging.Info("MatchSessionToPane: Matched session %s to pane %s via PID %d",
						sess.ID, proc.PaneID, pid)
					return m.paneMap.GetByID(proc.PaneID)
				}
			}
			// PID found but no pane (process not in tmux)
			logging.Debug("MatchSessionToPane: Session %s owned by PID %d but no pane found", sess.ID, pid)
			return nil
		}
	}

	logging.Debug("MatchSessionToPane: No PID found owning session %s", sess.ID)
	return nil
}

// HasRunningProcess returns true if there is a Claude process running
// that owns the given session (determined by memory scanning).
func (m *Matcher) HasRunningProcess(sess *session.Session) bool {
	if sess == nil {
		return false
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	logging.Info("HasRunningProcess: checking sess.ID=%s, pidToSessionID has %d entries", sess.ID, len(m.pidToSessionID))

	// Check if any PID owns this session
	for pid, sessionID := range m.pidToSessionID {
		if sessionID == sess.ID {
			logging.Info("HasRunningProcess: FOUND - PID %d owns session %s", pid, sess.ID)
			return true
		}
	}

	logging.Debug("HasRunningProcess: NOT FOUND - session %s not in pidToSessionID", sess.ID)
	return false
}

// GetPaneByID returns the pane with the given ID.
func (m *Matcher) GetPaneByID(id string) *Pane {
	return m.paneMap.GetByID(id)
}

// AllPanes returns all tmux panes.
func (m *Matcher) AllPanes() []*Pane {
	return m.paneMap.All()
}

// GetPIDToSessionID returns a copy of the PID to session ID mapping.
// This is populated by memory scanning and is primarily for debugging.
func (m *Matcher) GetPIDToSessionID() map[int]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[int]string)
	for k, v := range m.pidToSessionID {
		result[k] = v
	}
	return result
}

// IsClaudeProcess returns true if the command appears to be a Claude process.
func IsClaudeProcess(command string) bool {
	return ClaudeProcessRegex.MatchString(command)
}

// ExtractSessionID extracts the session ID from a command line if present.
// Returns empty string if no --resume or -r flag is found.
// Note: This is less reliable than memory scanning but kept for debugging.
func ExtractSessionID(command string) string {
	matches := ResumeSessionIDRegex.FindStringSubmatch(command)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

// resolveSymlinks resolves any symlinks in the path for consistent comparison.
// If resolution fails, returns the original path.
func resolveSymlinks(path string) string {
	// First clean the path
	path = filepath.Clean(path)

	// Try to resolve symlinks
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}
	return resolved
}

// NormalizePath normalizes a path for comparison by resolving symlinks
// and cleaning the path. This is useful for comparing paths that might
// be represented differently (e.g., /tmp vs /private/tmp on macOS).
func NormalizePath(path string) string {
	if path == "" {
		return ""
	}
	return resolveSymlinks(filepath.Clean(path))
}

// PathsEqual compares two paths after normalization.
// Returns true if they refer to the same location.
func PathsEqual(path1, path2 string) bool {
	if path1 == "" || path2 == "" {
		return false
	}
	return NormalizePath(path1) == NormalizePath(path2)
}

// FindProcessBySessionID searches for a Claude process with the given session ID.
// Returns nil if not found.
func (m *Matcher) FindProcessBySessionID(sessionID string) *ClaudeProcess {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for i := range m.claudeProcesses {
		if m.claudeProcesses[i].SessionID == sessionID {
			return &m.claudeProcesses[i]
		}
	}
	return nil
}

// FindProcessesByCWD searches for Claude processes with the given CWD.
// Returns all matching processes (there may be multiple).
func (m *Matcher) FindProcessesByCWD(cwd string) []ClaudeProcess {
	m.mu.RLock()
	defer m.mu.RUnlock()

	normalizedCWD := NormalizePath(cwd)
	var result []ClaudeProcess

	for _, proc := range m.claudeProcesses {
		if NormalizePath(proc.CWD) == normalizedCWD {
			result = append(result, proc)
		}
	}

	return result
}

// String returns a description of the ClaudeProcess for debugging.
func (cp *ClaudeProcess) String() string {
	var parts []string
	parts = append(parts, "PID="+strconv.Itoa(cp.PID))

	if cp.SessionID != "" {
		parts = append(parts, "session="+cp.SessionID)
	}
	if cp.PaneID != "" {
		parts = append(parts, "pane="+cp.PaneID)
	}
	if cp.CWD != "" {
		parts = append(parts, "cwd="+cp.CWD)
	}

	return "ClaudeProcess{" + strings.Join(parts, ", ") + "}"
}
