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
//
// Caching optimization:
//   - PIDCache stores PID -> session file mappings to avoid rescanning known PIDs
//   - SessionWatcher uses fsnotify to detect new session files (e.g., after /clear)
//   - forceFullScan can be set to bypass the cache (triggered by 'r' key)
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

	// pidCache caches PID -> session file mappings to avoid redundant memory scans.
	// If nil, caching is disabled.
	pidCache *PIDCache

	// sessionWatcher monitors session directories for new .jsonl files.
	// If nil, fsnotify watching is disabled.
	sessionWatcher *SessionWatcher

	// forceFullScan when true bypasses the cache and rescans all PIDs.
	// This is set by SetForceFullScan() and cleared after each Refresh().
	forceFullScan bool
}

// NewMatcher creates a new Matcher with default runners.
// Initializes PID cache and session watcher for optimized scanning.
func NewMatcher() *Matcher {
	m := &Matcher{
		paneMap:          NewPaneMap(),
		processList:      NewProcessList(),
		claudeProcesses:  make([]ClaudeProcess, 0),
		pidToSessionID:   make(map[int]string),
		sessionFilePaths: make([]string, 0),
	}

	// Initialize PID cache
	m.pidCache = NewPIDCache()
	if err := m.pidCache.Load(); err != nil {
		logging.Warn("Matcher: Failed to load PID cache: %v", err)
	}

	// Initialize session watcher with callback to handle new files
	var err error
	m.sessionWatcher, err = NewSessionWatcher(func(path string) {
		// New session file detected - invalidate cache for affected PIDs
		logging.Info("Matcher: New session file detected by watcher: %s", path)
		m.handleNewSessionFile(path)
	})
	if err != nil {
		logging.Warn("Matcher: Failed to create session watcher: %v", err)
	} else {
		m.sessionWatcher.Start()
	}

	return m
}

// NewMatcherWithRunners creates a new Matcher with custom runners.
// This is primarily used for testing with mock output.
// Note: PID cache and session watcher are NOT initialized for test matchers.
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
//
// Caching behavior:
//   - If forceFullScan is true, clears cache and scans all PIDs
//   - Otherwise, uses cache for known PIDs and only scans uncached PIDs
//   - Updates cache with new scan results
func (m *Matcher) scanAndMatchSessions(pids []int) {
	logging.Info("scanAndMatchSessions: Starting for %d PIDs with %d session paths (forceFullScan=%v)",
		len(pids), len(m.sessionFilePaths), m.forceFullScan)

	// Handle force full scan (cache buster from 'r' key)
	if m.forceFullScan && m.pidCache != nil {
		logging.Info("scanAndMatchSessions: Force full scan requested, clearing cache")
		m.pidCache.Clear()
		m.forceFullScan = false // Clear the flag
	}

	// Determine which PIDs need scanning
	var pidsToScan []int
	if m.pidCache != nil {
		// Use cached results for known PIDs
		cachedMapping := m.pidCache.GetCachedMapping()
		for pid, sessionID := range cachedMapping {
			// Only use cached values for PIDs that are still in our list
			for _, p := range pids {
				if p == pid {
					m.pidToSessionID[pid] = sessionID
					logging.Debug("scanAndMatchSessions: Using cached result for PID %d -> %s", pid, sessionID)
					break
				}
			}
		}

		// Get PIDs not in cache
		pidsToScan = m.pidCache.GetUncachedPIDs(pids)
		logging.Info("scanAndMatchSessions: %d PIDs cached, %d need scanning", len(pids)-len(pidsToScan), len(pidsToScan))
	} else {
		// No cache, scan all PIDs
		pidsToScan = pids
	}

	// Perform memory scanning for uncached PIDs
	if len(pidsToScan) > 0 {
		var pidToSession map[int]string

		if m.memoryScanner != nil {
			// Use mock scanner for testing
			pidToSession = m.memoryScanner.ScanAllPIDsForSessions(pidsToScan, m.sessionFilePaths)
		} else {
			// Use real memory scanning with NULL-prefix approach
			pidToSession = ScanAllPIDsForSessions(pidsToScan, m.sessionFilePaths)
		}

		// Store results and update cache
		for pid, sessionID := range pidToSession {
			m.pidToSessionID[pid] = sessionID
			logging.Info("scanAndMatchSessions: PID %d -> session %s", pid, sessionID)

			// Update cache with session file path
			if m.pidCache != nil {
				sessionFile := m.findSessionFilePath(sessionID)
				if sessionFile != "" {
					m.pidCache.Set(pid, sessionFile)
				}
			}
		}

		// Save cache if dirty
		if m.pidCache != nil && m.pidCache.IsDirty() {
			if err := m.pidCache.Save(); err != nil {
				logging.Warn("scanAndMatchSessions: Failed to save PID cache: %v", err)
			}
		}
	}

	// Update session watcher with directories from session file paths
	if m.sessionWatcher != nil {
		dirs := m.extractSessionDirs()
		m.sessionWatcher.SyncWatchedDirs(dirs)
	}

	logging.Info("scanAndMatchSessions: Complete. Matched %d PIDs to sessions", len(m.pidToSessionID))
}

