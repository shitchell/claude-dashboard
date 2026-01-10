package helpers

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFixturesDir(t *testing.T) {
	dir := FixturesDir(t)

	// Should contain our fixture files
	if _, err := os.Stat(filepath.Join(dir, "minimal.jsonl")); err != nil {
		t.Errorf("minimal.jsonl not found in fixtures dir: %v", err)
	}
}

func TestLoadFixture(t *testing.T) {
	data := LoadFixture(t, "minimal.jsonl")

	if len(data) == 0 {
		t.Error("LoadFixture returned empty data")
	}

	// Should contain expected content
	content := string(data)
	if !containsString(content, "claude-sonnet-4-20250514") {
		t.Error("Fixture should contain model name")
	}
}

func TestFixturePath(t *testing.T) {
	path := FixturePath(t, "minimal.jsonl")

	if !filepath.IsAbs(path) {
		t.Error("FixturePath should return absolute path")
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("FixturePath returned invalid path: %v", err)
	}
}

func TestCreateTempSession(t *testing.T) {
	tmpDir := t.TempDir()
	content := `{"type":"system","subtype":"init"}`

	path := CreateTempSession(t, tmpDir, "test.jsonl", content)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read created session: %v", err)
	}

	if string(data) != content {
		t.Errorf("Created session content = %q, want %q", string(data), content)
	}
}

func TestEncodeProjectPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/home/user/project", "-home-user-project"},
		{"/", "-"},
		{"", ""},
	}

	for _, tt := range tests {
		result := EncodeProjectPath(tt.input)
		if result != tt.expected {
			t.Errorf("EncodeProjectPath(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestCreateTempProjectDir(t *testing.T) {
	tmpDir := t.TempDir()
	projectPath := "/home/user/myproject"

	projectDir := CreateTempProjectDir(t, tmpDir, projectPath)

	// Should have created the directory
	if _, err := os.Stat(projectDir); err != nil {
		t.Errorf("Project directory was not created: %v", err)
	}

	// Should contain encoded path
	expectedName := "-home-user-myproject"
	if filepath.Base(projectDir) != expectedName {
		t.Errorf("Project dir name = %q, want %q", filepath.Base(projectDir), expectedName)
	}
}

func TestSessionBuilder(t *testing.T) {
	session := NewSessionBuilder().
		WithID("custom-id").
		WithModel("claude-opus-4-5-20251101").
		WithProject("myproject", "/home/user/myproject").
		WithSummary("Test summary").
		WithTmuxPane("%5").
		WithMessageCount(10).
		WithTurnCount(3).
		Build()

	if session.ID != "custom-id" {
		t.Errorf("ID = %q, want %q", session.ID, "custom-id")
	}
	if session.Model != "claude-opus-4-5-20251101" {
		t.Errorf("Model = %q, want %q", session.Model, "claude-opus-4-5-20251101")
	}
	if session.ProjectName != "myproject" {
		t.Errorf("ProjectName = %q, want %q", session.ProjectName, "myproject")
	}
	if session.ProjectPath != "/home/user/myproject" {
		t.Errorf("ProjectPath = %q, want %q", session.ProjectPath, "/home/user/myproject")
	}
	if session.Summary != "Test summary" {
		t.Errorf("Summary = %q, want %q", session.Summary, "Test summary")
	}
	if session.TmuxPane != "%5" {
		t.Errorf("TmuxPane = %q, want %q", session.TmuxPane, "%5")
	}
	if session.MessageCount != 10 {
		t.Errorf("MessageCount = %d, want %d", session.MessageCount, 10)
	}
	if session.TurnCount != 3 {
		t.Errorf("TurnCount = %d, want %d", session.TurnCount, 3)
	}
}

func TestJSONLBuilder(t *testing.T) {
	builder := NewJSONLBuilder().
		AddInit("test-session", "claude-sonnet-4-20250514", "/home/user/project").
		AddUserMessage("u1", "", "Hello").
		AddAssistantMessage("a1", "u1", "claude-sonnet-4-20250514", "Hi there!").
		AddSummary("Greeting conversation", "a1").
		AddResult(true, 0.01)

	content := builder.Build()

	// Should contain all expected types
	if !containsString(content, `"type":"system"`) {
		t.Error("Missing system entry")
	}
	if !containsString(content, `"type":"user"`) {
		t.Error("Missing user entry")
	}
	if !containsString(content, `"type":"assistant"`) {
		t.Error("Missing assistant entry")
	}
	if !containsString(content, `"type":"summary"`) {
		t.Error("Missing summary entry")
	}
	if !containsString(content, `"type":"result"`) {
		t.Error("Missing result entry")
	}
}

func TestJSONLBuilderWriteToFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "session.jsonl")

	NewJSONLBuilder().
		AddInit("test", "claude-sonnet-4-20250514", "/home/user").
		WriteToFile(t, path)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if len(data) == 0 {
		t.Error("File should not be empty")
	}
}

// containsString is a simple helper for string containment check.
func containsString(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || len(needle) == 0 ||
		(len(haystack) > 0 && containsStringImpl(haystack, needle)))
}

func containsStringImpl(haystack, needle string) bool {
	for i := 0; i <= len(haystack)-len(needle); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
