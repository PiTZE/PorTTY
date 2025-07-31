#!/bin/bash
set -e

# PorTTY Installer
# This script downloads and installs PorTTY v0.1 as a systemd service

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}PorTTY Installer${NC}"
echo "This script will install PorTTY v0.1 and set it up as a systemd service."
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

# Create installation directory
INSTALL_DIR="/usr/local/bin"
echo "Creating installation directory..."
mkdir -p "$INSTALL_DIR"

# Download PorTTY binary
echo "Downloading PorTTY v0.1..."
curl -L -o "$INSTALL_DIR/portty" "https://github.com/PiTZE/PorTTY/releases/download/v0.1/portty"
chmod +x "$INSTALL_DIR/portty"

# Create systemd service file
if [ -d "/etc/systemd/system" ]; then
  echo "Creating systemd service..."
  cat > /etc/systemd/system/portty.service << EOF
[Unit]
Description=PorTTY Terminal Server
After=network.target

[Service]
ExecStart=$INSTALL_DIR/portty run 0.0.0.0:7314
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

  # Reload systemd, enable and start the service
  echo "Enabling and starting PorTTY service..."
  systemctl daemon-reload
  systemctl enable portty.service
  systemctl start portty.service
  
  echo -e "${GREEN}PorTTY has been installed as a systemd service!${NC}"
  echo "You can manage it with:"
  echo "  - Start: sudo systemctl start portty"
  echo "  - Stop: sudo systemctl stop portty"
  echo "  - Status: sudo systemctl status portty"
  echo "  - Logs: sudo journalctl -u portty"
else
  echo -e "${YELLOW}Systemd not detected. Installing as standalone binary only.${NC}"
  echo "You can start PorTTY manually with:"
  echo "  $INSTALL_DIR/portty run"
fi

echo ""
echo -e "${GREEN}Installation complete!${NC}"
echo "PorTTY is now available at: http://$(hostname -I | awk '{print $1}'):7314"
echo ""
echo "If you're running a firewall, make sure port 7314 is open:"
echo "  - UFW: sudo ufw allow 7314/tcp"
echo "  - FirewallD: sudo firewall-cmd --permanent --add-port=7314/tcp && sudo firewall-cmd --reload"
echo ""
echo "Enjoy using PorTTY!"