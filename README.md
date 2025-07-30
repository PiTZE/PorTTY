# PorTTY

A standalone Go binary that serves a browser-based shell terminal.

## Overview

- Single static binary
- Browser-based terminal using xterm.js
- WebSocket communication
- Tmux integration
- Responsive design
- Copy/paste support

## Building

```bash
# Build the binary
./build.sh

# Run the server
./portty
```

## Usage

Once the server is running, open your browser to http://localhost:8080/ to access the terminal.

## Dependencies

- Go 1.21+
- xterm.js 5.5.0 (loaded from CDN)
- github.com/creack/pty v1.1.24
- github.com/gorilla/websocket v1.2.0

## Directory Structure

```
.
├── go.mod
├── go.sum
├── cmd/
│   └── portty/
│       ├── main.go
│       └── web/
│           ├── index.html
│           ├── js/
│           │   └── terminal.js
│           └── css/
│               └── styles.css
└── internal/
    ├── ptybridge/
    │   └── ptybridge.go
    └── websocket/
        └── websocket.go
```

## License

MIT