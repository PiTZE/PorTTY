package config

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// ============================================================================
// CONFIGURATION TYPES
// ============================================================================

type Config struct {
	Server    ServerConfig    `toml:"server"`
	Terminal  TerminalConfig  `toml:"terminal"`
	WebSocket WebSocketConfig `toml:"websocket"`
	UI        UIConfig        `toml:"ui"`
}

type ServerConfig struct {
	DefaultAddress      string        `toml:"default_address"`
	SessionName         string        `toml:"session_name"`
	PidFileName         string        `toml:"pid_file_name"`
	Version             string        `toml:"version"`
	PidFilePermissions  os.FileMode   `toml:"pid_file_permissions"`
	ShutdownTimeout     time.Duration `toml:"shutdown_timeout"`
	FallbackTempDir     string        `toml:"fallback_temp_dir"`
	PTYOperationTimeout time.Duration `toml:"pty_operation_timeout"`
	TmuxCleanupTimeout  time.Duration `toml:"tmux_cleanup_timeout"`
	UseTmux             bool          `toml:"use_tmux"`
}

type TerminalConfig struct {
	DefaultRows  int    `toml:"default_rows"`
	DefaultCols  int    `toml:"default_cols"`
	DefaultTerm  string `toml:"default_term"`
	DefaultColor string `toml:"default_color"`
	DefaultShell string `toml:"default_shell"`
}

type WebSocketConfig struct {
	WriteWait            time.Duration `toml:"write_wait"`
	PongWait             time.Duration `toml:"pong_wait"`
	PingPeriod           time.Duration `toml:"ping_period"`
	MaxMessageSize       int64         `toml:"max_message_size"`
	MessageChannelBuffer int           `toml:"message_channel_buffer"`
	ReadBufferSize       int           `toml:"read_buffer_size"`
	WriteBufferSize      int           `toml:"write_buffer_size"`
	ErrorRetryDelay      time.Duration `toml:"error_retry_delay"`
}

type UIConfig struct {
	FontFamily string `toml:"font_family"`
	FontSize   int    `toml:"font_size"`
}

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

func getDefaultShell() string {
	if currentUser, err := user.Current(); err == nil {
		if passwdShell := getShellFromPasswd(currentUser.Username); passwdShell != "" {
			return passwdShell
		}
	}

	if shell := os.Getenv("SHELL"); shell != "" {
		if _, err := os.Stat(shell); err == nil {
			return shell
		}
	}

	commonShells := []string{
		"/run/current-system/sw/bin/zsh",
		"/run/current-system/sw/bin/bash",
		"/usr/bin/zsh",
		"/bin/zsh",
		"/usr/bin/bash",
		"/bin/bash",
		"/usr/bin/fish",
		"/bin/fish",
		"/usr/bin/sh",
		"/bin/sh",
	}

	for _, shell := range commonShells {
		if _, err := os.Stat(shell); err == nil {
			return shell
		}
	}

	return "/bin/sh"
}

func getShellFromPasswd(username string) string {
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
				if _, err := os.Stat(shell); err == nil {
					return shell
				}
			}
		}
	}

	return ""
}

func getSystemMonospaceFont() string {
	return "monospace"
}

func getConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".portty")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}

	return configDir, nil
}

func getConfigPath() (string, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(configDir, "config.toml"), nil
}

// ============================================================================
// CONFIGURATION DEFAULTS
// ============================================================================

func newDefaultConfig() *Config {
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
			UseTmux:             false,
		},
		Terminal: TerminalConfig{
			DefaultRows:  24,
			DefaultCols:  80,
			DefaultTerm:  "xterm-256color",
			DefaultColor: "truecolor",
			DefaultShell: getDefaultShell(),
		},
		WebSocket: WebSocketConfig{
			WriteWait:            10 * time.Second,
			PongWait:             60 * time.Second,
			PingPeriod:           (60 * time.Second * 9) / 10,
			MaxMessageSize:       16384,
			MessageChannelBuffer: 100,
			ReadBufferSize:       4096,
			WriteBufferSize:      4096,
			ErrorRetryDelay:      50 * time.Millisecond,
		},
		UI: UIConfig{
			FontFamily: getSystemMonospaceFont(),
			FontSize:   14,
		},
	}
}

// ============================================================================
// CONFIGURATION LOADING AND SAVING
// ============================================================================

func Load() (*Config, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get config path: %w", err)
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		config := newDefaultConfig()
		if err := config.Save(); err != nil {
			return nil, fmt.Errorf("failed to create default config: %w", err)
		}
		return config, nil
	}

	var config Config
	if _, err := toml.DecodeFile(configPath, &config); err != nil {
		return nil, fmt.Errorf("failed to decode config file: %w", err)
	}

	return &config, nil
}

func (c *Config) Save() error {
	configPath, err := getConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}

	file, err := os.Create(configPath)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer file.Close()

	encoder := toml.NewEncoder(file)
	if err := encoder.Encode(c); err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	return nil
}

// ============================================================================
// GLOBAL CONFIGURATION INSTANCE
// ============================================================================

var Default *Config

func init() {
	var err error
	Default, err = Load()
	if err != nil {
		Default = newDefaultConfig()
	}
}
