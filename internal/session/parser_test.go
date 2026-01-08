package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewParser(t *testing.T) {
	t.Run("nil config uses defaults", func(t *testing.T) {
		parser := NewParser(nil)
		if parser.config.MaxScanBytes != 64*1024 {
			t.Errorf("MaxScanBytes = %d, want %d", parser.config.MaxScanBytes, 64*1024)
		}
		if parser.config.BufferSize != 64*1024 {
			t.Errorf("BufferSize = %d, want %d", parser.config.BufferSize, 64*1024)
		}
	})

	t.Run("custom config is used", func(t *testing.T) {
		config := &ParserConfig{
			MaxScanBytes: 1024,
			BufferSize:   2048,
		}
		parser := NewParser(config)
		if parser.config.MaxScanBytes != 1024 {
			t.Errorf("MaxScanBytes = %d, want 1024", parser.config.MaxScanBytes)
		}
		if parser.config.BufferSize != 2048 {
			t.Errorf("BufferSize = %d, want 2048", parser.config.BufferSize)
		}
	})
}

func TestParseValidSession(t *testing.T) {
	parser := NewParser(nil)
	path := "testdata/valid_session.jsonl"

	metadata, err := parser.Parse(path)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if metadata.Model != "claude-opus-4-5-20251101" {
		t.Errorf("Model = %q, want %q", metadata.Model, "claude-opus-4-5-20251101")
	}

	if metadata.CWD != "/home/user/project" {
		t.Errorf("CWD = %q, want %q", metadata.CWD, "/home/user/project")
	}

	if metadata.Summary != "Create Python script for data processing" {
		t.Errorf("Summary = %q, want %q", metadata.Summary, "Create Python script for data processing")
	}

	if metadata.MessageCount != 4 {
		t.Errorf("MessageCount = %d, want 4", metadata.MessageCount)
	}

	if metadata.TurnCount != 1 {
		t.Errorf("TurnCount = %d, want 1", metadata.TurnCount)
	}

	// Session ID should be extracted from filename
	if metadata.ID != "valid_session" {
		t.Errorf("ID = %q, want %q", metadata.ID, "valid_session")
	}
}

func TestParseEmptyFile(t *testing.T) {
	parser := NewParser(nil)
	path := "testdata/empty.jsonl"

	_, err := parser.Parse(path)
	if err != ErrEmptyFile {
		t.Errorf("Parse() error = %v, want ErrEmptyFile", err)
	}
}

func TestParseMalformedJSON(t *testing.T) {
	parser := NewParser(nil)
	path := "testdata/malformed.jsonl"

	// The parser should handle malformed lines gracefully
	// and still extract what it can from valid lines
	metadata, err := parser.Parse(path)
	if err != nil {
		t.Fatalf("Parse() error = %v, want nil (should skip malformed lines)", err)
	}

	// Should have found the init message from the first valid line
	if metadata.CWD != "/home/user/project" {
		t.Errorf("CWD = %q, want %q", metadata.CWD, "/home/user/project")
	}
}

func TestParseNoInitMessage(t *testing.T) {
	parser := NewParser(nil)
	path := "testdata/no_init.jsonl"

	_, err := parser.Parse(path)
	if err != ErrNoInitMessage {
		t.Errorf("Parse() error = %v, want ErrNoInitMessage", err)
	}
}

func TestParseNoSummary(t *testing.T) {
	parser := NewParser(nil)
	path := "testdata/no_summary.jsonl"

	metadata, err := parser.Parse(path)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Without a summary message, the parser should extract summary from assistant text
	if metadata.Summary == "" {
		t.Error("Summary should not be empty - should be extracted from assistant message")
	}

	// The summary should start with "I'll analyze"
	if !strings.HasPrefix(metadata.Summary, "I'll analyze") {
		t.Errorf("Summary = %q, want prefix %q", metadata.Summary, "I'll analyze")
	}
}

func TestParseMultipleTurns(t *testing.T) {
	parser := NewParser(nil)
	path := "testdata/multiple_turns.jsonl"

	metadata, err := parser.Parse(path)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if metadata.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Model = %q, want %q", metadata.Model, "claude-sonnet-4-20250514")
	}

	// Should count 3 turns (3 user-assistant pairs)
	if metadata.TurnCount != 3 {
		t.Errorf("TurnCount = %d, want 3", metadata.TurnCount)
	}

	// 1 init + 3 users + 3 assistants + 1 summary = 8 messages
	if metadata.MessageCount != 8 {
		t.Errorf("MessageCount = %d, want 8", metadata.MessageCount)
	}

	if metadata.Summary != "Multi-turn conversation about three questions" {
		t.Errorf("Summary = %q, want %q", metadata.Summary, "Multi-turn conversation about three questions")
	}
}

func TestParseNonexistentFile(t *testing.T) {
	parser := NewParser(nil)
	_, err := parser.Parse("testdata/nonexistent.jsonl")
	if err == nil {
		t.Error("Parse() should return error for nonexistent file")
	}
}

