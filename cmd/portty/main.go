package main

// ============================================================================
// IMPORTS
// ============================================================================

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/PiTZE/PorTTY/internal/config"
	"github.com/PiTZE/PorTTY/internal/interfaces"
	"github.com/PiTZE/PorTTY/internal/logger"
	"github.com/PiTZE/PorTTY/internal/ptybridge"
	"github.com/PiTZE/PorTTY/internal/websocket"
)

// ============================================================================
// CONSTANTS AND GLOBAL VARIABLES
// ============================================================================

//go:embed assets
var webContent embed.FS

// Configuration instance
var cfg = config.Default

// ============================================================================
// TYPE DEFINITIONS
// ============================================================================

// ServerManager implements the ServerManager interface
type ServerManager struct {
	addressParser  interfaces.AddressParser
	processManager interfaces.ProcessManager
	pidFileManager interfaces.PIDFileManager
	tmuxManager    interfaces.TmuxSessionManager
	httpManager    interfaces.HTTPServerManager
	wsHandler      interfaces.WebSocketHandler
}

// AddressParser implements the AddressParser interface
type AddressParser struct{}

// ProcessManager implements the ProcessManager interface
type ProcessManager struct{}

// PIDFileManager implements the PIDFileManager interface
type PIDFileManager struct{}

// TmuxSessionManager implements the TmuxSessionManager interface
type TmuxSessionManager struct{}

// HTTPServerManager implements the HTTPServerManager interface
type HTTPServerManager struct{}

// HTTPServerWrapper wraps the standard http.Server to implement HTTPServer interface
type HTTPServerWrapper struct {
	server *http.Server
}

// ============================================================================
// INTERFACE IMPLEMENTATIONS
// ============================================================================

// AddressParser implementations
func (ap *AddressParser) ParseAddress(address string) (string, int, error) {
	return parseAddress(address)
}

// ProcessManager implementations
func (pm *ProcessManager) CheckTmuxInstalled() bool {
	return checkTmuxInstalled()
}

func (pm *ProcessManager) CheckSessionExists(sessionName string) bool {
	return checkSessionExists(sessionName)
}

func (pm *ProcessManager) FindAndKillProcess() error {
	findAndKillProcess()
	return nil
}

func (pm *ProcessManager) StopServer(pidFilePath string) error {
	stopServer(pidFilePath)
	return nil
}

// PIDFileManager implementations
func (pfm *PIDFileManager) WritePIDFile(pidFilePath string, pid int) error {
	return os.WriteFile(pidFilePath, []byte(strconv.Itoa(pid)), cfg.Server.PidFilePermissions)
}

func (pfm *PIDFileManager) ReadPIDFile(pidFilePath string) (int, error) {
	pidBytes, err := os.ReadFile(pidFilePath)
	if err != nil {
		return 0, err
	}
	pidStr := strings.TrimSpace(string(pidBytes))
	return strconv.Atoi(pidStr)
}

func (pfm *PIDFileManager) RemovePIDFile(pidFilePath string) error {
	return os.Remove(pidFilePath)
}

// TmuxSessionManager implementations
func (tsm *TmuxSessionManager) CleanupTmuxSessions(ctx context.Context) error {
	cleanupTmuxSessions(ctx)
	return nil
}

// HTTPServerManager implementations
func (hsm *HTTPServerManager) CreateServer(address string, handler http.Handler) interfaces.HTTPServer {
	server := &http.Server{
		Addr:    address,
		Handler: handler,
	}
	return &HTTPServerWrapper{server: server}
}

// HTTPServerWrapper implementations
func (hsw *HTTPServerWrapper) ListenAndServe() error {
	return hsw.server.ListenAndServe()
}

func (hsw *HTTPServerWrapper) Shutdown(ctx context.Context) error {
	return hsw.server.Shutdown(ctx)
}

