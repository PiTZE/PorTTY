#!/usr/bin/env bash
set -e

# ============================================================================
# SCRIPT-LEVEL VARIABLES
# ============================================================================

if [[ "$0" == *"bash" ]]; then
    SCRIPT_NAME="portty-installer"
else
    SCRIPT_NAME=$(basename "$0")
fi

detect_interactive_terminal() {
    if [ -t 0 ]; then
        if [ -n "$TERM" ] && [ "$TERM" != "dumb" ]; then
            if [ -n "$PS1" ] || [ -n "$BASH_VERSION" ] || [ -n "$ZSH_VERSION" ]; then
                return 0
            fi
        fi
        return 0
    fi
    
    if [ -n "$CI" ] || [ -n "$GITHUB_ACTIONS" ] || [ -n "$JENKINS_URL" ]; then
        return 1
    fi
    
    if [ ! -t 1 ] || [ ! -t 2 ]; then
        return 1
    fi
    
    return 1
}

IS_INTERACTIVE_TERMINAL=true

SKIP_TMUX_CHECK=false
USE_TMUX=false
USER_INSTALL=false
SKIP_CHECKSUM=false

TMP_FILES=""
BG_PIDS=""
if [ -w "/var/run" ] && [ -d "/var/run" ]; then
    PID_FILE="/var/run/portty-install-$$.pid"
elif [ -w "/tmp" ]; then
    PID_FILE="/tmp/portty-install-$$.pid"
else
    PID_FILE="${HOME}/.portty-install-$$.pid"
fi

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m'

DEFAULT_PORT="7314"
DEFAULT_INTERFACE="0.0.0.0"

INSTALL_DIR="/usr/local/bin"
SERVICE_FILE="/etc/systemd/system/portty.service"
BINARY_FILE="$INSTALL_DIR/portty"
LOG_DIR="/var/log/portty"
INSTALL_LOG="$LOG_DIR/install.log"
RUNTIME_LOG="$LOG_DIR/portty.log"
SYSTEMCTL_CMD="systemctl"

MODE="install"
INTERACTIVE=true
YES_FLAG=false
FORCE_UPDATE=false
VERBOSE=false

LOG_LEVEL_DEBUG=0
LOG_LEVEL_INFO=1
LOG_LEVEL_WARNING=2
LOG_LEVEL_ERROR=3
LOG_LEVEL_FATAL=4

CURRENT_LOG_LEVEL=${PORTTY_LOG_LEVEL:-1}

# ============================================================================
# SIGNAL HANDLING AND CLEANUP
# ============================================================================

trap 'cleanup EXIT' EXIT
trap 'cleanup SIGINT' SIGINT
trap 'cleanup SIGTERM' SIGTERM

cleanup() {
    local exit_code=${1:-0}
    local cleanup_context="${2:-general cleanup}"
    
    case "$exit_code" in
        "SIGINT")
            exit_code=130
            cleanup_context="interrupted by user (Ctrl+C)"
            ;;
        "SIGTERM")
            exit_code=143
            cleanup_context="terminated by system signal"
            ;;
        "EXIT")
            exit_code=0
            cleanup_context="normal script completion"
            ;;
    esac
    
    log_debug "Starting cleanup: $cleanup_context"
    
    if [ -n "$TMP_FILES" ]; then
        log_debug "Cleaning up temporary files and directories: $TMP_FILES"
        local cleanup_failed=false
        for tmp_item in $TMP_FILES; do
            if [ -f "$tmp_item" ]; then
                if ! rm -f "$tmp_item" 2>/dev/null; then
                    log_warning "Failed to remove temporary file: $tmp_item"
                    cleanup_failed=true
                else
                    log_debug "Removed temporary file: $tmp_item"
                fi
            elif [ -d "$tmp_item" ]; then
                if ! rm -rf "$tmp_item" 2>/dev/null; then
                    log_warning "Failed to remove temporary directory: $tmp_item"
                    cleanup_failed=true
                else
                    log_debug "Removed temporary directory: $tmp_item"
                fi
            fi
        done
        
        if [ "$cleanup_failed" = true ]; then
            log_warning "Some temporary files/directories could not be removed - manual cleanup may be needed"
        fi
    fi
    
    if [ -n "$BG_PIDS" ]; then
        log_debug "Terminating background processes: $BG_PIDS"
        for pid in $BG_PIDS; do
            if kill -0 "$pid" 2>/dev/null; then
                log_debug "Terminating process $pid"
                if ! kill "$pid" 2>/dev/null; then
                    log_debug "Failed to terminate process $pid gracefully, trying SIGKILL"
                    kill -9 "$pid" 2>/dev/null || true
                fi
                sleep 0.5
                if kill -0 "$pid" 2>/dev/null; then
                    log_warning "Process $pid may still be running after cleanup"
                fi
            fi
        done
    fi
    
    if [ -f "$PID_FILE" ]; then
        if ! rm -f "$PID_FILE" 2>/dev/null; then
            log_warning "Failed to remove PID file: $PID_FILE"
        else
            log_debug "Removed PID file: $PID_FILE"
        fi
    fi
    
    if [ $exit_code -ne 0 ]; then
        log_error "Script terminated with errors (exit code: $exit_code)"
        log_error "Context: $cleanup_context"
        
        case $exit_code in
            1)
                log_info "💡 General error - check the error messages above for specific issues"
                ;;
            130)
                log_info "💡 Script was interrupted - you can safely run it again"
                ;;
            143)
                log_info "💡 Script was terminated - check system resources and try again"
                ;;
            *)
                log_info "💡 Unexpected error - check logs and system status"
                ;;
        esac
        
        if [ -f "$INSTALL_LOG" ]; then
            log_info "💡 Check installation log: $INSTALL_LOG"
        fi
        if [ -f "$RUNTIME_LOG" ]; then
            log_info "💡 Check runtime log: $RUNTIME_LOG"
        fi
    else
        log_debug "Cleanup completed successfully: $cleanup_context"
    fi
    
    if [ "$1" != "EXIT" ]; then
        exit $exit_code
    fi
}

# ============================================================================
# INPUT VALIDATION FUNCTIONS
# ============================================================================

validate_port() {
    local port="$1"
    local context="${2:-port}"
    
    if [ -z "$port" ]; then
        log_error_with_context "Port cannot be empty" "$context validation" "Provide a port number between 1-65535"
        return 1
    fi
    
    if ! [[ "$port" =~ ^[0-9]+$ ]]; then
        log_error_with_context "Invalid port format: '$port'" "$context validation" "Port must be a number between 1-65535"
        return 1
    fi
    
    if [ "$port" -lt 1 ] || [ "$port" -gt 65535 ]; then
        log_error_with_context "Port out of range: $port" "$context validation" "Port must be between 1-65535"
        return 1
    fi
    
    # Check for commonly restricted ports (warn but don't fail)
    if [ "$port" -lt 1024 ] && [ "$EUID" -ne 0 ]; then
        log_warning "Port $port requires root privileges (ports < 1024)"
        log_info "Consider using a port >= 1024 or run with sudo"
    fi
    
    return 0
}

validate_interface() {
    local interface="$1"
    local context="${2:-interface}"
    
    if [ -z "$interface" ]; then
        log_error_with_context "Interface cannot be empty" "$context validation" "Use 'localhost', '0.0.0.0', or a specific IP address"
        return 1
    fi
    
    # Basic validation for common interface formats
    case "$interface" in
        "localhost"|"127.0.0.1"|"::1")
            log_debug "Local interface detected: $interface"
            ;;
        "0.0.0.0"|"::"|"*")
            log_debug "All interfaces binding detected: $interface"
            ;;
        [0-9]*.[0-9]*.[0-9]*.[0-9]*)
            # Basic IPv4 validation
            if [[ "$interface" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]]; then
                log_debug "IPv4 interface detected: $interface"
            else
                log_warning "Interface format may be invalid: $interface"
            fi
            ;;
        *:*)
            # Likely IPv6
            log_debug "IPv6 interface detected: $interface"
            ;;
        *)
            # Hostname or other format
            log_debug "Hostname interface detected: $interface"
            ;;
    esac
    
    return 0
}

validate_directory() {
    local dir="$1"
    local context="${2:-directory}"
    local create_if_missing="${3:-false}"
    
    if [ -z "$dir" ]; then
        log_error_with_context "Directory path cannot be empty" "$context validation" "Provide a valid directory path"
        return 1
    fi
    
    # Check if directory exists
    if [ ! -d "$dir" ]; then
        if [ "$create_if_missing" = "true" ]; then
            if ! mkdir -p "$dir" 2>/dev/null; then
                log_error_with_context "Cannot create directory: $dir" "$context validation" "Check parent directory permissions or choose a different path"
                return 1
            fi
            log_info "Created directory: $dir"
        else
            log_error_with_context "Directory does not exist: $dir" "$context validation" "Create the directory first or choose an existing path"
            return 1
        fi
    fi
    
    # Check if directory is writable
    if [ ! -w "$dir" ]; then
        log_error_with_context "Directory is not writable: $dir" "$context validation" "Check directory permissions or run with appropriate privileges"
        return 1
    fi
    
    return 0
}

validate_yes_no_input() {
    local input="$1"
    local context="${2:-user input}"
    
    case "${input,,}" in
        "y"|"yes"|"true"|"1")
            return 0
            ;;
        "n"|"no"|"false"|"0"|"")
            return 1
            ;;
        *)
            log_warning "Invalid input '$input' for $context. Assuming 'no'."
            return 1
            ;;
    esac
}

# ============================================================================
# UTILITY FUNCTIONS
# ============================================================================

get_log_level() {
    local level_str="$1"
    case "${level_str^^}" in
        "DEBUG") echo "$LOG_LEVEL_DEBUG" ;;
        "INFO") echo "$LOG_LEVEL_INFO" ;;
        "WARNING") echo "$LOG_LEVEL_WARNING" ;;
        "ERROR") echo "$LOG_LEVEL_ERROR" ;;
        "FATAL") echo "$LOG_LEVEL_FATAL" ;;
        *) echo "$LOG_LEVEL_INFO" ;;
    esac
}

