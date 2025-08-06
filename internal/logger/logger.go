package logger

// ============================================================================
// IMPORTS
// ============================================================================

import (
	"fmt"
	"log"
	"os"
	"time"
)

// ============================================================================
// TYPE DEFINITIONS
// ============================================================================

// Logger interface defines structured logging methods
type Logger interface {
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, err error, fields ...Field)
	Fatal(msg string, err error, fields ...Field)
}

// Field represents a key-value pair for structured logging
type Field struct {
	Key   string
	Value interface{}
}

// logger implements the Logger interface
type logger struct {
	component string
}

// ============================================================================
// FIELD HELPER FUNCTIONS
// ============================================================================

// String creates a string field
func String(key, value string) Field {
	return Field{Key: key, Value: value}
}

// Int creates an integer field
func Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

// Error creates an error field
func Error(err error) Field {
	return Field{Key: "error", Value: err}
}

// Duration creates a duration field
func Duration(key string, value time.Duration) Field {
	return Field{Key: key, Value: value}
}

// Bool creates a boolean field
func Bool(key string, value bool) Field {
	return Field{Key: key, Value: value}
}

// ============================================================================
// CORE BUSINESS LOGIC
// ============================================================================

// New creates a new logger instance for a specific component
func New(component string) Logger {
	return &logger{
		component: component,
	}
}

// formatMessage formats a log message with fields
func (l *logger) formatMessage(level, msg string, fields []Field) string {
	timestamp := time.Now().Format("2006/01/02 15:04:05")
	formatted := fmt.Sprintf("%s [%s] %s: %s", timestamp, level, l.component, msg)

	if len(fields) > 0 {
		formatted += " |"
		for _, field := range fields {
			formatted += fmt.Sprintf(" %s=%v", field.Key, field.Value)
		}
	}

	return formatted
}

// Info logs an informational message
func (l *logger) Info(msg string, fields ...Field) {
	formatted := l.formatMessage("INFO", msg, fields)
	log.Println(formatted)
}

// Warn logs a warning message
func (l *logger) Warn(msg string, fields ...Field) {
	formatted := l.formatMessage("WARN", msg, fields)
	log.Println(formatted)
}

// Error logs an error message
func (l *logger) Error(msg string, err error, fields ...Field) {
	allFields := append(fields, Error(err))
	formatted := l.formatMessage("ERROR", msg, allFields)
	log.Println(formatted)
}

// Fatal logs a fatal error message and exits
func (l *logger) Fatal(msg string, err error, fields ...Field) {
	allFields := append(fields, Error(err))
	formatted := l.formatMessage("FATAL", msg, allFields)
	log.Println(formatted)
	os.Exit(1)
}

// ============================================================================
// GLOBAL LOGGER INSTANCES
// ============================================================================

var (
	// ServerLogger is the logger for server operations
	ServerLogger = New("server")

	// WebSocketLogger is the logger for WebSocket operations
	WebSocketLogger = New("websocket")

	// PTYBridgeLogger is the logger for PTY bridge operations
	PTYBridgeLogger = New("ptybridge")
)
