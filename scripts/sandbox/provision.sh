#!/bin/bash
set -euo pipefail
# Only runs inside the application's dedicated imported WSL2 distribution.
[ "$(id -u)" = 0 ] || exit 1
[ -n "${CLAUDEFF_PROXY:-}" ] || { echo 'missing managed proxy'; exit 1; }
export DEBIAN_FRONTEND=noninteractive
apt-get -o Acquire::http::Proxy="$CLAUDEFF_PROXY" -o Acquire::https::Proxy="$CLAUDEFF_PROXY" update >/dev/null
apt-get -o Acquire::http::Proxy="$CLAUDEFF_PROXY" -o Acquire::https::Proxy="$CLAUDEFF_PROXY" install -y --no-install-recommends bubblewrap iptables curl ca-certificates git locales >/dev/null
id claudeff >/dev/null 2>&1 || useradd --create-home --shell /bin/bash claudeff
locale-gen en_US.UTF-8 zh_CN.UTF-8 en_GB.UTF-8 ja_JP.UTF-8 >/dev/null
mkdir -p /opt/claudeff /workspace
chown claudeff:claudeff /workspace
chmod 700 /home/claudeff
cat > /etc/wsl.conf <<'CONFIG'
[automount]
enabled=false
mountFsTab=false
[interop]
enabled=false
appendWindowsPath=false
[time]
useWindowsTimezone=false
[user]
default=claudeff
CONFIG
# Changes take effect at the dedicated distribution's next termination/restart.
