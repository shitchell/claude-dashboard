# Claude Dashboard

A terminal dashboard for browsing and managing Claude Code sessions.

## Overview

Claude Dashboard is a TUI (Terminal User Interface) application that provides a unified view of all your Claude Code sessions. It allows you to:

- See all Claude Code sessions across projects in one place
- Monitor session status at a glance (active, idle, or exited)
- Jump directly to any active session via tmux integration
- Filter and sort sessions by various criteria
- Run continuously in a side panel with low overhead

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/shitchell/claude-dashboard.git
cd claude-dashboard

# Build the binary
go build -o claude-dashboard .

# Optionally install to your PATH
go install .
```

### Prerequisites

- Go 1.21 or later
- tmux (optional, for session navigation)

## Usage

```bash
# Start with defaults
claude-dashboard

# Start in grid mode
claude-dashboard -m grid

# Sort by project name
claude-dashboard --sort project

# Filter to a specific project
claude-dashboard -p myproject

# Show sessions from the last 24 hours
claude-dashboard --max-age 24h

# Show only running sessions
claude-dashboard --running
```

### Command-Line Options

| Option | Description |
|--------|-------------|
| `-c, --config PATH` | Path to config file |
| `-m, --mode MODE` | Display mode: `list` or `grid` |
| `-l, --layout LAYOUT` | Layout type: `list` or `grid` |
| `--columns COLS` | Comma-separated column list |
| `--refresh DURATION` | Refresh interval (e.g., `5s`, `1m`) |
| `--no-refresh` | Disable auto-refresh |
| `--no-tmux` | Disable tmux integration |
| `--no-cache` | Disable session caching |
| `--sort FIELD` | Sort by: `modified`, `project`, `status`, `messages`, `turns`, `summary` |
| `--asc` | Sort in ascending order |
| `-p, --project NAME` | Filter to project (matches path or name) |
| `--max-age DURATION` | Filter by age (e.g., `24h`, `7d`) |
| `--running` | Show only running sessions (exclude exited) |
| `-v, --version` | Show version and exit |
| `-h, --help` | Show help |

## Keyboard Navigation

| Key | Action |
|-----|--------|
| `j` / `Down` | Move selection down |
| `k` / `Up` | Move selection up |
| `h` / `Left` | Move left (grid mode) |
| `l` / `Right` | Move right (grid mode) |
| `Enter` | Navigate to selected session (tmux) or show details |
| `r` | Refresh session list |
| `s` | Cycle sort order |
| `f` | Toggle filter |
| `/` | Search sessions |
| `?` | Show help |
| `q` / `Esc` | Quit |

## Session States

| State | Indicator | Meaning |
|-------|-----------|---------|
| Active | Green dot | Claude is currently working (running tools, generating) |
| Idle | Yellow dot | Session is open, waiting for user input |
| Exited | No indicator | Session has no running process |

## tmux Integration

When running inside tmux, Claude Dashboard can navigate directly to session panes:

- **Active/Idle sessions**: Pressing Enter switches to the pane running that Claude process
- **Exited sessions**: Option to resume session in current or new pane

tmux integration is enabled by default and can be disabled with `--no-tmux` or in the config file.

## Configuration

Configuration is loaded from `~/.config/claude-dashboard/config.yaml`. Settings can be overridden via command-line flags.

See [CONFIG.md](CONFIG.md) for the full configuration reference.

### Example Configuration

```yaml
# Display mode: list or grid
mode: list

# Layout type for session list
layout: list

# Columns to display (in order)
columns:
  - status
  - name
  - preview
  - modified

# Column widths
widths:
  name: 20
  preview: 40
  modified: 10

# Status indicators
indicators:
  active: ">"
  idle: "-"
  exited: ""

# Sorting
sort:
  field: modified
  ascending: false

# Auto-refresh
refresh:
  enabled: true
  interval: 5

# tmux integration
tmux:
  enabled: true
  navigation_method: select-pane
```

## Display Modes

### List Mode

Traditional row-based display showing sessions in a table format:

```
 Claude Sessions                                              3 active
----------------------------------------------------------------------------
 > claude-dashboard    Implement bubbletea TUI with...              25s
 - dotfiles            Fix git hook permissions...                   3m
   work-project        Review PR #423 changes                        2d
```

### Grid Mode

Card-based display for a more visual overview:

```
+---------------------------+  +---------------------------+
| > claude-dashboard        |  | - dotfiles                |
| Implement bubbletea...    |  | Fix git hook...           |
|                      25s  |  |                       3m  |
+---------------------------+  +---------------------------+
```

## Architecture

The application follows a clean architecture with separation of concerns:

```
internal/
  config/   - Configuration loading, merging, validation
  session/  - Session discovery, parsing, caching
  tmux/     - tmux integration for pane detection and navigation
  ui/       - Bubbletea TUI components
  util/     - Utility functions (time formatting, path handling)
```

## Building

```bash
# Build
go build -o claude-dashboard .

# Run tests
go test ./...

# Run tests with coverage
go test ./... -cover
```

## License

MIT License - see [LICENSE](LICENSE) for details.
