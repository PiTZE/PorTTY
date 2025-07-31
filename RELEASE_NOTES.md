# PorTTY v0.1 Release Notes

This is the initial release of PorTTY, a standalone Go binary that serves a browser-based shell terminal with tmux integration.

## Features

- **Single static binary** - No external dependencies except tmux
- **Browser-based terminal** - Access your terminal from any web browser
- **Real-time communication** - WebSocket connection for low-latency terminal interaction
- **Session persistence** - Uses tmux to maintain terminal sessions between connections
- **Responsive design** - Automatically adjusts to browser window size
- **Copy/paste support** - Full clipboard integration
- **Connection resilience** - Automatic reconnection with exponential backoff
- **Graceful shutdown** - Properly cleans up resources on termination

## Installation

### Quick Install

```bash
curl -sSL https://raw.githubusercontent.com/PiTZE/PorTTY/master/install.sh | sudo bash
```

Then access your terminal at: http://your-server-ip:7314

### Manual Installation

1. Download the binary for your platform
2. Make it executable: `chmod +x portty`
3. Run it: `./portty run`

## Performance Optimizations

This release includes several performance optimizations:

- Removed mutex locking in PTY operations for better throughput
- Increased buffer sizes for improved data transfer
- Implemented message buffering with channels to prevent blocking
- Added debounced resize handling to reduce unnecessary resize events
- Optimized terminal settings for better rendering performance

## Known Issues

- WebSocket connections are not encrypted by default (use HTTPS/WSS in production)
- No built-in authentication (consider using a reverse proxy with authentication)

## Acknowledgements

- [xterm.js](https://xtermjs.org/) for the terminal emulator
- [creack/pty](https://github.com/creack/pty) for PTY handling
- [gorilla/websocket](https://github.com/gorilla/websocket) for WebSocket communication