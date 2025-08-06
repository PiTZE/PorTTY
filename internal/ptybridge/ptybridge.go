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

var cfg = config.Default

var sessionMutex sync.Mutex

// ============================================================================
// TYPE DEFINITIONS
// ============================================================================

type PTYBridge struct {
	cmd         *exec.Cmd
	pty         *os.File
	done        chan struct{}
	sessionName string
	ctx         context.Context
	cancel      context.CancelFunc
}

type ResizeMessage struct {
	Type       string     `json:"type"`
	Dimensions Dimensions `json:"dimensions"`
}

type Dimensions struct {
	Cols int `json:"cols"`
	Rows int `json:"rows"`
}

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

func checkSessionExists(sessionName string) bool {
	cmd := exec.Command("tmux", "has-session", "-t", sessionName)
	err := cmd.Run()
	return err == nil
}

// ============================================================================
// CORE BUSINESS LOGIC
// ============================================================================

func New(parentCtx context.Context) (*PTYBridge, error) {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()

	ctx, cancel := context.WithCancel(parentCtx)

	var cmd *exec.Cmd
	var ptmx *os.File
	var err error
	var sessionName string

	if cfg.Server.UseTmux {
		sessionExists := checkSessionExists(cfg.Server.SessionName)
		sessionName = cfg.Server.SessionName

		if sessionExists {
			logger.PTYBridgeLogger.Info("Attaching to existing tmux session", logger.String("session", cfg.Server.SessionName))
			cmd = exec.CommandContext(ctx, "tmux", "attach-session", "-t", cfg.Server.SessionName)
		} else {
			logger.PTYBridgeLogger.Info("Creating new tmux session", logger.String("session", cfg.Server.SessionName))

			killCmd := exec.CommandContext(ctx, "tmux", "kill-session", "-t", cfg.Server.SessionName)
			killCmd.Run()

			cmd = exec.CommandContext(ctx, "tmux", "new-session", "-s", cfg.Server.SessionName)
		}
	} else {
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

	go bridge.monitorContext()

	return bridge, nil
}

func (p *PTYBridge) monitorContext() {
	<-p.ctx.Done()
	logger.PTYBridgeLogger.Info("PTY bridge context cancelled, initiating shutdown")
	p.Close()
}

func (p *PTYBridge) Read(ctx context.Context, b []byte) (int, error) {
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-p.ctx.Done():
		return 0, p.ctx.Err()
	default:
	}

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

func (p *PTYBridge) Write(ctx context.Context, b []byte) (int, error) {
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-p.ctx.Done():
		return 0, p.ctx.Err()
	default:
	}

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

func (p *PTYBridge) ProcessInput(ctx context.Context, data []byte) error {
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

func (p *PTYBridge) Resize(rows, cols int) error {
	return pty.Setsize(p.pty, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
}

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

	if p.cancel != nil {
		p.cancel()
	}

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

func (p *PTYBridge) Done() <-chan struct{} {
	return p.done
}

func (p *PTYBridge) Copy(dst io.Writer) {
	io.Copy(dst, p.pty)
}

// ============================================================================
// INTERFACE COMPLIANCE CHECKS
// ============================================================================

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

type Factory struct{}

func NewFactory() interfaces.PTYBridgeFactory {
	return &Factory{}
}

func (f *Factory) NewPTYBridge(ctx context.Context) (interfaces.PTYBridge, error) {
	return New(ctx)
}

func NewPTYBridge(ctx context.Context) (interfaces.PTYBridge, error) {
	return New(ctx)
}
