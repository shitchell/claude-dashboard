# Claude Dashboard Configuration Reference

Configuration file location: `~/.config/claude-dashboard/config.yaml`

## Complete Configuration

```yaml
# Display mode: list or grid
mode: list

# Layout type for session list
layout: list

# Columns to display (in order)
# Available: status, name, preview, modified, project, messages, turns, model
columns:
  - status
  - name
  - preview
  - modified

# Column widths (in characters)
widths:
  name: 20
  preview: 40
  modified: 10
  status: 3
  project: 15
  messages: 8
  turns: 6
  model: 15

# Side panel configuration
side_panel:
  enabled: false
  min_width: 20
  width: 30

# Grid mode configuration
grid:
  card_width: 35
  card_height: 4
  gap: 1

# Status indicators
indicators:
  active: ">"
  idle: "-"
  exited: ""

# Sorting configuration
sort:
  # Sort field: modified, project, status, messages, turns, summary
  field: modified
  # Sort direction: true = ascending, false = descending
  ascending: false

# Filter configuration
filter:
  # Maximum age of sessions to show (e.g., "24h", "7d", "30d")
  max_age: ""
  # Minimum age of sessions to show
  min_age: ""
  # Filter to specific project (partial match on path or name)
  project: ""
  # Hide exited sessions
  exclude_exited: false
  # Search text (matches summary, project name)
  search_text: ""
  # Filter by model name (partial match)
  model: ""

# tmux integration
tmux:
  # Enable/disable tmux integration
  enabled: true
  # Highlight sessions with running processes
  highlight_running: true
  # Navigation method: "select-pane" or "switch-client"
  navigation_method: select-pane

# Cache configuration
cache:
  # Enable/disable metadata caching
  enabled: true
  # Cache directory (relative to XDG_CACHE_HOME)
  dir: ".cache/claude-dashboard"
  # Cache filename
  filename: "sessions.json"

# Auto-refresh configuration
refresh:
  # Enable/disable auto-refresh
  enabled: true
  # Refresh interval in seconds (minimum: 1)
  interval: 5

# Session discovery configuration
sessions:
  # Claude projects directory
  projects_dir: ""  # Defaults to ~/.claude/projects
  # Maximum length for session names in display
  max_name_length: 40
  # Maximum length for preview text
  max_preview_length: 80
```

## Configuration Sections

### Display Mode (`mode`)

Controls the overall display mode of the application.

| Value | Description |
|-------|-------------|
| `list` | Traditional table view with rows |
| `grid` | Card-based grid view |

### Layout (`layout`)

Controls the layout style within the display mode.

| Value | Description |
|-------|-------------|
| `list` | Linear list layout |
| `grid` | Grid/card layout |

### Columns (`columns`)

List of columns to display, in order. Available columns:

| Column | Description |
|--------|-------------|
| `status` | Status indicator (active/idle/exited) |
| `name` | Session name (from summary or filename) |
| `preview` | Preview of last assistant message |
| `modified` | Relative time since last modification |
| `project` | Project name |
| `messages` | Total message count |
| `turns` | Number of conversation turns |
| `model` | Claude model used |

### Column Widths (`widths`)

Configure the width (in characters) for each column. Set to `0` for auto-width.

### Side Panel (`side_panel`)

Configuration for side panel mode (compact view for narrow terminals).

| Field | Type | Description |
|-------|------|-------------|
| `enabled` | bool | Enable side panel mode |
| `min_width` | int | Minimum panel width |
| `width` | int | Fixed panel width (0 for auto) |

### Grid (`grid`)

Configuration for grid layout mode.

| Field | Type | Description |
|-------|------|-------------|
| `card_width` | int | Width of each card |
| `card_height` | int | Height of each card |
| `gap` | int | Gap between cards |

### Indicators (`indicators`)

Customize the status indicators. Can be any characters (including emoji).

| Field | Default | Description |
|-------|---------|-------------|
| `active` | `>` | Active session indicator |
| `idle` | `-` | Idle session indicator |
| `exited` | `` | Exited session indicator |

