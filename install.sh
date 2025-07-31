#!/bin/bash
set -e

# PorTTY Installer/Uninstaller
# This script can install, update, or uninstall PorTTY

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
DEFAULT_PORT="7314"
DEFAULT_INTERFACE="0.0.0.0"
INSTALL_DIR="/usr/local/bin"
SERVICE_FILE="/etc/systemd/system/portty.service"
BINARY_FILE="$INSTALL_DIR/portty"
IS_UPDATE=false
MODE="install" # Default mode is install

# Function to uninstall PorTTY
uninstall_portty() {
  echo -e "${YELLOW}Uninstalling PorTTY...${NC}"
  
  # Check if systemd service exists
  if [ -f "$SERVICE_FILE" ]; then
    echo "Stopping and removing PorTTY service..."
    systemctl stop portty.service 2>/dev/null || true
    systemctl disable portty.service 2>/dev/null || true
    rm -f "$SERVICE_FILE"
    systemctl daemon-reload
    echo "PorTTY service has been removed."
  else
    echo "No PorTTY service found."
  fi
  
  # Remove binary
  if [ -f "$BINARY_FILE" ]; then
    echo "Removing PorTTY binary..."
    rm -f "$BINARY_FILE"
    echo "PorTTY binary has been removed."
  else
    echo "No PorTTY binary found at $BINARY_FILE."
  fi
  
  echo -e "${GREEN}PorTTY has been completely uninstalled from your system.${NC}"
  exit 0
}

# Parse command line arguments
if [ "$1" = "--uninstall" ] || [ "$1" = "-u" ]; then
  MODE="uninstall"
fi

# Display header based on mode
if [ "$MODE" = "install" ]; then
  echo -e "${GREEN}PorTTY Installer${NC}"
  echo "This script will install PorTTY v0.1 and set it up as a systemd service."
  echo "Use '$0 --uninstall' to remove PorTTY from your system."
else
  echo -e "${YELLOW}PorTTY Uninstaller${NC}"
  echo "This will completely remove PorTTY from your system."
  read -p "Are you sure you want to uninstall PorTTY? [y/N] " confirm
  if [[ "$confirm" != [yY] && "$confirm" != [yY][eE][sS] ]]; then
    echo "Uninstall cancelled."
    exit 0
  fi
  uninstall_portty
fi

echo ""

# Check if running as root
if [ "$EUID" -ne 0 ]; then
  echo -e "${YELLOW}Warning: Not running as root. Installation may fail.${NC}"
  echo "Consider running with sudo if you encounter permission issues."
  echo ""
fi

# Check if tmux is installed
if ! command -v tmux &> /dev/null; then
  echo -e "${RED}Error: tmux is not installed.${NC}"
  echo "Please install tmux first:"
  echo "  - Debian/Ubuntu: sudo apt-get install tmux"
  echo "  - CentOS/RHEL: sudo yum install tmux"
  echo "  - macOS: brew install tmux"
  exit 1
fi

# Check if PorTTY is already installed
if [ -f "$SERVICE_FILE" ]; then
  echo -e "${BLUE}PorTTY service is already installed.${NC}"
  echo "This script will update your configuration."
  IS_UPDATE=true
  
  # Extract current settings if possible
  if grep -q "ExecStart=" "$SERVICE_FILE"; then
    CURRENT_CONFIG=$(grep "ExecStart=" "$SERVICE_FILE" | sed 's/ExecStart=.*portty run //')
    if [[ "$CURRENT_CONFIG" =~ ([^:]+):([0-9]+) ]]; then
      DEFAULT_INTERFACE="${BASH_REMATCH[1]}"
      DEFAULT_PORT="${BASH_REMATCH[2]}"
      echo "Current configuration: Interface=$DEFAULT_INTERFACE, Port=$DEFAULT_PORT"
    fi
  fi
fi

# Interactive configuration
echo -e "${GREEN}PorTTY Configuration${NC}"
echo "Please provide the following information (press Enter for default values):"

