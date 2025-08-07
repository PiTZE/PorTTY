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

var cfg = config.Default

const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[0;31m"
	ColorGreen  = "\033[0;32m"
	ColorYellow = "\033[1;33m"
	ColorBlue   = "\033[0;34m"
	ColorBold   = "\033[1m"
)

// ============================================================================
// TYPE DEFINITIONS
// ============================================================================

type ServerManager struct {
	addressParser  interfaces.AddressParser
	processManager interfaces.ProcessManager
	pidFileManager interfaces.PIDFileManager
	tmuxManager    interfaces.TmuxSessionManager
	httpManager    interfaces.HTTPServerManager
	wsHandler      interfaces.WebSocketHandler
}

type AddressParser struct{}

type ProcessManager struct{}

type PIDFileManager struct{}

type TmuxSessionManager struct{}

type HTTPServerManager struct{}
type HTTPServerWrapper struct {
	server *http.Server
}

// ============================================================================
// INTERFACE IMPLEMENTATIONS
// ============================================================================

func (ap *AddressParser) ParseAddress(address string) (string, int, error) {
	return parseAddress(address)
}
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

func (tsm *TmuxSessionManager) CleanupTmuxSessions(ctx context.Context) error {
	cleanupTmuxSessions(ctx)
	return nil
}

func (hsm *HTTPServerManager) CreateServer(address string, handler http.Handler) interfaces.HTTPServer {
	server := &http.Server{
		Addr:    address,
		Handler: handler,
	}
	return &HTTPServerWrapper{server: server}
}

func (hsw *HTTPServerWrapper) ListenAndServe() error {
	return hsw.server.ListenAndServe()
}

func (hsw *HTTPServerWrapper) Shutdown(ctx context.Context) error {
	return hsw.server.Shutdown(ctx)
}