ensure_directories() {
    local log_dir_created=false
    local fallback_log_dir=""
    local error_context=""
    
    if [ -w "$(dirname "$LOG_DIR")" ] || mkdir -p "$LOG_DIR" 2>/dev/null; then
        if [ -w "$LOG_DIR" ]; then
            log_dir_created=true
        else
            error_context="Default log directory exists but is not writable: $LOG_DIR"
        fi
    else
        error_context="Cannot create or access default log directory: $LOG_DIR"
    fi
    
    if [ "$log_dir_created" = false ]; then
        if [ -n "$HOME" ] && [ -w "$HOME" ]; then
            fallback_log_dir="$HOME/.portty/logs"
            if mkdir -p "$fallback_log_dir" 2>/dev/null; then
                LOG_DIR="$fallback_log_dir"
                INSTALL_LOG="$LOG_DIR/install.log"
                RUNTIME_LOG="$LOG_DIR/portty.log"
                log_dir_created=true
                echo "Info: Using fallback log directory: $LOG_DIR" >&2
            else
                error_context="$error_context; Cannot create user log directory: $fallback_log_dir"
            fi
        else
            error_context="$error_context; HOME directory not available or not writable"
        fi
    fi
    
    if [ "$log_dir_created" = false ]; then
        fallback_log_dir="/tmp/portty-logs-$$"
        if mkdir -p "$fallback_log_dir" 2>/dev/null; then
            LOG_DIR="$fallback_log_dir"
            INSTALL_LOG="$LOG_DIR/install.log"
            RUNTIME_LOG="$LOG_DIR/portty.log"
            log_dir_created=true
            echo "Warning: Using temporary log directory: $LOG_DIR" >&2
        else
            error_context="$error_context; Cannot create temporary log directory: $fallback_log_dir"
        fi
    fi
    
    if [ "$log_dir_created" = false ]; then
        echo "Error: Could not create any log directory." >&2
        echo "Context: $error_context" >&2
        echo "Suggestion: Check filesystem permissions or run with appropriate privileges" >&2
        return 1
    fi
    
    if ! touch "$RUNTIME_LOG" "$INSTALL_LOG" 2>/dev/null; then
        echo "Warning: Could not create log files in $LOG_DIR" >&2
        echo "Suggestion: Check directory permissions: ls -la $LOG_DIR" >&2
        return 1
    fi
    
    if [ "$VERBOSE" = true ]; then
        echo "Info: Log files created: $RUNTIME_LOG, $INSTALL_LOG" >&2
    fi
    
    return 0
}

require_root() {
    if [ "$EUID" -ne 0 ]; then
        log_error "This operation requires root privileges."
        log_info "Please run with sudo or use --user flag for user-local installation:"
        log_info "  System installation: sudo $SCRIPT_NAME $MODE"
        log_info "  User installation:   $SCRIPT_NAME $MODE --user"
        exit 1
    fi
}

check_root() {
    if [ "$USER_INSTALL" = true ]; then
        log_debug "User installation mode - root not required"
        return 0
    fi
    
    if [ "$EUID" -ne 0 ]; then
        log_warning "Not running as root. Some operations may fail."
        log_info "Consider running with sudo for system installation or use --user flag:"
        log_info "  System installation: sudo $SCRIPT_NAME $MODE"
        log_info "  User installation:   $SCRIPT_NAME $MODE --user"
        return 1
    fi
    return 0
}

setup_installation_paths() {
    if [ "$USER_INSTALL" = true ]; then
        # User installation paths
        INSTALL_DIR="$HOME/.local/bin"
        SERVICE_FILE="$HOME/.config/systemd/user/portty.service"
        BINARY_FILE="$INSTALL_DIR/portty"
        LOG_DIR="$HOME/.local/share/portty/logs"
        INSTALL_LOG="$LOG_DIR/install.log"
        RUNTIME_LOG="$LOG_DIR/portty.log"
        SYSTEMCTL_CMD="systemctl --user"
    else
        # System installation paths (already set as defaults)
        # No changes needed for system paths
        true
    fi
}

log_installation_paths() {
    if [ "$USER_INSTALL" = true ]; then
        log_debug "User installation paths configured:"
        log_debug "  Install dir: $INSTALL_DIR"
        log_debug "  Service file: $SERVICE_FILE"
        log_debug "  Log dir: $LOG_DIR"
    else
        log_debug "System installation paths configured:"
        log_debug "  Install dir: $INSTALL_DIR"
        log_debug "  Service file: $SERVICE_FILE"
        log_debug "  Log dir: $LOG_DIR"
    fi
}

# ============================================================================
# NETWORK CONNECTIVITY FUNCTIONS
# ============================================================================

check_network_connectivity() {
    local test_url="${1:-https://api.github.com}"
    local timeout="${2:-10}"
    local context="${3:-network connectivity}"
    
    log_debug "Testing network connectivity to $test_url"
    
    local connectivity_ok=false
    local error_details=""
    
    if command -v curl >/dev/null 2>&1; then
        if curl -s --connect-timeout "$timeout" --max-time "$timeout" --head "$test_url" >/dev/null 2>&1; then
            connectivity_ok=true
            log_debug "Network connectivity confirmed via curl"
        else
            error_details="curl failed to connect to $test_url"
        fi
    fi
    
    if [ "$connectivity_ok" = false ] && command -v wget >/dev/null 2>&1; then
        if wget -q --timeout="$timeout" --spider "$test_url" >/dev/null 2>&1; then
            connectivity_ok=true
            log_debug "Network connectivity confirmed via wget"
        else
            error_details="$error_details; wget failed to connect to $test_url"
        fi
    fi
    
    if [ "$connectivity_ok" = false ]; then
        local hostname
        hostname=$(echo "$test_url" | sed 's|https\?://||' | cut -d'/' -f1)
        if command -v nslookup >/dev/null 2>&1; then
            if nslookup "$hostname" >/dev/null 2>&1; then
                log_warning "DNS resolution works but HTTP connection failed"
                error_details="$error_details; DNS works but HTTP connection failed"
            else
                error_details="$error_details; DNS resolution failed for $hostname"
            fi
        fi
    fi
    
    if [ "$connectivity_ok" = false ]; then
        log_error_with_context "Network connectivity check failed" "$context" "Check internet connection and firewall settings"
        log_debug "Error details: $error_details"
        return 1
    fi
    
    return 0
}

check_github_api_access() {
    local timeout="${1:-10}"
    
    log_debug "Checking GitHub API access for version detection"
    
    if ! check_network_connectivity "https://api.github.com" "$timeout" "GitHub API access"; then
        log_warning "Cannot access GitHub API - will use fallback version"
        return 1
    fi
    
    return 0
}

# ============================================================================
# DEPENDENCY CHECKING FUNCTIONS
# ============================================================================

check_required_commands() {
    local missing_commands=""
    local optional_commands=""
    local critical_missing=false
    
    log_debug "Checking required system commands"
    
    local required_cmds="uname mkdir chmod touch tar"
    for cmd in $required_cmds; do
        if ! command -v "$cmd" >/dev/null 2>&1; then
            missing_commands="$missing_commands $cmd"
            critical_missing=true
        fi
    done
    
    local download_available=false
    if command -v curl >/dev/null 2>&1; then
        download_available=true
        log_debug "curl available for downloads"
    elif command -v wget >/dev/null 2>&1; then
        download_available=true
        log_debug "wget available for downloads"
    else
        missing_commands="$missing_commands curl-or-wget"
        critical_missing=true
    fi
    
    if ! command -v sha256sum >/dev/null 2>&1 && ! command -v shasum >/dev/null 2>&1; then
        optional_commands="$optional_commands sha256sum-or-shasum"
        log_warning "No SHA256 checksum tool found - checksums will be skipped"
    fi
    
    if ! command -v systemctl >/dev/null 2>&1; then
        optional_commands="$optional_commands systemctl"
        log_info "systemctl not found - service management will be limited"
    fi
    
    if [ "$critical_missing" = true ]; then
        log_fatal_with_context "Critical system commands missing:$missing_commands" "dependency check" "Install missing commands using your system package manager"
        return 1
    fi
    
    if [ -n "$optional_commands" ]; then
        log_info "Optional commands missing:$optional_commands"
        log_info "Some features may be limited but installation can continue"
    fi
    
    return 0
}

check_tmux() {
    if ! command -v tmux &> /dev/null; then
        log_warning "tmux is not installed."
        log_info "PorTTY v0.2+ supports dual shell modes:"
        log_info "  • Default Shell Mode: Direct shell access (current mode)"
        log_info "  • tmux Mode: Session persistence and multi-client support"
        echo
        log_info "For optimal experience with session persistence, install tmux:"
        echo "  - Debian/Ubuntu: sudo apt install tmux"
        echo "  - CentOS/RHEL: sudo yum install tmux"
        echo "  - Fedora: sudo dnf install tmux"
        echo "  - Arch Linux: sudo pacman -S tmux"
        echo "  - macOS: brew install tmux"
        echo "  - Alpine: apk add tmux"
        echo
        log_info "Installation will continue with default shell mode."
        return 0
    fi
    
    local tmux_version
    tmux_version=$(tmux -V 2>/dev/null)
    if [ $? -eq 0 ]; then
        log_info "tmux is installed and functional: $tmux_version"
        log_info "Session persistence available"
    else
        log_warning "tmux is installed but may not be functional"
        log_info "Installation will continue with default shell mode"
    fi
    
    return 0
}

check_system_resources() {
    log_debug "Checking system resources"
    
    local install_dir_parent
    install_dir_parent=$(dirname "$INSTALL_DIR")
    if [ -d "$install_dir_parent" ]; then
        local available_space
        available_space=$(df "$install_dir_parent" 2>/dev/null | awk 'NR==2 {print $4}')
        if [ -n "$available_space" ] && [ "$available_space" -lt 10240 ]; then
            log_warning "Low disk space in $install_dir_parent: ${available_space}KB available"
            log_info "PorTTY binary is typically 5-15MB - ensure sufficient space"
        fi
    fi
    
    if command -v ss >/dev/null 2>&1; then
        if ss -tuln | grep -q ":$DEFAULT_PORT "; then
            log_warning "Port $DEFAULT_PORT appears to be in use"
            log_info "You may need to stop the existing service or choose a different port"
        fi
    elif command -v netstat >/dev/null 2>&1; then
        if netstat -tuln 2>/dev/null | grep -q ":$DEFAULT_PORT "; then
            log_warning "Port $DEFAULT_PORT appears to be in use"
            log_info "You may need to stop the existing service or choose a different port"
        fi
    fi
    
    return 0
}