// ServerManager implementations
func (sm *ServerManager) Start(ctx context.Context, address string) error {
	// Use the injected dependencies instead of calling functions directly
	if !sm.processManager.CheckTmuxInstalled() {
		return fmt.Errorf("tmux is not installed. Please install tmux to use PorTTY")
	}

	host, port, err := sm.addressParser.ParseAddress(address)
	if err != nil {
		return fmt.Errorf("failed to parse server address: %w", err)
	}

	if sm.processManager.CheckSessionExists(cfg.Server.SessionName) {
		logger.ServerLogger.Info("Found existing tmux session", logger.String("session", cfg.Server.SessionName))
	}

	// Get PID file path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = cfg.Server.FallbackTempDir
	}
	pidFilePath := filepath.Join(homeDir, cfg.Server.PidFileName)

	pid := os.Getpid()
	if err := sm.pidFileManager.WritePIDFile(pidFilePath, pid); err != nil {
		logger.ServerLogger.Warn("failed to write PID file", logger.String("path", pidFilePath), logger.Error(err))
	}

	// Create application-level context for coordinated shutdown
	appCtx, appCancel := context.WithCancel(ctx)
	defer appCancel()

	mux := http.NewServeMux()

	// Pass application context to WebSocket handler
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		sm.wsHandler.HandleWS(appCtx, w, r)
	})

	webFS, err := fs.Sub(webContent, "assets")
	if err != nil {
		return fmt.Errorf("failed to create sub-filesystem for embedded assets: %w", err)
	}

	fileServer := http.FileServer(http.FS(webFS))
	mux.Handle("/", fileServer)

	bindAddr := fmt.Sprintf("%s:%d", host, port)
	server := sm.httpManager.CreateServer(bindAddr, mux)

	// Start HTTP server in goroutine
	serverErrChan := make(chan error, 1)
	go func() {
		logger.ServerLogger.Info("Starting PorTTY", logger.String("url", "http://"+bindAddr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrChan <- err
		}
	}()

	// Setup signal handling for graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Wait for shutdown signal or server error
	select {
	case <-stop:
		logger.ServerLogger.Info("Received shutdown signal")
	case err := <-serverErrChan:
		return fmt.Errorf("HTTP server failed: %w", err)
	case <-appCtx.Done():
		logger.ServerLogger.Info("Application context cancelled")
	}

	// Begin coordinated shutdown sequence
	logger.ServerLogger.Info("Beginning graceful shutdown")

	// Cancel application context to signal all components to shutdown
	appCancel()

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer shutdownCancel()

	// Shutdown HTTP server
	logger.ServerLogger.Info("Shutting down HTTP server")
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.ServerLogger.Error("failed to gracefully shutdown HTTP server", err)
	}

	// Clean up PID file
	if err := sm.pidFileManager.RemovePIDFile(pidFilePath); err != nil && !os.IsNotExist(err) {
		logger.ServerLogger.Warn("failed to remove PID file", logger.String("path", pidFilePath), logger.Error(err))
	}

	// Clean up tmux sessions
	logger.ServerLogger.Info("Cleaning up tmux sessions")
	if err := sm.tmuxManager.CleanupTmuxSessions(shutdownCtx); err != nil {
		logger.ServerLogger.Error("failed to cleanup tmux sessions", err)
	}

	logger.ServerLogger.Info("Server gracefully stopped")
	return nil
}

func (sm *ServerManager) Stop(ctx context.Context) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = cfg.Server.FallbackTempDir
	}
	pidFilePath := filepath.Join(homeDir, cfg.Server.PidFileName)

	return sm.processManager.StopServer(pidFilePath)
}

// ============================================================================
// FACTORY FUNCTIONS
// ============================================================================

// NewServerManager creates a new server manager with dependency injection
func NewServerManager() interfaces.ServerManager {
	// Create PTY factory
	ptyFactory := ptybridge.NewFactory()

	// Create WebSocket handler with PTY factory
	wsHandler := websocket.NewHandler(ptyFactory)

	return &ServerManager{
		addressParser:  &AddressParser{},
		processManager: &ProcessManager{},
		pidFileManager: &PIDFileManager{},
		tmuxManager:    &TmuxSessionManager{},
		httpManager:    &HTTPServerManager{},
		wsHandler:      wsHandler,
	}
}

