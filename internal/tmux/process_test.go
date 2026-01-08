package tmux

import (
	"testing"
)

// TestParseProcessLine tests parsing individual lines of ps output.
func TestParseProcessLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected *Process
	}{
		{
			name: "valid process with TTY",
			line: "12345\tpts/42\tbash",
			expected: &Process{
				PID:     12345,
				TTY:     "pts/42",
				Command: "bash",
			},
		},
		{
			name: "process with leading spaces",
			line: "  12345\tpts/42\tbash",
			expected: &Process{
				PID:     12345,
				TTY:     "pts/42",
				Command: "bash",
			},
		},
		{
			name: "process with full command",
			line: "12346\tpts/43\tclaude --resume abc123",
			expected: &Process{
				PID:     12346,
				TTY:     "pts/43",
				Command: "claude --resume abc123",
			},
		},
		{
			name: "process with complex command",
			line: "12347\tpts/44\t/usr/local/bin/node /home/user/.nvm/versions/node/v18.0.0/bin/claude",
			expected: &Process{
				PID:     12347,
				TTY:     "pts/44",
				Command: "/usr/local/bin/node /home/user/.nvm/versions/node/v18.0.0/bin/claude",
			},
		},
		{
			name:     "process without TTY (question mark)",
			line:     "12348\t?\tsome-daemon",
			expected: nil,
		},
		{
			name:     "process without TTY (empty)",
			line:     "12349\t\tsome-daemon",
			expected: nil,
		},
		{
			name:     "too few fields",
			line:     "12350\tpts/42",
			expected: nil,
		},
		{
			name:     "empty line",
			line:     "",
			expected: nil,
		},
		{
			name:     "invalid PID",
			line:     "abc\tpts/42\tbash",
			expected: nil,
		},
		{
			name:     "empty command",
			line:     "12351\tpts/42\t",
			expected: nil,
		},
		{
			name: "tty without pts prefix",
			line: "12352\ttty1\tlogin",
			expected: &Process{
				PID:     12352,
				TTY:     "tty1",
				Command: "login",
			},
		},
		{
			name: "command with tabs",
			line: "12353\tpts/45\tcmd\twith\ttabs",
			expected: &Process{
				PID:     12353,
				TTY:     "pts/45",
				Command: "cmd\twith\ttabs",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseProcessLine(tt.line)
			if tt.expected == nil {
				if result != nil {
					t.Errorf("parseProcessLine(%q) = %+v, want nil", tt.line, result)
				}
				return
			}
			if result == nil {
				t.Errorf("parseProcessLine(%q) = nil, want %+v", tt.line, tt.expected)
				return
			}
			if result.PID != tt.expected.PID {
				t.Errorf("PID = %d, want %d", result.PID, tt.expected.PID)
			}
			if result.TTY != tt.expected.TTY {
				t.Errorf("TTY = %q, want %q", result.TTY, tt.expected.TTY)
			}
			if result.Command != tt.expected.Command {
				t.Errorf("Command = %q, want %q", result.Command, tt.expected.Command)
			}
		})
	}
}

// mockProcessRunner is a test implementation of ProcessRunner.
type mockProcessRunner struct {
	processOutput []byte
	cwds          map[int]string
}

func (m *mockProcessRunner) ListProcesses() ([]byte, error) {
	return m.processOutput, nil
}

func (m *mockProcessRunner) ReadCWD(pid int) (string, error) {
	if cwd, ok := m.cwds[pid]; ok {
		return cwd, nil
	}
	return "", nil
}

// TestProcessListParseOutput tests the ProcessList's ability to parse ps output.
func TestProcessListParseOutput(t *testing.T) {
	pl := NewProcessList()

	output := []byte(`12345	pts/42	bash
12346	pts/42	claude --resume abc123
12347	pts/43	vim file.txt
12348	?	systemd
`)

	err := pl.parseOutput(output)
	if err != nil {
		t.Fatalf("parseOutput() error = %v", err)
	}

	// Should have 3 processes (one without TTY is skipped)
	if pl.Count() != 3 {
		t.Errorf("Count() = %d, want 3", pl.Count())
	}

	// Test GetByTTY
	procs := pl.GetByTTY("pts/42")
	if len(procs) != 2 {
		t.Errorf("GetByTTY(pts/42) returned %d processes, want 2", len(procs))
	}

	// Verify one is bash and one is claude
	commands := make(map[string]bool)
	for _, p := range procs {
		commands[p.Command] = true
	}
	if !commands["bash"] {
		t.Error("Missing bash process on pts/42")
	}
	if !commands["claude --resume abc123"] {
		t.Error("Missing claude process on pts/42")
	}

	// Test single process on TTY
	procs = pl.GetByTTY("pts/43")
	if len(procs) != 1 {
		t.Errorf("GetByTTY(pts/43) returned %d processes, want 1", len(procs))
	}

	// Test non-existent TTY
	procs = pl.GetByTTY("pts/99")
	if len(procs) != 0 {
		t.Errorf("GetByTTY(pts/99) returned %d processes, want 0", len(procs))
	}
}