# ============================================================================
# LOGGING FUNCTIONS
# ============================================================================

if [ -n "$PORTTY_LOG_LEVEL" ]; then
    CURRENT_LOG_LEVEL=$(get_log_level "$PORTTY_LOG_LEVEL")
fi

log_with_level() {
    local level="$1"
    local level_name="$2"
    local color="$3"
    local message="$4"
    local log_file="$5"
    
    if [ "$CURRENT_LOG_LEVEL" -le "$level" ]; then
        local timestamp
        timestamp=$(date '+%Y-%m-%d %H:%M:%S')
        echo -e "${color}[${timestamp}] [${level_name}]${NC} ${message}" | tee -a "$log_file"
    fi
}

log_debug() {
    log_with_level "$LOG_LEVEL_DEBUG" "DEBUG" "$BLUE" "$1" "$RUNTIME_LOG"
    return 0
}

log_info() {
    log_with_level "$LOG_LEVEL_INFO" "INFO" "$GREEN" "$1" "$RUNTIME_LOG"
    return 0
}

log_warning() {
    log_with_level "$LOG_LEVEL_WARNING" "WARNING" "$YELLOW" "$1" "$RUNTIME_LOG"
    return 0
}

log_error() {
    log_with_level "$LOG_LEVEL_ERROR" "ERROR" "$RED" "$1" "$RUNTIME_LOG"
    return 1
}

log_fatal() {
    log_with_level "$LOG_LEVEL_FATAL" "FATAL" "$RED" "$1" "$RUNTIME_LOG"
    return 1
}

log_error_with_context() {
    local error_msg="$1"
    local context="$2"
    local recovery_suggestion="$3"
    
    log_error "$error_msg"
    if [ -n "$context" ]; then
        log_error "Context: $context"
    fi
    if [ -n "$recovery_suggestion" ]; then
        log_info "💡 Suggestion: $recovery_suggestion"
    fi
    return 1
}

log_fatal_with_context() {
    local error_msg="$1"
    local context="$2"
    local recovery_suggestion="$3"
    
    log_fatal "$error_msg"
    if [ -n "$context" ]; then
        log_fatal "Context: $context"
    fi
    if [ -n "$recovery_suggestion" ]; then
        log_info "💡 Recovery: $recovery_suggestion"
    fi
    return 1
}

log_step() {
    log_with_level "$LOG_LEVEL_INFO" "STEP" "$BLUE" "${BOLD}$1${NC}" "$RUNTIME_LOG"
    return 0
}

log_install() {
    log_with_level "$LOG_LEVEL_INFO" "INSTALL" "$GREEN" "$1" "$INSTALL_LOG"
    return 0
}

log_install_step() {
    log_with_level "$LOG_LEVEL_INFO" "INSTALL" "$BLUE" "${BOLD}$1${NC}" "$INSTALL_LOG"
    return 0
}

log_install_error() {
    log_with_level "$LOG_LEVEL_ERROR" "INSTALL" "$RED" "$1" "$INSTALL_LOG"
    return 1
}

# ============================================================================
# HELP AND VERSION FUNCTIONS
# ============================================================================

show_help() {
    echo -e "${BOLD}PorTTY Application Manager${NC}"
    echo "Unified script for installation, configuration, and runtime management"
    echo
    echo -e "${BOLD}USAGE:${NC}"
    echo "  $SCRIPT_NAME [COMMAND] [OPTIONS]"
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
    echo "  --user                     Install for current user only (no root required)"
    echo "  --tmux                     Enable tmux mode for session persistence"
    echo "  --force                    Force operation (skip checks)"
    echo "  --skip-checksum            Skip SHA256 checksum verification (not recommended)"
    echo "  --verbose                  Enable verbose logging"
    echo "  --debug                    Enable debug logging"
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
    echo -e "${BOLD}SHELL MODES:${NC}"
    echo "  PorTTY v0.2+ supports dual shell modes:"
    echo "  • Default Shell Mode: Direct shell access (primary mode)"
    echo "  • tmux Mode: Session persistence and multi-client support (optional)"
    echo "  Use --tmux flag to enable tmux mode during installation"
    echo
    echo -e "${BOLD}INSTALLATION MODES:${NC}"
    echo "  System Installation (default):"
    echo "    • Requires root privileges (sudo)"
    echo "    • Installs to /usr/local/bin"
    echo "    • Creates system service in /etc/systemd/system"
    echo "    • Logs to /var/log/portty"
    echo "    • Available to all users"
    echo
    echo "  User Installation (--user flag):"
    echo "    • No root privileges required"
    echo "    • Installs to ~/.local/bin"
    echo "    • Creates user service in ~/.config/systemd/user"
    echo "    • Logs to ~/.local/share/portty/logs"
    echo "    • Available only to current user"
    echo
    echo -e "${BOLD}EXAMPLES:${NC}"
    echo "  $SCRIPT_NAME                         # Interactive installation"
    echo "  $SCRIPT_NAME install --user          # User installation (no sudo needed)"
    echo "  $SCRIPT_NAME install --tmux          # System install with tmux mode"
    echo "  $SCRIPT_NAME install --user --tmux   # User install with tmux mode"
    echo "  $SCRIPT_NAME install -i localhost -p 8080  # Install with specific settings"
    echo "  sudo $SCRIPT_NAME install --tmux -i 0.0.0.0 -p 7314 -y  # System install, non-interactive"
    echo "  $SCRIPT_NAME uninstall --user -y     # Uninstall user installation"
    echo "  $SCRIPT_NAME status --user           # Check user installation status"
    echo "  $SCRIPT_NAME start --user            # Start user service"
    echo "  $SCRIPT_NAME logs --user             # View user installation logs"
    echo
    echo -e "${BOLD}INSTALLATION METHODS:${NC}"
    echo "  Direct download: curl -sSL https://raw.githubusercontent.com/PiTZE/PorTTY/master/install.sh > install.sh"
    echo "  System install: chmod +x install.sh && sudo ./install.sh [OPTIONS]"
    echo "  User install:   chmod +x install.sh && ./install.sh install --user [OPTIONS]"
    echo "  Or pipe to bash: curl -sSL https://raw.githubusercontent.com/PiTZE/PorTTY/master/install.sh | bash -s -- [OPTIONS]"
    echo
    echo -e "${BOLD}INTERACTIVE MODE:${NC}"
    echo "  Run without any commands or options for interactive menu"
    echo
    echo "For more information, visit: https://github.com/PiTZE/PorTTY"
    exit 0
}

show_version() {
    echo "PorTTY Application Manager v0.2+"
    echo "PorTTY: A lightweight, web-based terminal emulator with dual shell modes"
    echo "  • Default Shell Mode: Direct shell access (primary)"
    echo "  • tmux Mode: Session persistence and multi-client support (optional)"
    echo "https://github.com/PiTZE/PorTTY"
    exit 0
}

# ============================================================================
# PLATFORM DETECTION FUNCTIONS
# ============================================================================

detect_platform() {
    local os=""
    local arch=""
    local platform=""
    
    log_debug "Detecting system platform..."
    
    local uname_os
    uname_os=$(uname -s 2>/dev/null)
    case "$uname_os" in
        "Linux")
            os="linux"
            ;;
        "Darwin")
            os="darwin"
            ;;
        *)
            log_warning "Unknown or unsupported OS: $uname_os"
            return 1
            ;;
    esac
    
    local uname_arch
    uname_arch=$(uname -m 2>/dev/null)
    case "$uname_arch" in
        "x86_64"|"amd64")
            arch="amd64"
            ;;
        "aarch64"|"arm64")
            arch="arm64"
            ;;
        "armv7l"|"armv6l"|"arm")
            arch="arm"
            ;;
        *)
            log_warning "Unknown or unsupported architecture: $uname_arch"
            return 1
            ;;
    esac
    
    platform="${os}-${arch}"
    log_debug "Detected platform: $platform (OS: $os, Architecture: $arch)"
    
    DETECTED_OS="$os"
    DETECTED_ARCH="$arch"
    DETECTED_PLATFORM="$platform"
    
    return 0
}

# ============================================================================
# VERSION DETECTION FUNCTIONS
# ============================================================================

get_latest_version() {
    local api_url="https://api.github.com/repos/PiTZE/PorTTY/releases/latest"
    local fallback_version="v0.2"
    local timeout=10
    
    log_debug "Attempting to fetch latest version from GitHub API..."
    
    local latest_version
    if command -v curl &> /dev/null; then
        latest_version=$(curl -s --connect-timeout "$timeout" --max-time "$timeout" "$api_url" 2>/dev/null | \
            grep '"tag_name":' | \
            sed -E 's/.*"tag_name":[[:space:]]*"([^"]+)".*/\1/')
    elif command -v wget &> /dev/null; then
        latest_version=$(wget -q --timeout="$timeout" -O - "$api_url" 2>/dev/null | \
            grep '"tag_name":' | \
            sed -E 's/.*"tag_name":[[:space:]]*"([^"]+)".*/\1/')
    else
        log_warning "Neither curl nor wget found. Using fallback version."
        echo "$fallback_version"
        return 0
    fi
    
    if [[ -n "$latest_version" && "$latest_version" =~ ^v[0-9]+\.[0-9]+(\.[0-9]+)?(-.*)?$ ]]; then
        log_debug "Successfully detected latest version: $latest_version"
        echo "$latest_version"
    else
        log_warning "Failed to detect latest version or invalid format. Using fallback version: $fallback_version"
        echo "$fallback_version"
    fi
}

# ============================================================================
# INSTALLATION FUNCTIONS
# ============================================================================

