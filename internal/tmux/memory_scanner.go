package tmux

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/shitchell/claude-dashboard/internal/logging"
)

// MaxRegionSize is the maximum size of a memory region to scan (256KB).
// V8 stores session file paths in 256KB anonymous rw-p regions.
// Limiting to this size dramatically improves performance while maintaining reliability.
const MaxRegionSize = 256 * 1024

// UUIDPattern matches UUID-formatted session IDs in filenames.
// This is used to filter session files - we skip agent-*.jsonl files.
var UUIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// MemoryScanner provides methods to scan process memory for patterns.
// It reads from /proc/<pid>/maps and /proc/<pid>/mem.
type MemoryScanner struct {
	// pid is the process ID to scan.
	pid int
}

// MemoryRegion represents a readable memory region from /proc/<pid>/maps.
type MemoryRegion struct {
	Start uint64
	End   uint64
	Perms string
	Path  string
}

// NewMemoryScanner creates a new MemoryScanner for the given process.
func NewMemoryScanner(pid int) *MemoryScanner {
	return &MemoryScanner{pid: pid}
}

// ScanForPatterns scans the process memory for the given patterns.
// Returns a map of pattern string -> occurrence count.
// Patterns should be byte slices (e.g., []byte("session-id")).
//
// This function reads /proc/<pid>/maps to find readable regions,
// then reads those regions from /proc/<pid>/mem and counts pattern occurrences.
//
// Errors are logged but not fatal - the function returns partial results
// if some regions cannot be read (e.g., process exited during scan).
func (ms *MemoryScanner) ScanForPatterns(patterns [][]byte) map[string]int {
	results := make(map[string]int)

	// Initialize results for all patterns
	for _, p := range patterns {
		results[string(p)] = 0
	}

	if len(patterns) == 0 {
		return results
	}

	// Read memory maps
	regions, err := ms.readMemoryMaps()
	if err != nil {
		logging.Debug("Failed to read memory maps for PID %d: %v", ms.pid, err)
		return results
	}

	logging.Debug("PID %d: Found %d readable memory regions", ms.pid, len(regions))

	// Open mem file
	memPath := fmt.Sprintf("/proc/%d/mem", ms.pid)
	memFile, err := os.Open(memPath)
	if err != nil {
		logging.Debug("Failed to open memory file for PID %d: %v", ms.pid, err)
		return results
	}
	defer memFile.Close()

	// Scan each region
	var totalBytes int64
	for _, region := range regions {
		size := region.End - region.Start
		if size > MaxRegionSize {
			continue // Skip regions larger than 256KB
		}

		counts, err := ms.scanRegion(memFile, region, patterns)
		if err != nil {
			// Process may have exited or region may have been unmapped
			logging.Debug("PID %d: Error scanning region %x-%x: %v",
				ms.pid, region.Start, region.End, err)
			continue
		}

		// Accumulate counts
		for pattern, count := range counts {
			results[pattern] += count
		}
		totalBytes += int64(size)
	}

	logging.Debug("PID %d: Scanned %d bytes total", ms.pid, totalBytes)
	return results
}

// ReadAllMemory reads all readable memory regions <= MaxRegionSize into a single buffer.
// This is optimized for the "read once, match all" pattern.
func (ms *MemoryScanner) ReadAllMemory() ([]byte, error) {
	// Read memory maps
	regions, err := ms.readMemoryMaps()
	if err != nil {
		return nil, fmt.Errorf("reading memory maps: %w", err)
	}

	// Open mem file
	memPath := fmt.Sprintf("/proc/%d/mem", ms.pid)
	memFile, err := os.Open(memPath)
	if err != nil {
		return nil, fmt.Errorf("opening memory file: %w", err)
	}
	defer memFile.Close()

	// Pre-calculate total size to allocate buffer once
	var totalSize int64
	for _, region := range regions {
		size := region.End - region.Start
		if size <= MaxRegionSize {
			totalSize += int64(size)
		}
	}

	// Allocate buffer
	buffer := make([]byte, 0, totalSize)
	readBuf := make([]byte, MaxRegionSize)

	// Read each region
	for _, region := range regions {
		size := region.End - region.Start
		if size > MaxRegionSize {
			continue
		}

		// Seek to region start
		_, err := memFile.Seek(int64(region.Start), io.SeekStart)
		if err != nil {
			continue // Skip regions we can't seek to
		}

		// Read region data
		n, err := memFile.Read(readBuf[:size])
		if err != nil && !errors.Is(err, io.EOF) {
			continue // Skip regions we can't read
		}

		buffer = append(buffer, readBuf[:n]...)
	}

	logging.Debug("PID %d: Read %d bytes of memory", ms.pid, len(buffer))
	return buffer, nil
}

// readMemoryMaps parses /proc/<pid>/maps and returns readable regions.
func (ms *MemoryScanner) readMemoryMaps() ([]MemoryRegion, error) {
	mapsPath := fmt.Sprintf("/proc/%d/maps", ms.pid)
	file, err := os.Open(mapsPath)
	if err != nil {
		return nil, fmt.Errorf("opening maps: %w", err)
	}
	defer file.Close()

	var regions []MemoryRegion
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		region, err := parseMapLine(line)
		if err != nil {
			continue // Skip malformed lines
		}

		// Only include readable regions
		if !strings.Contains(region.Perms, "r") {
			continue
		}

		regions = append(regions, region)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning maps: %w", err)
	}

	return regions, nil
}

