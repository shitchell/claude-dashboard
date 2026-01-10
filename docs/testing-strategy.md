# Testing Strategy

This document defines the comprehensive testing strategy for claude-dashboard. It serves as a reference for writing, organizing, and running tests at all levels.

## 1. Philosophy

### 1.1 Core Principles

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

### 1.2 Testing Goals

- **Confidence in Refactoring**: Change code without fear of breaking functionality
- **Living Documentation**: Tests explain how components work together
- **Regression Prevention**: Bugs stay fixed once they're fixed
- **Design Validation**: Tests expose design problems early

## 2. Test Tiers

### 2.1 Unit Tests

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

**Examples in Codebase**:
- `internal/session/filter_test.go` - Testing filter logic
- `internal/session/sort_test.go` - Testing sort comparators
- `internal/util/time_test.go` - Testing time formatting
- `internal/config/config_test.go` - Testing type conversions

**Pattern**:
```go
func TestParseDisplayMode(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected DisplayMode
    }{
        {"valid list", "list", DisplayModeList},
        {"valid grid", "grid", DisplayModeGrid},
        {"invalid falls back", "unknown", DisplayModeList},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := ParseDisplayMode(tt.input)
            if got != tt.expected {
                t.Errorf("ParseDisplayMode(%q) = %v, want %v",
                    tt.input, got, tt.expected)
            }
        })
    }
}
```

### 2.2 Integration Tests

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

**Examples in Codebase**:
- `internal/session/service_test.go` - Full scan/parse/cache pipeline
- `internal/ui/integration_test.go` - TUI navigation with mocks
- `internal/tmux/navigate_test.go` - Navigator with mock runners
- `internal/tmux/matcher_test.go` - Process matching with mocks

**Pattern**:
```go
func TestIntegrationScanParseCacheReturn(t *testing.T) {
    // Arrange: create temp dirs and test files
    tmpDir := t.TempDir()
    projectsDir := filepath.Join(tmpDir, "projects")
    createTestSession(t, projectDir, "session1.jsonl", ...)

    // Create service with real components
    svc, err := NewService(&ServiceConfig{
        ProjectsDir: projectsDir,
        CacheDir:    cacheDir,
    })

    // Act: exercise the full pipeline
    sessions, err := svc.LoadAll()

    // Assert: verify end-to-end behavior
    if len(sessions) != 3 {
        t.Errorf("LoadAll() returned %d sessions, want 3", len(sessions))
    }
}
```

### 2.3 Component Tests (Bubbletea)

**Definition**: Tests that verify TUI components respond correctly to messages and produce expected output.

**Characteristics**:
- Use `teatest` package for controlled TUI testing
- Send messages and verify model state changes
- Optionally use golden files for output verification
- Medium speed: 100ms - 2s per test

**When to Use**:
- TUI model behavior in response to key presses
- View rendering at specific dimensions
- Navigation and cursor management
- State transitions (loading, error, ready)

**Examples in Codebase**:
- `internal/ui/model_test.go` - Model state management
- `internal/ui/update_test.go` - Message handling
- `internal/ui/integration_test.go` - Full interaction flows

**Pattern** (using teatest):
```go
func TestNavigationFlow(t *testing.T) {
    sessions := []*session.Session{{ID: "1"}, {ID: "2"}}
    m := createTestModel(sessions, mockNavigatorFactory.Create())

    tm := teatest.NewTestModel(t, m,
        teatest.WithInitialTermSize(80, 24))

    // Send navigation keys
    tm.Send(tea.KeyMsg{Type: tea.KeyDown})
    tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

    // Wait for expected output
    teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
        return bytes.Contains(b, []byte("session-2"))
    }, teatest.WithDuration(2*time.Second))

    tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
    tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
}
```

### 2.4 End-to-End Tests

**Definition**: Tests that verify the complete application behavior with real external dependencies.

**Characteristics**:
- Spawn actual tmux sessions
- Create real Claude session files
- Exercise the full binary
- Slow: 5s - 60s per test
- Require specific environment setup

**When to Use**:
- Critical user workflows (session navigation, tmux integration)
- Release verification
- Smoke testing after major changes

**Proposed Approach**:
See Section 4.2 for detailed E2E framework design.

## 3. Directory Structure

### 3.1 Current Structure

