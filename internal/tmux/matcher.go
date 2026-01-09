package tmux

import (
	"path/filepath"
	"regexp"
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
// It maintains the state needed to match Claude sessions to their
// running tmux panes.
type Matcher struct {
	mu sync.RWMutex

	// paneMap maps TTYs to tmux panes.
	paneMap *PaneMap

	// processList contains all running processes.
	processList *ProcessList

	// claudeProcesses contains only Claude-related processes.
	claudeProcesses []ClaudeProcess

	// sessionToPaneID caches the mapping of session IDs to pane IDs.
	sessionToPaneID map[string]string

	// pidToSessionID caches the mapping of PIDs to session IDs.
	// This is populated by memory scanning and is the primary
	// source for session-to-pane matching.
	pidToSessionID map[int]string

	// cwdToPanes caches the mapping of CWDs to pane IDs.
	// Multiple processes can share the same CWD, so we store a slice.
	cwdToPanes map[string][]string

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
		sessionToPaneID:  make(map[string]string),
		pidToSessionID:   make(map[int]string),
		cwdToPanes:       make(map[string][]string),
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
		sessionToPaneID:  make(map[string]string),
		pidToSessionID:   make(map[int]string),
		cwdToPanes:       make(map[string][]string),
		sessionFilePaths: make([]string, 0),
		processRunner:    processRunner,
		tmuxRunner:       tmuxRunner,
	}
}

// MemoryScannerInterface allows for mocking memory scanning in tests.
type MemoryScannerInterface interface {
	// MatchPIDToSession returns the session ID for a PID based on memory scanning.
	MatchPIDToSession(pid int, sessionPatterns map[string][]byte) (string, int)
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
// 4. Reads CWD for Claude processes
// 5. Matches Claude processes to panes
//
// Returns an error if either tmux or process discovery fails.
func (m *Matcher) Refresh() error {
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

	// Find Claude processes and match them to panes
	logging.Debug("Matcher.Refresh: Finding and matching Claude processes...")
	m.findAndMatchClaudeProcesses()

	logging.Info("Matcher.Refresh: Complete. sessionToPaneID=%d, cwdToPanes=%d, pidToSessionID=%d",
		len(m.sessionToPaneID), len(m.cwdToPanes), len(m.pidToSessionID))
	return nil
}

// findAndMatchClaudeProcesses identifies Claude processes and matches them to panes.
func (m *Matcher) findAndMatchClaudeProcesses() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clear previous state
	m.claudeProcesses = make([]ClaudeProcess, 0)
	m.sessionToPaneID = make(map[string]string)
	m.pidToSessionID = make(map[int]string)
	m.cwdToPanes = make(map[string][]string)

	runner := m.processRunner
	if runner == nil {
		runner = &defaultProcessRunner{}
	}

	allProcs := m.processList.All()
	logging.Debug("findAndMatchClaudeProcesses: Scanning %d processes for Claude commands", len(allProcs))

	// First pass: identify Claude processes
	var claudePIDs []int
	for _, proc := range allProcs {
		if !IsClaudeProcess(proc.Command) {
			continue
		}

		logging.Debug("findAndMatchClaudeProcesses: Found Claude process PID=%d Command=%s TTY=%s",
			proc.PID, proc.Command, proc.TTY)

		claudeProc := ClaudeProcess{
			Process:   proc,
			SessionID: ExtractSessionID(proc.Command),
		}

		// Try to get CWD for this process
		if cwd, err := runner.ReadCWD(proc.PID); err == nil {
			claudeProc.CWD = cwd
			logging.Debug("findAndMatchClaudeProcesses: PID=%d CWD=%s", proc.PID, cwd)
		} else {
			logging.Debug("findAndMatchClaudeProcesses: PID=%d CWD read failed: %v", proc.PID, err)
		}

		// Try to find the pane for this process
		pane := m.paneMap.GetByTTY(proc.TTY)
		if pane != nil {
			claudeProc.PaneID = pane.ID
			logging.Debug("findAndMatchClaudeProcesses: PID=%d matched to pane %s via TTY %s",
				proc.PID, pane.ID, proc.TTY)

			// Cache CWD mapping
			if claudeProc.CWD != "" {
				// Resolve symlinks for consistent matching
				resolvedCWD := resolveSymlinks(claudeProc.CWD)
				// Append to slice - multiple processes can share the same CWD
				m.cwdToPanes[resolvedCWD] = append(m.cwdToPanes[resolvedCWD], pane.ID)
			}
		} else {
			logging.Debug("findAndMatchClaudeProcesses: PID=%d no pane found for TTY %s", proc.PID, proc.TTY)
		}

		m.claudeProcesses = append(m.claudeProcesses, claudeProc)
		claudePIDs = append(claudePIDs, proc.PID)
	}

	logging.Info("findAndMatchClaudeProcesses: Found %d Claude processes", len(m.claudeProcesses))

	// Second pass: memory scanning to match PIDs to sessions
	if len(m.sessionFilePaths) > 0 && len(claudePIDs) > 0 {
		m.scanAndMatchSessions(claudePIDs)
	}

	// Third pass: build sessionToPaneID from memory scan results and command-line args
	for i := range m.claudeProcesses {
		proc := &m.claudeProcesses[i]

		// First try memory scan result
		if sessionID, ok := m.pidToSessionID[proc.PID]; ok {
			proc.SessionID = sessionID
			logging.Debug("PID %d: session ID from memory scan: %s", proc.PID, sessionID)
		}

		// Cache session -> pane mapping
		if proc.SessionID != "" && proc.PaneID != "" {
			m.sessionToPaneID[proc.SessionID] = proc.PaneID
		}
	}
}

