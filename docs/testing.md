# Testing Guide

This document provides practical guidance for running and writing tests for claude-dashboard.

## Quick Start

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests for a specific package
go test -v ./internal/session/...

# Run a specific test
go test -v -run TestParseSummary ./internal/session/...

# Run tests with race detector
go test -race ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Test Structure

### Directory Layout

```
claude-dashboard/
├── main_test.go                    # Application entry tests
├── testdata/
│   └── sessions/                   # Shared test fixtures
│       ├── minimal.jsonl           # Minimal valid session
│       ├── full.jsonl              # Complete session with all fields
│       └── multimodel.jsonl        # Session with model changes
├── test/
│   ├── helpers/                    # Shared test utilities
│   │   ├── fixtures.go             # Fixture loading and session builders
│   │   ├── assertions.go           # Custom assertion helpers
│   │   └── tmux.go                 # Tmux test utilities
│   └── e2e/                        # End-to-end tests
│       ├── harness.go              # E2E test harness
│       └── navigation_test.go      # Navigation E2E tests
└── internal/
    ├── config/*_test.go            # Config unit tests
    ├── session/*_test.go           # Session unit/integration tests
    ├── tmux/*_test.go              # Tmux unit/integration tests
    └── ui/*_test.go                # UI unit/integration tests
```

### Test Tiers

| Tier | Speed | Location | When to Use |
|------|-------|----------|-------------|
| Unit | < 100ms | `*_test.go` in package | Pure functions, algorithms |
| Integration | 100ms - 5s | `*_test.go` in package | Component interactions |
| E2E | 5s - 60s | `test/e2e/` | Full application workflows |

## Running E2E Tests

E2E tests require tmux and are disabled by default. To run them:

```bash
# Enable and run E2E tests
CLAUDE_E2E_TESTS=1 go test -v ./test/e2e/...

# Run with timeout (recommended)
CLAUDE_E2E_TESTS=1 go test -v -timeout 5m ./test/e2e/...
```

### E2E Test Requirements

- tmux must be installed and accessible
- Tests create and destroy temporary tmux sessions
- Tests are isolated and clean up after themselves

## Writing Tests

### Using Test Helpers

```go
import "github.com/shitchell/claude-dashboard/test/helpers"

func TestSomething(t *testing.T) {
    // Load a fixture file
    data := helpers.LoadFixture(t, "minimal.jsonl")

    // Create a temp project directory
    tmpDir := t.TempDir()
    projectDir := helpers.CreateTempProjectDir(t, tmpDir, "/home/user/project")

    // Build a test session
    session := helpers.NewSessionBuilder().
        WithID("test-id").
        WithModel("claude-sonnet-4-20250514").
        WithProject("myproject", "/home/user/myproject").
        Build()

    // Use custom assertions
    helpers.AssertNoError(t, err)
    helpers.AssertEqual(t, got, want)
}
```

### Creating Session Fixtures Programmatically

```go
func TestWithDynamicSession(t *testing.T) {
    tmpDir := t.TempDir()

    // Build JSONL content
    content := helpers.NewJSONLBuilder().
        AddInit("session-id", "claude-sonnet-4-20250514", "/home/user/project").
        AddUserMessage("u1", "", "Hello").
        AddAssistantMessage("a1", "u1", "claude-sonnet-4-20250514", "Hi!").
        AddSummary("Greeting conversation", "a1").
        Build()

    // Write to file
    sessionPath := helpers.CreateTempSession(t, tmpDir, "session.jsonl", content)

    // Use in test...
}
```

### Testing Bubbletea Components

The project uses `teatest` for TUI testing:

```go
import (
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/x/exp/teatest"
)

func TestUINavigation(t *testing.T) {
    m := NewModel(config)

    tm := teatest.NewTestModel(t, m,
        teatest.WithInitialTermSize(80, 24))

    // Send keys
    tm.Send(tea.KeyMsg{Type: tea.KeyDown})

    // Wait for output
    teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
        return bytes.Contains(b, []byte("expected"))
    }, teatest.WithDuration(2*time.Second))

    // Quit
    tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
    tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
}
```

