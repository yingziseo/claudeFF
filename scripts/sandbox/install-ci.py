"""Cloud-only integration: authenticated gateway -> actual agent -> official Code install.
Never run on the user's Linux server. The entire runner is disposable.
"""
import base64
import json
import os
import select
import socket
import subprocess
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

class Gateway(BaseHTTPRequestHandler):
    def log_message(self, *_):
        pass
    def do_CONNECT(self):
        if self.headers.get('Proxy-Authorization') != 'Basic ' + base64.b64encode(b'ci:fixture').decode():
            self.send_error(407)
            return
        host, port = self.path.rsplit(':', 1)
        try:
            upstream = socket.create_connection((host.strip('[]'), int(port)), 20)
        except OSError:
            self.send_error(502)
            return
        self.send_response(200)
        self.end_headers()
        self.wfile.flush()
        self.close_connection = True
        with upstream:
            while True:
                readable, _, _ = select.select([self.connection, upstream], [], [], 30)
                if not readable:
                    return
                for current in readable:
                    try:
                        data = current.recv(65536)
                        if not data:
                            return
                        (upstream if current is self.connection else self.connection).sendall(data)
                    except OSError:
                        return

server = ThreadingHTTPServer(('127.0.0.1', 0), Gateway)
threading.Thread(target=server.serve_forever, daemon=True).start()
config = {
    'mode': 'install', 'revision': 1,
    'proxy': {'protocol': 'http', 'host': '127.0.0.1', 'port': server.server_port, 'username': 'ci', 'password': 'fixture'},
    'profile': {'id': 'cloud-install-test', 'revision': 1, 'source': 'browser', 'seed': '102468', 'language': 'en-US', 'timezone': 'UTC'},
}
process = subprocess.Popen(['/opt/claudeff/wsl-agent'], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, text=True)
process.stdin.write(json.dumps(config) + '\n')
process.stdin.flush()
ready = False
for line in process.stdout:
    try:
        event = json.loads(line)
    except json.JSONDecodeError:
        continue
    if event['type'] == 'error':
        raise RuntimeError(event['data'])
    if event['type'] == 'progress':
        print(event['data'], flush=True)
    if event['type'] == 'version':
        print('Official Claude Code version: ' + event['data'], flush=True)
    if event['type'] == 'ready':
        ready = True
if process.wait(timeout=10) != 0 or not ready:
    raise RuntimeError('official Code installation did not complete')
process.stdin.close()
server.shutdown()
print('Actual official Code install and restricted --version startup passed. Windows WSL2 integration and account login remain separate checks.', flush=True)
