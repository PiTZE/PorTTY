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
	maxMessageSize = 8192
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
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

	// Set up WebSocket connection
	conn.SetReadLimit(maxMessageSize)
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// Start a goroutine to read from the WebSocket and write to the PTY
	go func() {
		defer wg.Done()
		defer cancel()
		defer ptyBridge.Close()

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
					} else if !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
						// Only log if it's not a normal closure
						log.Printf("WebSocket connection closed: %v", err)
					}
					return
				}

				// Process both text and binary messages
				if messageType == websocket.TextMessage || messageType == websocket.BinaryMessage {
					// Only log if debug logging is needed
					if len(message) > 0 && len(message) < 100 {
						// Avoid logging large messages or control sequences
						log.Printf("Received message type %d, length: %d", messageType, len(message))
					}

					// Process the input
					if err := ptyBridge.ProcessInput(message); err != nil {
						log.Printf("Error processing input: %v", err)
						// Don't break on processing errors, try to continue
						// Only break if it's a fatal error
						if err == io.EOF || err == io.ErrClosedPipe {
							return
						}
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

		// Buffer for reading from the PTY
		buf := make([]byte, 4096)

		for {
			select {
			case <-ctx.Done():
				log.Println("Context done, exiting writer goroutine")
				return
			default:
				// Try to read from the PTY
				n, err := ptyBridge.Read(buf)

				if err != nil {
					if err == io.EOF {
						log.Println("PTY closed (EOF), exiting writer goroutine")
						return
					}

					// Handle other errors
					log.Printf("PTY read error: %v", err)

					// For transient errors, we'll continue after a short delay
					// For permanent errors, we'll exit
					if err == io.ErrClosedPipe || err == io.ErrUnexpectedEOF {
						return
					}

					// For other errors, wait a bit before retrying
					time.Sleep(100 * time.Millisecond)
					continue
				}

				// If we read something, send it to the WebSocket
				if n > 0 {
					// Reduce logging to avoid spam
					if n > 100 {
						log.Printf("Read %d bytes from PTY, sending to WebSocket", n)
					}

					// Set a write deadline and send the data
					conn.SetWriteDeadline(time.Now().Add(writeWait))
					if err := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
						log.Printf("WebSocket write error: %v", err)

						// Check if it's a fatal error
						if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
							return
						}

						// For other errors, wait a bit before continuing
						time.Sleep(100 * time.Millisecond)
					}
				}
			}
		}
	}()

	// Start a separate goroutine for ping messages
	go func() {
		defer wg.Done()
		defer cancel()

		ticker := time.NewTicker(pingPeriod)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Send ping
				conn.SetWriteDeadline(time.Now().Add(writeWait))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					log.Printf("Error sending ping: %v", err)
					return
				}
			}
		}
	}()

	// Wait for the PTY bridge to be done or context to be cancelled
	select {
	case <-ptyBridge.Done():
		log.Println("PTY bridge done")
	case <-ctx.Done():
		log.Println("Context cancelled")
	}

	// Cancel the context to signal all goroutines to stop
	cancel()

	// Wait for all goroutines to finish
	wg.Wait()

	// Close the connection
	conn.Close()
	ptyBridge.Close()
	log.Println("WebSocket connection closed")
}
