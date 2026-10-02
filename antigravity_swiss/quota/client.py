"""
Pure Python Standard Library Client for Google CloudCode & OAuth Endpoints.
==========================================================================
Directly interacts with:
- POST /v1internal:retrieveUserQuotaSummary
- POST /v1internal:fetchAvailableModels
- POST /v1internal:generateContent
- POST /token (OAuth2 refresh)

Zero third-party dependencies: strictly uses urllib.request / urllib.error / http.client.
Provides synchronous methods and non-blocking asyncio wrappers via asyncio.to_thread.
"""

from __future__ import annotations

import asyncio
import datetime
import email.utils
import json
import logging
import socket
import urllib.error
import urllib.parse
import urllib.request
from typing import Any, Dict, Optional, Tuple

from antigravity_swiss.core.constants import (
    GOOGLE_AVAILABLE_MODELS_URL,
    GOOGLE_DEFAULT_CLIENT_ID,
    GOOGLE_DEFAULT_CLIENT_SECRET,
    GOOGLE_GENERATE_CONTENT_URL,
    GOOGLE_OAUTH_TOKEN_URL,
    GOOGLE_QUOTA_SUMMARY_URL,
    GOOGLE_USER_AGENT,
)
from antigravity_swiss.core.errors import (
    QuotaAuthExpiredError,
    QuotaError,
    QuotaNetworkError,
    QuotaRateLimitError,
    QuotaUnavailableError,
)

logger = logging.getLogger("antigravity_swiss.quota.client")

DEFAULT_CLOUDCODE_URL = "https://cloudcode-pa.googleapis.com"
DEFAULT_TIMEOUT_SEC = 15.0