func (sm *ServerManager) Start(ctx context.Context, address string) error {
	if cfg.Server.UseTmux {
		if !sm.processManager.CheckTmuxInstalled() {
			return fmt.Errorf("tmux is not installed. Please install tmux to use PorTTY with --tmux flag")
		}

		if sm.processManager.CheckSessionExists(cfg.Server.SessionName) {
			logger.ServerLogger.Info("Found existing tmux session", logger.String("session", cfg.Server.SessionName))
		}
	}

	host, port, err := sm.addressParser.ParseAddress(address)
	if err != nil {
		return fmt.Errorf("failed to parse server address: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = cfg.Server.FallbackTempDir
	}
	pidFilePath := filepath.Join(homeDir, cfg.Server.PidFileName)

	pid := os.Getpid()
	if err := sm.pidFileManager.WritePIDFile(pidFilePath, pid); err != nil {
		logger.ServerLogger.Warn("failed to write PID file", logger.String("path", pidFilePath), logger.Error(err))
	}

	appCtx, appCancel := context.WithCancel(ctx)
	defer appCancel()

	mux := http.NewServeMux()

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

	serverErrChan := make(chan error, 1)
	go func() {
		logger.ServerLogger.Info("Starting PorTTY", logger.String("url", "http://"+bindAddr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrChan <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case <-stop:
		logger.ServerLogger.Info("Received shutdown signal")
	case err := <-serverErrChan:
		return fmt.Errorf("HTTP server failed: %w", err)
	case <-appCtx.Done():
		logger.ServerLogger.Info("Application context cancelled")
	}

	logger.ServerLogger.Info("Beginning graceful shutdown")

	appCancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer shutdownCancel()

	logger.ServerLogger.Info("Shutting down HTTP server")
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.ServerLogger.Error("failed to gracefully shutdown HTTP server", err)
	}

	if err := sm.pidFileManager.RemovePIDFile(pidFilePath); err != nil && !os.IsNotExist(err) {
		logger.ServerLogger.Warn("failed to remove PID file", logger.String("path", pidFilePath), logger.Error(err))
	}

	if cfg.Server.UseTmux {
		logger.ServerLogger.Info("Cleaning up tmux sessions")
		if err := sm.tmuxManager.CleanupTmuxSessions(shutdownCtx); err != nil {
			logger.ServerLogger.Error("failed to cleanup tmux sessions", err)
		}
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
	ptyFactory := ptybridge.NewFactory()
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

func checkTmuxInstalled() bool {
	_, err := exec.LookPath("tmux")
	return err == nil
}

func checkSessionExists(sessionName string) bool {
	cmd := exec.Command("tmux", "has-session", "-t", sessionName)
	err := cmd.Run()
	return err == nil
}

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

func showRunHelp() {
	programName := filepath.Base(os.Args[0])

	fmt.Printf("PorTTY - Run Command\n")
	fmt.Printf("Start the PorTTY web-based terminal server\n")
	fmt.Printf("\n")

	fmt.Printf("USAGE:\n")
	fmt.Printf("  %s run [OPTIONS]\n", programName)
	fmt.Printf("\n")

	fmt.Printf("OPTIONS:\n")
	fmt.Printf("  -h, --help                 Show this help message and exit\n")
	fmt.Printf("  -a, --address ADDR         Specify the address to bind to (format: [host]:[port])\n")
	fmt.Printf("  -i, --interface INTERFACE  Specify the interface to bind to\n")
	fmt.Printf("                             Examples: localhost, 0.0.0.0, 127.0.0.1\n")
	fmt.Printf("  -p, --port PORT            Specify the port to listen on (1-65535)\n")
	fmt.Printf("                             Default: %s\n", func() string {
		_, port, _ := net.SplitHostPort(cfg.Server.DefaultAddress)
		return port
	}())
	fmt.Printf("  --tmux                     Enable tmux mode for session persistence\n")
	fmt.Printf("                             (default: direct shell mode)\n")
	fmt.Printf("  --verbose                  Enable verbose logging output\n")
	fmt.Printf("  --debug                    Enable debug logging output\n")
	fmt.Printf("\n")

	fmt.Printf("SHELL MODES:\n")
	fmt.Printf("  PorTTY v0.2+ supports dual shell modes:\n")
	fmt.Printf("  • Default Shell Mode: Direct shell access (primary mode)\n")
	fmt.Printf("    - Faster startup, no tmux overhead\n")
	fmt.Printf("    - Each connection is independent\n")
	fmt.Printf("    - Native terminal experience\n")
	fmt.Printf("  • tmux Mode: Session persistence and multi-client support (optional)\n")
	fmt.Printf("    - Sessions persist across connection closures\n")
	fmt.Printf("    - Multiple browsers can connect to same session\n")
	fmt.Printf("    - Requires tmux to be installed\n")
	fmt.Printf("\n")

	fmt.Printf("EXAMPLES:\n")
	fmt.Printf("  %s run                              # Start with direct shell on default address\n", programName)
	fmt.Printf("  %s run --tmux                       # Start with tmux on default address\n", programName)
	fmt.Printf("  %s run -i 0.0.0.0 -p 7314           # Start on all interfaces, port 7314\n", programName)
	fmt.Printf("  %s run -a 0.0.0.0:7314              # Start on all interfaces using address format\n", programName)
	fmt.Printf("  %s run -i localhost -p 8080         # Start on localhost, port 8080\n", programName)
	fmt.Printf("  %s run -a 0.0.0.0:7314 --tmux       # Start with tmux on all interfaces\n", programName)
	fmt.Printf("  %s run --interface 127.0.0.1 --port 9000 --verbose  # Verbose mode\n", programName)
	fmt.Printf("\n")

	fmt.Printf("NETWORK CONFIGURATION:\n")
	fmt.Printf("  Interface Options:\n")
	fmt.Printf("    localhost, 127.0.0.1    Local access only (secure)\n")
	fmt.Printf("    0.0.0.0                 All interfaces (requires firewall setup)\n")
	fmt.Printf("    [specific IP]           Bind to specific network interface\n")
	fmt.Printf("\n")
	fmt.Printf("  Port Considerations:\n")
	fmt.Printf("    1-1023                  Privileged ports (require root/sudo)\n")
	fmt.Printf("    1024-65535              User ports (recommended for non-root)\n")
	fmt.Printf("    7314                    Default PorTTY port\n")
	fmt.Printf("\n")

	fmt.Printf("SECURITY NOTES:\n")
	fmt.Printf("  • No built-in authentication - use reverse proxy for production\n")
	fmt.Printf("  • Binding to 0.0.0.0 exposes terminal to network - secure accordingly\n")
	fmt.Printf("  • Consider firewall rules when binding to public interfaces\n")
	fmt.Printf("\n")

	fmt.Printf("For more information, visit: https://github.com/PiTZE/PorTTY\n")
}

func showStopHelp() {
	programName := filepath.Base(os.Args[0])

	fmt.Printf("PorTTY - Stop Command\n")
	fmt.Printf("Gracefully stop the running PorTTY server\n")
	fmt.Printf("\n")

	fmt.Printf("USAGE:\n")
	fmt.Printf("  %s stop [OPTIONS]\n", programName)
	fmt.Printf("\n")

	fmt.Printf("OPTIONS:\n")
	fmt.Printf("  -h, --help     Show this help message and exit\n")
	fmt.Printf("  --verbose      Enable verbose logging during stop process\n")
	fmt.Printf("  --debug        Enable debug logging during stop process\n")
	fmt.Printf("\n")

	fmt.Printf("DESCRIPTION:\n")
	fmt.Printf("  This command gracefully stops the running PorTTY server using the following process:\n")
	fmt.Printf("  1. Attempts to read the PID from the PID file (~/.portty.pid)\n")
	fmt.Printf("  2. Sends SIGTERM signal to the process for graceful shutdown\n")
	fmt.Printf("  3. If PID file method fails, searches for process by name\n")
	fmt.Printf("  4. Cleans up PID file after successful termination\n")
	fmt.Printf("\n")

	fmt.Printf("SHUTDOWN PROCESS:\n")
	fmt.Printf("  • Graceful HTTP server shutdown with active connection handling\n")
	fmt.Printf("  • WebSocket connections are properly closed\n")
	fmt.Printf("  • tmux sessions are preserved (if using tmux mode)\n")
	fmt.Printf("  • PID file cleanup\n")
	fmt.Printf("  • Resource cleanup and memory deallocation\n")
	fmt.Printf("\n")

	fmt.Printf("EXAMPLES:\n")
	fmt.Printf("  %s stop                    # Stop the server normally\n", programName)
	fmt.Printf("  %s stop --verbose          # Stop with detailed output\n", programName)
	fmt.Printf("\n")

	fmt.Printf("TROUBLESHOOTING:\n")
	fmt.Printf("  If the stop command fails:\n")
	fmt.Printf("  • Check if the server is actually running: ps aux | grep portty\n")
	fmt.Printf("  • Verify PID file exists: ls -la ~/.portty.pid\n")
	fmt.Printf("  • Force kill if necessary: pkill -f 'portty run'\n")
	fmt.Printf("  • Check for orphaned tmux sessions: tmux list-sessions\n")
	fmt.Printf("\n")

	fmt.Printf("For more information, visit: https://github.com/PiTZE/PorTTY\n")
}

func showVersion() {
	fmt.Printf("PorTTY %s\n", cfg.Server.Version)
	fmt.Println("A lightweight, web-based terminal emulator with dual shell mode support")
	fmt.Println("https://github.com/PiTZE/PorTTY")
}

func showHelp() {
	programName := filepath.Base(os.Args[0])
	version := cfg.Server.Version

	fmt.Printf("PorTTY %s - Web-based Terminal Emulator\n", version)
	fmt.Printf("A lightweight, browser-based terminal with dual shell mode support\n")
	fmt.Printf("\n")

	fmt.Printf("USAGE:\n")
	fmt.Printf("  %s [GLOBAL_OPTIONS] COMMAND [COMMAND_OPTIONS]\n", programName)
	fmt.Printf("\n")

	fmt.Printf("GLOBAL OPTIONS:\n")
	fmt.Printf("  -h, --help                 Show this help message and exit\n")
	fmt.Printf("  -v, --version              Display version information and exit\n")
	fmt.Printf("\n")

	fmt.Printf("COMMANDS:\n")
	fmt.Printf("  run [options]              Start the PorTTY server\n")
	fmt.Printf("  stop [options]             Stop the running PorTTY server\n")
	fmt.Printf("  help [command]             Show help for specific command\n")
	fmt.Printf("  version                    Display version information\n")
	fmt.Printf("\n")

	fmt.Printf("SHELL MODES:\n")
	fmt.Printf("  PorTTY v0.2+ supports dual shell modes:\n")
	fmt.Printf("  • Default Shell Mode (Primary):\n")
	fmt.Printf("    - Direct shell access (zsh, bash, etc.)\n")
	fmt.Printf("    - Faster startup, no dependencies\n")
	fmt.Printf("    - Each connection is independent\n")
	fmt.Printf("  • tmux Mode (Optional):\n")
	fmt.Printf("    - Session persistence across connections\n")
	fmt.Printf("    - Multi-client support\n")
	fmt.Printf("    - Requires tmux installation\n")
	fmt.Printf("\n")

	fmt.Printf("QUICK START:\n")
	fmt.Printf("  %s run                     # Start with default settings\n", programName)
	fmt.Printf("  %s run --tmux              # Start with session persistence\n", programName)
	fmt.Printf("  %s run -i 0.0.0.0 -p 8080  # Start on all interfaces, port 8080\n", programName)
	fmt.Printf("  %s stop                    # Stop the server\n", programName)
	fmt.Printf("\n")

	fmt.Printf("COMMON EXAMPLES:\n")
	fmt.Printf("  Local development:\n")
	fmt.Printf("    %s run -a localhost:7314                 # Local access only\n", programName)
	fmt.Printf("    %s run --interface localhost --port 8080 # Alternative syntax\n", programName)
	fmt.Printf("\n")
	fmt.Printf("  Network access:\n")
	fmt.Printf("    %s run -a 0.0.0.0:7314                  # All interfaces\n", programName)
	fmt.Printf("    %s run -i 0.0.0.0 -p 7314 --tmux        # With session persistence\n", programName)
	fmt.Printf("\n")
	fmt.Printf("  Advanced usage:\n")
	fmt.Printf("    %s run --tmux --verbose                  # With detailed logging\n", programName)
	fmt.Printf("    %s run -i 0.0.0.0 -p 9000 --debug       # Debug mode, port 9000\n", programName)
	fmt.Printf("\n")

	fmt.Printf("CONFIGURATION:\n")
	fmt.Printf("  Default Address: %s\n", cfg.Server.DefaultAddress)
	fmt.Printf("  PID File: ~/.portty.pid\n")
	fmt.Printf("  Session Name: %s (tmux mode)\n", cfg.Server.SessionName)
	fmt.Printf("\n")

	fmt.Printf("SECURITY CONSIDERATIONS:\n")
	fmt.Printf("  • No built-in authentication mechanism\n")
	fmt.Printf("  • Designed for trusted network environments\n")
	fmt.Printf("  • Use reverse proxy (nginx/apache) for production\n")
	fmt.Printf("  • Consider firewall rules for network binding\n")
	fmt.Printf("\n")

	fmt.Printf("HELP FOR SPECIFIC COMMANDS:\n")
	fmt.Printf("  %s help run                # Detailed help for run command\n", programName)
	fmt.Printf("  %s help stop               # Detailed help for stop command\n", programName)
	fmt.Printf("  %s run --help              # Alternative help syntax\n", programName)
	fmt.Printf("\n")

	fmt.Printf("For more information, documentation, and updates:\n")
	fmt.Printf("  GitHub: https://github.com/PiTZE/PorTTY\n")
	fmt.Printf("  Issues: https://github.com/PiTZE/PorTTY/issues\n")
}

// ============================================================================
// CORE BUSINESS LOGIC
// ============================================================================

func runServer(address string) {
	serverManager := NewServerManager()
	ctx := context.Background()
	if err := serverManager.Start(ctx, address); err != nil {
		logger.ServerLogger.Fatal("failed to start server", err, logger.String("address", address))
	}
}

func cleanupTmuxSessions(ctx context.Context) {
	cleanupCtx, cleanupCancel := context.WithTimeout(ctx, cfg.Server.TmuxCleanupTimeout)
	defer cleanupCancel()
	checkCmd := exec.CommandContext(cleanupCtx, "tmux", "has-session", "-t", cfg.Server.SessionName)
	if err := checkCmd.Run(); err != nil {
		if err == context.DeadlineExceeded {
			logger.ServerLogger.Warn("tmux session check timed out", logger.String("session", cfg.Server.SessionName))
		} else {
			logger.ServerLogger.Info("tmux session does not exist, skipping cleanup", logger.String("session", cfg.Server.SessionName))
		}
	} else {
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

	cleanupCmd := exec.CommandContext(cleanupCtx, "bash", "-c", "tmux list-sessions -F '#{session_name}' 2>/dev/null | grep '^PorTTY-' | xargs -I{} tmux kill-session -t {} 2>/dev/null || true")
	if err := cleanupCmd.Run(); err != nil {
		if err == context.DeadlineExceeded {
			logger.ServerLogger.Warn("tmux cleanup timed out")
		} else {
			logger.ServerLogger.Warn("failed to cleanup orphaned tmux sessions", logger.Error(err))
		}
	}
}

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

// ============================================================================
// INPUT VALIDATION FUNCTIONS
// ============================================================================

func validatePort(port string, context string) error {
	if port == "" {
		return fmt.Errorf("port cannot be empty")
	}

	portNum, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("invalid port format '%s': port must be a number between 1-65535", port)
	}

	if portNum < 1 || portNum > 65535 {
		return fmt.Errorf("port out of range: %d (must be between 1-65535)", portNum)
	}

	if portNum < 1024 && os.Geteuid() != 0 {
		logWarning(fmt.Sprintf("Port %d requires root privileges", portNum))
		logInfo("Consider using a port >= 1024 or run with sudo")
	}

	return nil
}

func validateInterface(iface string, context string) error {
	if iface == "" {
		return fmt.Errorf("interface cannot be empty")
	}

	switch iface {
	case "localhost", "127.0.0.1", "::1":
	case "0.0.0.0", "::", "*":
	default:
		if net.ParseIP(iface) == nil && iface != "localhost" {
		}
	}

	return nil
}

func validateAddress(address string, context string) (string, int, error) {
	if address == "" {
		return "", 0, fmt.Errorf("address cannot be empty")
	}

	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return "", 0, fmt.Errorf("failed to parse address '%s': %w", address, err)
	}

	if err := validateInterface(host, context+" host"); err != nil {
		return "", 0, fmt.Errorf("invalid host in address '%s': %w", address, err)
	}

	if err := validatePort(portStr, context+" port"); err != nil {
		return "", 0, fmt.Errorf("invalid port in address '%s': %w", address, err)
	}

	port, _ := strconv.Atoi(portStr)
	if host == "" {
		host = "localhost"
	}

	return host, port, nil
}

// ============================================================================
// ERROR HANDLING FUNCTIONS
// ============================================================================

func logErrorWithContext(err error, context string, suggestion string) {
	fmt.Fprintf(os.Stderr, "%s[ERROR]%s %s: %v\n", ColorRed, ColorReset, context, err)
	if suggestion != "" {
		fmt.Fprintf(os.Stderr, "%s💡 Suggestion:%s %s\n", ColorYellow, ColorReset, suggestion)
	}
}

func logFatalWithContext(err error, context string, suggestion string) {
	fmt.Fprintf(os.Stderr, "%s[FATAL]%s %s: %v\n", ColorRed, ColorReset, context, err)
	if suggestion != "" {
		fmt.Fprintf(os.Stderr, "%s💡 Recovery:%s %s\n", ColorYellow, ColorReset, suggestion)
	}
}

func logInfo(message string) {
	fmt.Printf("%s[INFO]%s %s\n", ColorGreen, ColorReset, message)
}

func logWarning(message string) {
	fmt.Printf("%s[WARNING]%s %s\n", ColorYellow, ColorReset, message)
}

func logSuccess(message string) {
	fmt.Printf("%s[SUCCESS]%s %s\n", ColorGreen, ColorReset, message)
}

// ============================================================================
// ARGUMENT PARSING FUNCTIONS
// ============================================================================

type Arguments struct {
	Command     string
	Address     string
	Interface   string
	Port        string
	UseTmux     bool
	Verbose     bool
	Debug       bool
	ShowHelp    bool
	ShowVersion bool
}

func parseArguments(args []string) (*Arguments, error) {
	result := &Arguments{
		Address: cfg.Server.DefaultAddress,
	}

	if len(args) == 0 {
		result.ShowHelp = true
		return result, nil
	}

	// Check for global flags first (before command)
	if len(args) > 0 {
		switch args[0] {
		case "-h", "--help":
			result.ShowHelp = true
			return result, nil
		case "-v", "--version":
			result.ShowVersion = true
			return result, nil
		}
	}

	// Parse command
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		result.Command = args[0]
		args = args[1:]
	} else {
		result.ShowHelp = true
		return result, nil
	}

	// Special handling for help command with subcommands
	if result.Command == "help" {
		if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
			// help run, help stop, etc.
			result.Command = args[0]
			result.ShowHelp = true
			return result, nil
		}
		// Just "help" by itself
		result.ShowHelp = true
		result.Command = ""
		return result, nil
	}

	// Parse command-specific arguments
	for i := 0; i < len(args); i++ {
		arg := args[i]

		switch arg {
		case "-h", "--help":
			result.ShowHelp = true
			return result, nil

		case "-a", "--address":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("missing argument for %s", arg)
			}
			result.Address = args[i+1]
			i++

		case "-i", "--interface":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("missing argument for %s", arg)
			}
			result.Interface = args[i+1]
			i++

		case "-p", "--port":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("missing argument for %s", arg)
			}
			result.Port = args[i+1]
			i++

		case "--tmux":
			result.UseTmux = true

		case "--verbose":
			result.Verbose = true

		case "--debug":
			result.Debug = true

		default:
			if strings.HasPrefix(arg, "-") {
				return nil, fmt.Errorf("unknown option: %s", arg)
			} else {
				// No positional arguments allowed - enforce explicit flags only
				return nil, fmt.Errorf("unexpected argument: %s (use explicit flags like -a, -i, -p instead)", arg)
			}
		}
	}

	return result, nil
}

