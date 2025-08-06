package interfaces

// ============================================================================
// IMPORTS
// ============================================================================

import (
	"context"
	"io"
	"net/http"
)

// ============================================================================
// PTY INTERFACES
// ============================================================================

// PTYReader defines the interface for reading from a PTY
type PTYReader interface {
	Read(ctx context.Context, b []byte) (int, error)
}

// PTYWriter defines the interface for writing to a PTY
type PTYWriter interface {
	Write(ctx context.Context, b []byte) (int, error)
}

// PTYResizer defines the interface for resizing a PTY
type PTYResizer interface {
	Resize(rows, cols int) error
}

// PTYInputProcessor defines the interface for processing input messages
type PTYInputProcessor interface {
	ProcessInput(ctx context.Context, data []byte) error
}

// PTYLifecycle defines the interface for PTY lifecycle management
type PTYLifecycle interface {
	Close() error
	Done() <-chan struct{}
}

// PTYManager combines all PTY operations into a single interface
type PTYManager interface {
	PTYReader
	PTYWriter
	PTYResizer
	PTYInputProcessor
	PTYLifecycle
}

// PTYCopier defines the interface for copying PTY data to a writer
type PTYCopier interface {
	Copy(dst io.Writer)
}

// PTYBridge combines PTY management with copying capabilities
type PTYBridge interface {
	PTYManager
	PTYCopier
}

// ============================================================================
// WEBSOCKET INTERFACES
// ============================================================================

// WebSocketHandler defines the interface for handling WebSocket connections
type WebSocketHandler interface {
	HandleWS(appCtx context.Context, w http.ResponseWriter, r *http.Request)
}

// WebSocketUpgrader defines the interface for upgrading HTTP connections to WebSocket
type WebSocketUpgrader interface {
	Upgrade(w http.ResponseWriter, r *http.Request, responseHeader http.Header) (WebSocketConnection, error)
}

// WebSocketConnection defines the interface for WebSocket connection operations
type WebSocketConnection interface {
	ReadMessage() (messageType int, p []byte, err error)
	WriteMessage(messageType int, data []byte) error
	SetReadLimit(limit int64)
	SetReadDeadline(t interface{}) error
	SetWriteDeadline(t interface{}) error
	SetPongHandler(h func(appData string) error)
	Close() error
}

// ============================================================================
// SERVER INTERFACES
// ============================================================================

// ServerManager defines the interface for server lifecycle management
type ServerManager interface {
	Start(ctx context.Context, address string) error
	Stop(ctx context.Context) error
}

// AddressParser defines the interface for parsing server addresses
type AddressParser interface {
	ParseAddress(address string) (host string, port int, err error)
}

// ProcessManager defines the interface for process management operations
type ProcessManager interface {
	CheckTmuxInstalled() bool
	CheckSessionExists(sessionName string) bool
	FindAndKillProcess() error
	StopServer(pidFilePath string) error
}

// PIDFileManager defines the interface for PID file operations
type PIDFileManager interface {
	WritePIDFile(pidFilePath string, pid int) error
	ReadPIDFile(pidFilePath string) (int, error)
	RemovePIDFile(pidFilePath string) error
}

// TmuxSessionManager defines the interface for tmux session management
type TmuxSessionManager interface {
	CleanupTmuxSessions(ctx context.Context) error
}

// HTTPServerManager defines the interface for HTTP server operations
type HTTPServerManager interface {
	CreateServer(address string, handler http.Handler) HTTPServer
}

// HTTPServer defines the interface for HTTP server lifecycle
type HTTPServer interface {
	ListenAndServe() error
	Shutdown(ctx context.Context) error
}

// ============================================================================
// FACTORY INTERFACES
// ============================================================================

// PTYBridgeFactory defines the interface for creating PTY bridges
type PTYBridgeFactory interface {
	NewPTYBridge(ctx context.Context) (PTYBridge, error)
}

// WebSocketHandlerFactory defines the interface for creating WebSocket handlers
type WebSocketHandlerFactory interface {
	NewWebSocketHandler(ptyFactory PTYBridgeFactory) WebSocketHandler
}

// ServerFactory defines the interface for creating server components
type ServerFactory interface {
	NewServerManager(
		addressParser AddressParser,
		processManager ProcessManager,
		pidFileManager PIDFileManager,
		tmuxManager TmuxSessionManager,
		httpManager HTTPServerManager,
		wsHandler WebSocketHandler,
	) ServerManager
}

// ============================================================================
// CONFIGURATION INTERFACES
// ============================================================================

// ConfigProvider defines the interface for configuration access
type ConfigProvider interface {
	GetServerConfig() ServerConfig
	GetTerminalConfig() TerminalConfig
	GetWebSocketConfig() WebSocketConfig
}

// ServerConfig defines server configuration interface
type ServerConfig interface {
	GetDefaultAddress() string
	GetSessionName() string
	GetPidFileName() string
	GetVersion() string
	GetShutdownTimeout() interface{}
	GetPTYOperationTimeout() interface{}
	GetTmuxCleanupTimeout() interface{}
}

// TerminalConfig defines terminal configuration interface
type TerminalConfig interface {
	GetDefaultRows() int
	GetDefaultCols() int
	GetDefaultTerm() string
	GetDefaultColor() string
}

// WebSocketConfig defines WebSocket configuration interface
type WebSocketConfig interface {
	GetWriteWait() interface{}
	GetPongWait() interface{}
	GetPingPeriod() interface{}
	GetMaxMessageSize() int64
	GetMessageChannelBuffer() int
	GetReadBufferSize() int
	GetWriteBufferSize() int
	GetErrorRetryDelay() interface{}
}
