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
	"sync"
	"unicode/utf8"

	"github.com/shitchell/claude-dashboard/internal/logging"
)

// MaxRegionSize is the maximum size of a memory region to scan (256KB).
// V8 stores session file paths in 256KB anonymous rw-p regions.
// Regions larger than this are skipped.
const MaxRegionSize = 256 * 1024

// NumScanWorkers is the number of goroutines used for parallel PID scanning.
// Benchmarks showed 8 workers achieve ~3x speedup (379ms -> 128ms).
const NumScanWorkers = 8

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
		if size != MaxRegionSize {
			continue // Only scan exactly 256KB regions
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
	var regionCount int
	for _, region := range regions {
		size := region.End - region.Start
		if size == MaxRegionSize {
			totalSize += int64(size)
			regionCount++
		}
	}
	logging.Debug("PID %d: Found %d regions (exactly 256KB) totaling %d bytes", ms.pid, regionCount, totalSize)

	// Allocate buffer
	buffer := make([]byte, 0, totalSize)
	readBuf := make([]byte, MaxRegionSize)

	// Read each region
	for _, region := range regions {
		size := region.End - region.Start
		if size != MaxRegionSize {
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

// ScanUntilMatch reads exactly 256KB regions one at a time, checking for pattern matches.
// Returns (sessionID, true) on first match, avoiding unnecessary memory reads.
// This implements the early exit optimization: once a session pattern is found,
// remaining regions are not read.
func (ms *MemoryScanner) ScanUntilMatch(patterns map[string][]byte) (string, bool) {
	if len(patterns) == 0 {
		return "", false
	}

	// Read memory maps
	regions, err := ms.readMemoryMaps()
	if err != nil {
		logging.Debug("ScanUntilMatch PID %d: Failed to read memory maps: %v", ms.pid, err)
		return "", false
	}

	// Open mem file
	memPath := fmt.Sprintf("/proc/%d/mem", ms.pid)
	memFile, err := os.Open(memPath)
	if err != nil {
		logging.Debug("ScanUntilMatch PID %d: Failed to open memory file: %v", ms.pid, err)
		return "", false
	}
	defer memFile.Close()

	// Pre-allocate a single buffer for reading regions
	readBuf := make([]byte, MaxRegionSize)

	// Count total and filtered regions for logging
	var totalRegions, scannedRegions int
	for _, region := range regions {
		size := region.End - region.Start
		if size == MaxRegionSize {
			totalRegions++
		}
	}

	logging.Debug("ScanUntilMatch PID %d: %d exactly-256KB regions to scan", ms.pid, totalRegions)

	// Scan each exactly-256KB region until a match is found
	for _, region := range regions {
		size := region.End - region.Start
		if size != MaxRegionSize {
			continue // Only scan exactly 256KB regions
		}

		scannedRegions++

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

		regionData := readBuf[:n]

		// Check all patterns against this region
		for sessionID, pattern := range patterns {
			if bytes.Contains(regionData, pattern) {
				logging.Info("ScanUntilMatch PID %d: MATCH! Found session %s in region %d/%d",
					ms.pid, sessionID, scannedRegions, totalRegions)
				return sessionID, true
			}
		}
	}

	logging.Debug("ScanUntilMatch PID %d: No match found after scanning %d regions", ms.pid, scannedRegions)
	return "", false
}

// readMemoryMaps parses /proc/<pid>/maps and returns readable, writable, private anonymous regions.
// V8 stores session file paths in anonymous rw-p regions.
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

		// Only include rw-p (read-write private) regions - this is where V8 stores session paths
		if region.Perms != "rw-p" {
			continue
		}

		// Only include anonymous regions (no path, or [heap], [stack], [anon:*])
		if region.Path != "" && !strings.HasPrefix(region.Path, "[") {
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

// encodePath converts a filesystem path to Claude's .claude/projects/ format.
// Algorithm:
//   - [a-zA-Z0-9-] preserved
//   - All other chars -> max(1, floor(utf8_bytes/2)) dashes
func encodePath(path string) string {
	var result strings.Builder
	for _, r := range path {
		if isASCIIAlphanumOrDash(r) {
			result.WriteRune(r)
		} else {
			byteLen := utf8.RuneLen(r)
			dashes := byteLen / 2
			if dashes < 1 {
				dashes = 1
			}
			result.WriteString(strings.Repeat("-", dashes))
		}
	}
	return result.String()
}

// isASCIIAlphanumOrDash returns true if the rune is an ASCII alphanumeric or dash.
func isASCIIAlphanumOrDash(r rune) bool {
	return (r >= 'a' && r <= 'z') ||
		(r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') ||
		r == '-'
}

// filterSessionPathsByCWD filters session file paths to only those matching
// the encoded CWD. This dramatically reduces the number of patterns to search.
// Returns the original paths if CWD cannot be read (graceful fallback).
func filterSessionPathsByCWD(paths []string, pid int) []string {
	cwdPath := fmt.Sprintf("/proc/%d/cwd", pid)
	cwd, err := os.Readlink(cwdPath)
	if err != nil {
		logging.Debug("filterSessionPathsByCWD: cannot read CWD for PID %d: %v", pid, err)
		return paths // Fallback to all paths
	}

	encodedCWD := encodePath(cwd)

	var filtered []string
	for _, path := range paths {
		if strings.Contains(path, encodedCWD) {
			filtered = append(filtered, path)
		}
	}

	logging.Debug("filterSessionPathsByCWD: PID %d CWD=%s encoded=%s filtered %d->%d paths",
		pid, cwd, encodedCWD, len(paths), len(filtered))

	if len(filtered) == 0 {
		// If filtering produced no results, fall back to all paths
		// (the CWD may not have any sessions, or encoding mismatch)
		return paths
	}

	return filtered
}

// buildCombinedPattern creates a single regex that matches any of the given paths
// with a NULL prefix. This is much faster than looping with bytes.Contains().
// Pattern format: \x00(path1|path2|...)
func buildCombinedPattern(paths []string) (*regexp.Regexp, error) {
	if len(paths) == 0 {
		return nil, nil
	}

	var escaped []string
	for _, path := range paths {
		escaped = append(escaped, regexp.QuoteMeta(path))
	}

	pattern := `\x00(` + strings.Join(escaped, "|") + `)`
	return regexp.Compile(pattern)
}

// ScanWithCombinedPattern scans memory using CWD filtering and combined regex.
// This is the optimized version achieving ~22x speedup.
// Returns (sessionID, true) on first match, ("", false) if no match.
func (ms *MemoryScanner) ScanWithCombinedPattern(sessionPaths []string) (string, bool) {
	if len(sessionPaths) == 0 {
		return "", false
	}

	// Step 1: Filter paths by CWD
	filteredPaths := filterSessionPathsByCWD(sessionPaths, ms.pid)
	logging.Debug("ScanWithCombinedPattern PID %d: %d paths after CWD filtering",
		ms.pid, len(filteredPaths))

	// Filter to UUID-only session paths
	uuidPaths := FilterUUIDSessionPaths(filteredPaths)
	if len(uuidPaths) == 0 {
		logging.Debug("ScanWithCombinedPattern PID %d: no UUID paths after filtering", ms.pid)
		return "", false
	}

	// Step 2: Build combined pattern
	combinedRegex, err := buildCombinedPattern(uuidPaths)
	if err != nil {
		logging.Warn("ScanWithCombinedPattern PID %d: failed to build pattern: %v", ms.pid, err)
		return "", false
	}
	if combinedRegex == nil {
		return "", false
	}

	// Step 3: Scan memory regions
	regions, err := ms.readMemoryMaps()
	if err != nil {
		logging.Debug("ScanWithCombinedPattern PID %d: failed to read maps: %v", ms.pid, err)
		return "", false
	}

	memPath := fmt.Sprintf("/proc/%d/mem", ms.pid)
	memFile, err := os.Open(memPath)
	if err != nil {
		logging.Debug("ScanWithCombinedPattern PID %d: failed to open mem: %v", ms.pid, err)
		return "", false
	}
	defer memFile.Close()

	readBuf := make([]byte, MaxRegionSize)
	var scannedRegions int

	for _, region := range regions {
		size := region.End - region.Start
		if size != MaxRegionSize {
			continue
		}
		scannedRegions++

		_, err := memFile.Seek(int64(region.Start), io.SeekStart)
		if err != nil {
			continue
		}

		n, err := memFile.Read(readBuf[:size])
		if err != nil && !errors.Is(err, io.EOF) {
			continue
		}

		// Use FindSubmatch to get the matched path
		match := combinedRegex.FindSubmatch(readBuf[:n])
		if len(match) > 1 {
			matchedPath := string(match[1])
			sessionID := extractSessionIDFromPath(matchedPath)
			logging.Info("ScanWithCombinedPattern PID %d: MATCH! Session %s in region %d",
				ms.pid, sessionID, scannedRegions)
			return sessionID, true
		}
	}

	logging.Debug("ScanWithCombinedPattern PID %d: no match after %d regions", ms.pid, scannedRegions)
	return "", false
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

// MatchPIDToSessionNullPrefix scans a process using the optimized combined pattern approach.
// Uses CWD filtering and combined alternation regex for ~22x speedup.
//
// Returns (sessionID, true) if a match is found, ("", false) otherwise.
func MatchPIDToSessionNullPrefix(pid int, sessionPaths []string) (string, bool) {
	if len(sessionPaths) == 0 {
		logging.Debug("PID %d: No session paths provided", pid)
		return "", false
	}

	scanner := NewMemoryScanner(pid)
	return scanner.ScanWithCombinedPattern(sessionPaths)
}

// scanResult holds the result of scanning a single PID for session ownership.
// Used by the worker pool in ScanAllPIDsForSessions.
type scanResult struct {
	pid       int
	sessionID string
	found     bool
}

// ScanAllPIDsForSessions scans multiple PIDs and returns a map of PID -> sessionID.
// This is the main entry point for the memory scanner.
//
// Algorithm:
//  1. For each PID, read all <=256KB readable memory regions into a buffer
//  2. For each UUID session file path, search for \x00 + path in the buffer
//  3. If found, map PID -> sessionID and break (a PID owns at most one session)
//
// Uses a worker pool with NumScanWorkers (8) goroutines for parallel scanning,
// achieving approximately 3x speedup compared to sequential scanning.
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

	logging.Info("Scanning %d PIDs for %d UUID session paths using %d workers",
		len(pids), len(uuidPaths), NumScanWorkers)

	// Create channels for worker pool
	jobs := make(chan int, len(pids))
	results := make(chan scanResult, len(pids))

	// Start worker goroutines
	var wg sync.WaitGroup
	for w := 0; w < NumScanWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for pid := range jobs {
				sessionID, found := MatchPIDToSessionNullPrefix(pid, uuidPaths)
				results <- scanResult{pid: pid, sessionID: sessionID, found: found}
			}
		}()
	}

	// Send all PIDs to the jobs channel
	for _, pid := range pids {
		jobs <- pid
	}
	close(jobs)

	// Wait for all workers to complete, then close results channel
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results from all workers
	for r := range results {
		if r.found {
			result[r.pid] = r.sessionID
			logging.Info("PID %d -> session %s", r.pid, r.sessionID)
		} else {
			logging.Debug("PID %d -> no session match", r.pid)
		}
	}

	logging.Info("Memory scan complete: matched %d of %d PIDs", len(result), len(pids))
	return result
}