verify_checksum() {
    local binary_file="$1"
    local version="$2"
    local binary_name="$3"
    
    if [ "$SKIP_CHECKSUM" = true ]; then
        log_install "⚠ Checksum verification skipped (--skip-checksum flag used)"
        return 0
    fi
    
    log_install_step "Verifying SHA256 checksum..."
    
    local checksum_cmd=""
    if command -v sha256sum >/dev/null 2>&1; then
        checksum_cmd="sha256sum"
    elif command -v shasum >/dev/null 2>&1; then
        checksum_cmd="shasum -a 256"
    else
        log_install "⚠ No SHA256 checksum tool found (sha256sum or shasum)"
        log_install "⚠ Skipping checksum verification - install sha256sum or shasum for security"
        return 0
    fi
    
    local checksums_url="https://github.com/PiTZE/PorTTY/releases/download/${version}/checksums.txt"
    local checksums_file
    if command -v mktemp >/dev/null 2>&1; then
        checksums_file=$(mktemp -t "portty.checksums.XXXXXX")
    else
        checksums_file="/tmp/portty.checksums.tmp.$$"
    fi
    
    TMP_FILES="$TMP_FILES $checksums_file"
    
    log_install "Downloading checksums file..."
    if ! curl -L -s -o "$checksums_file" "$checksums_url" 2>/dev/null; then
        log_install "⚠ Failed to download checksums file from: $checksums_url"
        log_install "⚠ Skipping checksum verification - checksums file not available"
        return 0
    fi
    
    if [ ! -s "$checksums_file" ]; then
        log_install "⚠ Checksums file is empty or invalid"
        log_install "⚠ Skipping checksum verification"
        return 0
    fi
    
    local expected_checksum=""
    local checksum_line=""
    
    checksum_line=$(grep -E "^[a-fA-F0-9]{64}[[:space:]]+${binary_name}$" "$checksums_file" 2>/dev/null | head -1)
    
    if [ -n "$checksum_line" ]; then
        expected_checksum=$(echo "$checksum_line" | awk '{print $1}')
        log_install "Found checksum for ${binary_name}: ${expected_checksum:0:16}..."
    else
        log_install "⚠ No checksum found for binary: $binary_name"
        log_install "⚠ Skipping checksum verification"
        return 0
    fi
    
    log_install "Calculating SHA256 checksum of downloaded binary..."
    local actual_checksum
    actual_checksum=$($checksum_cmd "$binary_file" 2>/dev/null | awk '{print $1}')
    
    if [ -z "$actual_checksum" ]; then
        log_install_error "Failed to calculate checksum of downloaded binary"
        return 1
    fi
    
    if [ "${actual_checksum,,}" = "${expected_checksum,,}" ]; then
        log_install "✓ Checksum verification passed"
        log_install "  Expected: ${expected_checksum:0:16}..."
        log_install "  Actual:   ${actual_checksum:0:16}..."
        return 0
    else
        log_install_error "✗ Checksum verification failed!"
        log_install_error "  Expected: ${expected_checksum}"
        log_install_error "  Actual:   ${actual_checksum}"
        log_install_error "  Binary may be corrupted or tampered with"
        return 1
    fi
}

download_binary() {
    local version
    version=$(get_latest_version)
    
    local platform_archive=""
    local fallback_archive="portty-${version}-linux-amd64.tar.gz"
    local download_url=""
    local platform_detected=false
    
    log_install_step "Downloading PorTTY ${version}..."
    
    if ! check_network_connectivity "https://github.com" 15 "GitHub download access"; then
        log_install_error "Cannot access GitHub for downloads"
        log_install "💡 Troubleshooting steps:"
        log_install "  1. Check internet connection: ping 8.8.8.8"
        log_install "  2. Check DNS resolution: nslookup github.com"
        log_install "  3. Check firewall settings for HTTPS traffic"
        log_install "  4. Try again later if GitHub is experiencing issues"
        return 1
    fi
    
    if [ -f "$BINARY_FILE" ] && [ "$FORCE_UPDATE" = false ]; then
        log_install "Binary already exists, skipping download"
        return 0
    fi
    
    if detect_platform; then
        platform_archive="portty-${version}-${DETECTED_PLATFORM}.tar.gz"
        platform_detected=true
        log_install "Detected platform: ${DETECTED_PLATFORM}"
        log_install "Will attempt platform-specific archive: ${platform_archive}"
    else
        log_install "Platform detection failed, will use generic archive"
        platform_detected=false
    fi
    
    if ! validate_directory "$INSTALL_DIR" "install directory" "true"; then
        log_install_error "Cannot use install directory: $INSTALL_DIR"
        return 1
    fi
    
    local temp_archive
    local temp_extract_dir
    if command -v mktemp >/dev/null 2>&1; then
        temp_archive=$(mktemp -t "portty.archive.XXXXXX")
        temp_extract_dir=$(mktemp -d -t "portty.extract.XXXXXX")
    else
        temp_archive="/tmp/portty.archive.tmp.$$"
        temp_extract_dir="/tmp/portty.extract.tmp.$$"
        mkdir -p "$temp_extract_dir"
    fi
    
    if [ ! -w "$(dirname "$temp_archive")" ]; then
        log_install_error "Cannot write to temporary directory: $(dirname "$temp_archive")"
        log_install "Falling back to /tmp for temporary files"
        temp_archive="/tmp/portty.archive.tmp.$$"
        temp_extract_dir="/tmp/portty.extract.tmp.$$"
        mkdir -p "$temp_extract_dir"
    fi
    
    TMP_FILES="$TMP_FILES $temp_archive $temp_extract_dir"
    
    local max_attempts=3
    local base_delay=2
    local download_success=false
    local last_error=""
    
    download_with_retry() {
        local url="$1"
        local archive_name="$2"
        local attempt=1
        
        log_install "Attempting download: $archive_name"
        log_debug "Download URL: $url"
        
        while [ "$attempt" -le "$max_attempts" ]; do
            log_install "Download attempt $attempt/$max_attempts..."
            
            # Try curl first
            if command -v curl >/dev/null 2>&1; then
                local curl_output
                curl_output=$(curl -L --fail --connect-timeout 30 --max-time 300 \
                    --progress-bar -o "$temp_archive" "$url" 2>&1)
                local curl_exit_code=$?
                
                if [ $curl_exit_code -eq 0 ]; then
                    if [ -s "$temp_archive" ]; then
                        local file_size
                        file_size=$(wc -c < "$temp_archive")
                        if [ "$file_size" -gt 1000 ]; then
                            log_install "✓ Downloaded $archive_name successfully (${file_size} bytes)"
                            return 0
                        else
                            last_error="Downloaded archive too small: ${file_size} bytes"
                            log_install "⚠ $last_error"
                        fi
                    else
                        last_error="Downloaded archive is empty"
                        log_install "⚠ $last_error"
                    fi
                else
                    case $curl_exit_code in
                        6) last_error="Could not resolve host (DNS issue)" ;;
                        7) last_error="Failed to connect to server" ;;
                        22) last_error="HTTP error (file not found or server error)" ;;
                        28) last_error="Connection timeout" ;;
                        *) last_error="curl failed with exit code $curl_exit_code" ;;
                    esac
                    log_install "⚠ $last_error"
                fi
            elif command -v wget >/dev/null 2>&1; then
                if wget --timeout=30 --tries=1 -O "$temp_archive" "$url" 2>/dev/null; then
                    if [ -s "$temp_archive" ] && [ "$(wc -c < "$temp_archive")" -gt 1000 ]; then
                        local file_size
                        file_size=$(wc -c < "$temp_archive")
                        log_install "✓ Downloaded $archive_name successfully (${file_size} bytes)"
                        return 0
                    else
                        last_error="Downloaded archive invalid or too small"
                        log_install "⚠ $last_error"
                    fi
                else
                    last_error="wget download failed"
                    log_install "⚠ $last_error"
                fi
            else
                last_error="No download tool available (curl or wget required)"
                log_install_error "$last_error"
                return 1
            fi
            
            # Clean up failed download
            rm -f "$temp_archive" 2>/dev/null || true
            
            # Exponential backoff delay
            if [ "$attempt" -lt "$max_attempts" ]; then
                local delay=$((base_delay * attempt))
                log_install "Waiting ${delay} seconds before retry..."
                sleep "$delay"
            fi
            
            ((attempt++))
        done
        
        log_install_error "Failed to download $archive_name after $max_attempts attempts"
        log_install_error "Last error: $last_error"
        return 1
    }
    
    extract_binary_from_archive() {
        local archive_file="$1"
        local extract_dir="$2"
        local expected_binary_name="portty"
        
        log_install_step "Extracting binary from archive..."
        
        if ! command -v tar >/dev/null 2>&1; then
            log_install_error "tar command not found - required to extract .tar.gz archives"
            return 1
        fi
        
        if ! tar -xzf "$archive_file" -C "$extract_dir" 2>/dev/null; then
            log_install_error "Failed to extract archive: $archive_file"
            log_install "💡 Archive may be corrupted or not a valid .tar.gz file"
            return 1
        fi
        
        local binary_path=""
        if [ -f "$extract_dir/$expected_binary_name" ]; then
            binary_path="$extract_dir/$expected_binary_name"
        elif [ -f "$extract_dir/portty" ]; then
            binary_path="$extract_dir/portty"
        else
            binary_path=$(find "$extract_dir" -name "portty" -type f -executable 2>/dev/null | head -1)
        fi
        
        if [ -z "$binary_path" ] || [ ! -f "$binary_path" ]; then
            log_install_error "Binary 'portty' not found in extracted archive"
            log_install "💡 Archive contents:"
            ls -la "$extract_dir" 2>/dev/null || true
            return 1
        fi
        
        if [ ! -x "$binary_path" ]; then
            log_install "Making binary executable..."
            chmod +x "$binary_path" 2>/dev/null || true
        fi
        
        if cp "$binary_path" "$BINARY_FILE" 2>/dev/null; then
            chmod +x "$BINARY_FILE"
            log_install "✓ Binary extracted and installed successfully"
            return 0
        else
            log_install_error "Failed to copy binary to final location: $BINARY_FILE"
            return 1
        fi
    }
    
    if [ "$platform_detected" = true ]; then
        download_url="https://github.com/PiTZE/PorTTY/releases/download/${version}/${platform_archive}"
        
        if download_with_retry "$download_url" "$platform_archive"; then
            download_success=true
        else
            log_install "Platform-specific download failed, trying generic archive..."
        fi
    fi
    
    if [ "$download_success" = false ]; then
        download_url="https://github.com/PiTZE/PorTTY/releases/download/${version}/${fallback_archive}"
        
        if download_with_retry "$download_url" "$fallback_archive"; then
            download_success=true
        fi
    fi
    
    if [ "$download_success" = true ]; then
        # Extract and install binary from archive
        if extract_binary_from_archive "$temp_archive" "$temp_extract_dir"; then
            log_install "✓ PorTTY binary installed successfully"
            return 0
        else
            log_install_error "Failed to extract and install binary from archive"
            return 1
        fi
    fi
    
    log_install_error "Failed to download PorTTY binary after all attempts"
    log_install "💡 Troubleshooting suggestions:"
    log_install "  1. Check GitHub releases page: https://github.com/PiTZE/PorTTY/releases"
    log_install "  2. Verify version exists: $version"
    log_install "  3. Try manual download and place in: $INSTALL_DIR"
    log_install "  4. Check network connectivity and firewall settings"
    return 1
}

