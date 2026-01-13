#!/bin/bash

# install.sh - Install/upgrade/uninstall ollama-auto-ctx as a systemd service
# Downloads the latest release from GitHub automatically
# Usage: ./install.sh [install|upgrade|uninstall] [OPTIONS]

set -e

# GitHub repository
GITHUB_REPO="lennarkivistik/ollama-auto-ctx"
GITHUB_API_URL="https://api.github.com/repos/${GITHUB_REPO}/releases/latest"

# Default values
ACTION="install"
SERVICE_NAME="ollama-auto-ctx"
SERVICE_USER="ollama-auto-ctx"
INSTALL_DIR="/opt/ollama-auto-ctx"
DATA_DIR="/var/lib/ollama-auto-ctx"
LOG_DIR="/var/log/ollama-auto-ctx"
CONFIG_DIR="/etc/ollama-auto-ctx"
LISTEN_ADDR=":11435"
UPSTREAM_URL="http://localhost:11434"
CALIBRATION_FILE="${DATA_DIR}/calibration.json"
BINARY_PATH=""
BUILD_BINARY=false
DOWNLOAD_RELEASE=true
VERSION=""

# Detect OS and architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)
    
    case "$ARCH" in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        armv7l)
            ARCH="arm"
            ;;
        *)
            echo "Unsupported architecture: $ARCH"
            exit 1
            ;;
    esac
    
    case "$OS" in
        linux)
            OS="linux"
            ;;
        darwin)
            OS="darwin"
            ;;
        *)
            echo "Unsupported operating system: $OS"
            exit 1
            ;;
    esac
    
    PLATFORM="${OS}_${ARCH}"
    echo "Detected platform: $PLATFORM"
}

# Get the latest release version from GitHub
get_latest_version() {
    echo "Checking for latest release..."
    
    if command -v curl &> /dev/null; then
        LATEST_RELEASE=$(curl -s "$GITHUB_API_URL")
    elif command -v wget &> /dev/null; then
        LATEST_RELEASE=$(wget -qO- "$GITHUB_API_URL")
    else
        echo "Error: curl or wget is required to download releases"
        exit 1
    fi
    
    # Parse version from JSON (using grep/sed for portability)
    LATEST_VERSION=$(echo "$LATEST_RELEASE" | grep -o '"tag_name": *"[^"]*"' | head -1 | sed 's/"tag_name": *"//;s/"//')
    
    if [[ -z "$LATEST_VERSION" ]]; then
        echo "Error: Could not determine latest version from GitHub"
        echo "You may need to build from source or specify --binary-path"
        exit 1
    fi
    
    echo "Latest version: $LATEST_VERSION"
}

# Get current installed version
get_installed_version() {
    if [[ -f "$INSTALL_DIR/ollama-auto-ctx" ]]; then
        # Try to get version from binary (if it supports --version)
        INSTALLED_VERSION=$("$INSTALL_DIR/ollama-auto-ctx" --version 2>/dev/null | head -1 || echo "unknown")
        
        # Also check version file if exists
        if [[ -f "$INSTALL_DIR/.version" ]]; then
            INSTALLED_VERSION=$(cat "$INSTALL_DIR/.version")
        fi
    else
        INSTALLED_VERSION=""
    fi
}