// findSessionFilePath finds the full path for a session ID.
func (m *Matcher) findSessionFilePath(sessionID string) string {
	for _, path := range m.sessionFilePaths {
		if strings.HasSuffix(path, "/"+sessionID+".jsonl") {
			return path
		}
	}
	return ""
}

// extractSessionDirs extracts unique directories from session file paths.
func (m *Matcher) extractSessionDirs() []string {
	dirSet := make(map[string]struct{})
	for _, path := range m.sessionFilePaths {
		dir := filepath.Dir(path)
		dirSet[dir] = struct{}{}
	}

	dirs := make([]string, 0, len(dirSet))
	for dir := range dirSet {
		dirs = append(dirs, dir)
	}
	return dirs
}

// handleNewSessionFile is called by the session watcher when a new .jsonl file is created.
// This typically happens after /clear is executed in Claude.
func (m *Matcher) handleNewSessionFile(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Add the new path to our session file paths
	found := false
	for _, p := range m.sessionFilePaths {
		if p == path {
			found = true
			break
		}
	}
	if !found {
		m.sessionFilePaths = append(m.sessionFilePaths, path)
		logging.Info("Matcher: Added new session file to paths: %s", path)
	}

	// Clear cache entries that might be stale due to /clear
	// The directory of the new file indicates which project was affected
	dir := filepath.Dir(path)

	if m.pidCache != nil {
		// Check each cached entry to see if it's in the same directory
		// and might need re-scanning
		cachedDirs := m.pidCache.GetAllSessionDirs()
		for _, cachedDir := range cachedDirs {
			if cachedDir == dir {
				// This directory has a new file - clear all entries for it
				// to force a rescan on next Refresh()
				logging.Info("Matcher: Invalidating cache entries in directory %s due to new session file", dir)
				// Note: We can't easily clear by directory, so set forceFullScan
				// for a simpler but more aggressive approach
				m.forceFullScan = true
				break
			}
		}
	}
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

// SetForceFullScan sets the flag to force a full memory scan on next Refresh().
// This is typically called when the user presses 'r' for manual refresh.
// The flag is cleared automatically after the scan completes.
func (m *Matcher) SetForceFullScan(force bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.forceFullScan = force
	logging.Debug("Matcher: SetForceFullScan(%v)", force)
}

// Close releases resources used by the Matcher.
// This should be called when the Matcher is no longer needed.
func (m *Matcher) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Save PID cache
	if m.pidCache != nil && m.pidCache.IsDirty() {
		if err := m.pidCache.Save(); err != nil {
			logging.Warn("Matcher.Close: Failed to save PID cache: %v", err)
		}
	}

	// Stop session watcher
	if m.sessionWatcher != nil {
		if err := m.sessionWatcher.Stop(); err != nil {
			logging.Warn("Matcher.Close: Failed to stop session watcher: %v", err)
			return err
		}
	}

	logging.Debug("Matcher: Closed successfully")
	return nil
}

// GetPIDCache returns the PID cache (for testing/debugging).
func (m *Matcher) GetPIDCache() *PIDCache {
	return m.pidCache
}

// GetSessionWatcher returns the session watcher (for testing/debugging).
func (m *Matcher) GetSessionWatcher() *SessionWatcher {
	return m.sessionWatcher
}
