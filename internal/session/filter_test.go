package session

import (
	"testing"
	"time"
)

// createTestSessionData creates a Session struct for testing.
func createTestSessionData(id, projectPath, projectName, summary string, status Status, modTime time.Time, model string) *Session {
	return &Session{
		ID:           id,
		FilePath:     "/path/to/" + id + ".jsonl",
		ProjectPath:  projectPath,
		ProjectName:  projectName,
		Summary:      summary,
		Preview:      summary, // Use summary as preview for simplicity
		Model:        model,
		Status:       status,
		ModTime:      modTime,
		MessageCount: 10,
		TurnCount:    3,
	}
}

func TestApplyFiltersEmpty(t *testing.T) {
	now := time.Now()
	sessions := []*Session{
		createTestSessionData("s1", "/home/user/p1", "p1", "Session 1", StatusExited, now, "claude-opus-4-5-20251101"),
		createTestSessionData("s2", "/home/user/p2", "p2", "Session 2", StatusIdle, now, "claude-sonnet-4-20250514"),
	}

	// Empty filter should return all sessions
	result := ApplyFilters(sessions, FilterConfig{})

	if len(result) != 2 {
		t.Errorf("ApplyFilters() returned %d sessions, want 2", len(result))
	}
}

func TestFilterByAge(t *testing.T) {
	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)
	twoDaysAgo := now.Add(-48 * time.Hour)
	oneWeekAgo := now.Add(-7 * 24 * time.Hour)

	sessions := []*Session{
		createTestSessionData("recent", "/home/user/p1", "p1", "Recent", StatusExited, now, "claude-opus-4-5-20251101"),
		createTestSessionData("hour", "/home/user/p1", "p1", "Hour ago", StatusExited, oneHourAgo, "claude-opus-4-5-20251101"),
		createTestSessionData("days", "/home/user/p1", "p1", "Two days ago", StatusExited, twoDaysAgo, "claude-opus-4-5-20251101"),
		createTestSessionData("week", "/home/user/p1", "p1", "Week ago", StatusExited, oneWeekAgo, "claude-opus-4-5-20251101"),
	}

	t.Run("MaxAge 24h", func(t *testing.T) {
		result := FilterByAge(sessions, 24*time.Hour)

		if len(result) != 2 {
			t.Errorf("FilterByAge(24h) returned %d sessions, want 2", len(result))
		}

		// Should include "recent" and "hour"
		found := make(map[string]bool)
		for _, s := range result {
			found[s.ID] = true
		}
		if !found["recent"] || !found["hour"] {
			t.Error("FilterByAge(24h) should include 'recent' and 'hour'")
		}
	})

	t.Run("MaxAge 3 days", func(t *testing.T) {
		result := FilterByAge(sessions, 3*24*time.Hour)

		if len(result) != 3 {
			t.Errorf("FilterByAge(3d) returned %d sessions, want 3", len(result))
		}
	})

	t.Run("MaxAge 1 minute excludes all but recent", func(t *testing.T) {
		result := FilterByAge(sessions, 1*time.Minute)

		if len(result) != 1 {
			t.Errorf("FilterByAge(1m) returned %d sessions, want 1", len(result))
		}
		if result[0].ID != "recent" {
			t.Errorf("FilterByAge(1m) should only include 'recent', got %q", result[0].ID)
		}
	})
}

func TestFilterByMinAge(t *testing.T) {
	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)
	twoDaysAgo := now.Add(-48 * time.Hour)

	sessions := []*Session{
		createTestSessionData("recent", "/home/user/p1", "p1", "Recent", StatusExited, now, "claude-opus-4-5-20251101"),
		createTestSessionData("hour", "/home/user/p1", "p1", "Hour ago", StatusExited, oneHourAgo, "claude-opus-4-5-20251101"),
		createTestSessionData("days", "/home/user/p1", "p1", "Two days ago", StatusExited, twoDaysAgo, "claude-opus-4-5-20251101"),
	}

	result := ApplyFilters(sessions, FilterConfig{MinAge: 30 * time.Minute})

	if len(result) != 2 {
		t.Errorf("MinAge filter returned %d sessions, want 2", len(result))
	}

	// Should include "hour" and "days" (older than 30 minutes)
	found := make(map[string]bool)
	for _, s := range result {
		found[s.ID] = true
	}
	if !found["hour"] || !found["days"] {
		t.Error("MinAge filter should include 'hour' and 'days'")
	}
	if found["recent"] {
		t.Error("MinAge filter should not include 'recent'")
	}
}

