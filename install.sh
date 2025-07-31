#!/bin/bash
# PorTTY Application Manager
# Unified script for installation, configuration, and runtime management
set -e

# Determine script name for help messages
if [[ "$0" == *"bash" ]]; then
  SCRIPT_NAME="portty-installer"
else
  SCRIPT_NAME=$(basename "$0")
fi

# Trap signals for graceful shutdown
trap 'cleanup EXIT' EXIT
trap 'cleanup SIGINT' SIGINT
trap 'cleanup SIGTERM' SIGTERM

# Temporary files and background processes
TMP_FILES=""
BG_PIDS=""
PID_FILE="./.portty-install.pid"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m'

# Default configuration
DEFAULT_PORT="7314"
DEFAULT_INTERFACE="0.0.0.0"
INSTALL_DIR="/usr/local/bin"
SERVICE_FILE="/etc/systemd/system/portty.service"
BINARY_FILE="$INSTALL_DIR/portty"
LOG_DIR="/var/log/portty"
INSTALL_LOG="$LOG_DIR/install.log"
RUNTIME_LOG="$LOG_DIR/portty.log"

# Script state
MODE="install"  # Default mode
INTERACTIVE=true
YES_FLAG=false
FORCE_UPDATE=false
VERBOSE=false

# Log level constants
LOG_LEVEL_DEBUG=0
LOG_LEVEL_INFO=1
LOG_LEVEL_WARNING=2
LOG_LEVEL_ERROR=3
LOG_LEVEL_FATAL=4

# Default log level
CURRENT_LOG_LEVEL=${PORTTY_LOG_LEVEL:-1}  # Default to INFO

# Convert log level string to numeric value
get_log_level() {
    local level_str="$1"
    case "${level_str^^}" in
        "DEBUG") echo $LOG_LEVEL_DEBUG ;;
        "INFO") echo $LOG_LEVEL_INFO ;;
        "WARNING") echo $LOG_LEVEL_WARNING ;;
        "ERROR") echo $LOG_LEVEL_ERROR ;;
        "FATAL") echo $LOG_LEVEL_FATAL ;;
        *) echo $LOG_LEVEL_INFO ;;
    esac
}

# Set current log level from environment variable if provided
if [ -n "$PORTTY_LOG_LEVEL" ]; then
    CURRENT_LOG_LEVEL=$(get_log_level "$PORTTY_LOG_LEVEL")
fi

# Enhanced logging functions with log levels
log_with_level() {
    local level="$1"
    local level_name="$2"
    local color="$3"
    local message="$4"
    local log_file="$5"
    
    # Only log if current level is less than or equal to the message level
    if [ "$CURRENT_LOG_LEVEL" -le "$level" ]; then
        local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
        echo -e "${color}[${timestamp}] [${level_name}]${NC} ${message}" | tee -a "$log_file"
    fi
}

log_debug() {
    log_with_level $LOG_LEVEL_DEBUG "DEBUG" "$BLUE" "$1" "$RUNTIME_LOG"
    return 0
}

log_info() {
    log_with_level $LOG_LEVEL_INFO "INFO" "$GREEN" "$1" "$RUNTIME_LOG"
    return 0
}

log_warning() {
    log_with_level $LOG_LEVEL_WARNING "WARNING" "$YELLOW" "$1" "$RUNTIME_LOG"
    return 0
}

log_error() {
    log_with_level $LOG_LEVEL_ERROR "ERROR" "$RED" "$1" "$RUNTIME_LOG"
    return 1
}

log_fatal() {
    log_with_level $LOG_LEVEL_FATAL "FATAL" "$RED" "$1" "$RUNTIME_LOG"
    return 1
}

log_step() {
    log_with_level $LOG_LEVEL_INFO "STEP" "$BLUE" "${BOLD}$1${NC}" "$RUNTIME_LOG"
    return 0
}

log_install() {
    log_with_level $LOG_LEVEL_INFO "INSTALL" "$GREEN" "$1" "$INSTALL_LOG"
    return 0
}

log_install_step() {
    log_with_level $LOG_LEVEL_INFO "INSTALL" "$BLUE" "${BOLD}$1${NC}" "$INSTALL_LOG"
    return 0
}

log_install_error() {
    log_with_level $LOG_LEVEL_ERROR "INSTALL" "$RED" "$1" "$INSTALL_LOG"
    return 1
}

