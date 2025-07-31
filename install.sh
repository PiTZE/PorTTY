#!/bin/bash
set -e

# PorTTY Installer/Uninstaller
# This script can install, update, or uninstall PorTTY
# Supports both interactive and command-line argument modes

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
INTERACTIVE=true # Default to interactive mode
YES_FLAG=false # For non-interactive confirmation

# Function to display help
show_help() {
  echo "PorTTY Installer/Uninstaller v0.1"
  echo "Install, update, or remove PorTTY terminal server"
  echo ""
  echo "USAGE:"
  echo "  $0 [OPTIONS]"
  echo ""
  echo "OPTIONS:"
  echo "  -h, --help                 Show this help message"
  echo "  -v, --version              Display version information"
  echo "  -u, --uninstall            Uninstall PorTTY"
  echo "  -y, --yes                  Automatic yes to prompts (non-interactive mode)"
  echo "  -i, --interface INTERFACE  Specify interface to bind to (localhost or 0.0.0.0)"
  echo "                             Default: $DEFAULT_INTERFACE"
  echo "  -p, --port PORT            Specify port to listen on (1-65535)"
  echo "                             Default: $DEFAULT_PORT"
  echo "  -d, --directory DIR        Specify installation directory"
  echo "                             Default: $INSTALL_DIR"
  echo ""
  echo "EXAMPLES:"
  echo "  $0                         # Interactive installation"
  echo "  $0 -i localhost -p 7314    # Install with specific interface and port"
  echo "  $0 -u -y                   # Uninstall without confirmation prompt"
  echo "  $0 -i 0.0.0.0 -p 7314 -y   # Non-interactive installation with specific settings"
  echo "  $0 -d /opt/portty          # Install to custom directory"
  echo ""
  echo "For more information, visit: https://github.com/PiTZE/PorTTY"
  exit 0
}

# Function to display version
show_version() {
  echo "PorTTY Installer/Uninstaller v0.1"
  exit 0
}

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
while [[ $# -gt 0 ]]; do
  case $1 in
    -h|--help)
      show_help
      ;;
    -v|--version)
      show_version
      ;;
    -u|--uninstall)
      MODE="uninstall"
      shift
      ;;
    -y|--yes)
      YES_FLAG=true
      INTERACTIVE=false
      shift
      ;;
    -i|--interface)
      if [[ -n "$2" && "$2" != -* ]]; then
        DEFAULT_INTERFACE="$2"
        INTERFACE="$2"
        shift 2
      else
        echo -e "${RED}Error: Argument for $1 is missing${NC}"
        exit 1
      fi
      ;;
    -p|--port)
      if [[ -n "$2" && "$2" != -* ]]; then
        if [[ "$2" =~ ^[0-9]+$ ]] && [ "$2" -ge 1 ] && [ "$2" -le 65535 ]; then
          DEFAULT_PORT="$2"
          PORT="$2"
          shift 2
        else
          echo -e "${RED}Error: Invalid port number: $2${NC}"
          exit 1
        fi
      else
        echo -e "${RED}Error: Argument for $1 is missing${NC}"
        exit 1
      fi
      ;;
    -d|--directory)
      if [[ -n "$2" && "$2" != -* ]]; then
        INSTALL_DIR="$2"
        BINARY_FILE="$INSTALL_DIR/portty"
        shift 2
      else
        echo -e "${RED}Error: Argument for $1 is missing${NC}"
        exit 1
      fi
      ;;
    --)
      # End of options
      shift
      break
      ;;
    -*)
      echo -e "${RED}Error: Unknown option: $1${NC}"
      show_help
      ;;
    *)
      # Unknown positional argument
      echo -e "${RED}Error: Unknown argument: $1${NC}"
      show_help
      ;;
  esac
done

# Display header based on mode
if [ "$MODE" = "install" ]; then
  echo -e "${GREEN}PorTTY Installer${NC}"
  echo "This script will install PorTTY v0.1 and set it up as a systemd service."
  echo "Use '$0 --uninstall' to remove PorTTY from your system."