# Download release binary from GitHub
download_release() {
    local version="$1"
    local os="$2"
    local arch="$3"
    
    # Construct download URL
    # Expected asset name: ollama-auto-ctx_<version>_<os>_<arch>.tar.gz or ollama-auto-ctx_<os>_<arch>
    local base_url="https://github.com/${GITHUB_REPO}/releases/download/${version}"
    
    # Try different naming conventions
    local asset_names=(
        "ollama-auto-ctx-${os}-${arch}-${version}"
        "ollama-auto-ctx-${os}-${arch}-${version#v}"
        "ollama-auto-ctx_${version#v}_${os}_${arch}.tar.gz"
        "ollama-auto-ctx_${os}_${arch}.tar.gz"
        "ollama-auto-ctx_${version#v}_${os}_${arch}"
        "ollama-auto-ctx_${os}_${arch}"
        "ollama-auto-ctx-${os}-${arch}"
    )
    
    local tmp_dir=$(mktemp -d)
    local downloaded=false
    
    for asset_name in "${asset_names[@]}"; do
        local download_url="${base_url}/${asset_name}"
        echo "Trying to download: $download_url"
        if [[ -n "$DEBUG" ]]; then
            echo "  Asset name: $asset_name"
            echo "  Base URL: $base_url"
        fi
        
        if command -v curl &> /dev/null; then
            if curl -fsSL -o "${tmp_dir}/${asset_name}" "$download_url" 2>/dev/null; then
                downloaded=true
                break
            fi
        elif command -v wget &> /dev/null; then
            if wget -q -O "${tmp_dir}/${asset_name}" "$download_url" 2>/dev/null; then
                downloaded=true
                break
            fi
        fi
    done
    
    if [[ "$downloaded" != true ]]; then
        echo ""
        echo "Could not download pre-built binary for ${os}-${arch}"
        echo "Tried the following URLs:"
        for asset_name in "${asset_names[@]}"; do
            echo "  - ${base_url}/${asset_name}"
        done
        echo ""
        echo "Options:"
        echo "  1. Build from source: Use --build flag"
        echo "  2. Provide binary: Use --binary-path /path/to/binary"
        echo "  3. Check releases: https://github.com/${GITHUB_REPO}/releases"
        echo ""
        rm -rf "$tmp_dir"
        return 1
    fi
    
    # Extract if tarball, otherwise it's a direct binary
    if [[ "$asset_name" == *.tar.gz ]]; then
        echo "Extracting archive..."
        tar -xzf "${tmp_dir}/${asset_name}" -C "$tmp_dir"
        # Find the binary (should be named ollama-auto-ctx)
        BINARY_PATH=$(find "$tmp_dir" -name "ollama-auto-ctx" -type f | head -1)
        if [[ -z "$BINARY_PATH" ]]; then
            # Try finding any executable file
            BINARY_PATH=$(find "$tmp_dir" -type f -executable | head -1)
        fi
    else
        # Direct binary
        BINARY_PATH="${tmp_dir}/${asset_name}"
    fi
    
    if [[ ! -f "$BINARY_PATH" ]]; then
        echo "Error: Could not find binary in downloaded archive"
        rm -rf "$tmp_dir"
        return 1
    fi
    
    chmod +x "$BINARY_PATH"
    echo "Downloaded successfully: $asset_name"
    
    # Save version
    echo "$version" > "${tmp_dir}/.version"
    VERSION_FILE="${tmp_dir}/.version"
    
    TMP_DIR="$tmp_dir"
    return 0
}