# Ensure directories exist
ensure_directories() {
    mkdir -p "$LOG_DIR"
    touch "$RUNTIME_LOG" "$INSTALL_LOG"
    if [ "$VERBOSE" = true ]; then
        log_info "Log files: $RUNTIME_LOG, $INSTALL_LOG"
    fi
}

# Cleanup function for graceful shutdown
cleanup() {
    local exit_code=${1:-0}
    
    # Convert signal name to exit code if needed
    case "$exit_code" in
        "SIGINT") exit_code=130 ;;
        "SIGTERM") exit_code=143 ;;
        "EXIT") exit_code=0 ;;
    esac
    
    # Clean up temporary files
    if [ -n "$TMP_FILES" ]; then
        log_debug "Cleaning up temporary files: $TMP_FILES"
        rm -f $TMP_FILES
    fi
    
    # Kill background processes
    if [ -n "$BG_PIDS" ]; then
        log_debug "Terminating background processes: $BG_PIDS"
        for pid in $BG_PIDS; do
            if kill -0 $pid 2>/dev/null; then
                log_debug "Killing process $pid"
                kill $pid 2>/dev/null || true
            fi
        done
    fi
    
    # Remove PID file if it exists
    if [ -f "$PID_FILE" ]; then
        log_debug "Removing PID file: $PID_FILE"
        rm -f "$PID_FILE"
    fi
    
    if [ $exit_code -ne 0 ]; then
        log_error "Script terminated with errors (exit code: $exit_code)"
    else
        log_debug "Cleanup completed successfully"
    fi
    
    # Only exit if this is a direct call, not from the EXIT trap
    if [ "$1" != "EXIT" ]; then
        exit $exit_code
    fi
}

# Check if running as root
check_root() {
    if [ "$EUID" -ne 0 ]; then
        log_warning "Not running as root. Some operations may fail."
        log_info "Consider running with sudo for full functionality."
        return 1
    fi
    return 0
}

# Check if tmux is installed
check_tmux() {
    if ! command -v tmux &> /dev/null; then
        log_error "tmux is not installed."
        log_info "Please install tmux first:"
        echo "  - Debian/Ubuntu: sudo apt-get install tmux"
        echo "  - CentOS/RHEL: sudo yum install tmux"
        echo "  - macOS: brew install tmux"
        return 1
    fi
    return 0
}

# Function to display help
show_help() {
    echo -e "${BOLD}PorTTY Application Manager${NC}"
    echo "Unified script for installation, configuration, and runtime management"
    echo
    echo -e "${BOLD}USAGE:${NC}"
    echo "  $SCRIPT_NAME [COMMAND] [OPTIONS]"
    echo
    echo -e "${BOLD}COMMANDS:${NC}"
    echo "  install                    Install PorTTY (default)"
    echo "  uninstall                  Remove PorTTY completely"
    echo "  update                     Update PorTTY installation"
    echo "  status                     Check PorTTY status"
    echo "  start                      Start PorTTY service"
    echo "  stop                       Stop PorTTY service"
    echo "  restart                    Restart PorTTY service"
    echo "  enable                     Enable PorTTY service at boot"
    echo "  disable                    Disable PorTTY service at boot"
    echo "  logs                       Show PorTTY logs"
    echo "  config                     Show current configuration"
    echo
    echo -e "${BOLD}OPTIONS:${NC}"
    echo "  -h, --help                 Show this help message"
    echo "  -v, --version              Display version information"
    echo "  -y, --yes                  Automatic yes to prompts (non-interactive)"
    echo "  -f, --force                Force operation (skip checks)"
    echo "  -i, --interface INTERFACE  Specify interface to bind to"
    echo "                             Default: $DEFAULT_INTERFACE"
    echo "  -p, --port PORT            Specify port to listen on (1-65535)"
    echo "                             Default: $DEFAULT_PORT"
    echo "  -d, --directory DIR        Specify installation directory"
    echo "                             Default: $INSTALL_DIR"
    echo "  --verbose                  Enable verbose logging"
    echo "  --debug                    Enable debug logging"
    echo
    echo -e "${BOLD}EXAMPLES:${NC}"
    echo "  $SCRIPT_NAME                         # Interactive installation"
    echo "  $SCRIPT_NAME install -i localhost -p 8080  # Install with specific settings"
    echo "  $SCRIPT_NAME uninstall -y            # Uninstall without confirmation"
    echo "  $SCRIPT_NAME status                  # Check service status"
    echo "  $SCRIPT_NAME start                   # Start the service"
    echo "  $SCRIPT_NAME logs                    # View application logs"
    echo "  sudo $SCRIPT_NAME install -i 0.0.0.0 -p 7314 -y  # Non-interactive install"
    echo
    echo -e "${BOLD}INSTALLATION METHODS:${NC}"
    echo "  Direct download: curl -sSL https://raw.githubusercontent.com/PiTZE/PorTTY/master/install.sh > install.sh"
    echo "  Then run: chmod +x install.sh && ./install.sh [OPTIONS]"
    echo "  Or pipe to bash: curl -sSL https://raw.githubusercontent.com/PiTZE/PorTTY/master/install.sh | bash -s -- [OPTIONS]"
    echo
    echo -e "${BOLD}INTERACTIVE MODE:${NC}"
    echo "  Run without any commands or options for interactive menu"
    echo
    echo "For more information, visit: https://github.com/PiTZE/PorTTY"
    exit 0
}

