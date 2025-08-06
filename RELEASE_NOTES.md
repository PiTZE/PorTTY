# PorTTY v0.2 Release Notes

This release transforms PorTTY into a Progressive Web App (PWA) with enhanced user experience, comprehensive architecture improvements, and improved maintainability through interface-based design.

## New Features

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
- **Consistent Typography** - JetBrains Mono font standardized across entire application
- **Connection Status Visibility** - Always-visible connection status with fallback mechanisms
- **PWA Installation Banner** - Non-intrusive installation prompts
- **Mobile-Optimized** - Better responsive design for mobile devices

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
      --font-family: 'JetBrains Mono', monospace;
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

### Quick Install

```bash
sudo bash -c "$(curl -fsSL https://raw.githubusercontent.com/PiTZE/PorTTY/master/install.sh)"
```

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
- `cmd/portty/main.go` - Now uses interface-based architecture with dependency injection
- `internal/websocket/websocket.go` - Refactored to use interfaces and structured logging
- `internal/ptybridge/ptybridge.go` - Enhanced with context propagation and interface compliance

## Breaking Changes

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

## Known Issues

- WebSocket connections are not encrypted by default (use HTTPS/WSS in production)
- No built-in authentication (consider using a reverse proxy with authentication)

## Development Benefits

### For Contributors
- **Better Testability** - Interface-based design makes unit testing much easier
- **Clear Contracts** - Interfaces define clear contracts between components
- **Easier Mocking** - Interfaces can be easily mocked for testing
- **Modular Development** - Components can be developed and tested independently

### For Maintainers
- **Structured Logging** - Better debugging and monitoring capabilities
- **Configuration Management** - Centralized configuration reduces errors
- **Graceful Shutdown** - Proper resource cleanup prevents issues
- **Error Handling** - Consistent error handling patterns across components

## Acknowledgements

- [xterm.js](https://xtermjs.org/) for the terminal emulator
- [creack/pty](https://github.com/creack/pty) for PTY handling
- [gorilla/websocket](https://github.com/gorilla/websocket) for WebSocket communication
- PWA best practices from web.dev and MDN documentation
- Go interface design patterns from the Go community