create_service_file() {
    local interface="$1"
    local port="$2"
    
    log_install_step "Creating systemd service file..."
    
    local service_dir
    service_dir=$(dirname "$SERVICE_FILE")
    if ! mkdir -p "$service_dir" 2>/dev/null; then
        log_install_error "Failed to create service directory: $service_dir"
        return 1
    fi
    
    local exec_command="$BINARY_FILE run"
    if [ "$USE_TMUX" = true ]; then
        exec_command="$exec_command --tmux"
    fi
    exec_command="$exec_command $interface:$port"
    
    if [ "$USER_INSTALL" = true ]; then
        cat > "$SERVICE_FILE" << EOF
[Unit]
Description=PorTTY Terminal Server (User)
After=network.target

[Service]
Type=simple
WorkingDirectory=%h/.local/bin
ExecStart=$exec_command
Restart=always
RestartSec=5
StandardOutput=append:$RUNTIME_LOG
StandardError=append:$RUNTIME_LOG

[Install]
WantedBy=default.target
EOF
    else
        cat > "$SERVICE_FILE" << EOF
[Unit]
Description=PorTTY Terminal Server
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=$INSTALL_DIR
ExecStart=$exec_command
Restart=always
RestartSec=5
StandardOutput=append:$RUNTIME_LOG
StandardError=append:$RUNTIME_LOG

[Install]
WantedBy=multi-user.target
EOF
    fi
    
    if [ $? -eq 0 ]; then
        log_install "Service file created successfully ✓"
        return 0
    else
        log_install_error "Failed to create service file"
        return 1
    fi
}

install_portty() {
    local interface="${1:-$DEFAULT_INTERFACE}"
    local port="${2:-$DEFAULT_PORT}"
    
    log_install_step "Starting PorTTY installation..."
    
    if [ "$USE_TMUX" = true ]; then
        if ! command -v tmux &> /dev/null; then
            log_error "tmux is required when using --tmux flag but is not installed."
            log_info "Please install tmux first:"
            echo "  - Debian/Ubuntu: sudo apt install tmux"
            echo "  - CentOS/RHEL: sudo yum install tmux"
            echo "  - macOS: brew install tmux"
            return 1
        fi
        log_info "tmux found - session persistence mode enabled"
    elif [ "$SKIP_TMUX_CHECK" != "true" ]; then
        check_tmux
    fi
    
    download_binary || return 1
    
    if command -v systemctl &> /dev/null; then
        create_service_file "$interface" "$port" || return 1
        
        $SYSTEMCTL_CMD daemon-reload
        $SYSTEMCTL_CMD enable portty.service
        
        $SYSTEMCTL_CMD start portty.service
        
        if $SYSTEMCTL_CMD is-active --quiet portty.service; then
            log_install "PorTTY service is running! ✓"
            log_install "Installation completed successfully ✓"
            return 0
        else
            log_install_error "PorTTY service failed to start"
            if [ "$USER_INSTALL" = true ]; then
                log_install_error "Check logs with: journalctl --user -u portty"
            else
                log_install_error "Check logs with: journalctl -u portty"
            fi
            return 1
        fi
    else
        log_install "Systemd not detected. Service installation skipped."
        log_install "PorTTY binary installed at: $BINARY_FILE"
        log_install "To start PorTTY manually, run: $BINARY_FILE run $interface:$port"
        log_install "Installation completed successfully ✓"
        return 0
    fi
}

uninstall_portty() {
    log_step "Uninstalling PorTTY..."
    
    if [ -f "$SERVICE_FILE" ]; then
        log_info "Stopping and removing PorTTY service..."
        if command -v systemctl &> /dev/null; then
            $SYSTEMCTL_CMD stop portty.service 2>/dev/null || true
            $SYSTEMCTL_CMD disable portty.service 2>/dev/null || true
            $SYSTEMCTL_CMD daemon-reload
        fi
        rm -f "$SERVICE_FILE"
        log_info "PorTTY service has been removed."
    else
        log_info "No PorTTY service found."
    fi
    
    if [ -f "$BINARY_FILE" ]; then
        log_info "Removing PorTTY binary..."
        rm -f "$BINARY_FILE"
        log_info "PorTTY binary has been removed."
    else
        log_info "No PorTTY binary found at $BINARY_FILE."
    fi
    
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

update_portty() {
    log_step "Updating PorTTY..."
    
    if [ ! -f "$BINARY_FILE" ]; then
        log_error "PorTTY is not installed. Use 'install' command first."
        return 1
    fi
    
    if command -v systemctl &> /dev/null; then
        $SYSTEMCTL_CMD stop portty.service 2>/dev/null || true
        
        local current_config=""
        if [ -f "$SERVICE_FILE" ]; then
            current_config=$(grep "ExecStart=" "$SERVICE_FILE" | sed 's/ExecStart=.*portty run //')
        fi
    fi
    
    FORCE_UPDATE=true
    download_binary || return 1
    
    if command -v systemctl &> /dev/null; then
        $SYSTEMCTL_CMD start portty.service
        
        if $SYSTEMCTL_CMD is-active --quiet portty.service; then
            log_info "PorTTY updated successfully ✓"
            return 0
        else
            log_error "Failed to restart PorTTY after update"
            return 1
        fi
    else
        log_info "Systemd not detected. Service not restarted."
        log_info "PorTTY updated successfully ✓"
        return 0
    fi
}

# ============================================================================
# STATUS AND SERVICE MANAGEMENT FUNCTIONS
# ============================================================================

get_configured_port() {
    local service_file_to_check=""
    local configured_port="$DEFAULT_PORT"
    local configured_interface="$DEFAULT_INTERFACE"
    
    # Determine which service file to check based on installation mode
    if [ "$USER_INSTALL" = true ]; then
        service_file_to_check="$HOME/.config/systemd/user/portty.service"
    else
        service_file_to_check="/etc/systemd/system/portty.service"
    fi
    
    # If the determined service file doesn't exist, try the other location
    if [ ! -f "$service_file_to_check" ]; then
        if [ "$USER_INSTALL" = true ]; then
            service_file_to_check="/etc/systemd/system/portty.service"
        else
            service_file_to_check="$HOME/.config/systemd/user/portty.service"
        fi
    fi
    
    # If no service file exists, return defaults
    if [ ! -f "$service_file_to_check" ]; then
        log_debug "No service file found, using default port: $DEFAULT_PORT"
        echo "$DEFAULT_PORT:$DEFAULT_INTERFACE"
        return 0
    fi
    
    log_debug "Reading configuration from: $service_file_to_check"
    
    # Extract ExecStart line from service file
    local exec_line
    exec_line=$(grep "^ExecStart=" "$service_file_to_check" 2>/dev/null)
    
    if [ -z "$exec_line" ]; then
        log_debug "No ExecStart line found, using default port: $DEFAULT_PORT"
        echo "$DEFAULT_PORT:$DEFAULT_INTERFACE"
        return 0
    fi
    
    log_debug "Found ExecStart line: $exec_line"
    
    # Parse different ExecStart formats:
    # Format 1: ExecStart=/usr/local/bin/portty run 0.0.0.0:7314
    # Format 2: ExecStart=/usr/local/bin/portty run --tmux localhost:8080
    # Format 3: ExecStart=/usr/local/bin/portty run --tmux 127.0.0.1:9000
    
    local interface_port=""
    
    # Remove the binary path and 'run' command, handle --tmux flag
    local args_part
    args_part=$(echo "$exec_line" | sed 's/^ExecStart=[^ ]* run //')
    
    # Check if --tmux flag is present and remove it
    if [[ "$args_part" =~ --tmux[[:space:]]+ ]]; then
        args_part=$(echo "$args_part" | sed 's/--tmux[[:space:]]\+//')
        log_debug "tmux mode detected in service configuration"
    fi
    
    # Extract interface:port from remaining arguments
    # Handle both IPv4/hostname and IPv6 formats:
    # - Regular: interface:port (e.g., localhost:8080, 127.0.0.1:9000)
    # - IPv6: [interface]:port (e.g., [::1]:8080, [2001:db8::1]:9000)
    
    local interface_port_pattern=""
    if [[ "$args_part" =~ \[([^\]]+)\]:([0-9]+)[[:space:]]*$ ]]; then
        # IPv6 format: [interface]:port
        configured_interface="[${BASH_REMATCH[1]}]"
        configured_port="${BASH_REMATCH[2]}"
        log_debug "Detected IPv6 format - Interface: $configured_interface, Port: $configured_port"
    elif [[ "$args_part" =~ ([^[:space:]]+):([0-9]+)[[:space:]]*$ ]]; then
        # Regular format: interface:port
        configured_interface="${BASH_REMATCH[1]}"
        configured_port="${BASH_REMATCH[2]}"
        log_debug "Detected regular format - Interface: $configured_interface, Port: $configured_port"
    else
        log_debug "Could not parse interface:port from: $args_part"
        configured_interface="$DEFAULT_INTERFACE"
        configured_port="$DEFAULT_PORT"
    fi
    
    # Validate port number
    if [[ "$configured_port" =~ ^[0-9]+$ ]] && [ "$configured_port" -ge 1 ] && [ "$configured_port" -le 65535 ]; then
        log_debug "Successfully parsed - Interface: $configured_interface, Port: $configured_port"
        echo "$configured_port:$configured_interface"
        return 0
    else
        log_debug "Invalid port number parsed: $configured_port, using default"
    fi
    
    # Fallback to defaults if parsing failed
    log_debug "Port parsing failed, using defaults - Interface: $DEFAULT_INTERFACE, Port: $DEFAULT_PORT"
    echo "$DEFAULT_PORT:$DEFAULT_INTERFACE"
    return 0
}