func TestFilterByProject(t *testing.T) {
	now := time.Now()
	sessions := []*Session{
		createTestSessionData("s1", "/home/user/project-alpha", "project-alpha", "Alpha session", StatusExited, now, "claude-opus-4-5-20251101"),
		createTestSessionData("s2", "/home/user/project-beta", "project-beta", "Beta session", StatusExited, now, "claude-opus-4-5-20251101"),
		createTestSessionData("s3", "/home/user/another", "another", "Another session", StatusExited, now, "claude-opus-4-5-20251101"),
	}

	t.Run("match by project name", func(t *testing.T) {
		result := FilterByProject(sessions, "alpha")

		if len(result) != 1 {
			t.Errorf("FilterByProject(alpha) returned %d sessions, want 1", len(result))
		}
		if len(result) > 0 && result[0].ProjectName != "project-alpha" {
			t.Errorf("FilterByProject(alpha) returned wrong project: %q", result[0].ProjectName)
		}
	})

	t.Run("match by project path", func(t *testing.T) {
		result := FilterByProject(sessions, "/home/user/project")

		// Should match both "project-alpha" and "project-beta"
		if len(result) != 2 {
			t.Errorf("FilterByProject(/home/user/project) returned %d sessions, want 2", len(result))
		}
	})

	t.Run("case insensitive match", func(t *testing.T) {
		result := FilterByProject(sessions, "BETA")

		if len(result) != 1 {
			t.Errorf("FilterByProject(BETA) returned %d sessions, want 1", len(result))
		}
	})

	t.Run("no match", func(t *testing.T) {
		result := FilterByProject(sessions, "gamma")

		if len(result) != 0 {
			t.Errorf("FilterByProject(gamma) returned %d sessions, want 0", len(result))
		}
	})
}

func TestFilterByStatus(t *testing.T) {
	now := time.Now()
	sessions := []*Session{
		createTestSessionData("s1", "/home/user/p1", "p1", "Exited", StatusExited, now, "claude-opus-4-5-20251101"),
		createTestSessionData("s2", "/home/user/p2", "p2", "Idle", StatusIdle, now, "claude-opus-4-5-20251101"),
		createTestSessionData("s3", "/home/user/p3", "p3", "Active", StatusActive, now, "claude-opus-4-5-20251101"),
		createTestSessionData("s4", "/home/user/p4", "p4", "Another Idle", StatusIdle, now, "claude-opus-4-5-20251101"),
	}

	t.Run("filter active", func(t *testing.T) {
		result := FilterActive(sessions)

		if len(result) != 1 {
			t.Errorf("FilterActive() returned %d sessions, want 1", len(result))
		}
		if len(result) > 0 && result[0].Status != StatusActive {
			t.Errorf("FilterActive() returned non-active session")
		}
	})

	t.Run("filter idle", func(t *testing.T) {
		result := FilterByStatus(sessions, StatusIdle)

		if len(result) != 2 {
			t.Errorf("FilterByStatus(Idle) returned %d sessions, want 2", len(result))
		}
	})

	t.Run("filter running (not exited)", func(t *testing.T) {
		result := FilterRunning(sessions)

		if len(result) != 3 {
			t.Errorf("FilterRunning() returned %d sessions, want 3", len(result))
		}
	})

	t.Run("exclude exited", func(t *testing.T) {
		result := ApplyFilters(sessions, FilterConfig{ExcludeExited: true})

		if len(result) != 3 {
			t.Errorf("ExcludeExited returned %d sessions, want 3", len(result))
		}
		for _, s := range result {
			if s.Status == StatusExited {
				t.Error("ExcludeExited filter returned an exited session")
			}
		}
	})
}

func TestFilterBySearchText(t *testing.T) {
	now := time.Now()
	sessions := []*Session{
		createTestSessionData("s1", "/home/user/p1", "dashboard", "Building a dashboard", StatusExited, now, "claude-opus-4-5-20251101"),
		createTestSessionData("s2", "/home/user/p2", "api-service", "Creating REST API", StatusExited, now, "claude-opus-4-5-20251101"),
		createTestSessionData("s3", "/home/user/p3", "cli-tool", "Command line interface", StatusExited, now, "claude-opus-4-5-20251101"),
	}

	t.Run("match summary", func(t *testing.T) {
		result := ApplyFilters(sessions, FilterConfig{SearchText: "dashboard"})

		if len(result) != 1 {
			t.Errorf("SearchText(dashboard) returned %d sessions, want 1", len(result))
		}
	})

	t.Run("match project name", func(t *testing.T) {
		result := ApplyFilters(sessions, FilterConfig{SearchText: "cli"})

		if len(result) != 1 {
			t.Errorf("SearchText(cli) returned %d sessions, want 1", len(result))
		}
	})

	t.Run("case insensitive", func(t *testing.T) {
		result := ApplyFilters(sessions, FilterConfig{SearchText: "REST"})

		if len(result) != 1 {
			t.Errorf("SearchText(REST) returned %d sessions, want 1", len(result))
		}
	})
}

