# Claude Dashboard Architecture

## Overview

Claude Dashboard is a terminal user interface (TUI) application built with Go that provides a unified view of all Claude Code sessions. It enables browsing, filtering, sorting, and navigating to sessions across projects using tmux integration.

The application follows a clean, layered architecture with clear separation of concerns:

- **Entry Point**: `main.go` - Initialization and bootstrap
- **Configuration**: `internal/config/` - Config loading, merging, and validation
- **Session Management**: `internal/session/` - JSONL parsing, caching, filtering, and sorting
- **Process/Tmux Integration**: `internal/tmux/` - Pane detection and process matching
- **UI/Rendering**: `internal/ui/` - Bubbletea TUI with list and grid layouts
- **Utilities**: `internal/util/`, `internal/logging/`, `internal/constants/`

## Architecture Diagram

```mermaid
classDiagram
    direction TB

    class Main {
        +run(args []string) int
        +printUsage()
    }

    class Config {
        +Mode DisplayMode
        +Layout LayoutType
        +Columns []ColumnType
        +Sort SortConfig
        +Filter FilterConfig
        +Tmux TmuxConfig
        +Cache CacheConfig
        +Refresh RefreshConfig
        +Sessions SessionsConfig
        +syncFromStrings()
        +syncToStrings()
    }

    class DisplayMode {
        List
        Grid
    }

    class LayoutType {
        List
        Grid
    }

    class ColumnType {
        Name
        Preview
        Modified
        Status
        Project
        Messages
        Turns
        Model
    }

    class SortConfig {
        +Field SortField
        +Ascending bool
    }

    class FilterConfig {
        +MaxAge Duration
        +MinAge Duration
        +Project string
        +ExcludeExited bool
        +SearchText string
        +Model string
    }

    class Flags {
        +ConfigPath string
        +Mode string
        +Layout string
        +Columns string
        +Version bool
        +Help bool
        +ParseFlags(args) *Flags, error
        +Apply(cfg *Config)
    }

    class Logging {
        -globalLogger *Logger
        +Init(config *Config) error
        +Debug(format, args...)
        +Info(format, args...)
        +Warn(format, args...)
        +Error(format, args...)
        +Close() error
    }

    class Service {
        -scanner *Scanner
        -parser *Parser
        -cache *Cache
        +LoadAll() []*Session, error
        +Refresh(existing []*Session) []*Session, error
        +processResult(result ScanResult) *Session, error
        +GetSessionFilePaths() []string
    }

    class ServiceConfig {
        +ProjectsDir string
        +CacheDir string
        +ParserConfig *ParserConfig
    }

    class Scanner {
        +ProjectsDir string
        +Scan() []ScanResult, error
        +ScanMainSessionsOnly() []ScanResult, error
    }

    class Parser {
        -config ParserConfig
        +Parse(path string) *SessionMetadata, error
        +ParseLastEntry(path string) *LastEntry, error
        -readLastLine(file, size) []byte, error
    }

    class Cache {
        -path string
        -entries map[string]CacheEntry
        +Load() error
        +Save() error
        +Get(path string, mtime Time) *SessionMetadata
        +Set(path string, mtime Time, metadata *SessionMetadata)
        +Prune(validPaths map[string]struct{})
    }

    class Session {
        +ID string
        +FilePath string
        +ProjectPath string
        +ProjectName string
        +CWD string
        +Summary string
        +Model string
        +Preview string
        +MessageCount int
        +TurnCount int
        +ModTime Time
        +Status Status
        +TmuxPane string
        +ToMetadata() SessionMetadata
    }

    class Status {
        Exited
        Idle
        Active
    }

    class Filter {
        +ApplyFilters(sessions, cfg) []*Session
        +FilterByAge(sessions, maxAge) []*Session
        +FilterByProject(sessions, project) []*Session
        +FilterRunning(sessions) []*Session
    }

    class Sort {
        +ApplySorting(sessions, cfg) []*Session
        +SortByModTimeDesc(sessions) []*Session
        +SortByProjectAsc(sessions) []*Session
    }

    class SortField {
        ModTime
        ProjectName
        Status
        MessageCount
        TurnCount
        Summary
    }

    class Matcher {
        -refreshing bool
        -paneMap *PaneMap
        -processList *ProcessList
        -claudeProcesses []ClaudeProcess
        -pidToSessionID map[int]string
        -sessionFilePaths []string
        -pidCache *PIDCache
        -sessionWatcher *SessionWatcher
        -forceFullScan bool
        +Refresh() error
        +SetSessionFilePaths(paths []string)
        +MatchSessionToPane(session) *Pane
        +HasRunningProcess(session) bool
        +FindClaudeProcesses() []ClaudeProcess
        +GetPIDToSessionID() map[int]string
        +SetForceFullScan(force bool)
        +Close() error
    }

    class PIDCache {
        -mu sync.RWMutex
        -path string
        -version int
        -entries map[int]PIDCacheEntry
        -dirty bool
        +Load() error
        +Save() error
        +Get(pid int) string, bool
        +Set(pid int, sessionFile string)
        +Remove(pid int)
        +ValidatePIDs()
        +GetUncachedPIDs(allPIDs []int) []int
        +GetCachedMapping() map[int]string
        +Clear()
    }

    class SessionWatcher {
        -watcher *fsnotify.Watcher
        -dirs map[string]bool
        -onNewFile func(path string)
        +WatchDirectory(dir string) error
        +UnwatchDirectory(dir string) error
        +Start()
        +Stop() error
        +SyncWatchedDirs(newDirs []string)
    }

    class PaneMap {
        -byTTY map[string]*Pane
        -byID map[string]*Pane
        +Discover() error
        +GetByTTY(tty string) *Pane
        +GetByID(id string) *Pane
    }

    class Pane {
        +TTY string
        +ID string
        +SessionName string
        +WindowIndex string
        +WindowName string
        +PaneIndex string
        +PaneTitle string
        +Description() string
    }

    class ProcessList {
        -processes map[int]Process
        +Discover() error
        +All() []Process
    }

    class ClaudeProcess {
        +Process Process
        +SessionID string
        +CWD string
    }

    class Navigator {
        -method string
        +GoToPane(paneID string) error
        +ResumeInNewPane(sessionID, cwd string) error
        +IsAvailable() bool
        +Method() string
    }

    class StateModule {
        +DetermineStatus(lastEntry, hasProcess) Status
        +DetermineStatusFromSession(session, matcher, parser) Status
        +UpdateSessionStatus(session, matcher, parser) *Session
        +UpdateAllSessionStatuses(sessions, matcher, parser)
    }

    class Model {
        -sessions []*Session
        -filteredSessions []*Session
        -cursorIndex int
        -viewMode ViewMode
        -layout Layout
        -matcher *Matcher
        -service *Service
        -config *Config
        +Init() tea.Cmd
        +Update(msg) tea.Model, tea.Cmd
        +View() string
        +SelectedSession() *Session
    }

    class ViewMode {
        List
        Grid
        Help
        Search
    }

    class Layout {
        +Render(sessions, cursor, width, height, styles, cfg) string
        +HandleKey(msg, sessionCount, cursor) int, bool
        +Name() string
    }

    class ListLayout {
        -keys KeyMap
        +Render(sessions, cursor, width, height, styles, cfg) string
        +HandleKey(msg, sessionCount, cursor) int, bool
    }

    class GridLayout {
        -keys KeyMap
        -selectedRow int
        -selectedCol int
        +Render(sessions, cursor, width, height, styles, cfg) string
        +HandleKey(msg, sessionCount, cursor) int, bool
    }

    class Styles {
        +HeaderStyle lipgloss.Style
        +FooterStyle lipgloss.Style
        +SelectedRowStyle lipgloss.Style
        +InactiveRowStyle lipgloss.Style
    }

    class Messages {
        +sessionsLoadedMsg
        +sessionsRefreshedMsg
        +statusUpdatedMsg
        +refreshTickMsg
        +matcherRefreshedMsg
        +navigationResultMsg
        +errorMsg
        +sessionSelectedMsg
    }

    Main --> Config: loads
    Main --> Flags: parses
    Main --> Logging: initializes
    Main --> Service: creates
    Main --> Model: creates and runs

    Config --> DisplayMode: uses
    Config --> LayoutType: uses
    Config --> ColumnType: uses
    Config --> SortConfig: contains
    Config --> FilterConfig: contains
    Config --> Flags: applies from

    Service --> Scanner: uses
    Service --> Parser: uses
    Service --> Cache: uses
    Service --> Session: returns

    Scanner --> ScanResult: returns

    Parser --> SessionMetadata: returns
    Parser --> LastEntry: returns

    Cache --> SessionMetadata: stores/retrieves

    Session --> Status: has

    Filter --> Session: filters
    Sort --> Session: sorts
    Sort --> SortField: uses

    Matcher --> PaneMap: uses
    Matcher --> ProcessList: uses
    Matcher --> ClaudeProcess: tracks
    Matcher --> Pane: matches to
    Matcher --> PIDCache: caches PIDs
    Matcher --> SessionWatcher: watches dirs

    PaneMap --> Pane: contains

    ProcessList --> Process: contains

    Navigator --> Pane: navigates to
    Navigator --> PaneNavigator: implements

    StateModule --> Session: determines status for
    StateModule --> Parser: uses
    StateModule --> Matcher: uses

    Model --> Service: loads sessions from
    Model --> Config: configured by
    Model --> Layout: uses for rendering
    Model --> Matcher: updates status via
    Model --> ViewMode: tracks
    Model --> Session: displays

    Layout <|-- ListLayout: implements
    Layout <|-- GridLayout: implements

    ListLayout --> Styles: uses
    GridLayout --> Styles: uses

    Model --> Messages: handles
    Model --> Navigator: uses for navigation
```

