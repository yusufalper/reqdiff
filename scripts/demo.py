#!/usr/bin/env python3
"""Two synthetic API environments for the README demo. No dependencies."""
import argparse
import json
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path not in ("/staging/users", "/production/users"):
            self.send_error(404)
            return
        staging = self.path.startswith("/staging/")
        time.sleep(0.06 if staging else 0.16)
        user = {"id": 42, "name": "Alex", "username": "alex"} if staging else {
            "id": "42", "name": "Alex", "avatar": "https://example.com/alex.png"
        }
        body = json.dumps({"user": user}).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *_):
        pass


def make_server(port=8787):
    return ThreadingHTTPServer(("127.0.0.1", port), Handler)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--port", type=int, default=8787)
    args = parser.parse_args()
    server = make_server(args.port)
    print(f"Demo API on http://127.0.0.1:{server.server_port} (Ctrl+C to stop)", flush=True)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()
