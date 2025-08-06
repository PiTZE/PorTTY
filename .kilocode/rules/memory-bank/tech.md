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
- **Addons** (8 comprehensive addons):
  - xterm-addon-fit v0.10.0 (terminal sizing)
  - xterm-addon-attach v0.11.0 (WebSocket attachment)
  - xterm-addon-webgl v0.18.0 (hardware acceleration)
  - xterm-addon-search v0.15.0 (search functionality)
  - xterm-addon-unicode11 v0.8.0 (Unicode 11 support)
  - xterm-addon-web-links v0.11.0 (clickable web links)
  - xterm-addon-clipboard v0.1.0 (clipboard operations)
- **Styling**: Custom CSS with Nerd Font support and CSS custom properties
- **Fonts**: JetBrainsMono Nerd Font with comprehensive fallback stack for icon rendering
- **JavaScript**: Vanilla JS with ES6+ features (no framework dependencies)

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
   - Smart renderer selection (WebGL for desktop, Canvas for mobile)
   - WebGL hardware acceleration with context loss handling
   - Instant resize response without debouncing
   - Mobile device detection for optimal performance
   - Comprehensive addon integration for advanced features
   - Font size management with keyboard shortcuts
   - Advanced search overlay with keyboard navigation
   - Minimal DOM manipulation with efficient event handling

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
```

## Code Organization Standards

### File Structure Patterns
- Consistent section dividers: `// ============================================================================` for Go files
- Standardized file organization:
  1. Package declaration and imports
  2. Constants and type definitions
  3. Global variables and configuration
  4. Utility functions
  5. Core business logic
  6. Main execution logic
- Section headers with descriptive comments in ALL CAPS

### Frontend Code Organization (v0.2+)
- **Organized Asset Structure**: Dedicated directories for different asset types
  ```
  cmd/portty/assets/
  ├── css/terminal.css         # Centralized styling
  ├── js/terminal.js           # WebSocket client logic
  ├── js/sw.js                 # Service worker
  └── icons/                   # Theme-based icon system
      ├── light-theme-icon.svg # Black icon for light backgrounds
      └── dark-theme-icon.svg  # White icon for dark backgrounds
  ```
- **CSS Custom Properties**: Centralized theming configuration using CSS variables
  ```css
  :root {
      --font-family: 'JetBrainsMono Nerd Font', 'JetBrainsMono NF', 'JetBrains Mono', 'Nerd Font Complete', 'Nerd Font', monospace;
      --font-size: 14px;
      --background-color: #000000;
      --foreground-color: #f0f0f0;
  }
  ```
- **JavaScript Classes**: Object-oriented approach for complex functionality (e.g., ConnectionStatusManager)
- **DRY Principles**: Eliminate code duplication between CSS and JavaScript
- **Single Responsibility**: Each file/class has one clear purpose
- **Consistent Font Usage**: JetBrains Mono standardized across entire application
- **Theme-Based Icons**: Separate icons for light and dark theme compatibility

### Coding Philosophy
- **Minimal Commenting**: Function-level documentation only, self-documenting code through clear naming
- **Naming Conventions**: Standard Go conventions (PascalCase for exported, camelCase for unexported)
- **Error Handling**: Consistent patterns using `log.Printf()`, `log.Fatalf()` with context wrapping via `fmt.Errorf()` with `%w` verb
- **Visual Consistency**: Uniform logging patterns and error message formats across components
- **Configuration Centralization**: Use CSS custom properties and constants to avoid hardcoded values

### Project-Specific Standards
- **WebSocket Handling**: Consistent connection lifecycle management patterns
- **PTY Management**: Standardized terminal session handling approaches
- **Configuration**: Constants for default values and configuration management
- **Testing**: Go testing conventions with `_test.go` files and proper test organization
- **PWA Standards**: Service worker caching, manifest configuration, and offline-first approach