## Component Descriptions

### 1. **Entry Point** (`main.go`)

**Purpose**: Application bootstrap and initialization

**Key Functions**:
- `main()` - Entry point that calls `run()`
- `run(args []string) int` - Main application logic:
  1. Initializes logging
  2. Parses command-line flags
  3. Loads configuration (defaults <- file <- flags)
  4. Creates session service
  5. Creates and runs the Bubbletea TUI model

**Exit Codes**:
- `0`: Success
- `1`: Configuration error
- `2`: Runtime error

---

### 2. **Configuration** (`internal/config/`)

**Purpose**: Load, validate, and manage application configuration

**Key Components**:

#### `config.go` - Core Configuration Types
- **Config**: Main configuration struct combining all settings
- **DisplayMode**: Enum (List or Grid)
- **LayoutType**: Enum (List or Grid)
- **ColumnType**: Enum (Name, Preview, Modified, Status, Project, Messages, Turns, Model)
- **SortConfig**: Specifies sort field and direction
- **FilterConfig**: Specifies filtering criteria (project, age, status, search)
- **TmuxConfig**: Enables/disables tmux integration
- **CacheConfig**: Cache settings
- **RefreshConfig**: Auto-refresh interval
- **SessionsConfig**: Session discovery settings

#### `load.go` - Configuration Loading
- **Load(configPath string, flags *Flags)**: Main entry point
  - Loads defaults
  - Merges file configuration
  - Applies CLI flag overrides
  - Validates final config
