package tmux

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// Process format string for ps command.
// Fields are tab-separated: PID, TTY, full command with arguments.
const ProcessPSFormat = "%p\t%y\t%a"

// Process represents a running process with its TTY and command.
type Process struct {
	// PID is the process ID.
	PID int

	// TTY is the normalized TTY device (e.g., "pts/42").
	TTY string

	// Command is the full command line including arguments.
	Command string

	// CWD is the current working directory of the process.
	// This is populated by reading /proc/<pid>/cwd and may be empty
	// if the process exited or permissions don't allow reading.
	CWD string
}

// ClaudeProcess represents a Claude Code process with additional
// metadata extracted from the command line and runtime state.
type ClaudeProcess struct {
	Process

	// SessionID is the session ID if running with --resume flag.
	// Empty string if not resuming a specific session.
	SessionID string

	// PaneID is the tmux pane ID where this process is running.
	// Empty string if not in a tmux pane (or pane lookup failed).
	PaneID string
}

// ProcessRunner is an interface for executing process list commands.
// This allows for dependency injection in tests.
type ProcessRunner interface {
	// ListProcesses returns the output of the ps command.
	ListProcesses() ([]byte, error)

	// ReadCWD reads the current working directory of a process.
	// Returns the resolved symlink path or error if unreadable.
	ReadCWD(pid int) (string, error)
}

// defaultProcessRunner is the production implementation that executes
// actual ps commands and reads from /proc.
type defaultProcessRunner struct{}

// ListProcesses executes ps to list all processes with their TTY and command.
func (r *defaultProcessRunner) ListProcesses() ([]byte, error) {
	cmd := exec.Command("ps", "--no-headers", "ax", "-o", ProcessPSFormat)
	return cmd.Output()
}

// ReadCWD reads the current working directory of a process via /proc.
func (r *defaultProcessRunner) ReadCWD(pid int) (string, error) {
	path := filepath.Join("/proc", strconv.Itoa(pid), "cwd")
	return os.Readlink(path)
}

// ProcessList maintains a list of all processes with TTYs.
// It supports concurrent access and provides efficient lookups.
type ProcessList struct {
	mu sync.RWMutex

	// processes is the list of all processes.
	processes []Process

	// byTTY maps normalized TTY strings to processes running on that TTY.
	// Multiple processes can share the same TTY.
	byTTY map[string][]Process

	// runner is the command runner for executing process commands.
	// If nil, the default runner is used.
	runner ProcessRunner
}

// NewProcessList creates a new empty ProcessList.
func NewProcessList() *ProcessList {
	return &ProcessList{
		processes: make([]Process, 0),
		byTTY:     make(map[string][]Process),
	}
}

// NewProcessListWithRunner creates a new ProcessList with a custom runner.
// This is primarily used for testing with mock process output.
func NewProcessListWithRunner(runner ProcessRunner) *ProcessList {
	return &ProcessList{
		processes: make([]Process, 0),
		byTTY:     make(map[string][]Process),
		runner:    runner,
	}
}

// Discover queries the system for all running processes and populates
// the ProcessList. This clears any existing data before populating.
//
// Returns an error if the ps command fails.
func (pl *ProcessList) Discover() error {
	runner := pl.runner
	if runner == nil {
		runner = &defaultProcessRunner{}
	}

	output, err := runner.ListProcesses()
	if err != nil {
		return fmt.Errorf("listing processes: %w", err)
	}

	return pl.parseOutput(output)
}

// parseOutput parses the output of ps and populates the list.
func (pl *ProcessList) parseOutput(output []byte) error {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	// Clear existing data
	pl.processes = make([]Process, 0)
	pl.byTTY = make(map[string][]Process)

	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		proc := parseProcessLine(line)
		if proc != nil {
			pl.processes = append(pl.processes, *proc)
			pl.byTTY[proc.TTY] = append(pl.byTTY[proc.TTY], *proc)
		}
	}

	return scanner.Err()
}

// parseProcessLine parses a single line of ps output.
// Returns nil if the line cannot be parsed.
func parseProcessLine(line string) *Process {
	// Trim leading whitespace (ps output often has leading spaces)
	line = strings.TrimLeft(line, " \t")

	// Split on tabs - we expect: PID\tTTY\tCOMMAND
	fields := strings.SplitN(line, "\t", 3)
	if len(fields) < 3 {
		return nil
	}

	pid, err := strconv.Atoi(strings.TrimSpace(fields[0]))
	if err != nil {
		return nil
	}

	tty := strings.TrimSpace(fields[1])
	// Skip processes without a TTY (shown as "?" in ps output)
	if tty == "?" || tty == "" {
		return nil
	}

	command := fields[2]
	if command == "" {
		return nil
	}

	return &Process{
		PID:     pid,
		TTY:     NormalizeTTY(tty),
		Command: command,
	}
}

// GetByTTY returns all processes running on the given TTY.
// The TTY is normalized before lookup.
func (pl *ProcessList) GetByTTY(tty string) []Process {
	pl.mu.RLock()
	defer pl.mu.RUnlock()

	normalizedTTY := NormalizeTTY(tty)
	// Return a copy to avoid race conditions
	procs := pl.byTTY[normalizedTTY]
	result := make([]Process, len(procs))
	copy(result, procs)
	return result
}

// All returns a slice of all processes.
func (pl *ProcessList) All() []Process {
	pl.mu.RLock()
	defer pl.mu.RUnlock()

	result := make([]Process, len(pl.processes))
	copy(result, pl.processes)
	return result
}

// Count returns the total number of processes.
func (pl *ProcessList) Count() int {
	pl.mu.RLock()
	defer pl.mu.RUnlock()

	return len(pl.processes)
}

// PopulateCWD reads the CWD for all processes from /proc.
// This is a separate step because it can be slow for many processes.
// Processes that can't have their CWD read (e.g., exited, permission denied)
// will have an empty CWD field.
func (pl *ProcessList) PopulateCWD() {
	runner := pl.runner
	if runner == nil {
		runner = &defaultProcessRunner{}
	}

	pl.mu.Lock()
	defer pl.mu.Unlock()

	for i := range pl.processes {
		cwd, err := runner.ReadCWD(pl.processes[i].PID)
		if err == nil {
			pl.processes[i].CWD = cwd
		}
	}

	// Also update the byTTY map
	for tty := range pl.byTTY {
		for i := range pl.byTTY[tty] {
			cwd, err := runner.ReadCWD(pl.byTTY[tty][i].PID)
			if err == nil {
				pl.byTTY[tty][i].CWD = cwd
			}
		}
	}
}