// ============================================================================
// INTERFACE COMPLIANCE CHECKS
// ============================================================================

// Compile-time interface compliance checks
var (
	_ interfaces.ServerManager      = (*ServerManager)(nil)
	_ interfaces.AddressParser      = (*AddressParser)(nil)
	_ interfaces.ProcessManager     = (*ProcessManager)(nil)
	_ interfaces.PIDFileManager     = (*PIDFileManager)(nil)
	_ interfaces.TmuxSessionManager = (*TmuxSessionManager)(nil)
	_ interfaces.HTTPServerManager  = (*HTTPServerManager)(nil)
	_ interfaces.HTTPServer         = (*HTTPServerWrapper)(nil)
)

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

// parseAddress parses the address and returns host and port
func parseAddress(address string) (string, int, error) {
	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return "", 0, fmt.Errorf("failed to parse address %q: %w", address, err)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, fmt.Errorf("failed to parse port %q in address %q: %w", portStr, address, err)
	}

	if host == "" {
		host = "localhost"
	}

	return host, port, nil
}

// checkTmuxInstalled checks if tmux is installed
func checkTmuxInstalled() bool {
	_, err := exec.LookPath("tmux")
	return err == nil
}

// checkSessionExists checks if a tmux session exists
func checkSessionExists(sessionName string) bool {
	cmd := exec.Command("tmux", "has-session", "-t", sessionName)
	err := cmd.Run()
	return err == nil
}

// findAndKillProcess tries to find and kill the PorTTY process by name
func findAndKillProcess() {
	logger.ServerLogger.Info("Trying to find PorTTY process by name")

	cmd := exec.Command("bash", "-c", "pgrep -f 'portty run'")
	output, err := cmd.Output()
	if err != nil {
		logger.ServerLogger.Info("No PorTTY process found")
		return
	}

	pids := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, pidStr := range pids {
		if pidStr == "" {
			continue
		}

		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}

		process, err := os.FindProcess(pid)
		if err != nil {
			continue
		}

		if err := process.Signal(syscall.SIGTERM); err != nil {
			logger.ServerLogger.Error("failed to send SIGTERM signal to process", err, logger.Int("pid", pid))
			continue
		}

		logger.ServerLogger.Info("Sent termination signal to PorTTY", logger.Int("pid", pid))
	}
}

// ============================================================================
// HELP AND VERSION FUNCTIONS
// ============================================================================

// showRunHelp displays help for the run command
func showRunHelp() {
	programName := filepath.Base(os.Args[0])

	fmt.Println("PorTTY - Run Command")
	fmt.Println("Start the PorTTY server")
	fmt.Println()

	fmt.Println("USAGE:")
	fmt.Printf("  %s run [OPTIONS] [address]\n", programName)
	fmt.Println()

	fmt.Println("OPTIONS:")
	fmt.Println("  -h, --help            Show this help message")
	fmt.Println("  -a, --address ADDR    Specify the address to bind to (format: [host]:[port])")
	fmt.Println("  -p, --port PORT       Specify the port to listen on")
	fmt.Println()

	fmt.Println("ARGUMENTS:")
	fmt.Println("  address               Address to bind to (format: [host]:[port])")
	fmt.Printf("                        Default: %s\n", cfg.Server.DefaultAddress)
	fmt.Println()

	fmt.Println("EXAMPLES:")
	fmt.Printf("  %s run                       # Start on localhost:7314\n", programName)
	fmt.Printf("  %s run :7314                 # Start on all interfaces, port 7314\n", programName)
	fmt.Printf("  %s run 0.0.0.0:7314          # Start on all interfaces, port 7314\n", programName)
	fmt.Printf("  %s run -p 7314               # Start on localhost, port 7314\n", programName)
	fmt.Printf("  %s run -a 0.0.0.0 -p 7314    # Start on all interfaces, port 7314\n", programName)
	fmt.Printf("  %s run --address localhost --port 7314  # Start on localhost, port 7314\n", programName)
}

