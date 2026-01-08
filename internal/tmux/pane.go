package tmux

import (
	"bufio"
	"bytes"
	"strings"
	"sync"

	"github.com/shitchell/claude-dashboard/internal/logging"
)

// TmuxListPanesFormat is the format string used for tmux list-panes command.
// Fields are tab-separated for reliable parsing:
// - pane_tty: TTY device (e.g., /dev/pts/42)
// - pane_id: Unique pane identifier (e.g., %0)
// - session_name: tmux session name
// - window_index: Window number within session
// - window_name: Window title
// - pane_index: Pane number within window
// - pane_title: Pane title
const TmuxListPanesFormat = "#{pane_tty}\t#{pane_id}\t#{session_name}\t#{window_index}\t#{window_name}\t#{pane_index}\t#{pane_title}"

// TTYDevPrefix is the prefix that may appear on TTY device paths
// on some systems. We normalize by stripping this prefix.
const TTYDevPrefix = "/dev/"

// Pane represents a single tmux pane with all identifying information.
type Pane struct {
	// TTY is the normalized TTY device (e.g., "pts/42" without /dev/ prefix).
	TTY string

	// ID is the unique pane identifier (e.g., "%0", "%1").
	ID string

	// SessionName is the tmux session name.
	SessionName string

	// WindowIndex is the window number within the session.
	WindowIndex string

	// WindowName is the window title.
	WindowName string

	// PaneIndex is the pane number within the window.
	PaneIndex string

	// PaneTitle is the pane title.
	PaneTitle string
}

// Description returns a human-readable description of the pane location.
// Format: "session > window_index:window_name > pane_index:pane_title"
func (p *Pane) Description() string {
	return p.SessionName + " > " + p.WindowIndex + ":" + p.WindowName + " > " + p.PaneIndex + ":" + p.PaneTitle
}

// PaneMap maintains a mapping of TTY devices to tmux panes.
// It supports concurrent access and provides efficient lookups
// by both TTY and pane ID.
type PaneMap struct {
	mu sync.RWMutex

	// byTTY maps normalized TTY strings to panes.
	byTTY map[string]*Pane

	// byID maps pane IDs to panes.
	byID map[string]*Pane

	// runner is the command runner for executing tmux commands.
	// If nil, the default runner is used.
	runner CommandRunner
}

// NewPaneMap creates a new empty PaneMap.
func NewPaneMap() *PaneMap {
	return &PaneMap{
		byTTY: make(map[string]*Pane),
		byID:  make(map[string]*Pane),
	}
}

// NewPaneMapWithRunner creates a new PaneMap with a custom command runner.
// This is primarily used for testing with mock tmux output.
func NewPaneMapWithRunner(runner CommandRunner) *PaneMap {
	return &PaneMap{
		byTTY:  make(map[string]*Pane),
		byID:   make(map[string]*Pane),
		runner: runner,
	}
}

// NormalizeTTY converts a TTY path to a normalized form by stripping
// the /dev/ prefix if present. This ensures consistent matching
// regardless of whether the system reports TTYs with or without
// the prefix.
//
// Examples:
//   - "/dev/pts/42" -> "pts/42"
//   - "pts/42" -> "pts/42"
//   - "/dev/tty1" -> "tty1"
func NormalizeTTY(tty string) string {
	return strings.TrimPrefix(tty, TTYDevPrefix)
}

// Discover queries tmux for all panes and populates the PaneMap.
// This clears any existing data in the map before populating.
//
// Returns ErrNotInTmux if not running inside a tmux session,
// or an error if the tmux command fails.
func (pm *PaneMap) Discover() error {
	logging.Debug("Discovering tmux panes...")

	var output []byte
	var err error

	if pm.runner != nil {
		output, err = runTmuxWithRunner(pm.runner, "list-panes", "-a", "-F", TmuxListPanesFormat)
	} else {
		output, err = runTmux("list-panes", "-a", "-F", TmuxListPanesFormat)
	}

	if err != nil {
		logging.Debug("Failed to discover tmux panes: %v", err)
		return err
	}

	return pm.parseOutput(output)
}

// parseOutput parses the output of tmux list-panes and populates the map.
func (pm *PaneMap) parseOutput(output []byte) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Clear existing data
	pm.byTTY = make(map[string]*Pane)
	pm.byID = make(map[string]*Pane)

	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		pane := parsePaneLine(line)
		if pane != nil {
			logging.Debug("Found tmux pane: %s (tty=%s, session=%s)", pane.ID, pane.TTY, pane.SessionName)
			pm.byTTY[pane.TTY] = pane
			pm.byID[pane.ID] = pane
		}
	}

	logging.Info("Discovered %d tmux panes", len(pm.byID))
	return scanner.Err()
}

// parsePaneLine parses a single line of tmux list-panes output.
// Returns nil if the line cannot be parsed (e.g., wrong number of fields).
func parsePaneLine(line string) *Pane {
	fields := strings.Split(line, "\t")
	if len(fields) < 7 {
		// Log warning but don't fail - allows graceful handling of
		// unexpected tmux output formats
		return nil
	}

	return &Pane{
		TTY:         NormalizeTTY(fields[0]),
		ID:          fields[1],
		SessionName: fields[2],
		WindowIndex: fields[3],
		WindowName:  fields[4],
		PaneIndex:   fields[5],
		PaneTitle:   fields[6],
	}
}

// GetByTTY returns the pane associated with the given TTY.
// The TTY is normalized before lookup.
// Returns nil if no pane is found for the TTY.
func (pm *PaneMap) GetByTTY(tty string) *Pane {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	normalizedTTY := NormalizeTTY(tty)
	return pm.byTTY[normalizedTTY]
}

// GetByID returns the pane with the given pane ID.
// Returns nil if no pane is found with the ID.
func (pm *PaneMap) GetByID(id string) *Pane {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	return pm.byID[id]
}

// All returns a slice of all panes in the map.
// The order is not guaranteed.
func (pm *PaneMap) All() []*Pane {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	panes := make([]*Pane, 0, len(pm.byID))
	for _, pane := range pm.byID {
		panes = append(panes, pane)
	}
	return panes
}

// Count returns the number of panes in the map.
func (pm *PaneMap) Count() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	return len(pm.byID)
}
