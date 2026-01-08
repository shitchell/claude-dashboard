// Package main provides the entry point for claude-dashboard,
// a TUI application for browsing and managing Claude Code sessions.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/shitchell/claude-dashboard/internal/session"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// run is the main entry point, separated for testability.
// This will be implemented in Phase 16: CLI & Main.
func run() error {
	// Parse command line flags for checkpoint verification
	sortField := flag.String("sort", "modtime", "Sort field: modtime, name, messages, turns, summary")
	sortAsc := flag.Bool("asc", false, "Sort in ascending order")
	filterProject := flag.String("project", "", "Filter by project name/path")
	filterAge := flag.Duration("age", 0, "Filter by max age (e.g., 24h, 7d)")
	flag.Parse()

	fmt.Println("claude-dashboard - Checkpoint 1: Core Data Pipeline Verification")
	fmt.Println()

	// Create the service
	svc, err := session.NewService(nil)
	if err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}

	// Load all sessions
	startTime := time.Now()
	sessions, err := svc.LoadAll()
	if err != nil {
		return fmt.Errorf("failed to load sessions: %w", err)
	}
	loadDuration := time.Since(startTime)

	fmt.Printf("Loaded %d sessions in %v\n", len(sessions), loadDuration)

	// Apply filtering if specified
	if *filterProject != "" || *filterAge > 0 {
		filterCfg := session.FilterConfig{
			Project: *filterProject,
			MaxAge:  *filterAge,
		}
		sessions = session.ApplyFilters(sessions, filterCfg)
		fmt.Printf("After filtering: %d sessions\n", len(sessions))
	}

	// Apply sorting if specified (non-default)
	if *sortField != "modtime" || *sortAsc {
		sortCfg := session.SortConfig{
			Ascending: *sortAsc,
		}
		switch *sortField {
		case "name":
			sortCfg.Field = session.SortByProjectName
		case "messages":
			sortCfg.Field = session.SortByMessageCount
		case "turns":
			sortCfg.Field = session.SortByTurnCount
		case "summary":
			sortCfg.Field = session.SortBySummary
		default:
			sortCfg.Field = session.SortByModTime
		}
		session.ApplySorting(sessions, sortCfg)
	}

	fmt.Println()
	fmt.Println("Sessions:")
	fmt.Println(string(make([]byte, 80)))

	// Print sessions
	for i, s := range sessions {
		// Truncate summary for display
		summary := s.Summary
		if len(summary) > 50 {
			summary = summary[:47] + "..."
		}

		fmt.Printf("%d. [%s] %s\n", i+1, s.ID[:8], s.ProjectName)
		fmt.Printf("   Summary: %s\n", summary)
		fmt.Printf("   Model: %s | Messages: %d | Turns: %d | Modified: %s\n",
			s.Model, s.MessageCount, s.TurnCount, s.ModTime.Format("2006-01-02 15:04:05"))
		fmt.Printf("   Path: %s\n", s.FilePath)
		fmt.Println()
	}

	return nil
}
