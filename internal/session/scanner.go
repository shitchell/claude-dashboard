package session

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/shitchell/claude-dashboard/internal/constants"
)

// DefaultProjectsDir returns the default path to Claude Code's projects directory.
// This expands ~ to the user's home directory.
func DefaultProjectsDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, constants.ClaudeProjectsDir), nil
}

// Scanner finds JSONL session files in Claude Code's projects directory.
type Scanner struct {
	// ProjectsDir is the directory to scan for sessions.
	// Defaults to ~/.claude/projects if not set.
	ProjectsDir string
}

// NewScanner creates a new Scanner with the given projects directory.
// If projectsDir is empty, it uses the default ~/.claude/projects.
func NewScanner(projectsDir string) (*Scanner, error) {
	if projectsDir == "" {
		var err error
		projectsDir, err = DefaultProjectsDir()
		if err != nil {
			return nil, err
		}
	}
	return &Scanner{ProjectsDir: projectsDir}, nil
}

// ScanResult contains information about a discovered JSONL file.
type ScanResult struct {
	// FilePath is the absolute path to the JSONL file.
	FilePath string

	// ProjectDir is the name of the parent project directory (encoded).
	ProjectDir string

	// SessionID is the UUID of the session (filename without extension).
	SessionID string

	// IsSubAgent is true if this is a sub-agent session (agent-*.jsonl).
	IsSubAgent bool

	// AgentID is the agent identifier for sub-agent sessions.
	AgentID string

	// ModTime is the file modification time.
	ModTime int64
}

// Scan discovers all JSONL session files in the projects directory.
// It returns a slice of ScanResult structs, one for each discovered file.
// Sub-agent session files (agent-*.jsonl) are included but flagged as such.
func (s *Scanner) Scan() ([]ScanResult, error) {
	var results []ScanResult

	// Check if the projects directory exists
	info, err := os.Stat(s.ProjectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			// Directory doesn't exist, return empty results
			return results, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		// Not a directory, return empty results
		return results, nil
	}

	// Read the projects directory
	projectDirs, err := os.ReadDir(s.ProjectsDir)
	if err != nil {
		return nil, err
	}

	// Iterate through each project directory
	for _, projectDir := range projectDirs {
		if !projectDir.IsDir() {
			continue
		}

		projectPath := filepath.Join(s.ProjectsDir, projectDir.Name())

		// Read the project directory for JSONL files
		files, err := os.ReadDir(projectPath)
		if err != nil {
			// Skip directories we can't read
			continue
		}

		for _, file := range files {
			if file.IsDir() {
				continue
			}

			fileName := file.Name()
			if !strings.HasSuffix(fileName, ".jsonl") {
				continue
			}

			filePath := filepath.Join(projectPath, fileName)

			// Get file info for modification time
			fileInfo, err := file.Info()
			if err != nil {
				continue
			}

			result := ScanResult{
				FilePath:   filePath,
				ProjectDir: projectDir.Name(),
				ModTime:    fileInfo.ModTime().Unix(),
			}

			// Check if this is a sub-agent file
			if strings.HasPrefix(fileName, "agent-") {
				result.IsSubAgent = true
				// Extract agent ID from filename (agent-<id>.jsonl)
				result.AgentID = strings.TrimSuffix(strings.TrimPrefix(fileName, "agent-"), ".jsonl")
				result.SessionID = result.AgentID
			} else {
				// Regular session file: <uuid>.jsonl
				result.SessionID = strings.TrimSuffix(fileName, ".jsonl")
			}

			results = append(results, result)
		}
	}

	return results, nil
}

// ScanMainSessionsOnly is like Scan but excludes sub-agent session files.
func (s *Scanner) ScanMainSessionsOnly() ([]ScanResult, error) {
	allResults, err := s.Scan()
	if err != nil {
		return nil, err
	}

	var mainSessions []ScanResult
	for _, result := range allResults {
		if !result.IsSubAgent {
			mainSessions = append(mainSessions, result)
		}
	}

	return mainSessions, nil
}