class CloudCodeClient:
    """
    Standard library HTTP client communicating with Google CloudCode endpoints.
    """

    def __init__(
        self,
        base_url: str = DEFAULT_CLOUDCODE_URL,
        token_url: str = GOOGLE_OAUTH_TOKEN_URL,
        user_agent: str = f"{GOOGLE_USER_AGENT} linux/amd64",
        timeout: float = DEFAULT_TIMEOUT_SEC,
    ) -> None:
        self.base_url = base_url.rstrip("/")
        self.token_url = token_url
        self.user_agent = user_agent
        self.timeout = timeout
        self.last_clock_drift_seconds: float = 0.0

    def _parse_http_date_drift(self, date_header: Optional[str]) -> float:
        """Parse HTTP Date response header and calculate clock drift (server - local)."""
        if not date_header:
            return self.last_clock_drift_seconds
        try:
            parsed_dt = email.utils.parsedate_to_datetime(date_header)
            if parsed_dt.tzinfo is None:
                parsed_dt = parsed_dt.replace(tzinfo=datetime.timezone.utc)
            else:
                parsed_dt = parsed_dt.astimezone(datetime.timezone.utc)
            now = datetime.datetime.now(datetime.timezone.utc)
            drift = (parsed_dt - now).total_seconds()
            self.last_clock_drift_seconds = drift
            return drift
        except Exception as exc:
            logger.debug("Could not parse HTTP Date header '%s': %s", date_header, exc)
            return self.last_clock_drift_seconds

    def _execute_request_sync(
        self,
        url: str,
        payload_dict: Dict[str, Any],
        access_token: Optional[str] = None,
        extra_headers: Optional[Dict[str, str]] = None,
    ) -> Tuple[Dict[str, Any], float]:
        """
        Synchronous HTTP POST request execution with strict error mapping.
        Returns: (response_json_dict, clock_drift_seconds)
        """
        body_bytes = json.dumps(payload_dict).encode("utf-8")
        headers = {
            "Content-Type": "application/json",
            "User-Agent": self.user_agent,
            "Content-Length": str(len(body_bytes)),
        }
        if access_token:
            headers["Authorization"] = f"Bearer {access_token}"
        if extra_headers:
            headers.update(extra_headers)

        req = urllib.request.Request(url, data=body_bytes, headers=headers, method="POST")

        try:
            with urllib.request.urlopen(req, timeout=self.timeout) as resp:
                raw_body = resp.read()
                date_header = resp.headers.get("Date") if hasattr(resp, "headers") else None
                drift = self._parse_http_date_drift(date_header)

                try:
                    data = json.loads(raw_body.decode("utf-8")) if raw_body else {}
                except json.JSONDecodeError as jde:
                    raise QuotaError(f"Malformed JSON response from {url}: {jde}") from jde

                return data, drift

        except urllib.error.HTTPError as he:
            date_header = he.headers.get("Date") if hasattr(he, "headers") else None
            drift = self._parse_http_date_drift(date_header)
            raw_err = he.read().decode("utf-8", errors="replace") if hasattr(he, "read") else ""
            err_json: Dict[str, Any] = {}
            try:
                err_json = json.loads(raw_err)
            except Exception:
                err_json = {"raw": raw_err}

            msg = err_json.get("error", {}).get("message", raw_err) if isinstance(err_json.get("error"), dict) else raw_err

            if he.code == 401:
                raise QuotaAuthExpiredError(f"HTTP 401 Unauthenticated: {msg}", data=err_json) from he
            elif he.code == 429:
                raise QuotaRateLimitError(f"HTTP 429 Resource Exhausted: {msg}", data=err_json) from he
            elif he.code in (502, 503, 504):
                raise QuotaUnavailableError(f"HTTP {he.code} Unavailable: {msg}", data=err_json) from he
            else:
                raise QuotaError(f"HTTP {he.code} Error from {url}: {msg}", code=-32030, data=err_json) from he

        except (urllib.error.URLError, socket.timeout, TimeoutError, OSError) as ne:
            raise QuotaNetworkError(f"Network error connecting to {url}: {ne}") from ne

    # --- Synchronous API Methods ---

    def retrieve_user_quota_summary_sync(
        self,
        access_token: str,
        project: str = "",
    ) -> Tuple[Dict[str, Any], float]:
        """Call POST /v1internal:retrieveUserQuotaSummary synchronously."""
        url = f"{self.base_url}/v1internal:retrieveUserQuotaSummary"
        return self._execute_request_sync(url, {"project": project}, access_token=access_token)

    def fetch_available_models_sync(
        self,
        access_token: str,
        project: str = "",
    ) -> Tuple[Dict[str, Any], float]:
        """Call POST /v1internal:fetchAvailableModels synchronously."""
        url = f"{self.base_url}/v1internal:fetchAvailableModels"
        return self._execute_request_sync(url, {"project": project}, access_token=access_token)

    def generate_content_warmup_sync(
        self,
        access_token: str,
        model: str = "gemini-3.5-flash-lite",
        project: str = "",
    ) -> Tuple[Dict[str, Any], float]:
        """Call POST /v1internal:generateContent with 1-token prompt synchronously."""
        url = f"{self.base_url}/v1internal:generateContent"
        payload = {
            "project": project,
            "model": model,
            "request": {
                "contents": [{"role": "user", "parts": [{"text": " "}]}],
                "generationConfig": {"maxOutputTokens": 1, "temperature": 0.0},
            },
        }
        return self._execute_request_sync(url, payload, access_token=access_token)

    def refresh_access_token_sync(
        self,
        refresh_token: str,
        client_id: Optional[str] = None,
        client_secret: Optional[str] = None,
    ) -> Dict[str, Any]:
        """Execute OAuth2 token refresh synchronously against token endpoint."""
        cid = client_id or GOOGLE_DEFAULT_CLIENT_ID
        csec = client_secret or GOOGLE_DEFAULT_CLIENT_SECRET

        target_url = self.token_url
        if "127.0.0.1" in self.base_url or "localhost" in self.base_url:
            target_url = f"{self.base_url}/token"

        payload = {
            "client_id": cid,
            "client_secret": csec,
            "refresh_token": refresh_token,
            "grant_type": "refresh_token",
        }
        data, _ = self._execute_request_sync(target_url, payload)
        if "access_token" not in data:
            raise QuotaAuthExpiredError("OAuth token refresh response did not contain access_token", data=data)
        return data

    # --- Asynchronous Wrappers (Non-blocking via asyncio.to_thread) ---

    async def retrieve_user_quota_summary(
        self,
        access_token: str,
        project: str = "",
    ) -> Tuple[Dict[str, Any], float]:
        return await asyncio.to_thread(self.retrieve_user_quota_summary_sync, access_token, project)

    async def fetch_available_models(
        self,
        access_token: str,
        project: str = "",
    ) -> Tuple[Dict[str, Any], float]:
        return await asyncio.to_thread(self.fetch_available_models_sync, access_token, project)

    async def generate_content_warmup(
        self,
        access_token: str,
        model: str = "gemini-3.5-flash-lite",
        project: str = "",
    ) -> Tuple[Dict[str, Any], float]:
        return await asyncio.to_thread(self.generate_content_warmup_sync, access_token, model, project)

    async def refresh_access_token(
        self,
        refresh_token: str,
        client_id: Optional[str] = None,
        client_secret: Optional[str] = None,
    ) -> Dict[str, Any]:
        return await asyncio.to_thread(self.refresh_access_token_sync, refresh_token, client_id, client_secret)
