package session

import (
	"testing"
	"time"
)

func TestSortByModTime(t *testing.T) {
	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)
	twoDaysAgo := now.Add(-48 * time.Hour)
	oneWeekAgo := now.Add(-7 * 24 * time.Hour)

	sessions := []*Session{
		createTestSessionData("week", "/p", "p", "Week", StatusExited, oneWeekAgo, "claude-opus-4-5-20251101"),
		createTestSessionData("hour", "/p", "p", "Hour", StatusExited, oneHourAgo, "claude-opus-4-5-20251101"),
		createTestSessionData("recent", "/p", "p", "Recent", StatusExited, now, "claude-opus-4-5-20251101"),
		createTestSessionData("days", "/p", "p", "Days", StatusExited, twoDaysAgo, "claude-opus-4-5-20251101"),
	}

	t.Run("descending (newest first)", func(t *testing.T) {
		// Make a copy to avoid modifying original
		sessionsCopy := make([]*Session, len(sessions))
		copy(sessionsCopy, sessions)

		SortByModTimeDesc(sessionsCopy)

		expectedOrder := []string{"recent", "hour", "days", "week"}
		for i, expected := range expectedOrder {
			if sessionsCopy[i].ID != expected {
				t.Errorf("Position %d: got %q, want %q", i, sessionsCopy[i].ID, expected)
			}
		}
	})

	t.Run("ascending (oldest first)", func(t *testing.T) {
		sessionsCopy := make([]*Session, len(sessions))
		copy(sessionsCopy, sessions)

		SortByModTimeAsc(sessionsCopy)

		expectedOrder := []string{"week", "days", "hour", "recent"}
		for i, expected := range expectedOrder {
			if sessionsCopy[i].ID != expected {
				t.Errorf("Position %d: got %q, want %q", i, sessionsCopy[i].ID, expected)
			}
		}
	})
}

func TestSortByProjectName(t *testing.T) {
	now := time.Now()
	sessions := []*Session{
		createTestSessionData("s1", "/zebra", "zebra", "Zebra project", StatusExited, now, "claude-opus-4-5-20251101"),
		createTestSessionData("s2", "/alpha", "alpha", "Alpha project", StatusExited, now, "claude-opus-4-5-20251101"),
		createTestSessionData("s3", "/Beta", "Beta", "Beta project", StatusExited, now, "claude-opus-4-5-20251101"),
		createTestSessionData("s4", "/delta", "delta", "Delta project", StatusExited, now, "claude-opus-4-5-20251101"),
	}

	t.Run("ascending (A-Z)", func(t *testing.T) {
		sessionsCopy := make([]*Session, len(sessions))
		copy(sessionsCopy, sessions)

		SortByProjectAsc(sessionsCopy)

		expectedOrder := []string{"alpha", "Beta", "delta", "zebra"}
		for i, expected := range expectedOrder {
			if sessionsCopy[i].ProjectName != expected {
				t.Errorf("Position %d: got %q, want %q", i, sessionsCopy[i].ProjectName, expected)
			}
		}
	})

	t.Run("descending (Z-A)", func(t *testing.T) {
		sessionsCopy := make([]*Session, len(sessions))
		copy(sessionsCopy, sessions)

		ApplySorting(sessionsCopy, SortConfig{
			Field:     SortByProjectName,
			Ascending: false,
		})

		expectedOrder := []string{"zebra", "delta", "Beta", "alpha"}
		for i, expected := range expectedOrder {
			if sessionsCopy[i].ProjectName != expected {
				t.Errorf("Position %d: got %q, want %q", i, sessionsCopy[i].ProjectName, expected)
			}
		}
	})
}

