package websocket

import (
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"your.org/portty/internal/ptybridge"
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
	defer conn.Close()

	// Create a new PTY bridge
	ptyBridge, err := ptybridge.New()
	if err != nil {
		log.Printf("Error creating PTY bridge: %v", err)
		return
	}
	defer ptyBridge.Close()

	// Set up WebSocket connection
	conn.SetReadLimit(maxMessageSize)
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// Start a goroutine to read from the WebSocket and write to the PTY
	go func() {
		defer ptyBridge.Close()
		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("WebSocket read error: %v", err)
				}
				break
			}

			// Process both text and binary messages
			if messageType == websocket.TextMessage || messageType == websocket.BinaryMessage {
				// Log the message for debugging (only first 20 bytes to avoid flooding logs)
				if len(message) > 0 {
					logLen := len(message)
					if logLen > 20 {
						logLen = 20
					}
					log.Printf("Received message type %d, first %d bytes: %v", messageType, logLen, message[:logLen])
				}

				// Process the input
				if err := ptyBridge.ProcessInput(message); err != nil {
					log.Printf("Error processing input: %v", err)
					break
				}
			}
		}
	}()

	// Start a goroutine to read from PTY and write to WebSocket
	go func() {
		ticker := time.NewTicker(pingPeriod)
		defer func() {
			ticker.Stop()
			conn.Close()
		}()

		// Buffer for reading from the PTY
		buf := make([]byte, 4096)

		for {
			select {
			case <-ptyBridge.Done():
				log.Println("PTY bridge done, exiting writer goroutine")
				return

			case <-ticker.C:
				// Send ping
				conn.SetWriteDeadline(time.Now().Add(writeWait))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					log.Printf("Error sending ping: %v", err)
					return
				}

			default:
				// Try to read from the PTY
				n, err := ptyBridge.Read(buf)
				if err != nil {
					if err == io.EOF {
						log.Println("PTY closed (EOF), exiting writer goroutine")
						return
					}
					log.Printf("PTY read error: %v", err)
					return
				}

				// If we read something, send it to the WebSocket
				if n > 0 {
					log.Printf("Read %d bytes from PTY, sending to WebSocket", n)
					conn.SetWriteDeadline(time.Now().Add(writeWait))
					if err := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
						log.Printf("WebSocket write error: %v", err)
						return
					}
				} else {
					// If we didn't read anything, sleep a bit to avoid busy-waiting
					time.Sleep(10 * time.Millisecond)
				}
			}
		}
	}()

	// Wait until the connection is closed
	<-ptyBridge.Done()
	log.Println("WebSocket connection closed")
}