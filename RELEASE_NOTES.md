# PorTTY v0.2 Release Notes

This release transforms PorTTY into a Progressive Web App (PWA) with enhanced user experience, code refactoring, and improved maintainability.

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

## Code Quality Improvements

### Eliminated Code Duplications
- **Centralized Configuration** - CSS custom properties for consistent theming
- **Font Standardization** - Single source of truth for JetBrains Mono font
- **Connection Management** - Consolidated multiple connection status functions into single class
- **Removed Redundant Files** - Cleaned up unused connection-manager.js

### Architecture Enhancements
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

New PWA-related files:
- `manifest.json` - PWA app manifest
- `sw.js` - Service worker for caching
- `PWA_ARCHITECTURE.md` - PWA implementation documentation

## Breaking Changes

- Removed `connection-manager.js` (functionality moved to `terminal.js`)
- Updated service worker cache to v0.2 (will refresh cached assets)

## Known Issues

- WebSocket connections are not encrypted by default (use HTTPS/WSS in production)
- No built-in authentication (consider using a reverse proxy with authentication)

## Acknowledgements

- [xterm.js](https://xtermjs.org/) for the terminal emulator
- [creack/pty](https://github.com/creack/pty) for PTY handling
- [gorilla/websocket](https://github.com/gorilla/websocket) for WebSocket communication
- PWA best practices from web.dev and MDN documentation