- **findConfigFile()**: Searches standard config locations
- **loadFile()**: Loads YAML configuration from file
- **validate()**: Validates configuration values

#### `flags.go` - CLI Flag Parsing
- **ParseFlags(args []string)**: Parses command-line arguments
- **Flags** struct: Holds parsed CLI options
- **Apply(cfg *Config)**: Applies flag overrides to config

#### `defaults.go` - Default Values
- **Defaults()**: Returns configuration with sensible defaults
- Defaults use standard paths (~/.claude/projects, ~/.config/claude-dashboard, etc.)

---

### 3. **Session Management** (`internal/session/`)

**Purpose**: Discover, parse, cache, filter, and sort sessions

#### `session.go` - Core Data Structures
- **Session**: Complete session information
  - ID, FilePath, ProjectPath, ProjectName, CWD, Summary, Model
  - MessageCount, TurnCount, ModTime
  - Status (Exited/Idle/Active)
  - TmuxPane (pane ID if running)
- **SessionMetadata**: Cacheable subset (excludes Status, TmuxPane)
- **Status**: Enum (Exited, Idle, Active)

#### `service.go` - Session Service
- **Service**: Orchestrates scanning, parsing, and caching
  - **LoadAll()**: Initial load of all sessions
  - **Refresh(existing)**: Efficient update checking modification times
  - **processResult()**: Converts ScanResult to Session using cache
  - **GetSessionFilePaths()**: Returns paths for memory scanning

#### `scanner.go` - File Discovery
- **Scanner**: Recursively finds JSONL files in ~/.claude/projects
  - **ScanMainSessionsOnly()**: Scans and returns session file info
  - **ScanResult**: Contains FilePath, SessionID, ModTime, etc.

#### `parser.go` - JSONL Parsing
- **Parser**: Extracts metadata from JSONL files without full parsing
  - **Parse()**: Reads entire file, counts messages/turns, extracts summary
  - **ParseLastEntry()**: Reads only last line for quick status check
  - Handles malformed JSON gracefully
  - Extracts session ID, project path, model, CWD, preview

#### `jsonl.go` - Message Type Definitions
- **MessageType**: Enum (System, User, Assistant, Summary, Result, etc.)
- **SystemLine, UserLine, AssistantLine, etc.**: Specific message type structures
- **ContentBlock**: Message content (text, thinking, tool_use)
- **LastEntry**: Quick last-line state summary

