# Testing Guide

This document provides comprehensive guidance for testing claude-dashboard, including philosophy, patterns, tooling, and practical examples.

## Table of Contents

1. [Quick Start](#quick-start)
2. [Philosophy](#philosophy)
3. [Test Tiers](#test-tiers)
4. [Directory Structure](#directory-structure)
5. [Running Tests](#running-tests)
6. [Writing Tests](#writing-tests)
7. [Tooling](#tooling)
8. [E2E Testing](#e2e-testing)
9. [CI Integration](#ci-integration)
10. [Best Practices](#best-practices)

---

## Quick Start

```bash
# Run all unit and integration tests
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

# Run E2E tests (requires tmux and claude CLI)
go test -tags=e2e -v ./test/e2e/...
```

---

## Philosophy

### Core Principles

1. **Tests Prove Behavior, Not Implementation**
   - Tests should verify observable behavior and outcomes
   - Avoid testing internal implementation details that might change
   - Focus on "what" not "how"

2. **Tests Must Be Deterministic**
   - Same input always produces same output
   - No flaky tests - if a test fails intermittently, fix it or remove it
   - Control all external dependencies through mocking or fixtures

3. **Tests Document Intent**
   - Test names describe expected behavior in plain English
   - Test structure follows Arrange-Act-Assert pattern
   - Comments explain "why", code shows "what"

4. **Fast Feedback Loop**
   - Unit tests run in milliseconds
   - Integration tests run in seconds
   - E2E tests run in minutes (but provide high confidence)

5. **Bugs Require Tests**
   - Every bug fix includes a test that would have caught the bug
   - Tests prove the fix works and prevent regression

### Testing Goals

- **Confidence in Refactoring**: Change code without fear of breaking functionality
- **Living Documentation**: Tests explain how components work together
- **Regression Prevention**: Bugs stay fixed once they're fixed
- **Design Validation**: Tests expose design problems early

---

## Test Tiers

### Unit Tests

**Definition**: Tests that verify a single function, method, or component in isolation.

**Characteristics**:
- No external dependencies (filesystem, network, tmux, processes)
- Fast: < 100ms per test
- Self-contained: no shared state between tests
- Fine-grained: one assertion per logical concept

**When to Use**:
- Pure functions and algorithms
- Data transformation logic
- Configuration parsing
- Business logic isolated from I/O

**Examples**:
- `internal/session/filter_test.go` - Filter logic
- `internal/session/sort_test.go` - Sort comparators
- `internal/util/time_test.go` - Time formatting
- `internal/config/config_test.go` - Type conversions

### Integration Tests

**Definition**: Tests that verify multiple components working together, potentially with controlled external dependencies.

**Characteristics**:
- May use filesystem (via `t.TempDir()`)
- May use mock implementations of interfaces
- Medium speed: 100ms - 5s per test
- Test realistic data flows through the system

**When to Use**:
- Service layer interactions (scan -> parse -> cache -> return)
- UI model updates in response to messages
- Configuration loading and merging
- Component interactions through interfaces

**Examples**:
- `internal/session/service_test.go` - Full scan/parse/cache pipeline
- `internal/ui/integration_test.go` - TUI navigation with mocks
- `internal/tmux/navigate_test.go` - Navigator with mock runners
- `internal/tmux/matcher_test.go` - Process matching with mocks

### Component Tests (Bubbletea)

**Definition**: Tests that verify TUI components respond correctly to messages and produce expected output.

**Characteristics**:
- Use `teatest` package for controlled TUI testing
- Send messages and verify model state changes
- Medium speed: 100ms - 2s per test

**When to Use**:
- TUI model behavior in response to key presses
- View rendering at specific dimensions
- Navigation and cursor management
- State transitions (loading, error, ready)

**Examples**:
- `internal/ui/model_test.go` - Model state management
- `internal/ui/update_test.go` - Message handling
- `internal/ui/integration_test.go` - Full interaction flows

### End-to-End Tests

**Definition**: Tests that verify the complete application behavior with real external dependencies.

**Characteristics**:
- Spawn actual tmux sessions
- Can spawn real Claude Code instances
- Exercise the full binary
- Slow: 5s - 60s per test
- Require specific environment setup

**When to Use**:
- Critical user workflows (session navigation, tmux integration)
- Release verification
- Smoke testing after major changes

**Location**: `test/e2e/`

---

## Directory Structure

```
claude-dashboard/
├── main_test.go                    # Application entry point tests
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
│       ├── navigation_test.go      # Navigation E2E tests
│       └── clear_scenario_test.go  # Session mapping tests
└── internal/
    ├── config/*_test.go            # Config unit tests
    ├── session/*_test.go           # Session unit/integration tests
    ├── tmux/*_test.go              # Tmux unit/integration tests
    └── ui/*_test.go                # UI unit/integration tests
```

### Naming Conventions

| Convention | Example | Purpose |
|------------|---------|---------|
| `*_test.go` | `config_test.go` | Standard Go test files |
| `Test<Function>` | `TestParseDisplayMode` | Test function names |
| `t.Run("<description>")` | `t.Run("valid input")` | Subtests for variations |
| `testdata/` | `testdata/sessions/` | Test fixture data |

---

## Running Tests

### Standard Tests

```bash
# Run all tests (excludes E2E)
go test ./...

# Run with verbose output
go test -v ./...

# Run specific package
go test -v ./internal/session/...

# Run specific test
go test -v -run TestParseDisplayMode ./internal/config/...

# Run with race detector
go test -race ./...

# Run with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### E2E Tests

E2E tests are gated by:
1. **Build tag**: `//go:build e2e` - Tests won't compile without `-tags=e2e`
2. **Runtime checks**: Tests skip if `claude` or `tmux` CLI not available

```bash
# Run E2E tests (requires tmux and claude CLI)
go test -tags=e2e -v ./test/e2e/...

# Run with timeout (recommended)
go test -tags=e2e -v -timeout 5m ./test/e2e/...
```

**E2E Test Requirements**:
- tmux must be installed and accessible
- For Claude-spawning tests, the `claude` CLI must be installed
- Tests create and destroy temporary tmux sessions
- Tests are isolated and clean up after themselves

---

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

---

## Tooling

### Bubbletea Testing (teatest)

The project uses `github.com/charmbracelet/x/exp/teatest` for TUI testing.

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

### Mock Implementations

```go
// Define interface (if not already defined)
type CommandRunner interface {
    Run(args ...string) ([]byte, error)
}

// Create mock implementation
type mockRunner struct {
    output []byte
    err    error
    calls  [][]string
}

func (m *mockRunner) Run(args ...string) ([]byte, error) {
    m.calls = append(m.calls, args)
    return m.output, m.err
}

// Use in tests
func TestWithMock(t *testing.T) {
    mock := &mockRunner{
        output: []byte("expected output"),
        err:    nil,
    }

    component := NewComponent(mock)
    component.DoSomething()

    if len(mock.calls) != 1 {
        t.Errorf("Expected 1 call, got %d", len(mock.calls))
    }
}
```

---

## E2E Testing

### E2E Harness

The E2E harness (`test/e2e/harness.go`) provides utilities for E2E testing:

```go
func TestE2EWorkflow(t *testing.T) {
    h := NewTestHarness(t)  // Skips if E2E disabled or tmux unavailable

    // Create test session
    h.CreateSession("my-test")

    // Create test fixtures
    projectsDir := h.CreateTestProjectsDir()
    h.CreateTestSession(projectsDir, "/path/to/project", "session-id", "claude-sonnet-4-20250514", "Summary")

    // Run dashboard
    pane := h.RunDashboard(projectsDir)

    // Wait for content
    if !h.WaitForContent(pane, "expected", 10*time.Second) {
        t.Fatal("Content not found")
    }

    // Send keys
    h.SendKeys(pane, "j")  // Navigate down
    h.SendKeys(pane, "q")  // Quit
}
```

### Claude Instance Spawning

For tests that need real Claude instances:

```go
func TestWithClaude(t *testing.T) {
    SkipIfNoClaude(t)  // Skip if claude CLI not available

    h := NewTestHarness(t)
    h.CreateSession("claude-test")

    // Spawn a Claude instance
    instance := h.SpawnClaude("Say hello")

    // Send commands to Claude
    h.SendCommand(instance, "/clear")

    // Wait for new session after /clear
    newSessionID := h.WaitForNewSession(instance, 30*time.Second)

    // Spawn dashboard and verify
    dashboardPane := h.SpawnDashboard("")
    h.WaitForContent(dashboardPane, newSessionID[:8], 15*time.Second)
}
```

---

## CI Integration

### CI Pipeline

Tests run automatically in CI:

1. Unit and integration tests with race detector
2. Coverage report generation
3. E2E tests in separate job with tmux installed

### Running What CI Runs

```bash
# Simulate CI test run
go test -race -coverprofile=coverage.out ./...

# For E2E (if tmux and claude CLI available)
go test -tags=e2e -v ./test/e2e/...
```

### GitHub Actions Configuration

```yaml
name: Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - name: Run Tests
        run: go test -race -coverprofile=coverage.out ./...
      - name: Upload Coverage
        uses: codecov/codecov-action@v4

  e2e:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - name: Install tmux
        run: sudo apt-get install -y tmux
      - name: Run E2E Tests
        run: go test -tags=e2e -v ./test/e2e/...
```

---

## Best Practices

### General Guidelines

1. **Keep tests fast**: Unit tests should run in milliseconds
2. **Use t.TempDir()**: Automatically cleaned up after test
3. **Use t.Helper()**: For better error messages in helper functions
4. **Test behavior, not implementation**: Focus on what, not how
5. **One assertion per concept**: Makes failures easier to diagnose
6. **Use descriptive test names**: `TestParseValidSession_WithSummary`
7. **Clean up resources**: Use `defer` or `t.Cleanup()`

### E2E Test Guidelines

1. **Use polling, not fixed sleeps**: Use `WaitForContent()` or `require.Eventually()`
2. **Create isolated environments**: Each test should have its own tmux session
3. **Clean up after tests**: The harness handles cleanup via `t.Cleanup()`
4. **Set appropriate timeouts**: E2E tests need longer timeouts

### Assertion Best Practices

```go
// Good - descriptive error message
if got != want {
    t.Errorf("ParseDisplayMode(%q) = %v, want %v", input, got, want)
}

// Avoid - unhelpful error message
if got != want {
    t.Error("wrong result")
}
```

### Debugging Failing Tests

```bash
# Verbose output
go test -v -run TestFailingTest ./internal/session/...
```

```go
// Add debug logging in tests
func TestSomething(t *testing.T) {
    t.Log("Setting up test...")
    result := DoSomething()
    t.Logf("Result: %+v", result)
}
```

---

## Coverage

```bash
# Generate coverage
go test -coverprofile=coverage.out ./...

# View in browser
go tool cover -html=coverage.out

# View summary
go tool cover -func=coverage.out | grep total
```

**Target coverage**: ~80% for business logic, lower acceptable for UI code.

---

## Current Test Inventory

| Package | Test Files | Approximate Coverage |
|---------|------------|---------------------|
| `main` | 1 | Low |
| `internal/config` | 4 | Good |
| `internal/logging` | 1 | Good |
| `internal/session` | 8 | Good |
| `internal/tmux` | 7 | Good |
| `internal/ui` | 10 | Good |
| `internal/util` | 2 | Good |
| `test/e2e` | 3 | E2E coverage |
| `test/helpers` | 1 | Test utilities |

---

## Key Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/charmbracelet/bubbletea` | TUI framework |
| `github.com/charmbracelet/x/exp/teatest` | TUI testing |
| `github.com/stretchr/testify` | Assertions |
| `github.com/GianlucaP106/gotmux` | tmux control |
