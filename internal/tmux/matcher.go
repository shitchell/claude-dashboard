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

	// cwdToPaneID caches the mapping of CWDs to pane IDs.
	cwdToPaneID map[string]string

	// processRunner is an optional custom runner for process commands.
	processRunner ProcessRunner

	// tmuxRunner is an optional custom runner for tmux commands.
	tmuxRunner CommandRunner
}

// NewMatcher creates a new Matcher with default runners.
func NewMatcher() *Matcher {
	return &Matcher{
		paneMap:         NewPaneMap(),
		processList:     NewProcessList(),
		claudeProcesses: make([]ClaudeProcess, 0),
		sessionToPaneID: make(map[string]string),
		cwdToPaneID:     make(map[string]string),
	}
}

// NewMatcherWithRunners creates a new Matcher with custom runners.
// This is primarily used for testing with mock output.
func NewMatcherWithRunners(processRunner ProcessRunner, tmuxRunner CommandRunner) *Matcher {
	return &Matcher{
		paneMap:         NewPaneMapWithRunner(tmuxRunner),
		processList:     NewProcessListWithRunner(processRunner),
		claudeProcesses: make([]ClaudeProcess, 0),
		sessionToPaneID: make(map[string]string),
		cwdToPaneID:     make(map[string]string),
		processRunner:   processRunner,
		tmuxRunner:      tmuxRunner,
	}
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
	logging.Debug("Refreshing matcher state...")

	// Discover tmux panes
	if err := m.paneMap.Discover(); err != nil {
		// If not in tmux, we can still continue without pane info
		if err != ErrNotInTmux {
			logging.Warn("Failed to discover tmux panes: %v", err)
			return err
		}
		logging.Debug("Not running in tmux session, skipping pane discovery")
	}

	// Discover all processes
	if err := m.processList.Discover(); err != nil {
		logging.Warn("Failed to discover processes: %v", err)
		return err
	}

	// Find Claude processes and match them to panes
	m.findAndMatchClaudeProcesses()

	logging.Debug("Matcher refresh complete")
	return nil
}

// findAndMatchClaudeProcesses identifies Claude processes and matches them to panes.
func (m *Matcher) findAndMatchClaudeProcesses() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clear previous state
	m.claudeProcesses = make([]ClaudeProcess, 0)
	m.sessionToPaneID = make(map[string]string)
	m.cwdToPaneID = make(map[string]string)

	runner := m.processRunner
	if runner == nil {
		runner = &defaultProcessRunner{}
	}

	allProcs := m.processList.All()

	for _, proc := range allProcs {
		if !IsClaudeProcess(proc.Command) {
			continue
		}

		claudeProc := ClaudeProcess{
			Process:   proc,
			SessionID: ExtractSessionID(proc.Command),
		}

		// Try to get CWD for this process
		if cwd, err := runner.ReadCWD(proc.PID); err == nil {
			claudeProc.CWD = cwd
		}

		// Try to find the pane for this process
		pane := m.paneMap.GetByTTY(proc.TTY)
		if pane != nil {
			claudeProc.PaneID = pane.ID

			// Cache mappings
			if claudeProc.SessionID != "" {
				m.sessionToPaneID[claudeProc.SessionID] = pane.ID
			}
			if claudeProc.CWD != "" {
				// Resolve symlinks for consistent matching
				resolvedCWD := resolveSymlinks(claudeProc.CWD)
				m.cwdToPaneID[resolvedCWD] = pane.ID
			}
		}

		m.claudeProcesses = append(m.claudeProcesses, claudeProc)
	}

	logging.Debug("Found %d Claude processes", len(m.claudeProcesses))
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
// 1. Match by session ID (from --resume flag in command)
// 2. Match by CWD (session's CWD matches process's CWD)
func (m *Matcher) MatchSessionToPane(sess *session.Session) *Pane {
	if sess == nil {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Strategy 1: Match by session ID
	if paneID, ok := m.sessionToPaneID[sess.ID]; ok {
		logging.Debug("Matched session %s to pane %s by session ID", sess.ID, paneID)
		return m.paneMap.GetByID(paneID)
	}

	// Strategy 2: Match by CWD
	if sess.CWD != "" {
		resolvedCWD := resolveSymlinks(sess.CWD)
		if paneID, ok := m.cwdToPaneID[resolvedCWD]; ok {
			logging.Debug("Matched session %s to pane %s by CWD", sess.ID, paneID)
			return m.paneMap.GetByID(paneID)
		}
	}

	logging.Debug("No pane match found for session %s", sess.ID)
	return nil
}

// HasRunningProcess returns true if there is a Claude process running
// that matches the given session (by session ID or CWD).
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

	// Check by CWD
	if sess.CWD != "" {
		resolvedCWD := resolveSymlinks(sess.CWD)
		if _, ok := m.cwdToPaneID[resolvedCWD]; ok {
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
