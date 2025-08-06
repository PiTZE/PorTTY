# PorTTY - Browser-Based Terminal Access

## Project Overview
PorTTY is a lightweight Go application that provides secure, persistent terminal access through any web browser. It eliminates the need for SSH clients by offering a simple web interface to access terminal sessions that persist across connections.

## Main Objectives
- Enable terminal access from any device with a web browser
- Maintain session continuity across disconnections
- Provide single-binary deployment for easy installation
- Offer real-time, low-latency terminal interaction

## Key Features
- **Single Static Binary**: Complete application in one executable file
- **Persistent Sessions**: Uses tmux to maintain terminal state between connections
- **WebSocket Communication**: Real-time bidirectional data transfer
- **Auto-Reconnection**: Automatic recovery from network interruptions
- **Responsive Interface**: Terminal adapts to browser window size with instant resize
- **Multi-Client Support**: Multiple browsers can connect to the same session
- **Progressive Web App**: Installable web app with offline capabilities
- **Advanced Terminal Features**: Search, clipboard, web links, Unicode 11 support
- **Hardware Acceleration**: WebGL rendering for desktop, optimized canvas for mobile
- **Smart Device Detection**: Automatic optimization for mobile and desktop devices

## Technologies Used
- **Backend**: Go (Golang) with embedded web assets and interface-based architecture
- **Frontend**: xterm.js v5.5.0 with 8 comprehensive addons for advanced features
- **Session Management**: tmux for persistence
- **Communication**: WebSocket protocol with keep-alive and auto-reconnection
- **PTY Handling**: creack/pty library
- **Web Framework**: Standard Go net/http package
- **PWA Features**: Service worker, manifest, and offline caching

## Significance
PorTTY addresses the common challenge of remote terminal access by providing a browser-based solution that requires no client-side installation. It's particularly valuable for:
- System administrators needing quick terminal access
- Developers working across multiple devices
- Educational environments requiring terminal access
- Situations where SSH clients are unavailable or restricted

The project demonstrates efficient Go programming practices, real-time web communication, and practical system integration through its tmux session management.