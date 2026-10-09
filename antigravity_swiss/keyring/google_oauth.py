"""
Google OAuth Token Extractor for Antigravity Swiss Knife.
=========================================================
Implements loopback desktop authorization flow:
1. Starts local HTTP server on an ephemeral loopback port (127.0.0.1:0).
2. Launches user's browser with Google OAuth 2.0 authorization URL.
3. Listens for callback at /oauth/callback.
4. Exchanges authorization code for refresh_token, access_token, and email.
5. Displays a clean success confirmation page in the browser.
"""

from __future__ import annotations

import base64
import html
import json
import logging
import threading
import urllib.parse
import urllib.request
import webbrowser
from http.server import BaseHTTPRequestHandler, HTTPServer
from typing import Any, Dict, Optional, Tuple

from antigravity_swiss.core.constants import (
    GOOGLE_DEFAULT_CLIENT_ID,
    GOOGLE_DEFAULT_CLIENT_SECRET,
    GOOGLE_OAUTH_TOKEN_URL,
)

logger = logging.getLogger("antigravity_swiss.google_oauth")

GOOGLE_AUTH_URL = "https://accounts.google.com/o/oauth2/v2/auth"
GOOGLE_SCOPES = "openid email profile https://www.googleapis.com/auth/cloud-platform"
GOOGLE_USERINFO_URL = "https://www.googleapis.com/oauth2/v3/userinfo"


SUCCESS_HTML = """<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>Antigravity Swiss Knife - Login Successful</title>
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; background:#f8fafd; color:#1f1f1f; display:flex; align-items:center; justify-content:center; height:100vh; margin:0;">
  <div style="background:#ffffff; border:1px solid #dadce0; border-radius:12px; padding:36px 48px; text-align:center; max-width:440px; box-shadow: 0 1px 3px rgba(60,64,67,0.08), 0 4px 12px rgba(60,64,67,0.05);">
    <div style="width:52px; height:52px; margin:0 auto 16px; background:#e8f0fe; border-radius:50%; display:flex; align-items:center; justify-content:center; color:#1a73e8;">
      <svg xmlns="http://www.w3.org/2000/svg" width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="#1a73e8" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="m9 12 2 2 4-4"/></svg>
    </div>
    <h2 style="margin:0 0 8px; color:#1f1f1f; font-size:20px; font-weight:700;">Authentication Successful</h2>
    <p style="color:#5f6368; font-size:14px; line-height:1.5; margin:0 0 16px;">Antigravity Swiss Knife has received and verified your credentials. You can safely close this browser window and return to the application.</p>
    <p style="color:#5f6368; font-size:12px; margin:0 0 20px;">This tab will attempt to auto-close in <span id="countdown" style="font-weight:700; color:#1a73e8;">5</span> seconds.</p>
    <button onclick="try{window.close();}catch(e){}try{window.open('','_self','');window.close();}catch(e){}" style="background:#1a73e8; color:#ffffff; border:none; border-radius:9999px; padding:10px 28px; font-size:13px; font-weight:600; cursor:pointer; white-space:nowrap; box-shadow:0 1px 2px rgba(26,115,232,0.2);">Close Window</button>
  </div>
  <script>
    let remaining = 5;
    const countEl = document.getElementById('countdown');
    const timer = setInterval(function() {
      remaining--;
      if (countEl) countEl.textContent = remaining;
      if (remaining <= 0) {
        clearInterval(timer);
        try { window.close(); } catch(e) {}
        try { window.open('', '_self', ''); window.close(); } catch(e) {}
      }
    }, 1000);
  </script>
</body>
</html>
"""

