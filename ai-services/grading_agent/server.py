from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import hmac
import json

from .app import GradingAgentApplication
from .config import Settings
from .errors import AgentError


class GradingAgentHTTPServer(ThreadingHTTPServer):
    daemon_threads = True
    allow_reuse_address = True

    def __init__(self, address, application):
        super().__init__(address, GradingAgentHandler)
        self.application = application


class GradingAgentHandler(BaseHTTPRequestHandler):
    server_version = "EduGradeGradingAgent/1"

    def do_GET(self):
        if self.path == "/health":
            self._json(
                200,
                {
                    "service": "grading-agent",
                    "status": "healthy",
                    "mode": "shadow",
                    "capability_profile": self.server.application.matrix.profile_id,
                },
            )
            return
        if self.path == "/ready":
            ready = self.server.application.readiness()
            self._json(
                200 if ready else 503,
                {
                    "service": "grading-agent",
                    "status": "ready" if ready else "not_ready",
                    "model_runtime": "available" if ready else "unavailable",
                },
            )
            return
        self._error(AgentError("invalid_request", "route not found", status=404))

    def do_POST(self):
        if self.path != "/grading/grade":
            self._error(AgentError("invalid_request", "route not found", status=404))
            return
        try:
            self._authorize()
            payload = self._read_json()
            request_id = payload.get("request_id", "") if isinstance(payload, dict) else ""
            idempotency_key = self.headers.get("Idempotency-Key", "").strip()
            if not idempotency_key:
                raise AgentError(
                    "invalid_request",
                    "Idempotency-Key header is required",
                    status=400,
                    request_id=request_id,
                )
            suggestion, replayed = self.server.application.grade(payload, idempotency_key)
            self._json(200, suggestion, extra_headers={"Idempotent-Replay": "true" if replayed else "false"})
        except AgentError as exc:
            self._error(exc)
        except Exception:
            self._error(AgentError("internal_error", "grading-agent operation failed", status=500))

    def _authorize(self):
        expected = self.server.application.settings.service_token
        supplied = self.headers.get("Authorization", "")
        prefix = "Bearer "
        candidate = supplied[len(prefix) :] if supplied.startswith(prefix) else ""
        if not candidate or not hmac.compare_digest(candidate, expected):
            raise AgentError("unauthorized", "valid service authentication is required", status=401)

    def _read_json(self):
        content_type = self.headers.get("Content-Type", "").split(";", 1)[0].strip().lower()
        if content_type != "application/json":
            raise AgentError("invalid_request", "Content-Type must be application/json", status=415)
        raw_length = self.headers.get("Content-Length", "")
        try:
            length = int(raw_length)
        except ValueError as exc:
            raise AgentError("invalid_request", "Content-Length is required", status=411) from exc
        if length <= 0 or length > self.server.application.settings.max_request_bytes:
            raise AgentError("invalid_request", "request body size is invalid", status=413)
        body = self.rfile.read(length)
        try:
            return json.loads(body.decode("utf-8"))
        except (UnicodeDecodeError, json.JSONDecodeError) as exc:
            raise AgentError("invalid_request", "request body must be valid UTF-8 JSON", status=400) from exc

    def _error(self, error):
        self._json(error.status, error.payload())

    def _json(self, status, payload, extra_headers=None):
        body = json.dumps(payload, ensure_ascii=False, separators=(",", ":")).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.send_header("Cache-Control", "no-store")
        self.send_header("X-Content-Type-Options", "nosniff")
        for name, value in (extra_headers or {}).items():
            self.send_header(name, value)
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, _format, *_args):
        return


def serve(settings=None):
    settings = settings or Settings.from_env()
    application = GradingAgentApplication(settings)
    server = GradingAgentHTTPServer((settings.host, settings.port), application)
    server.serve_forever()