func TestParseLastEntryValidSession(t *testing.T) {
	parser := NewParser(nil)
	path := "testdata/valid_session.jsonl"

	entry, err := parser.ParseLastEntry(path)
	if err != nil {
		t.Fatalf("ParseLastEntry() error = %v", err)
	}

	if entry.Type != MessageTypeSummary {
		t.Errorf("Type = %q, want %q", entry.Type, MessageTypeSummary)
	}

	if entry.Preview != "Create Python script for data processing" {
		t.Errorf("Preview = %q, want %q", entry.Preview, "Create Python script for data processing")
	}

	if entry.IsComplete {
		t.Error("IsComplete = true, want false")
	}
}

func TestParseLastEntryWithResult(t *testing.T) {
	parser := NewParser(nil)
	path := "testdata/with_result.jsonl"

	entry, err := parser.ParseLastEntry(path)
	if err != nil {
		t.Fatalf("ParseLastEntry() error = %v", err)
	}

	if entry.Type != MessageTypeResult {
		t.Errorf("Type = %q, want %q", entry.Type, MessageTypeResult)
	}

	if !entry.IsComplete {
		t.Error("IsComplete = false, want true")
	}

	if entry.Subtype != "success" {
		t.Errorf("Subtype = %q, want %q", entry.Subtype, "success")
	}
}

func TestParseLastEntryEmptyFile(t *testing.T) {
	parser := NewParser(nil)
	path := "testdata/empty.jsonl"

	_, err := parser.ParseLastEntry(path)
	if err != ErrEmptyFile {
		t.Errorf("ParseLastEntry() error = %v, want ErrEmptyFile", err)
	}
}

