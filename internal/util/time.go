// Package util provides utility functions for the claude-dashboard application.
package util

import (
	"fmt"
	"time"
)

// Time thresholds for relative time formatting.
// These define the boundaries between different time units.
const (
	// SecondsPerMinute is the number of seconds in a minute.
	SecondsPerMinute = 60

	// SecondsPerHour is the number of seconds in an hour.
	SecondsPerHour = 60 * SecondsPerMinute

	// SecondsPerDay is the number of seconds in a day.
	SecondsPerDay = 24 * SecondsPerHour

	// SecondsPerWeek is the number of seconds in a week.
	SecondsPerWeek = 7 * SecondsPerDay

	// SecondsPerMonth is an approximation of seconds in a month (30 days).
	SecondsPerMonth = 30 * SecondsPerDay

	// SecondsPerYear is an approximation of seconds in a year (365 days).
	SecondsPerYear = 365 * SecondsPerDay
)

// RelativeTime formats a time.Time as a human-readable relative time string.
// For example: "just now", "5m ago", "2h ago", "3d ago", "2w ago", "1mo ago", "1y ago".
//
// The formatting uses the following thresholds:
//   - < 60 seconds: "just now"
//   - < 60 minutes: "Xm ago"
//   - < 24 hours: "Xh ago"
//   - < 7 days: "Xd ago"
//   - < 30 days: "Xw ago"
//   - < 365 days: "Xmo ago"
//   - >= 365 days: "Xy ago"
//
// If the time is in the future, it returns "in the future".
func RelativeTime(t time.Time) string {
	return RelativeTimeFrom(t, time.Now())
}

// RelativeTimeFrom formats a time.Time relative to a reference time.
// This is useful for testing and for computing relative times against
// a specific point in time rather than the current time.
func RelativeTimeFrom(t time.Time, reference time.Time) string {
	diff := reference.Sub(t)

	// Handle future times
	if diff < 0 {
		return "in the future"
	}

	seconds := int64(diff.Seconds())

	switch {
	case seconds < SecondsPerMinute:
		return "just now"
	case seconds < SecondsPerHour:
		minutes := seconds / SecondsPerMinute
		return fmt.Sprintf("%dm ago", minutes)
	case seconds < SecondsPerDay:
		hours := seconds / SecondsPerHour
		return fmt.Sprintf("%dh ago", hours)
	case seconds < SecondsPerWeek:
		days := seconds / SecondsPerDay
		return fmt.Sprintf("%dd ago", days)
	case seconds < SecondsPerMonth:
		weeks := seconds / SecondsPerWeek
		return fmt.Sprintf("%dw ago", weeks)
	case seconds < SecondsPerYear:
		months := seconds / SecondsPerMonth
		return fmt.Sprintf("%dmo ago", months)
	default:
		years := seconds / SecondsPerYear
		return fmt.Sprintf("%dy ago", years)
	}
}
