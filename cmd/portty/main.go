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
	"syscall"
	"time"

	"github.com/PiTZE/PorTTY/internal/websocket"
)

//go:embed assets
var webContent embed.FS

func main() {
	// Parse command line flags
	port := flag.Int("port", 8080, "Port to listen on")
	flag.Parse()

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
		Addr:    fmt.Sprintf("0.0.0.0:%d", *port),
		Handler: mux,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Starting PorTTY on http://0.0.0.0:%d", *port)
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

	// Kill the main tmux session
	log.Println("Cleaning up tmux session...")
	killCmd := exec.Command("tmux", "kill-session", "-t", "PorTTY")
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
