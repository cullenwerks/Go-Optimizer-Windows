package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogLevel represents the severity of a log message.
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
)

// String returns the human-readable name for a LogLevel.
func (ll LogLevel) String() string {
	switch ll {
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

// Logger provides structured, leveled logging to a file and optionally to the
// console (stdout). All writes are serialised with a mutex so the logger is
// safe for concurrent use.
type Logger struct {
	level   LogLevel
	file    *os.File
	console bool
	mu      sync.Mutex
}

// DefaultLogger is the package-level logger instance that the convenience
// functions (Debug, Info, Warn, Error) delegate to.  Set it with SetDefault.
var DefaultLogger *Logger

// SetDefault assigns l as the package-level DefaultLogger.
func SetDefault(l *Logger) {
	DefaultLogger = l
}

// DefaultLogPath returns the default log file path.  On Windows this resolves
// to %APPDATA%\SysCleaner\syscleaner.log; on other platforms it falls back to
// the user config directory provided by os.UserConfigDir.
func DefaultLogPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		// Fallback: write next to the executable.
		configDir = "."
	}
	return filepath.Join(configDir, "SysCleaner", "syscleaner.log")
}

// New creates a new Logger.  The log file at logPath is opened in append mode
// and created (together with any missing parent directories) if it does not
// already exist.  If console is true every log line is additionally written to
// os.Stdout.
func New(level LogLevel, logPath string, console bool) (*Logger, error) {
	// Ensure the parent directory exists.
	dir := filepath.Dir(logPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("logger: create log directory %s: %w", dir, err)
	}

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("logger: open log file %s: %w", logPath, err)
	}

	return &Logger{
		level:   level,
		file:    f,
		console: console,
	}, nil
}

// Close closes the underlying log file.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.file.Close()
}

// log is the internal method that formats and writes a single log line.  Lines
// that are below the configured level are silently discarded.
//
// Format: [2006-01-02 15:04:05] [LEVEL] message
func (l *Logger) log(level LogLevel, format string, args ...interface{}) {
	if level < l.level {
		return
	}

	msg := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	line := fmt.Sprintf("[%s] [%s] %s\n", timestamp, level.String(), msg)

	l.mu.Lock()
	defer l.mu.Unlock()

	// Write to the log file.
	_, _ = l.file.WriteString(line)

	// Optionally mirror to the console.
	if l.console {
		fmt.Print(line)
	}
}

// Debug logs a message at Debug level.
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(LevelDebug, format, args...)
}

// Info logs a message at Info level.
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(LevelInfo, format, args...)
}

// Warn logs a message at Warn level.
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(LevelWarn, format, args...)
}

// Error logs a message at Error level.
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(LevelError, format, args...)
}

// ---------------------------------------------------------------------------
// Package-level convenience functions — these delegate to DefaultLogger.
// If DefaultLogger is nil the message is routed through the standard "log"
// package so that callers never silently lose log output.
// ---------------------------------------------------------------------------

// Debug logs a message at Debug level using the DefaultLogger.
func Debug(format string, args ...interface{}) {
	if DefaultLogger != nil {
		DefaultLogger.Debug(format, args...)
		return
	}
	log.Printf("[DEBUG] "+format, args...)
}

// Info logs a message at Info level using the DefaultLogger.
func Info(format string, args ...interface{}) {
	if DefaultLogger != nil {
		DefaultLogger.Info(format, args...)
		return
	}
	log.Printf("[INFO] "+format, args...)
}

// Warn logs a message at Warn level using the DefaultLogger.
func Warn(format string, args ...interface{}) {
	if DefaultLogger != nil {
		DefaultLogger.Warn(format, args...)
		return
	}
	log.Printf("[WARN] "+format, args...)
}

// Error logs a message at Error level using the DefaultLogger.
func Error(format string, args ...interface{}) {
	if DefaultLogger != nil {
		DefaultLogger.Error(format, args...)
		return
	}
	log.Printf("[ERROR] "+format, args...)
}
