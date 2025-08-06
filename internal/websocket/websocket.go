package websocket

// ============================================================================
// IMPORTS
// ============================================================================

import (
	"context"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/PiTZE/PorTTY/internal/config"
	"github.com/PiTZE/PorTTY/internal/interfaces"
	"github.com/PiTZE/PorTTY/internal/logger"
	"github.com/PiTZE/PorTTY/internal/ptybridge"
	"github.com/gorilla/websocket"
)

// ============================================================================
// CONSTANTS AND GLOBAL VARIABLES
// ============================================================================

// Configuration instance
var cfg = config.Default

var upgrader = websocket.Upgrader{
	ReadBufferSize:  int(cfg.WebSocket.ReadBufferSize),
	WriteBufferSize: int(cfg.WebSocket.WriteBufferSize),
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// ============================================================================
// TYPE DEFINITIONS
// ============================================================================

// Handler implements the WebSocketHandler interface
type Handler struct {
	ptyFactory interfaces.PTYBridgeFactory
	upgrader   *websocket.Upgrader
}

// ============================================================================
// CORE BUSINESS LOGIC
// ============================================================================

// NewHandler creates a new WebSocket handler with dependency injection
func NewHandler(ptyFactory interfaces.PTYBridgeFactory) interfaces.WebSocketHandler {
	return &Handler{
		ptyFactory: ptyFactory,
		upgrader: &websocket.Upgrader{
			ReadBufferSize:  int(cfg.WebSocket.ReadBufferSize),
			WriteBufferSize: int(cfg.WebSocket.WriteBufferSize),
			CheckOrigin:     func(r *http.Request) bool { return true },
		},
	}
}

// HandleWS handles WebSocket connections with application context coordination
func (h *Handler) HandleWS(appCtx context.Context, w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.WebSocketLogger.Error("failed to upgrade connection to WebSocket", err)
		return
	}

	var wg sync.WaitGroup
	wg.Add(3)

	// Create connection context that respects application shutdown
	ctx, cancel := context.WithCancel(appCtx)
	defer cancel()

	ptyBridge, err := h.ptyFactory.NewPTYBridge(ctx)
	if err != nil {
		logger.WebSocketLogger.Error("failed to create PTY bridge", err)
		conn.Close()
		return
	}

	messageChan := make(chan []byte, cfg.WebSocket.MessageChannelBuffer)

	conn.SetReadLimit(cfg.WebSocket.MaxMessageSize)
	conn.SetReadDeadline(time.Now().Add(cfg.WebSocket.PongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(cfg.WebSocket.PongWait))
		return nil
	})

	// Goroutine 1: Read messages from WebSocket
	go func() {
		defer wg.Done()
		defer cancel()
		defer close(messageChan)

		for {
			select {
			case <-ctx.Done():
				logger.WebSocketLogger.Info("WebSocket reader shutting down due to context cancellation")
				return
			default:
				conn.SetReadDeadline(time.Now().Add(cfg.WebSocket.PongWait))

				messageType, message, err := conn.ReadMessage()
				if err != nil {
					if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
						logger.WebSocketLogger.Error("unexpected WebSocket read error", err)
					}
					return
				}

				if messageType == websocket.TextMessage || messageType == websocket.BinaryMessage {
					select {
					case messageChan <- message:
					case <-ctx.Done():
						return
					default:
						logger.WebSocketLogger.Warn("message channel full, dropping message")
					}
				}
			}
		}
	}()

	// Goroutine 2: Process messages from channel to PTY
	go func() {
		defer wg.Done()
		defer ptyBridge.Close()

		for {
			select {
			case <-ctx.Done():
				logger.WebSocketLogger.Info("WebSocket message processor shutting down due to context cancellation")
				return
			case message, ok := <-messageChan:
				if !ok {
					return
				}

				if err := ptyBridge.ProcessInput(ctx, message); err != nil {
					if err == io.EOF || err == io.ErrClosedPipe {
						logger.WebSocketLogger.Error("fatal error processing input", err)
						return
					}
					if err == context.Canceled || err == context.DeadlineExceeded {
						logger.WebSocketLogger.Info("PTY input processing cancelled")
						return
					}
				}
			}
		}
	}()

	// Goroutine 3: Read from PTY and write to WebSocket
	go func() {
		defer wg.Done()
		defer cancel()
		defer conn.Close()

		buf := make([]byte, cfg.WebSocket.MaxMessageSize)

		for {
			select {
			case <-ctx.Done():
				logger.WebSocketLogger.Info("WebSocket writer shutting down due to context cancellation")
				return
			default:
				n, err := ptyBridge.Read(ctx, buf)

				if err != nil {
					if err == io.EOF {
						return
					}

					if err == io.ErrClosedPipe || err == io.ErrUnexpectedEOF {
						return
					}

					if err == context.Canceled || err == context.DeadlineExceeded {
						logger.WebSocketLogger.Info("PTY read cancelled")
						return
					}

					select {
					case <-time.After(cfg.WebSocket.ErrorRetryDelay):
						continue
					case <-ctx.Done():
						return
					}
				}

				if n > 0 {
					conn.SetWriteDeadline(time.Now().Add(cfg.WebSocket.WriteWait))
					if err := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
						if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
							return
						}

						select {
						case <-time.After(cfg.WebSocket.ErrorRetryDelay):
							continue
						case <-ctx.Done():
							return
						}
					}
				}
			}
		}
	}()

	// Wait for connection to close or context cancellation
	select {
	case <-ptyBridge.Done():
		logger.WebSocketLogger.Info("PTY bridge closed, terminating WebSocket connection")
	case <-ctx.Done():
		logger.WebSocketLogger.Info("Context cancelled, terminating WebSocket connection")
	}

	// Cancel connection context and wait for all goroutines to finish
	cancel()

	// Wait for all goroutines to complete with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.WebSocketLogger.Info("All WebSocket goroutines completed")
	case <-time.After(cfg.WebSocket.WriteWait):
		logger.WebSocketLogger.Warn("Timeout waiting for WebSocket goroutines to complete")
	}

	// Final cleanup
	conn.Close()
	ptyBridge.Close()
}