Examples:
```yaml
# Emoji indicators
indicators:
  active: ">"
  idle: "-"
  exited: "x"

# ASCII indicators
indicators:
  active: "[*]"
  idle: "[ ]"
  exited: "[x]"
```

### Sort (`sort`)

Configure default sorting behavior.

| Field | Type | Description |
|-------|------|-------------|
| `field` | string | Sort field (see below) |
| `ascending` | bool | Sort direction |

Available sort fields:
- `modified` - Last modification time
- `project` - Project name
- `status` - Session status
- `messages` - Message count
- `turns` - Turn count
- `summary` - Session summary text

### Filter (`filter`)

Configure default filtering behavior.

| Field | Type | Description |
|-------|------|-------------|
| `max_age` | string | Maximum session age (e.g., `24h`, `7d`) |
| `min_age` | string | Minimum session age |
| `project` | string | Project name filter (partial match) |
| `exclude_exited` | bool | Hide exited sessions |
| `search_text` | string | Search term |
| `model` | string | Model name filter |

Duration formats:
- `30s` - 30 seconds
- `5m` - 5 minutes
- `2h` - 2 hours
- `7d` - 7 days

### tmux (`tmux`)

Configuration for tmux integration.

| Field | Type | Description |
|-------|------|-------------|
| `enabled` | bool | Enable tmux integration |
| `highlight_running` | bool | Highlight sessions with running processes |
| `navigation_method` | string | How to navigate to panes |

Navigation methods:
- `select-pane` - Use `tmux select-pane` (default)
- `switch-client` - Use `tmux switch-client`

### Cache (`cache`)

Configuration for session metadata caching.

| Field | Type | Description |
|-------|------|-------------|
| `enabled` | bool | Enable caching |
| `dir` | string | Cache directory path |
| `filename` | string | Cache file name |

### Refresh (`refresh`)

Configuration for auto-refresh behavior.

| Field | Type | Description |
|-------|------|-------------|
| `enabled` | bool | Enable auto-refresh |
| `interval` | int | Refresh interval in seconds |

### Sessions (`sessions`)

Configuration for session discovery.

| Field | Type | Description |
|-------|------|-------------|
| `projects_dir` | string | Claude projects directory |
| `max_name_length` | int | Max display length for names |
| `max_preview_length` | int | Max display length for previews |

## Command-Line Override

All configuration options can be overridden via command-line flags:

| Config | Flag |
|--------|------|
| `mode` | `-m, --mode` |
| `layout` | `-l, --layout` |
| `columns` | `--columns` |
| `sort.field` | `--sort` |
| `sort.ascending` | `--asc` |
| `filter.project` | `-p, --project` |
| `filter.max_age` | `--max-age` |
| `filter.exclude_exited` | `--running` |
| `refresh.enabled` | `--no-refresh` |
| `refresh.interval` | `--refresh` |
| `tmux.enabled` | `--no-tmux` |
| `cache.enabled` | `--no-cache` |

## XDG Base Directory Support

The application respects XDG Base Directory specifications:

| Type | Environment Variable | Default |
|------|---------------------|---------|
| Config | `XDG_CONFIG_HOME` | `~/.config` |
| Cache | `XDG_CACHE_HOME` | `~/.cache` |

Configuration file is looked for at:
1. `$XDG_CONFIG_HOME/claude-dashboard/config.yaml`
2. `$XDG_CONFIG_HOME/claude-dashboard/config.yml`

## Example Configurations

### Minimal Configuration

```yaml
# Just change the mode
mode: grid
```

### Compact Side Panel

```yaml
mode: list
columns:
  - status
  - name
  - modified
widths:
  name: 12
  modified: 5
indicators:
  active: "*"
  idle: " "
  exited: "x"
```

### Power User Configuration

```yaml
mode: list
columns:
  - status
  - name
  - project
  - messages
  - turns
  - modified
widths:
  name: 25
  project: 15
  messages: 6
  turns: 6
  modified: 10
sort:
  field: project
  ascending: true
filter:
  max_age: "7d"
  exclude_exited: true
refresh:
  interval: 10
tmux:
  navigation_method: switch-client
```
