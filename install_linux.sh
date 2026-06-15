#!/usr/bin/env bash
set -euo pipefail

APP_NAME="zapretstatd"
INSTALL_DIR="/usr/local/bin"
SERVICE_FILE="/etc/systemd/system/${APP_NAME}.service"
BUILD_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# ── Colors ─────────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
info()    { echo -e "${GREEN}[INFO]${NC}  $*"; }
warn()    { echo -e "${YELLOW}[WARN]${NC}  $*"; }
error()   { echo -e "${RED}[ERROR]${NC} $*" >&2; exit 1; }

# ── Checks ─────────────────────────────────────────────────────────────────────
[[ $EUID -ne 0 ]] && error "Run as root: sudo $0"
command -v go &>/dev/null || error "Go is not installed. Install from https://go.dev/dl/"

# ── Build ──────────────────────────────────────────────────────────────────────
info "Building ${APP_NAME}..."
cd "${BUILD_DIR}"
go build -o "/tmp/${APP_NAME}" ./zapretstatd/main.go
install -m 755 "/tmp/${APP_NAME}" "${INSTALL_DIR}/${APP_NAME}"
rm -f "/tmp/${APP_NAME}"
info "Binary installed to ${INSTALL_DIR}/${APP_NAME}"

# ── Systemd service ────────────────────────────────────────────────────────────
info "Creating systemd service..."
cat > "${SERVICE_FILE}" <<EOF
[Unit]
Description=Zapretstat Daemon
After=network.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/${APP_NAME}
Restart=on-failure
RestartSec=5s
StandardOutput=journal
StandardError=journal
SyslogIdentifier=${APP_NAME}

# Hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable --now "${APP_NAME}.service"

info "Service status:"
systemctl status "${APP_NAME}.service" --no-pager || true

echo
info "Done! Useful commands:"
echo "  sudo systemctl status  ${APP_NAME}"
echo "  sudo systemctl restart ${APP_NAME}"
echo "  sudo journalctl -u ${APP_NAME} -f"
