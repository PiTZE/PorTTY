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

	"github.com/creack/pty"
)

// PTYBridge manages the connection between a PTY and a client
type PTYBridge struct {
	cmd  *exec.Cmd
	pty  *os.File
	mu   sync.Mutex
	done chan struct{}
}

// ResizeMessage represents a terminal resize request
type ResizeMessage struct {
	Type string `json:"type"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}

// New creates a new PTY bridge
func New() (*PTYBridge, error) {
	// Clean up any existing PorTTY sessions (in case of previous abrupt termination)
	cleanupCmd := exec.Command("tmux", "kill-session", "-t", "PorTTY")
	// Ignore errors since the session might not exist
	cleanupCmd.Run()

	// Start tmux with a new session named "PorTTY"
	cmd := exec.Command("tmux", "new-session", "-A", "-s", "PorTTY")

	// Set environment variables for better terminal experience
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
	)

	// Start the command with a pty
	ptmx, err := pty.Start(cmd)
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

	log.Printf("Started PTY with tmux session")

	return &PTYBridge{
		cmd:  cmd,
		pty:  ptmx,
		done: make(chan struct{}),
	}, nil
}

// Read reads from the PTY
func (p *PTYBridge) Read(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.pty.Read(b)
}

// Write writes to the PTY
func (p *PTYBridge) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.pty.Write(b)
}

// ProcessInput processes input from the client
func (p *PTYBridge) ProcessInput(data []byte) error {
	// First, try to parse as JSON for resize messages
	var resizeMsg ResizeMessage
	if err := json.Unmarshal(data, &resizeMsg); err == nil && resizeMsg.Type == "resize" {
		return p.Resize(resizeMsg.Rows, resizeMsg.Cols)
	}

	// Otherwise, treat as regular input
	// Sanitize input (whitelist approach)
	sanitized := sanitizeInput(data)
	_, err := p.Write(sanitized)
	return err
}

// Resize resizes the PTY
func (p *PTYBridge) Resize(rows, cols int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return pty.Setsize(p.pty, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
}

// Close closes the PTY and kills the process
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

	// Kill the tmux session
	killCmd := exec.Command("tmux", "kill-session", "-t", "PorTTY")
	killCmd.Run()

	// Kill the process
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
// This is a simple implementation that allows common terminal control sequences
func sanitizeInput(data []byte) []byte {
	// For a real implementation, you would use a more sophisticated approach
	// This is a simplified version that allows ASCII and common control sequences
	result := make([]byte, 0, len(data))
	for i := 0; i < len(data); i++ {
		b := data[i]
		// Allow ASCII printable characters
		if b >= 32 && b <= 126 {
			result = append(result, b)
			continue
		}

		// Allow common control characters
		switch b {
		case 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F,
			0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1A, 0x1B, 0x1C, 0x1D, 0x1E, 0x1F, 0x7F:
			// Allow all control characters (0x01-0x1F and DEL)
			result = append(result, b)

			// For ESC sequences, include the whole sequence
			if b == 0x1B && i+1 < len(data) {
				// Handle ESC sequences
				if i+1 < len(data) && data[i+1] == '[' {
					result = append(result, data[i+1])
					i++
					// Consume until the end of the sequence
					for j := i + 1; j < len(data); j++ {
						c := data[j]
						result = append(result, c)
						i++
						// End of sequence
						if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '@' || c == '`' || c == '{' || c == '|' || c == '}' || c == '~' {
							break
						}
					}
				} else if i+1 < len(data) && data[i+1] == ']' {
					// OSC sequences (Operating System Command)
					result = append(result, data[i+1])
					i++
					// OSC sequences end with BEL (7) or ST (ESC \)
					for j := i + 1; j < len(data); j++ {
						c := data[j]
						result = append(result, c)
						i++
						if c == 0x07 { // BEL
							break
						}
						if c == 0x1B && j+1 < len(data) && data[j+1] == '\\' {
							result = append(result, data[j+1])
							i++
							break
						}
					}
				} else if i+1 < len(data) && data[i+1] == '>' {
					// Device Attributes sequences
					result = append(result, data[i+1])
					i++
					// Consume until the end of the sequence
					for j := i + 1; j < len(data); j++ {
						c := data[j]
						result = append(result, c)
						i++
						// End of sequence
						if c == 'c' {
							break
						}
					}
				} else {
					// Handle other ESC sequences
					for j := i + 1; j < len(data) && j < i+20; j++ { // Limit to 20 chars to prevent overflow
						c := data[j]
						result = append(result, c)
						i++
						// Most ESC sequences are short
						if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
							break
						}
					}
				}
			}
		}
	}
	return result
}
