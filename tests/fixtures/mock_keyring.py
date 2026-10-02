"""
Hermetic mock for Linux Secret Service API and secret-tool CLI.
Simulates secret-tool lookup, store, clear, and search without modifying the host keyring.
Complies with org.freedesktop.Secret.Generic schema and zalando/go-keyring attributes.
"""

import json
import os
import sys
from typing import Any, Dict, List, Optional, Tuple


class MockKeyringBackend:
    """In-memory Secret Service collection storing secrets keyed by attributes."""

    def __init__(self):
        # Key: (service, username) -> Value: dict of {secret, label, created, modified}
        self._store: Dict[Tuple[str, str], Dict[str, Any]] = {}
        self.is_locked: bool = False

    def store(self, service: str, username: str, secret: str, label: Optional[str] = None) -> None:
        """Stores a secret with service and username attributes."""
        if self.is_locked:
            raise PermissionError("Keyring is locked")
        self._store[(service, username)] = {
            "secret": secret,
            "label": label or f"Password for '{username}' on '{service}'",
            "service": service,
            "username": username,
        }

    def lookup(self, service: str, username: str) -> Optional[str]:
        """Looks up a secret by service and username."""
        if self.is_locked:
            raise PermissionError("Keyring is locked")
        item = self._store.get((service, username))
        if item is None:
            return None
        return item["secret"]

    def clear(self, service: str, username: str) -> bool:
        """Clears a secret by service and username."""
        if self.is_locked:
            raise PermissionError("Keyring is locked")
        key = (service, username)
        if key in self._store:
            del self._store[key]
            return True
        return False

    def search(self, service: Optional[str] = None, username: Optional[str] = None) -> List[Dict[str, Any]]:
        """Searches secrets matching given attributes."""
        if self.is_locked:
            raise PermissionError("Keyring is locked")
        results = []
        for (svc, usr), item in self._store.items():
            if service and svc != service:
                continue
            if username and usr != username:
                continue
            results.append(dict(item))
        return results

    def dump_state(self) -> Dict[str, Any]:
        """Returns internal state for test verification."""
        return {
            f"{k[0]}:{k[1]}": v["secret"]
            for k, v in self._store.items()
        }

    def reset(self) -> None:
        """Wipes all secrets."""
        self._store.clear()
        self.is_locked = False


def create_mock_secret_tool_script(bin_dir: str, state_file_path: str) -> str:
    """
    Creates an executable 'secret-tool' mock script in bin_dir.
    The mock script persists secrets to state_file_path (JSON),
    replicating secret-tool's exact CLI arguments, stdout/stderr, and exit codes:
      - secret-tool lookup service <svc> username <usr> -> stdout (no newline), exit 0 on hit, exit 1 on miss
      - secret-tool store --label=... service <svc> username <usr> -> reads stdin, exit 0
      - secret-tool clear service <svc> username <usr> -> exit 0
      - secret-tool search service <svc> username <usr> -> formatted output
    """
    os.makedirs(bin_dir, exist_ok=True)
    script_path = os.path.join(bin_dir, "secret-tool")

    script_content = f"""#!{sys.executable}
import sys
import json
import os

state_file = {json.dumps(state_file_path)}

def load_state():
    if os.path.exists(state_file):
        try:
            with open(state_file, 'r', encoding='utf-8') as f:
                return json.load(f)
        except Exception:
            return {{}}
    return {{}}

def save_state(state):
    os.makedirs(os.path.dirname(state_file), exist_ok=True)
    with open(state_file, 'w', encoding='utf-8') as f:
        json.dump(state, f, indent=2)

def parse_attrs(args):
    attrs = {{}}
    i = 0
    while i < len(args):
        if i + 1 < len(args) and not args[i].startswith('--'):
            attrs[args[i]] = args[i+1]
            i += 2
        else:
            i += 1
    return attrs

def main():
    args = sys.argv[1:]
    if not args:
        sys.exit(1)
    
    cmd = args[0]
    sub_args = args[1:]
    state = load_state()

    if cmd == "lookup":
        attrs = parse_attrs(sub_args)
        key = f"{{attrs.get('service', '')}}:{{attrs.get('username', '')}}"
        if key in state:
            sys.stdout.write(state[key])
            sys.stdout.flush()
            sys.exit(0)
        else:
            sys.exit(1)

    elif cmd == "store":
        attrs = parse_attrs(sub_args)
        secret = sys.stdin.read()
        key = f"{{attrs.get('service', '')}}:{{attrs.get('username', '')}}"
        state[key] = secret
        save_state(state)
        sys.exit(0)

    elif cmd == "clear":
        attrs = parse_attrs(sub_args)
        key = f"{{attrs.get('service', '')}}:{{attrs.get('username', '')}}"
        if key in state:
            del state[key]
            save_state(state)
        sys.exit(0)

    elif cmd == "search":
        attrs = parse_attrs(sub_args)
        key = f"{{attrs.get('service', '')}}:{{attrs.get('username', '')}}"
        if key in state:
            sys.stdout.write(f"[/mock]\\nlabel = Mock Secret\\nsecret = {{state[key]}}\\n")
            sys.stdout.flush()
        sys.exit(0)

    else:
        sys.stderr.write(f"Unknown command: {{cmd}}\\n")
        sys.exit(1)

if __name__ == "__main__":
    main()
"""
    with open(script_path, "w", encoding="utf-8") as f:
        f.write(script_content)
    os.chmod(script_path, 0o755)
    return script_path