check_status() {
    log_step "Checking PorTTY status..."
    
    echo -e "${BOLD}System Information:${NC}"
    echo "  OS: $(uname -s) $(uname -r)"
    echo "  Architecture: $(uname -m)"
    echo "  Current User: $(whoami)"
    
    # Show platform detection results
    if detect_platform; then
        echo "  Detected Platform: ${DETECTED_PLATFORM} (${DETECTED_OS}/${DETECTED_ARCH})"
    else
        echo "  Platform Detection: Failed - will use generic binary"
    fi
    echo
    
    echo -e "${BOLD}PorTTY Status:${NC}"
    
    if [ -f "$BINARY_FILE" ]; then
        echo "  Binary: ✓ Installed at $BINARY_FILE"
        local binary_version
        binary_version=$("$BINARY_FILE" --version 2>/dev/null | head -1)
        if [ -n "$binary_version" ]; then
            echo "  Version: $binary_version"
        fi
    else
        echo "  Binary: ✗ Not found"
    fi
    
    if [ -f "$SERVICE_FILE" ]; then
        echo "  Service: ✓ Installed"
        
        if $SYSTEMCTL_CMD is-enabled --quiet portty.service; then
            echo "  Enabled: ✓ Yes"
        else
            echo "  Enabled: ✗ No"
        fi
        
        if $SYSTEMCTL_CMD is-active --quiet portty.service; then
            echo "  Status: ✓ Running"
            local pid
            pid=$($SYSTEMCTL_CMD show --property MainPID --value portty.service)
            echo "  PID: $pid"
            
            # Get configured port and interface from service file
            local port_interface_info
            port_interface_info=$(get_configured_port)
            local configured_port="${port_interface_info%%:*}"
            local configured_interface="${port_interface_info##*:}"
            
            echo "  Interface: $configured_interface"
            echo "  Port: $configured_port"
            
            # Check if service is actually listening on the configured port
            local port_info
            port_info=$(ss -tulpn | grep ":$configured_port" | head -1)
            if [ -n "$port_info" ]; then
                echo "  Listening: ✓ $port_info"
            else
                echo "  Listening: ✗ Not detected on port $configured_port"
                # Also check if it might be listening on default port
                if [ "$configured_port" != "$DEFAULT_PORT" ]; then
                    local default_port_info
                    default_port_info=$(ss -tulpn | grep ":$DEFAULT_PORT" | head -1)
                    if [ -n "$default_port_info" ]; then
                        echo "  Note: Found service listening on default port $DEFAULT_PORT instead"
                    fi
                fi
            fi
        else
            echo "  Status: ✗ Not running"
            
            # Still show configured settings even when not running
            local port_interface_info
            port_interface_info=$(get_configured_port)
            local configured_port="${port_interface_info%%:*}"
            local configured_interface="${port_interface_info##*:}"
            
            if [ "$configured_port" != "$DEFAULT_PORT" ] || [ "$configured_interface" != "$DEFAULT_INTERFACE" ]; then
                echo "  Configured Interface: $configured_interface"
                echo "  Configured Port: $configured_port"
            fi
        fi
    else
        echo "  Service: ✗ Not installed"
    fi
    
    if command -v tmux &> /dev/null; then
        echo "  tmux: ✓ Installed (session persistence available)"
    else
        echo "  tmux: ⚠ Not installed (optional - enables session persistence)"
    fi
    
    return 0
}

start_service() {
    log_step "Starting PorTTY service..."
    
    if [ ! -f "$SERVICE_FILE" ]; then
        log_error_with_context "Service not installed" "service start" "Run '$SCRIPT_NAME install' first to install PorTTY"
        return 1
    fi
    
    # Check if service is already running
    if $SYSTEMCTL_CMD is-active --quiet portty.service; then
        log_info "PorTTY service is already running"
        return 0
    fi
    
    log_info "Starting PorTTY service..."
    if ! $SYSTEMCTL_CMD start portty.service 2>/dev/null; then
        log_error_with_context "Failed to start PorTTY service" "systemctl start" "Check service logs: journalctl -u portty.service"
        
        # Provide additional diagnostic information
        if [ "$USER_INSTALL" = true ]; then
            log_info "💡 For user services, try: systemctl --user start portty.service"
            log_info "💡 Check user service logs: journalctl --user -u portty.service"
        else
            log_info "💡 Check system service logs: journalctl -u portty.service"
            log_info "💡 Verify service file: cat $SERVICE_FILE"
        fi
        return 1
    fi
    
    # Wait a moment for service to fully start
    sleep 2
    
    if $SYSTEMCTL_CMD is-active --quiet portty.service; then
        log_info "PorTTY service started successfully ✓"
        
        # Show connection information
        local port_interface_info
        port_interface_info=$(get_configured_port)
        local configured_port="${port_interface_info%%:*}"
        local configured_interface="${port_interface_info##*:}"
        
        log_info "Service is now accessible at: http://$configured_interface:$configured_port"
        return 0
    else
        log_error_with_context "Service started but is not active" "service verification" "Check service logs for startup errors"
        return 1
    fi
}

stop_service() {
    log_step "Stopping PorTTY service..."
    
    if [ ! -f "$SERVICE_FILE" ]; then
        log_warning "Service not installed - nothing to stop"
        return 0
    fi
    
    # Check if service is already stopped
    if ! $SYSTEMCTL_CMD is-active --quiet portty.service; then
        log_info "PorTTY service is already stopped"
        return 0
    fi
    
    log_info "Stopping PorTTY service..."
    if ! $SYSTEMCTL_CMD stop portty.service 2>/dev/null; then
        log_error_with_context "Failed to stop PorTTY service" "systemctl stop" "Service may be unresponsive - try force kill"
        
        # Provide additional diagnostic information
        if [ "$USER_INSTALL" = true ]; then
            log_info "💡 For user services, try: systemctl --user stop portty.service"
            log_info "💡 Force stop: systemctl --user kill portty.service"
        else
            log_info "💡 Force stop: systemctl kill portty.service"
            log_info "💡 Check process: ps aux | grep portty"
        fi
        return 1
    fi
    
    # Wait a moment for service to fully stop
    sleep 2
    
    if ! $SYSTEMCTL_CMD is-active --quiet portty.service; then
        log_info "PorTTY service stopped successfully ✓"
        return 0
    else
        log_error_with_context "Service stop command succeeded but service is still active" "service verification" "Try force stopping the service"
        return 1
    fi
}

restart_service() {
    log_step "Restarting PorTTY service..."
    
    if [ ! -f "$SERVICE_FILE" ]; then
        log_error_with_context "Service not installed" "service restart" "Run '$SCRIPT_NAME install' first to install PorTTY"
        return 1
    fi
    
    log_info "Restarting PorTTY service..."
    if ! $SYSTEMCTL_CMD restart portty.service 2>/dev/null; then
        log_error_with_context "Failed to restart PorTTY service" "systemctl restart" "Check service configuration and logs"
        
        # Provide additional diagnostic information
        if [ "$USER_INSTALL" = true ]; then
            log_info "💡 For user services, try: systemctl --user restart portty.service"
            log_info "💡 Check user service logs: journalctl --user -u portty.service"
        else
            log_info "💡 Check system service logs: journalctl -u portty.service"
            log_info "💡 Verify service file: cat $SERVICE_FILE"
        fi
        return 1
    fi
    
    # Wait a moment for service to fully restart
    sleep 3
    
    if $SYSTEMCTL_CMD is-active --quiet portty.service; then
        log_info "PorTTY service restarted successfully ✓"
        
        # Show connection information
        local port_interface_info
        port_interface_info=$(get_configured_port)
        local configured_port="${port_interface_info%%:*}"
        local configured_interface="${port_interface_info##*:}"
        
        log_info "Service is now accessible at: http://$configured_interface:$configured_port"
        return 0
    else
        log_error_with_context "Service restart command succeeded but service is not active" "service verification" "Check service logs for startup errors"
        return 1
    fi
}

enable_service() {
    log_step "Enabling PorTTY service at boot..."
    
    if [ ! -f "$SERVICE_FILE" ]; then
        log_error_with_context "Service not installed" "service enable" "Run '$SCRIPT_NAME install' first to install PorTTY"
        return 1
    fi
    
    # Check if service is already enabled
    if $SYSTEMCTL_CMD is-enabled --quiet portty.service; then
        log_info "PorTTY service is already enabled at boot"
        return 0
    fi
    
    log_info "Enabling PorTTY service at boot..."
    if ! $SYSTEMCTL_CMD enable portty.service 2>/dev/null; then
        log_error_with_context "Failed to enable PorTTY service" "systemctl enable" "Check service file permissions and systemd configuration"
        
        # Provide additional diagnostic information
        if [ "$USER_INSTALL" = true ]; then
            log_info "💡 For user services, try: systemctl --user enable portty.service"
            log_info "💡 Ensure user lingering: loginctl enable-linger $USER"
        else
            log_info "💡 Check service file: ls -la $SERVICE_FILE"
            log_info "💡 Reload systemd: systemctl daemon-reload"
        fi
        return 1
    fi
    
    if $SYSTEMCTL_CMD is-enabled --quiet portty.service; then
        log_info "PorTTY service enabled at boot ✓"
        if [ "$USER_INSTALL" = true ]; then
            log_info "💡 Note: User services require user login to start automatically"
            log_info "💡 For persistent user services, consider: loginctl enable-linger $USER"
        fi
        return 0
    else
        log_error_with_context "Enable command succeeded but service is not enabled" "service verification" "Check systemd configuration"
        return 1
    fi
}

