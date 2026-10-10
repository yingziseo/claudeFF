#!/bin/bash
set -euo pipefail
[ "$(id -u)" = 0 ] || exit 1
port="$1"
[[ "$port" =~ ^[0-9]+$ ]] && (( port > 0 && port < 65536 )) || exit 1
uid=$(id -u claudeff)
# Separate IPv4/IPv6 chains; limits apply to the entire unprivileged UID and all children.
for tool in iptables ip6tables; do
 "$tool" -w -N CLAUDEFF 2>/dev/null || true
 "$tool" -w -F CLAUDEFF
 "$tool" -w -C OUTPUT -m owner --uid-owner "$uid" -j CLAUDEFF 2>/dev/null || "$tool" -w -I OUTPUT 1 -m owner --uid-owner "$uid" -j CLAUDEFF
 if [ "$tool" = iptables ]; then
  "$tool" -w -A CLAUDEFF -d 127.0.0.1/32 -p tcp --dport "$port" -j ACCEPT
 fi
 "$tool" -w -A CLAUDEFF -j REJECT
 "$tool" -w -C OUTPUT -m owner --uid-owner "$uid" -j CLAUDEFF
 done
