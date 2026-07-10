from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
import time


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path in ("/health", "/ready", "/"):
            self._json(200, {
                "service": "ai-services-placeholder",
                "status": "placeholder",
                "message": "AI/OCR worker runtime is not configured in this Story.",
                "mock": False,
                "timestamp": int(time.time()),
            })
            return
        self._json(404, {"error": "not_found"})

    def log_message(self, fmt, *args):
        return

    def _json(self, status, payload):
        body = json.dumps(payload).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)


def main():
    port = int(os.getenv("EDUGRADE_AI_PLACEHOLDER_PORT", "8100"))
    ThreadingHTTPServer(("0.0.0.0", port), Handler).serve_forever()


if __name__ == "__main__":
    main()