// ============================================================================
// INTERFACE COMPLIANCE CHECKS
// ============================================================================

// Compile-time interface compliance checks
var (
	_ interfaces.WebSocketHandler = (*Handler)(nil)
)

// ============================================================================
// FACTORY FUNCTIONS
// ============================================================================

// Factory implements the WebSocketHandlerFactory interface
type Factory struct{}

// NewFactory creates a new WebSocket handler factory
func NewFactory() interfaces.WebSocketHandlerFactory {
	return &Factory{}
}

// NewWebSocketHandler creates a new WebSocket handler instance
func (f *Factory) NewWebSocketHandler(ptyFactory interfaces.PTYBridgeFactory) interfaces.WebSocketHandler {
	return NewHandler(ptyFactory)
}

// ============================================================================
// BACKWARD COMPATIBILITY
// ============================================================================

// HandleWS provides backward compatibility for the original function signature
func HandleWS(appCtx context.Context, w http.ResponseWriter, r *http.Request) {
	// Create default PTY factory for backward compatibility
	ptyFactory := &defaultPTYFactory{}
	handler := NewHandler(ptyFactory)
	handler.HandleWS(appCtx, w, r)
}

// defaultPTYFactory provides a default implementation for backward compatibility
type defaultPTYFactory struct{}

// NewPTYBridge creates a PTY bridge using the original ptybridge.New function
func (f *defaultPTYFactory) NewPTYBridge(ctx context.Context) (interfaces.PTYBridge, error) {
	// Import ptybridge to use the original New function
	// This maintains backward compatibility
	return ptybridge.New(ctx)
}