# Parse command line options
while [[ $# -gt 0 ]]; do
    case $1 in
        install)
            ACTION="install"
            shift
            ;;
        upgrade|update)
            ACTION="upgrade"
            shift
            ;;
        uninstall)
            ACTION="uninstall"
            shift
            ;;
        --version)
            VERSION="$2"
            shift 2
            ;;
        --service-name)
            SERVICE_NAME="$2"
            shift 2
            ;;
        --user)
            SERVICE_USER="$2"
            shift 2
            ;;
        --install-dir)
            INSTALL_DIR="$2"
            shift 2
            ;;
        --data-dir)
            DATA_DIR="$2"
            shift 2
            ;;
        --log-dir)
            LOG_DIR="$2"
            shift 2
            ;;
        --config-dir)
            CONFIG_DIR="$2"
            shift 2
            ;;
        --listen-addr)
            LISTEN_ADDR="$2"
            shift 2
            ;;
        --upstream-url)
            UPSTREAM_URL="$2"
            shift 2
            ;;
        --calibration-file)
            CALIBRATION_FILE="$2"
            shift 2
            ;;
        --binary-path)
            BINARY_PATH="$2"
            DOWNLOAD_RELEASE=false
            BUILD_BINARY=false
            shift 2
            ;;
        --build)
            BUILD_BINARY=true
            DOWNLOAD_RELEASE=false
            shift
            ;;
        --no-download)
            DOWNLOAD_RELEASE=false
            shift
            ;;
        --help|-h)
            echo "Usage: $0 [install|upgrade|uninstall] [OPTIONS]"
            echo ""
            echo "Install, upgrade, or uninstall ollama-auto-ctx as a systemd service"
            echo "Downloads the latest release from GitHub by default"
            echo ""
            echo "Release naming: ollama-auto-ctx-{os}-{arch}-{version}"
            echo "Example: ollama-auto-ctx-linux-amd64-v1.1.1"
            echo ""
            echo "Actions:"
            echo "  install    Install the service (default)"
            echo "  upgrade    Upgrade to the latest version (or specified version)"
            echo "  uninstall  Uninstall the service"
            echo ""
            echo "Options:"
            echo "  --version VERSION      Install specific version (e.g., v1.1.0)"
            echo "  --service-name NAME    Service name (default: ollama-auto-ctx)"
            echo "  --user USER            Service user (default: ollama-auto-ctx)"
            echo "  --install-dir DIR      Installation directory (default: /opt/ollama-auto-ctx)"
            echo "  --data-dir DIR         Data directory (default: /var/lib/ollama-auto-ctx)"
            echo "  --log-dir DIR          Log directory (default: /var/log/ollama-auto-ctx)"
            echo "  --config-dir DIR       Config directory (default: /etc/ollama-auto-ctx)"
            echo "  --listen-addr ADDR     Listen address (default: :11435)"
            echo "  --upstream-url URL     Upstream Ollama URL (default: http://localhost:11434)"
            echo "  --calibration-file FILE Calibration file path"
            echo "  --binary-path PATH     Use existing binary (skip download)"
            echo "  --build                Build from source instead of downloading"
            echo "  --no-download          Don't download, use existing binary"
            echo "  --help, -h             Show this help message"
            echo ""
            echo "Examples:"
            echo "  sudo ./install.sh install              # Install latest from GitHub"
            echo "  sudo ./install.sh upgrade              # Upgrade to latest version"
            echo "  sudo ./install.sh install --version v1.1.0  # Install specific version"
            echo "  sudo ./install.sh install --build      # Build from source"
            echo "  sudo ./install.sh uninstall            # Uninstall"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Function to uninstall the service
uninstall_service() {
    echo "Uninstalling ollama-auto-ctx service..."
    echo "Service Name: $SERVICE_NAME"
    echo "User: $SERVICE_USER"
    echo "Install Dir: $INSTALL_DIR"
    echo "Data Dir: $DATA_DIR"
    echo "Log Dir: $LOG_DIR"
    echo "Config Dir: $CONFIG_DIR"
    echo ""

    # Check if running as root
    if [[ $EUID -ne 0 ]]; then
        echo "This script must be run as root (use sudo)"
        exit 1
    fi

    # Stop and disable service
    if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
        echo "Stopping service..."
        systemctl stop "$SERVICE_NAME"
    fi

    if systemctl is-enabled --quiet "$SERVICE_NAME" 2>/dev/null; then
        echo "Disabling service..."
        systemctl disable "$SERVICE_NAME"
    fi

    # Remove service file
    if [[ -f "/etc/systemd/system/$SERVICE_NAME.service" ]]; then
        echo "Removing service file..."
        rm -f "/etc/systemd/system/$SERVICE_NAME.service"
    fi

    # Reload systemd
    echo "Reloading systemd..."
    systemctl daemon-reload

    # Remove user and group
    if id "$SERVICE_USER" &>/dev/null; then
        echo "Removing service user..."
        userdel "$SERVICE_USER" 2>/dev/null || true
    fi

    # Remove directories (ask for confirmation)
    echo "Remove installation directories? (y/N)"
    read -r -p "" response
    case "$response" in
        [yY][eE][sS]|[yY])
            echo "Removing directories..."
            rm -rf "$INSTALL_DIR"
            rm -rf "$DATA_DIR"
            rm -rf "$LOG_DIR"
            rm -rf "$CONFIG_DIR"
            echo "Directories removed."
            ;;
        *)
            echo "Keeping directories. You can manually remove them later:"
            echo "  sudo rm -rf $INSTALL_DIR $DATA_DIR $LOG_DIR $CONFIG_DIR"
            ;;
    esac

    echo ""
    echo "Uninstallation complete!"
}

