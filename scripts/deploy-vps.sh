#!/usr/bin/env bash
set -euo pipefail

# Deploy Marrow to a VPS with systemd
# Run as root on the target server

BINARY_URL="${BINARY_URL:-}"
USER_NAME="marrow"
WORK_DIR="/var/lib/marrow"
CONFIG_DIR="/etc/marrow"

if [ "$EUID" -ne 0 ]; then
    echo "Please run as root"
    exit 1
fi

echo "==> Creating user and directories"
if ! id -u "$USER_NAME" &>/dev/null; then
    useradd --system --home-dir "$WORK_DIR" --create-home "$USER_NAME"
fi

mkdir -p "$WORK_DIR" "$CONFIG_DIR" "$WORK_DIR/data"
chown -R "$USER_NAME:$USER_NAME" "$WORK_DIR"

if [ -n "$BINARY_URL" ]; then
    echo "==> Downloading binary from $BINARY_URL"
    curl -sSL -o /usr/local/bin/marrow "$BINARY_URL"
else
    echo "==> Please copy the 'marrow' binary to /usr/local/bin/marrow manually"
fi

chmod +x /usr/local/bin/marrow

echo "==> Installing systemd service"
cp marrow.service /etc/systemd/system/marrow.service
systemctl daemon-reload
systemctl enable marrow

echo "==> Setting up firewall"
if command -v ufw &>/dev/null; then
    ufw allow 80/tcp
    ufw allow 443/tcp
    echo "UFW rules added. Run 'ufw enable' to activate if not already enabled."
fi

echo "==> Done. Next steps:"
echo "   1. Create $CONFIG_DIR/env or $WORK_DIR/config.toml"
echo "   2. Run: systemctl start marrow"
echo "   3. Check logs: journalctl -u marrow -f"
