package websocket

// ============================================================================
// IMPORTS
// ============================================================================

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

// ============================================================================
// CONSTANTS AND GLOBAL VARIABLES
// ============================================================================

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 16384
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// ============================================================================
// CORE BUSINESS LOGIC
// ============================================================================

// HandleWS handles WebSocket connections
func HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error upgrading to WebSocket: %v", err)
		return
	}

	var wg sync.WaitGroup
	wg.Add(3)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ptyBridge, err := ptybridge.New()
	if err != nil {
		log.Printf("Error creating PTY bridge: %v", err)
		conn.Close()
		return
	}

	messageChan := make(chan []byte, 100)

	conn.SetReadLimit(maxMessageSize)
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	go func() {
		defer wg.Done()
		defer cancel()
		defer close(messageChan)

		for {
			select {
			case <-ctx.Done():
				return
			default:
				conn.SetReadDeadline(time.Now().Add(pongWait))

				messageType, message, err := conn.ReadMessage()
				if err != nil {
					if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
						log.Printf("WebSocket read error: %v", err)
					}
					return
				}

				if messageType == websocket.TextMessage || messageType == websocket.BinaryMessage {
					select {
					case messageChan <- message:
					default:
						log.Printf("Warning: Message channel full, dropping message")
					}
				}
			}
		}
	}()

	go func() {
		defer wg.Done()
		defer ptyBridge.Close()

		for {
			select {
			case <-ctx.Done():
				return
			case message, ok := <-messageChan:
				if !ok {
					return
				}

				if err := ptyBridge.ProcessInput(message); err != nil {
					if err == io.EOF || err == io.ErrClosedPipe {
						log.Printf("Fatal error processing input: %v", err)
						return
					}
				}
			}
		}
	}()

	go func() {
		defer wg.Done()
		defer cancel()
		defer conn.Close()

		buf := make([]byte, 16384)

		for {
			select {
			case <-ctx.Done():
				return
			default:
				n, err := ptyBridge.Read(buf)

				if err != nil {
					if err == io.EOF {
						return
					}

					if err == io.ErrClosedPipe || err == io.ErrUnexpectedEOF {
						return
					}

					time.Sleep(50 * time.Millisecond)
					continue
				}

				if n > 0 {
					conn.SetWriteDeadline(time.Now().Add(writeWait))
					if err := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
						if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
							return
						}

						time.Sleep(50 * time.Millisecond)
					}
				}
			}
		}
	}()

	select {
	case <-ptyBridge.Done():
	case <-ctx.Done():
	}

	cancel()

	wg.Wait()

	conn.Close()
	ptyBridge.Close()
}