# Function to display version
show_version() {
    echo "PorTTY Application Manager v0.1"
    echo "PorTTY: A lightweight, web-based terminal emulator powered by tmux"
    echo "https://github.com/PiTZE/PorTTY"
    exit 0
}

# Download PorTTY binary
download_binary() {
    local version="v0.1"
    local url="https://github.com/PiTZE/PorTTY/releases/download/${version}/portty"
    
    log_install_step "Downloading PorTTY ${version}..."
    
    if [ -f "$BINARY_FILE" ] && [ "$FORCE_UPDATE" = false ]; then
        log_install "Binary already exists, skipping download"
        return 0
    fi
    
    # Create installation directory if it doesn't exist
    mkdir -p "$INSTALL_DIR"
    
    # Download with retry logic
    local max_attempts=3
    local attempt=1
    
    while [ $attempt -le $max_attempts ]; do
        if curl -L -o "$BINARY_FILE.tmp" "$url"; then
            mv "$BINARY_FILE.tmp" "$BINARY_FILE"
            chmod +x "$BINARY_FILE"
            log_install "PorTTY binary downloaded successfully ✓"
            return 0
        else
            log_install_error "Download attempt $attempt failed"
            if [ $attempt -lt $max_attempts ]; then
                log_install "Retrying in 2 seconds..."
                sleep 2
            fi
        fi
        ((attempt++))
    done
    
    log_install_error "Failed to download PorTTY binary after $max_attempts attempts"
    return 1
}

# Create systemd service file
create_service_file() {
    local interface="$1"
    local port="$2"
    
    log_install_step "Creating systemd service file..."
    
    cat > "$SERVICE_FILE" << EOF
[Unit]
Description=PorTTY Terminal Server
After=network.target
[Service]
Type=simple
User=root
WorkingDirectory=$INSTALL_DIR
ExecStart=$BINARY_FILE run $interface:$port
Restart=always
RestartSec=5
StandardOutput=append:$RUNTIME_LOG
StandardError=append:$RUNTIME_LOG
[Install]
WantedBy=multi-user.target
EOF
    if [ $? -eq 0 ]; then
        log_install "Service file created successfully ✓"
        return 0
    else
        log_install_error "Failed to create service file"
        return 1
    fi
}

# Install PorTTY
install_portty() {
    local interface="${1:-$DEFAULT_INTERFACE}"
    local port="${2:-$DEFAULT_PORT}"
    
    log_install_step "Starting PorTTY installation..."
    
    # Check prerequisites
    check_tmux || return 1
    
    # Download binary
    download_binary || return 1
    
    # Create service file
    create_service_file "$interface" "$port" || return 1
    
    # Reload systemd and enable service
    systemctl daemon-reload
    systemctl enable portty.service
    
    # Start the service
    systemctl start portty.service
    
    # Check if service started successfully
    if systemctl is-active --quiet portty.service; then
        log_install "PorTTY service is running! ✓"
        log_install "Installation completed successfully ✓"
        return 0
    else
        log_install_error "PorTTY service failed to start"
        log_install_error "Check logs with: journalctl -u portty"
        return 1
    fi
}

