package ptybridge

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
)

// PTYBridge manages the connection between a PTY and a client
type PTYBridge struct {
	cmd         *exec.Cmd
	pty         *os.File
	mu          sync.Mutex
	done        chan struct{}
	sessionName string
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

// Read reads from the PTY
func (p *PTYBridge) Read(b []byte) (int, error) {
	// Use a timeout to prevent deadlocks
	readChan := make(chan readResult, 1)

	go func() {
		p.mu.Lock()
		defer p.mu.Unlock()
		n, err := p.pty.Read(b)
		readChan <- readResult{n: n, err: err}
	}()

	// Wait for the read to complete with a timeout
	select {
	case result := <-readChan:
		return result.n, result.err
	case <-time.After(5 * time.Second):
		// If we timeout, return a temporary error
		return 0, fmt.Errorf("read timeout")
	}
}

// readResult holds the result of a read operation
type readResult struct {
	n   int
	err error
}

// Write writes to the PTY
func (p *PTYBridge) Write(b []byte) (int, error) {
	// Use a timeout to prevent deadlocks
	writeChan := make(chan writeResult, 1)

	go func() {
		p.mu.Lock()
		defer p.mu.Unlock()
		n, err := p.pty.Write(b)
		writeChan <- writeResult{n: n, err: err}
	}()

	// Wait for the write to complete with a timeout
	select {
	case result := <-writeChan:
		return result.n, result.err
	case <-time.After(5 * time.Second):
		// If we timeout, return a temporary error
		return 0, fmt.Errorf("write timeout")
	}
}

// writeResult holds the result of a write operation
type writeResult struct {
	n   int
	err error
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
					log.Printf("Resizing terminal to %d rows, %d cols", resizeMsg.Dimensions.Rows, resizeMsg.Dimensions.Cols)
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

	// Apply basic sanitization to prevent security issues
	// but preserve all terminal control sequences
	sanitized := sanitizeInput(data)

	// Write the data to the PTY
	n, err := p.Write(sanitized)
	if err != nil {
		log.Printf("Error writing to PTY: %v", err)
	} else if n != len(sanitized) {
		log.Printf("Warning: Only wrote %d of %d bytes to PTY", n, len(sanitized))
	}

	return err
}

// Resize resizes the PTY
func (p *PTYBridge) Resize(rows, cols int) error {
	// Use a timeout to prevent deadlocks
	resizeChan := make(chan error, 1)

	go func() {
		p.mu.Lock()
		defer p.mu.Unlock()
		err := pty.Setsize(p.pty, &pty.Winsize{
			Rows: uint16(rows),
			Cols: uint16(cols),
		})
		resizeChan <- err
	}()

	// Wait for the resize to complete with a timeout
	select {
	case err := <-resizeChan:
		return err
	case <-time.After(5 * time.Second):
		// If we timeout, return a temporary error
		return fmt.Errorf("resize timeout")
	}
}

// Close closes the PTY but keeps the tmux session alive
func (p *PTYBridge) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Use a flag to prevent double-closing
	select {
	case <-p.done:
		// Already closed
		return nil
	default:
		// Signal that we're done
		close(p.done)
	}

	// We don't kill the tmux session anymore, as it's shared between clients
	log.Printf("Client disconnected from tmux session: %s", p.sessionName)

	// Kill the client process (not the session)
	if p.cmd.Process != nil {
		p.cmd.Process.Signal(syscall.SIGTERM)
		p.cmd.Process.Kill()
	}

	// Close the PTY
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

// sanitizeInput sanitizes input to prevent security issues
// This is a simplified implementation that allows all common terminal control sequences
func sanitizeInput(data []byte) []byte {
	// For security reasons, we'll still do basic filtering
	// but we'll be much more permissive to avoid breaking terminal functionality

	// Quick check for empty data
	if len(data) == 0 {
		return data
	}

	// Check for extremely large inputs that might be malicious
	if len(data) > 8192 {
		// Truncate to a reasonable size
		data = data[:8192]
	}

	// For most terminal input, we'll just pass it through
	// This avoids complex parsing that could break escape sequences
	return data
}
