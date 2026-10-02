"""
Test helpers, runners, IPC socket client, and reference TOTP engine
used across Tiers 1-4 opaque-box test suites.
"""

import base64
from datetime import datetime, timezone
import hashlib
import hmac
import json
import os
import socket
import struct
import subprocess
import sys
import time
from typing import Any, Dict, List, Optional, Tuple
import uuid


def run_cli(*args: str, env: Optional[Dict[str, str]] = None, timeout: float = 10.0) -> subprocess.CompletedProcess:
    """Executes 'python -m antigravity_swiss' with given arguments and custom environment."""
    merged_env = os.environ.copy()
    if env:
        merged_env.update(env)
    cmd = [sys.executable, "-m", "antigravity_swiss"] + list(args)
    return subprocess.run(
        cmd,
        capture_output=True,
        text=True,
        env=merged_env,
        timeout=timeout
    )


class SocketIpcClient:
    """Client for Unix Domain Socket JSON-RPC 2.0 / NDJSON daemon communication."""

    def __init__(self, socket_path: str):
        self.socket_path = socket_path
        self.sock: Optional[socket.socket] = None

    def connect(self, timeout: float = 5.0) -> None:
        self.sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        self.sock.settimeout(timeout)
        self.sock.connect(self.socket_path)

    def close(self) -> None:
        if self.sock:
            try:
                self.sock.close()
            except Exception:
                pass
            self.sock = None

    def call(self, method: str, params: Optional[Dict[str, Any]] = None, req_id: int = 1) -> Dict[str, Any]:
        """Sends a JSON-RPC 2.0 request and returns the decoded JSON-RPC response."""
        if not self.sock:
            self.connect()
        request = {
            "jsonrpc": "2.0",
            "id": req_id,
            "method": method,
            "params": params or {}
        }
        msg = (json.dumps(request) + "\n").encode("utf-8")
        self.sock.sendall(msg)  # type: ignore

        # Read line response
        buffer = b""
        while b"\n" not in buffer:
            chunk = self.sock.recv(4096)  # type: ignore
            if not chunk:
                break
            buffer += chunk

        line = buffer.split(b"\n")[0].decode("utf-8")
        return json.loads(line)


class ReferenceTotp:
    """Authoritative reference implementation of RFC 6238 for expected output validation."""

    TEST_SECRET_RFC6238 = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"  # 12345678901234567890 in Base32

    # Official RFC 6238 Appendix B test vectors (T -> expected 8-digit and 6-digit codes)
    OFFICIAL_VECTORS = [
        (59, "94287082", "287082"),
        (1111111109, "07081804", "081804"),
        (1111111111, "14050471", "050471"),
        (1234567890, "89005924", "005924"),
        (2000000000, "69279037", "279037"),
        (20000000000, "65353130", "353130")
    ]

    @staticmethod
    def generate(secret: str, for_time: int, digits: int = 6, interval: int = 30) -> str:
        clean = secret.strip().replace(" ", "").replace("-", "").upper()
        pad = (8 - (len(clean) % 8)) % 8
        clean += "=" * pad
        key = base64.b32decode(clean)
        counter = for_time // interval
        msg = struct.pack(">Q", counter)
        h = hmac.new(key, msg, hashlib.sha1).digest()
        offset = h[-1] & 0x0F
        code = struct.unpack(">I", h[offset:offset+4])[0] & 0x7FFFFFFF
        otp = code % (10 ** digits)
        return f"{otp:0{digits}d}"

    @staticmethod
    def countdown(for_time: float, interval: int = 30) -> Tuple[int, float]:
        remaining_secs = interval - (int(for_time) % interval)
        fraction = (interval - (for_time % interval)) / float(interval)
        return remaining_secs, fraction


class CredentialBuilder:
    """Creates valid and invalid OAuth credentials matching Linux Secret Service schema."""

    @staticmethod
    def build_valid_payload(
        email: str = "user@gmail.com",
        access_token: str = "ya29.valid_test_token_abc123",
        refresh_token: str = "1//0test_refresh_token_xyz789",
        expiry_seconds: int = 3600
    ) -> str:
        expiry_dt = datetime.now(timezone.utc).timestamp() + expiry_seconds
        # Return exact JSON structure mined from secret-tool
        payload = {
            "token": {
                "access_token": access_token,
                "token_type": "Bearer",
                "refresh_token": refresh_token,
                "expiry": f"{int(expiry_dt)}"
            },
            "auth_method": "consumer",
            "id_token": f"eyJhbGciOiJSUzI1NiIsImtpZCI6IjEyMyJ9.{email}.mock"
        }
        return json.dumps(payload)

    @staticmethod
    def build_expired_payload(email: str = "expired@gmail.com") -> str:
        return CredentialBuilder.build_valid_payload(
            email=email,
            access_token="ya29.expired_token",
            refresh_token="1//0valid_refresh_token",
            expiry_seconds=-3600
        )


class FingerprintBuilder:
    """Builds valid 36-byte UUID profiles and test sets."""

    @staticmethod
    def generate_profile() -> Dict[str, str]:
        return {
            "machineid": str(uuid.uuid4()),
            "updaterId": str(uuid.uuid4()),
            "installation_id": str(uuid.uuid4()),
            "installation_uuid": str(uuid.uuid4())
        }