func buildFinalAddress(args *Arguments) (string, error) {
	if args.Interface != "" && args.Port != "" {
		if err := validateInterface(args.Interface, "interface argument"); err != nil {
			return "", fmt.Errorf("invalid interface: %w", err)
		}
		if err := validatePort(args.Port, "port argument"); err != nil {
			return "", fmt.Errorf("invalid port: %w", err)
		}
		return fmt.Sprintf("%s:%s", args.Interface, args.Port), nil
	}

	if args.Interface != "" {
		if err := validateInterface(args.Interface, "interface argument"); err != nil {
			return "", fmt.Errorf("invalid interface: %w", err)
		}
		_, defaultPort, _ := net.SplitHostPort(cfg.Server.DefaultAddress)
		return fmt.Sprintf("%s:%s", args.Interface, defaultPort), nil
	}

	if args.Port != "" {
		if err := validatePort(args.Port, "port argument"); err != nil {
			return "", fmt.Errorf("invalid port: %w", err)
		}
		defaultHost, _, _ := net.SplitHostPort(cfg.Server.DefaultAddress)
		if defaultHost == "" {
			defaultHost = "localhost"
		}
		return fmt.Sprintf("%s:%s", defaultHost, args.Port), nil
	}

	_, _, err := validateAddress(args.Address, "address argument")
	if err != nil {
		return "", err
	}

	return args.Address, nil
}

