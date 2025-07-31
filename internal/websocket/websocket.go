package websocket

import (
	"context"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/PiTZE/PorTTY/internal/ptybridge"
	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 16384 // Increased from 8192 to 16KB for better performance
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096, // Increased from 1024 to 4KB
	WriteBufferSize: 4096, // Increased from 1024 to 4KB
	// Allow all origins
	CheckOrigin: func(r *http.Request) bool { return true },
}

// HandleWS handles WebSocket connections
func HandleWS(w http.ResponseWriter, r *http.Request) {
	// Upgrade the HTTP connection to a WebSocket connection
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error upgrading to WebSocket: %v", err)
		return
	}

	// Create a wait group to manage goroutines
	var wg sync.WaitGroup
	wg.Add(3) // We'll have 3 goroutines

	// Create a context to manage goroutine lifecycle
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a new PTY bridge
	ptyBridge, err := ptybridge.New()
	if err != nil {
		log.Printf("Error creating PTY bridge: %v", err)
		conn.Close()
		return
	}

	// Create a buffered channel for messages
	messageChan := make(chan []byte, 100)

	// Set up WebSocket connection
	conn.SetReadLimit(maxMessageSize)
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// Start a goroutine to read from the WebSocket
	go func() {
		defer wg.Done()
		defer cancel()
		defer close(messageChan)

		for {
			select {
			case <-ctx.Done():
				return
			default:
				// Set a reasonable read deadline to prevent hanging
				conn.SetReadDeadline(time.Now().Add(pongWait))

				messageType, message, err := conn.ReadMessage()
				if err != nil {
					if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
						log.Printf("WebSocket read error: %v", err)
					}
					return
				}

				// Process both text and binary messages
				if messageType == websocket.TextMessage || messageType == websocket.BinaryMessage {
					// Send message to processing goroutine
					select {
					case messageChan <- message:
						// Message sent successfully
					default:
						// Channel is full, log warning and drop message
						log.Printf("Warning: Message channel full, dropping message")
					}
				}
			}
		}
	}()

	// Start a goroutine to process messages
	go func() {
		defer wg.Done()
		defer ptyBridge.Close()

		for {
			select {
			case <-ctx.Done():
				return
			case message, ok := <-messageChan:
				if !ok {
					// Channel closed
					return
				}

				// Process the input
				if err := ptyBridge.ProcessInput(message); err != nil {
					// Only log serious errors
					if err == io.EOF || err == io.ErrClosedPipe {
						log.Printf("Fatal error processing input: %v", err)
						return
					}
				}
			}
		}
	}()

	// Start a goroutine to read from PTY and write to WebSocket
	go func() {
		defer wg.Done()
		defer cancel()
		defer conn.Close()

		// Buffer for reading from the PTY - increased to 16KB
		buf := make([]byte, 16384)

		for {
			select {
			case <-ctx.Done():
				return
			default:
				// Try to read from the PTY
				n, err := ptyBridge.Read(buf)

				if err != nil {
					if err == io.EOF {
						return
					}

					// For permanent errors, we'll exit
					if err == io.ErrClosedPipe || err == io.ErrUnexpectedEOF {
						return
					}

					// For other errors, wait a bit before retrying
					time.Sleep(50 * time.Millisecond) // Reduced from 100ms to 50ms
					continue
				}

				// If we read something, send it to the WebSocket
				if n > 0 {
					// Set a write deadline and send the data
					conn.SetWriteDeadline(time.Now().Add(writeWait))
					if err := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
						// Check if it's a fatal error
						if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
							return
						}

						// For other errors, wait a bit before continuing
						time.Sleep(50 * time.Millisecond) // Reduced from 100ms to 50ms
					}
				}
			}
		}
	}()

	// Wait for the PTY bridge to be done or context to be cancelled
	select {
	case <-ptyBridge.Done():
		// PTY bridge done
	case <-ctx.Done():
		// Context cancelled
	}

	// Cancel the context to signal all goroutines to stop
	cancel()

	// Wait for all goroutines to finish
	wg.Wait()

	// Close the connection and PTY bridge
	conn.Close()
	ptyBridge.Close()
}