// TestProcessListAll tests the All() method.
func TestProcessListAll(t *testing.T) {
	pl := NewProcessList()

	output := []byte(`12345	pts/42	bash
12346	pts/43	vim
`)

	err := pl.parseOutput(output)
	if err != nil {
		t.Fatalf("parseOutput() error = %v", err)
	}

	procs := pl.All()
	if len(procs) != 2 {
		t.Errorf("All() returned %d processes, want 2", len(procs))
	}

	// Verify we got both processes
	pids := make(map[int]bool)
	for _, p := range procs {
		pids[p.PID] = true
	}
	if !pids[12345] || !pids[12346] {
		t.Errorf("All() missing expected PIDs, found: %v", pids)
	}
}

// TestProcessListClearOnParse tests that parse clears previous data.
func TestProcessListClearOnParse(t *testing.T) {
	pl := NewProcessList()

	// First parse
	output1 := []byte(`12345	pts/42	bash
12346	pts/43	vim
`)
	err := pl.parseOutput(output1)
	if err != nil {
		t.Fatalf("parseOutput() error = %v", err)
	}

	if pl.Count() != 2 {
		t.Errorf("Initial Count() = %d, want 2", pl.Count())
	}

	// Second parse with different data
	output2 := []byte(`12347	pts/44	zsh
`)
	err = pl.parseOutput(output2)
	if err != nil {
		t.Fatalf("parseOutput() error = %v", err)
	}

	// Should only have process from second parse
	if pl.Count() != 1 {
		t.Errorf("After re-parse Count() = %d, want 1", pl.Count())
	}

	// Old TTYs should be gone
	if procs := pl.GetByTTY("pts/42"); len(procs) != 0 {
		t.Error("Old TTY pts/42 still has processes after re-parse")
	}

	// New TTY should exist
	if procs := pl.GetByTTY("pts/44"); len(procs) != 1 {
		t.Error("New TTY pts/44 not found after re-parse")
	}
}

// TestNewProcessList tests that NewProcessList creates an empty, usable list.
func TestNewProcessList(t *testing.T) {
	pl := NewProcessList()

	if pl == nil {
		t.Fatal("NewProcessList() = nil")
	}

	if pl.Count() != 0 {
		t.Errorf("NewProcessList().Count() = %d, want 0", pl.Count())
	}

	// Should be safe to call GetByTTY on empty list
	procs := pl.GetByTTY("pts/42")
	if len(procs) != 0 {
		t.Errorf("GetByTTY on empty list = %d processes, want 0", len(procs))
	}

	// Should be safe to call All on empty list
	all := pl.All()
	if len(all) != 0 {
		t.Errorf("All() on empty list = %d processes, want 0", len(all))
	}
}

// TestProcessListWithRunner tests using a custom runner.
func TestProcessListWithRunner(t *testing.T) {
	runner := &mockProcessRunner{
		processOutput: []byte(`12345	pts/42	claude
`),
		cwds: map[int]string{
			12345: "/home/user/project",
		},
	}

	pl := NewProcessListWithRunner(runner)
	err := pl.Discover()
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if pl.Count() != 1 {
		t.Errorf("Count() = %d, want 1", pl.Count())
	}

	procs := pl.GetByTTY("pts/42")
	if len(procs) != 1 {
		t.Fatalf("GetByTTY(pts/42) returned %d processes, want 1", len(procs))
	}

	if procs[0].PID != 12345 {
		t.Errorf("PID = %d, want 12345", procs[0].PID)
	}
	if procs[0].Command != "claude" {
		t.Errorf("Command = %q, want claude", procs[0].Command)
	}
}

// TestProcessListParseOutputWithMalformedLines tests that malformed lines
// are skipped without causing errors.
func TestProcessListParseOutputWithMalformedLines(t *testing.T) {
	pl := NewProcessList()

	output := []byte(`12345	pts/42	bash
malformed line without tabs
12346	pts/43	vim
short
`)

	err := pl.parseOutput(output)
	if err != nil {
		t.Fatalf("parseOutput() error = %v", err)
	}

	// Should only have 2 valid processes
	if pl.Count() != 2 {
		t.Errorf("Count() = %d, want 2", pl.Count())
	}
}

// TestProcessListParseOutputEmptyLines tests that empty lines are handled.
func TestProcessListParseOutputEmptyLines(t *testing.T) {
	pl := NewProcessList()

	output := []byte(`
12345	pts/42	bash

12346	pts/43	vim

`)

	err := pl.parseOutput(output)
	if err != nil {
		t.Fatalf("parseOutput() error = %v", err)
	}

	if pl.Count() != 2 {
		t.Errorf("Count() = %d, want 2", pl.Count())
	}
}
