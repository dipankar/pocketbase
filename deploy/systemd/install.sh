#!/bin/bash
# PocketBase Enterprise Installation Script
# Run as root on each server

set -e

POCKETBASE_VERSION="${POCKETBASE_VERSION:-latest}"
INSTALL_DIR="/usr/local/bin"
DATA_DIR="/var/lib/pocketbase"
CONFIG_DIR="/etc/pocketbase"
LOG_DIR="/var/log/pocketbase"

echo "=== PocketBase Enterprise Installation ==="

# Check root
if [ "$EUID" -ne 0 ]; then
    echo "Please run as root"
    exit 1
fi

# Create pocketbase user
if ! id "pocketbase" &>/dev/null; then
    echo "Creating pocketbase user..."
    useradd --system --no-create-home --shell /bin/false pocketbase
fi

# Create directories
echo "Creating directories..."
mkdir -p "$DATA_DIR/control-plane"
mkdir -p "$DATA_DIR/tenant-node"
mkdir -p "$DATA_DIR/gateway"
mkdir -p "$CONFIG_DIR"
mkdir -p "$LOG_DIR"

# Set permissions
chown -R pocketbase:pocketbase "$DATA_DIR"
chown -R pocketbase:pocketbase "$LOG_DIR"
chmod 750 "$DATA_DIR"
chmod 750 "$LOG_DIR"

# Download PocketBase binary (if not exists)
if [ ! -f "$INSTALL_DIR/pocketbase" ]; then
    echo "Downloading PocketBase..."
    # Replace with actual download URL when available
    # wget -O /tmp/pocketbase.zip "https://github.com/pocketbase/pocketbase/releases/download/v${POCKETBASE_VERSION}/pocketbase_${POCKETBASE_VERSION}_linux_amd64.zip"
    # unzip /tmp/pocketbase.zip -d "$INSTALL_DIR"
    # chmod +x "$INSTALL_DIR/pocketbase"
    echo "NOTE: Copy the pocketbase binary to $INSTALL_DIR/pocketbase manually"
fi

# Copy systemd service files
echo "Installing systemd services..."
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cp "$SCRIPT_DIR/pocketbase-control-plane.service" /etc/systemd/system/
cp "$SCRIPT_DIR/pocketbase-tenant-node.service" /etc/systemd/system/
cp "$SCRIPT_DIR/pocketbase-gateway.service" /etc/systemd/system/

# Copy example environment files
echo "Installing configuration templates..."
cp "$SCRIPT_DIR/control-plane.env.example" "$CONFIG_DIR/"
cp "$SCRIPT_DIR/tenant-node.env.example" "$CONFIG_DIR/"
cp "$SCRIPT_DIR/gateway.env.example" "$CONFIG_DIR/"

# Reload systemd
systemctl daemon-reload

echo ""
echo "=== Installation Complete ==="
echo ""
echo "Next steps:"
echo "1. Copy pocketbase binary to $INSTALL_DIR/pocketbase"
echo "2. Configure environment files in $CONFIG_DIR/"
echo "   - For control plane: cp control-plane.env.example control-plane.env"
echo "   - For tenant node:   cp tenant-node.env.example tenant-node.env"
echo "   - For gateway:       cp gateway.env.example gateway.env"
echo "3. Start services:"
echo "   - Control Plane: systemctl enable --now pocketbase-control-plane"
echo "   - Tenant Node:   systemctl enable --now pocketbase-tenant-node"
echo "   - Gateway:       systemctl enable --now pocketbase-gateway"
echo ""
echo "View logs: journalctl -u pocketbase-control-plane -f"
