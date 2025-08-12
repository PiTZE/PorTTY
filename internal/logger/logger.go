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

type Logger interface {
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, err error, fields ...Field)
	Fatal(msg string, err error, fields ...Field)
}

type Field struct {
	Key   string
	Value interface{}
}

type logger struct {
	component string
}

// ============================================================================
// FIELD HELPER FUNCTIONS
// ============================================================================

func String(key, value string) Field {
	return Field{Key: key, Value: value}
}

func Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

func Error(err error) Field {
	return Field{Key: "error", Value: err}
}

func Duration(key string, value time.Duration) Field {
	return Field{Key: key, Value: value}
}

func Bool(key string, value bool) Field {
	return Field{Key: key, Value: value}
}

// ============================================================================
// CORE BUSINESS LOGIC
// ============================================================================

func New(component string) Logger {
	return &logger{
		component: component,
	}
}

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

func (l *logger) Info(msg string, fields ...Field) {
	formatted := l.formatMessage("INFO", msg, fields)
	log.Println(formatted)
}

func (l *logger) Warn(msg string, fields ...Field) {
	formatted := l.formatMessage("WARN", msg, fields)
	log.Println(formatted)
}

func (l *logger) Error(msg string, err error, fields ...Field) {
	allFields := append(fields, Error(err))
	formatted := l.formatMessage("ERROR", msg, allFields)
	log.Println(formatted)
}

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
	ServerLogger    = New("server")
	WebSocketLogger = New("websocket")
	PTYBridgeLogger = New("ptybridge")
)