# Function to upgrade the service
upgrade_service() {
    echo "Upgrading ollama-auto-ctx..."
    
    # Check if running as root
    if [[ $EUID -ne 0 ]]; then
        echo "This script must be run as root (use sudo)"
        exit 1
    fi
    
    # Check current version
    get_installed_version
    echo "Current version: ${INSTALLED_VERSION:-not installed}"
    
    # Get target version
    if [[ -z "$VERSION" ]]; then
        get_latest_version
        VERSION="$LATEST_VERSION"
    fi
    echo "Target version: $VERSION"
    
    # Check if upgrade needed
    if [[ "$INSTALLED_VERSION" == "$VERSION" ]]; then
        echo "Already at version $VERSION, nothing to upgrade"
        exit 0
    fi
    
    # Detect platform and download
    detect_platform

    if ! download_release "$VERSION" "$OS" "$ARCH"; then
        echo "Failed to download release"
        exit 1
    fi
    
    # Stop service if running
    local was_running=false
    if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
        echo "Stopping service for upgrade..."
        systemctl stop "$SERVICE_NAME"
        was_running=true
    fi
    
    # Backup old binary
    if [[ -f "$INSTALL_DIR/ollama-auto-ctx" ]]; then
        echo "Backing up old binary..."
        cp "$INSTALL_DIR/ollama-auto-ctx" "$INSTALL_DIR/ollama-auto-ctx.backup"
    fi
    
    # Install new binary
    echo "Installing new binary..."
    cp "$BINARY_PATH" "$INSTALL_DIR/ollama-auto-ctx"
    chmod +x "$INSTALL_DIR/ollama-auto-ctx"
    chown "$SERVICE_USER:$SERVICE_USER" "$INSTALL_DIR/ollama-auto-ctx"
    
    # Save version
    echo "$VERSION" > "$INSTALL_DIR/.version"
    chown "$SERVICE_USER:$SERVICE_USER" "$INSTALL_DIR/.version"
    
    # Cleanup
    if [[ -n "$TMP_DIR" ]]; then
        rm -rf "$TMP_DIR"
    fi
    
    # Restart service if it was running
    if [[ "$was_running" == true ]]; then
        echo "Starting service..."
        systemctl start "$SERVICE_NAME"
    fi
    
    echo ""
    echo "Upgrade complete!"
    echo "  From: ${INSTALLED_VERSION:-unknown}"
    echo "  To:   $VERSION"
    echo ""
    echo "To check status:"
    echo "  sudo systemctl status $SERVICE_NAME"
}

