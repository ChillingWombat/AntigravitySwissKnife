"""
Safe Protobuf Text Format Reader & In-Place Mutator (Feature F12).
===================================================================
Inspects and updates `installation_uuid: "<UUID>"` inside `antigravity_state.pbtxt`
without corrupting other protobuf message blocks, onboarding flags, or migrations.
"""

from __future__ import annotations

import logging
import os
from pathlib import Path
import re
from typing import Optional

logger = logging.getLogger("antigravity_swiss.fingerprint.pbtxt_parser")

INSTALLATION_UUID_REGEX = re.compile(
    r'^(?P<indent>\s*)installation_uuid:\s*"(?P<uuid>[0-9a-fA-F-]+)"(?P<comment>.*)$',
    re.MULTILINE,
)


class PbtxtParser:
    """Safe reader and updater for Antigravity's text-format protobuf state."""

    @staticmethod
    def extract_installation_uuid(pbtxt_path: Path | str) -> Optional[str]:
        """Reads installation_uuid from the specified pbtxt file."""
        path = Path(pbtxt_path).expanduser().resolve()
        if not path.exists():
            return None

        try:
            content = path.read_text(encoding="utf-8")
            match = INSTALLATION_UUID_REGEX.search(content)
            if match:
                return match.group("uuid")
        except Exception as exc:
            logger.error("Error reading installation_uuid from %s: %s", path, exc)
        return None

    @staticmethod
    def update_installation_uuid(pbtxt_path: Path | str, new_uuid: str) -> bool:
        """
        Updates installation_uuid in-place within the pbtxt file.
        Preserves all comments, indentation, onboarding flags, and message blocks.
        Writes atomically via temporary file and os.replace.
        """
        path = Path(pbtxt_path).expanduser().resolve()
        if not path.exists():
            # If file does not exist, create minimal pbtxt with installation_uuid
            path.parent.mkdir(parents=True, exist_ok=True)
            content = f'installation_uuid: "{new_uuid}"\n'
        else:
            content = path.read_text(encoding="utf-8")
            match = INSTALLATION_UUID_REGEX.search(content)
            if match:
                # Replace existing line preserving indentation and comments
                def _repl(m: re.Match) -> str:
                    indent = m.group("indent")
                    comment = m.group("comment")
                    return f'{indent}installation_uuid: "{new_uuid}"{comment}'

                content = INSTALLATION_UUID_REGEX.sub(_repl, content, count=1)
            else:
                # Key not present, append cleanly to EOF
                if content and not content.endswith("\n"):
                    content += "\n"
                content += f'installation_uuid: "{new_uuid}"\n'

        tmp_path = path.with_suffix(".tmp")
        try:
            tmp_path.write_text(content, encoding="utf-8")
            os.replace(tmp_path, path)
            logger.info("Successfully updated installation_uuid in %s to %s", path, new_uuid)
            return True
        except Exception as exc:
            logger.error("Failed to write updated pbtxt to %s: %s", path, exc)
            if tmp_path.exists():
                tmp_path.unlink(missing_ok=True)
            raise
