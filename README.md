# PorTTY v0.1

A standalone Go binary that serves a browser-based shell terminal with tmux integration.

## Quick Install

```bash
curl -sSL https://raw.githubusercontent.com/PiTZE/PorTTY/master/install.sh | sudo bash
```

Then access your terminal at: http://your-server-ip:7314

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

### Prerequisites

- Go 1.21+
- tmux (installed on the system where PorTTY will run)

### Building from Source

```bash
# Clone the repository
git clone https://github.com/PiTZE/PorTTY.git
cd PorTTY

# Build the binary
./build.sh

# The binary will be created as 'portty' in the current directory
```

### Nix Environment (Optional)

If you use Nix, a development shell is provided:

```bash
# Enter the development environment
nix-shell

# Build PorTTY within the environment
./build.sh
```

## Usage

### Starting the Server

```bash
# Start on default address (localhost:7314)
./portty run

# Start on a specific address
./portty run localhost:8080

# Start on all interfaces, port 8080
./portty run :8080

# Start on a specific interface and port
./portty run 0.0.0.0:8080
```

### Stopping the Server

```bash
./portty stop
```

### Getting Help

```bash
./portty help
```

### Accessing the Terminal

Once the server is running, open your browser to:
- `http://localhost:7314` (if using default port)
- `http://localhost:8080` (if using port 8080)
- Or whatever address you configured

## How It Works

PorTTY creates a bridge between your web browser and a terminal session using the following components:

1. **Web Server** - A Go HTTP server that serves the web interface and handles WebSocket connections
2. **WebSocket Handler** - Manages real-time bidirectional communication between browser and server
3. **PTY Bridge** - Connects the WebSocket to a tmux session using a pseudo-terminal (PTY)
4. **Frontend Terminal** - An xterm.js-based terminal emulator in the browser

When you connect to PorTTY:
- If a tmux session named "PorTTY" exists, you attach to it
- If no such session exists, a new one is created
- Multiple browser connections can attach to the same session simultaneously
- When all connections are closed, the tmux session persists for future connections

## Configuration

PorTTY has minimal configuration and is designed to work out of the box:

- **Default Port**: 7314
- **Default Address**: localhost
- **Session Name**: PorTTY (fixed)
- **PID File**: `.portty.pid` in your home directory

## Dependencies

### Runtime Dependencies

- tmux (must be installed on the system)

### Go Dependencies

- `github.com/creack/pty v1.1.24` - For PTY handling
- `github.com/gorilla/websocket v1.2.0` - For WebSocket communication

### Frontend Dependencies (loaded from CDN)

- xterm.js 5.5.0
- xterm-addon-fit 0.10.0
- xterm-addon-attach 0.11.0

## Directory Structure

```
.
├── cmd/
│   └── portty/
│       ├── main.go              # Main application entry point
│       └── assets/
│           ├── index.html       # Frontend HTML
│           ├── terminal.css     # Terminal styling
│           └── terminal.js      # Frontend JavaScript
├── internal/
│   ├── ptybridge/
│   │   └── ptybridge.go        # PTY and tmux session management
│   └── websocket/
│       └── websocket.go        # WebSocket handling
├── .gitignore                  # Git ignore rules
├── build.sh                    # Build script
├── go.mod                      # Go module definition
├── README.md                   # This file
└── shell.nix                   # Nix development environment
```

## Security Considerations

- PorTTY runs a web server that provides terminal access
- By default, it only listens on localhost, but can be configured to listen on all interfaces
- Consider the security implications of exposing terminal access to a network
- WebSocket connections are not encrypted by default (use HTTPS/WSS in production)
- The application does not include authentication - consider adding a reverse proxy with authentication for production use

## Troubleshooting

### tmux Not Found

If you see an error about tmux not being installed:
```bash
# On Ubuntu/Debian
sudo apt-get install tmux

# On CentOS/RHEL
sudo yum install tmux

# On macOS
brew install tmux
```

### Port Already in Use

If the default port is already in use:
```bash
# Use a different port
./portty run localhost:8080
```

### Session Issues

If you encounter problems with terminal sessions:
```bash
# Kill any existing PorTTY tmux sessions
tmux kill-session -t PorTTY

# Check for orphaned sessions
tmux list-sessions
```

## Contributing

Contributions are welcome! Please feel free to submit issues and pull requests.

When contributing:
1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Ensure the code builds with `./build.sh`
5. Submit a pull request

## License

MIT License - see LICENSE file for details.