func TestTruncatePreview(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "short text unchanged",
			input:    "Hello world",
			expected: "Hello world",
		},
		{
			name:     "long text truncated",
			input:    strings.Repeat("a", 100),
			expected: strings.Repeat("a", 77) + "...",
		},
		{
			name:     "newlines replaced with spaces",
			input:    "Line 1\nLine 2\nLine 3",
			expected: "Line 1 Line 2 Line 3",
		},
		{
			name:     "multiple spaces collapsed",
			input:    "Word1    Word2     Word3",
			expected: "Word1 Word2 Word3",
		},
		{
			name:     "leading/trailing whitespace trimmed",
			input:    "  text  ",
			expected: "text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncatePreview(tt.input)
			if result != tt.expected {
				t.Errorf("truncatePreview(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractSummary(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single sentence",
			input:    "This is a summary.",
			expected: "This is a summary.",
		},
		{
			name:     "first sentence extracted",
			input:    "First sentence. Second sentence. Third sentence.",
			expected: "First sentence.",
		},
		{
			name:     "exclamation mark ends sentence",
			input:    "Important! More details follow.",
			expected: "Important!",
		},
		{
			name:     "question mark ends sentence",
			input:    "Can you help? I need assistance.",
			expected: "Can you help?",
		},
		{
			name:     "no sentence ender truncates",
			input:    strings.Repeat("word ", 50),
			expected: truncatePreview(strings.Repeat("word ", 50)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractSummary(tt.input)
			if result != tt.expected {
				t.Errorf("extractSummary(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseWithProjectPath(t *testing.T) {
	// Create a temp directory structure that mimics the real one
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "-home-testuser-myproject")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}

	// Create a valid session file
	sessionFile := filepath.Join(projectDir, "test-session-uuid.jsonl")
	content := `{"type":"system","subtype":"init","uuid":"init","timestamp":"2025-01-01T00:00:00Z","sessionId":"test","model":"claude-opus-4-5-20251101","cwd":"/home/testuser/myproject"}`
	if err := os.WriteFile(sessionFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	parser := NewParser(nil)
	metadata, err := parser.Parse(sessionFile)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Project path should be decoded from directory name
	if metadata.ProjectPath != "/home/testuser/myproject" {
		t.Errorf("ProjectPath = %q, want %q", metadata.ProjectPath, "/home/testuser/myproject")
	}

	if metadata.ProjectName != "myproject" {
		t.Errorf("ProjectName = %q, want %q", metadata.ProjectName, "myproject")
	}

	if metadata.ID != "test-session-uuid" {
		t.Errorf("ID = %q, want %q", metadata.ID, "test-session-uuid")
	}
}

func TestParseAgentFile(t *testing.T) {
	// Create a temp directory
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "-home-user-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}

	// Create an agent session file
	agentFile := filepath.Join(projectDir, "agent-abc123.jsonl")
	content := `{"type":"system","subtype":"init","uuid":"init","timestamp":"2025-01-01T00:00:00Z","sessionId":"test","model":"claude-opus-4-5-20251101","cwd":"/home/user/project"}`
	if err := os.WriteFile(agentFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	parser := NewParser(nil)
	metadata, err := parser.Parse(agentFile)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// ID should strip the "agent-" prefix
	if metadata.ID != "abc123" {
		t.Errorf("ID = %q, want %q", metadata.ID, "abc123")
	}
}

func TestParseLastEntryWithSystem(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "session.jsonl")
	content := `{"type":"system","subtype":"init","uuid":"init","timestamp":"2025-01-01T12:00:00Z","sessionId":"test","model":"claude-opus-4-5-20251101","cwd":"/home/user/project"}`
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	parser := NewParser(nil)
	entry, err := parser.ParseLastEntry(filePath)
	if err != nil {
		t.Fatalf("ParseLastEntry() error = %v", err)
	}

	if entry.Type != MessageTypeSystem {
		t.Errorf("Type = %q, want %q", entry.Type, MessageTypeSystem)
	}
	if entry.Subtype != "init" {
		t.Errorf("Subtype = %q, want %q", entry.Subtype, "init")
	}
}

func TestParseLastEntryWithUser(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "session.jsonl")
	content := `{"type":"system","subtype":"init","cwd":"/home/user/project"}
{"type":"user","uuid":"u1","timestamp":"2025-01-01T12:00:00Z","message":{"role":"user","content":"Hello world"}}`
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	parser := NewParser(nil)
	entry, err := parser.ParseLastEntry(filePath)
	if err != nil {
		t.Fatalf("ParseLastEntry() error = %v", err)
	}

	if entry.Type != MessageTypeUser {
		t.Errorf("Type = %q, want %q", entry.Type, MessageTypeUser)
	}
	if entry.Preview != "Hello world" {
		t.Errorf("Preview = %q, want %q", entry.Preview, "Hello world")
	}
	if entry.IsToolResult {
		t.Error("IsToolResult = true, want false")
	}
}

func TestParseLastEntryWithToolResult(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "session.jsonl")
	content := `{"type":"system","subtype":"init","cwd":"/home/user/project"}
{"type":"user","uuid":"u1","timestamp":"2025-01-01T12:00:00Z","message":{"role":"user","content":"tool result"},"toolUseResult":{"type":"file"}}`
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	parser := NewParser(nil)
	entry, err := parser.ParseLastEntry(filePath)
	if err != nil {
		t.Fatalf("ParseLastEntry() error = %v", err)
	}

	if !entry.IsToolResult {
		t.Error("IsToolResult = false, want true")
	}
}

func TestParseLastEntryWithAssistant(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "session.jsonl")
	content := `{"type":"system","subtype":"init","cwd":"/home/user/project"}
{"type":"assistant","uuid":"a1","timestamp":"2025-01-01T12:00:00Z","message":{"id":"msg_1","type":"message","model":"claude-opus-4-5-20251101","role":"assistant","content":[{"type":"text","text":"I can help with that."}]}}`
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	parser := NewParser(nil)
	entry, err := parser.ParseLastEntry(filePath)
	if err != nil {
		t.Fatalf("ParseLastEntry() error = %v", err)
	}

	if entry.Type != MessageTypeAssistant {
		t.Errorf("Type = %q, want %q", entry.Type, MessageTypeAssistant)
	}
	if entry.Preview != "I can help with that." {
		t.Errorf("Preview = %q, want %q", entry.Preview, "I can help with that.")
	}
}

func TestParseLastEntryNonexistentFile(t *testing.T) {
	parser := NewParser(nil)
	_, err := parser.ParseLastEntry("/nonexistent/path/file.jsonl")
	if err == nil {
		t.Error("ParseLastEntry() should return error for nonexistent file")
	}
}

func TestParseLastEntryMalformedJSON(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "session.jsonl")
	content := `{this is not valid json}`
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	parser := NewParser(nil)
	_, err := parser.ParseLastEntry(filePath)
	if err != ErrMalformedJSON {
		t.Errorf("ParseLastEntry() error = %v, want ErrMalformedJSON", err)
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected int
	}{
		{
			name:     "single line no newline",
			input:    []byte("single line"),
			expected: 1,
		},
		{
			name:     "single line with newline",
			input:    []byte("single line\n"),
			expected: 1, // Empty string after newline is not added
		},
		{
			name:     "multiple lines",
			input:    []byte("line1\nline2\nline3"),
			expected: 3,
		},
		{
			name:     "windows line endings",
			input:    []byte("line1\r\nline2\r\n"),
			expected: 2,
		},
		{
			name:     "empty",
			input:    []byte(""),
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := splitLines(tt.input)
			if len(lines) != tt.expected {
				t.Errorf("splitLines() returned %d lines, want %d", len(lines), tt.expected)
			}
		})
	}
}

func TestDefaultParserConfig(t *testing.T) {
	config := DefaultParserConfig()
	if config.MaxScanBytes != 64*1024 {
		t.Errorf("MaxScanBytes = %d, want %d", config.MaxScanBytes, 64*1024)
	}
	if config.BufferSize != 64*1024 {
		t.Errorf("BufferSize = %d, want %d", config.BufferSize, 64*1024)
	}
}
