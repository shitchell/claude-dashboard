// Package main provides the entry point for claude-dashboard,
// a TUI application for browsing and managing Claude Code sessions.
package main

import (
	"fmt"
	"os"
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
	fmt.Println("claude-dashboard - Phase 0 scaffold")
	fmt.Println("Full implementation coming in later phases.")
	return nil
}
