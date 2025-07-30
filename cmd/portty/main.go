package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
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

//go:embed assets
var webContent embed.FS

const (
	// SessionName is the name of the tmux session
	SessionName = "PorTTY"
	// PidFileName is the name of the file that stores the PID
	PidFileName = ".portty.pid"
)

func main() {
	// Parse command line flags
	port := flag.Int("port", 8080, "Port to listen on")
	command := flag.String("command", "run", "Command to execute: run or stop")
	flag.Parse()

	// Get the home directory for storing the PID file
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "/tmp" // Fallback to /tmp if we can't get the home directory
	}
	pidFilePath := filepath.Join(homeDir, PidFileName)

	// Handle commands
	switch *command {
	case "run":
		runServer(*port, pidFilePath)
	case "stop":
		stopServer(pidFilePath)
	default:
		log.Fatalf("Unknown command: %s. Use 'run' or 'stop'", *command)
	}
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

// runServer starts the PorTTY server
func runServer(port int, pidFilePath string) {
	// Check if tmux is installed
	if !checkTmuxInstalled() {
		log.Fatalf("tmux is not installed. Please install tmux to use PorTTY.")
	}

	// Check if the tmux session already exists
	if checkSessionExists(SessionName) {
		log.Printf("Found existing tmux session: %s", SessionName)
	}

	// Write PID to file
	pid := os.Getpid()
	if err := os.WriteFile(pidFilePath, []byte(strconv.Itoa(pid)), 0644); err != nil {
		log.Printf("Warning: Failed to write PID file: %v", err)
	}

	// Set up HTTP server
	mux := http.NewServeMux()

	// Handle WebSocket connections
	mux.HandleFunc("/ws", websocket.HandleWS)

	// Create a sub-filesystem for the web content
	webFS, err := fs.Sub(webContent, "assets")
	if err != nil {
		log.Fatalf("Failed to create sub-filesystem: %v", err)
	}

	// Serve static files
	fileServer := http.FileServer(http.FS(webFS))
	mux.Handle("/", fileServer)

	// Create HTTP server
	server := &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", port),
		Handler: mux,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Starting PorTTY on http://0.0.0.0:%d", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Set up graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Wait for interrupt signal
	<-stop
	log.Println("Shutting down server...")

	// Remove PID file
	os.Remove(pidFilePath)

	// Kill the main tmux session
	log.Println("Cleaning up tmux session...")
	killCmd := exec.Command("tmux", "kill-session", "-t", SessionName)
	if err := killCmd.Run(); err != nil {
		log.Printf("Failed to kill tmux session: %v", err)
	}

	// Also clean up any orphaned sessions with PorTTY prefix
	cleanupCmd := exec.Command("bash", "-c", "tmux list-sessions -F '#{session_name}' 2>/dev/null | grep '^PorTTY-' | xargs -I{} tmux kill-session -t {} 2>/dev/null || true")
	cleanupCmd.Run()

	// Create a deadline for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server gracefully stopped")
}

// stopServer stops the PorTTY server
func stopServer(pidFilePath string) {
	// Read PID from file
	pidBytes, err := os.ReadFile(pidFilePath)
	if err != nil {
		// Try to find the process by name if PID file doesn't exist
		findAndKillProcess()
		return
	}

	// Parse PID
	pidStr := strings.TrimSpace(string(pidBytes))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		log.Printf("Invalid PID in file: %v", err)
		findAndKillProcess()
		return
	}

	// Send signal to process
	process, err := os.FindProcess(pid)
	if err != nil {
		log.Printf("Failed to find process with PID %d: %v", pid, err)
		findAndKillProcess()
		return
	}

	// Send SIGTERM to the process
	if err := process.Signal(syscall.SIGTERM); err != nil {
		log.Printf("Failed to send signal to process: %v", err)
		findAndKillProcess()
		return
	}

	log.Printf("Sent termination signal to PorTTY (PID: %d)", pid)

	// Remove PID file
	os.Remove(pidFilePath)
}

// findAndKillProcess tries to find and kill the PorTTY process by name
func findAndKillProcess() {
	log.Println("Trying to find PorTTY process by name...")

	// Use pgrep to find the process
	cmd := exec.Command("bash", "-c", "pgrep -f 'portty --command run|portty --port'")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("No PorTTY process found: %v", err)
		return
	}

	// Parse PIDs
	pids := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, pidStr := range pids {
		if pidStr == "" {
			continue
		}

		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}

		// Kill the process
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