#### `cache.go` - Session Caching
- **Cache**: Thread-safe mtime-based caching
  - **Load()/Save()**: File persistence (JSON format)
  - **Get()/Set()**: In-memory cache operations
  - **Prune()**: Remove stale entries
  - Version tracking for format changes
  - Avoids re-parsing unchanged files

#### `filter.go` - Session Filtering
- **FilterConfig**: Specifies filter criteria
- **ApplyFilters()**: Applies all active filters (AND logic)
  - By age (MaxAge, MinAge)
  - By project (path or name, case-insensitive)
  - By status (Active, Idle, Exited)
  - By search text (Summary, Preview, ProjectName)
  - By model
  - By running status (ExcludeExited)

#### `sort.go` - Session Sorting
- **SortField**: Enum (ModTime, ProjectName, Status, MessageCount, TurnCount, Summary)
- **SortConfig**: Field and direction (ascending/descending)
- **ApplySorting()**: In-place sort with custom comparators
  - Status priority: Active > Idle > Exited
  - Case-insensitive text sorting

---

### 4. **Process and Tmux Integration** (`internal/tmux/`)

**Purpose**: Find running Claude processes and match them to tmux panes

#### `pane.go` - Pane Discovery
- **Pane**: Represents a tmux pane
  - TTY, ID, SessionName, WindowIndex, WindowName, PaneIndex, PaneTitle
  - **Description()**: Human-readable location string
- **PaneMap**: Thread-safe TTY/ID -> Pane mapping
  - **Discover()**: Queries `tmux list-panes` and populates map
  - **GetByTTY()/GetByID()**: Lookup methods
  - Normalizes TTY paths (strips /dev/ prefix)

#### `process.go` - Process Discovery
- **Process**: Represents a running process
  - PID, TTY, Command, CWD
- **ProcessList**: Discovers and caches running processes
  - **Discover()**: Uses `ps` to list processes
  - **All()**: Returns all processes

#### `matcher.go` - Session-to-Pane Matching
- **Matcher**: Coordinates pane and process discovery
  - Maintains PaneMap and ProcessList
  - Uses memory scanning as sole source for session-to-PID mapping
  - Session-to-pane derived on-the-fly: sessionID -> PID -> TTY -> pane
  - Uses `refreshing` flag to prevent concurrent refresh cycles (memory scanning is slow)
  - **PID Caching**: Uses PIDCache to avoid redundant memory scans for known PIDs
  - **fsnotify Integration**: Uses SessionWatcher to detect new session files (e.g., after /clear)
  - **Refresh()**: Updates all mappings (panes, processes, memory scan)
  - **MatchSessionToPane()**: Finds pane for session via PID lookup
  - **HasRunningProcess()**: Checks if session is owned by a PID
  - **FindClaudeProcesses()**: Identifies Claude processes
  - **SetSessionFilePaths()**: Configures paths for memory scanning
  - **SetForceFullScan()**: Bypasses cache on next refresh (triggered by 'r' key)
  - **Close()**: Releases resources (saves cache, stops watcher)
- **ClaudeProcess**: A process running Claude with extracted SessionID

#### `pid_cache.go` - PID-to-Session Caching
- **PIDCache**: Thread-safe cache for PID -> session file mappings
  - Stored in XDG_RUNTIME_DIR (ephemeral, cleared on reboot)
  - Fallback to /tmp/claude-dashboard-$UID/ if XDG_RUNTIME_DIR not set
  - Uses flock for concurrent write safety
  - Version field for cache format migration
  - **Load()**: Loads cache from disk, validates PIDs still exist
  - **Save()**: Atomic write (temp file + rename) with flock
  - **Get()/Set()**: In-memory cache operations
  - **ValidatePIDs()**: Removes entries for dead processes
  - **GetUncachedPIDs()**: Returns PIDs not in cache for scanning
  - **Clear()**: Clears all entries (cache buster)

#### `session_watcher.go` - fsnotify Session File Watcher
- **SessionWatcher**: Monitors session directories for new .jsonl files
  - Uses fsnotify for efficient file system event monitoring
  - Detects CREATE events for UUID-patterned .jsonl files
  - Triggers callback when new session files appear (e.g., after /clear)
  - **WatchDirectory()**: Adds directory to watch list
  - **SyncWatchedDirs()**: Syncs watched dirs with current session paths
  - **Start()**: Starts event processing goroutine
  - **Stop()**: Stops watcher and releases resources

