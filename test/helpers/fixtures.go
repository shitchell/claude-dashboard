// Package helpers provides shared test utilities for claude-dashboard tests.
package helpers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shitchell/claude-dashboard/internal/session"
)

// FixturesDir returns the path to the testdata/sessions directory.
// It locates the project root by looking for go.mod.
func FixturesDir(t *testing.T) string {
	t.Helper()

	// Start from the current working directory and look for go.mod
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	// Walk up to find go.mod
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "testdata", "sessions")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("Could not find project root (go.mod) starting from %s", cwd)
		}
		dir = parent
	}
}

// LoadFixture reads a fixture file from testdata/sessions.
func LoadFixture(t *testing.T, name string) []byte {
	t.Helper()

	path := filepath.Join(FixturesDir(t), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to load fixture %q: %v", name, err)
	}
	return data
}

// FixturePath returns the full path to a fixture file.
func FixturePath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(FixturesDir(t), name)
}

// CreateTempSession creates a temporary session file for testing.
// It returns the path to the created file.
func CreateTempSession(t *testing.T, dir string, name string, content string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create temp session %q: %v", name, err)
	}
	return path
}

// EncodeProjectPath encodes a filesystem path to the Claude projects directory
// name format. This is the reverse of util.DecodeProjectPath.
// Example: "/home/user/project" -> "-home-user-project"
func EncodeProjectPath(path string) string {
	// Replace slashes with dashes
	return strings.ReplaceAll(path, "/", "-")
}

// CreateTempProjectDir creates a temporary project directory structure
// mimicking the Claude projects directory format.
// Returns the path to the created project directory.
func CreateTempProjectDir(t *testing.T, baseDir string, projectPath string) string {
	t.Helper()

	// Convert project path to directory name (e.g., /home/user/project -> -home-user-project)
	encodedPath := EncodeProjectPath(projectPath)
	projectDir := filepath.Join(baseDir, encodedPath)

	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create project directory %q: %v", projectDir, err)
	}
	return projectDir
}

// SessionBuilder provides a fluent interface for building test sessions.
type SessionBuilder struct {
	session *session.Session
}

// NewSessionBuilder creates a new SessionBuilder with default values.
func NewSessionBuilder() *SessionBuilder {
	return &SessionBuilder{
		session: &session.Session{
			ID:           "test-session-" + randomID(),
			Model:        "claude-sonnet-4-20250514",
			ProjectName:  "test-project",
			ProjectPath:  "/home/user/test-project",
			CWD:          "/home/user/test-project",
			MessageCount: 2,
			TurnCount:    1,
			ModTime:      time.Now(),
		},
	}
}

// WithID sets the session ID.
func (b *SessionBuilder) WithID(id string) *SessionBuilder {
	b.session.ID = id
	return b
}

// WithModel sets the model.
func (b *SessionBuilder) WithModel(model string) *SessionBuilder {
	b.session.Model = model
	return b
}

// WithProject sets the project name and path.
func (b *SessionBuilder) WithProject(name, path string) *SessionBuilder {
	b.session.ProjectName = name
	b.session.ProjectPath = path
	return b
}

// WithSummary sets the session summary.
func (b *SessionBuilder) WithSummary(summary string) *SessionBuilder {
	b.session.Summary = summary
	return b
}

// WithTmuxPane sets the tmux pane ID.
func (b *SessionBuilder) WithTmuxPane(paneID string) *SessionBuilder {
	b.session.TmuxPane = paneID
	return b
}

// WithModTime sets the modification timestamp.
func (b *SessionBuilder) WithModTime(t time.Time) *SessionBuilder {
	b.session.ModTime = t
	return b
}

// WithMessageCount sets the message count.
func (b *SessionBuilder) WithMessageCount(count int) *SessionBuilder {
	b.session.MessageCount = count
	return b
}

// WithTurnCount sets the turn count.
func (b *SessionBuilder) WithTurnCount(count int) *SessionBuilder {
	b.session.TurnCount = count
	return b
}

// Build returns the constructed session.
func (b *SessionBuilder) Build() *session.Session {
	return b.session
}

// randomID generates a simple pseudo-random ID for testing.
func randomID() string {
	return time.Now().Format("20060102150405.000000")
}

// JSONLEntryBuilder builds JSONL entries for session files.
type JSONLEntryBuilder struct {
	entries []map[string]interface{}
}

// NewJSONLBuilder creates a new JSONLEntryBuilder.
func NewJSONLBuilder() *JSONLEntryBuilder {
	return &JSONLEntryBuilder{
		entries: make([]map[string]interface{}, 0),
	}
}

// AddInit adds an init message.
func (b *JSONLEntryBuilder) AddInit(sessionID, model, cwd string) *JSONLEntryBuilder {
	b.entries = append(b.entries, map[string]interface{}{
		"type":      "system",
		"subtype":   "init",
		"uuid":      "init-" + sessionID,
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		"sessionId": sessionID,
		"model":     model,
		"cwd":       cwd,
	})
	return b
}

// AddUserMessage adds a user message.
func (b *JSONLEntryBuilder) AddUserMessage(uuid, parentUuid, content string) *JSONLEntryBuilder {
	b.entries = append(b.entries, map[string]interface{}{
		"type":       "user",
		"uuid":       uuid,
		"parentUuid": parentUuid,
		"timestamp":  time.Now().UTC().Format(time.RFC3339Nano),
		"message": map[string]interface{}{
			"role":    "user",
			"content": content,
		},
	})
	return b
}

// AddAssistantMessage adds an assistant message.
func (b *JSONLEntryBuilder) AddAssistantMessage(uuid, parentUuid, model, content string) *JSONLEntryBuilder {
	b.entries = append(b.entries, map[string]interface{}{
		"type":       "assistant",
		"uuid":       uuid,
		"parentUuid": parentUuid,
		"timestamp":  time.Now().UTC().Format(time.RFC3339Nano),
		"message": map[string]interface{}{
			"id":    "msg_" + uuid,
			"type":  "message",
			"model": model,
			"role":  "assistant",
			"content": []map[string]interface{}{
				{"type": "text", "text": content},
			},
			"stop_reason": "end_turn",
		},
	})
	return b
}

// AddSummary adds a summary entry.
func (b *JSONLEntryBuilder) AddSummary(summary, leafUuid string) *JSONLEntryBuilder {
	b.entries = append(b.entries, map[string]interface{}{
		"type":     "summary",
		"summary":  summary,
		"leafUuid": leafUuid,
	})
	return b
}

// AddResult adds a result entry.
func (b *JSONLEntryBuilder) AddResult(success bool, cost float64) *JSONLEntryBuilder {
	subtype := "success"
	if !success {
		subtype = "failure"
	}
	b.entries = append(b.entries, map[string]interface{}{
		"type":      "result",
		"subtype":   subtype,
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		"cost":      cost,
		"isError":   !success,
	})
	return b
}

// Build returns the JSONL content as a string.
func (b *JSONLEntryBuilder) Build() string {
	var result string
	for _, entry := range b.entries {
		data, _ := json.Marshal(entry)
		result += string(data) + "\n"
	}
	return result
}

// WriteToFile writes the JSONL content to a file.
func (b *JSONLEntryBuilder) WriteToFile(t *testing.T, path string) {
	t.Helper()
	content := b.Build()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write JSONL file %q: %v", path, err)
	}
}