else
  echo -e "${YELLOW}PorTTY Uninstaller${NC}"
  echo "This will completely remove PorTTY from your system."
  
  if [ "$INTERACTIVE" = true ]; then
    read -p "Are you sure you want to uninstall PorTTY? [y/N] " confirm
    if [[ "$confirm" != [yY] && "$confirm" != [yY][eE][sS] ]]; then
      echo "Uninstall cancelled."
      exit 0
    fi
  else
    echo "Proceeding with uninstall (--yes flag provided)..."
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
      CURRENT_INTERFACE="${BASH_REMATCH[1]}"
      CURRENT_PORT="${BASH_REMATCH[2]}"
      
      # Only use current interface as default if not specified via command line
      if [ -z "$INTERFACE" ]; then
        DEFAULT_INTERFACE="${CURRENT_INTERFACE}"
      fi
      
      # Always show current configuration but keep 7314 as the default port
      echo "Current configuration: Interface=${CURRENT_INTERFACE}, Port=${CURRENT_PORT}"
    fi
  fi
fi

# Interactive configuration if needed
if [ "$INTERACTIVE" = true ]; then
  echo -e "${GREEN}PorTTY Configuration${NC}"
  echo "Please provide the following information (press Enter for default values):"

  # Ask for interface
  read -p "Interface to bind to [localhost or 0.0.0.0] (default: $DEFAULT_INTERFACE): " input_interface
  INTERFACE=${input_interface:-$DEFAULT_INTERFACE}

  # Ask for port
  read -p "Port to listen on (default: $DEFAULT_PORT): " input_port
  PORT=${input_port:-$DEFAULT_PORT}
else
  # Use values from command line or defaults
  INTERFACE=${INTERFACE:-$DEFAULT_INTERFACE}
  PORT=${PORT:-$DEFAULT_PORT}
  echo -e "${GREEN}Using non-interactive mode with provided arguments${NC}"
fi

# Validate interface
if [[ "$INTERFACE" != "localhost" && "$INTERFACE" != "0.0.0.0" && "$INTERFACE" != "127.0.0.1" ]]; then
  echo -e "${YELLOW}Warning: Unusual interface specified ($INTERFACE). Using it anyway, but please ensure it's correct.${NC}"
fi

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
  echo "Creating installation directory ($INSTALL_DIR)..."
  mkdir -p "$INSTALL_DIR"

  # Download PorTTY binary
  echo "Downloading PorTTY v0.1..."
  curl -L -o "$BINARY_FILE" "https://github.com/PiTZE/PorTTY/releases/download/v0.1/portty"
  chmod +x "$BINARY_FILE"
else
  # If installation directory has changed, move the binary
  if [ ! -f "$BINARY_FILE" ]; then
    echo "Moving PorTTY binary to new location ($INSTALL_DIR)..."
    mkdir -p "$INSTALL_DIR"
    # Find the existing binary
    EXISTING_BINARY=$(which portty 2>/dev/null || echo "/usr/local/bin/portty")
    if [ -f "$EXISTING_BINARY" ]; then
      cp "$EXISTING_BINARY" "$BINARY_FILE"
      chmod +x "$BINARY_FILE"
    else
      echo "Downloading PorTTY v0.1 to new location..."
      curl -L -o "$BINARY_FILE" "https://github.com/PiTZE/PorTTY/releases/download/v0.1/portty"
      chmod +x "$BINARY_FILE"
    fi
  else
    echo "Keeping existing PorTTY binary..."
  fi
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
ExecStart=$BINARY_FILE run $INTERFACE:$PORT
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
  echo "  $BINARY_FILE run $INTERFACE:$PORT"
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
echo "For automated deployments, you can use command-line options:"
echo "  sudo $0 -i localhost -p 8080 -y    # Non-interactive installation"
echo "  sudo $0 -u -y                      # Non-interactive uninstallation"
echo "  sudo $0 -h                         # Show all available options"
echo ""
echo "Enjoy using PorTTY!"