```
claude-dashboard/
├── main_test.go                    # Application entry point tests
├── internal/
│   ├── config/
│   │   ├── config_test.go          # Type conversion tests
│   │   ├── defaults_test.go        # Default value tests
│   │   ├── flags_test.go           # CLI flag parsing tests
│   │   └── load_test.go            # Config file loading tests
│   ├── logging/
│   │   └── logging_test.go         # Logger tests
│   ├── session/
│   │   ├── cache_test.go           # Cache operations
│   │   ├── filter_test.go          # Filter logic
│   │   ├── jsonl_test.go           # JSONL parsing
│   │   ├── parser_test.go          # Session file parsing
│   │   ├── scanner_test.go         # Directory scanning
│   │   ├── service_test.go         # Integration tests
│   │   ├── session_test.go         # Session struct tests
│   │   └── sort_test.go            # Sort logic
│   ├── tmux/
│   │   ├── exec_test.go            # Command execution
│   │   ├── matcher_test.go         # Session-to-pane matching
│   │   ├── memory_scanner_test.go  # Memory scanning
│   │   ├── navigate_test.go        # Pane navigation
│   │   ├── pane_test.go            # Pane operations
│   │   ├── process_test.go         # Process detection
│   │   └── state_test.go           # State management
│   ├── ui/
│   │   ├── columns_test.go         # Column rendering
│   │   ├── integration_test.go     # TUI integration tests
│   │   ├── keys_test.go            # Key binding tests
│   │   ├── layout_grid_test.go     # Grid layout
│   │   ├── layout_list_test.go     # List layout
│   │   ├── layout_test.go          # Layout logic
│   │   ├── model_test.go           # Model state
│   │   ├── styles_test.go          # Style definitions
│   │   ├── update_test.go          # Update handlers
│   │   └── view_test.go            # View rendering
│   └── util/
│       ├── path_test.go            # Path utilities
│       └── time_test.go            # Time utilities
└── testdata/                       # (to be created)
    ├── sessions/                   # Sample JSONL files
    └── golden/                     # Golden file outputs
```

### 3.2 Recommended Additions

```
claude-dashboard/
├── testdata/
│   ├── sessions/                   # Reusable session fixtures
│   │   ├── minimal.jsonl           # Minimal valid session
│   │   ├── full.jsonl              # All fields populated
│   │   ├── multimodel.jsonl        # Multiple model switches
│   │   └── large.jsonl             # Performance testing
│   └── golden/                     # Golden file outputs (if using)
│       └── TestFullOutput.golden
├── test/
│   ├── e2e/                        # End-to-end tests
│   │   ├── navigation_test.go      # Full navigation workflows
│   │   ├── tmux_test.go            # tmux integration
│   │   └── fixtures/               # E2E-specific fixtures
│   └── helpers/                    # Shared test utilities
│       ├── fixtures.go             # Fixture loading
│       ├── tmux.go                 # tmux test helpers
│       └── assertions.go           # Custom assertions
└── internal/
    └── testing/                    # Internal test utilities
        └── mocks/                  # Shared mock implementations
```

### 3.3 Naming Conventions

| Convention | Example | Purpose |
|------------|---------|---------|
| `*_test.go` | `config_test.go` | Standard Go test files |
| `Test<Function>` | `TestParseDisplayMode` | Test function names |
| `t.Run("<description>")` | `t.Run("valid input")` | Subtests for variations |
| `testdata/` | `testdata/sessions/` | Test fixture data |
| `*.golden` | `TestFullOutput.golden` | Golden file outputs |
| `mock_*.go` | `mock_runner.go` | Mock implementations |

## 4. Tooling

### 4.1 Bubbletea Testing (teatest)

The project already uses `github.com/charmbracelet/x/exp/teatest` for TUI testing.

**Installation**: Already in `go.mod`

**Key Features**:

1. **TestModel Creation**
   ```go
   tm := teatest.NewTestModel(t, model,
       teatest.WithInitialTermSize(80, 24))
   ```

2. **Sending Input**
   ```go
   // Send key messages
   tm.Send(tea.KeyMsg{Type: tea.KeyDown})
   tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

   // Type string input
   tm.Type("search query")
   ```

3. **Reading Output**
   ```go
   // Get current output (non-blocking)
   output := tm.Output()

   // Get final output (blocks until program exits)
   finalOutput := tm.FinalOutput(t)
   ```

4. **Waiting for Conditions**
   ```go
   teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
       return bytes.Contains(b, []byte("expected text"))
   }, teatest.WithDuration(2*time.Second),
      teatest.WithCheckInterval(50*time.Millisecond))
   ```

