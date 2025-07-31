# PorTTY

A standalone Go binary that serves a browser-based shell terminal with tmux integration.

## Quick Install

```bash
# Option 1: Standard method
curl -sSL https://raw.githubusercontent.com/PiTZE/PorTTY/master/install.sh | sudo bash

# Option 2: Alternative method (more reliable in some environments)
sudo bash -c "$(curl -fsSL https://raw.githubusercontent.com/PiTZE/PorTTY/master/install.sh)"

# Option 3: Using wget instead of curl
wget -O - https://raw.githubusercontent.com/PiTZE/PorTTY/master/install.sh | sudo bash
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
- **Systemd service integration** - Run as a system service with auto-restart
- **Interactive installation** - User-friendly setup with configuration options
- **Performance optimized** - Enhanced buffer sizes and efficient data handling

## Installation

### Prerequisites

- Go 1.21+ (for building from source)
- tmux (installed on the system where PorTTY will run)

### Automated Installation (Recommended)

The install.sh script provides a comprehensive installation experience:

#### Interactive Installation

```bash
# Using curl | bash
curl -sSL https://raw.githubusercontent.com/PiTZE/PorTTY/master/install.sh | sudo bash

# Alternative method (more reliable)
sudo bash -c "$(curl -fsSL https://raw.githubusercontent.com/PiTZE/PorTTY/master/install.sh)"

# Using wget
wget -O - https://raw.githubusercontent.com/PiTZE/PorTTY/master/install.sh | sudo bash
```

#### Non-interactive Installation with Custom Settings

```bash
# Using curl | bash with parameters
curl -sSL https://raw.githubusercontent.com/PiTZE/PorTTY/master/install.sh | sudo bash -s -- -i 0.0.0.0 -p 8080 -y

# Alternative method with parameters
sudo bash -c "$(curl -fsSL https://raw.githubusercontent.com/PiTZE/PorTTY/master/install.sh) -i 0.0.0.0 -p 8080 -y"
```

#### Show All Available Options

```bash
curl -sSL https://raw.githubusercontent.com/PiTZE/PorTTY/master/install.sh | bash -s -- --help
```

### Building from Source

```bash
git clone https://github.com/PiTZE/PorTTY.git && cd PorTTY && ./build.sh
```
The binary will be created as 'portty' in the current directory

### Nix Environment (Optional)

If you use Nix, a development shell is provided:

```bash
# Enter the development environment
nix-shell

# Build PorTTY within the environment
./build.sh
```

## Usage

### Command Line Interface

```bash
# Start on default address (localhost:7314)
./portty run

# Start on a specific address
./portty run localhost:7314

# Start on all interfaces, port 7314
./portty run :7314

# Start on a specific interface and port
./portty run 0.0.0.0:7314

# Use command-line options
./portty run --address 0.0.0.0 --port 8080
./portty run -p 8080

# Stop the server
./portty stop

# Get help
./portty help
./portty run --help

# Show version
./portty --version
```

### Service Management (when installed via install.sh)

```bash
# Start the service
sudo systemctl start portty

# Stop the service
sudo systemctl stop portty

# Restart the service
sudo systemctl restart portty

# Enable at boot
sudo systemctl enable portty

# Check status
sudo systemctl status portty

# View logs
sudo journalctl -u portty -f
```

### Interactive Management

The install.sh script also provides an interactive management interface:

```bash
# Launch interactive menu (if installed)
sudo /usr/local/bin/portty install

# Available options in interactive mode:
# - Installation & Setup
# - Service Management
# - Status & Information
# - Logs & Configuration
```

### Accessing the Terminal

Once the server is running, open your browser to:
- `http://localhost:7314` (default port)
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
- **Service File**: `/etc/systemd/system/portty.service` (when installed via install.sh)
- **Log Directory**: `/var/log/portty/` (when installed via install.sh)

### Environment Variables

When installed via install.sh, you can configure logging behavior:

```bash
# Set log level
export PORTTY_LOG_LEVEL=DEBUG  # DEBUG, INFO, WARNING, ERROR, FATAL
```

## Performance Optimizations

PorTTY v0.1 includes several performance optimizations:

- Removed mutex locking in PTY operations for better throughput
- Increased buffer sizes (16KB) for improved data transfer
- Implemented message buffering with channels to prevent blocking
- Added debounced resize handling to reduce unnecessary resize events
- Optimized terminal settings for better rendering performance
- Enhanced WebSocket buffer sizes for better real-time communication

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
├── install.sh                  # Installation and management script
├── README.md                   # This file
├── RELEASE_NOTES.md            # Release notes
└── shell.nix                   # Nix development environment
```

## Security Considerations

⚠️ **Important Security Notes**

- PorTTY runs a web server that provides terminal access
- By default, it only listens on localhost, but can be configured to listen on all interfaces
- **Never expose PorTTY directly to the internet without proper security measures**
- WebSocket connections are not encrypted by default (use HTTPS/WSS in production)
- The application does not include authentication

### Recommended Security Practices

1. **Use a Reverse Proxy**: Place PorTTY behind a reverse proxy like Nginx or Apache
2. **Add Authentication**: Implement authentication at the reverse proxy level
3. **Use HTTPS**: Always use HTTPS/WSS in production environments
4. **Network Restrictions**: Use firewall rules to restrict access to trusted networks
5. **Regular Updates**: Keep PorTTY and its dependencies updated

### Example Nginx Configuration

```nginx
server {
    listen 443 ssl;
    server_name your-domain.com;
    
    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;
    
    auth_basic "Restricted Access";
    auth_basic_user_file /etc/nginx/.htpasswd;
    
    location / {
        proxy_pass http://localhost:7314;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## Troubleshooting

### tmux Not Found

If you see an error about tmux not being installed:

```bash
# On Ubuntu/Debian
sudo apt-get install tmux

# On CentOS/RHEL
sudo yum install tmux

# On Fedora
sudo dnf install tmux

# On macOS
brew install tmux
```

### Port Already in Use

If the default port is already in use:

```bash
# Use a different port
./portty run localhost:8000

# Or find what's using the port
sudo netstat -tulpn | grep :7314
sudo lsof -i :7314
```

### Session Issues

If you encounter problems with terminal sessions:

```bash
# Kill any existing PorTTY tmux sessions
tmux kill-session -t PorTTY

# Check for orphaned sessions
tmux list-sessions
```

### Service Issues

When using systemd service:

```bash
# Check service status
sudo systemctl status portty

# View service logs
sudo journalctl -u portty

# Check for configuration errors
sudo systemctl cat portty
```

### Connection Issues

If you can't connect to the terminal:

```bash
# Check if the server is running
ps aux | grep portty

# Check if the port is listening
sudo netstat -tulpn | grep :7314 # or your own port

# Check firewall rules
sudo ufw status
sudo iptables -L -n

# Test with curl
curl -I http://localhost:7314 # or your own port
```

## Contributing

Contributions are welcome! Please feel free to submit issues and pull requests.

When contributing:
1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Ensure the code builds with `./build.sh`
5. Submit a pull request

### Development Setup

```bash
# Clone the repository
git clone https://github.com/PiTZE/PorTTY.git
cd PorTTY

# Use Nix environment (recommended)
nix-shell

# Or install dependencies manually
# Ensure Go 1.21+ and tmux are installed

# Build the project
./build.sh

# Run the development server
./portty run
```

## License

MIT License - see LICENSE file for details.