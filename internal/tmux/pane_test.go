package tmux

import (
	"testing"
)

// TestNormalizeTTY verifies that TTY paths are normalized correctly
// by stripping the /dev/ prefix when present.
func TestNormalizeTTY(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "with /dev/ prefix",
			input:    "/dev/pts/42",
			expected: "pts/42",
		},
		{
			name:     "without prefix",
			input:    "pts/42",
			expected: "pts/42",
		},
		{
			name:     "tty device with prefix",
			input:    "/dev/tty1",
			expected: "tty1",
		},
		{
			name:     "tty device without prefix",
			input:    "tty1",
			expected: "tty1",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "single slash dev",
			input:    "/dev/",
			expected: "",
		},
		{
			name:     "just /dev without trailing slash",
			input:    "/dev",
			expected: "/dev",
		},
		{
			name:     "pts with high number",
			input:    "/dev/pts/999",
			expected: "pts/999",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeTTY(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeTTY(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestParsePaneLine tests parsing individual lines of tmux list-panes output.
func TestParsePaneLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected *Pane
	}{
		{
			name: "valid line with /dev/ prefix",
			line: "/dev/pts/42\t%0\tmySession\t0\tmyWin\t0\tmyShell",
			expected: &Pane{
				TTY:         "pts/42",
				ID:          "%0",
				SessionName: "mySession",
				WindowIndex: "0",
				WindowName:  "myWin",
				PaneIndex:   "0",
				PaneTitle:   "myShell",
			},
		},
		{
			name: "valid line without /dev/ prefix",
			line: "pts/43\t%1\twork\t1\tcode\t0\tvim",
			expected: &Pane{
				TTY:         "pts/43",
				ID:          "%1",
				SessionName: "work",
				WindowIndex: "1",
				WindowName:  "code",
				PaneIndex:   "0",
				PaneTitle:   "vim",
			},
		},
		{
			name:     "too few fields",
			line:     "pts/42\t%0\tsession",
			expected: nil,
		},
		{
			name:     "empty line",
			line:     "",
			expected: nil,
		},
		{
			name: "line with spaces in names",
			line: "/dev/pts/10\t%5\tMy Session\t2\tMy Window\t1\tMy Title",
			expected: &Pane{
				TTY:         "pts/10",
				ID:          "%5",
				SessionName: "My Session",
				WindowIndex: "2",
				WindowName:  "My Window",
				PaneIndex:   "1",
				PaneTitle:   "My Title",
			},
		},
		{
			name: "extra fields ignored",
			line: "/dev/pts/42\t%0\tsession\t0\twin\t0\ttitle\textra\tfields",
			expected: &Pane{
				TTY:         "pts/42",
				ID:          "%0",
				SessionName: "session",
				WindowIndex: "0",
				WindowName:  "win",
				PaneIndex:   "0",
				PaneTitle:   "title",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parsePaneLine(tt.line)
			if tt.expected == nil {
				if result != nil {
					t.Errorf("parsePaneLine(%q) = %+v, want nil", tt.line, result)
				}
				return
			}
			if result == nil {
				t.Errorf("parsePaneLine(%q) = nil, want %+v", tt.line, tt.expected)
				return
			}
			if *result != *tt.expected {
				t.Errorf("parsePaneLine(%q) = %+v, want %+v", tt.line, result, tt.expected)
			}
		})
	}
}

// TestPaneDescription tests the Description method.
func TestPaneDescription(t *testing.T) {
	pane := &Pane{
		TTY:         "pts/42",
		ID:          "%0",
		SessionName: "mySession",
		WindowIndex: "0",
		WindowName:  "myWin",
		PaneIndex:   "1",
		PaneTitle:   "bash",
	}

	expected := "mySession > 0:myWin > 1:bash"
	result := pane.Description()
	if result != expected {
		t.Errorf("Description() = %q, want %q", result, expected)
	}
}

// TestPaneMapParseOutput tests the PaneMap's ability to parse tmux output.
func TestPaneMapParseOutput(t *testing.T) {
	pm := NewPaneMap()

	output := []byte(`/dev/pts/42	%0	session1	0	win1	0	shell
/dev/pts/43	%1	session1	0	win1	1	vim
pts/44	%2	session2	0	code	0	bash
`)

	err := pm.parseOutput(output)
	if err != nil {
		t.Fatalf("parseOutput() error = %v", err)
	}

	// Verify count
	if pm.Count() != 3 {
		t.Errorf("Count() = %d, want 3", pm.Count())
	}

	// Test GetByTTY with /dev/ prefix (should normalize)
	pane := pm.GetByTTY("/dev/pts/42")
	if pane == nil {
		t.Error("GetByTTY(/dev/pts/42) = nil, want pane")
	} else if pane.ID != "%0" {
		t.Errorf("GetByTTY(/dev/pts/42).ID = %q, want %%0", pane.ID)
	}

	// Test GetByTTY without prefix
	pane = pm.GetByTTY("pts/43")
	if pane == nil {
		t.Error("GetByTTY(pts/43) = nil, want pane")
	} else if pane.ID != "%1" {
		t.Errorf("GetByTTY(pts/43).ID = %q, want %%1", pane.ID)
	}

	// Test GetByID
	pane = pm.GetByID("%2")
	if pane == nil {
		t.Error("GetByID(%2) = nil, want pane")
	} else if pane.TTY != "pts/44" {
		t.Errorf("GetByID(%%2).TTY = %q, want pts/44", pane.TTY)
	}

	// Test non-existent TTY
	pane = pm.GetByTTY("pts/99")
	if pane != nil {
		t.Errorf("GetByTTY(pts/99) = %+v, want nil", pane)
	}

	// Test non-existent ID
	pane = pm.GetByID("%99")
	if pane != nil {
		t.Errorf("GetByID(%%99) = %+v, want nil", pane)
	}
}

// TestPaneMapParseOutputWithMalformedLines tests that malformed lines
// are skipped without causing errors.
func TestPaneMapParseOutputWithMalformedLines(t *testing.T) {
	pm := NewPaneMap()

	output := []byte(`/dev/pts/42	%0	session1	0	win1	0	shell
malformed line without tabs
/dev/pts/43	%1	session1	0	win1	1	vim
short	line
`)

	err := pm.parseOutput(output)
	if err != nil {
		t.Fatalf("parseOutput() error = %v", err)
	}

	// Should only have 2 valid panes
	if pm.Count() != 2 {
		t.Errorf("Count() = %d, want 2", pm.Count())
	}
}

// TestPaneMapAll tests the All() method.
func TestPaneMapAll(t *testing.T) {
	pm := NewPaneMap()

	output := []byte(`/dev/pts/42	%0	session1	0	win1	0	shell
/dev/pts/43	%1	session1	0	win1	1	vim
`)

	err := pm.parseOutput(output)
	if err != nil {
		t.Fatalf("parseOutput() error = %v", err)
	}

	panes := pm.All()
	if len(panes) != 2 {
		t.Errorf("All() returned %d panes, want 2", len(panes))
	}

	// Verify we got both panes (order not guaranteed)
	foundIDs := make(map[string]bool)
	for _, p := range panes {
		foundIDs[p.ID] = true
	}
	if !foundIDs["%0"] || !foundIDs["%1"] {
		t.Errorf("All() missing expected pane IDs, found: %v", foundIDs)
	}
}

// TestPaneMapClearOnDiscover tests that Discover clears previous data.
func TestPaneMapClearOnDiscover(t *testing.T) {
	pm := NewPaneMap()

	// First parse
	output1 := []byte(`/dev/pts/42	%0	session1	0	win1	0	shell
/dev/pts/43	%1	session1	0	win1	1	vim
`)
	err := pm.parseOutput(output1)
	if err != nil {
		t.Fatalf("parseOutput() error = %v", err)
	}

	if pm.Count() != 2 {
		t.Errorf("Initial Count() = %d, want 2", pm.Count())
	}

	// Second parse with different data
	output2 := []byte(`/dev/pts/44	%2	session2	0	code	0	bash
`)
	err = pm.parseOutput(output2)
	if err != nil {
		t.Fatalf("parseOutput() error = %v", err)
	}

	// Should only have panes from second parse
	if pm.Count() != 1 {
		t.Errorf("After re-parse Count() = %d, want 1", pm.Count())
	}

	// Old panes should be gone
	if pane := pm.GetByID("%0"); pane != nil {
		t.Errorf("Old pane %%0 still present after re-parse")
	}

	// New pane should exist
	if pane := pm.GetByID("%2"); pane == nil {
		t.Errorf("New pane %%2 not found after re-parse")
	}
}

// TestNewPaneMap tests that NewPaneMap creates an empty, usable map.
func TestNewPaneMap(t *testing.T) {
	pm := NewPaneMap()

	if pm == nil {
		t.Fatal("NewPaneMap() = nil")
	}

	if pm.Count() != 0 {
		t.Errorf("NewPaneMap().Count() = %d, want 0", pm.Count())
	}

	// Should be safe to call GetByTTY on empty map
	pane := pm.GetByTTY("pts/42")
	if pane != nil {
		t.Errorf("GetByTTY on empty map = %+v, want nil", pane)
	}

	// Should be safe to call All on empty map
	panes := pm.All()
	if len(panes) != 0 {
		t.Errorf("All() on empty map = %d panes, want 0", len(panes))
	}
}
