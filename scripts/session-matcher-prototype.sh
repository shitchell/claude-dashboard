#!/bin/bash
#
# Session Matcher Prototype
# Maps Claude PIDs to sessions via NULL-prefixed memory scanning
#

set -euo pipefail

CLAUDE_PROJECTS_DIR="$HOME/.claude/projects"

# ------------------------------------------------------------------------------
# Phase 1: Discover Claude PIDs
# ------------------------------------------------------------------------------
discover_claude_pids() {
    # Match processes where command is exactly "claude" (not claude-dashboard, etc)
    ps -eo pid,comm | awk '$2 == "claude" {print $1}'
}

# ------------------------------------------------------------------------------
# Phase 2: Discover session files (UUID pattern only, skip agent files)
# ------------------------------------------------------------------------------
discover_session_files() {
    find "$CLAUDE_PROJECTS_DIR" -type f -name '*.jsonl' 2>/dev/null | grep -E '/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\.jsonl$'
}

# ------------------------------------------------------------------------------
# Phase 3: Read process memory once, return it
# ------------------------------------------------------------------------------
read_process_memory() {
    local pid="$1"
    local tmpfile="$2"

    # Read all readable memory regions into tmpfile
    grep -E '^[0-9a-f]+-[0-9a-f]+ r' /proc/"$pid"/maps 2>/dev/null | while read -r line; do
        local start end size
        start=$(echo "$line" | cut -d- -f1)
        end=$(echo "$line" | cut -d- -f2 | cut -d' ' -f1)
        size=$((16#$end - 16#$start))

        # Skip regions > 256KB
        [ "$size" -gt 262144 ] && continue

        dd if=/proc/"$pid"/mem bs=4096 skip=$((16#$start/4096)) count=$(((size+4095)/4096)) 2>/dev/null || true
    done > "$tmpfile"
}

# ------------------------------------------------------------------------------
# Phase 4: Check if buffer contains NULL-prefixed path
# ------------------------------------------------------------------------------
buffer_contains_session() {
    local buffer_file="$1"
    local session_path="$2"
    local pattern_file

    pattern_file=$(mktemp)
    printf '\x00%s' "$session_path" > "$pattern_file"

    if grep -qaF -f "$pattern_file" "$buffer_file" 2>/dev/null; then
        rm -f "$pattern_file"
        return 0
    else
        rm -f "$pattern_file"
        return 1
    fi
}

# ------------------------------------------------------------------------------
# Phase 5: Extract session ID from path
# ------------------------------------------------------------------------------
extract_session_id() {
    local path="$1"
    basename "$path" .jsonl
}

# ------------------------------------------------------------------------------
# Main: Match PIDs to Sessions
# ------------------------------------------------------------------------------
main() {
    local -A pid_to_session=()
    local -A matched_sessions=()

    # Discover
    echo "=== Phase 1: Discovering Claude PIDs ===" >&2
    mapfile -t pids < <(discover_claude_pids)
    echo "Found ${#pids[@]} Claude processes: ${pids[*]}" >&2

    echo -e "\n=== Phase 2: Discovering session files ===" >&2
    mapfile -t session_files < <(discover_session_files)
    echo "Found ${#session_files[@]} session files" >&2

    echo -e "\n=== Phase 3: Memory scanning (read once per PID, match all) ===" >&2

    for pid in "${pids[@]}"; do
        [ -z "$pid" ] && continue
        [ ! -d "/proc/$pid" ] && continue

        echo "  Scanning PID $pid..." >&2

        # Read memory ONCE
        local mem_buffer
        mem_buffer=$(mktemp)
        read_process_memory "$pid" "$mem_buffer"

        local buffer_size
        buffer_size=$(stat -c%s "$mem_buffer" 2>/dev/null || echo 0)
        echo "    Read ${buffer_size} bytes" >&2

        # Check ALL session paths in-memory (cheap)
        for session_path in "${session_files[@]}"; do
            if buffer_contains_session "$mem_buffer" "$session_path"; then
                local session_id
                session_id=$(extract_session_id "$session_path")
                pid_to_session[$pid]="$session_id"
                matched_sessions[$session_id]=1
                echo "    MATCH: $session_id" >&2
                break  # PID owns one session, done
            fi
        done

        rm -f "$mem_buffer"

        if [ -z "${pid_to_session[$pid]:-}" ]; then
            echo "    No session found" >&2
        fi
    done

    # Output results
    echo -e "\n=== Results ===" >&2
    echo ""
    printf "%-8s %-40s %s\n" "STATUS" "SESSION_ID" "PID"
    printf "%-8s %-40s %s\n" "------" "----------" "---"

    # Active sessions (connected to PID)
    for pid in "${!pid_to_session[@]}"; do
        local session_id="${pid_to_session[$pid]}"
        printf "%-8s %-40s %s\n" "active" "$session_id" "$pid"
    done

    # Exited sessions (not connected to any PID)
    for session_path in "${session_files[@]}"; do
        local session_id
        session_id=$(extract_session_id "$session_path")
        if [ -z "${matched_sessions[$session_id]:-}" ]; then
            printf "%-8s %-40s %s\n" "exited" "$session_id" "-"
        fi
    done
}

main "$@"
