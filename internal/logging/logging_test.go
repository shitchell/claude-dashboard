package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLevelString(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
		{Level(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := tt.level.String()
			if got != tt.expected {
				t.Errorf("Level.String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestNewLogger(t *testing.T) {
	// Create a temp directory for logs
	tempDir := t.TempDir()

	t.Run("creates logger with default config", func(t *testing.T) {
		config := &Config{
			LogDir:  tempDir,
			Enabled: true,
		}

		logger, err := NewLogger(config)
		if err != nil {
			t.Fatalf("NewLogger() error = %v", err)
		}
		defer logger.Close()

		if !logger.enabled {
			t.Error("Expected logger to be enabled")
		}

		// Check that log file was created
		logPath := filepath.Join(tempDir, DefaultLogFileName)
		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			t.Error("Expected log file to be created")
		}
	})

	t.Run("disabled logger does not create file", func(t *testing.T) {
		subDir := filepath.Join(tempDir, "disabled")
		config := &Config{
			LogDir:  subDir,
			Enabled: false,
		}

		logger, err := NewLogger(config)
		if err != nil {
			t.Fatalf("NewLogger() error = %v", err)
		}
		defer logger.Close()

		if logger.enabled {
			t.Error("Expected logger to be disabled")
		}

		// Directory should not exist since logger is disabled
		if _, err := os.Stat(subDir); !os.IsNotExist(err) {
			// Could exist from other tests, but log file should not
			logPath := filepath.Join(subDir, DefaultLogFileName)
			if _, err := os.Stat(logPath); !os.IsNotExist(err) {
				t.Error("Expected no log file when disabled")
			}
		}
	})

	t.Run("nil config uses defaults", func(t *testing.T) {
		// This would try to use the user's home directory
		// We can't easily test this without mocking, so just verify it doesn't panic
		logger, err := NewLogger(nil)
		if err != nil {
			// This is acceptable - might fail due to permissions
			t.Logf("NewLogger(nil) error = %v (acceptable)", err)
			return
		}
		defer logger.Close()
	})
}

func TestLoggerMethods(t *testing.T) {
	tempDir := t.TempDir()
	logFileName := "test.log"

	config := &Config{
		LogDir:      tempDir,
		LogFileName: logFileName,
		Level:       LevelDebug,
		Enabled:     true,
	}

	logger, err := NewLogger(config)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer logger.Close()

	// Write some log messages
	logger.Debug("debug message %d", 1)
	logger.Info("info message %d", 2)
	logger.Warn("warn message %d", 3)
	logger.Error("error message %d", 4)

	// Close to flush
	logger.Close()

	// Read and verify log file
	logPath := filepath.Join(tempDir, logFileName)
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)

	// Check that all messages are present
	if !strings.Contains(logContent, "[DEBUG] debug message 1") {
		t.Error("Expected debug message in log")
	}
	if !strings.Contains(logContent, "[INFO] info message 2") {
		t.Error("Expected info message in log")
	}
	if !strings.Contains(logContent, "[WARN] warn message 3") {
		t.Error("Expected warn message in log")
	}
	if !strings.Contains(logContent, "[ERROR] error message 4") {
		t.Error("Expected error message in log")
	}
}

func TestLoggerLevelFiltering(t *testing.T) {
	tempDir := t.TempDir()
	logFileName := "level-test.log"

	config := &Config{
		LogDir:      tempDir,
		LogFileName: logFileName,
		Level:       LevelWarn, // Only warn and error
		Enabled:     true,
	}

	logger, err := NewLogger(config)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}

	// Write all levels
	logger.Debug("debug")
	logger.Info("info")
	logger.Warn("warn")
	logger.Error("error")

	logger.Close()

	// Read and verify log file
	logPath := filepath.Join(tempDir, logFileName)
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)

	// Debug and Info should be filtered out
	if strings.Contains(logContent, "debug") {
		t.Error("Debug message should be filtered")
	}
	if strings.Contains(logContent, "info") {
		t.Error("Info message should be filtered")
	}

	// Warn and Error should be present
	if !strings.Contains(logContent, "warn") {
		t.Error("Warn message should be present")
	}
	if !strings.Contains(logContent, "error") {
		t.Error("Error message should be present")
	}
}

func TestLoggerSetLevel(t *testing.T) {
	tempDir := t.TempDir()
	logFileName := "set-level-test.log"

	config := &Config{
		LogDir:      tempDir,
		LogFileName: logFileName,
		Level:       LevelError, // Start with error only
		Enabled:     true,
	}

	logger, err := NewLogger(config)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}

	// Write at info level - should be filtered
	logger.Info("info1")

	// Change level
	logger.SetLevel(LevelInfo)

	// Write at info level again - should now work
	logger.Info("info2")

	logger.Close()

	// Read and verify
	logPath := filepath.Join(tempDir, logFileName)
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)

	if strings.Contains(logContent, "info1") {
		t.Error("info1 should be filtered")
	}
	if !strings.Contains(logContent, "info2") {
		t.Error("info2 should be present")
	}
}

func TestDisabledLoggerNoOp(t *testing.T) {
	config := &Config{
		Enabled: false,
	}

	logger, err := NewLogger(config)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}

	// These should all be no-ops and not panic
	logger.Debug("debug")
	logger.Info("info")
	logger.Warn("warn")
	logger.Error("error")
	logger.Close()
}

func TestGlobalLogger(t *testing.T) {
	tempDir := t.TempDir()
	logFileName := "global-test.log"

	config := &Config{
		LogDir:      tempDir,
		LogFileName: logFileName,
		Level:       LevelDebug,
		Enabled:     true,
	}

	// Initialize global logger
	err := Init(config)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer Close()

	if !IsEnabled() {
		t.Error("Expected global logger to be enabled")
	}

	// Use global log functions
	Debug("global debug")
	Info("global info")
	Warn("global warn")
	Error("global error")

	// Close to flush
	Close()

	// Read and verify
	logPath := filepath.Join(tempDir, logFileName)
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)

	if !strings.Contains(logContent, "global debug") {
		t.Error("Expected global debug in log")
	}
	if !strings.Contains(logContent, "global error") {
		t.Error("Expected global error in log")
	}
}

func TestGlobalLoggerNoInit(t *testing.T) {
	// Ensure global logger is closed from previous tests
	Close()

	// These should all be no-ops and not panic when not initialized
	Debug("no-init debug")
	Info("no-init info")
	Warn("no-init warn")
	Error("no-init error")

	if IsEnabled() {
		t.Error("Expected global logger to not be enabled")
	}
}

func TestLoggerWriter(t *testing.T) {
	tempDir := t.TempDir()
	logFileName := "writer-test.log"

	config := &Config{
		LogDir:      tempDir,
		LogFileName: logFileName,
		Level:       LevelDebug,
		Enabled:     true,
	}

	logger, err := NewLogger(config)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer logger.Close()

	// Get a writer and write to it
	writer := logger.Writer(LevelInfo)
	_, err = writer.Write([]byte("writer test message"))
	if err != nil {
		t.Fatalf("Writer.Write() error = %v", err)
	}

	logger.Close()

	// Read and verify
	logPath := filepath.Join(tempDir, logFileName)
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "writer test message") {
		t.Error("Expected writer message in log")
	}
}
