package main

// ============================================================================
// IMPORTS
// ============================================================================

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/PiTZE/PorTTY/internal/websocket"
)

// ============================================================================
// CONSTANTS AND GLOBAL VARIABLES
// ============================================================================

//go:embed assets
var webContent embed.FS

const (
	SessionName    = "PorTTY"
	PidFileName    = ".portty.pid"
	DefaultAddress = "localhost:7314"
)

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

// parseAddress parses the address and returns host and port
func parseAddress(address string) (string, int, error) {
	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return "", 0, fmt.Errorf("invalid address format: %w", err)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, fmt.Errorf("invalid port: %w", err)
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
	log.Println("Trying to find PorTTY process by name...")

	cmd := exec.Command("bash", "-c", "pgrep -f 'portty run'")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("No PorTTY process found")
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
			log.Printf("Failed to send signal to process %d: %v", pid, err)
			continue
		}

		log.Printf("Sent termination signal to PorTTY (PID: %d)", pid)
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
	fmt.Printf("                        Default: %s\n", DefaultAddress)
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
	fmt.Println("PorTTY v0.1")
	fmt.Println("A lightweight, web-based terminal emulator powered by tmux")
	fmt.Println("https://github.com/PiTZE/PorTTY")
}

// showHelp displays usage information
func showHelp() {
	programName := filepath.Base(os.Args[0])
	version := "v0.1"

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
	fmt.Printf("                 Default: %s\n", DefaultAddress)
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

// runServer starts the PorTTY server
func runServer(address string, pidFilePath string) {
	if !checkTmuxInstalled() {
		log.Fatalf("tmux is not installed. Please install tmux to use PorTTY.")
	}

	host, port, err := parseAddress(address)
	if err != nil {
		log.Fatalf("Error parsing address: %v", err)
	}

	if checkSessionExists(SessionName) {
		log.Printf("Found existing tmux session: %s", SessionName)
	}

	pid := os.Getpid()
	if err := os.WriteFile(pidFilePath, []byte(strconv.Itoa(pid)), 0644); err != nil {
		log.Printf("Warning: Failed to write PID file: %v", err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/ws", websocket.HandleWS)

	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/x-icon")
		w.WriteHeader(http.StatusNoContent)
	})

	webFS, err := fs.Sub(webContent, "assets")
	if err != nil {
		log.Fatalf("Failed to create sub-filesystem: %v", err)
	}

	fileServer := http.FileServer(http.FS(webFS))
	mux.Handle("/", fileServer)

	bindAddr := fmt.Sprintf("%s:%d", host, port)
	server := &http.Server{
		Addr:    bindAddr,
		Handler: mux,
	}

	go func() {
		log.Printf("Starting PorTTY on http://%s", bindAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop
	log.Println("Shutting down server...")

	os.Remove(pidFilePath)

	log.Println("Cleaning up tmux session...")
	killCmd := exec.Command("tmux", "kill-session", "-t", SessionName)
	if err := killCmd.Run(); err != nil {
		log.Printf("Failed to kill tmux session: %v", err)
	}

	cleanupCmd := exec.Command("bash", "-c", "tmux list-sessions -F '#{session_name}' 2>/dev/null | grep '^PorTTY-' | xargs -I{} tmux kill-session -t {} 2>/dev/null || true")
	cleanupCmd.Run()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server gracefully stopped")
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
		log.Printf("Invalid PID in file: %v", err)
		findAndKillProcess()
		return
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		log.Printf("Failed to find process with PID %d: %v", pid, err)
		findAndKillProcess()
		return
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		log.Printf("Failed to send signal to process: %v", err)
		findAndKillProcess()
		return
	}

	log.Printf("Sent termination signal to PorTTY (PID: %d)", pid)

	os.Remove(pidFilePath)
}

// ============================================================================
// MAIN EXECUTION LOGIC
// ============================================================================

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "/tmp"
	}
	pidFilePath := filepath.Join(homeDir, PidFileName)

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
		address := DefaultAddress

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

		runServer(address, pidFilePath)
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