5. **Golden File Assertions**
   ```go
   out, _ := io.ReadAll(tm.FinalOutput(t))
   teatest.RequireEqualOutput(t, out)
   // Run with -update flag to regenerate golden files
   ```

6. **Final Model State**
   ```go
   fm := tm.FinalModel(t)
   model := fm.(Model)
   // Assert on model.cursorIndex, model.sessions, etc.
   ```

**Best Practices**:
- Set fixed terminal size for reproducible output
- Use `lipgloss.SetColorProfile(termenv.Ascii)` in CI for consistent colors
- Add `.gitattributes` entry: `*.golden -text` to preserve line endings

### 4.2 E2E Framework (Proposed)

For testing real tmux interactions and Claude session workflows, we recommend using a combination of approaches:

**Option A: gotmux Library** (Recommended)

Use [gotmux](https://github.com/GianlucaP106/gotmux) for programmatic tmux control:

```go
import "github.com/GianlucaP106/gotmux"

func TestE2ENavigation(t *testing.T) {
    if os.Getenv("TMUX") == "" {
        t.Skip("E2E tests require running inside tmux")
    }

    // Create a test session
    tmuxClient, err := gotmux.DefaultTmux()
    if err != nil {
        t.Fatalf("Failed to connect to tmux: %v", err)
    }

    session, err := tmuxClient.NewSession(&gotmux.SessionOptions{
        Name: "claude-dashboard-test",
        StartDirectory: t.TempDir(),
    })
    if err != nil {
        t.Fatalf("Failed to create test session: %v", err)
    }
    defer session.Kill()

    // Create test fixture files
    createE2ETestFixtures(t, session)

    // Run the dashboard in the session
    window, _ := session.GetWindowByIndex(0)
    pane, _ := window.GetPaneByIndex(0)
    pane.SendKeys("./claude-dashboard")

    // Interact and verify
    time.Sleep(500 * time.Millisecond)
    pane.SendKeys("j")  // Navigate down
    pane.SendKeys("Enter")  // Select

    // Verify navigation occurred
    // ...
}
```

**Option B: Shell-based Commands**

For simpler cases, direct tmux commands via the existing CommandRunner:

```go
func TestE2ETmuxNavigation(t *testing.T) {
    runner := NewRealCommandRunner()

    // Create test session
    output, err := runner.Run("new-session", "-d", "-s", "test")
    // ... setup and verification
}
```

**E2E Test Requirements**:
- Tests should be skipped unless `CLAUDE_E2E_TESTS=1` environment variable is set
- Tests must clean up all created tmux sessions/windows
- Use unique session names with test identifiers
- Timeout after 30 seconds to prevent hanging

### 4.3 Logging in Tests

**Current Approach**:
The project uses `internal/logging` for application logging. Tests can leverage this for verification.

**Test-Specific Logging**:

```go
func TestWithLogging(t *testing.T) {
    // Create test-specific logger
    tempDir := t.TempDir()
    logger, err := logging.NewLogger(&logging.Config{
        LogDir:  tempDir,
        Enabled: true,
        Level:   logging.LevelDebug,
    })
    if err != nil {
        t.Fatalf("Failed to create logger: %v", err)
    }
    defer logger.Close()

    // Use logger in component
    component := NewComponent(WithLogger(logger))

    // Execute test
    component.DoSomething()

    // Verify log output
    logContent, _ := os.ReadFile(filepath.Join(tempDir, "dashboard.log"))
    if !strings.Contains(string(logContent), "expected message") {
        t.Error("Expected log message not found")
    }
}
```

**Structured Test Output**:
For test reports, use Go's built-in test output with `-v` flag and `t.Log()`:

```go
t.Run("scenario name", func(t *testing.T) {
    t.Log("Setting up test fixtures")
    // setup...

    t.Log("Executing operation")
    result := operation()

    t.Logf("Result: %+v", result)
    // assertions...
})
```

### 4.4 Mock Implementations

**Current Mocks in Codebase**:

| Mock | Location | Purpose |
|------|----------|---------|
| `mockNavigatorRunner` | `tmux/navigate_test.go` | Mock tmux command execution |
| `MockNavigator` | `ui/integration_test.go` | Mock pane navigation |
| `mockMemoryScanner` | `tmux/memory_scanner_test.go` | Mock process memory scanning |
| `combinedMockRunner` | `tmux/matcher_test.go` | Mock multiple command types |

**Pattern for Creating Mocks**:

```go
// Define interface (if not already defined)
type CommandRunner interface {
    Run(args ...string) ([]byte, error)
}

// Create mock implementation
type mockRunner struct {
    output []byte
    err    error
    calls  [][]string  // Record all calls
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

    // Verify mock was called correctly
    if len(mock.calls) != 1 {
        t.Errorf("Expected 1 call, got %d", len(mock.calls))
    }
}
```

## 5. Writing Guidelines

### 5.1 Unit Test Patterns

**Table-Driven Tests** (preferred for multiple cases):
```go
func TestFunction(t *testing.T) {
    tests := []struct {
        name     string
        input    InputType
        expected OutputType
        wantErr  bool
    }{
        {"valid input", validInput, validOutput, false},
        {"invalid input", invalidInput, OutputType{}, true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Function(tt.input)

            if (err != nil) != tt.wantErr {
                t.Errorf("Function() error = %v, wantErr %v", err, tt.wantErr)
                return
            }

            if !tt.wantErr && got != tt.expected {
                t.Errorf("Function() = %v, want %v", got, tt.expected)
            }
        })
    }
}
```

**Subtest Groups** (for related scenarios):
```go
func TestSession(t *testing.T) {
    t.Run("creation", func(t *testing.T) {
        t.Run("with valid path", func(t *testing.T) { ... })
        t.Run("with invalid path", func(t *testing.T) { ... })
    })

    t.Run("serialization", func(t *testing.T) {
        t.Run("to metadata", func(t *testing.T) { ... })
        t.Run("from metadata", func(t *testing.T) { ... })
    })
}
```

### 5.2 Integration Test Patterns

**Setup and Teardown**:
```go
func TestServiceIntegration(t *testing.T) {
    // Setup
    tmpDir := t.TempDir()  // Automatically cleaned up
    setupTestFixtures(t, tmpDir)

    svc := createTestService(t, tmpDir)

    // Test multiple phases
    t.Run("initial load", func(t *testing.T) {
        sessions, err := svc.LoadAll()
        // assertions
    })

    t.Run("refresh after change", func(t *testing.T) {
        modifyFixtures(t, tmpDir)
        sessions, err := svc.Refresh(existingSessions)
        // assertions
    })
}
```

**Testing Error Paths**:
```go
func TestServiceErrors(t *testing.T) {
    t.Run("handles missing directory", func(t *testing.T) {
        svc := createServiceWithPath("/nonexistent/path")
        sessions, err := svc.LoadAll()

        // Should gracefully return empty, not error
        if err != nil {
            t.Errorf("unexpected error: %v", err)
        }
        if len(sessions) != 0 {
            t.Errorf("expected empty result, got %d", len(sessions))
        }
    })
}
```

### 5.3 E2E Test Patterns

**Test Isolation**:
```go
func TestE2EWorkflow(t *testing.T) {
    if os.Getenv("CLAUDE_E2E_TESTS") != "1" {
        t.Skip("E2E tests disabled (set CLAUDE_E2E_TESTS=1)")
    }

    // Create isolated test environment
    testID := fmt.Sprintf("test-%d", time.Now().UnixNano())
    cleanup := setupE2EEnvironment(t, testID)
    defer cleanup()

    // Run test
    // ...
}
```

**Timeout Handling**:
```go
func TestE2EWithTimeout(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    done := make(chan struct{})
    go func() {
        // Run test operations
        close(done)
    }()

    select {
    case <-done:
        // Test completed
    case <-ctx.Done():
        t.Fatal("Test timed out")
    }
}
```

### 5.4 Assertion Best Practices

**Use Descriptive Error Messages**:
```go
// Good
if got != want {
    t.Errorf("ParseDisplayMode(%q) = %v, want %v", input, got, want)
}

// Avoid
if got != want {
    t.Error("wrong result")
}
```

**Test One Concept Per Test**:
```go
// Good - separate tests for different behaviors
func TestFilter_ByProject(t *testing.T) { ... }
func TestFilter_ByAge(t *testing.T) { ... }
func TestFilter_Combined(t *testing.T) { ... }

// Avoid - testing multiple unrelated things
func TestFilter(t *testing.T) {
    // tests project filter, age filter, and combination in one
}
```

**Use `t.Helper()` for Test Utilities**:
```go
func createTestSession(t *testing.T, dir, name string) *Session {
    t.Helper()  // Ensures line numbers point to caller

    path := filepath.Join(dir, name)
    if err := os.WriteFile(path, content, 0644); err != nil {
        t.Fatalf("Failed to create test session: %v", err)
    }
    // ...
}
```

## 6. CI Integration

### 6.1 Test Commands

**Local Development**:
```bash
# Run all tests
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

**CI Pipeline**:
```bash
# Full test suite with race detection and coverage
go test -race -coverprofile=coverage.out -covermode=atomic ./...

# Update golden files (when output intentionally changes)
go test -v ./... -update

# E2E tests (separate job, requires tmux)
CLAUDE_E2E_TESTS=1 go test -v ./test/e2e/...
```

### 6.2 CI Configuration (GitHub Actions)

```yaml
# .github/workflows/test.yml
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
        with:
          files: ./coverage.out

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
        run: CLAUDE_E2E_TESTS=1 go test -v ./test/e2e/...
```

### 6.3 Pre-Commit Hooks

```bash
#!/bin/bash
# .git/hooks/pre-commit

# Run tests before committing
go test ./...
if [ $? -ne 0 ]; then
    echo "Tests failed. Commit aborted."
    exit 1
fi
```

## 7. Migration Plan (for Ticket 004)

This section summarizes the work needed to implement this testing strategy.

### 7.1 Phase 1: Foundation (Priority: High)

1. **Create Test Infrastructure**
   - [ ] Create `testdata/sessions/` directory with fixture files
   - [ ] Create `test/helpers/` package with shared utilities
   - [ ] Create `internal/testing/mocks/` for centralized mock definitions

2. **Standardize Existing Tests**
   - [ ] Review all existing tests for consistency with this strategy
   - [ ] Add missing `t.Helper()` calls to utility functions
   - [ ] Improve error messages to follow guidelines

3. **Add Test Coverage Tracking**
   - [ ] Set up coverage reporting in CI
   - [ ] Identify packages with low coverage

### 7.2 Phase 2: Component Testing (Priority: High)

1. **Enhance TUI Tests**
   - [ ] Add golden file tests for key views
   - [ ] Add tests for all keyboard shortcuts
   - [ ] Test search mode and filtering

2. **Add Missing Unit Tests**
   - [ ] `internal/constants/` package needs tests
   - [ ] Increase coverage in session parsing edge cases

### 7.3 Phase 3: Integration Testing (Priority: Medium)

1. **Service Integration Tests**
   - [ ] Test full refresh cycle with file changes
   - [ ] Test cache persistence and recovery
   - [ ] Test concurrent access scenarios

2. **tmux Integration Tests**
   - [ ] Test all navigation methods
   - [ ] Test session-to-pane matching accuracy
   - [ ] Test error handling (tmux not available, pane not found)

### 7.4 Phase 4: E2E Testing (Priority: Medium)

1. **E2E Framework Setup**
   - [ ] Add gotmux dependency
   - [ ] Create E2E test helpers
   - [ ] Set up E2E CI job

2. **Core E2E Tests**
   - [ ] Test application startup and shutdown
   - [ ] Test navigation to Claude session pane
   - [ ] Test session resume flow

### 7.5 Phase 5: Documentation and Maintenance (Priority: Low)

1. **Document Test Patterns**
   - [ ] Add examples to this document for new patterns discovered
   - [ ] Create contributor guide for writing tests

2. **Ongoing Maintenance**
   - [ ] Review and update golden files as needed
   - [ ] Monitor for flaky tests and fix immediately
   - [ ] Keep test dependencies updated

---

## Appendix A: Current Test Inventory

| Package | Test Files | Test Count | Coverage |
|---------|------------|------------|----------|
| `main` | 1 | 4 | Low |
| `internal/config` | 4 | ~25 | Good |
| `internal/logging` | 1 | ~10 | Good |
| `internal/session` | 8 | ~50 | Good |
| `internal/tmux` | 7 | ~40 | Good |
| `internal/ui` | 10 | ~80 | Good |
| `internal/util` | 2 | ~15 | Good |
| `internal/constants` | 0 | 0 | None |

## Appendix B: Key Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/charmbracelet/bubbletea` | v1.3.10 | TUI framework |
| `github.com/charmbracelet/x/exp/teatest` | latest | TUI testing |
| `github.com/charmbracelet/x/exp/golden` | latest | Golden file assertions |

## Appendix C: Reference Links

- [Writing Bubble Tea Tests](https://charm.land/blog/teatest/) - Official teatest guide
- [gotmux](https://github.com/GianlucaP106/gotmux) - Go library for tmux control
- [go-tmux](https://github.com/jubnzv/go-tmux) - Alternative tmux library
- [Go Testing Best Practices](https://go.dev/doc/tutorial/add-a-test) - Official Go testing guide
