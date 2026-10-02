"""
In-process loopback mock server emulating Google CloudCode API endpoints:
- POST /v1internal:retrieveUserQuotaSummary
- POST /v1internal:fetchAvailableModels
- POST /v1internal:generateContent
- POST /token (OAuth2 token refresh)

Provides /test_control/ endpoints to deterministically manipulate quota,
clock drift, time progression, and transient network errors for hermetic testing.
Supports per-token multi-account profiles and realistic keep-alive simulation.
"""

from datetime import datetime, timedelta, timezone
from http.server import BaseHTTPRequestHandler, HTTPServer
import json
import threading
from typing import Any, Dict, List, Optional
from urllib.parse import urlparse


class MockCloudCodeHandler(BaseHTTPRequestHandler):
    """Handles incoming Google CloudCode and OAuth API requests."""

    def log_message(self, format, *args):
        # Suppress standard logging to keep test output clean
        pass

    @property
    def backend(self) -> "MockCloudCodeServer":
        return getattr(self.server, "mock_backend")

    def _send_json(self, status_code: int, data: Dict[str, Any], extra_headers: Optional[Dict[str, str]] = None) -> None:
        payload = json.dumps(data).encode("utf-8")
        self.send_response_only(status_code)
        self.send_header("Server", "MockCloudCode/1.0")
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(payload)))
        
        # Inject simulated Date header for clock drift testing
        simulated_now = self.backend.get_simulated_time() + timedelta(seconds=self.backend.clock_drift_seconds)
        # Format as RFC 7231 / HTTP Date: e.g. Thu, 01 Oct 2026 05:04:53 GMT
        date_str = simulated_now.strftime("%a, %d %b %Y %H:%M:%S GMT")
        self.send_header("Date", date_str)

        if extra_headers:
            for k, v in extra_headers.items():
                self.send_header(k, v)
        self.end_headers()
        self.wfile.write(payload)

    def _read_body_json(self) -> Dict[str, Any]:
        length = int(self.headers.get("Content-Length", 0))
        if length > 0:
            raw = self.rfile.read(length).decode("utf-8")
            try:
                return json.loads(raw)
            except Exception:
                return {"_raw": raw}
        return {}

    def _extract_token(self, auth_header: str) -> str:
        return auth_header.replace("Bearer ", "").strip()

    def _get_profile(self, token: str) -> Dict[str, Any]:
        return self.backend.get_profile(token)

    def do_POST(self):
        parsed = urlparse(self.path)
        path = parsed.path
        body = self._read_body_json()
        auth_header = self.headers.get("Authorization", "")

        # Record incoming request for assertions
        self.backend.recorded_requests.append({
            "path": path,
            "headers": dict(self.headers),
            "body": body
        })

        # --- Test Control Endpoints ---
        if path == "/test_control/set_quota":
            remaining = float(body.get("remaining", 1.0))
            reset_in = int(body.get("reset_in_seconds", 18000))
            weekly_remaining = float(body.get("weekly_remaining", 1.0))
            token = body.get("token")
            self.backend.set_quota(remaining, reset_in, weekly_remaining)
            if token:
                self.backend.set_account_profile(token, remaining, reset_in, weekly_remaining)
            self._send_json(200, {"status": "ok", "remaining": remaining})
            return

        if path == "/test_control/set_account_quota":
            token = str(body.get("token", ""))
            remaining = float(body.get("remaining", 1.0))
            reset_in = int(body.get("reset_in_seconds", 18000))
            weekly_remaining = float(body.get("weekly_remaining", 1.0))
            self.backend.set_account_profile(token, remaining, reset_in, weekly_remaining)
            self._send_json(200, {"status": "ok", "token": token, "remaining": remaining})
            return

        if path == "/test_control/set_clock_drift":
            drift = float(body.get("drift_seconds", 0.0))
            self.backend.set_clock_drift(drift)
            self._send_json(200, {"status": "ok", "drift_seconds": drift})
            return

        if path == "/test_control/apply_preset":
            token = str(body.get("token", ""))
            preset = str(body.get("preset", "healthy"))
            self.backend.apply_preset(token, preset)
            self._send_json(200, {"status": "ok", "token": token, "preset": preset})
            return

        if path == "/test_control/advance_time":
            seconds = int(body.get("seconds", 0))
            self.backend.advance_time(seconds)
            self._send_json(200, {"status": "ok", "simulated_now": self.backend.get_simulated_time().isoformat()})
            return

        if path == "/test_control/simulate_transient_error":
            status = int(body.get("status", 503))
            count = int(body.get("count", 1))
            self.backend.transient_errors_remaining = count
            self.backend.transient_error_status = status
            self._send_json(200, {"status": "ok", "error_status": status, "count": count})
            return

        # Check for simulated transient errors (e.g. 503 Gateway Down)
        if self.backend.transient_errors_remaining > 0:
            self.backend.transient_errors_remaining -= 1
            code = self.backend.transient_error_status
            self._send_json(code, {
                "error": {
                    "code": code,
                    "message": "Transient backend error simulated by test harness.",
                    "status": "UNAVAILABLE" if code == 503 else "RESOURCE_EXHAUSTED"
                }
            })
            return

        # --- OAuth Token Refresh (/token) ---
        if path in ("/token", "/oauth2/v4/token", "/o/oauth2/token"):
            refresh_tok = body.get("refresh_token") or ""
            if "invalid" in refresh_tok:
                self._send_json(400, {"error": "invalid_grant", "error_description": "Bad refresh token"})
                return
            new_access_token = f"ya29.mock_refreshed_{len(self.backend.recorded_requests)}"
            self._send_json(200, {
                "access_token": new_access_token,
                "expires_in": 3600,
                "token_type": "Bearer",
                "scope": "https://www.googleapis.com/auth/cloud-platform"
            })
            return

        # Auth validation for CloudCode APIs
        token = self._extract_token(auth_header)
        if not token or "invalid" in token or "expired" in token:
            self._send_json(401, {
                "error": {
                    "code": 401,
                    "message": "Request had invalid authentication credentials.",
                    "status": "UNAUTHENTICATED"
                }
            })
            return

        profile = self._get_profile(token)
        now = self.backend.get_simulated_time()

        # --- retrieveUserQuotaSummary ---
        if path.endswith(":retrieveUserQuotaSummary"):
            reset_time = profile["reset_time"].strftime("%Y-%m-%dT%H:%M:%SZ")
            weekly_reset = (now + timedelta(days=7)).strftime("%Y-%m-%dT%H:%M:%SZ")

            cur_remaining = profile["remaining_fraction"]
            if now >= profile["reset_time"] and profile.get("warmup_fired", False):
                cur_remaining = 1.0

            resp_data = {
                "groups": [
                    {
                        "displayName": "Gemini Models",
                        "description": "Models within this group: Gemini Flash, Gemini Pro",
                        "buckets": [
                            {
                                "bucketId": "gemini-weekly",
                                "displayName": "Weekly Limit Remaining",
                                "window": "weekly",
                                "resetTime": weekly_reset,
                                "remainingFraction": profile["weekly_remaining_fraction"],
                                "description": "Weekly contractual quota allocation."
                            },
                            {
                                "bucketId": "gemini-5h",
                                "displayName": "Five Hour Limit Remaining",
                                "window": "5h",
                                "resetTime": reset_time,
                                "remainingFraction": cur_remaining,
                                "description": "5-hour rolling burst quota."
                            }
                        ]
                    },
                    {
                        "displayName": "Claude and GPT models",
                        "description": "Models within this group: Claude Opus, Claude Sonnet, GPT-OSS",
                        "buckets": [
                            {
                                "bucketId": "3p-weekly",
                                "displayName": "Weekly Limit Remaining",
                                "window": "weekly",
                                "resetTime": weekly_reset,
                                "remainingFraction": 1.0
                            },
                            {
                                "bucketId": "3p-5h",
                                "displayName": "Five Hour Limit Remaining",
                                "window": "5h",
                                "resetTime": reset_time,
                                "remainingFraction": 1.0
                            }
                        ]
                    }
                ],
                "description": "Within each group, models share a weekly limit and a 5-hour limit."
            }
            self._send_json(200, resp_data)
            return

        # --- fetchAvailableModels ---
        if path.endswith(":fetchAvailableModels"):
            reset_str = profile["reset_time"].strftime("%Y-%m-%dT%H:%M:%SZ")
            resp_data = {
                "defaultAgentModelId": "gemini-3.8-flash-high",
                "tieredModelIds": {
                    "flashLite": ["gemini-3.5-flash-lite"],
                    "flash": ["gemini-3.8-flash-tiered"],
                    "pro": ["gemini-3.1-pro-low"]
                },
                "models": {
                    "gemini-3.8-flash-high": {
                        "displayName": "Gemini 3.8 Flash (High)",
                        "supportsImages": True,
                        "supportsThinking": True,
                        "recommended": True,
                        "maxTokens": 1048576,
                        "maxOutputTokens": 65536,
                        "quotaInfo": {
                            "remainingFraction": profile["remaining_fraction"],
                            "resetTime": reset_str
                        }
                    },
                    "gemini-3.5-flash-lite": {
                        "displayName": "Gemini 3.5 Flash Lite",
                        "supportsImages": True,
                        "supportsThinking": False,
                        "recommended": False,
                        "maxTokens": 524288,
                        "maxOutputTokens": 8192,
                        "quotaInfo": {
                            "remainingFraction": profile["remaining_fraction"],
                            "resetTime": reset_str
                        }
                    },
                    "gemini-3.1-pro-low": {
                        "displayName": "Gemini 3.1 Pro (Low)",
                        "supportsImages": True,
                        "supportsThinking": True,
                        "recommended": False,
                        "maxTokens": 2097152,
                        "maxOutputTokens": 65536,
                        "quotaInfo": {
                            "remainingFraction": profile["remaining_fraction"],
                            "resetTime": reset_str
                        }
                    },
                    "claude-sonnet-4-6": {
                        "displayName": "Claude Sonnet 4.6 (Thinking)",
                        "supportsImages": True,
                        "supportsThinking": True,
                        "recommended": False,
                        "maxTokens": 200000,
                        "maxOutputTokens": 64000,
                        "quotaInfo": {
                            "remainingFraction": 1.0,
                            "resetTime": reset_str
                        }
                    }
                }
            }
            self._send_json(200, resp_data)
            return

        # --- generateContent (Keep-Alive Warmup Ping) ---
        if path.endswith(":generateContent"):
            gen_config = body.get("request", {}).get("generationConfig", {})
            max_out = gen_config.get("maxOutputTokens", 1)

            # If quota is exhausted and resetTime has NOT arrived:
            if profile["remaining_fraction"] <= 0.0 and now < profile["reset_time"]:
                self._send_json(429, {
                    "error": {
                        "code": 429,
                        "message": "Resource has been exhausted (e.g. check quota).",
                        "status": "RESOURCE_EXHAUSTED"
                    }
                })
                return

            # Warmup ping succeeded! Update both profile and global state
            new_reset = now + timedelta(hours=5)
            profile["warmup_fired"] = True
            profile["remaining_fraction"] = 1.0
            profile["reset_time"] = new_reset

            self.backend.warmup_fired = True
            self.backend.current_quota_fraction = 1.0
            self.backend.quota_reset_time = new_reset

            self._send_json(200, {
                "candidates": [
                    {
                        "content": {
                            "parts": [{"text": " "}],
                            "role": "model"
                        },
                        "finishReason": "STOP"
                    }
                ],
                "usageMetadata": {
                    "promptTokenCount": 1,
                    "candidatesTokenCount": max_out,
                    "totalTokenCount": 1 + max_out
                }
            })
            return

        # Default fallback
        self._send_json(404, {"error": "Not Found", "path": path})

    def do_GET(self):
        parsed = urlparse(self.path)
        if parsed.path == "/test_control/requests":
            self._send_json(200, {"requests": self.backend.recorded_requests})
            return
        self._send_json(404, {"error": "Not Found"})


