package main

import (
	"bytes"
	"io"
	"os"
	"testing"
)

// TestRunVersionFlag verifies that --version prints version and exits successfully.
func TestRunVersionFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "version long flag",
			args: []string{"--version"},
		},
		{
			name: "version short flag",
			args: []string{"-v"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			old := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			exitCode := run(tt.args)

			// Restore stdout and get output
			w.Close()
			os.Stdout = old
			var buf bytes.Buffer
			io.Copy(&buf, r)

			if exitCode != ExitCodeSuccess {
				t.Errorf("run() = %d, want %d", exitCode, ExitCodeSuccess)
			}

			output := buf.String()
			if output == "" {
				t.Error("version flag should print output")
			}
		})
	}
}

// TestRunHelpFlag verifies that --help prints usage and exits successfully.
func TestRunHelpFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "help long flag",
			args: []string{"--help"},
		},
		{
			name: "help short flag",
			args: []string{"-h"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			old := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			exitCode := run(tt.args)

			// Restore stdout and get output
			w.Close()
			os.Stdout = old
			var buf bytes.Buffer
			io.Copy(&buf, r)

			if exitCode != ExitCodeSuccess {
				t.Errorf("run() = %d, want %d", exitCode, ExitCodeSuccess)
			}

			output := buf.String()
			if output == "" {
				t.Error("help flag should print output")
			}
		})
	}
}

// TestRunInvalidFlag verifies that invalid flags return an error exit code.
func TestRunInvalidFlag(t *testing.T) {
	// Capture stderr to suppress error output
	oldStderr := os.Stderr
	_, w, _ := os.Pipe()
	os.Stderr = w

	exitCode := run([]string{"--invalid-flag-that-does-not-exist"})

	w.Close()
	os.Stderr = oldStderr

	if exitCode != ExitCodeConfigError {
		t.Errorf("run() = %d, want %d for invalid flag", exitCode, ExitCodeConfigError)
	}
}

// TestRunConfigLoadError verifies that config file errors return an error exit code.
func TestRunConfigLoadError(t *testing.T) {
	// Capture stderr to suppress error output
	oldStderr := os.Stderr
	_, w, _ := os.Pipe()
	os.Stderr = w

	exitCode := run([]string{"-c", "/nonexistent/path/config.yaml"})

	w.Close()
	os.Stderr = oldStderr

	if exitCode != ExitCodeConfigError {
		t.Errorf("run() = %d, want %d for config error", exitCode, ExitCodeConfigError)
	}
}