// scanAndMatchSessions performs memory scanning to match PIDs to session IDs.
// This is called with the mutex already held.
func (m *Matcher) scanAndMatchSessions(pids []int) {
	logging.Info("scanAndMatchSessions: Starting memory scan for %d PIDs with %d session paths",
		len(pids), len(m.sessionFilePaths))

	// Build patterns from session paths
	sessionPatterns := BuildSessionPatterns(m.sessionFilePaths)
	if len(sessionPatterns) == 0 {
		logging.Warn("scanAndMatchSessions: No session patterns built from paths")
		return
	}
	logging.Debug("scanAndMatchSessions: Built %d session patterns", len(sessionPatterns))

	// Scan each PID
	for i, pid := range pids {
		logging.Debug("scanAndMatchSessions: Scanning PID %d (%d/%d)...", pid, i+1, len(pids))

		var sessionID string
		var count int

		if m.memoryScanner != nil {
			// Use mock scanner for testing
			sessionID, count = m.memoryScanner.MatchPIDToSession(pid, sessionPatterns)
		} else {
			// Use real memory scanning
			sessionID, count = MatchPIDToSession(pid, sessionPatterns)
		}

		if sessionID != "" {
			m.pidToSessionID[pid] = sessionID
			logging.Info("scanAndMatchSessions: PID %d -> session %s (count: %d)", pid, sessionID, count)
		} else {
			logging.Debug("scanAndMatchSessions: PID %d -> no session match", pid)
		}
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
// Matching strategy (in order of precedence):
// 1. Match by session ID (from memory scanning or --resume flag)
// 2. Match by ProjectPath (more specific than CWD)
// 3. Match by CWD (session's CWD matches process's CWD)
//
// Memory scanning is the primary strategy: we scan process memory for
// session file paths to reliably identify which session each process is running.
// This works even for fresh sessions without --resume flags.
//
// When multiple panes match the same CWD, we return the first match.
func (m *Matcher) MatchSessionToPane(sess *session.Session) *Pane {
	if sess == nil {
		logging.Debug("MatchSessionToPane: session is nil")
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	logging.Debug("MatchSessionToPane: Looking for session ID=%s, ProjectPath=%s, CWD=%s",
		sess.ID, sess.ProjectPath, sess.CWD)
	logging.Debug("MatchSessionToPane: sessionToPaneID has %d entries", len(m.sessionToPaneID))
	for sid, pid := range m.sessionToPaneID {
		logging.Debug("MatchSessionToPane:   sessionToPaneID[%s] = %s", sid, pid)
	}

	// Strategy 1: Match by session ID (most reliable)
	if paneID, ok := m.sessionToPaneID[sess.ID]; ok {
		logging.Info("MatchSessionToPane: Matched session %s to pane %s by session ID", sess.ID, paneID)
		pane := m.paneMap.GetByID(paneID)
		if pane == nil {
			logging.Warn("MatchSessionToPane: paneMap.GetByID(%s) returned nil!", paneID)
		}
		return pane
	}
	logging.Debug("MatchSessionToPane: No session ID match for %s", sess.ID)

	// Strategy 2: Match by ProjectPath (more specific than CWD)
	if sess.ProjectPath != "" {
		resolvedProjectPath := resolveSymlinks(sess.ProjectPath)
		logging.Debug("MatchSessionToPane: Trying ProjectPath match, resolved=%s", resolvedProjectPath)
		if paneIDs, ok := m.cwdToPanes[resolvedProjectPath]; ok && len(paneIDs) > 0 {
			logging.Info("MatchSessionToPane: Matched session %s to pane %s by ProjectPath (of %d candidates)",
				sess.ID, paneIDs[0], len(paneIDs))
			return m.paneMap.GetByID(paneIDs[0])
		}
		logging.Debug("MatchSessionToPane: No ProjectPath match")
	}

	// Strategy 3: Match by CWD
	if sess.CWD != "" {
		resolvedCWD := resolveSymlinks(sess.CWD)
		logging.Debug("MatchSessionToPane: Trying CWD match, resolved=%s", resolvedCWD)
		logging.Debug("MatchSessionToPane: cwdToPanes has %d entries", len(m.cwdToPanes))
		for cwd, panes := range m.cwdToPanes {
			logging.Debug("MatchSessionToPane:   cwdToPanes[%s] = %v", cwd, panes)
		}
		if paneIDs, ok := m.cwdToPanes[resolvedCWD]; ok && len(paneIDs) > 0 {
			logging.Info("MatchSessionToPane: Matched session %s to pane %s by CWD (of %d candidates)",
				sess.ID, paneIDs[0], len(paneIDs))
			return m.paneMap.GetByID(paneIDs[0])
		}
		logging.Debug("MatchSessionToPane: No CWD match")
	}

	logging.Info("MatchSessionToPane: No pane match found for session %s (ID=%s, ProjectPath=%s, CWD=%s)",
		sess.ID, sess.ID, sess.ProjectPath, sess.CWD)
	return nil
}

// HasRunningProcess returns true if there is a Claude process running
// that matches the given session (by session ID, ProjectPath, or CWD).
func (m *Matcher) HasRunningProcess(sess *session.Session) bool {
	if sess == nil {
		return false
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check by session ID
	if _, ok := m.sessionToPaneID[sess.ID]; ok {
		return true
	}

	// Check by ProjectPath
	if sess.ProjectPath != "" {
		resolvedProjectPath := resolveSymlinks(sess.ProjectPath)
		if paneIDs, ok := m.cwdToPanes[resolvedProjectPath]; ok && len(paneIDs) > 0 {
			return true
		}
	}

	// Check by CWD
	if sess.CWD != "" {
		resolvedCWD := resolveSymlinks(sess.CWD)
		if paneIDs, ok := m.cwdToPanes[resolvedCWD]; ok && len(paneIDs) > 0 {
			return true
		}
	}

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

// GetCWDToPanes returns a copy of the CWD to panes mapping.
// This is primarily for debugging.
func (m *Matcher) GetCWDToPanes() map[string][]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string][]string)
	for k, v := range m.cwdToPanes {
		copied := make([]string, len(v))
		copy(copied, v)
		result[k] = copied
	}
	return result
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

// matchByMostRecent selects the most recently started Claude process
// when multiple processes are running in the same directory.
// This is a heuristic - the process with the highest PID is likely
// the most recently started.
func matchByMostRecent(processes []ClaudeProcess) *ClaudeProcess {
	if len(processes) == 0 {
		return nil
	}

	highest := &processes[0]
	for i := range processes {
		if processes[i].PID > highest.PID {
			highest = &processes[i]
		}
	}
	return highest
}

// String returns a description of the ClaudeProcess for debugging.
func (cp *ClaudeProcess) String() string {
	var parts []string
	parts = append(parts, "PID="+strings.TrimSpace(string(rune(cp.PID))))

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