# Uninstall PorTTY
uninstall_portty() {
    log_step "Uninstalling PorTTY..."
    
    # Stop and disable service if it exists
    if [ -f "$SERVICE_FILE" ]; then
        log_info "Stopping and removing PorTTY service..."
        systemctl stop portty.service 2>/dev/null || true
        systemctl disable portty.service 2>/dev/null || true
        rm -f "$SERVICE_FILE"
        systemctl daemon-reload
        log_info "PorTTY service has been removed."
    else
        log_info "No PorTTY service found."
    fi
    
    # Remove binary
    if [ -f "$BINARY_FILE" ]; then
        log_info "Removing PorTTY binary..."
        rm -f "$BINARY_FILE"
        log_info "PorTTY binary has been removed."
    else
        log_info "No PorTTY binary found at $BINARY_FILE."
    fi
    
    # Remove logs
    if [ -d "$LOG_DIR" ]; then
        if [ "$YES_FLAG" = true ]; then
            log_info "Removing log files..."
            rm -rf "$LOG_DIR"
        else
            read -p "Remove log files? [y/N]: " remove_logs
            if [[ "$remove_logs" =~ ^[Yy]$ ]]; then
                rm -rf "$LOG_DIR"
                log_info "Log files removed."
            else
                log_info "Log files kept in $LOG_DIR"
            fi
        fi
    fi
    
    log_info "PorTTY has been completely uninstalled from your system."
    return 0
}

# Update PorTTY
update_portty() {
    log_step "Updating PorTTY..."
    
    # Check if PorTTY is installed
    if [ ! -f "$BINARY_FILE" ]; then
        log_error "PorTTY is not installed. Use 'install' command first."
        return 1
    fi
    
    # Stop the service
    systemctl stop portty.service 2>/dev/null || true
    
    # Get current configuration
    local current_config=""
    if [ -f "$SERVICE_FILE" ]; then
        current_config=$(grep "ExecStart=" "$SERVICE_FILE" | sed 's/ExecStart=.*portty run //')
    fi
    
    # Download new binary
    FORCE_UPDATE=true
    download_binary || return 1
    
    # Restart service
    systemctl start portty.service
    
    if systemctl is-active --quiet portty.service; then
        log_info "PorTTY updated successfully ✓"
        return 0
    else
        log_error "Failed to restart PorTTY after update"
        return 1
    fi
}

# Check PorTTY status
check_status() {
    log_step "Checking PorTTY status..."
    
    echo -e "${BOLD}System Information:${NC}"
    echo "  OS: $(uname -s) $(uname -r)"
    echo "  Architecture: $(uname -m)"
    echo "  Current User: $(whoami)"
    echo
    
    echo -e "${BOLD}PorTTY Status:${NC}"
    
    # Check binary
    if [ -f "$BINARY_FILE" ]; then
        echo "  Binary: ✓ Installed at $BINARY_FILE"
        local binary_version=$("$BINARY_FILE" --version 2>/dev/null | head -1)
        if [ -n "$binary_version" ]; then
            echo "  Version: $binary_version"
        fi
    else
        echo "  Binary: ✗ Not found"
    fi
    
    # Check service
    if [ -f "$SERVICE_FILE" ]; then
        echo "  Service: ✓ Installed"
        
        if systemctl is-enabled --quiet portty.service; then
            echo "  Enabled: ✓ Yes"
        else
            echo "  Enabled: ✗ No"
        fi
        
        if systemctl is-active --quiet portty.service; then
            echo "  Status: ✓ Running"
            local pid=$(systemctl show --property MainPID --value portty.service)
            echo "  PID: $pid"
            
            # Show listening port
            local port_info=$(ss -tulpn | grep ":$DEFAULT_PORT" | head -1)
            if [ -n "$port_info" ]; then
                echo "  Listening: $port_info"
            fi
        else
            echo "  Status: ✗ Not running"
        fi
    else
        echo "  Service: ✗ Not installed"
    fi
    
    # Check tmux
    if command -v tmux &> /dev/null; then
        echo "  tmux: ✓ Installed"
    else
        echo "  tmux: ✗ Not installed"
    fi
    
    return 0
}

# Service management functions
start_service() {
    log_step "Starting PorTTY service..."
    
    if [ ! -f "$SERVICE_FILE" ]; then
        log_error "Service not installed. Use 'install' command first."
        return 1
    fi
    
    systemctl start portty.service
    
    if systemctl is-active --quiet portty.service; then
        log_info "PorTTY service started successfully ✓"
        return 0
    else
        log_error "Failed to start PorTTY service"
        return 1
    fi
}

