# PorTTY Architecture

## System Overview

PorTTY follows a clean, modular architecture with clear separation of concerns:

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│   Browser   │────▶│  Web Server  │────▶│    tmux     │
│  (xterm.js) │◀────│  (Go HTTP)   │◀────│  (session)  │
└─────────────┘     └──────────────┘     └─────────────┘
       ▲                    │                     ▲
       │              ┌─────▼─────┐               │
       └──────────────│ WebSocket │───────────────┘
                      │  Handler  │
                      └───────────┘
                            │
                      ┌─────▼─────┐
                      │PTY Bridge │
                      └───────────┘
```

## Core Components

### 1. Main Application (`cmd/portty/main.go`)
- **Purpose**: Entry point and HTTP server management
- **Key Functions**:
  - `runServer()`: Starts HTTP server and handles shutdown
  - `parseAddress()`: Validates and parses bind address
  - `checkTmuxInstalled()`: Ensures tmux dependency
- **Design Decisions**:
  - Embedded web assets for single binary distribution
  - Graceful shutdown with cleanup
  - PID file management for process control

### 2. PTY Bridge (`internal/ptybridge/ptybridge.go`)
- **Purpose**: Manages pseudo-terminal and tmux session lifecycle
- **Key Functions**:
  - `New()`: Creates or attaches to tmux session
  - `ProcessInput()`: Handles client messages (input, resize, keepalive)
  - `Read()/Write()`: Direct PTY I/O operations
- **Design Patterns**:
  - Singleton tmux session per server instance
  - Session persistence across connections
  - Clean separation of concerns for PTY operations

### 3. WebSocket Handler (`internal/websocket/websocket.go`)
- **Purpose**: Real-time bidirectional communication
- **Key Functions**:
  - `HandleWS()`: Main WebSocket connection handler
  - Three concurrent goroutines:
    1. Read from WebSocket → PTY
    2. Process messages from channel
    3. Read from PTY → WebSocket
- **Design Patterns**:
  - Channel-based message buffering
  - Context-based cancellation
  - Graceful connection lifecycle management

### 4. Frontend (`cmd/portty/assets/`)
- **Components**:
  - `index.html`: PWA-enabled HTML structure with meta tags
  - `terminal.css`: Centralized styling using CSS custom properties
  - `terminal.js`: Enhanced WebSocket client with consolidated connection management
  - `manifest.json`: PWA manifest for app installation
  - `sw.js`: Service worker for offline caching and PWA functionality
- **Key Features**:
  - Progressive Web App capabilities with installation prompts
  - Auto-reconnection with exponential backoff
  - Consolidated connection status management
  - Responsive terminal sizing
  - Performance-optimized xterm.js configuration
  - Centralized theming with CSS custom properties

## Data Flow

1. **Connection Establishment**:
   - Browser requests `/` → Serves embedded HTML/CSS/JS
   - JavaScript initiates WebSocket connection to `/ws`
   - Server creates PTY bridge to tmux session
   - Bidirectional data flow established

2. **Input Processing**:
   - User keystrokes → xterm.js → WebSocket → PTY bridge → tmux
   - Special messages (resize, keepalive) handled separately

3. **Output Processing**:
   - tmux output → PTY → WebSocket → xterm.js → Browser display
   - Buffered reading for performance

## Key Design Decisions

1. **Single Binary Distribution**:
   - All assets embedded using Go's `embed` package
   - No external file dependencies except tmux
   - Simplifies deployment and updates

2. **tmux Integration**:
   - Provides session persistence
   - Enables multi-client connections
   - Handles terminal multiplexing

3. **Performance Optimizations**:
   - Large buffer sizes (16KB) for data transfer
   - Channel buffering to prevent blocking
   - Debounced resize events
   - No unnecessary mutex locking

4. **Error Handling**:
   - Graceful degradation on errors
   - Automatic reconnection logic
   - Clear error messaging to users

## File Structure

```
PorTTY/
├── cmd/portty/
│   ├── main.go              # Main application entry
│   └── assets/              # Embedded web assets
│       ├── index.html       # PWA-enabled HTML structure
│       ├── terminal.css     # Centralized styling with CSS custom properties
│       ├── terminal.js      # Enhanced WebSocket client with connection management
│       ├── manifest.json    # PWA manifest for app installation
│       ├── sw.js            # Service worker for offline caching
│       └── favicon.ico      # Application icon
├── internal/
│   ├── ptybridge/           # PTY and tmux management
│   │   └── ptybridge.go
│   └── websocket/           # WebSocket handling
│       └── websocket.go
├── build.sh                 # Build script
├── install.sh               # Installation script
├── go.mod                   # Go module definition
├── go.sum                   # Dependency checksums
└── PWA_ARCHITECTURE.md      # PWA implementation documentation
```

## Development Standards

PorTTY follows consistent development patterns across all components. Detailed coding standards including file organization, naming conventions, error handling patterns, and project-specific implementation guidelines are documented in the technology stack specifications.

## Security Considerations

- No built-in authentication (by design)
- Expects reverse proxy for production use
- WebSocket origin checking disabled for flexibility
- Designed for trusted network environments