disable_service() {
    log_step "Disabling PorTTY service at boot..."
    
    if [ ! -f "$SERVICE_FILE" ]; then
        log_warning "Service not installed - nothing to disable"
        return 0
    fi
    
    # Check if service is already disabled
    if ! $SYSTEMCTL_CMD is-enabled --quiet portty.service; then
        log_info "PorTTY service is already disabled at boot"
        return 0
    fi
    
    log_info "Disabling PorTTY service at boot..."
    if ! $SYSTEMCTL_CMD disable portty.service 2>/dev/null; then
        log_error_with_context "Failed to disable PorTTY service" "systemctl disable" "Check service file permissions and systemd configuration"
        
        # Provide additional diagnostic information
        if [ "$USER_INSTALL" = true ]; then
            log_info "💡 For user services, try: systemctl --user disable portty.service"
        else
            log_info "💡 Check service file: ls -la $SERVICE_FILE"
            log_info "💡 Reload systemd: systemctl daemon-reload"
        fi
        return 1
    fi
    
    if ! $SYSTEMCTL_CMD is-enabled --quiet portty.service; then
        log_info "PorTTY service disabled at boot ✓"
        return 0
    else
        log_error_with_context "Disable command succeeded but service is still enabled" "service verification" "Check systemd configuration"
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
    if [ "$USER_INSTALL" = true ]; then
        journalctl --user -u portty.service --no-pager -n 20
    else
        journalctl -u portty.service --no-pager -n 20
    fi
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
        local exec_line
        exec_line=$(grep "ExecStart=" "$SERVICE_FILE")
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

# ============================================================================
# INTERACTIVE MENU FUNCTIONS
# ============================================================================

show_main_menu() {
    if [ "$IS_INTERACTIVE_TERMINAL" = false ]; then
        log_info "Running in non-interactive mode. Using default installation."
        
        if ! command -v tmux &> /dev/null; then
            log_warning "tmux is not installed."
            log_info "PorTTY will run in default shell mode (direct shell access)."
            log_info "For session persistence, consider installing tmux later."
            SKIP_TMUX_CHECK=true
        fi
        
        if check_root && install_portty "$DEFAULT_INTERFACE" "$DEFAULT_PORT"; then
            log_info "PorTTY installed successfully!"
            echo "Access PorTTY at: http://$DEFAULT_INTERFACE:$DEFAULT_PORT"
        else
            log_error "Installation failed!"
            exit 1
        fi
        return
    fi

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
                # Installation Mode Selection
                if ! select_installation_mode; then
                    read -p "Press Enter to continue..."
                    continue
                fi
                
                # Installation Directory Selection
                echo
                echo "Installation Directory:"
                echo "Current: $INSTALL_DIR"
                read -p "Custom directory (press Enter to use current): " custom_dir
                if [ -n "$custom_dir" ]; then
                    if validate_directory "$custom_dir" "custom installation directory" "false"; then
                        INSTALL_DIR="$custom_dir"
                        BINARY_FILE="$INSTALL_DIR/portty"
                        echo "Using custom directory: $INSTALL_DIR"
                    else
                        echo -e "${RED}Invalid directory, using default: $INSTALL_DIR${NC}"
                    fi
                fi
                
                # Interface and Port Configuration
                echo
                read -p "Interface [$DEFAULT_INTERFACE]: " interface
                interface=${interface:-$DEFAULT_INTERFACE}
                
                read -p "Port [$DEFAULT_PORT]: " port
                port=${port:-$DEFAULT_PORT}
                
                # Shell Mode Selection
                echo
                echo "Shell Mode Selection:"
                echo "1. Default Shell Mode (direct shell access)"
                echo "2. tmux Mode (session persistence and multi-client support)"
                read -p "Choose shell mode [1]: " shell_mode
                shell_mode=${shell_mode:-1}
                
                if [ "$shell_mode" = "2" ]; then
                    if command -v tmux &> /dev/null; then
                        USE_TMUX=true
                        echo "tmux mode selected - session persistence enabled"
                    else
                        echo -e "${RED}Error: tmux is not installed but required for tmux mode${NC}"
                        echo "Please install tmux first or choose default shell mode"
                        read -p "Press Enter to continue..."
                        continue
                    fi
                else
                    USE_TMUX=false
                    echo "Default shell mode selected"
                fi
                
                # Advanced Options
                echo
                echo "Advanced Options:"
                read -p "Skip checksum verification? [y/N]: " skip_checksum
                if [[ "$skip_checksum" =~ ^[Yy]$ ]]; then
                    SKIP_CHECKSUM=true
                    echo "Checksum verification will be skipped"
                else
                    SKIP_CHECKSUM=false
                fi
                
                read -p "Force update (overwrite existing)? [y/N]: " force_update
                if [[ "$force_update" =~ ^[Yy]$ ]]; then
                    FORCE_UPDATE=true
                    echo "Force update enabled"
                else
                    FORCE_UPDATE=false
                fi
                
                read -p "Enable verbose logging? [y/N]: " verbose_logging
                if [[ "$verbose_logging" =~ ^[Yy]$ ]]; then
                    VERBOSE=true
                    CURRENT_LOG_LEVEL="$LOG_LEVEL_DEBUG"
                    echo "Verbose logging enabled"
                else
                    VERBOSE=false
                fi
                
                echo
                echo "Installation Summary:"
                echo "  Mode: $([ "$USER_INSTALL" = true ] && echo "User" || echo "System")"
                echo "  Directory: $INSTALL_DIR"
                echo "  Interface: $interface"
                echo "  Port: $port"
                echo "  Shell Mode: $([ "$USE_TMUX" = true ] && echo "tmux" || echo "Default")"
                echo "  Skip Checksum: $([ "$SKIP_CHECKSUM" = true ] && echo "Yes" || echo "No")"
                echo "  Force Update: $([ "$FORCE_UPDATE" = true ] && echo "Yes" || echo "No")"
                echo "  Verbose: $([ "$VERBOSE" = true ] && echo "Yes" || echo "No")"
                echo
                read -p "Proceed with installation? [Y/n]: " proceed
                if [[ ! "$proceed" =~ ^[Nn]$ ]]; then
                    if install_portty "$interface" "$port"; then
                        echo -e "${GREEN}Installation successful!${NC}"
                    else
                        echo -e "${RED}Installation failed!${NC}"
                    fi
                else
                    echo "Installation cancelled."
                fi
                read -p "Press Enter to continue..."
                ;;
            2)
                # Update Mode Selection
                echo
                echo "Update Mode:"
                echo "1. System Installation"
                echo "2. User Installation"
                read -p "Choose installation mode to update [1]: " update_mode
                update_mode=${update_mode:-1}
                
                if [ "$update_mode" = "2" ]; then
                    USER_INSTALL=true
                    setup_installation_paths
                    echo "Updating user installation"
                else
                    USER_INSTALL=false
                    setup_installation_paths
                    echo "Updating system installation"
                    if [ "$EUID" -ne 0 ]; then
                        echo -e "${RED}Error: System update requires root privileges${NC}"
                        echo "Please run with sudo or choose user installation mode"
                        read -p "Press Enter to continue..."
                        continue
                    fi
                fi
                
                # Advanced Options for Update
                echo
                echo "Advanced Options:"
                read -p "Skip checksum verification? [y/N]: " skip_checksum
                if [[ "$skip_checksum" =~ ^[Yy]$ ]]; then
                    SKIP_CHECKSUM=true
                fi
                
                read -p "Enable verbose logging? [y/N]: " verbose_logging
                if [[ "$verbose_logging" =~ ^[Yy]$ ]]; then
                    VERBOSE=true
                    CURRENT_LOG_LEVEL="$LOG_LEVEL_INFO"
                fi
                
                if update_portty; then
                    echo -e "${GREEN}Update successful!${NC}"
                else
                    echo -e "${RED}Update failed!${NC}"
                fi
                read -p "Press Enter to continue..."
                ;;
            3)
                # Uninstall Mode Selection
                echo
                echo "Uninstall Mode:"
                echo "1. System Installation"
                echo "2. User Installation"
                read -p "Choose installation mode to uninstall [1]: " uninstall_mode
                uninstall_mode=${uninstall_mode:-1}
                
                if [ "$uninstall_mode" = "2" ]; then
                    USER_INSTALL=true
                    setup_installation_paths
                    echo "Uninstalling user installation"
                else
                    USER_INSTALL=false
                    setup_installation_paths
                    echo "Uninstalling system installation"
                    if [ "$EUID" -ne 0 ]; then
                        echo -e "${RED}Error: System uninstall requires root privileges${NC}"
                        echo "Please run with sudo or choose user installation mode"
                        read -p "Press Enter to continue..."
                        continue
                    fi
                fi
                
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

# Helper function to select installation mode for installation operations
select_installation_mode() {
    echo
    echo "Installation Mode:"
    echo "1. System Installation (requires sudo, available to all users)"
    echo "2. User Installation (no sudo required, current user only)"
    read -p "Choose installation mode [1]: " install_mode
    install_mode=${install_mode:-1}
    
    if [ "$install_mode" = "2" ]; then
        # Check if running as root - user installation not recommended for root
        if [ "$EUID" -eq 0 ]; then
            echo -e "${YELLOW}Warning: User installation as root is not recommended${NC}"
            echo "This will create service files in /root/.config/systemd/user/ which may cause conflicts"
            echo "Consider using system installation instead"
            read -p "Continue with user installation anyway? [y/N]: " continue_user
            if [[ ! "$continue_user" =~ ^[Yy]$ ]]; then
                echo "Switching to system installation mode"
                USER_INSTALL=false
                setup_installation_paths
                echo "System installation mode selected"
                return 0
            fi
        fi
        
        USER_INSTALL=true
        setup_installation_paths
        echo "User installation mode selected"
        log_debug "User install paths: SERVICE_FILE=$SERVICE_FILE, INSTALL_DIR=$INSTALL_DIR"
        return 0
    else
        USER_INSTALL=false
        setup_installation_paths
        echo "System installation mode selected"
        log_debug "System install paths: SERVICE_FILE=$SERVICE_FILE, INSTALL_DIR=$INSTALL_DIR"
        if [ "$EUID" -ne 0 ]; then
            echo -e "${RED}Error: System installation requires root privileges${NC}"
            echo "Please run with sudo or choose user installation mode"
            return 1
        fi
        return 0
    fi
}