stop_service() {
    log_step "Stopping PorTTY service..."
    
    if [ ! -f "$SERVICE_FILE" ]; then
        log_warning "Service not installed."
        return 0
    fi
    
    systemctl stop portty.service
    
    if ! systemctl is-active --quiet portty.service; then
        log_info "PorTTY service stopped successfully ✓"
        return 0
    else
        log_error "Failed to stop PorTTY service"
        return 1
    fi
}

restart_service() {
    log_step "Restarting PorTTY service..."
    
    if [ ! -f "$SERVICE_FILE" ]; then
        log_error "Service not installed. Use 'install' command first."
        return 1
    fi
    
    systemctl restart portty.service
    
    if systemctl is-active --quiet portty.service; then
        log_info "PorTTY service restarted successfully ✓"
        return 0
    else
        log_error "Failed to restart PorTTY service"
        return 1
    fi
}

enable_service() {
    log_step "Enabling PorTTY service at boot..."
    
    if [ ! -f "$SERVICE_FILE" ]; then
        log_error "Service not installed. Use 'install' command first."
        return 1
    fi
    
    systemctl enable portty.service
    
    if systemctl is-enabled --quiet portty.service; then
        log_info "PorTTY service enabled at boot ✓"
        return 0
    else
        log_error "Failed to enable PorTTY service"
        return 1
    fi
}

disable_service() {
    log_step "Disabling PorTTY service at boot..."
    
    if [ ! -f "$SERVICE_FILE" ]; then
        log_warning "Service not installed."
        return 0
    fi
    
    systemctl disable portty.service
    
    if ! systemctl is-enabled --quiet portty.service; then
        log_info "PorTTY service disabled at boot ✓"
        return 0
    else
        log_error "Failed to disable PorTTY service"
        return 1
    fi
}

show_logs() {
    log_step "Showing PorTTY logs..."
    
    if [ -f "$RUNTIME_LOG" ]; then
        echo -e "${BOLD}Recent logs:${NC}"
        echo "================"
        tail -20 "$RUNTIME_LOG"
    else
        log_info "No log file found at $RUNTIME_LOG"
    fi
    
    echo
    echo -e "${BOLD}Systemd logs:${NC}"
    echo "==============="
    journalctl -u portty.service --no-pager -n 20
}

show_config() {
    log_step "Showing current configuration..."
    
    if [ -f "$SERVICE_FILE" ]; then
        echo -e "${BOLD}Service Configuration:${NC}"
        echo "======================"
        grep -E "(ExecStart|User|WorkingDirectory)" "$SERVICE_FILE"
        
        echo
        echo -e "${BOLD}Current Settings:${NC}"
        echo "================="
        local exec_line=$(grep "ExecStart=" "$SERVICE_FILE")
        if [[ "$exec_line" =~ run[[:space:]]+([^:]+):([0-9]+) ]]; then
            echo "Interface: ${BASH_REMATCH[1]}"
            echo "Port: ${BASH_REMATCH[2]}"
        fi
    else
        log_info "No service configuration found. PorTTY may not be installed."
    fi
    
    echo
    echo -e "${BOLD}Default Settings:${NC}"
    echo "=================="
    echo "Default Interface: $DEFAULT_INTERFACE"
    echo "Default Port: $DEFAULT_PORT"
    echo "Install Directory: $INSTALL_DIR"
    echo "Log Directory: $LOG_DIR"
}

# Interactive menu functions
show_main_menu() {
    while true; do
        clear
        echo -e "${BOLD}PorTTY Application Manager${NC}"
        echo "================================="
        echo
        echo "1. Installation & Setup"
        echo "2. Service Management"
        echo "3. Status & Information"
        echo "4. Logs & Configuration"
        echo "5. Exit"
        echo
        read -p "Enter your choice (1-5): " choice
        
        case $choice in
            1) installation_menu ;;
            2) service_menu ;;
            3) 
                check_status
                read -p "Press Enter to continue..."
                ;;
            4) logs_config_menu ;;
            5) return ;;
            *)
                echo -e "${RED}Invalid choice. Please try again.${NC}"
                read -p "Press Enter to continue..."
                ;;
        esac
    done
}

