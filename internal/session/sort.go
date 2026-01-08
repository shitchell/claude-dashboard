package session

import (
	"sort"
	"strings"
)

// SortField represents the field to sort sessions by.
type SortField int

const (
	// SortByModTime sorts by modification time.
	SortByModTime SortField = iota

	// SortByProjectName sorts by project name (alphabetically).
	SortByProjectName

	// SortByStatus sorts by status (active > idle > exited).
	SortByStatus

	// SortByMessageCount sorts by number of messages.
	SortByMessageCount

	// SortByTurnCount sorts by number of conversation turns.
	SortByTurnCount

	// SortBySummary sorts by summary text (alphabetically).
	SortBySummary
)

// SortConfig specifies how to sort sessions.
type SortConfig struct {
	// Field is the field to sort by.
	Field SortField

	// Ascending determines sort direction.
	// When false (default), sorts in descending order.
	Ascending bool
}

// ApplySorting sorts a slice of sessions in place based on the given configuration.
// The slice is modified directly and also returned for convenience.
func ApplySorting(sessions []*Session, cfg SortConfig) []*Session {
	if len(sessions) == 0 {
		return sessions
	}

	// Select the comparison function based on field
	var less func(i, j int) bool

	switch cfg.Field {
	case SortByModTime:
		less = func(i, j int) bool {
			return sessions[i].ModTime.Before(sessions[j].ModTime)
		}

	case SortByProjectName:
		less = func(i, j int) bool {
			return strings.ToLower(sessions[i].ProjectName) < strings.ToLower(sessions[j].ProjectName)
		}

	case SortByStatus:
		// Status priority: active (2) > idle (1) > exited (0)
		// Sort by status value descending (higher is "less" in ascending order)
		less = func(i, j int) bool {
			return sessions[i].Status < sessions[j].Status
		}

	case SortByMessageCount:
		less = func(i, j int) bool {
			return sessions[i].MessageCount < sessions[j].MessageCount
		}

	case SortByTurnCount:
		less = func(i, j int) bool {
			return sessions[i].TurnCount < sessions[j].TurnCount
		}

	case SortBySummary:
		less = func(i, j int) bool {
			return strings.ToLower(sessions[i].Summary) < strings.ToLower(sessions[j].Summary)
		}

	default:
		// Default to modification time if unknown field
		less = func(i, j int) bool {
			return sessions[i].ModTime.Before(sessions[j].ModTime)
		}
	}

	// If descending, invert the comparison
	if !cfg.Ascending {
		originalLess := less
		less = func(i, j int) bool {
			return originalLess(j, i)
		}
	}

	sort.Slice(sessions, less)

	return sessions
}

// SortByModTimeDesc sorts sessions by modification time (newest first).
// This is a convenience function for the most common sort operation.
func SortByModTimeDesc(sessions []*Session) []*Session {
	return ApplySorting(sessions, SortConfig{
		Field:     SortByModTime,
		Ascending: false,
	})
}

// SortByModTimeAsc sorts sessions by modification time (oldest first).
func SortByModTimeAsc(sessions []*Session) []*Session {
	return ApplySorting(sessions, SortConfig{
		Field:     SortByModTime,
		Ascending: true,
	})
}

// SortByProjectAsc sorts sessions by project name (alphabetically, A-Z).
func SortByProjectAsc(sessions []*Session) []*Session {
	return ApplySorting(sessions, SortConfig{
		Field:     SortByProjectName,
		Ascending: true,
	})
}

// SortByStatusDesc sorts sessions by status (active first, then idle, then exited).
func SortByStatusDesc(sessions []*Session) []*Session {
	return ApplySorting(sessions, SortConfig{
		Field:     SortByStatus,
		Ascending: false,
	})
}