#### `navigate.go` - Pane Navigation
- **Navigator**: Switches to or creates tmux panes
  - **GoToPane()**: Switches to existing pane (select-pane or switch-client)
  - **ResumeInNewPane()**: Creates new pane and runs `claude --resume`
  - **IsAvailable()**: Checks if tmux is available
  - Supports two navigation methods:
    - `select-pane`: Direct selection (same session)
    - `switch-client`: Client switching (cross-session)

#### `state.go` - Status Determination
- **DetermineStatus()**: Determines session status from last entry + running process
  - Active: Claude is responding or tool is running
  - Idle: Waiting for input or session completed
  - Exited: No running process
  - Uses message types and recency heuristics
- **UpdateSessionStatus()**: Updates a session's status and TmuxPane fields
- **UpdateAllSessionStatuses()**: Bulk status update (refreshes matcher once)

#### `exec.go` - Command Execution
- **CommandRunner interface**: Abstraction for executing tmux commands
- **runTmux()**: Low-level tmux command execution
- Checks TMUX env var and tmux availability
- Error types: ErrNotInTmux, ErrTmuxNotFound

#### `memory_scanner.go` - Process Memory Scanning
- **MemoryScanner**: Scans process memory to find session IDs
  - Reads /proc/{pid}/maps and /proc/{pid}/mem
  - Uses NULL-prefixed patterns (\x00 + full path) to avoid false positives
  - Only scans exactly 256KB memory regions (V8's session path storage regions)
  - Filters to UUID-patterned session files (skips agent-*.jsonl)
  - **ScanWithCombinedPattern()**: Optimized scanning with CWD filtering and combined regex
    - Pre-filters session paths by process CWD (~200 -> ~16 paths)
    - Uses single combined alternation regex \x00(path1|path2|...) instead of per-pattern loop
    - Achieves ~22x speedup (3.3s -> 150ms per scan)
  - **ScanUntilMatch()**: Legacy method scanning regions one-by-one with early exit
  - **ReadAllMemory()**: Reads all eligible regions into single buffer
  - **ScanAllPIDsForSessions()**: Main entry point for batch scanning
  - **encodePath()**: Converts filesystem paths to Claude's .claude/projects/ encoding
  - **filterSessionPathsByCWD()**: Filters session paths by process working directory
  - **buildCombinedPattern()**: Creates single regex from multiple session paths
  - Used as sole source for PID-to-SessionID mapping (no CWD fallback)

---

### 5. **UI/TUI** (`internal/ui/`)

**Purpose**: Render and interact with the terminal interface

#### `model.go` - Main TUI Model
- **Model**: Bubbletea model (the hub of the UI)
  - **sessions**: All loaded sessions
  - **filteredSessions**: After applying filters
  - **cursorIndex**: Currently selected session index
  - **layout**: Current display layout (List or Grid)
  - **matcher**: For status updates
  - **viewMode**: Current view (List, Grid, Help, Search)
  - **uiMode**: Full screen or side-panel
  - **sortConfig/filterConfig**: Active sorting and filtering
  - **searchQuery**: Current search text
  - **loading/refreshing/lastError**: Status flags
  - **Init()**: Starts loading and refresh ticker
  - **SelectedSession()**: Get currently selected session
  - **applyFiltersAndSort()**: Applies current filters and sort
  - **restoreCursor()**: Restores cursor position by session ID
  - **updateUIMode()**: Switches between normal and side-panel based on width
  - **switchLayout()/SetLayout()**: Changes display layout

#### `update.go` - Message Handling
- **Update(msg)**: Main update loop handler
- Handles messages:
  - **WindowSizeMsg**: Window resize
  - **sessionsLoadedMsg**: Initial load complete
  - **sessionsRefreshedMsg**: Refresh tick results
  - **statusUpdatedMsg**: Status update results
  - **refreshTickMsg**: Auto-refresh timer
  - **KeyMsg**: Keyboard input
  - **errorMsg, matcherRefreshedMsg**: Status updates
- **handleSessionsLoaded()**: Process initial load, apply filters/sort
- **handleRefreshTick()**: Schedule next refresh and matcher refresh
- **loadSessionsCmd()/refreshSessionsCmd()**: Async command generators
- **updateStatusCmd()**: Updates session statuses via tmux matcher
- **tickCmd()**: Returns timer command for auto-refresh

#### `view.go` - Rendering
- **View()**: Main render function
  - Renders based on state hierarchy (loading -> error -> content)
  - Calls layout.Render() for main content
  - Assembles header + content + footer
- **renderFullView()**: Assembles header, content, footer
- **renderLayoutContent()**: Delegates to layout system
- **renderLoading(), renderError(), renderEmpty()**: State-specific renders

#### `layout.go` - Layout Interface
- **Layout interface**: Defines layout system
  - **Render()**: Render sessions to string
  - **HandleKey()**: Process keyboard navigation
  - **Name()**: Display name (List/Grid)
- **LayoutType enum**: List or Grid
- Implementations: ListLayout, GridLayout

#### `layout_list.go` - List Layout
- **ListLayout**: Vertical list display
  - **Render()**: Creates table with configurable columns
  - **HandleKey()**: Handles j/k/arrows for navigation
  - Supports column visibility and width configuration
  - Shows status indicator, name, preview, modified time, etc.

#### `layout_grid.go` - Grid Layout
- **GridLayout**: Card-based grid display
  - **Render()**: Creates cards arranged in grid
  - **HandleKey()**: Handles hjkl/arrows for 2D navigation
  - Supports side panel with session list
  - Shows status, summary, message count on cards

#### `columns.go` - Column Management
- **ColumnConfig**: Configure which columns to show
- **FormatColumn()**: Format session data for display
- **MeasureColumn()**: Calculate column widths
- Width calculation and truncation

#### `styles.go` - Styling
- **Styles struct**: Colors, bold, underline, etc.
- **NewStyles()**: Create default styles
- **NewSidePanelStyles()**: Create side-panel optimized styles
- Uses lipgloss for styling

#### `keys.go` - Keybindings
- **KeyMap**: Map of action names to key sequences
- **DefaultKeyMap()**: Standard keybindings
  - Navigation: j/k/h/l, arrows
  - Enter: Select/navigate
  - r: Refresh
  - s: Sort
  - f: Filter
  - /: Search
  - ?: Help
  - q/Esc: Quit

#### `messages.go` - Custom Messages
- **sessionsLoadedMsg**: Initial load complete
- **sessionsRefreshedMsg**: Refresh tick results
- **statusUpdatedMsg**: Status update complete
- **refreshTickMsg**: Auto-refresh timer
- **navigationResultMsg**: Navigation result
- **matcherRefreshedMsg**: Matcher refresh complete
- **errorMsg**: Error occurred
- **sessionSelectedMsg**: Session selected

---

### 6. **Utilities**

#### `internal/logging/` - File-Based Logging
- **Logger**: Thread-safe file-based logger (avoids stdout/stderr which corrupt TUI)
- **Level enum**: Debug, Info, Warn, Error
- **Init()**: Initialize global logger
- **Package functions**: Debug(), Info(), Warn(), Error(), Close()
- Creates log file at ~/.cache/claude-dashboard/logs/

#### `internal/constants/` - Configuration Constants
- **Version**: Application version
- **AppName**: Application name for display
- **Paths**: Default directories (projects, config, cache)
- **UI defaults**: Column widths, preview length, etc.
- **Refresh constants**: Default intervals

#### `internal/util/` - Helper Functions
- **path.go**: Path utilities
  - **DecodeProjectPath()**: URL-decode project path
  - **ProjectNameFromPath()**: Extract project name from path
- **time.go**: Time formatting
  - **FormatTime()**: Human-readable time display

---

## Data Flow

### 1. **Startup Flow**

```
main()
  |
run()
  |-> logging.Init()
  |-> config.ParseFlags()
  |-> config.Load() [defaults <- file <- flags]
  |-> session.NewService()
  |   |-> NewScanner()
  |   |-> NewParser()
  |   +-> NewCache()
  |-> ui.NewModel()
  |   |-> NewStyles()
  |   |-> NewListLayout() or NewGridLayout()
  |   +-> tmux.NewMatcher()
  +-> tea.NewProgram().Run()
      +-> Model.Init()
          |-> loadSessionsCmd()
          +-> tickCmd()
```

### 2. **Session Loading Flow**

```
loadSessionsCmd()
  |
Service.LoadAll()
  |-> Scanner.ScanMainSessionsOnly()
  |   +-> Returns: []ScanResult {FilePath, SessionID, ModTime, ...}
  |-> For each ScanResult:
  |   +-> processResult()
  |       |-> Cache.Get() [cache hit?]
  |       |   +-> Return cached SessionMetadata
  |       +-> Cache.miss:
  |           +-> Parser.Parse(filePath)
  |               |-> Read JSONL line-by-line
  |               |-> Extract: ID, ProjectPath, CWD, Model, Summary, Preview
  |               |-> Count messages and turns
  |               +-> Return: SessionMetadata
  |           +-> Cache.Set()
  +-> ApplySorting() [by ModTime, descending]

Model.handleSessionsLoaded()
  |-> Store sessions
  |-> applyFiltersAndSort()
  |   |-> Filter.ApplyFilters()
  |   +-> Sort.ApplySorting()
  +-> updateStatusCmd()
      +-> StateModule.UpdateAllSessionStatuses()
          |-> Matcher.Refresh()
          |   |-> PaneMap.Discover()
          |   |   +-> tmux list-panes -> Parse -> Map TTY->Pane
          |   |-> ProcessList.Discover()
          |   |   +-> ps -o pid,tty,command -> Parse -> Store
          |   |-> Find Claude processes
          |   +-> Match processes to panes
          +-> For each Session:
              +-> UpdateSessionStatus()
                  |-> DetermineStatus(lastEntry, hasProcess)
                  |   +-> Return: Active/Idle/Exited
                  +-> Find matching pane ID
```

### 3. **Refresh Flow** (on timer tick)

```
handleRefreshTick()
  |-> refreshSessionsCmd()
  |   +-> Service.Refresh(existing)
  |       |-> Scanner.ScanMainSessionsOnly()
  |       |-> Compare file mtimes
  |       |-> Reuse unchanged sessions
  |       +-> Re-parse changed/new sessions
  +-> refreshMatcherCmd()
      +-> Matcher.Refresh()
          +-> [Same as above]

Model.handleSessionsRefreshed()
  |-> Save cursor position
  |-> Update sessions
  |-> applyFiltersAndSort()
  +-> restoreCursor()
```

### 4. **Rendering Flow**

```
Model.View()
  |-> Check state (loading, error, empty)
  +-> renderFullView()
      |-> viewHeader()
      |   +-> Show session count, status summary
      |-> renderLayoutContent()
      |   +-> Layout.Render(filteredSessions, cursorIndex, width, height)
      |       |-> ListLayout: Create table with selected highlight
      |       +-> GridLayout: Create card grid with selection
      +-> viewFooter()
          +-> Show active count, refresh status, help hint
```

### 5. **Navigation Flow**

```
Model.Update(SessionSelectedMsg)
  |-> SelectedSession()
  |-> Navigator.IsAvailable()
  +-> Navigator.GoToPane(paneID)
      |-> Verify pane exists
      |-> Build tmux command (select-pane or switch-client)
      +-> Execute tmux command
```

---

## Key Design Patterns

### 1. **Service Layer Pattern**
- **Service** orchestrates Scanner, Parser, and Cache
- Provides clean interface for UI (LoadAll, Refresh)
- Handles cache invalidation and dirty tracking

### 2. **Filtering and Sorting Composition**
- Filters and sorts are separate, composable operations
- Multiple filters combined with AND logic
- Can be applied independently or together

### 3. **State Machine (Session Status)**
- Status determined by combining:
  - Process existence (from matcher)
  - Last JSONL message type (from parser)
  - Message timestamp recency
- Clear transitions: Exited -> Idle -> Active

### 4. **Layout System**
- Layout interface enables pluggable display strategies
- List and Grid layouts implement same interface
- Layout-specific navigation logic encapsulated

### 5. **Async Command Pattern (Bubbletea)**
- Long-running operations (load, refresh, matcher) are async
- Commands return messages processed by Update()
- UI remains responsive during background operations

### 6. **Cache Invalidation**
- Mtime-based validation (fast check)
- Cache entries marked dirty when modified
- Atomic save operations (Write to temp, rename)
- Version tracking for format changes

### 7. **Process Matching**
- Single strategy: Memory scanning with NULL-prefixed patterns
  - Reads /proc/{pid}/mem for each Claude process
  - Searches for \x00 + full session path in memory
  - NULL prefix discriminates true ownership from paths in chat history
  - Only scans regions <= 256KB (V8's session storage regions)
  - Filters to UUID-patterned sessions (skips agent-*.jsonl)

### 8. **Configuration Cascade**
- Priority: Defaults < File < Flags
- Each layer overrides previous
- Validation runs on final merged config

---

## Error Handling Strategy

1. **Non-Fatal Errors**:
   - Failed to read session file -> Skip and log
   - Failed to load cache -> Start fresh
   - Failed to discover tmux panes -> Continue without status
   - Permission denied on directories -> Return empty

2. **Fatal Errors**:
   - Invalid config -> Exit with error
   - Session service creation fails -> Exit with error
   - Bubbletea program error -> Exit with error

3. **User-Facing Errors**:
   - Displayed in error state
   - Can be cleared by user action
   - Logged to file

---

## Concurrency and Thread Safety

1. **Logger**: Mutex-protected singleton
2. **Cache**: Mutex-protected map with atomic saves
3. **PaneMap**: RWMutex for concurrent reads
4. **Model**: Immutable during message handling (Bubbletea pattern)
5. **Matcher**: Mutex-protected state during refresh

---

## Testing Infrastructure

### Test Packages

#### `test/helpers/` - Shared Test Utilities

**Purpose**: Reusable test fixtures, assertions, and mocks

**Key Components**:

- **SessionBuilder**: Fluent API for creating test sessions
  - `WithID()`, `WithModel()`, `WithProject()`, `WithSummary()`, etc.
  - `Build()` returns configured `*session.Session`

- **JSONLEntryBuilder**: Fluent API for creating JSONL test data
  - `AddInit()`, `AddUserMessage()`, `AddAssistantMessage()`, `AddSummary()`, `AddResult()`
  - `Build()` returns JSONL string, `WriteToFile()` writes to path

- **MockTmuxRunner**: Mock implementation of tmux command runner
  - `SetResponse()` configures command responses
  - `GetCallCount()`, `GetLastCall()` for verification
  - `Reset()` clears state between tests

- **TmuxTestSession**: Helper for real tmux integration tests
  - `SendKeys()`, `CapturePane()`, `WaitForContent()`
  - Automatic cleanup on test completion

- **Custom Assertions**: `AssertEqual()`, `AssertContains()`, `AssertSessionsEqual()`, etc.

#### `test/e2e/` - End-to-End Tests

**Purpose**: Test full application with real tmux and Claude sessions

**Key Components**:

- **TestHarness**: E2E test orchestrator using gotmux library (streamlined API)
  - `CreateSession()`: Creates isolated tmux session
  - `RunDashboard()`: Launches dashboard binary in test pane
  - `SpawnClaude()`: Spawns real Claude Code instance, returns `ClaudeInstance`
  - `SpawnDashboard()`: Spawns dashboard in new window
  - `WaitForContent()`: Polls pane output for expected content
  - `WaitForSessionFile()`: Detects new Claude session files
  - `SnapshotSessionFiles()`: Captures session file list for comparison
  - `WaitForSessionSince()`: Detects new sessions created after a snapshot
  - `SendKeys()`: Simulates keyboard input
  - `LogProof()`: Captures and logs pane content as test evidence
  - `Cleanup()`: Tears down test resources

- **ClaudeInstance**: Represents a spawned Claude Code process
  - `Pane`: The tmux pane where Claude is running
  - `SessionID`: Detected from session file after spawn
  - `PID`: Process ID of the Claude process
  - `SessionFile`: Path to the session's .jsonl file

- **E2E Tests**: `navigation_test.go`, `clear_scenario_test.go`
  - Navigation tests: startup, j/k movement, search, quit, help
  - `/clear` scenario: Tests session mapping after `/clear` command

**Gating**: E2E tests are gated by build tags (`//go:build e2e`). Run with `go test -tags=e2e ./test/e2e/...`. Tests skip at runtime if `claude` CLI is not available.

#### `testdata/` - Test Fixtures

- `testdata/sessions/`: JSONL session fixtures
  - `minimal.jsonl`: Minimal valid session
  - `full.jsonl`: Complete session with all message types
  - `multimodel.jsonl`: Session demonstrating model switching

### Testing Approach

The codebase includes test files for most packages with:
- Unit tests for core logic (filtering, sorting, parsing)
- Mock implementations for dependencies (Runner, Scanner, etc.)
- Table-driven tests for comprehensive coverage
- Integration tests for service operations
- Component tests using bubbletea's teatest framework
- E2E tests with real tmux sessions (opt-in)

---

## Performance Optimizations

1. **Caching**: Avoid re-parsing unchanged JSONL files
2. **Lazy Last-Line Reading**: Use ParseLastEntry() for status checks
3. **Process Memory Matching**: Cache PID-to-SessionID mappings
4. **Parallel PID Scanning**: Worker pool with 8 goroutines for concurrent memory scanning (~3x speedup)
5. **PID Cache**: Persist PID -> session mappings to avoid rescanning known PIDs across refreshes
6. **fsnotify Watcher**: Instant detection of new session files (e.g., after /clear) without polling
7. **Batch Status Updates**: UpdateAllSessionStatuses() refreshes matcher once
8. **Cursor Position Preservation**: Survives session list refresh
9. **Buffer Management**: Custom buffer sizes for large JSONL files
10. **CWD-Based Path Filtering**: Pre-filter session paths by process CWD before scanning (~12x reduction)
11. **Combined Alternation Regex**: Single regex pattern instead of per-path loop (~22x speedup total)

---

This architecture provides a maintainable, extensible foundation for a powerful Claude Code session manager. The separation of concerns allows independent evolution of configuration, session management, tmux integration, and UI components.