service_menu() {
    # Helper function to select installation mode for service operations
    select_service_mode() {
        local operation="$1"
        echo
        echo "Service Mode Selection for $operation:"
        echo "1. System Service (requires sudo)"
        echo "2. User Service (current user only)"
        
        # Try to detect existing installation
        local system_service="/etc/systemd/system/portty.service"
        local user_service="$HOME/.config/systemd/user/portty.service"
        local detected_mode=""
        
        if [ -f "$system_service" ] && [ -f "$user_service" ]; then
            echo "Note: Both system and user services detected"
        elif [ -f "$system_service" ]; then
            echo "Note: System service detected"
            detected_mode="1"
        elif [ -f "$user_service" ]; then
            echo "Note: User service detected"
            detected_mode="2"
        else
            echo "Note: No existing service detected"
        fi
        
        local default_choice="${detected_mode:-1}"
        read -p "Choose service mode [$default_choice]: " service_mode
        service_mode=${service_mode:-$default_choice}
        
        if [ "$service_mode" = "2" ]; then
            USER_INSTALL=true
            setup_installation_paths
            echo "User service mode selected"
            return 0
        else
            USER_INSTALL=false
            setup_installation_paths
            echo "System service mode selected"
            if [ "$EUID" -ne 0 ]; then
                echo -e "${RED}Error: System service operations require root privileges${NC}"
                echo "Please run with sudo or choose user service mode"
                return 1
            fi
            return 0
        fi
    }

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
                if select_service_mode "start"; then
                    if start_service; then
                        echo -e "${GREEN}Service started successfully!${NC}"
                    else
                        echo -e "${RED}Failed to start service!${NC}"
                    fi
                fi
                read -p "Press Enter to continue..."
                ;;
            2)
                if select_service_mode "stop"; then
                    if stop_service; then
                        echo -e "${GREEN}Service stopped successfully!${NC}"
                    else
                        echo -e "${RED}Failed to stop service!${NC}"
                    fi
                fi
                read -p "Press Enter to continue..."
                ;;
            3)
                if select_service_mode "restart"; then
                    if restart_service; then
                        echo -e "${GREEN}Service restarted successfully!${NC}"
                    else
                        echo -e "${RED}Failed to restart service!${NC}"
                    fi
                fi
                read -p "Press Enter to continue..."
                ;;
            4)
                if select_service_mode "enable"; then
                    if enable_service; then
                        echo -e "${GREEN}Service enabled at boot!${NC}"
                        if [ "$USER_INSTALL" = true ]; then
                            echo -e "${YELLOW}Note: User services require user login to start automatically${NC}"
                            echo -e "${YELLOW}For persistent user services, consider: loginctl enable-linger $USER${NC}"
                        fi
                    else
                        echo -e "${RED}Failed to enable service!${NC}"
                    fi
                fi
                read -p "Press Enter to continue..."
                ;;
            5)
                if select_service_mode "disable"; then
                    if disable_service; then
                        echo -e "${GREEN}Service disabled at boot!${NC}"
                    else
                        echo -e "${RED}Failed to disable service!${NC}"
                    fi
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
    # Helper function to select installation mode for logs and config operations
    select_logs_mode() {
        local operation="$1"
        echo
        echo "Installation Mode Selection for $operation:"
        echo "1. System Installation"
        echo "2. User Installation"
        
        # Try to detect existing installation
        local system_service="/etc/systemd/system/portty.service"
        local user_service="$HOME/.config/systemd/user/portty.service"
        local system_binary="/usr/local/bin/portty"
        local user_binary="$HOME/.local/bin/portty"
        local detected_mode=""
        
        if ([ -f "$system_service" ] || [ -f "$system_binary" ]) && ([ -f "$user_service" ] || [ -f "$user_binary" ]); then
            echo "Note: Both system and user installations detected"
        elif [ -f "$system_service" ] || [ -f "$system_binary" ]; then
            echo "Note: System installation detected"
            detected_mode="1"
        elif [ -f "$user_service" ] || [ -f "$user_binary" ]; then
            echo "Note: User installation detected"
            detected_mode="2"
        else
            echo "Note: No existing installation detected"
        fi
        
        local default_choice="${detected_mode:-1}"
        read -p "Choose installation mode [$default_choice]: " logs_mode
        logs_mode=${logs_mode:-$default_choice}
        
        if [ "$logs_mode" = "2" ]; then
            USER_INSTALL=true
            setup_installation_paths
            echo "User installation mode selected"
            echo "  Log directory: $LOG_DIR"
            echo "  Service file: $SERVICE_FILE"
            return 0
        else
            USER_INSTALL=false
            setup_installation_paths
            echo "System installation mode selected"
            echo "  Log directory: $LOG_DIR"
            echo "  Service file: $SERVICE_FILE"
            return 0
        fi
    }

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
                if select_logs_mode "logs"; then
                    echo
                    show_logs
                fi
                read -p "Press Enter to continue..."
                ;;
            2)
                if select_logs_mode "configuration"; then
                    echo
                    show_config
                fi
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

# ============================================================================
# ARGUMENT PARSING FUNCTIONS
# ============================================================================

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
                    if validate_interface "$2" "command line interface"; then
                        DEFAULT_INTERFACE="$2"
                        shift 2
                    else
                        log_fatal_with_context "Invalid interface argument: $2" "command line parsing" "Use 'localhost', '0.0.0.0', or a valid IP address"
                        exit 1
                    fi
                else
                    log_fatal_with_context "Missing argument for $1" "command line parsing" "Provide an interface (e.g., --interface localhost)"
                    exit 1
                fi
                ;;
            -p|--port)
                if [[ -n "$2" && "$2" != -* ]]; then
                    if validate_port "$2" "command line port"; then
                        DEFAULT_PORT="$2"
                        shift 2
                    else
                        log_fatal_with_context "Invalid port argument: $2" "command line parsing" "Use a port number between 1-65535"
                        exit 1
                    fi
                else
                    log_fatal_with_context "Missing argument for $1" "command line parsing" "Provide a port number (e.g., --port 8080)"
                    exit 1
                fi
                ;;
            -d|--directory)
                if [[ -n "$2" && "$2" != -* ]]; then
                    if validate_directory "$2" "command line directory" "false"; then
                        INSTALL_DIR="$2"
                        BINARY_FILE="$INSTALL_DIR/portty"
                        shift 2
                    else
                        log_fatal_with_context "Invalid directory argument: $2" "command line parsing" "Provide a valid, writable directory path"
                        exit 1
                    fi
                else
                    log_fatal_with_context "Missing argument for $1" "command line parsing" "Provide a directory path (e.g., --directory /usr/local/bin)"
                    exit 1
                fi
                ;;
            --verbose)
                VERBOSE=true
                CURRENT_LOG_LEVEL="$LOG_LEVEL_INFO"
                shift
                ;;
            --debug)
                CURRENT_LOG_LEVEL="$LOG_LEVEL_DEBUG"
                shift
                ;;
            --tmux)
                USE_TMUX=true
                shift
                ;;
            --user)
                USER_INSTALL=true
                shift
                ;;
            --skip-checksum)
                SKIP_CHECKSUM=true
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

# ============================================================================
# MAIN EXECUTION LOGIC
# ============================================================================

main() {
    parse_arguments "$@"
    
    # Setup installation paths based on user/system mode
    setup_installation_paths
    
    # Create PID file with proper permissions and error handling
    if ! echo $$ > "$PID_FILE" 2>/dev/null; then
        echo "Warning: Could not create PID file: $PID_FILE" >&2
    else
        chmod 600 "$PID_FILE" 2>/dev/null || true
    fi
    
    # Ensure directories exist before any logging operations
    ensure_directories
    
    # Log installation paths after directories are created
    log_installation_paths
    
    # Check required system commands early
    if ! check_required_commands; then
        log_fatal "Critical system dependencies missing - cannot continue"
        exit 1
    fi
    
    # Check system resources for installation operations
    if [ "$MODE" = "install" ] || [ "$MODE" = "update" ]; then
        check_system_resources
    fi
    
    if [ "$MODE" = "install" ] && [ $# -eq 0 ]; then
        if [ "$IS_INTERACTIVE_TERMINAL" = true ]; then
            INTERACTIVE=true
        fi
    fi
    
    case "$MODE" in
        install)
            if [ "$INTERACTIVE" = true ]; then
                show_main_menu
            else
                # For non-interactive mode, check root privileges only if not user install
                if [ "$USER_INSTALL" = false ]; then
                    require_root
                fi
                log_step "Installing PorTTY..."
                if install_portty "$DEFAULT_INTERFACE" "$DEFAULT_PORT"; then
                    log_info "PorTTY installed successfully!"
                    echo "Access PorTTY at: http://$DEFAULT_INTERFACE:$DEFAULT_PORT"
                else
                    log_error "Installation failed!"
                    exit 1
                fi
            fi
            ;;
        uninstall)
            if [ "$USER_INSTALL" = false ]; then
                require_root
            fi
            if [ "$INTERACTIVE" = true ] && [ "$YES_FLAG" = false ]; then
                read -p "Are you sure you want to uninstall PorTTY? [y/N]: " confirm
                if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
                    echo "Uninstallation cancelled."
                    exit 0
                fi
            fi
            if uninstall_portty; then
                log_info "PorTTY uninstalled successfully!"
            else
                log_error "Uninstallation failed!"
                exit 1
            fi
            ;;
        update)
            if [ "$USER_INSTALL" = false ]; then
                require_root
            fi
            if update_portty; then
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
            if [ "$USER_INSTALL" = false ]; then
                require_root
            fi
            if start_service; then
                log_info "PorTTY service started successfully!"
            else
                log_error "Failed to start service!"
                exit 1
            fi
            ;;
        stop)
            if [ "$USER_INSTALL" = false ]; then
                require_root
            fi
            if stop_service; then
                log_info "PorTTY service stopped successfully!"
            else
                log_error "Failed to stop service!"
                exit 1
            fi
            ;;
        restart)
            if [ "$USER_INSTALL" = false ]; then
                require_root
            fi
            if restart_service; then
                log_info "PorTTY service restarted successfully!"
            else
                log_error "Failed to restart service!"
                exit 1
            fi
            ;;
        enable)
            if [ "$USER_INSTALL" = false ]; then
                require_root
            fi
            if enable_service; then
                log_info "PorTTY service enabled at boot!"
            else
                log_error "Failed to enable service!"
                exit 1
            fi
            ;;
        disable)
            if [ "$USER_INSTALL" = false ]; then
                require_root
            fi
            if disable_service; then
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
            show_main_menu
            ;;
    esac
}

main "$@"