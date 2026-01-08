package session

import (
	"os"
	"path/filepath"
	"time"

	"github.com/shitchell/claude-dashboard/internal/constants"
)

// Service orchestrates session discovery, parsing, and caching.
// It provides a unified interface for loading and refreshing session data.
type Service struct {
	// scanner discovers JSONL session files in the projects directory.
	scanner *Scanner

	// parser extracts metadata from JSONL files.
	parser *Parser

	// cache stores parsed metadata to avoid re-parsing unchanged files.
	cache *Cache

	// projectsDir is the directory containing Claude Code projects.
	projectsDir string

	// cacheDir is the directory where cache files are stored.
	cacheDir string
}

// ServiceConfig contains configuration options for the Service.
type ServiceConfig struct {
	// ProjectsDir is the directory to scan for sessions.
	// If empty, defaults to ~/.claude/projects.
	ProjectsDir string

	// CacheDir is the directory for cache files.
	// If empty, defaults to ~/.cache/claude-dashboard.
	CacheDir string

	// ParserConfig is optional parser configuration.
	// If nil, defaults are used.
	ParserConfig *ParserConfig
}

// NewService creates a new Service with the given configuration.
// If config is nil, all defaults are used.
func NewService(config *ServiceConfig) (*Service, error) {
	if config == nil {
		config = &ServiceConfig{}
	}

	// Resolve projects directory
	projectsDir := config.ProjectsDir
	if projectsDir == "" {
		var err error
		projectsDir, err = DefaultProjectsDir()
		if err != nil {
			return nil, err
		}
	}

	// Resolve cache directory
	cacheDir := config.CacheDir
	if cacheDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		cacheDir = filepath.Join(homeDir, constants.CacheDir)
	}

	// Create the scanner
	scanner, err := NewScanner(projectsDir)
	if err != nil {
		return nil, err
	}

	// Create the parser
	parser := NewParser(config.ParserConfig)

	// Create and load the cache
	cachePath := filepath.Join(cacheDir, constants.CacheFileName)
	cache := NewCache(cachePath)
	if err := cache.Load(); err != nil {
		// Non-fatal: we can continue without cache
		// The cache will be empty and rebuilt
	}

	return &Service{
		scanner:     scanner,
		parser:      parser,
		cache:       cache,
		projectsDir: projectsDir,
		cacheDir:    cacheDir,
	}, nil
}

// LoadAll discovers and parses all sessions, using the cache where valid.
// This is typically called on startup to populate the initial session list.
// Returns a slice of Session pointers sorted by modification time (newest first).
func (s *Service) LoadAll() ([]*Session, error) {
	// Scan for all session files
	scanResults, err := s.scanner.ScanMainSessionsOnly()
	if err != nil {
		return nil, err
	}

	// Track valid paths for cache pruning
	validPaths := make(map[string]struct{}, len(scanResults))
	for _, result := range scanResults {
		validPaths[result.FilePath] = struct{}{}
	}

	// Prune cache of deleted files
	s.cache.Prune(validPaths)

	// Process each scan result
	sessions := make([]*Session, 0, len(scanResults))
	for _, result := range scanResults {
		session, err := s.processResult(result)
		if err != nil {
			// Skip files that can't be parsed
			// This could be due to empty files, malformed JSON, etc.
			continue
		}
		sessions = append(sessions, session)
	}

	// Save cache if there were changes
	if s.cache.IsDirty() {
		// Non-fatal if save fails
		_ = s.cache.Save()
	}

	// Sort by modification time (newest first) by default
	ApplySorting(sessions, SortConfig{
		Field:     SortByModTime,
		Ascending: false,
	})

	return sessions, nil
}

// Refresh updates the session list with any changes since the last load.
// It checks file modification times to detect changed/new/deleted sessions.
// This is more efficient than LoadAll() for periodic updates.
//
// The existing slice is modified in place where possible:
// - New sessions are added
// - Deleted sessions are removed
// - Changed sessions are re-parsed
//
// Returns the updated session slice.
func (s *Service) Refresh(existing []*Session) ([]*Session, error) {
	// Scan for current session files
	scanResults, err := s.scanner.ScanMainSessionsOnly()
	if err != nil {
		return existing, err
	}

	// Build maps for efficient lookup
	// Map scan results by file path
	scanMap := make(map[string]ScanResult, len(scanResults))
	for _, result := range scanResults {
		scanMap[result.FilePath] = result
	}

	// Map existing sessions by file path
	existingMap := make(map[string]*Session, len(existing))
	for _, session := range existing {
		existingMap[session.FilePath] = session
	}

	// Track valid paths for cache pruning
	validPaths := make(map[string]struct{}, len(scanResults))
	for _, result := range scanResults {
		validPaths[result.FilePath] = struct{}{}
	}

	// Prune cache of deleted files
	s.cache.Prune(validPaths)

	// Build updated session list
	sessions := make([]*Session, 0, len(scanResults))

	for _, result := range scanResults {
		existingSession, exists := existingMap[result.FilePath]

		if exists {
			// Check if file has changed by comparing modification times
			existingMtime := existingSession.ModTime.Unix()
			if existingMtime == result.ModTime {
				// File unchanged, reuse existing session
				sessions = append(sessions, existingSession)
				continue
			}
		}

		// New or changed file - process it
		session, err := s.processResult(result)
		if err != nil {
			// Skip files that can't be parsed
			continue
		}
		sessions = append(sessions, session)
	}

	// Save cache if there were changes
	if s.cache.IsDirty() {
		// Non-fatal if save fails
		_ = s.cache.Save()
	}

	// Sort by modification time (newest first) by default
	ApplySorting(sessions, SortConfig{
		Field:     SortByModTime,
		Ascending: false,
	})

	return sessions, nil
}

// processResult converts a ScanResult to a Session, using cache where valid.
// Returns an error if the file cannot be parsed.
func (s *Service) processResult(result ScanResult) (*Session, error) {
	// Convert Unix timestamp to time.Time for cache lookup
	mtime := time.Unix(result.ModTime, 0)

	// Check cache first
	cached := s.cache.Get(result.FilePath, mtime)
	if cached != nil {
		// Cache hit - convert metadata to session
		session := FromMetadata(*cached)
		return &session, nil
	}

	// Cache miss - parse the file
	metadata, err := s.parser.Parse(result.FilePath)
	if err != nil {
		return nil, err
	}

	// Update cache
	s.cache.Set(result.FilePath, mtime, metadata)

	// Convert to session
	session := FromMetadata(*metadata)
	return &session, nil
}

// GetCache returns the underlying cache for testing or advanced use.
func (s *Service) GetCache() *Cache {
	return s.cache
}

// GetScanner returns the underlying scanner for testing or advanced use.
func (s *Service) GetScanner() *Scanner {
	return s.scanner
}

// GetParser returns the underlying parser for testing or advanced use.
func (s *Service) GetParser() *Parser {
	return s.parser
}
