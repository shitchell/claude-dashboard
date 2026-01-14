# Claude Dashboard - Project Instructions

## Iterative Development Mode (Major Version 0)

**IMPORTANT**: Check `./internal/constants/constants.go` for the current version.

When the major version is `0` (e.g., `0.1.0`, `0.2.0`):
- We are in **ITERATIVE DEVELOPMENT MODE**
- **NO backwards compatibility fixes** should be applied
- **NO deprecation warnings** - just remove old code
- Out with the old, in with the new
- Leaving old methodologies creates bloat and cognitive overhead
- We have no clients to support during v0.x development

**If you see deprecated or backwards-compatible code during v0.x:**
- Report it as an issue
- Recommend removal rather than deprecation
- Clean it up immediately if appropriate

## Project Structure

- `internal/tmux/` - Tmux integration, memory scanning, process matching
- `internal/session/` - JSONL session parsing
- `internal/ui/` - Bubbletea TUI components
- `internal/config/` - Configuration management
- `internal/constants/` - Application constants and version
- `docs/` - Architecture and design documentation
- `_work/` - Tickets, identities, and work tracking

## Key Files

- `docs/architecture.md` - System architecture overview
- `_work/tickets/README.md` - Ticket workflow documentation
- `_work/identities/*/README.md` - Role-based workflow guides

## Testing

```bash
go test ./...           # Run all tests
go test -race ./...     # Run with race detector
go vet ./...            # Static analysis
```

## Build

```bash
go build -o claude-dashboard .
```