func main() {
	args, err := parseArguments(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sError:%s %s\n\n", ColorRed, ColorReset, err)
		showHelp()
		os.Exit(1)
	}

	if args.ShowHelp {
		if args.Command != "" {
			switch args.Command {
			case "run":
				showRunHelp()
			case "stop":
				showStopHelp()
			default:
				showHelp()
			}
		} else {
			showHelp()
		}
		return
	}

	if args.ShowVersion {
		showVersion()
		return
	}

	if args.Debug {
		logInfo("Debug logging enabled")
	} else if args.Verbose {
		logInfo("Verbose logging enabled")
	}

	switch args.Command {
	case "run":
		if args.UseTmux {
			cfg.Server.UseTmux = true
		}

		address, err := buildFinalAddress(args)
		if err != nil {
			logFatalWithContext(err, "address validation", "Check address format: [host]:[port] (e.g., localhost:7314)")
			os.Exit(1)
		}

		_, _, err = validateAddress(address, "final address")
		if err != nil {
			logFatalWithContext(err, "address validation", "Use format [host]:[port] where host is localhost/IP and port is 1-65535")
			os.Exit(1)
		}

		runServer(address)

	case "stop":
		homeDir, err := os.UserHomeDir()
		if err != nil {
			homeDir = cfg.Server.FallbackTempDir
		}
		pidFilePath := filepath.Join(homeDir, cfg.Server.PidFileName)
		stopServer(pidFilePath)

	case "help":
		showHelp()

	case "version":
		showVersion()

	default:
		fmt.Fprintf(os.Stderr, "%sError:%s Unknown command: %s\n\n", ColorRed, ColorReset, args.Command)
		showHelp()
		os.Exit(1)
	}
}
