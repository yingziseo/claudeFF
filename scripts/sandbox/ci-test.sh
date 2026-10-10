#!/bin/bash
set -euo pipefail
# Runs ONLY in an ephemeral GitHub Linux runner, never the user's server.
uid=$(id -u claudeff)
root_token=/root/claudeff-test-private
printf 'must-not-be-readable' > "$root_token"
export CLAUDEFF_PROXY=http://127.0.0.1:18765 CLAUDEFF_LANG=en_US.UTF-8 CLAUDEFF_TZ=UTC
bash scripts/sandbox/firewall.sh 18765
check(){ runuser -u claudeff -- env CLAUDEFF_PROXY="$CLAUDEFF_PROXY" CLAUDEFF_LANG="$CLAUDEFF_LANG" CLAUDEFF_TZ="$CLAUDEFF_TZ" bash /opt/claudeff/launch.sh bash -c "$1"; }
check 'test ! -e /mnt/c && test ! -e /root/claudeff-test-private && test ! -e /run/WSL && test ! -e /home/runner && test ! -e /opt/claudeff/wsl-agent'
check 'printf project-ok >/workspace/check; test "$(cat /workspace/check)" = project-ok'
check '! curl --noproxy "*" --connect-timeout 2 --max-time 3 -s http://1.1.1.1 >/dev/null'
check '! curl --noproxy "*" --connect-timeout 2 --max-time 3 -s http://[2606:4700:4700::1111] >/dev/null'
check '! iptables -F OUTPUT 2>/dev/null; test "$(id -u)" != 0'
echo 'Isolation checks passed: permitted workspace; denied host paths, root secret, direct IPv4/IPv6 and firewall mutation.'
python3 - <<'PY' &
from http.server import HTTPServer,BaseHTTPRequestHandler
class Handler(BaseHTTPRequestHandler):
 def do_GET(self):
  self.send_response(200);self.end_headers();self.wfile.write(b'managed-proxy-ok')
 def log_message(self,*args):pass
HTTPServer(('127.0.0.1',18765),Handler).serve_forever()
PY
relay_pid=$!
trap 'kill "$relay_pid" 2>/dev/null || true' EXIT
sleep 1
check 'test "$(curl --noproxy "" --proxy "$HTTPS_PROXY" --fail --silent http://example.com)" = managed-proxy-ok'
echo 'Allowed managed proxy route passed.'