func TestFilterByModel(t *testing.T) {
	now := time.Now()
	sessions := []*Session{
		createTestSessionData("s1", "/home/user/p1", "p1", "Opus session", StatusExited, now, "claude-opus-4-5-20251101"),
		createTestSessionData("s2", "/home/user/p2", "p2", "Sonnet session", StatusExited, now, "claude-sonnet-4-20250514"),
		createTestSessionData("s3", "/home/user/p3", "p3", "Another Opus", StatusExited, now, "claude-opus-4-5-20251101"),
	}

	t.Run("filter by opus", func(t *testing.T) {
		result := ApplyFilters(sessions, FilterConfig{Model: "opus"})

		if len(result) != 2 {
			t.Errorf("Model(opus) returned %d sessions, want 2", len(result))
		}
	})

	t.Run("filter by sonnet", func(t *testing.T) {
		result := ApplyFilters(sessions, FilterConfig{Model: "sonnet"})

		if len(result) != 1 {
			t.Errorf("Model(sonnet) returned %d sessions, want 1", len(result))
		}
	})
}

func TestCombinedFilters(t *testing.T) {
	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)
	twoDaysAgo := now.Add(-48 * time.Hour)

	sessions := []*Session{
		createTestSessionData("s1", "/home/user/project-a", "project-a", "Recent A", StatusActive, now, "claude-opus-4-5-20251101"),
		createTestSessionData("s2", "/home/user/project-a", "project-a", "Hour ago A", StatusIdle, oneHourAgo, "claude-sonnet-4-20250514"),
		createTestSessionData("s3", "/home/user/project-b", "project-b", "Recent B", StatusExited, now, "claude-opus-4-5-20251101"),
		createTestSessionData("s4", "/home/user/project-b", "project-b", "Old B", StatusExited, twoDaysAgo, "claude-opus-4-5-20251101"),
	}

	t.Run("project and age", func(t *testing.T) {
		result := ApplyFilters(sessions, FilterConfig{
			Project: "project-a",
			MaxAge:  24 * time.Hour,
		})

		// Should match s1 and s2 (project-a and within 24h)
		if len(result) != 2 {
			t.Errorf("Combined filter returned %d sessions, want 2", len(result))
		}
	})

	t.Run("project and status", func(t *testing.T) {
		result := ApplyFilters(sessions, FilterConfig{
			Project:       "project-b",
			ExcludeExited: true,
		})

		// project-b has no running sessions
		if len(result) != 0 {
			t.Errorf("Combined filter returned %d sessions, want 0", len(result))
		}
	})

	t.Run("age and status and model", func(t *testing.T) {
		result := ApplyFilters(sessions, FilterConfig{
			MaxAge:        24 * time.Hour,
			ExcludeExited: true,
			Model:         "opus",
		})

		// s1 is the only one: recent, active, opus
		if len(result) != 1 {
			t.Errorf("Combined filter returned %d sessions, want 1", len(result))
		}
		if len(result) > 0 && result[0].ID != "s1" {
			t.Errorf("Expected session s1, got %q", result[0].ID)
		}
	})
}

func TestFilterDoesNotModifyOriginal(t *testing.T) {
	now := time.Now()
	sessions := []*Session{
		createTestSessionData("s1", "/home/user/p1", "p1", "Session 1", StatusExited, now, "claude-opus-4-5-20251101"),
		createTestSessionData("s2", "/home/user/p2", "p2", "Session 2", StatusActive, now, "claude-opus-4-5-20251101"),
	}

	originalLen := len(sessions)

	// Apply a filter that removes one session
	result := FilterByStatus(sessions, StatusActive)

	// Verify original slice is unchanged
	if len(sessions) != originalLen {
		t.Errorf("Original slice length changed: %d -> %d", originalLen, len(sessions))
	}

	// Verify result has only one session
	if len(result) != 1 {
		t.Errorf("Result length = %d, want 1", len(result))
	}
}
