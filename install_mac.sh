#!/usr/bin/env bash
set -euo pipefail

APP_NAME="zapretstatd"
INSTALL_DIR="/usr/local/bin"
LABEL="com.zapretstat.daemon"
PLIST_DIR="${HOME}/Library/LaunchAgents"
PLIST_FILE="${PLIST_DIR}/${LABEL}.plist"
LOG_DIR="${HOME}/Library/Logs/zapretstat"
BUILD_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# ── Colors ─────────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
info()  { echo -e "${GREEN}[INFO]${NC}  $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*" >&2; exit 1; }

# ── Checks ─────────────────────────────────────────────────────────────────────
command -v go &>/dev/null || error "Go is not installed. Install from https://go.dev/dl/ or: brew install go"

# ── Build ──────────────────────────────────────────────────────────────────────
info "Building ${APP_NAME}..."
cd "${BUILD_DIR}"
go build -o "/tmp/${APP_NAME}" ./zapretstatd/main.go

# /usr/local/bin может требовать sudo на macOS
if [[ -w "${INSTALL_DIR}" ]]; then
    install -m 755 "/tmp/${APP_NAME}" "${INSTALL_DIR}/${APP_NAME}"
else
    warn "${INSTALL_DIR} is not writable, using sudo..."
    sudo install -m 755 "/tmp/${APP_NAME}" "${INSTALL_DIR}/${APP_NAME}"
fi
rm -f "/tmp/${APP_NAME}"
info "Binary installed to ${INSTALL_DIR}/${APP_NAME}"

# ── Stop existing service ──────────────────────────────────────────────────────
if launchctl list | grep -q "${LABEL}" 2>/dev/null; then
    warn "Stopping existing service..."
    launchctl unload "${PLIST_FILE}" 2>/dev/null || true
fi

# ── LaunchAgent plist ──────────────────────────────────────────────────────────
info "Creating LaunchAgent..."
mkdir -p "${PLIST_DIR}" "${LOG_DIR}"

cat > "${PLIST_FILE}" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
    "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>${LABEL}</string>

    <key>ProgramArguments</key>
    <array>
        <string>${INSTALL_DIR}/${APP_NAME}</string>
    </array>

    <key>RunAtLoad</key>
    <true/>

    <key>KeepAlive</key>
    <dict>
        <key>SuccessfulExit</key>
        <false/>
    </dict>

    <key>ThrottleInterval</key>
    <integer>5</integer>

    <key>StandardOutPath</key>
    <string>${LOG_DIR}/stdout.log</string>

    <key>StandardErrorPath</key>
    <string>${LOG_DIR}/stderr.log</string>
</dict>
</plist>
EOF

launchctl load -w "${PLIST_FILE}"
info "Service loaded and started."

echo
info "Done! Useful commands:"
echo "  launchctl list | grep zapretstat          # check status"
echo "  launchctl stop  ${LABEL}   # stop"
echo "  launchctl start ${LABEL}   # start"
echo "  tail -f ${LOG_DIR}/stdout.log"
echo "  tail -f ${LOG_DIR}/stderr.log"
