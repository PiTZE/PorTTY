# PorTTY Technology Stack

## Core Technologies

### Backend (Go 1.21+)
- **Language**: Go (Golang)
- **Web Framework**: Standard library `net/http`
- **WebSocket**: `github.com/gorilla/websocket v1.2.0`
- **PTY Handling**: `github.com/creack/pty v1.1.24`
- **Asset Embedding**: Go 1.16+ `embed` package

### Frontend
- **Terminal Emulator**: xterm.js v5.5.0 (CDN)
- **Addons**:
  - xterm-addon-fit v0.10.0 (terminal sizing)
  - xterm-addon-attach v0.11.0 (WebSocket attachment)
- **Styling**: Custom CSS with JetBrains Mono font
- **JavaScript**: Vanilla JS (no framework dependencies)

### System Dependencies
- **tmux**: Required for session management
- **systemd**: Optional for service management
- **bash**: For installation and build scripts

## Development Setup

### Prerequisites
```bash
# Go 1.21 or higher
go version

# tmux (required at runtime)
tmux -V

# Git for version control
git --version
```

### Building from Source
```bash
# Clone repository
git clone https://github.com/PiTZE/PorTTY.git
cd PorTTY

# Build binary
./build.sh

# Run locally
./portty run
```

### Nix Development Environment
```bash
# Enter development shell
nix-shell

# Provides:
# - go, gopls, go-tools, delve
# - git, tmux, curl, jq
# - Configured GOPATH and environment
```

## Technical Constraints

### Performance Limits
- Maximum message size: 16KB
- WebSocket buffer sizes: 4KB read/write
- Terminal scrollback: 10,000 lines
- Default terminal size: 100x40

### Timeouts and Intervals
- WebSocket write timeout: 10 seconds
- Ping/Pong interval: 54 seconds (90% of pong wait)
- Pong wait timeout: 60 seconds
- Reconnection backoff: 1s * 1.5^attempts

### Browser Requirements
- Modern browsers with WebSocket support
- JavaScript enabled
- Recommended: Chrome 90+, Firefox 88+, Safari 14+

## Deployment Configuration

### Default Settings
- Port: 7314
- Interface: localhost (configurable)
- Session name: "PorTTY" (fixed)
- PID file: `~/.portty.pid`

### Systemd Service
```ini
[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/portty run 0.0.0.0:7314
Restart=always
RestartSec=5
```

### Installation Paths
- Binary: `/usr/local/bin/portty`
- Service: `/etc/systemd/system/portty.service`
- Logs: `/var/log/portty/`

## Build Process

### Build Script Features
- Automatic version detection from git tags
- Code formatting with `go fmt`
- Code vetting with `go vet`
- Binary stripping for size reduction
- Platform-specific archive creation

### Release Artifacts
- Binary with embedded assets
- No external file dependencies
- Platform detection (linux/darwin, amd64/arm64/arm)
- SHA256 checksums for verification

## Dependencies Management

### Direct Dependencies
```
github.com/creack/pty v1.1.24
github.com/gorilla/websocket v1.2.0
```

### Frontend Dependencies (CDN)
- No npm/node_modules required
- All frontend libraries loaded from CDN
- Specific versions pinned for stability

## Tool Usage Patterns

### Installation Script
- Interactive and non-interactive modes
- Service management integration
- Automatic dependency checking
- Logging and error handling
- Update and uninstall capabilities

### Runtime Patterns
- Single process model
- Graceful shutdown handling
- Signal handling (SIGTERM, SIGINT)
- Automatic session cleanup

## Performance Optimizations

1. **Buffer Management**:
   - 16KB buffers for PTY reading
   - 100-message channel buffer
   - No mutex locking in hot paths

2. **Connection Handling**:
   - Concurrent goroutines for I/O
   - Non-blocking message processing
   - Efficient error recovery

3. **Frontend Optimizations**:
   - Canvas renderer for performance
   - Debounced resize events
   - Minimal DOM manipulation

## Security Considerations

### Current Limitations
- No built-in TLS/HTTPS
- No authentication mechanism
- Origin checking disabled

### Recommended Production Setup
```nginx
# Nginx reverse proxy with auth
location / {
    auth_basic "Restricted";
    auth_basic_user_file /etc/nginx/.htpasswd;
    
    proxy_pass http://localhost:7314;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
}