class MockCloudCodeServer:
    """Threaded HTTP server running on 127.0.0.1 with test control hooks and per-account profile support."""

    def __init__(self, host: str = "127.0.0.1", port: int = 0):
        self.host = host
        self.port = port
        self.httpd: Optional[HTTPServer] = None
        self.thread: Optional[threading.Thread] = None

        # Simulated state
        self.time_offset_seconds: float = 0.0
        self.clock_drift_seconds: float = 0.0
        self.current_quota_fraction: float = 0.85
        self.weekly_quota_fraction: float = 0.90
        self.base_time = datetime.now(timezone.utc)
        self.quota_reset_time = self.base_time + timedelta(hours=3, minutes=45)
        self.warmup_fired: bool = False
        self.transient_errors_remaining: int = 0
        self.transient_error_status: int = 503
        self.recorded_requests: List[Dict[str, Any]] = []
        self.account_profiles: Dict[str, Dict[str, Any]] = {}

    def start(self) -> str:
        self.httpd = HTTPServer((self.host, self.port), MockCloudCodeHandler)
        self.httpd.mock_backend = self  # type: ignore
        self.port = self.httpd.server_port
        self.thread = threading.Thread(target=self.httpd.serve_forever, daemon=True)
        self.thread.start()
        return f"http://{self.host}:{self.port}"

    def stop(self) -> None:
        if self.httpd:
            self.httpd.shutdown()
            self.httpd.server_close()
        if self.thread and self.thread.is_alive():
            self.thread.join(timeout=2.0)

    def get_simulated_time(self) -> datetime:
        return self.base_time + timedelta(seconds=self.time_offset_seconds)

    def advance_time(self, seconds: float) -> None:
        self.time_offset_seconds += seconds

    def set_clock_drift(self, drift_seconds: float) -> None:
        self.clock_drift_seconds = drift_seconds

    def set_quota(self, remaining: float, reset_in_seconds: int = 18000, weekly_remaining: float = 1.0) -> None:
        self.current_quota_fraction = remaining
        self.weekly_quota_fraction = weekly_remaining
        self.quota_reset_time = self.get_simulated_time() + timedelta(seconds=reset_in_seconds)
        self.warmup_fired = False

    def set_account_profile(
        self,
        token: str,
        remaining: float,
        reset_in_seconds: int = 18000,
        weekly_remaining: float = 1.0,
    ) -> None:
        """Create or update quota profile for a specific access token."""
        self.account_profiles[token] = {
            "remaining_fraction": remaining,
            "weekly_remaining_fraction": weekly_remaining,
            "reset_time": self.get_simulated_time() + timedelta(seconds=reset_in_seconds),
            "warmup_fired": False,
        }

    def apply_preset(self, token: str, preset: str) -> None:
        """Apply named preset profile to a specific access token."""
        if preset == "healthy":
            self.set_account_profile(token, remaining=1.0, reset_in_seconds=18000, weekly_remaining=1.0)
        elif preset == "low":
            self.set_account_profile(token, remaining=0.04, reset_in_seconds=18000, weekly_remaining=1.0)
        elif preset == "depleted":
            self.set_account_profile(token, remaining=0.0, reset_in_seconds=18000, weekly_remaining=1.0)
        elif preset == "weekly_exhausted":
            self.set_account_profile(token, remaining=1.0, reset_in_seconds=18000, weekly_remaining=0.0)
        else:
            self.set_account_profile(token, remaining=0.85, reset_in_seconds=18000, weekly_remaining=0.90)

    def get_profile(self, token: str) -> Dict[str, Any]:
        """Get profile for token or return global fallback profile."""
        if token in self.account_profiles:
            return self.account_profiles[token]
        return {
            "remaining_fraction": self.current_quota_fraction,
            "weekly_remaining_fraction": self.weekly_quota_fraction,
            "reset_time": self.quota_reset_time,
            "warmup_fired": self.warmup_fired,
        }

    def reset_history(self, clear_profiles: bool = False) -> None:
        self.recorded_requests.clear()
        self.transient_errors_remaining = 0
        self.warmup_fired = False
        self.clock_drift_seconds = 0.0
        if clear_profiles:
            self.account_profiles.clear()