// parseMapLine parses a single line from /proc/<pid>/maps.
// Format: "start-end perms offset dev inode pathname"
// Example: "00400000-00452000 r-xp 00000000 08:02 173521 /usr/bin/dbus-daemon"
func parseMapLine(line string) (MemoryRegion, error) {
	var region MemoryRegion

	// Split on whitespace
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return region, errors.New("not enough fields")
	}

	// Parse address range (first field: "start-end")
	addrParts := strings.Split(fields[0], "-")
	if len(addrParts) != 2 {
		return region, errors.New("invalid address range")
	}

	start, err := strconv.ParseUint(addrParts[0], 16, 64)
	if err != nil {
		return region, fmt.Errorf("parsing start address: %w", err)
	}

	end, err := strconv.ParseUint(addrParts[1], 16, 64)
	if err != nil {
		return region, fmt.Errorf("parsing end address: %w", err)
	}

	region.Start = start
	region.End = end
	region.Perms = fields[1]

	// Path is optional (6th field if present)
	if len(fields) >= 6 {
		region.Path = fields[5]
	}

	return region, nil
}

// scanRegion scans a single memory region for patterns.
// Returns a map of pattern string -> count for this region.
func (ms *MemoryScanner) scanRegion(memFile *os.File, region MemoryRegion, patterns [][]byte) (map[string]int, error) {
	results := make(map[string]int)

	size := region.End - region.Start
	if size == 0 {
		return results, nil
	}

	// Seek to region start
	_, err := memFile.Seek(int64(region.Start), io.SeekStart)
	if err != nil {
		return results, fmt.Errorf("seeking to %x: %w", region.Start, err)
	}

	// Read region data
	data := make([]byte, size)
	n, err := memFile.Read(data)
	if err != nil && !errors.Is(err, io.EOF) {
		return results, fmt.Errorf("reading region: %w", err)
	}
	data = data[:n] // Trim to actual bytes read

	// Count pattern occurrences
	for _, pattern := range patterns {
		count := bytes.Count(data, pattern)
		if count > 0 {
			results[string(pattern)] = count
		}
	}

	return results, nil
}

// ScanProcessMemory is a convenience function that scans a single process
// for the given patterns. Returns pattern string -> occurrence count.
func ScanProcessMemory(pid int, patterns [][]byte) map[string]int {
	scanner := NewMemoryScanner(pid)
	return scanner.ScanForPatterns(patterns)
}

// FindBestMatch finds the pattern with the highest occurrence count.
// Returns the pattern string and its count, or ("", 0) if no matches.
func FindBestMatch(counts map[string]int) (string, int) {
	var bestPattern string
	var bestCount int

	for pattern, count := range counts {
		if count > bestCount {
			bestPattern = pattern
			bestCount = count
		}
	}

	return bestPattern, bestCount
}

// IsUUIDSessionFile returns true if the session ID matches UUID format.
// This filters out agent-*.jsonl files which don't have UUID session IDs.
func IsUUIDSessionFile(sessionID string) bool {
	return UUIDPattern.MatchString(sessionID)
}

// FilterUUIDSessionPaths filters session paths to only include UUID-patterned files.
// This excludes agent-*.jsonl files which have different session ID formats.
func FilterUUIDSessionPaths(sessionPaths []string) []string {
	var filtered []string
	for _, path := range sessionPaths {
		// Extract session ID from path
		parts := strings.Split(path, "/")
		if len(parts) < 1 {
			continue
		}
		filename := parts[len(parts)-1]
		sessionID := strings.TrimSuffix(filename, ".jsonl")

		if IsUUIDSessionFile(sessionID) {
			filtered = append(filtered, path)
		}
	}
	return filtered
}

// BuildNullPrefixedPatterns creates NULL-prefixed patterns from session file paths.
// The NULL prefix (\x00) discriminates true session ownership from text in chat history.
//
// Pattern format: \x00 + full session file path
// Example: \x00/home/guy/.claude/projects/-home-guy/abc123.jsonl
//
// This works because:
// - True owner: Session path is stored in V8/C data structure with NULL prefix
// - False positive: Paths in chat text have no NULL prefix
func BuildNullPrefixedPatterns(sessionPaths []string) map[string][]byte {
	patterns := make(map[string][]byte)

	for _, path := range sessionPaths {
		// Extract session ID from path
		parts := strings.Split(path, "/")
		if len(parts) < 1 {
			continue
		}
		filename := parts[len(parts)-1]
		sessionID := strings.TrimSuffix(filename, ".jsonl")

		// Skip non-UUID session files (agent-*.jsonl)
		if !IsUUIDSessionFile(sessionID) {
			continue
		}

		// Build NULL-prefixed pattern: \x00 + full path
		pattern := append([]byte{0x00}, []byte(path)...)
		patterns[sessionID] = pattern

		logging.Debug("Built NULL-prefixed pattern for session %s: len=%d", sessionID, len(pattern))
	}

	return patterns
}

