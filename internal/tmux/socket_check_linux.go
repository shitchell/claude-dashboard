//go:build linux

package tmux

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/shitchell/claude-dashboard/internal/logging"
)

// Socket detection constants - may need tuning based on real-world usage.
const (
	// SocketHeartbeatMax is the maximum socket count for background heartbeat.
	// Claude Code opens 1 socket every ~50s for telemetry/keepalive.
	// Counts above this indicate active API streaming.
	SocketHeartbeatMax = 1

	// MtimeStaleThreshold is how old the JSONL mtime must be to consider
	// the session idle when socket count is at heartbeat level.
	// Active responses continuously write to JSONL, so recent mtime + socket = active.
	MtimeStaleThreshold = 3 * time.Second
)

// CountProcessSockets counts open socket file descriptors for a process.
// Returns 0 if the process doesn't exist or /proc is inaccessible.
func CountProcessSockets(pid int) int {
	fdDir := filepath.Join("/proc", strconv.Itoa(pid), "fd")
	entries, err := os.ReadDir(fdDir)
	if err != nil {
		logging.Debug("CountProcessSockets: failed to read %s: %v", fdDir, err)
		return 0
	}

	count := 0
	for _, entry := range entries {
		link, err := os.Readlink(filepath.Join(fdDir, entry.Name()))
		if err == nil && strings.Contains(link, "socket") {
			count++
		}
	}

	logging.Debug("CountProcessSockets: PID %d has %d sockets", pid, count)
	return count
}

// GetFileMtimeAge returns how long ago the file was modified.
// Returns a very large duration if the file doesn't exist or can't be stat'd.
func GetFileMtimeAge(path string) time.Duration {
	info, err := os.Stat(path)
	if err != nil {
		return time.Hour * 24 * 365 // 1 year - effectively "very stale"
	}
	return time.Since(info.ModTime())
}

// IsProcessActive determines if a Claude process is actively streaming
// by checking socket count and JSONL file mtime.
//
// Detection logic:
//   - sockets == 0 → Idle (no network activity)
//   - sockets == 1 && mtime >= 3s → Idle (background heartbeat)
//   - sockets == 1 && mtime < 3s → Active (starting/ending response)
//   - sockets >= 2 → Active (streaming from API)
func IsProcessActive(pid int, jsonlPath string) bool {
	sockets := CountProcessSockets(pid)
	mtimeAge := GetFileMtimeAge(jsonlPath)

	// Idle: no sockets, OR just heartbeat (1 socket + stale file)
	idle := sockets == 0 || (sockets <= SocketHeartbeatMax && mtimeAge >= MtimeStaleThreshold)

	logging.Debug("IsProcessActive: PID %d, sockets=%d, mtimeAge=%v, idle=%v",
		pid, sockets, mtimeAge.Round(time.Millisecond), idle)

	return !idle
}