installation_menu() {
    while true; do
        clear
        echo -e "${BOLD}Installation & Setup${NC}"
        echo "======================="
        echo
        echo "1. Install PorTTY"
        echo "2. Update PorTTY"
        echo "3. Uninstall PorTTY"
        echo "4. Back to main menu"
        echo
        read -p "Enter your choice (1-4): " choice
        
        case $choice in
            1)
                read -p "Interface [$DEFAULT_INTERFACE]: " interface
                interface=${interface:-$DEFAULT_INTERFACE}
                
                read -p "Port [$DEFAULT_PORT]: " port
                port=${port:-$DEFAULT_PORT}
                
                if install_portty "$interface" "$port"; then
                    echo -e "${GREEN}Installation successful!${NC}"
                else
                    echo -e "${RED}Installation failed!${NC}"
                fi
                read -p "Press Enter to continue..."
                ;;
            2)
                if update_portty; then
                    echo -e "${GREEN}Update successful!${NC}"
                else
                    echo -e "${RED}Update failed!${NC}"
                fi
                read -p "Press Enter to continue..."
                ;;
            3)
                read -p "Are you sure you want to uninstall PorTTY? [y/N]: " confirm
                if [[ "$confirm" =~ ^[Yy]$ ]]; then
                    if uninstall_portty; then
                        echo -e "${GREEN}Uninstallation successful!${NC}"
                    else
                        echo -e "${RED}Uninstallation failed!${NC}"
                    fi
                else
                    echo "Uninstallation cancelled."
                fi
                read -p "Press Enter to continue..."
                ;;
            4) return ;;
            *)
                echo -e "${RED}Invalid choice. Please try again.${NC}"
                read -p "Press Enter to continue..."
                ;;
        esac
    done
}

service_menu() {
    while true; do
        clear
        echo -e "${BOLD}Service Management${NC}"
        echo "=================="
        echo
        echo "1. Start service"
        echo "2. Stop service"
        echo "3. Restart service"
        echo "4. Enable service at boot"
        echo "5. Disable service at boot"
        echo "6. Back to main menu"
        echo
        read -p "Enter your choice (1-6): " choice
        
        case $choice in
            1)
                if start_service; then
                    echo -e "${GREEN}Service started successfully!${NC}"
                else
                    echo -e "${RED}Failed to start service!${NC}"
                fi
                read -p "Press Enter to continue..."
                ;;
            2)
                if stop_service; then
                    echo -e "${GREEN}Service stopped successfully!${NC}"
                else
                    echo -e "${RED}Failed to stop service!${NC}"
                fi
                read -p "Press Enter to continue..."
                ;;
            3)
                if restart_service; then
                    echo -e "${GREEN}Service restarted successfully!${NC}"
                else
                    echo -e "${RED}Failed to restart service!${NC}"
                fi
                read -p "Press Enter to continue..."
                ;;
            4)
                if enable_service; then
                    echo -e "${GREEN}Service enabled at boot!${NC}"
                else
                    echo -e "${RED}Failed to enable service!${NC}"
                fi
                read -p "Press Enter to continue..."
                ;;
            5)
                if disable_service; then
                    echo -e "${GREEN}Service disabled at boot!${NC}"
                else
                    echo -e "${RED}Failed to disable service!${NC}"
                fi
                read -p "Press Enter to continue..."
                ;;
            6) return ;;
            *)
                echo -e "${RED}Invalid choice. Please try again.${NC}"
                read -p "Press Enter to continue..."
                ;;
        esac
    done
}

logs_config_menu() {
    while true; do
        clear
        echo -e "${BOLD}Logs & Configuration${NC}"
        echo "======================="
        echo
        echo "1. Show logs"
        echo "2. Show configuration"
        echo "3. Back to main menu"
        echo
        read -p "Enter your choice (1-3): " choice
        
        case $choice in
            1)
                show_logs
                read -p "Press Enter to continue..."
                ;;
            2)
                show_config
                read -p "Press Enter to continue..."
                ;;
            3) return ;;
            *)
                echo -e "${RED}Invalid choice. Please try again.${NC}"
                read -p "Press Enter to continue..."
                ;;
        esac
    done
}

