<div align="center">
  <img src="cmd/portty/assets/icons/dark-theme-icon.svg" alt="PorTTY Logo" width="128" height="128">
  
  # PorTTY
  
  Browser-based terminal with shell access and optional session persistence.
</div>

## Quick Install

```bash
sudo bash -c "$(curl -fsSL https://raw.githubusercontent.com/PiTZE/PorTTY/master/install.sh)"
```

Then access your terminal at: http://your-server-ip:7314

## Features

- **Single binary** - No external dependencies
- **Browser terminal** - Access from any web browser
- **Default shell mode** - Uses your default shell (zsh, bash, etc.)
- **Optional tmux mode** - Session persistence with `--tmux` flag
- **Nerd Font support** - Proper shell prompt icon rendering
- **PWA support** - Install as native app
- **Auto-reconnection** - Handles connection drops gracefully

## Usage

```bash
# Start with default shell (recommended)
./portty run

# Start with tmux for session persistence
./portty run --tmux

# Start on specific address/port
./portty run 0.0.0.0:8080

# Stop the server
./portty stop

# Get help
./portty help
```

## Shell Modes

### Default Shell Mode (Primary)
- Uses your default shell directly
- Faster startup, no overhead
- Each connection is independent

### tmux Mode (Optional)
- Session persistence across connections
- Multiple browsers can share the same session
- Use `--tmux` flag to enable

## Building from Source

```bash
git clone https://github.com/PiTZE/PorTTY.git
cd PorTTY
./build.sh
```

## Security

⚠️ **PorTTY provides terminal access - secure it properly:**

- Default: localhost only
- Production: Use reverse proxy with HTTPS and authentication
- Never expose directly to internet

Example Nginx config:
```nginx
location / {
    auth_basic "Terminal Access";
    auth_basic_user_file /etc/nginx/.htpasswd;
    
    proxy_pass http://localhost:7314;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
}
```

## License

MIT License