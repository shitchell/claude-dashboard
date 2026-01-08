// Package logging provides file-based logging for claude-dashboard.
// Logs are written to a file (not stderr) to avoid corrupting the TUI.
package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Log level constants define the severity of log messages.
type Level int

const (
	// LevelDebug is for detailed debugging information.
	LevelDebug Level = iota

	// LevelInfo is for general informational messages.
	LevelInfo

	// LevelWarn is for warning conditions.
	LevelWarn

	// LevelError is for error conditions.
	LevelError
)

// String returns a human-readable representation of the log level.
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// DefaultLogFileName is the default name for the log file.
const DefaultLogFileName = "claude-dashboard.log"

// Logger is a file-based logger that avoids stdout/stderr.
// This is critical for TUI applications to prevent log messages
// from corrupting the terminal display.
type Logger struct {
	// file is the underlying file handle for log output.
	file *os.File

	// logger is the standard library logger used for formatted output.
	logger *log.Logger

	// level is the minimum level for messages to be logged.
	level Level

	// mu protects concurrent access to the logger.
	mu sync.Mutex

	// enabled indicates whether logging is active.
	// When false, all log calls are no-ops.
	enabled bool
}

// Config contains configuration options for the logger.
type Config struct {
	// LogDir is the directory where log files are stored.
	// If empty, defaults to ~/.cache/claude-dashboard/logs.
	LogDir string

	// LogFileName is the name of the log file.
	// If empty, defaults to DefaultLogFileName.
	LogFileName string

	// Level is the minimum log level to record.
	// Defaults to LevelInfo.
	Level Level

	// Enabled indicates whether logging is active.
	// Defaults to true.
	Enabled bool
}

// DefaultConfig returns the default logger configuration.
func DefaultConfig() Config {
	return Config{
		Level:   LevelInfo,
		Enabled: true,
	}
}

// globalLogger is the package-level logger instance.
var globalLogger *Logger

// globalMu protects initialization of the global logger.
var globalMu sync.Mutex

// Init initializes the global logger with the given configuration.
// If called multiple times, subsequent calls close the previous logger
// and create a new one.
//
// Returns an error if the log file cannot be created.
func Init(config *Config) error {
	globalMu.Lock()
	defer globalMu.Unlock()

	// Close existing logger if present
	if globalLogger != nil {
		globalLogger.Close()
	}

	var err error
	globalLogger, err = NewLogger(config)
	return err
}

// NewLogger creates a new Logger with the given configuration.
// If config is nil, default configuration is used.
func NewLogger(config *Config) (*Logger, error) {
	if config == nil {
		defaultConfig := DefaultConfig()
		config = &defaultConfig
	}

	logger := &Logger{
		level:   config.Level,
		enabled: config.Enabled,
	}

	if !logger.enabled {
		// Return a disabled logger - no file needed
		return logger, nil
	}

	// Resolve log directory
	logDir := config.LogDir
	if logDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		logDir = filepath.Join(homeDir, ".cache", "claude-dashboard", "logs")
	}

	// Create log directory if it doesn't exist
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Resolve log file name
	logFileName := config.LogFileName
	if logFileName == "" {
		logFileName = DefaultLogFileName
	}

	logPath := filepath.Join(logDir, logFileName)

	// Open log file (append mode, create if not exists)
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	logger.file = file
	logger.logger = log.New(file, "", 0)

	return logger, nil
}

// Close closes the log file and releases resources.
// After Close, all log calls are no-ops.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.enabled = false
	if l.file != nil {
		err := l.file.Close()
		l.file = nil
		l.logger = nil
		return err
	}
	return nil
}

// SetLevel changes the minimum log level.
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// log writes a message at the specified level.
func (l *Logger) log(level Level, format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.enabled || l.logger == nil {
		return
	}

	if level < l.level {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	message := fmt.Sprintf(format, args...)
	l.logger.Printf("%s [%s] %s", timestamp, level.String(), message)
}

// Debug logs a debug message.
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(LevelDebug, format, args...)
}

// Info logs an informational message.
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(LevelInfo, format, args...)
}

// Warn logs a warning message.
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(LevelWarn, format, args...)
}

// Error logs an error message.
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(LevelError, format, args...)
}

// Writer returns an io.Writer that writes to the log at the specified level.
// This is useful for integrating with libraries that expect an io.Writer.
func (l *Logger) Writer(level Level) io.Writer {
	return &levelWriter{logger: l, level: level}
}

// levelWriter implements io.Writer and writes to the logger at a specific level.
type levelWriter struct {
	logger *Logger
	level  Level
}

func (w *levelWriter) Write(p []byte) (n int, err error) {
	w.logger.log(w.level, "%s", string(p))
	return len(p), nil
}

// Package-level convenience functions that use the global logger.

// Debug logs a debug message to the global logger.
func Debug(format string, args ...interface{}) {
	globalMu.Lock()
	l := globalLogger
	globalMu.Unlock()

	if l != nil {
		l.Debug(format, args...)
	}
}

// Info logs an informational message to the global logger.
func Info(format string, args ...interface{}) {
	globalMu.Lock()
	l := globalLogger
	globalMu.Unlock()

	if l != nil {
		l.Info(format, args...)
	}
}

// Warn logs a warning message to the global logger.
func Warn(format string, args ...interface{}) {
	globalMu.Lock()
	l := globalLogger
	globalMu.Unlock()

	if l != nil {
		l.Warn(format, args...)
	}
}

// Error logs an error message to the global logger.
func Error(format string, args ...interface{}) {
	globalMu.Lock()
	l := globalLogger
	globalMu.Unlock()

	if l != nil {
		l.Error(format, args...)
	}
}

// Close closes the global logger.
func Close() error {
	globalMu.Lock()
	defer globalMu.Unlock()

	if globalLogger != nil {
		err := globalLogger.Close()
		globalLogger = nil
		return err
	}
	return nil
}

// IsEnabled returns whether the global logger is active.
func IsEnabled() bool {
	globalMu.Lock()
	defer globalMu.Unlock()

	return globalLogger != nil && globalLogger.enabled
}
