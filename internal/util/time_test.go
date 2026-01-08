package util

import (
	"testing"
	"time"
)

func TestRelativeTimeFrom(t *testing.T) {
	// Use a fixed reference time for all tests
	reference := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		time     time.Time
		expected string
	}{
		// "just now" - less than 60 seconds ago
		{
			name:     "0 seconds ago",
			time:     reference,
			expected: "just now",
		},
		{
			name:     "30 seconds ago",
			time:     reference.Add(-30 * time.Second),
			expected: "just now",
		},
		{
			name:     "59 seconds ago",
			time:     reference.Add(-59 * time.Second),
			expected: "just now",
		},

		// Minutes - 60 seconds to 59 minutes
		{
			name:     "1 minute ago",
			time:     reference.Add(-60 * time.Second),
			expected: "1m ago",
		},
		{
			name:     "5 minutes ago",
			time:     reference.Add(-5 * time.Minute),
			expected: "5m ago",
		},
		{
			name:     "59 minutes ago",
			time:     reference.Add(-59 * time.Minute),
			expected: "59m ago",
		},

		// Hours - 60 minutes to 23 hours
		{
			name:     "1 hour ago",
			time:     reference.Add(-60 * time.Minute),
			expected: "1h ago",
		},
		{
			name:     "2 hours ago",
			time:     reference.Add(-2 * time.Hour),
			expected: "2h ago",
		},
		{
			name:     "23 hours ago",
			time:     reference.Add(-23 * time.Hour),
			expected: "23h ago",
		},

		// Days - 24 hours to 6 days
		{
			name:     "1 day ago",
			time:     reference.Add(-24 * time.Hour),
			expected: "1d ago",
		},
		{
			name:     "3 days ago",
			time:     reference.Add(-3 * 24 * time.Hour),
			expected: "3d ago",
		},
		{
			name:     "6 days ago",
			time:     reference.Add(-6 * 24 * time.Hour),
			expected: "6d ago",
		},

		// Weeks - 7 days to 29 days
		{
			name:     "1 week ago",
			time:     reference.Add(-7 * 24 * time.Hour),
			expected: "1w ago",
		},
		{
			name:     "2 weeks ago",
			time:     reference.Add(-14 * 24 * time.Hour),
			expected: "2w ago",
		},
		{
			name:     "4 weeks ago",
			time:     reference.Add(-28 * 24 * time.Hour),
			expected: "4w ago",
		},

		// Months - 30 days to 364 days
		{
			name:     "1 month ago",
			time:     reference.Add(-30 * 24 * time.Hour),
			expected: "1mo ago",
		},
		{
			name:     "3 months ago",
			time:     reference.Add(-90 * 24 * time.Hour),
			expected: "3mo ago",
		},
		{
			name:     "11 months ago",
			time:     reference.Add(-330 * 24 * time.Hour),
			expected: "11mo ago",
		},

		// Years - 365+ days
		{
			name:     "1 year ago",
			time:     reference.Add(-365 * 24 * time.Hour),
			expected: "1y ago",
		},
		{
			name:     "2 years ago",
			time:     reference.Add(-730 * 24 * time.Hour),
			expected: "2y ago",
		},
		{
			name:     "5 years ago",
			time:     reference.Add(-5 * 365 * 24 * time.Hour),
			expected: "5y ago",
		},

		// Future time
		{
			name:     "1 hour in the future",
			time:     reference.Add(1 * time.Hour),
			expected: "in the future",
		},
		{
			name:     "1 second in the future",
			time:     reference.Add(1 * time.Second),
			expected: "in the future",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RelativeTimeFrom(tt.time, reference)
			if result != tt.expected {
				t.Errorf("RelativeTimeFrom(%v, %v) = %q, want %q",
					tt.time, reference, result, tt.expected)
			}
		})
	}
}

func TestRelativeTimeFromEdgeCases(t *testing.T) {
	reference := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	// Test boundary between units
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		// Boundary: 59s -> 60s (just now -> 1m)
		{
			name:     "59 seconds is still just now",
			duration: -59 * time.Second,
			expected: "just now",
		},
		{
			name:     "60 seconds becomes 1 minute",
			duration: -60 * time.Second,
			expected: "1m ago",
		},

		// Boundary: 59m -> 60m (Xm -> 1h)
		{
			name:     "59 minutes 59 seconds is still minutes",
			duration: -59*time.Minute - 59*time.Second,
			expected: "59m ago",
		},
		{
			name:     "60 minutes becomes 1 hour",
			duration: -60 * time.Minute,
			expected: "1h ago",
		},

		// Boundary: 23h -> 24h (Xh -> 1d)
		{
			name:     "23 hours 59 minutes is still hours",
			duration: -23*time.Hour - 59*time.Minute,
			expected: "23h ago",
		},
		{
			name:     "24 hours becomes 1 day",
			duration: -24 * time.Hour,
			expected: "1d ago",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testTime := reference.Add(tt.duration)
			result := RelativeTimeFrom(testTime, reference)
			if result != tt.expected {
				t.Errorf("RelativeTimeFrom with duration %v = %q, want %q",
					tt.duration, result, tt.expected)
			}
		})
	}
}

func TestRelativeTimeUsesNow(t *testing.T) {
	// This test verifies that RelativeTime() uses the current time
	// by checking that a time in the past returns a valid relative string

	past := time.Now().Add(-5 * time.Minute)
	result := RelativeTime(past)

	// Should return "5m ago" (might be 4m or 6m depending on timing)
	if result != "4m ago" && result != "5m ago" && result != "6m ago" {
		t.Errorf("RelativeTime(%v) = %q, expected ~5m ago", past, result)
	}
}

func TestConstants(t *testing.T) {
	// Verify the time constants are correct
	if SecondsPerMinute != 60 {
		t.Errorf("SecondsPerMinute = %d, want 60", SecondsPerMinute)
	}
	if SecondsPerHour != 3600 {
		t.Errorf("SecondsPerHour = %d, want 3600", SecondsPerHour)
	}
	if SecondsPerDay != 86400 {
		t.Errorf("SecondsPerDay = %d, want 86400", SecondsPerDay)
	}
	if SecondsPerWeek != 604800 {
		t.Errorf("SecondsPerWeek = %d, want 604800", SecondsPerWeek)
	}
	if SecondsPerMonth != 2592000 {
		t.Errorf("SecondsPerMonth = %d, want 2592000", SecondsPerMonth)
	}
	if SecondsPerYear != 31536000 {
		t.Errorf("SecondsPerYear = %d, want 31536000", SecondsPerYear)
	}
}