### Testing with Mocks

Example of using mock tmux runner:

```go
func TestNavigation(t *testing.T) {
    runner := helpers.NewMockTmuxRunner()
    runner.SetResponse("list-panes", []byte("output"), nil)

    nav := tmux.NewNavigatorWithRunner("select-pane", runner)

    err := nav.GoToPane("%0")

    // Verify mock was called
    if runner.GetCallCount() != 1 {
        t.Errorf("Expected 1 call, got %d", runner.GetCallCount())
    }
}
```

## Test Patterns

### Table-Driven Tests

```go
func TestParseModel(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {"opus", "claude-opus-4-5-20251101", "opus"},
        {"sonnet", "claude-sonnet-4-20250514", "sonnet"},
        {"unknown", "unknown-model", "unknown"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := ParseModel(tt.input)
            if got != tt.expected {
                t.Errorf("ParseModel(%q) = %q, want %q",
                    tt.input, got, tt.expected)
            }
        })
    }
}
```

### Using t.Helper()

```go
func createTestConfig(t *testing.T) *Config {
    t.Helper() // Marks this as a helper function

    config, err := LoadConfig(...)
    if err != nil {
        t.Fatalf("Failed to create config: %v", err)
    }
    return config
}
```

### Testing Error Cases

```go
func TestParseEmptyFile(t *testing.T) {
    parser := NewParser(nil)
    _, err := parser.Parse("testdata/empty.jsonl")

    if err != ErrEmptyFile {
        t.Errorf("Parse(empty) error = %v, want ErrEmptyFile", err)
    }
}
```

## CI Integration

Tests run automatically in CI on push and pull request. The CI pipeline:

1. Runs all unit and integration tests
2. Runs with race detector (`-race`)
3. Generates coverage report
4. E2E tests run in a separate job with tmux installed

### Running What CI Runs

```bash
# Simulate CI test run
go test -race -coverprofile=coverage.out ./...

# For E2E (if tmux available)
CLAUDE_E2E_TESTS=1 go test -v ./test/e2e/...
```

## Debugging Failing Tests

### Verbose Output

```bash
go test -v -run TestFailingTest ./internal/session/...
```

### With Test Logging

Tests use `t.Log()` and `t.Logf()` for debug output (visible with `-v`):

```go
func TestSomething(t *testing.T) {
    t.Log("Starting test...")
    result := DoSomething()
    t.Logf("Result: %+v", result)
}
```

### Capturing Pane Output in E2E

```go
func TestE2EDebug(t *testing.T) {
    h := NewTestHarness(t)
    pane := h.RunDashboard(projectsDir)

    // Capture and log output for debugging
    output := h.CapturePane(pane)
    t.Logf("Pane output:\n%s", output)
}
```

## Best Practices

1. **Keep tests fast**: Unit tests should run in milliseconds
2. **Use t.TempDir()**: Automatically cleaned up after test
3. **Use t.Helper()**: For better error messages in helper functions
4. **Test behavior, not implementation**: Focus on what, not how
5. **One assertion per concept**: Makes failures easier to diagnose
6. **Use descriptive test names**: `TestParseValidSession_WithSummary`
7. **Clean up resources**: Use `defer` or `t.Cleanup()`

## Coverage

To view coverage:

```bash
# Generate coverage
go test -coverprofile=coverage.out ./...

# View in browser
go tool cover -html=coverage.out

# View summary
go tool cover -func=coverage.out | grep total
```

Target coverage: ~80% for business logic, lower acceptable for UI code.

## Related Documentation

- [Testing Strategy](./testing-strategy.md) - Comprehensive testing philosophy and patterns
- [Architecture](./architecture.md) - System design and component interactions