// showStopHelp displays help for the stop command
func showStopHelp() {
	programName := filepath.Base(os.Args[0])

	fmt.Println("PorTTY - Stop Command")
	fmt.Println("Stop the running PorTTY server")
	fmt.Println()

	fmt.Println("USAGE:")
	fmt.Printf("  %s stop\n", programName)
	fmt.Println()

	fmt.Println("DESCRIPTION:")
	fmt.Println("  This command will gracefully stop the running PorTTY server.")
	fmt.Println("  It first tries to find the server using the PID file, and if that fails,")
	fmt.Println("  it attempts to find the process by name.")
}

// showVersion displays version information
func showVersion() {
	fmt.Printf("PorTTY %s\n", cfg.Server.Version)
	fmt.Println("A lightweight, web-based terminal emulator powered by tmux")
	fmt.Println("https://github.com/PiTZE/PorTTY")
}

// showHelp displays usage information
func showHelp() {
	programName := filepath.Base(os.Args[0])
	version := cfg.Server.Version

	fmt.Printf("PorTTY %s - Web-based Terminal\n", version)
	fmt.Println("A lightweight, web-based terminal emulator powered by tmux")
	fmt.Println()

	fmt.Println("USAGE:")
	fmt.Printf("  %s [OPTIONS] COMMAND [ARGS]\n", programName)
	fmt.Println()

	fmt.Println("OPTIONS:")
	fmt.Println("  -h, --help     Show this help message and exit")
	fmt.Println()

	fmt.Println("COMMANDS:")
	fmt.Println("  run [address]  Start the server (default: localhost:7314)")
	fmt.Println("  stop           Stop the server")
	fmt.Println("  help           Show this help message")
	fmt.Println()

	fmt.Println("RUN OPTIONS:")
	fmt.Println("  address        Address to bind to (format: [host]:[port])")
	fmt.Printf("                 Default: %s\n", cfg.Server.DefaultAddress)
	fmt.Println()

	fmt.Println("EXAMPLES:")
	fmt.Printf("  %s run                    # Start on localhost:7314\n", programName)
	fmt.Printf("  %s run :7314              # Start on all interfaces, port 7314\n", programName)
	fmt.Printf("  %s run 0.0.0.0:7314       # Start on all interfaces, port 7314\n", programName)
	fmt.Printf("  %s run localhost:7314     # Start on localhost, port 7314\n", programName)
	fmt.Printf("  %s stop                   # Stop the server\n", programName)
	fmt.Printf("  %s -h                     # Show this help message\n", programName)
	fmt.Println()

	fmt.Println("For more information, visit: https://github.com/PiTZE/PorTTY")
}

// ============================================================================
// CORE BUSINESS LOGIC
// ============================================================================

// runServer starts the PorTTY server using the new interface-based architecture
func runServer(address string) {
	// Create server manager with dependency injection
	serverManager := NewServerManager()

	// Start the server using the interface-based approach
	ctx := context.Background()
	if err := serverManager.Start(ctx, address); err != nil {
		logger.ServerLogger.Fatal("failed to start server", err, logger.String("address", address))
	}
}