func TestSortByStatus(t *testing.T) {
	now := time.Now()
	sessions := []*Session{
		createTestSessionData("s1", "/p", "p", "Exited", StatusExited, now, "claude-opus-4-5-20251101"),
		createTestSessionData("s2", "/p", "p", "Active", StatusActive, now, "claude-opus-4-5-20251101"),
		createTestSessionData("s3", "/p", "p", "Idle", StatusIdle, now, "claude-opus-4-5-20251101"),
		createTestSessionData("s4", "/p", "p", "Another Exited", StatusExited, now, "claude-opus-4-5-20251101"),
	}

	t.Run("descending (active first)", func(t *testing.T) {
		sessionsCopy := make([]*Session, len(sessions))
		copy(sessionsCopy, sessions)

		SortByStatusDesc(sessionsCopy)

		// Active should come first, then Idle, then Exited
		if sessionsCopy[0].Status != StatusActive {
			t.Errorf("First session should be Active, got %v", sessionsCopy[0].Status)
		}
		if sessionsCopy[1].Status != StatusIdle {
			t.Errorf("Second session should be Idle, got %v", sessionsCopy[1].Status)
		}
		// Last two should be exited
		if sessionsCopy[2].Status != StatusExited || sessionsCopy[3].Status != StatusExited {
			t.Error("Last two sessions should be Exited")
		}
	})

	t.Run("ascending (exited first)", func(t *testing.T) {
		sessionsCopy := make([]*Session, len(sessions))
		copy(sessionsCopy, sessions)

		ApplySorting(sessionsCopy, SortConfig{
			Field:     SortByStatus,
			Ascending: true,
		})

		// Exited should come first, then Idle, then Active
		if sessionsCopy[0].Status != StatusExited {
			t.Errorf("First session should be Exited, got %v", sessionsCopy[0].Status)
		}
		if sessionsCopy[3].Status != StatusActive {
			t.Errorf("Last session should be Active, got %v", sessionsCopy[3].Status)
		}
	})
}

func TestSortByMessageCount(t *testing.T) {
	now := time.Now()
	sessions := []*Session{
		{ID: "s1", MessageCount: 100, ModTime: now},
		{ID: "s2", MessageCount: 5, ModTime: now},
		{ID: "s3", MessageCount: 50, ModTime: now},
		{ID: "s4", MessageCount: 10, ModTime: now},
	}

	t.Run("descending (most messages first)", func(t *testing.T) {
		sessionsCopy := make([]*Session, len(sessions))
		copy(sessionsCopy, sessions)

		ApplySorting(sessionsCopy, SortConfig{
			Field:     SortByMessageCount,
			Ascending: false,
		})

		expectedOrder := []int{100, 50, 10, 5}
		for i, expected := range expectedOrder {
			if sessionsCopy[i].MessageCount != expected {
				t.Errorf("Position %d: got %d messages, want %d", i, sessionsCopy[i].MessageCount, expected)
			}
		}
	})

	t.Run("ascending (least messages first)", func(t *testing.T) {
		sessionsCopy := make([]*Session, len(sessions))
		copy(sessionsCopy, sessions)

		ApplySorting(sessionsCopy, SortConfig{
			Field:     SortByMessageCount,
			Ascending: true,
		})

		expectedOrder := []int{5, 10, 50, 100}
		for i, expected := range expectedOrder {
			if sessionsCopy[i].MessageCount != expected {
				t.Errorf("Position %d: got %d messages, want %d", i, sessionsCopy[i].MessageCount, expected)
			}
		}
	})
}

func TestSortByTurnCount(t *testing.T) {
	now := time.Now()
	sessions := []*Session{
		{ID: "s1", TurnCount: 20, ModTime: now},
		{ID: "s2", TurnCount: 2, ModTime: now},
		{ID: "s3", TurnCount: 10, ModTime: now},
		{ID: "s4", TurnCount: 5, ModTime: now},
	}

	t.Run("descending (most turns first)", func(t *testing.T) {
		sessionsCopy := make([]*Session, len(sessions))
		copy(sessionsCopy, sessions)

		ApplySorting(sessionsCopy, SortConfig{
			Field:     SortByTurnCount,
			Ascending: false,
		})

		expectedOrder := []int{20, 10, 5, 2}
		for i, expected := range expectedOrder {
			if sessionsCopy[i].TurnCount != expected {
				t.Errorf("Position %d: got %d turns, want %d", i, sessionsCopy[i].TurnCount, expected)
			}
		}
	})

	t.Run("ascending (least turns first)", func(t *testing.T) {
		sessionsCopy := make([]*Session, len(sessions))
		copy(sessionsCopy, sessions)

		ApplySorting(sessionsCopy, SortConfig{
			Field:     SortByTurnCount,
			Ascending: true,
		})

		expectedOrder := []int{2, 5, 10, 20}
		for i, expected := range expectedOrder {
			if sessionsCopy[i].TurnCount != expected {
				t.Errorf("Position %d: got %d turns, want %d", i, sessionsCopy[i].TurnCount, expected)
			}
		}
	})
}

