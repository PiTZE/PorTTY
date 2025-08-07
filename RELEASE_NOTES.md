# PorTTY v0.2+ Release Notes

This release transforms PorTTY into a Progressive Web App (PWA) with enhanced user experience, comprehensive architecture improvements, dual shell mode support, Nerd Font integration for perfect terminal rendering, and world-class CLI tools with professional installation system.

## New Features

### Shell Mode Support
- **Default Shell Mode** - Uses user's default shell (zsh, bash, etc.) directly as primary mode
- **Optional tmux Mode** - Available via `--tmux` flag for session persistence
- **Smart Shell Detection** - Fixed NixOS compatibility by prioritizing `/etc/passwd` over environment variables
- **Cross-Platform Support** - Works with various shell configurations and distributions
- **Command Line Interface** - Added `--tmux` flag with updated help documentation

### Nerd Font Integration
- **Comprehensive Font Stack** - Added support for JetBrainsMono Nerd Font and popular Nerd Font variants
- **Local Font Detection** - Prioritizes locally installed Nerd Fonts for proper icon rendering
- **Perfect Icon Display** - Shell prompts now display special characters correctly (╭─, ⇡, !6, ❯, etc.)
- **Fallback Support** - Graceful degradation to standard fonts if Nerd Fonts unavailable

### Enhanced Scrollback Experience
- **Hidden Scrollbars** - Added CSS to hide scrollbars while maintaining 10,000 line scrollback functionality
- **Full Scrollback Access** - Users can scroll through terminal history without visible scrollbars
- **Cross-Browser Compatibility** - Works with WebKit, Firefox, and IE scrollbar hiding

### Terminal Resize Improvements
- **Robust Initialization** - Enhanced initial fit logic with multiple timing strategies
- **Error Handling** - Added proper error handling and retry mechanisms for resize operations
- **Fallback Timing** - Multiple resize attempts to ensure proper viewport filling on load

### Progressive Web App (PWA) Support
- **App Installation** - Install PorTTY as a native app on any device
- **Offline Caching** - Service worker caches app shell for faster loading
- **App Manifest** - Native app experience with proper icons and metadata
- **Installation Prompts** - Smart prompts to install the app when appropriate

### Enhanced Connection Management
- **Connection Status Indicator** - Real-time connection status in top-right corner
- **Connection Info Button** - Detailed connection information and diagnostics
- **Keep-Alive Mechanism** - Enhanced WebSocket keep-alive to prevent timeouts
- **Improved Reconnection** - Better handling of connection drops and recovery

### User Interface Improvements
- **Nerd Font Typography** - JetBrainsMono Nerd Font with comprehensive fallback stack
- **Connection Status Visibility** - Always-visible connection status with fallback mechanisms
- **PWA Installation Banner** - Non-intrusive installation prompts
- **Mobile-Optimized** - Better responsive design for mobile devices

## Professional CLI Tools & Installation System

### Enhanced Installation Script
- **Complete Feature Parity** - Interactive and command-line modes offer identical functionality
- **Professional User Experience** - Colored output with ANSI formatting (red errors, yellow warnings, green success)
- **Comprehensive Input Validation** - Port range checking (1-65535), interface validation, directory verification
- **Smart Privilege Handling** - Interactive mode entry without requiring root upfront
- **Robust Download System** - Support for compressed GitHub release archives with proper extraction
- **Enhanced Error Handling** - Context-aware error messages with actionable recovery suggestions
- **Interactive Menu System** - Intuitive menus for installation mode, shell mode, and service management
- **Production-Ready Quality** - Enterprise-grade installer suitable for professional deployment

### Enhanced PorTTY Binary CLI
- **Professional Argument Parsing** - Comprehensive flag support with both short (`-p`) and long (`--port`) formats
- **Multi-Level Help System** - Detailed documentation with examples and troubleshooting guidance
- **Input Validation** - Address, port, and interface validation with contextual error messages
- **Colored Output System** - Consistent ANSI color formatting matching installation script standards
- **Enhanced Error Handling** - Context-aware errors with specific recovery suggestions
- **Standard CLI Conventions** - Modern CLI design following established patterns

### Installation Script Features
- **Dual Installation Modes** - System installation (requires sudo) and user installation (no root needed)
- **Shell Mode Selection** - Choose between default shell mode and tmux mode during installation
- **Network Configuration** - Interface and port configuration with validation
- **Advanced Options** - Checksum verification, force updates, verbose logging, debug mode
- **Service Management** - Complete systemd service lifecycle management
- **Comprehensive Status Checking** - Detailed system information and service status reporting
- **Log Management** - Centralized logging with fallback directories and proper permissions

