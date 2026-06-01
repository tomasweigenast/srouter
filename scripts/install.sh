#!/bin/sh
# Install srouter on Alpine Linux.
# Run as root after extracting the tar.gz:
#   tar xzf srouter-linux.tar.gz && sh install.sh

set -e

BINARY=./srouter-linux
DEST=/usr/local/bin/srouter
SERVICE=/etc/init.d/srouter
CONFIG_DIR=/etc/srouter
DATA_DIR=/var/lib/srouter

if [ "$(id -u)" != "0" ]; then
  echo "ERROR: must run as root" >&2
  exit 1
fi

echo "==> Installing srouter..."

if ! command -v apk >/dev/null 2>&1; then
  echo "ERROR: this script is for Alpine Linux only" >&2
  exit 1
fi

# ── Runtime dependencies ─────────────────────────────────────────────────────
echo "  -> installing dependencies..."
apk add --no-cache \
  linux-pam \
  curl \
  iptables \
  stubby \
  dnscrypt-proxy \
  >/dev/null

# ── Logging daemons (needed for /var/log/messages) ───────────────────────────
# syslog writes all service logs to /var/log/messages
# klogd forwards kernel messages (iptables LOG rules) into syslog
echo "  -> enabling syslog and klogd..."
rc-update add syslog default 2>/dev/null || true
rc-update add klogd  default 2>/dev/null || true
rc-service syslog start 2>/dev/null || true
rc-service klogd  start 2>/dev/null || true

# ── Stop any running instance before replacing the binary ────────────────────
echo "  -> stopping service..."
rc-service srouter stop 2>/dev/null || true
pkill -f "${DEST}" 2>/dev/null || true
sleep 1
# Clear stale OpenRC state and PID file so 'start' always works
rc-service srouter zap 2>/dev/null || true
rm -f /run/srouter.pid

# ── Binary ───────────────────────────────────────────────────────────────────
echo "  -> copying binary..."
cp "${BINARY}" "${DEST}"
chmod +x "${DEST}"

# ── Directories ──────────────────────────────────────────────────────────────
echo "  -> creating directories..."
mkdir -p "${DATA_DIR}" "${CONFIG_DIR}"

# ── Default config (skip if already exists) ──────────────────────────────────
if [ ! -f "${CONFIG_DIR}/config.toml" ]; then
  echo "  -> writing default config..."
  cat > "${CONFIG_DIR}/config.toml" << 'EOF'
port               = ":8080"
db_path            = "/var/lib/srouter/data.db"
update_interval_ms = 2000
EOF
fi

# ── OpenRC service (skip if already exists) ──────────────────────────────────
if [ ! -f "${SERVICE}" ]; then
  echo "  -> installing OpenRC service..."
  cat > "${SERVICE}" << 'EOF'
#!/sbin/openrc-run

command="/usr/local/bin/srouter"
command_background=true
pidfile="/run/srouter.pid"
output_log="/var/log/srouter.log"
error_log="/var/log/srouter.log"

depend() {
    need net
    after dnsmasq firewall
}
EOF
  chmod +x "${SERVICE}"
  rc-update add srouter default 2>/dev/null || true
fi

# ── Start ────────────────────────────────────────────────────────────────────
echo "  -> starting service..."
rc-service srouter start

echo ""
echo "==> srouter installed and running."
echo "    Dashboard: http://192.168.0.1:8080"
echo "    Logs:      tail -f /var/log/srouter.log"
echo "    Config:    ${CONFIG_DIR}/config.toml"

# ── Cleanup installer files ──────────────────────────────────────────────────
rm -f "${BINARY}" srouter-linux.tar.gz "$0"