# Function to install the service
install_service() {
    # Check if running as root
    if [[ $EUID -ne 0 ]]; then
        echo "This script must be run as root (use sudo)"
        exit 1
    fi

    # Check if already installed
    if [[ -f "$INSTALL_DIR/ollama-auto-ctx" ]]; then
        get_installed_version
        echo "ollama-auto-ctx is already installed (version: ${INSTALLED_VERSION:-unknown})"
        echo "Use 'upgrade' to update to a newer version"
        echo "Or 'uninstall' first, then 'install'"
        exit 1
    fi

    # Determine how to get the binary
    if [[ -n "$BINARY_PATH" ]]; then
        # Using provided binary
        if [[ ! -f "$BINARY_PATH" ]]; then
            echo "Binary not found at: $BINARY_PATH"
            exit 1
        fi
        echo "Using provided binary: $BINARY_PATH"
    elif [[ "$BUILD_BINARY" == true ]]; then
        # Building from source
        if ! command -v go &> /dev/null; then
            echo "Go is not installed. Please install Go first, or use --binary-path"
            exit 1
        fi
        echo "Will build from source"
    elif [[ "$DOWNLOAD_RELEASE" == true ]]; then
        # Downloading from GitHub
        detect_platform
        
        if [[ -z "$VERSION" ]]; then
            get_latest_version
            VERSION="$LATEST_VERSION"
        fi
        
        if ! download_release "$VERSION" "$OS" "$ARCH"; then
            echo ""
            echo "Failed to download release. Try one of:"
            echo "  --build          Build from source"
            echo "  --binary-path    Provide pre-built binary"
            exit 1
        fi
    else
        echo "No binary source specified"
        echo "Use --build, --binary-path, or allow download"
        exit 1
    fi

    echo ""
    echo "Installing ollama-auto-ctx..."
    echo "Service Name: $SERVICE_NAME"
    echo "User: $SERVICE_USER"
    echo "Install Dir: $INSTALL_DIR"
    echo "Data Dir: $DATA_DIR"
    echo "Log Dir: $LOG_DIR"
    echo "Config Dir: $CONFIG_DIR"
    echo "Listen Addr: $LISTEN_ADDR"
    echo "Upstream URL: $UPSTREAM_URL"
    if [[ -n "$VERSION" ]]; then
        echo "Version: $VERSION"
    fi
    echo ""

    # Create directories
    echo "Creating directories..."
    mkdir -p "$INSTALL_DIR"
    mkdir -p "$DATA_DIR"
    mkdir -p "$LOG_DIR"
    mkdir -p "$CONFIG_DIR"

    # Create service user if it doesn't exist
    if ! id "$SERVICE_USER" &>/dev/null; then
        echo "Creating service user: $SERVICE_USER"
        useradd --system --shell /bin/false --home "$DATA_DIR" --user-group "$SERVICE_USER"
    else
        echo "Service user $SERVICE_USER already exists"
    fi

    # Install the binary
    if [[ -n "$BINARY_PATH" ]]; then
        echo "Copying binary..."
        cp "$BINARY_PATH" "$INSTALL_DIR/ollama-auto-ctx"
        chmod +x "$INSTALL_DIR/ollama-auto-ctx"
    elif [[ "$BUILD_BINARY" == true ]]; then
        echo "Building ollama-auto-ctx..."
        cd "$(dirname "$0")"
        go build -o "$INSTALL_DIR/ollama-auto-ctx" ./cmd/ollama-auto-ctx
    fi

    # Save version if known
    if [[ -n "$VERSION" ]]; then
        echo "$VERSION" > "$INSTALL_DIR/.version"
    fi

    # Cleanup temp directory
    if [[ -n "$TMP_DIR" ]]; then
        rm -rf "$TMP_DIR"
    fi

    # Set ownership
    chown -R "$SERVICE_USER:$SERVICE_USER" "$INSTALL_DIR"
    chown -R "$SERVICE_USER:$SERVICE_USER" "$DATA_DIR"
    chown -R "$SERVICE_USER:$SERVICE_USER" "$LOG_DIR"
    chown "$SERVICE_USER:$SERVICE_USER" "$CONFIG_DIR"

    # Create environment file
    cat > "$CONFIG_DIR/environment" << EOF
# ollama-auto-ctx environment configuration
# Edit this file to configure the proxy, then restart the service

# Mode: off, monitor, retry (default), protect
MODE=retry

# Network
LISTEN_ADDR=$LISTEN_ADDR
UPSTREAM_URL=$UPSTREAM_URL

# Storage
STORAGE=sqlite
STORAGE_PATH=$DATA_DIR/oac.sqlite
STORAGE_MAX_ROWS=3000

# Context sizing
MIN_CTX=1024
MAX_CTX=81920
BUCKETS=1024,2048,4096,8192,9216,10240,11264,12288,13312,14336,15360,16384,20480,24576,28672,32768,36864,40960,45056,49152,53248,57344,61440,65536,69632,73728,77824,81920,86016,90112,94208,98304,102400
HEADROOM=1.25

# Output budgeting
DEFAULT_OUTPUT_BUDGET=1024
MAX_OUTPUT_BUDGET=10240
DYNAMIC_DEFAULT_OUTPUT_BUDGET=false

# Override behavior
OVERRIDE_NUM_CTX=if_too_small

# Calibration
CALIBRATION_ENABLED=true
CALIBRATION_FILE=$CALIBRATION_FILE

# Performance
REQUEST_BODY_MAX_BYTES=10485760
RESPONSE_TAP_MAX_BYTES=5242880
SHOW_CACHE_TTL=5m
FLUSH_INTERVAL=100ms

# HTTP
CORS_ALLOW_ORIGIN=*

# Logging
LOG_LEVEL=info
EOF

    chown "$SERVICE_USER:$SERVICE_USER" "$CONFIG_DIR/environment"
    chmod 600 "$CONFIG_DIR/environment"

    # Create systemd service file
    cat > "/etc/systemd/system/$SERVICE_NAME.service" << EOF
[Unit]
Description=Ollama Context Proxy - Automatic context window optimization
After=network.target
Wants=network.target

[Service]
Type=simple
User=$SERVICE_USER
Group=$SERVICE_USER
EnvironmentFile=$CONFIG_DIR/environment
ExecStart=$INSTALL_DIR/ollama-auto-ctx
Restart=always
RestartSec=5

# Security settings
NoNewPrivileges=yes
PrivateTmp=yes
ProtectHome=yes
ProtectSystem=strict
ReadWritePaths=$DATA_DIR $LOG_DIR
WorkingDirectory=$DATA_DIR

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=$SERVICE_NAME

[Install]
WantedBy=multi-user.target
EOF

    # Reload systemd and enable service
    echo "Reloading systemd..."
    systemctl daemon-reload

    echo "Enabling service..."
    systemctl enable "$SERVICE_NAME"

    echo ""
    echo "Installation complete!"
    if [[ -n "$VERSION" ]]; then
        echo "Version: $VERSION"
    fi
    echo ""
    echo "Configuration file: $CONFIG_DIR/environment"
    echo "Service file: /etc/systemd/system/$SERVICE_NAME.service"
    echo ""
    echo "To start the service:"
    echo "  sudo systemctl start $SERVICE_NAME"
    echo ""
    echo "To check status:"
    echo "  sudo systemctl status $SERVICE_NAME"
    echo ""
    echo "To view logs:"
    echo "  sudo journalctl -u $SERVICE_NAME -f"
    echo ""
    echo "To edit configuration:"
    echo "  sudo nano $CONFIG_DIR/environment"
    echo "  sudo systemctl restart $SERVICE_NAME"
    echo ""
    echo "Dashboard available at: http://localhost${LISTEN_ADDR}/dashboard"
    echo "Proxying to Ollama at: $UPSTREAM_URL"
    echo ""
    echo "To upgrade later:"
    echo "  sudo $0 upgrade"
    echo ""
    echo "To uninstall:"
    echo "  sudo $0 uninstall"
}

# Main execution logic
case "$ACTION" in
    install)
        install_service
        ;;
    upgrade|update)
        upgrade_service
        ;;
    uninstall)
        uninstall_service
        ;;
    *)
        echo "Unknown action: $ACTION"
        echo "Use 'install', 'upgrade', or 'uninstall'"
        exit 1
        ;;
esac
