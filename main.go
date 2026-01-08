// Package main provides the entry point for claude-dashboard,
// a TUI application for browsing and managing Claude Code sessions.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/shitchell/claude-dashboard/internal/session"
	"github.com/shitchell/claude-dashboard/internal/tmux"
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

	// Checkpoint 2: tmux integration flags
	showTmux := flag.Bool("tmux", false, "Show tmux integration status (Checkpoint 2)")
	showProcesses := flag.Bool("processes", false, "Show detected Claude processes")
	showPanes := flag.Bool("panes", false, "Show all tmux panes")
	gotoPane := flag.String("goto", "", "Navigate to a specific pane ID (e.g., %0)")
	resumeSession := flag.String("resume", "", "Resume a session in a new pane by session ID")

	flag.Parse()

	// Handle Checkpoint 2 verification modes
	if *showTmux || *showProcesses || *showPanes || *gotoPane != "" || *resumeSession != "" {
		return runTmuxVerification(*showTmux, *showProcesses, *showPanes, *gotoPane, *resumeSession)
	}

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

// runTmuxVerification handles Checkpoint 2 tmux integration verification.
func runTmuxVerification(showTmux, showProcesses, showPanes bool, gotoPane, resumeSession string) error {
	fmt.Println("claude-dashboard - Checkpoint 2: tmux Integration Verification")
	fmt.Println()

	// Check if we're running inside tmux
	inTmux := tmux.InTmux()
	fmt.Printf("Running inside tmux: %v\n", inTmux)

	// Create navigator to check availability
	nav := tmux.NewNavigator("")
	navAvailable := nav.IsAvailable()
	fmt.Printf("Navigation available: %v\n", navAvailable)
	fmt.Printf("Claude CLI available: %v\n", tmux.IsClaudeAvailable())
	fmt.Println()

	// Handle navigation request
	if gotoPane != "" {
		fmt.Printf("Attempting to navigate to pane: %s\n", gotoPane)
		if !navAvailable {
			return fmt.Errorf("navigation not available (not in tmux or tmux not found)")
		}
		if err := nav.GoToPane(gotoPane); err != nil {
			return fmt.Errorf("failed to navigate to pane: %w", err)
		}
		fmt.Println("Successfully navigated to pane")
		return nil
	}

	// Handle resume request
	if resumeSession != "" {
		fmt.Printf("Attempting to resume session in new pane: %s\n", resumeSession)
		if !navAvailable {
			return fmt.Errorf("navigation not available (not in tmux or tmux not found)")
		}
		if err := nav.ResumeInNewPane(resumeSession, ""); err != nil {
			return fmt.Errorf("failed to resume session: %w", err)
		}
		fmt.Println("Successfully created new pane with resumed session")
		return nil
	}

	// Create matcher and refresh state
	matcher := tmux.NewMatcher()
	if err := matcher.Refresh(); err != nil {
		// If we're not in tmux, this is expected
		if err == tmux.ErrNotInTmux {
			fmt.Println("Note: Not running in tmux, pane discovery skipped")
			fmt.Println("tmux integration is gracefully disabled when not in tmux")
		} else {
			fmt.Printf("Warning: Failed to refresh matcher state: %v\n", err)
		}
	}

	// Show tmux status
	if showTmux {
		fmt.Println("=== tmux Integration Status ===")
		fmt.Println()

		panes := matcher.AllPanes()
		fmt.Printf("Discovered panes: %d\n", len(panes))

		claudeProcs := matcher.FindClaudeProcesses()
		fmt.Printf("Claude processes: %d\n", len(claudeProcs))
		fmt.Println()
	}

	// Show all panes
	if showPanes {
		fmt.Println("=== All tmux Panes ===")
		fmt.Println()

		panes := matcher.AllPanes()
		if len(panes) == 0 {
			if !inTmux {
				fmt.Println("No panes discovered (not running in tmux)")
			} else {
				fmt.Println("No panes discovered")
			}
		} else {
			for _, pane := range panes {
				fmt.Printf("Pane %s: %s\n", pane.ID, pane.Description())
				fmt.Printf("  TTY: %s\n", pane.TTY)
			}
		}
		fmt.Println()
	}

	// Show detected Claude processes
	if showProcesses {
		fmt.Println("=== Detected Claude Processes ===")
		fmt.Println()

		procs := matcher.FindClaudeProcesses()
		if len(procs) == 0 {
			fmt.Println("No Claude processes detected")
		} else {
			for _, proc := range procs {
				fmt.Printf("PID %d:\n", proc.PID)
				fmt.Printf("  Command: %s\n", proc.Command)
				fmt.Printf("  TTY: %s\n", proc.TTY)
				if proc.SessionID != "" {
					fmt.Printf("  Session ID: %s\n", proc.SessionID)
				}
				if proc.PaneID != "" {
					fmt.Printf("  Pane ID: %s\n", proc.PaneID)
				}
				if proc.CWD != "" {
					fmt.Printf("  CWD: %s\n", proc.CWD)
				}
				fmt.Println()
			}
		}
	}

	// If showing session status, load sessions and update their status
	if showTmux {
		fmt.Println("=== Session Status Integration ===")
		fmt.Println()

		// Load sessions
		svc, err := session.NewService(nil)
		if err != nil {
			return fmt.Errorf("failed to create service: %w", err)
		}

		sessions, err := svc.LoadAll()
		if err != nil {
			return fmt.Errorf("failed to load sessions: %w", err)
		}

		// Create parser for status determination
		parser := session.NewParser(nil)

		// Update all session statuses
		tmux.UpdateAllSessionStatuses(sessions, matcher, parser)

		// Show sessions with their status
		activeCount := 0
		idleCount := 0
		exitedCount := 0

		for _, s := range sessions {
			switch s.Status {
			case session.StatusActive:
				activeCount++
			case session.StatusIdle:
				idleCount++
			case session.StatusExited:
				exitedCount++
			}
		}

		fmt.Printf("Total sessions: %d\n", len(sessions))
		fmt.Printf("  Active: %d\n", activeCount)
		fmt.Printf("  Idle: %d\n", idleCount)
		fmt.Printf("  Exited: %d\n", exitedCount)
		fmt.Println()

		// Show a few sessions with their status
		fmt.Println("Recent sessions with status:")
		limit := 5
		if len(sessions) < limit {
			limit = len(sessions)
		}
		for i := 0; i < limit; i++ {
			s := sessions[i]
			statusStr := s.Status.String()
			paneInfo := ""
			if s.TmuxPane != "" {
				paneInfo = fmt.Sprintf(" [pane %s]", s.TmuxPane)
			}
			fmt.Printf("  %s: %s - %s%s\n", s.ID[:8], statusStr, s.ProjectName, paneInfo)
		}
	}

	return nil
}
