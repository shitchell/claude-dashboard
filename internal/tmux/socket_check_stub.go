//go:build !linux

package tmux

import (
	"time"
)

// Socket detection constants - kept in sync with socket_check_linux.go
const (
	SocketHeartbeatMax  = 1
	MtimeStaleThreshold = 3 * time.Second
)

// CountProcessSockets is not implemented on this platform.
// Returns -1 to signal that socket-based detection is unavailable.
func CountProcessSockets(pid int) int {
	return -1
}

// GetFileMtimeAge returns how long ago the file was modified.
// This implementation is platform-independent.
func GetFileMtimeAge(path string) time.Duration {
	// TODO: implement when needed for non-Linux platforms
	return time.Hour * 24 * 365
}

// IsProcessActive is not fully implemented on this platform.
// Returns false (idle) as a safe default - falls back to JSONL-based detection.
func IsProcessActive(pid int, jsonlPath string) bool {
	// On non-Linux platforms, we can't detect sockets via /proc.
	// Return false to indicate "not definitely active" - caller should
	// fall back to JSONL-based heuristics.
	return false
}