// cleanupTmuxSessions performs context-aware cleanup of tmux sessions
func cleanupTmuxSessions(ctx context.Context) {
	// Create timeout context for tmux cleanup operations
	cleanupCtx, cleanupCancel := context.WithTimeout(ctx, cfg.Server.TmuxCleanupTimeout)
	defer cleanupCancel()

	// Check if main session exists before attempting to kill it
	checkCmd := exec.CommandContext(cleanupCtx, "tmux", "has-session", "-t", cfg.Server.SessionName)
	if err := checkCmd.Run(); err != nil {
		if err == context.DeadlineExceeded {
			logger.ServerLogger.Warn("tmux session check timed out", logger.String("session", cfg.Server.SessionName))
		} else {
			logger.ServerLogger.Info("tmux session does not exist, skipping cleanup", logger.String("session", cfg.Server.SessionName))
		}
	} else {
		// Session exists, kill it
		killCmd := exec.CommandContext(cleanupCtx, "tmux", "kill-session", "-t", cfg.Server.SessionName)
		if err := killCmd.Run(); err != nil {
			if err == context.DeadlineExceeded {
				logger.ServerLogger.Warn("tmux session kill timed out", logger.String("session", cfg.Server.SessionName))
			} else {
				logger.ServerLogger.Error("failed to kill tmux session", err, logger.String("session", cfg.Server.SessionName))
			}
		} else {
			logger.ServerLogger.Info("successfully killed tmux session", logger.String("session", cfg.Server.SessionName))
		}
	}

	// Clean up any orphaned PorTTY sessions
	cleanupCmd := exec.CommandContext(cleanupCtx, "bash", "-c", "tmux list-sessions -F '#{session_name}' 2>/dev/null | grep '^PorTTY-' | xargs -I{} tmux kill-session -t {} 2>/dev/null || true")
	if err := cleanupCmd.Run(); err != nil {
		if err == context.DeadlineExceeded {
			logger.ServerLogger.Warn("tmux cleanup timed out")
		} else {
			logger.ServerLogger.Warn("failed to cleanup orphaned tmux sessions", logger.Error(err))
		}
	}
}

// stopServer stops the PorTTY server
func stopServer(pidFilePath string) {
	pidBytes, err := os.ReadFile(pidFilePath)
	if err != nil {
		findAndKillProcess()
		return
	}

	pidStr := strings.TrimSpace(string(pidBytes))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		logger.ServerLogger.Error("invalid PID in file", err, logger.String("pid", pidStr), logger.String("file", pidFilePath))
		findAndKillProcess()
		return
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		logger.ServerLogger.Error("failed to find process", err, logger.Int("pid", pid))
		findAndKillProcess()
		return
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		logger.ServerLogger.Error("failed to send SIGTERM signal to process", err, logger.Int("pid", pid))
		findAndKillProcess()
		return
	}

	logger.ServerLogger.Info("Sent termination signal to PorTTY", logger.Int("pid", pid))

	os.Remove(pidFilePath)
}

// ============================================================================
// MAIN EXECUTION LOGIC
// ============================================================================

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = cfg.Server.FallbackTempDir
	}
	pidFilePath := filepath.Join(homeDir, cfg.Server.PidFileName)

	for _, arg := range os.Args {
		if arg == "-h" || arg == "--help" {
			showHelp()
			return
		}
		if arg == "-v" || arg == "--version" {
			showVersion()
			return
		}
	}

	if len(os.Args) < 2 {
		showHelp()
		return
	}

	command := os.Args[1]

	switch command {
	case "run":
		address := cfg.Server.DefaultAddress

		args := os.Args[2:]
		for i := 0; i < len(args); i++ {
			arg := args[i]

			switch arg {
			case "-h", "--help":
				showRunHelp()
				return
			case "-a", "--address":
				if i+1 < len(args) {
					address = args[i+1]
					i++
				}
			case "-p", "--port":
				if i+1 < len(args) {
					port := args[i+1]
					host, _, err := net.SplitHostPort(address)
					if err != nil {
						host = "localhost"
					}
					if host == "" {
						host = "localhost"
					}
					address = fmt.Sprintf("%s:%s", host, port)
					i++
				}
			default:
				if !strings.HasPrefix(arg, "-") {
					address = arg
				}
			}
		}

		runServer(address)
	case "stop":
		if len(os.Args) > 2 && (os.Args[2] == "-h" || os.Args[2] == "--help") {
			showStopHelp()
			return
		}
		stopServer(pidFilePath)
	case "help":
		if len(os.Args) > 2 {
			switch os.Args[2] {
			case "run":
				showRunHelp()
			case "stop":
				showStopHelp()
			default:
				fmt.Printf("Unknown command: %s\n\n", os.Args[2])
				showHelp()
			}
		} else {
			showHelp()
		}
	case "version":
		showVersion()
	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		showHelp()
		os.Exit(1)
	}
}