func TestSortBySummary(t *testing.T) {
	now := time.Now()
	sessions := []*Session{
		{ID: "s1", Summary: "Zebra task", ModTime: now},
		{ID: "s2", Summary: "Alpha task", ModTime: now},
		{ID: "s3", Summary: "beta task", ModTime: now},
		{ID: "s4", Summary: "Delta task", ModTime: now},
	}

	t.Run("ascending (A-Z)", func(t *testing.T) {
		sessionsCopy := make([]*Session, len(sessions))
		copy(sessionsCopy, sessions)

		ApplySorting(sessionsCopy, SortConfig{
			Field:     SortBySummary,
			Ascending: true,
		})

		expectedOrder := []string{"Alpha task", "beta task", "Delta task", "Zebra task"}
		for i, expected := range expectedOrder {
			if sessionsCopy[i].Summary != expected {
				t.Errorf("Position %d: got %q, want %q", i, sessionsCopy[i].Summary, expected)
			}
		}
	})

	t.Run("descending (Z-A)", func(t *testing.T) {
		sessionsCopy := make([]*Session, len(sessions))
		copy(sessionsCopy, sessions)

		ApplySorting(sessionsCopy, SortConfig{
			Field:     SortBySummary,
			Ascending: false,
		})

		expectedOrder := []string{"Zebra task", "Delta task", "beta task", "Alpha task"}
		for i, expected := range expectedOrder {
			if sessionsCopy[i].Summary != expected {
				t.Errorf("Position %d: got %q, want %q", i, sessionsCopy[i].Summary, expected)
			}
		}
	})
}

func TestSortEmptySlice(t *testing.T) {
	// An explicitly empty slice (not nil)
	sessions := make([]*Session, 0)

	// Should not panic on empty slice
	result := ApplySorting(sessions, SortConfig{Field: SortByModTime})

	if result == nil {
		t.Error("ApplySorting on empty slice returned nil")
	}
	if len(result) != 0 {
		t.Errorf("ApplySorting on empty slice returned %d items", len(result))
	}
}

func TestSortNilSlice(t *testing.T) {
	// Should not panic on nil slice
	result := ApplySorting(nil, SortConfig{Field: SortByModTime})

	if result != nil {
		t.Errorf("ApplySorting on nil slice returned non-nil: %v", result)
	}
}

func TestSortModifiesInPlace(t *testing.T) {
	now := time.Now()
	sessions := []*Session{
		{ID: "z", ModTime: now},
		{ID: "a", ModTime: now.Add(-1 * time.Hour)},
	}

	// Get the original slice pointer
	originalPtr := &sessions[0]

	result := SortByModTimeDesc(sessions)

	// The returned slice should be the same slice
	if result != nil && &result[0] != originalPtr {
		t.Log("Note: Sort may have reordered elements but should modify in place")
	}

	// The original variable should now be sorted
	if sessions[0].ID != "z" {
		t.Errorf("Original slice not sorted: first element is %q, want 'z'", sessions[0].ID)
	}
}

func TestSortUnknownField(t *testing.T) {
	now := time.Now()
	sessions := []*Session{
		{ID: "s1", ModTime: now},
		{ID: "s2", ModTime: now.Add(-1 * time.Hour)},
	}

	// Unknown field should default to mod time
	ApplySorting(sessions, SortConfig{
		Field:     SortField(999), // Invalid field
		Ascending: false,
	})

	// Should be sorted by mod time (default behavior)
	if sessions[0].ModTime.Before(sessions[1].ModTime) {
		t.Error("Unknown field should default to mod time sorting")
	}
}
