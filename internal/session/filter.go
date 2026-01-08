package session

import (
	"strings"
	"time"
)

// FilterConfig specifies filtering criteria for sessions.
// Multiple filters are combined with AND logic (all must match).
type FilterConfig struct {
	// MaxAge filters out sessions older than this duration.
	// Zero value means no age filter.
	MaxAge time.Duration

	// MinAge filters out sessions newer than this duration.
	// Zero value means no minimum age filter.
	MinAge time.Duration

	// Project filters to sessions matching this project path or name.
	// Empty string means no project filter.
	// Matches against both ProjectPath and ProjectName (case-insensitive).
	Project string

	// Status filters to sessions with this specific status.
	// Use StatusExited (0) with ExcludeExited for more nuanced filtering.
	// Note: Since StatusExited is the zero value, use ExcludeExited instead.
	Status *Status

	// ExcludeExited filters out sessions that are not running in tmux.
	ExcludeExited bool

	// SearchText filters to sessions containing this text.
	// Searches Summary, Preview, ProjectName (case-insensitive).
	SearchText string

	// Model filters to sessions using this model.
	// Empty string means no model filter.
	Model string
}

// ApplyFilters filters a slice of sessions based on the given configuration.
// Sessions that match ALL criteria are returned.
// The original slice is not modified; a new slice is returned.
func ApplyFilters(sessions []*Session, cfg FilterConfig) []*Session {
	// If no filters are set, return a copy of the input
	if isEmptyFilter(cfg) {
		result := make([]*Session, len(sessions))
		copy(result, sessions)
		return result
	}

	// Pre-compute the current time for age filters
	now := time.Now()

	// Pre-compute lowercase search strings for case-insensitive matching
	projectLower := strings.ToLower(cfg.Project)
	searchLower := strings.ToLower(cfg.SearchText)
	modelLower := strings.ToLower(cfg.Model)

	result := make([]*Session, 0, len(sessions))

	for _, session := range sessions {
		if matchesFilters(session, cfg, now, projectLower, searchLower, modelLower) {
			result = append(result, session)
		}
	}

	return result
}

// isEmptyFilter returns true if no filters are set.
func isEmptyFilter(cfg FilterConfig) bool {
	return cfg.MaxAge == 0 &&
		cfg.MinAge == 0 &&
		cfg.Project == "" &&
		cfg.Status == nil &&
		!cfg.ExcludeExited &&
		cfg.SearchText == "" &&
		cfg.Model == ""
}

// matchesFilters checks if a session matches all filter criteria.
func matchesFilters(session *Session, cfg FilterConfig, now time.Time, projectLower, searchLower, modelLower string) bool {
	// MaxAge filter: exclude sessions older than MaxAge
	if cfg.MaxAge > 0 {
		age := now.Sub(session.ModTime)
		if age > cfg.MaxAge {
			return false
		}
	}

	// MinAge filter: exclude sessions newer than MinAge
	if cfg.MinAge > 0 {
		age := now.Sub(session.ModTime)
		if age < cfg.MinAge {
			return false
		}
	}

	// Project filter: match against ProjectPath or ProjectName
	if projectLower != "" {
		projectPathLower := strings.ToLower(session.ProjectPath)
		projectNameLower := strings.ToLower(session.ProjectName)
		if !strings.Contains(projectPathLower, projectLower) &&
			!strings.Contains(projectNameLower, projectLower) {
			return false
		}
	}

	// Status filter: exact match
	if cfg.Status != nil {
		if session.Status != *cfg.Status {
			return false
		}
	}

	// ExcludeExited filter
	if cfg.ExcludeExited {
		if session.Status == StatusExited {
			return false
		}
	}

	// SearchText filter: search in Summary, Preview, ProjectName
	if searchLower != "" {
		summaryLower := strings.ToLower(session.Summary)
		previewLower := strings.ToLower(session.Preview)
		projectNameLower := strings.ToLower(session.ProjectName)

		if !strings.Contains(summaryLower, searchLower) &&
			!strings.Contains(previewLower, searchLower) &&
			!strings.Contains(projectNameLower, searchLower) {
			return false
		}
	}

	// Model filter: exact match (case-insensitive)
	if modelLower != "" {
		sessionModelLower := strings.ToLower(session.Model)
		if !strings.Contains(sessionModelLower, modelLower) {
			return false
		}
	}

	return true
}

// FilterByAge is a convenience function that filters sessions by age.
// Returns sessions modified within the given duration.
func FilterByAge(sessions []*Session, maxAge time.Duration) []*Session {
	return ApplyFilters(sessions, FilterConfig{MaxAge: maxAge})
}

// FilterByProject is a convenience function that filters sessions by project.
// Returns sessions matching the given project path or name.
func FilterByProject(sessions []*Session, project string) []*Session {
	return ApplyFilters(sessions, FilterConfig{Project: project})
}

// FilterByStatus is a convenience function that filters sessions by status.
// Returns sessions with the given status.
func FilterByStatus(sessions []*Session, status Status) []*Session {
	return ApplyFilters(sessions, FilterConfig{Status: &status})
}

// FilterActive is a convenience function that returns only active sessions.
func FilterActive(sessions []*Session) []*Session {
	return FilterByStatus(sessions, StatusActive)
}

// FilterRunning is a convenience function that returns sessions that are
// running in tmux (either active or idle).
func FilterRunning(sessions []*Session) []*Session {
	return ApplyFilters(sessions, FilterConfig{ExcludeExited: true})
}
