package ptybridge

// ============================================================================
// IMPORTS
// ============================================================================

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/PiTZE/PorTTY/internal/config"
	"github.com/PiTZE/PorTTY/internal/interfaces"
	"github.com/PiTZE/PorTTY/internal/logger"
	"github.com/creack/pty"
)

// ============================================================================
// CONSTANTS AND GLOBAL VARIABLES
// ============================================================================

// Configuration instance
var cfg = config.Default

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
	ctx         context.Context
	cancel      context.CancelFunc
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

// New creates a new PTY bridge or connects to an existing one with context support
func New(parentCtx context.Context) (*PTYBridge, error) {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()

	// Create context for this PTY bridge
	ctx, cancel := context.WithCancel(parentCtx)

	var cmd *exec.Cmd
	var ptmx *os.File
	var err error
	var sessionName string

	if cfg.Server.UseTmux {
		// Tmux mode - existing behavior
		sessionExists := checkSessionExists(cfg.Server.SessionName)
		sessionName = cfg.Server.SessionName

		if sessionExists {
			logger.PTYBridgeLogger.Info("Attaching to existing tmux session", logger.String("session", cfg.Server.SessionName))
			cmd = exec.CommandContext(ctx, "tmux", "attach-session", "-t", cfg.Server.SessionName)
		} else {
			logger.PTYBridgeLogger.Info("Creating new tmux session", logger.String("session", cfg.Server.SessionName))

			// Kill any existing session with context
			killCmd := exec.CommandContext(ctx, "tmux", "kill-session", "-t", cfg.Server.SessionName)
			killCmd.Run()

			cmd = exec.CommandContext(ctx, "tmux", "new-session", "-s", cfg.Server.SessionName)
		}
	} else {
		// Direct shell mode - use user's default shell
		logger.PTYBridgeLogger.Info("Starting direct shell session", logger.String("shell", cfg.Terminal.DefaultShell))
		cmd = exec.CommandContext(ctx, cfg.Terminal.DefaultShell)
		sessionName = "DirectShell"
	}

	cmd.Env = append(os.Environ(),
		"TERM="+cfg.Terminal.DefaultTerm,
		"COLORTERM="+cfg.Terminal.DefaultColor,
	)

	ptmx, err = pty.Start(cmd)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start pty: %w", err)
	}

	if err := pty.Setsize(ptmx, &pty.Winsize{
		Rows: uint16(cfg.Terminal.DefaultRows),
		Cols: uint16(cfg.Terminal.DefaultCols),
	}); err != nil {
		ptmx.Close()
		cmd.Process.Kill()
		cancel()
		return nil, fmt.Errorf("failed to set initial terminal size: %w", err)
	}

	if cfg.Server.UseTmux {
		logger.PTYBridgeLogger.Info("Connected to tmux session", logger.String("session", cfg.Server.SessionName))
	} else {
		logger.PTYBridgeLogger.Info("Connected to direct shell", logger.String("shell", cfg.Terminal.DefaultShell))
	}

	bridge := &PTYBridge{
		cmd:         cmd,
		pty:         ptmx,
		done:        make(chan struct{}),
		sessionName: sessionName,
		ctx:         ctx,
		cancel:      cancel,
	}

	// Start monitoring context cancellation
	go bridge.monitorContext()

	return bridge, nil
}

// monitorContext monitors the context and handles cancellation
func (p *PTYBridge) monitorContext() {
	<-p.ctx.Done()
	logger.PTYBridgeLogger.Info("PTY bridge context cancelled, initiating shutdown")
	p.Close()
}

// Read reads data from the PTY with context awareness
func (p *PTYBridge) Read(ctx context.Context, b []byte) (int, error) {
	// Check if context is cancelled before reading
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-p.ctx.Done():
		return 0, p.ctx.Err()
	default:
	}

	// Use a channel to make the read operation cancellable
	type readResult struct {
		n   int
		err error
	}

	resultChan := make(chan readResult, 1)
	go func() {
		n, err := p.pty.Read(b)
		resultChan <- readResult{n, err}
	}()

	select {
	case result := <-resultChan:
		return result.n, result.err
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-p.ctx.Done():
		return 0, p.ctx.Err()
	}
}

