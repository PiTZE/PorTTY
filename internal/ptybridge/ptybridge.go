package ptybridge

// ============================================================================
// IMPORTS
// ============================================================================

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
)

// ============================================================================
// CONSTANTS AND GLOBAL VARIABLES
// ============================================================================

const GlobalSessionName = "PorTTY"

var sessionMutex sync.Mutex

// ============================================================================
// TYPE DEFINITIONS
// ============================================================================

// PTYBridge manages the connection between a PTY and a client
type PTYBridge struct {
	cmd         *exec.Cmd
	pty         *os.File
	done        chan struct{}
	sessionName string
}

// ResizeMessage represents a terminal resize request
type ResizeMessage struct {
	Type       string     `json:"type"`
	Dimensions Dimensions `json:"dimensions"`
}

// Dimensions represents terminal dimensions
type Dimensions struct {
	Cols int `json:"cols"`
	Rows int `json:"rows"`
}

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

// checkSessionExists checks if a tmux session exists
func checkSessionExists(sessionName string) bool {
	cmd := exec.Command("tmux", "has-session", "-t", sessionName)
	err := cmd.Run()
	return err == nil
}

// ============================================================================
// CORE BUSINESS LOGIC
// ============================================================================

// New creates a new PTY bridge or connects to an existing one
func New() (*PTYBridge, error) {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()

	sessionExists := checkSessionExists(GlobalSessionName)

	var cmd *exec.Cmd
	var ptmx *os.File
	var err error

	if sessionExists {
		log.Printf("Attaching to existing tmux session: %s", GlobalSessionName)
		cmd = exec.Command("tmux", "attach-session", "-t", GlobalSessionName)
	} else {
		log.Printf("Creating new tmux session: %s", GlobalSessionName)

		killCmd := exec.Command("tmux", "kill-session", "-t", GlobalSessionName)
		killCmd.Run()

		cmd = exec.Command("tmux", "new-session", "-s", GlobalSessionName)
	}

	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
	)

	ptmx, err = pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to start pty: %w", err)
	}

	if err := pty.Setsize(ptmx, &pty.Winsize{
		Rows: 24,
		Cols: 80,
	}); err != nil {
		ptmx.Close()
		cmd.Process.Kill()
		return nil, fmt.Errorf("failed to set initial terminal size: %w", err)
	}

	log.Printf("Connected to tmux session: %s", GlobalSessionName)

	return &PTYBridge{
		cmd:         cmd,
		pty:         ptmx,
		done:        make(chan struct{}),
		sessionName: GlobalSessionName,
	}, nil
}

// Read reads from the PTY directly
func (p *PTYBridge) Read(b []byte) (int, error) {
	return p.pty.Read(b)
}

// Write writes to the PTY directly
func (p *PTYBridge) Write(b []byte) (int, error) {
	return p.pty.Write(b)
}

// ProcessInput processes input from the client
func (p *PTYBridge) ProcessInput(data []byte) error {
	var msg map[string]interface{}
	if err := json.Unmarshal(data, &msg); err == nil {
		if msgType, ok := msg["type"].(string); ok {
			switch msgType {
			case "resize":
				var resizeMsg ResizeMessage
				if err := json.Unmarshal(data, &resizeMsg); err == nil {
					return p.Resize(resizeMsg.Dimensions.Rows, resizeMsg.Dimensions.Cols)
				} else {
					log.Printf("Error parsing resize message: %v", err)
				}
				return nil
			case "keepalive":
				return nil
			}
		}
	}

	_, err := p.Write(data)
	return err
}

// Resize resizes the PTY directly
func (p *PTYBridge) Resize(rows, cols int) error {
	return pty.Setsize(p.pty, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
}

// Close closes the PTY but keeps the tmux session alive
func (p *PTYBridge) Close() error {
	select {
	case <-p.done:
		return nil
	default:
		close(p.done)
	}

	log.Printf("Client disconnected from tmux session: %s", p.sessionName)

	return p.pty.Close()
}

// Done returns a channel that's closed when the PTY is closed
func (p *PTYBridge) Done() <-chan struct{} {
	return p.done
}

// Copy copies data from the reader to the PTY
func (p *PTYBridge) Copy(dst io.Writer) {
	io.Copy(dst, p.pty)
}
