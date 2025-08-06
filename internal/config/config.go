package config

import (
	"os"
	"os/user"
	"strings"
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
	UseTmux             bool
}

// TerminalConfig contains terminal-related configuration
type TerminalConfig struct {
	DefaultRows  int
	DefaultCols  int
	DefaultTerm  string
	DefaultColor string
	DefaultShell string
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
// UTILITY FUNCTIONS
// ============================================================================

// getDefaultShell returns the user's default shell
func getDefaultShell() string {
	// Try to get shell from user info and /etc/passwd first (more reliable)
	if currentUser, err := user.Current(); err == nil {
		// Try to read from /etc/passwd for the user's default shell
		if passwdShell := getShellFromPasswd(currentUser.Username); passwdShell != "" {
			return passwdShell
		}
	}

	// Fallback to environment variable
	if shell := os.Getenv("SHELL"); shell != "" {
		// Verify the shell exists
		if _, err := os.Stat(shell); err == nil {
			return shell
		}
	}

	// Check common shell locations in order of preference
	// Include NixOS and other distribution paths
	commonShells := []string{
		"/run/current-system/sw/bin/zsh",  // NixOS zsh
		"/run/current-system/sw/bin/bash", // NixOS bash
		"/usr/bin/zsh",                    // Standard zsh
		"/bin/zsh",                        // Alternative zsh
		"/usr/bin/bash",                   // Standard bash
		"/bin/bash",                       // Alternative bash
		"/usr/bin/fish",                   // Fish shell
		"/bin/fish",                       // Alternative fish
		"/usr/bin/sh",                     // Standard sh
		"/bin/sh",                         // Alternative sh
	}

	for _, shell := range commonShells {
		if _, err := os.Stat(shell); err == nil {
			return shell
		}
	}

	// Final fallback to /bin/sh
	return "/bin/sh"
}

// getShellFromPasswd tries to get the user's shell from /etc/passwd
func getShellFromPasswd(username string) string {
	// This is a simple implementation - in production you might want to use
	// a more robust method or a library
	passwdContent, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return ""
	}

	lines := strings.Split(string(passwdContent), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, username+":") {
			fields := strings.Split(line, ":")
			if len(fields) >= 7 {
				shell := fields[6]
				// Verify the shell exists
				if _, err := os.Stat(shell); err == nil {
					return shell
				}
			}
		}
	}

	return ""
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
			UseTmux:             false, // Default to direct shell mode
		},
		Terminal: TerminalConfig{
			DefaultRows:  24,
			DefaultCols:  80,
			DefaultTerm:  "xterm-256color",
			DefaultColor: "truecolor",
			DefaultShell: getDefaultShell(), // Get user's default shell
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