# Parse command line arguments
parse_arguments() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_help
                ;;
            -v|--version)
                show_version
                ;;
            -y|--yes)
                YES_FLAG=true
                INTERACTIVE=false
                shift
                ;;
            -f|--force)
                FORCE_UPDATE=true
                shift
                ;;
            -i|--interface)
                if [[ -n "$2" && "$2" != -* ]]; then
                    DEFAULT_INTERFACE="$2"
                    shift 2
                else
                    echo "Error: Argument for $1 is missing" >&2
                    exit 1
                fi
                ;;
            -p|--port)
                if [[ -n "$2" && "$2" != -* ]]; then
                    if [[ "$2" =~ ^[0-9]+$ ]] && [ "$2" -ge 1 ] && [ "$2" -le 65535 ]; then
                        DEFAULT_PORT="$2"
                        shift 2
                    else
                        echo "Error: Invalid port number: $2" >&2
                        exit 1
                    fi
                else
                    echo "Error: Argument for $1 is missing" >&2
                    exit 1
                fi
                ;;
            -d|--directory)
                if [[ -n "$2" && "$2" != -* ]]; then
                    INSTALL_DIR="$2"
                    BINARY_FILE="$INSTALL_DIR/portty"
                    shift 2
                else
                    echo "Error: Argument for $1 is missing" >&2
                    exit 1
                fi
                ;;
            --verbose)
                VERBOSE=true
                CURRENT_LOG_LEVEL=$LOG_LEVEL_INFO
                shift
                ;;
            --debug)
                CURRENT_LOG_LEVEL=$LOG_LEVEL_DEBUG
                shift
                ;;
            install|uninstall|update|status|start|stop|restart|enable|disable|logs|config)
                MODE="$1"
                shift
                ;;
            --)
                shift
                break
                ;;
            -*)
                echo "Error: Unknown option: $1" >&2
                show_help
                ;;
            *)
                echo "Error: Unknown argument: $1" >&2
                show_help
                ;;
        esac
    done
}

# Main script logic
main() {
    # Parse arguments first to handle --help and --version without side effects
    parse_arguments "$@"
    
    # Save the script PID for cleanup
    echo $$ > "$PID_FILE"
    
    # Ensure directories exist only after we know we're not just showing help/version
    ensure_directories
    
    # If no command specified, default to interactive mode
    if [ "$MODE" = "install" ] && [ $# -eq 0 ]; then
        INTERACTIVE=true
    fi
    
    # Execute based on mode
    case "$MODE" in
        install)
            if [ "$INTERACTIVE" = true ]; then
                show_main_menu
            else
                log_step "Installing PorTTY..."
                if check_root && check_tmux && install_portty "$DEFAULT_INTERFACE" "$DEFAULT_PORT"; then
                    log_info "PorTTY installed successfully!"
                    echo "Access PorTTY at: http://$DEFAULT_INTERFACE:$DEFAULT_PORT"
                else
                    log_error "Installation failed!"
                    exit 1
                fi
            fi
            ;;
        uninstall)
            if [ "$INTERACTIVE" = true ] && [ "$YES_FLAG" = false ]; then
                read -p "Are you sure you want to uninstall PorTTY? [y/N]: " confirm
                if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
                    echo "Uninstallation cancelled."
                    exit 0
                fi
            fi
            if check_root && uninstall_portty; then
                log_info "PorTTY uninstalled successfully!"
            else
                log_error "Uninstallation failed!"
                exit 1
            fi
            ;;
        update)
            if check_root && update_portty; then
                log_info "PorTTY updated successfully!"
            else
                log_error "Update failed!"
                exit 1
            fi
            ;;
        status)
            check_status
            ;;
        start)
            if check_root && start_service; then
                log_info "PorTTY service started successfully!"
            else
                log_error "Failed to start service!"
                exit 1
            fi
            ;;
        stop)
            if check_root && stop_service; then
                log_info "PorTTY service stopped successfully!"
            else
                log_error "Failed to stop service!"
                exit 1
            fi
            ;;
        restart)
            if check_root && restart_service; then
                log_info "PorTTY service restarted successfully!"
            else
                log_error "Failed to restart service!"
                exit 1
            fi
            ;;
        enable)
            if check_root && enable_service; then
                log_info "PorTTY service enabled at boot!"
            else
                log_error "Failed to enable service!"
                exit 1
            fi
            ;;
        disable)
            if check_root && disable_service; then
                log_info "PorTTY service disabled at boot!"
            else
                log_error "Failed to disable service!"
                exit 1
            fi
            ;;
        logs)
            show_logs
            ;;
        config)
            show_config
            ;;
        *)
            # Interactive mode
            show_main_menu
            ;;
    esac
}

# Run main function with all arguments
main "$@"