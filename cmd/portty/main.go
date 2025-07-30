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

	"your.org/portty/internal/websocket"
)

//go:embed web
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
	webFS, err := fs.Sub(webContent, "web")
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

	// Kill any remaining tmux sessions
	exec.Command("tmux", "kill-session", "-t", "PorTTY").Run()

	// Create a deadline for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server gracefully stopped")
}
