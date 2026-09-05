"""Request-scoped models for authenticated paper parsing, never grading policy."""

import http.client
import ipaddress
import json
import socket
import ssl
from dataclasses import replace
from urllib.parse import urlsplit

from .errors import AgentError
from .model import DashScopeNativeAdapter, LocalLlamaCppAdapter


class ManagedNativePaperAdapter(DashScopeNativeAdapter):
    def request_structured(self, request_id, messages, schema, name):
        messages = [*messages, {"role": "system", "content": "Return a JSON object matching this schema: " + json.dumps(schema, ensure_ascii=False)}]
        return super().request_structured(request_id, messages, schema, name)


def _endpoint(value):
    parsed = urlsplit(value)
    if (parsed.scheme != "https" or not parsed.hostname or parsed.username is not None
            or parsed.password is not None or parsed.query or parsed.fragment):
        raise ValueError("invalid managed endpoint")
    _ = parsed.port
    return parsed


def public_json_transport(url, payload, headers, timeout):
    """Pin a public DNS address while preserving TLS hostname verification."""
    parsed = _endpoint(url)
    addresses = socket.getaddrinfo(parsed.hostname, parsed.port or 443, type=socket.SOCK_STREAM)
    if not addresses or any(not ipaddress.ip_address(item[4][0]).is_global for item in addresses):
        raise AgentError("model_request_rejected", "provider must use a public HTTPS endpoint", status=502)
    address = addresses[0][4]
    body = json.dumps(payload, ensure_ascii=False, allow_nan=False).encode("utf-8")
    if len(body) > 2_000_000:
        raise AgentError("invalid_request", "document model request is too large", status=413)
    connection = http.client.HTTPSConnection(parsed.hostname, parsed.port or 443, timeout=timeout, context=ssl.create_default_context())
    # HTTPSConnection retains the original hostname for SNI/certificate checks;
    # only the socket destination is replaced with the address validated above.
    connection._create_connection = lambda _target, timeout, source_address=None: socket.create_connection(address[:2], timeout, source_address)
    try:
        connection.request("POST", parsed.path or "/", body=body, headers=headers)
        response = connection.getresponse()
        if not 200 <= response.status < 300:
            code = "model_auth_failed" if response.status in {401, 403} else "model_rate_limited" if response.status == 429 else "model_request_rejected"
            raise AgentError(code, f"provider returned HTTP {response.status}", status=502)
        raw = response.read(4_000_001)
        if len(raw) > 4_000_000:
            raise AgentError("model_output_invalid", "provider response is too large", status=502)
        return json.loads(raw)
    finally:
        connection.close()


def paper_model(application, payload):
    config = payload.pop("managed_model", None) if isinstance(payload, dict) else None
    if config is None:
        return application.model
    try:
        if not isinstance(config, dict):
            raise ValueError("invalid managed model")
        fields = ("adapter_type", "base_url", "api_key", "model_name", "model_version")
        if any(not isinstance(config.get(key), str) or not config[key].strip() for key in fields):
            raise ValueError("missing managed model fields")
        if len(config["api_key"]) < 16 or len(config["api_key"]) > 1024:
            raise ValueError("invalid managed credential")
        parsed = _endpoint(config["base_url"])
        adapter = config["adapter_type"]
        if adapter not in {"openai_compatible", "dashscope_native"}:
            raise ValueError("unsupported managed adapter")
        if adapter == "dashscope_native" and (parsed.hostname != "dashscope.aliyuncs.com" or parsed.path.rstrip("/") != "/api/v1"):
            raise ValueError("invalid native endpoint")
        settings = replace(application.settings, model_base_url=config["base_url"].rstrip("/"),
                           model_api_key=config["api_key"], model_name=config["model_name"],
                           model_version=config["model_version"], adapter_type=adapter)
        if adapter == "dashscope_native":
            return ManagedNativePaperAdapter(settings, transport=public_json_transport)
        def compatible_transport(url, body, headers, timeout):
            body.pop("chat_template_kwargs", None)
            body.pop("seed", None)
            schema = body["response_format"]["json_schema"]["schema"]
            body["response_format"] = {"type": "json_object"}
            body["messages"] = [*body["messages"], {"role": "system", "content": "Return a JSON object matching this schema: " + json.dumps(schema, ensure_ascii=False)}]
            return public_json_transport(url, body, headers, timeout)
        return LocalLlamaCppAdapter(settings, transport=compatible_transport)
    except (ValueError, TypeError) as exc:
        raise AgentError("invalid_request", "school model configuration is invalid", status=400) from exc