// Write writes data to the PTY with context awareness
func (p *PTYBridge) Write(ctx context.Context, b []byte) (int, error) {
	// Check if context is cancelled before writing
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-p.ctx.Done():
		return 0, p.ctx.Err()
	default:
	}

	// Use a channel to make the write operation cancellable
	type writeResult struct {
		n   int
		err error
	}

	resultChan := make(chan writeResult, 1)
	go func() {
		n, err := p.pty.Write(b)
		resultChan <- writeResult{n, err}
	}()

	select {
	case result := <-resultChan:
		return result.n, result.err
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-p.ctx.Done():
		return 0, p.ctx.Err()
	}
}

// ProcessInput processes input messages from WebSocket clients with context awareness
func (p *PTYBridge) ProcessInput(ctx context.Context, data []byte) error {
	// Check if context is cancelled before processing
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-p.ctx.Done():
		return p.ctx.Err()
	default:
	}

	var msg map[string]interface{}
	if err := json.Unmarshal(data, &msg); err == nil {
		if msgType, ok := msg["type"].(string); ok {
			switch msgType {
			case "resize":
				var resizeMsg ResizeMessage
				if err := json.Unmarshal(data, &resizeMsg); err == nil {
					return p.Resize(resizeMsg.Dimensions.Rows, resizeMsg.Dimensions.Cols)
				} else {
					logger.PTYBridgeLogger.Error("failed to parse resize message", err)
				}
				return nil
			case "keepalive":
				return nil
			}
		}
	}

	_, err := p.Write(ctx, data)
	return err
}

// Resize changes the terminal dimensions to the specified rows and columns
func (p *PTYBridge) Resize(rows, cols int) error {
	return pty.Setsize(p.pty, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
}

// Close closes the PTY connection while preserving the tmux session for reconnection
func (p *PTYBridge) Close() error {
	select {
	case <-p.done:
		return nil
	default:
		close(p.done)
	}

	if cfg.Server.UseTmux {
		logger.PTYBridgeLogger.Info("Client disconnected from tmux session", logger.String("session", p.sessionName))
	} else {
		logger.PTYBridgeLogger.Info("Client disconnected from direct shell", logger.String("session", p.sessionName))
	}

	// Cancel the context to signal shutdown
	if p.cancel != nil {
		p.cancel()
	}

	// Close PTY with timeout
	done := make(chan error, 1)
	go func() {
		done <- p.pty.Close()
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(cfg.Server.PTYOperationTimeout):
		logger.PTYBridgeLogger.Warn("PTY close operation timed out")
		return fmt.Errorf("PTY close timeout after %v", cfg.Server.PTYOperationTimeout)
	}
}

// Done returns a channel that signals when the PTY bridge is closed
func (p *PTYBridge) Done() <-chan struct{} {
	return p.done
}

// Copy continuously copies data from the PTY to the specified writer
func (p *PTYBridge) Copy(dst io.Writer) {
	io.Copy(dst, p.pty)
}

// ============================================================================
// INTERFACE COMPLIANCE CHECKS
// ============================================================================

// Compile-time interface compliance checks
var (
	_ interfaces.PTYReader         = (*PTYBridge)(nil)
	_ interfaces.PTYWriter         = (*PTYBridge)(nil)
	_ interfaces.PTYResizer        = (*PTYBridge)(nil)
	_ interfaces.PTYInputProcessor = (*PTYBridge)(nil)
	_ interfaces.PTYLifecycle      = (*PTYBridge)(nil)
	_ interfaces.PTYManager        = (*PTYBridge)(nil)
	_ interfaces.PTYCopier         = (*PTYBridge)(nil)
	_ interfaces.PTYBridge         = (*PTYBridge)(nil)
)

// ============================================================================
// FACTORY FUNCTIONS
// ============================================================================

// Factory implements the PTYBridgeFactory interface
type Factory struct{}

// NewFactory creates a new PTY bridge factory
func NewFactory() interfaces.PTYBridgeFactory {
	return &Factory{}
}

// NewPTYBridge creates a new PTY bridge instance
func (f *Factory) NewPTYBridge(ctx context.Context) (interfaces.PTYBridge, error) {
	return New(ctx)
}

// NewPTYBridge is a convenience function that creates a PTY bridge directly
func NewPTYBridge(ctx context.Context) (interfaces.PTYBridge, error) {
	return New(ctx)
}
