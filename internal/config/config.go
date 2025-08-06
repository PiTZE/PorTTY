package config

import (
	"os"
	"time"
)

// ============================================================================
// CONFIGURATION TYPES
// ============================================================================

// Config represents the complete application configuration
type Config struct {
	Server    ServerConfig
	Terminal  TerminalConfig
	WebSocket WebSocketConfig
}

// ServerConfig contains server-related configuration
type ServerConfig struct {
	DefaultAddress      string
	SessionName         string
	PidFileName         string
	Version             string
	PidFilePermissions  os.FileMode
	ShutdownTimeout     time.Duration
	FallbackTempDir     string
	PTYOperationTimeout time.Duration
	TmuxCleanupTimeout  time.Duration
}

// TerminalConfig contains terminal-related configuration
type TerminalConfig struct {
	DefaultRows  int
	DefaultCols  int
	DefaultTerm  string
	DefaultColor string
}

// WebSocketConfig contains WebSocket-related configuration
type WebSocketConfig struct {
	WriteWait            time.Duration
	PongWait             time.Duration
	PingPeriod           time.Duration
	MaxMessageSize       int64
	MessageChannelBuffer int
	ReadBufferSize       int
	WriteBufferSize      int
	ErrorRetryDelay      time.Duration
}

// ============================================================================
// CONFIGURATION CONSTANTS
// ============================================================================

// NewDefault returns a Config with all default values
func NewDefault() *Config {
	return &Config{
		Server: ServerConfig{
			DefaultAddress:      "localhost:7314",
			SessionName:         "PorTTY",
			PidFileName:         ".portty.pid",
			Version:             "v0.2",
			PidFilePermissions:  0644,
			ShutdownTimeout:     5 * time.Second,
			FallbackTempDir:     "/tmp",
			PTYOperationTimeout: 3 * time.Second,
			TmuxCleanupTimeout:  2 * time.Second,
		},
		Terminal: TerminalConfig{
			DefaultRows:  24,
			DefaultCols:  80,
			DefaultTerm:  "xterm-256color",
			DefaultColor: "truecolor",
		},
		WebSocket: WebSocketConfig{
			WriteWait:            10 * time.Second,
			PongWait:             60 * time.Second,
			PingPeriod:           (60 * time.Second * 9) / 10, // (pongWait * 9) / 10
			MaxMessageSize:       16384,
			MessageChannelBuffer: 100,
			ReadBufferSize:       4096,
			WriteBufferSize:      4096,
			ErrorRetryDelay:      50 * time.Millisecond,
		},
	}
}

// ============================================================================
// GLOBAL CONFIGURATION INSTANCE
// ============================================================================

// Default holds the default configuration instance
var Default = NewDefault()
