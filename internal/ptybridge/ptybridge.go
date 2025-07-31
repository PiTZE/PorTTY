package ptybridge

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

// PTYBridge manages the connection between a PTY and a client
type PTYBridge struct {
	cmd         *exec.Cmd
	pty         *os.File
	done        chan struct{}
	sessionName string
	// Removed mutex for better performance
}

// Global session name for all connections
const GlobalSessionName = "PorTTY"

// Global mutex to protect session creation
var sessionMutex sync.Mutex

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

// New creates a new PTY bridge or connects to an existing one
func New() (*PTYBridge, error) {
	// Use a mutex to ensure only one process tries to create/attach to the session at a time
	sessionMutex.Lock()
	defer sessionMutex.Unlock()

	// First, check if the session already exists
	sessionExists := checkSessionExists(GlobalSessionName)

	var cmd *exec.Cmd
	var ptmx *os.File
	var err error

	if sessionExists {
		// Session exists, attach to it
		log.Printf("Attaching to existing tmux session: %s", GlobalSessionName)
		cmd = exec.Command("tmux", "attach-session", "-t", GlobalSessionName)
	} else {
		// Session doesn't exist, create a new one
		log.Printf("Creating new tmux session: %s", GlobalSessionName)

		// First, kill any orphaned sessions with our name (just to be safe)
		killCmd := exec.Command("tmux", "kill-session", "-t", GlobalSessionName)
		killCmd.Run() // Ignore errors, as the session might not exist

		// Create a new session
		cmd = exec.Command("tmux", "new-session", "-s", GlobalSessionName)
	}

	// Set environment variables for better terminal experience
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
	)

	// Start the command with a pty
	ptmx, err = pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to start pty: %w", err)
	}

	// Set initial terminal size
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

// checkSessionExists checks if a tmux session exists
func checkSessionExists(sessionName string) bool {
	cmd := exec.Command("tmux", "has-session", "-t", sessionName)
	err := cmd.Run()
	return err == nil
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
	// First, try to parse as JSON for control messages
	var msg map[string]interface{}
	if err := json.Unmarshal(data, &msg); err == nil {
		if msgType, ok := msg["type"].(string); ok {
			switch msgType {
			case "resize":
				var resizeMsg ResizeMessage
				if err := json.Unmarshal(data, &resizeMsg); err == nil {
					// Only log resize events for debugging
					// log.Printf("Resizing terminal to %d rows, %d cols", resizeMsg.Dimensions.Rows, resizeMsg.Dimensions.Cols)
					return p.Resize(resizeMsg.Dimensions.Rows, resizeMsg.Dimensions.Cols)
				} else {
					log.Printf("Error parsing resize message: %v", err)
				}
				return nil
			case "keepalive":
				// Just acknowledge the keepalive, no action needed
				return nil
			}
		}
	}

	// Write the data directly to the PTY without any processing
	// This is the fastest path for terminal input
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
	// Use a flag to prevent double-closing
	select {
	case <-p.done:
		// Already closed
		return nil
	default:
		// Signal that we're done
		close(p.done)
	}

	// We don't kill the tmux session, as it's shared between clients
	// We also don't kill the client process, as it would terminate the tmux session
	log.Printf("Client disconnected from tmux session: %s", p.sessionName)

	// Just close the PTY
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