### PorTTY Binary Features
- **Enhanced Run Command** - `-a/--address`, `-i/--interface`, `-p/--port`, `--tmux`, `--verbose`, `--debug` options
- **Improved Stop Command** - Verbose and debug options for detailed shutdown logging
- **Global Options** - `-h/--help`, `-v/--version` work at any command level
- **Input Validation** - Port range validation with privilege warnings, interface format checking
- **Professional Help Documentation** - Multi-level help with examples and security guidance

### CLI Usage Examples
```bash
# Enhanced installer usage
./install.sh install --user --tmux -i localhost -p 8080 --verbose
./install.sh status --user
./install.sh logs --user

# Enhanced PorTTY binary usage
./portty run -i localhost -p 8080 --tmux --verbose
./portty run -a 0.0.0.0:7314 --debug
./portty stop --verbose
./portty --help
./portty run --help
```

### Code Quality & Standards
- **Minimal Commenting Philosophy** - Self-documenting code with clear naming conventions
- **Consistent Organization** - Proper section dividers and structured file organization
- **Professional Error Messages** - Clear problem identification with actionable solutions
- **Unified User Experience** - Consistent behavior and formatting across all tools

## Architecture Improvements

### Interface-Based Design
- **Complete Interface Abstraction** - All components now use interfaces for better testability
- **Dependency Injection** - Clean separation of concerns with injected dependencies
- **Factory Pattern** - Interface factories for creating components with proper dependencies
- **Compile-time Interface Checks** - Ensures all implementations satisfy their contracts

### Configuration Management
- **Centralized Configuration** - Single source of truth for all application settings
- **Type-Safe Access** - Structured configuration with proper types and defaults
- **Environment-Specific Settings** - Easy configuration for different deployment scenarios
- **Default Values** - Sensible defaults for all configuration options

### Structured Logging
- **Component-Based Loggers** - Separate loggers for server, websocket, and ptybridge components
- **Structured Fields** - Key-value logging with helper functions for common types
- **Log Levels** - Support for Info, Warn, Error, and Fatal log levels
- **Consistent Formatting** - Uniform log message format across all components

### Context Propagation
- **Graceful Shutdown** - Proper context cancellation for coordinated shutdown
- **Operation Timeouts** - Context-aware operations with configurable timeouts
- **Resource Cleanup** - Automatic cleanup of resources when contexts are cancelled
- **Error Handling** - Context-aware error handling and propagation

## Code Quality Improvements

### Eliminated Code Duplications
- **Centralized Configuration** - CSS custom properties for consistent theming
- **Font Standardization** - Single source of truth for JetBrains Mono font
- **Connection Management** - Consolidated multiple connection status functions into single class
- **Removed Redundant Files** - Cleaned up unused connection-manager.js

### Frontend Architecture Enhancements
- **CSS Custom Properties** - Centralized theming configuration:
  ```css
  :root {
      --font-family: 'JetBrainsMono Nerd Font', 'JetBrainsMono NF', 'JetBrains Mono', 'Nerd Font Complete', 'Nerd Font', monospace;
      --font-size: 14px;
      --background-color: #000000;
      --foreground-color: #f0f0f0;
  }
  ```
- **ConnectionStatusManager Class** - Object-oriented approach for connection handling
- **DRY Principles** - Eliminated ~40% of code duplication
- **Improved Maintainability** - Better code organization and documentation

### Backend Architecture Enhancements
- **Interface Segregation** - Small, focused interfaces for specific responsibilities
- **Single Responsibility** - Each component has a clear, single purpose
- **Open/Closed Principle** - Easy to extend without modifying existing code
- **Dependency Inversion** - High-level modules don't depend on low-level modules

## Technical Improvements

### PWA Infrastructure
- **Service Worker** - Caches static assets and CDN resources
- **Manifest.json** - Complete PWA manifest with icons and metadata
- **Cache Versioning** - Proper cache management with version v0.2
- **Offline Strategy** - Cache-first strategy for app shell resources

### Enhanced WebSocket Handling
- **Consolidated Management** - Single class handles all connection states
- **Better Error Handling** - Improved error messages and recovery
- **Performance Optimizations** - Reduced redundant status checks
- **Debugging Support** - Global access to terminal and connection manager

## Installation

### Enhanced Installation Options

#### System Installation (Default)
```bash
# Interactive installation with menus
sudo ./install.sh

# Non-interactive with options
sudo ./install.sh install --tmux -i 0.0.0.0 -p 7314 --verbose

# Quick install from web
sudo bash -c "$(curl -fsSL https://raw.githubusercontent.com/PiTZE/PorTTY/master/install.sh)"
```