ERROR_HTML = """<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>Antigravity Swiss Knife - Login Failed</title>
  <style>
    body {{
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
      background-color: #f8fafd;
      color: #1f1f1f;
      display: flex;
      align-items: center;
      justify-content: center;
      height: 100vh;
      margin: 0;
    }}
    .card {{
      background: #ffffff;
      border: 1px solid #dadce0;
      border-radius: 12px;
      padding: 36px 48px;
      text-align: center;
      max-width: 440px;
      box-shadow: 0 1px 3px rgba(60,64,67,0.08), 0 4px 12px rgba(60,64,67,0.05);
    }}
    h2 {{ color: #d93025; margin: 0 0 8px; font-size: 20px; font-weight: 700; }}
    p {{ color: #5f6368; font-size: 14px; line-height: 1.5; }}
  </style>
</head>
<body>
  <div class="card">
    <div style="width:52px; height:52px; margin:0 auto 16px; background:#fce8e6; border-radius:50%; display:flex; align-items:center; justify-content:center; color:#d93025;">
      <svg xmlns="http://www.w3.org/2000/svg" width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="#d93025" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="12" x2="12" y1="8" y2="12"/><line x1="12" x2="12.01" y1="16" y2="16"/></svg>
    </div>
    <h2>Authentication Error</h2>
    <p>{error_msg}</p>
    <p style="margin:0 0 20px;">Please return to Antigravity Swiss Knife and try again.</p>
    <button onclick="try{{window.close();}}catch(e){{}}try{{window.open('','_self','');window.close();}}catch(e){{}}" style="background:#1a73e8; color:#ffffff; border:none; border-radius:9999px; padding:10px 28px; font-size:13px; font-weight:600; cursor:pointer; white-space:nowrap; box-shadow:0 1px 2px rgba(26,115,232,0.2);">Close Window</button>
  </div>
</body>
</html>
"""


def _decode_jwt_email(id_token: str) -> str:
    """Extract email from Google ID token JWT payload without external libraries."""
    if not id_token or id_token.count(".") < 2:
        return ""
    try:
        parts = id_token.split(".")
        payload_b64 = parts[1]
        # Pad base64
        padded = payload_b64 + "=" * (-len(payload_b64) % 4)
        payload_bytes = base64.urlsafe_b64decode(padded)
        data = json.loads(payload_bytes.decode("utf-8"))
        return str(data.get("email", "") or "")
    except Exception as exc:
        logger.debug("Could not decode email from ID token: %s", exc)
        return ""


def _fetch_userinfo_email(access_token: str) -> str:
    """Fetch user's email from Google userinfo API."""
    if not access_token:
        return ""
    try:
        req = urllib.request.Request(
            GOOGLE_USERINFO_URL,
            headers={
                "Authorization": f"Bearer {access_token}",
                "Accept": "application/json",
            },
        )
        with urllib.request.urlopen(req, timeout=10) as resp:
            data = json.loads(resp.read().decode("utf-8"))
            return str(data.get("email", "") or "")
    except Exception as exc:
        logger.warning("Failed to fetch userinfo email: %s", exc)
        return ""


