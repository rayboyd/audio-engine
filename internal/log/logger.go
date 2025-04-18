package log

import (
	"fmt"
	stdlog "log"
	"os"
	"strings"
	"sync/atomic"
)

// LogLevel defines the severity of a log message.
type LogLevel uint32

// Constants for log levels.
const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

// String returns the string representation of the LogLevel.
func (l LogLevel) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// ParseLevel converts a string (case-insensitive) to a LogLevel.
// Returns LevelInfo and false if the string is not recognized.
func ParseLevel(levelStr string) (LogLevel, bool) {
	switch strings.ToUpper(levelStr) {
	case "DEBUG":
		return LevelDebug, true
	case "INFO":
		return LevelInfo, true
	case "WARN", "WARNING":
		return LevelWarn, true
	case "ERROR":
		return LevelError, true
	case "FATAL":
		return LevelFatal, true
	default:
		return LevelInfo, false // Default to Info on parse error
	}
}

// --- Global Logger State ---

// currentLevel holds the current global log level atomically.
var currentLevel atomic.Uint32

// logger is the standard logger instance used internally.
// We configure it to show date, time with microseconds.
var logger = stdlog.New(os.Stderr, "", stdlog.Ldate|stdlog.Ltime|stdlog.Lmicroseconds)

func init() {
	// Default level at startup. Can be overridden by config.
	SetLevel(LevelInfo)
}

// SetLevel sets the global logging level atomically.
func SetLevel(level LogLevel) {
	currentLevel.Store(uint32(level))
	// Optionally log the level change itself, using a direct logger call
	// logger.Printf("Log level set to %s", level.String())
}

// GetLevel gets the current global logging level atomically.
func GetLevel() LogLevel {
	return LogLevel(currentLevel.Load())
}

// shouldLog checks if a message at the given level should be logged based on the current global level.
func shouldLog(level LogLevel) bool {
	return level >= GetLevel()
}

// --- Public Logging Functions ---

// Debugf logs a formatted debug message if the level is appropriate.
func Debugf(format string, v ...interface{}) {
	if shouldLog(LevelDebug) {
		logger.Printf("[%s] %s", LevelDebug, fmt.Sprintf(format, v...))
	}
}

// Infof logs a formatted info message if the level is appropriate.
func Infof(format string, v ...interface{}) {
	if shouldLog(LevelInfo) {
		logger.Printf("[%s]  %s", LevelInfo, fmt.Sprintf(format, v...))
	}
}

// Warnf logs a formatted warning message if the level is appropriate.
func Warnf(format string, v ...interface{}) {
	if shouldLog(LevelWarn) {
		logger.Printf("[%s]  %s", LevelWarn, fmt.Sprintf(format, v...))
	}
}

// Errorf logs a formatted error message if the level is appropriate.
func Errorf(format string, v ...interface{}) {
	if shouldLog(LevelError) {
		logger.Printf("[%s] %s", LevelError, fmt.Sprintf(format, v...))
	}
}

// Fatalf logs a formatted fatal message and exits the application.
// Fatal messages are always logged regardless of the current level.
func Fatalf(format string, v ...interface{}) {
	// Fatal always logs and then exits
	logger.Fatalf("[%s] %s", LevelFatal, fmt.Sprintf(format, v...))
}

// --- Functions without formatting (convenience) ---

// Debug logs a debug message if the level is appropriate.
func Debug(v ...interface{}) {
	if shouldLog(LevelDebug) {
		logger.Printf("[%s] %s", LevelDebug, fmt.Sprint(v...))
	}
}

// Info logs an info message if the level is appropriate.
func Info(v ...interface{}) {
	if shouldLog(LevelInfo) {
		logger.Printf("[%s]  %s", LevelInfo, fmt.Sprint(v...))
	}
}

// Warn logs a warning message if the level is appropriate.
func Warn(v ...interface{}) {
	if shouldLog(LevelWarn) {
		logger.Printf("[%s]  %s", LevelWarn, fmt.Sprint(v...))
	}
}

// Error logs an error message if the level is appropriate.
func Error(v ...interface{}) {
	if shouldLog(LevelError) {
		logger.Printf("[%s] %s", LevelError, fmt.Sprint(v...))
	}
}

// Fatal logs a fatal message and exits the application.
func Fatal(v ...interface{}) {
	logger.Fatalf("[%s] %s", LevelFatal, fmt.Sprint(v...))
}