#### User Installation (No Root Required)
```bash
# Interactive user installation
./install.sh

# Non-interactive user installation
./install.sh install --user --tmux -i localhost -p 8080

# User installation with custom directory
./install.sh install --user -d ~/.local/bin --tmux
```

#### Installation Features
- **Dual Mode Support** - System-wide or user-local installation
- **Shell Mode Selection** - Choose default shell or tmux mode during installation
- **Network Configuration** - Configure interface and port with validation
- **Service Management** - Automatic systemd service creation and management
- **Comprehensive Validation** - Input validation with clear error messages
- **Recovery Guidance** - Detailed error messages with actionable solutions

Then access your terminal at: http://your-server-ip:7314

### PWA Installation
- Visit the web interface in a modern browser
- Look for the installation prompt or "Install PorTTY" button
- Install as a native app for the best experience

## File Structure Updates

### New Architecture Files
- `internal/interfaces/interfaces.go` - Complete interface definitions for all components
- `internal/config/config.go` - Centralized configuration management
- `internal/logger/logger.go` - Structured logging implementation

### New PWA-related Files
- `manifest.json` - PWA app manifest
- `sw.js` - Service worker for caching
- `PWA_ARCHITECTURE.md` - PWA implementation documentation

### Enhanced Existing Files
- `cmd/portty/main.go` - Now uses interface-based architecture with dependency injection and professional CLI argument system
- `internal/websocket/websocket.go` - Refactored to use interfaces and structured logging
- `internal/ptybridge/ptybridge.go` - Enhanced with context propagation and interface compliance
- `install.sh` - Completely rewritten with professional CLI features, feature parity, and comprehensive error handling

## Breaking Changes

### CLI Changes
- **PorTTY Binary** - Enhanced argument parsing with stricter validation (old positional arguments deprecated)
- **Installation Script** - Complete rewrite with new interactive menus and command structure
- **Error Handling** - More detailed error messages may affect automated scripts expecting specific formats

### Frontend Changes
- Removed `connection-manager.js` (functionality moved to `terminal.js`)
- Updated service worker cache to v0.2 (will refresh cached assets)

### Backend Changes
- **Interface-based Architecture** - Internal APIs now use interfaces (affects custom implementations)
- **Configuration Structure** - Configuration access now goes through structured config types
- **Logging Changes** - Log format has changed to structured format with component prefixes
- **Context Requirements** - All operations now require context parameters for proper cancellation

### Migration Notes
- Existing installations will continue to work without changes
- Custom integrations may need updates to use new interfaces
- Log parsing tools may need updates for new structured format
- Scripts using old PorTTY binary syntax should migrate to explicit flags
- Installation automation should use new command-line flags instead of interactive mode

## Known Issues

- WebSocket connections are not encrypted by default (use HTTPS/WSS in production)
- No built-in authentication (consider using a reverse proxy with authentication)

## Development Benefits

### For Contributors
- **Better Testability** - Interface-based design makes unit testing much easier
- **Clear Contracts** - Interfaces define clear contracts between components
- **Easier Mocking** - Interfaces can be easily mocked for testing
- **Modular Development** - Components can be developed and tested independently
- **Professional CLI Standards** - Both installer and binary follow modern CLI conventions
- **Self-Documenting Code** - Minimal commenting philosophy with clear naming conventions

### For Maintainers
- **Structured Logging** - Better debugging and monitoring capabilities
- **Configuration Management** - Centralized configuration reduces errors
- **Graceful Shutdown** - Proper resource cleanup prevents issues
- **Error Handling** - Consistent error handling patterns across components
- **Professional Installation Experience** - Enterprise-grade installer reduces support burden
- **Comprehensive Validation** - Input validation prevents common configuration errors
- **Unified User Experience** - Consistent behavior across all CLI tools

### For Users
- **Intuitive Installation** - Professional installer with clear guidance and error recovery
- **Flexible Deployment Options** - System-wide or user-local installation modes
- **Comprehensive Help System** - Multi-level documentation with examples and troubleshooting
- **Professional CLI Experience** - Modern argument parsing with validation and colored output
- **Error Recovery Guidance** - Clear error messages with actionable solutions

## Acknowledgements

- [xterm.js](https://xtermjs.org/) for the terminal emulator
- [creack/pty](https://github.com/creack/pty) for PTY handling
- [gorilla/websocket](https://github.com/gorilla/websocket) for WebSocket communication
- PWA best practices from web.dev and MDN documentation
- Go interface design patterns from the Go community