// BuildSessionPatterns creates search patterns from session file paths.
// DEPRECATED: Use BuildNullPrefixedPatterns instead for reliable matching.
//
// The pattern format is "<project-dir>/<session-id>" where project-dir
// is the encoded directory name (e.g., "-home-guy" for "/home/guy").
//
// Example:
//
//	filepath: "/home/guy/.claude/projects/-home-guy/abc123.jsonl"
//	pattern:  "-home-guy/abc123"
func BuildSessionPatterns(sessionPaths []string) map[string][]byte {
	patterns := make(map[string][]byte)

	for _, path := range sessionPaths {
		// Extract session ID and project dir from path
		// Path format: /home/user/.claude/projects/<project-dir>/<session-id>.jsonl
		parts := strings.Split(path, "/")
		if len(parts) < 2 {
			continue
		}

		// Get filename and remove .jsonl extension
		filename := parts[len(parts)-1]
		sessionID := strings.TrimSuffix(filename, ".jsonl")

		// Get project directory name (parent of the file)
		projectDir := parts[len(parts)-2]

		// Build pattern: "<project-dir>/<session-id>"
		pattern := projectDir + "/" + sessionID
		patterns[sessionID] = []byte(pattern)

		logging.Debug("Built pattern for session %s: %s", sessionID, pattern)
	}

	return patterns
}

// MatchPIDToSessionNullPrefix scans a process using NULL-prefixed patterns
// and returns the session ID if found. This is the primary matching method.
//
// Returns (sessionID, true) if a match is found, ("", false) otherwise.
func MatchPIDToSessionNullPrefix(pid int, sessionPaths []string) (string, bool) {
	if len(sessionPaths) == 0 {
		return "", false
	}

	// Build NULL-prefixed patterns
	patterns := BuildNullPrefixedPatterns(sessionPaths)
	if len(patterns) == 0 {
		return "", false
	}

	// Read all memory once
	scanner := NewMemoryScanner(pid)
	buffer, err := scanner.ReadAllMemory()
	if err != nil {
		logging.Debug("PID %d: Failed to read memory: %v", pid, err)
		return "", false
	}

	// Search for each pattern in the buffer
	for sessionID, pattern := range patterns {
		if bytes.Contains(buffer, pattern) {
			logging.Debug("PID %d: Found NULL-prefixed match for session %s", pid, sessionID)
			return sessionID, true
		}
	}

	logging.Debug("PID %d: No session match found", pid)
	return "", false
}

// MatchPIDToSession scans a process and determines which session it is running.
// DEPRECATED: Use MatchPIDToSessionNullPrefix for more reliable matching.
// Returns the session ID with the highest occurrence count, or "" if no match.
func MatchPIDToSession(pid int, sessionPatterns map[string][]byte) (string, int) {
	if len(sessionPatterns) == 0 {
		return "", 0
	}

	// Convert patterns map to slice for scanning
	var patterns [][]byte
	patternToSessionID := make(map[string]string)

	for sessionID, pattern := range sessionPatterns {
		patterns = append(patterns, pattern)
		patternToSessionID[string(pattern)] = sessionID
	}

	// Scan process memory
	counts := ScanProcessMemory(pid, patterns)

	// Find best match
	bestPattern, bestCount := FindBestMatch(counts)
	if bestCount == 0 {
		return "", 0
	}

	sessionID := patternToSessionID[bestPattern]
	logging.Debug("PID %d matched to session %s (count: %d)", pid, sessionID, bestCount)

	return sessionID, bestCount
}

// ScanAllPIDsForSessions scans multiple PIDs and returns a map of PID -> sessionID.
// This is the main entry point for the memory scanner.
//
// Algorithm:
//  1. For each PID, read all <=256KB readable memory regions into a buffer
//  2. For each UUID session file path, search for \x00 + path in the buffer
//  3. If found, map PID -> sessionID and break (a PID owns at most one session)
//
// Returns a map where:
// - Key: PID of a Claude process
// - Value: Session ID that process is running (or "" if no match)
func ScanAllPIDsForSessions(pids []int, sessionPaths []string) map[int]string {
	result := make(map[int]string)

	if len(pids) == 0 || len(sessionPaths) == 0 {
		return result
	}

	// Filter to UUID-only session paths
	uuidPaths := FilterUUIDSessionPaths(sessionPaths)
	if len(uuidPaths) == 0 {
		logging.Debug("No UUID session paths to scan")
		return result
	}

	logging.Info("Scanning %d PIDs for %d UUID session paths", len(pids), len(uuidPaths))

	for _, pid := range pids {
		sessionID, found := MatchPIDToSessionNullPrefix(pid, uuidPaths)
		if found {
			result[pid] = sessionID
			logging.Info("PID %d -> session %s", pid, sessionID)
		} else {
			logging.Debug("PID %d -> no session match", pid)
		}
	}

	logging.Info("Memory scan complete: matched %d of %d PIDs", len(result), len(pids))
	return result
}