# Ask for interface
read -p "Interface to bind to [localhost or 0.0.0.0] (default: $DEFAULT_INTERFACE): " INTERFACE
INTERFACE=${INTERFACE:-$DEFAULT_INTERFACE}

# Validate interface
if [[ "$INTERFACE" != "localhost" && "$INTERFACE" != "0.0.0.0" && "$INTERFACE" != "127.0.0.1" ]]; then
  echo -e "${YELLOW}Warning: Unusual interface specified. Using it anyway, but please ensure it's correct.${NC}"
fi

# Ask for port
read -p "Port to listen on (default: $DEFAULT_PORT): " PORT
PORT=${PORT:-$DEFAULT_PORT}

# Validate port
if ! [[ "$PORT" =~ ^[0-9]+$ ]] || [ "$PORT" -lt 1 ] || [ "$PORT" -gt 65535 ]; then
  echo -e "${RED}Error: Invalid port number. Using default port $DEFAULT_PORT.${NC}"
  PORT=$DEFAULT_PORT
fi

echo ""
echo "Using configuration: Interface=$INTERFACE, Port=$PORT"
echo ""

# Create installation directory if it doesn't exist
if [ "$IS_UPDATE" = false ]; then
  echo "Creating installation directory..."
  mkdir -p "$INSTALL_DIR"

  # Download PorTTY binary
  echo "Downloading PorTTY v0.1..."
  curl -L -o "$INSTALL_DIR/portty" "https://github.com/PiTZE/PorTTY/releases/download/v0.1/portty"
  chmod +x "$INSTALL_DIR/portty"
else
  echo "Keeping existing PorTTY binary..."
fi

# Create or update systemd service file
if [ -d "/etc/systemd/system" ]; then
  if [ "$IS_UPDATE" = true ]; then
    echo "Updating systemd service configuration..."
  else
    echo "Creating systemd service..."
  fi
  
  cat > "$SERVICE_FILE" << EOF
[Unit]
Description=PorTTY Terminal Server
After=network.target

[Service]
ExecStart=$INSTALL_DIR/portty run $INTERFACE:$PORT
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

  # Reload systemd, enable and start the service
  echo "Reloading systemd and restarting PorTTY service..."
  systemctl daemon-reload
  
  if [ "$IS_UPDATE" = false ]; then
    systemctl enable portty.service
  fi
  
  # Restart the service to apply changes
  systemctl restart portty.service
  
  # Check if service started successfully
  if systemctl is-active --quiet portty.service; then
    echo -e "${GREEN}PorTTY service is running!${NC}"
  else
    echo -e "${RED}Warning: PorTTY service failed to start. Check logs with: sudo journalctl -u portty${NC}"
  fi
  
  echo -e "${GREEN}PorTTY has been installed as a systemd service!${NC}"
  echo "You can manage it with:"
  echo "  - Start: sudo systemctl start portty"
  echo "  - Stop: sudo systemctl stop portty"
  echo "  - Status: sudo systemctl status portty"
  echo "  - Logs: sudo journalctl -u portty"
  echo "  - Restart: sudo systemctl restart portty"
else
  echo -e "${YELLOW}Systemd not detected. Installing as standalone binary only.${NC}"
  echo "You can start PorTTY manually with:"
  echo "  $INSTALL_DIR/portty run $INTERFACE:$PORT"
fi

echo ""
echo -e "${GREEN}Installation complete!${NC}"
echo "PorTTY is now available at: http://$INTERFACE:$PORT"
if [ "$INTERFACE" = "0.0.0.0" ]; then
  echo "You can also access it at: http://$(hostname -I | awk '{print $1}'):$PORT"
fi
echo ""
echo "If you're running a firewall, make sure port $PORT is open:"
echo "  - UFW: sudo ufw allow $PORT/tcp"
echo "  - FirewallD: sudo firewall-cmd --permanent --add-port=$PORT/tcp && sudo firewall-cmd --reload"
echo ""
echo "To uninstall PorTTY in the future, run:"
echo "  sudo $0 --uninstall"
echo ""
echo "Enjoy using PorTTY!"