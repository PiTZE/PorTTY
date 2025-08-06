# PorTTY Architecture

## System Overview

PorTTY follows a clean, modular architecture with clear separation of concerns:

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│   Browser   │────▶│  Web Server  │────▶│Default Shell│
│  (xterm.js) │◀────│  (Go HTTP)   │◀────│ (zsh/bash)  │
└─────────────┘     └──────────────┘     └─────────────┘
       ▲                    │                     ▲
       │              ┌─────▼─────┐               │
       └──────────────│ WebSocket │───────────────┘
                      │  Handler  │
                      └───────────┘
                            │
                      ┌─────▼─────┐
                      │PTY Bridge │ ──── Optional: tmux mode
                      └───────────┘      (--tmux flag)
```

## Core Components

### 1. Main Application (`cmd/portty/main.go`)
- **Purpose**: Entry point and HTTP server management with interface-based architecture
- **Key Functions**:
  - `NewServerManager()`: Creates server manager with dependency injection
  - `runServer()`: Starts HTTP server using interface-based approach
  - `parseAddress()`: Validates and parses bind address
  - `checkTmuxInstalled()`: Ensures tmux dependency
- **Design Decisions**:
  - Interface-based architecture for better testability
  - Dependency injection for component management
  - Embedded web assets for single binary distribution
  - Graceful shutdown with coordinated cleanup
  - PID file management for process control

### 2. Configuration Management (`internal/config/config.go`)
- **Purpose**: Centralized configuration with structured types
- **Key Features**:
  - Structured configuration types for all components
  - Default values and constants centralization
  - Type-safe configuration access
  - Environment-specific settings
  - Shell mode configuration (UseTmux flag)
  - Smart shell detection with NixOS compatibility
  - Cross-platform shell path resolution

### 3. Structured Logging (`internal/logger/logger.go`)
- **Purpose**: Component-specific logging with contextual fields
- **Key Features**:
  - Component-specific loggers (Server, WebSocket, PTYBridge)
  - Structured logging with contextual fields
  - Consistent log formatting across components
  - Error context preservation

### 4. Interface Definitions (`internal/interfaces/interfaces.go`)
- **Purpose**: Comprehensive interface definitions for all components
- **Key Features**:
  - Complete interface coverage for testability
  - Clear component boundaries and contracts
  - Factory pattern interfaces for component creation
  - Dependency injection support

### 5. PTY Bridge (`internal/ptybridge/ptybridge.go`)
- **Purpose**: Context-aware pseudo-terminal and tmux session lifecycle management
- **Key Functions**:
  - `New()`: Creates or attaches to tmux session with context support
  - `ProcessInput()`: Handles client messages (input, resize, keepalive) with context awareness
  - `Read()/Write()`: Context-aware PTY I/O operations
  - `Close()`: Closes PTY connection while preserving tmux session for reconnection
- **Design Patterns**:
  - Singleton tmux session per server instance
  - Session persistence across connections
  - Context-aware operations with cancellation support
  - Clean separation of concerns for PTY operations

### 6. WebSocket Handler (`internal/websocket/websocket.go`)
- **Purpose**: Interface-based real-time bidirectional communication
- **Key Functions**:
  - `NewHandler()`: Creates WebSocket handler with PTY factory injection
  - `HandleWS()`: Main WebSocket connection handler with context support
  - Three concurrent goroutines:
    1. Read from WebSocket → PTY
    2. Process messages from channel
    3. Read from PTY → WebSocket
- **Design Patterns**:
  - Interface-based dependency injection
  - Channel-based message buffering
  - Context-based cancellation and coordination
  - Graceful connection lifecycle management

### 7. Frontend (`cmd/portty/assets/`)
- **Components**:
  - `index.html`: PWA-enabled HTML structure with meta tags
  - `css/terminal.css`: Centralized styling using CSS custom properties
  - `js/terminal.js`: Enhanced WebSocket client with consolidated connection management
  - `js/sw.js`: Service worker for offline caching and PWA functionality
  - `manifest.json`: PWA manifest for app installation
  - `icons/`: Theme-based icon system with light and dark variants
- **Key Features**:
  - Progressive Web App capabilities with installation prompts
  - Auto-reconnection with exponential backoff
  - Consolidated connection status management
  - High-performance terminal resizing with WebGL acceleration
  - Instant resize response without debouncing for optimal UX
  - Performance-optimized xterm.js configuration with proper addon timing
  - Centralized theming with CSS custom properties
  - Organized asset structure with dedicated directories
  - Comprehensive xterm.js addon integration (8 addons)
  - Smart renderer selection (WebGL for desktop, Canvas for mobile)
  - Advanced search functionality with overlay UI
  - Font size management with keyboard shortcuts
  - Mobile device detection and optimization
  - WebGL context loss handling for production stability

## Data Flow

1. **Connection Establishment**:
   - Browser requests `/` → Serves embedded HTML/CSS/JS
   - JavaScript initiates WebSocket connection to `/ws`
   - Server creates PTY bridge to tmux session via factory pattern
   - Bidirectional data flow established with context coordination

2. **Input Processing**:
   - User keystrokes → xterm.js → WebSocket → PTY bridge → tmux
   - Special messages (resize, keepalive) handled separately with context awareness

3. **Output Processing**:
   - tmux output → PTY → WebSocket → xterm.js → Browser display
   - Buffered reading for performance with context cancellation support

## Key Design Decisions

1. **Interface-Based Architecture (v0.2)**:
   - All components implement well-defined interfaces
   - Dependency injection for better testability
   - Factory patterns for component creation
   - Clear separation of concerns and contracts

2. **Context Propagation (v0.2)**:
   - Application-level context for coordinated shutdown
   - Context-aware operations throughout the stack
   - Graceful cancellation and cleanup coordination

3. **Centralized Configuration (v0.2)**:
   - Structured configuration types
   - Constants and defaults centralization
   - Type-safe configuration access

4. **Structured Logging (v0.2)**:
   - Component-specific loggers
   - Contextual field logging
   - Consistent error handling patterns

5. **Single Binary Distribution**:
   - All assets embedded using Go's `embed` package
   - No external file dependencies except tmux
   - Simplifies deployment and updates

6. **tmux Integration**:
   - Provides session persistence (verified intact in v0.2)
   - Enables multi-client connections
   - Handles terminal multiplexing
   - Sessions persist across individual connection closures

7. **Performance Optimizations**:
   - Large buffer sizes (16KB) for data transfer
   - Channel buffering to prevent blocking
   - Instant terminal resizing with WebGL hardware acceleration
   - Context-aware operations without unnecessary blocking
   - Optimized addon loading order for proper initialization timing

8. **Error Handling**:
   - Graceful degradation on errors
   - Automatic reconnection logic
   - Clear error messaging to users
   - Structured error context preservation

## File Structure

```
PorTTY/
├── cmd/portty/
│   ├── main.go              # Interface-based main application entry
│   └── assets/              # Embedded web assets (organized structure)
│       ├── index.html       # PWA-enabled HTML structure
│       ├── manifest.json    # PWA manifest for app installation
│       ├── icon.svg         # Legacy application icon (SVG)
│       ├── css/             # Stylesheets directory
│       │   └── terminal.css # Centralized styling with CSS custom properties
│       ├── js/              # JavaScript files directory
│       │   ├── terminal.js  # Enhanced WebSocket client with connection management
│       │   └── sw.js        # Service worker for offline caching
│       └── icons/           # Theme-based icons directory
│           ├── light-theme-icon.svg  # Black icon for light backgrounds
│           └── dark-theme-icon.svg   # White icon for dark backgrounds
├── internal/
│   ├── config/              # Centralized configuration management
│   │   └── config.go
│   ├── interfaces/          # Comprehensive interface definitions
│   │   └── interfaces.go
│   ├── logger/              # Structured logging system
│   │   └── logger.go
│   ├── ptybridge/           # Context-aware PTY and tmux management
│   │   └── ptybridge.go
│   └── websocket/           # Interface-based WebSocket handling
│       └── websocket.go
├── build.sh                 # Build script
├── install.sh               # Installation script
├── go.mod                   # Go module definition
├── go.sum                   # Dependency checksums
└── PWA_ARCHITECTURE.md      # PWA implementation documentation
```

## Development Standards

PorTTY follows consistent development patterns across all components. Detailed coding standards including file organization, naming conventions, error handling patterns, and project-specific implementation guidelines are documented in the technology stack specifications.

## Shell Mode Behavior (v0.2+)

**Dual Mode Support**: PorTTY now supports both default shell and optional tmux modes:

### Default Shell Mode (Primary)
1. **Direct Shell Access**: Uses user's default shell (zsh, bash, etc.) directly
2. **No Session Persistence**: Each connection is independent
3. **Faster Startup**: No tmux overhead
4. **Native Experience**: Behaves exactly like a local terminal

### Optional tmux Mode (--tmux flag)
1. **Session Persistence**: tmux sessions persist across connection closures
2. **Multi-Client Support**: Multiple browsers can connect to same session
3. **Server Shutdown**: Explicitly kills tmux sessions during cleanup
4. **Session Continuity**: Users can reconnect and resume exactly where they left off

**Shell Detection**: Smart detection prioritizes `/etc/passwd` over environment variables for NixOS compatibility.

This behavior is implemented in:
- [`config.getDefaultShell()`](internal/config/config.go:56): Smart shell detection with NixOS support
- [`ptybridge.New()`](internal/ptybridge/ptybridge.go:74): Mode-aware PTY creation
- [`main.go`](cmd/portty/main.go): Command line flag parsing and conditional tmux checking

## Security Considerations

- No built-in authentication (by design)
- Expects reverse proxy for production use
- WebSocket origin checking disabled for flexibility
- Designed for trusted network environments