class GoogleOAuthExtractor:
    """
    Manages loopback HTTP flow to extract OAuth credentials from Google.
    """

    def __init__(
        self,
        client_id: str = GOOGLE_DEFAULT_CLIENT_ID,
        client_secret: str = GOOGLE_DEFAULT_CLIENT_SECRET,
    ) -> None:
        self.client_id = client_id
        self.client_secret = client_secret
        self.result: Optional[Dict[str, str]] = None
        self.error: Optional[str] = None
        self._server: Optional[HTTPServer] = None
        self._done_event = threading.Event()

    def start_flow(self, timeout_seconds: float = 120.0, open_browser: bool = True) -> Dict[str, str]:
        """
        Runs loopback server, launches browser, and waits for token exchange.
        Returns dict containing: 'email', 'refresh_token', 'access_token'.
        Raises TimeoutError or RuntimeError on failure.
        """
        extractor = self
        self._done_event.clear()
        self.result = None
        self.error = None

        class OAuthCallbackHandler(BaseHTTPRequestHandler):
            def log_message(self, format: str, *args: Any) -> None:
                pass  # Suppress default server access logs

            def do_GET(self) -> None:
                parsed_url = urllib.parse.urlparse(self.path)
                if parsed_url.path != "/oauth/callback":
                    self.send_response(404)
                    self.end_headers()
                    self.wfile.write(b"Not Found")
                    return

                params = urllib.parse.parse_qs(parsed_url.query)
                if "error" in params:
                    err_msg = params["error"][0]
                    extractor.error = f"OAuth authorization denied: {err_msg}"
                    self.send_response(200)
                    self.send_header("Content-Type", "text/html; charset=utf-8")
                    self.end_headers()
                    self.wfile.write(ERROR_HTML.format(error_msg=html.escape(err_msg)).encode("utf-8"))
                    extractor._done_event.set()
                    return

                code = params.get("code", [None])[0]
                if not code:
                    extractor.error = "No authorization code returned from Google"
                    self.send_response(400)
                    self.send_header("Content-Type", "text/html; charset=utf-8")
                    self.end_headers()
                    self.wfile.write(ERROR_HTML.format(error_msg=html.escape(extractor.error)).encode("utf-8"))
                    extractor._done_event.set()
                    return

                # Exchange code for tokens
                try:
                    port = self.server.server_port
                    redirect_uri = f"http://127.0.0.1:{port}/oauth/callback"
                    post_data = urllib.parse.urlencode({
                        "client_id": extractor.client_id,
                        "client_secret": extractor.client_secret,
                        "code": code,
                        "grant_type": "authorization_code",
                        "redirect_uri": redirect_uri,
                    }).encode("utf-8")

                    token_req = urllib.request.Request(
                        GOOGLE_OAUTH_TOKEN_URL,
                        data=post_data,
                        headers={"Content-Type": "application/x-www-form-urlencoded"},
                    )

                    with urllib.request.urlopen(token_req, timeout=15) as resp:
                        token_resp = json.loads(resp.read().decode("utf-8"))

                    refresh_token = token_resp.get("refresh_token", "")
                    access_token = token_resp.get("access_token", "")
                    id_token = token_resp.get("id_token", "")

                    email = _decode_jwt_email(id_token)
                    if not email and access_token:
                        email = _fetch_userinfo_email(access_token)

                    extractor.result = {
                        "email": email,
                        "refresh_token": refresh_token,
                        "access_token": access_token,
                    }

                    self.send_response(200)
                    self.send_header("Content-Type", "text/html; charset=utf-8")
                    self.end_headers()
                    self.wfile.write(SUCCESS_HTML.encode("utf-8"))
                except Exception as exc:
                    logger.error("Token exchange failed: %s", exc)
                    extractor.error = f"Token exchange failed: {exc}"
                    self.send_response(500)
                    self.send_header("Content-Type", "text/html; charset=utf-8")
                    self.end_headers()
                    self.wfile.write(ERROR_HTML.format(error_msg=html.escape(str(exc))).encode("utf-8"))
                finally:
                    extractor._done_event.set()

        # Bind to 127.0.0.1:0 for ephemeral port
        server = HTTPServer(("127.0.0.1", 0), OAuthCallbackHandler)
        self._server = server
        port = server.server_port
        redirect_uri = f"http://127.0.0.1:{port}/oauth/callback"

        query_params = urllib.parse.urlencode({
            "client_id": self.client_id,
            "redirect_uri": redirect_uri,
            "response_type": "code",
            "scope": GOOGLE_SCOPES,
            "access_type": "offline",
            "prompt": "select_account consent",
        })
        auth_url = f"{GOOGLE_AUTH_URL}?{query_params}"

        # Start server in daemon thread
        server_thread = threading.Thread(target=server.serve_forever, daemon=True)
        server_thread.start()

        try:
            logger.info("Starting Google OAuth flow on %s", redirect_uri)
            if open_browser:
                webbrowser.open(auth_url)

            signaled = self._done_event.wait(timeout=timeout_seconds)
            if not signaled:
                raise TimeoutError("Google OAuth login timed out waiting for user response.")

            if self.error:
                raise RuntimeError(self.error)

            if not self.result:
                raise RuntimeError("No OAuth tokens were received.")

            return self.result
        finally:
            server.shutdown()
            server.server_close()
            self._server = None
