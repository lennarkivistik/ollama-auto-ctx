#!/bin/bash

# install.sh - Install/uninstall ollama-auto-ctx as a systemd service
# Usage: ./install.sh [install|uninstall] [OPTIONS]

set -e

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
BUILD_BINARY=true

# Parse command line options
while [[ $# -gt 0 ]]; do
    case $1 in
        install)
            ACTION="install"
            shift
            ;;
        uninstall)
            ACTION="uninstall"
            shift
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
            BUILD_BINARY=false
            shift 2
            ;;
        --no-build)
            BUILD_BINARY=false
            shift
            ;;
        --help|-h)
            echo "Usage: $0 [install|uninstall] [OPTIONS]"
            echo ""
            echo "Install or uninstall ollama-auto-ctx as a systemd service"
            echo ""
            echo "Actions:"
            echo "  install    Install the service (default)"
            echo "  uninstall  Uninstall the service"
            echo ""
            echo "Options:"
            echo "  --service-name NAME    Service name (default: ollama-auto-ctx)"
            echo "  --user USER            Service user (default: ollama-auto-ctx)"
            echo "  --install-dir DIR      Installation directory (default: /opt/ollama-auto-ctx)"
            echo "  --data-dir DIR         Data directory (default: /var/lib/ollama-auto-ctx)"
            echo "  --log-dir DIR          Log directory (default: /var/log/ollama-auto-ctx)"
            echo "  --config-dir DIR       Config directory (default: /etc/ollama-auto-ctx)"
            echo "  --listen-addr ADDR     Listen address (default: :11435)"
            echo "  --upstream-url URL     Upstream Ollama URL (default: http://localhost:11434)"
            echo "  --calibration-file FILE Calibration file path (default: /var/lib/ollama-auto-ctx/calibration.json)"
            echo "  --binary-path PATH     Path to existing binary (skips build)"
            echo "  --no-build             Skip building, use existing binary in install dir"
            echo "  --help, -h             Show this help message"
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

# Function to install the service
install_service() {
    # Check if running as root
    if [[ $EUID -ne 0 ]]; then
        echo "This script must be run as root (use sudo)"
        exit 1
    fi

    # Check if Go is installed (only if building)
    if [[ "$BUILD_BINARY" == true ]] && ! command -v go &> /dev/null; then
        echo "Go is not installed. Please install Go first, or use --binary-path to specify an existing binary."
        exit 1
    fi

    # Check binary path
    if [[ -n "$BINARY_PATH" ]] && [[ ! -f "$BINARY_PATH" ]]; then
        echo "Binary not found at: $BINARY_PATH"
        exit 1
    fi

    echo "Installing ollama-auto-ctx..."
    echo "Service Name: $SERVICE_NAME"
    echo "User: $SERVICE_USER"
    echo "Install Dir: $INSTALL_DIR"
    echo "Data Dir: $DATA_DIR"
    echo "Log Dir: $LOG_DIR"
    echo "Config Dir: $CONFIG_DIR"
    echo "Listen Addr: $LISTEN_ADDR"
    echo "Upstream URL: $UPSTREAM_URL"
    echo "Calibration File: $CALIBRATION_FILE"
    if [[ -n "$BINARY_PATH" ]]; then
        echo "Binary Path: $BINARY_PATH"
    else
        echo "Build Binary: $BUILD_BINARY"
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
    echo "Copying binary from $BINARY_PATH..."
    cp "$BINARY_PATH" "$INSTALL_DIR/ollama-auto-ctx"
    chmod +x "$INSTALL_DIR/ollama-auto-ctx"
elif [[ "$BUILD_BINARY" == true ]]; then
    echo "Building ollama-auto-ctx..."
    cd "$(dirname "$0")"
    go build -o "$INSTALL_DIR/ollama-auto-ctx" ./cmd/ollama-auto-ctx
else
    echo "Using existing binary in $INSTALL_DIR/ollama-auto-ctx"
    if [[ ! -f "$INSTALL_DIR/ollama-auto-ctx" ]]; then
        echo "Error: Binary not found at $INSTALL_DIR/ollama-auto-ctx"
        echo "Use --binary-path to specify a binary, or remove --no-build to build from source"
        exit 1
    fi
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

# Network
LISTEN_ADDR=$LISTEN_ADDR
UPSTREAM_URL=$UPSTREAM_URL

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
echo "Service will be available at: $LISTEN_ADDR"
echo "Proxying to Ollama at: $UPSTREAM_URL"
echo ""
echo "To uninstall:"
echo "  sudo $0 uninstall"

}

# Main execution logic
case "$ACTION" in
    install)
        install_service
        ;;
    uninstall)
        uninstall_service
        ;;
    *)
        echo "Unknown action: $ACTION"
        echo "Use 'install' or 'uninstall'"
        exit 1
        ;;
esac
