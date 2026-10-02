"""
Safe Protobuf Text Format Reader & In-Place Mutator (Feature F10).
==================================================================
Inspects and updates `installation_uuid: "<UUID>"` and `installation_id: "<UUID>"`
inside `antigravity_state.pbtxt` without corrupting other protobuf message blocks,
onboarding flags, or migrations.
"""

from __future__ import annotations

import logging
import os
from pathlib import Path
import re
import tempfile
from typing import Optional

logger = logging.getLogger("antigravity_swiss.fingerprint.pbtxt_parser")


def _build_field_regex(field_name: str) -> re.Pattern:
    return re.compile(
        rf'^(?P<indent>[ \t]*){field_name}[ \t]*:[ \t]*["\']?(?P<val>[0-9a-fA-F-]+)["\']?(?P<comment>[ \t]*#.*|[ \t]*)$',
        re.MULTILINE,
    )


class PbtxtParser:
    """Safe reader and updater for Antigravity's text-format protobuf state."""

    @staticmethod
    def extract_field(pbtxt_path: Path | str, field_name: str) -> Optional[str]:
        """Reads specified field value from pbtxt file."""
        path = Path(pbtxt_path).expanduser().resolve()
        if not path.exists():
            return None

        try:
            content = path.read_text(encoding="utf-8")
            pattern = _build_field_regex(field_name)
            match = pattern.search(content)
            if match:
                return match.group("val")
        except Exception as exc:
            logger.error("Error reading %s from %s: %s", field_name, path, exc)
        return None

    @classmethod
    def extract_installation_uuid(cls, pbtxt_path: Path | str) -> Optional[str]:
        """Reads installation_uuid from the specified pbtxt file."""
        return cls.extract_field(pbtxt_path, "installation_uuid")

    @classmethod
    def extract_installation_id(cls, pbtxt_path: Path | str) -> Optional[str]:
        """Reads installation_id from the specified pbtxt file if present."""
        return cls.extract_field(pbtxt_path, "installation_id")

    @staticmethod
    def update_field(pbtxt_path: Path | str, field_name: str, new_value: str) -> bool:
        """
        Updates field_name in-place within the pbtxt file.
        Preserves indentation, comments, onboarding flags, and message blocks.
        Writes atomically via temporary file with mkstemp and os.replace.
        """
        path = Path(pbtxt_path).expanduser().resolve()
        clean_value = new_value.strip().strip('"').strip("'")
        pattern = _build_field_regex(field_name)

        if not path.exists():
            path.parent.mkdir(parents=True, exist_ok=True)
            content = f'{field_name}: "{clean_value}"\n'
        else:
            content = path.read_text(encoding="utf-8")
            match = pattern.search(content)
            if match:
                def _repl(m: re.Match) -> str:
                    indent = m.group("indent")
                    comment = m.group("comment")
                    return f'{indent}{field_name}: "{clean_value}"{comment}'

                content = pattern.sub(_repl, content, count=1)
            else:
                if content and not content.endswith("\n"):
                    content += "\n"
                content += f'{field_name}: "{clean_value}"\n'

        fd, tmp_path_str = tempfile.mkstemp(
            dir=path.parent,
            prefix=f".{path.name}.tmp.",
            text=True,
        )
        tmp_path = Path(tmp_path_str)
        try:
            os.fchmod(fd, 0o600)
            with os.fdopen(fd, "w", encoding="utf-8") as f:
                f.write(content)
                f.flush()
                os.fsync(f.fileno())
            os.replace(str(tmp_path), str(path))
            logger.info("Successfully updated %s in %s to %s", field_name, path, clean_value)
            return True
        except Exception as exc:
            logger.error("Failed to write updated pbtxt to %s: %s", path, exc)
            if tmp_path.exists():
                try:
                    tmp_path.unlink()
                except OSError:
                    pass
            raise

    @classmethod
    def update_installation_uuid(cls, pbtxt_path: Path | str, new_uuid: str) -> bool:
        """Updates installation_uuid in-place within pbtxt file."""
        return cls.update_field(pbtxt_path, "installation_uuid", new_uuid)

    @classmethod
    def update_installation_id(cls, pbtxt_path: Path | str, new_id: str) -> bool:
        """Updates installation_id in-place within pbtxt file."""
        return cls.update_field(pbtxt_path, "installation_id", new_id)

    @classmethod
    def update_installation_fields(
        cls,
        pbtxt_path: Path | str,
        installation_uuid: Optional[str] = None,
        installation_id: Optional[str] = None,
    ) -> bool:
        """Updates both installation_uuid and installation_id atomically in a single write."""
        path = Path(pbtxt_path).expanduser().resolve()
        if not path.exists():
            path.parent.mkdir(parents=True, exist_ok=True)
            content = ""
            if installation_uuid:
                content += f'installation_uuid: "{installation_uuid.strip()}"\n'
            if installation_id:
                content += f'installation_id: "{installation_id.strip()}"\n'
        else:
            content = path.read_text(encoding="utf-8")
            if installation_uuid:
                pat_uuid = _build_field_regex("installation_uuid")
                clean_uuid = installation_uuid.strip()
                if pat_uuid.search(content):
                    content = pat_uuid.sub(
                        lambda m: f'{m.group("indent")}installation_uuid: "{clean_uuid}"{m.group("comment")}',
                        content,
                        count=1,
                    )
                else:
                    if content and not content.endswith("\n"):
                        content += "\n"
                    content += f'installation_uuid: "{clean_uuid}"\n'

            if installation_id:
                pat_id = _build_field_regex("installation_id")
                clean_id = installation_id.strip()
                if pat_id.search(content):
                    content = pat_id.sub(
                        lambda m: f'{m.group("indent")}installation_id: "{clean_id}"{m.group("comment")}',
                        content,
                        count=1,
                    )
                else:
                    if content and not content.endswith("\n"):
                        content += "\n"
                    content += f'installation_id: "{clean_id}"\n'

        fd, tmp_path_str = tempfile.mkstemp(
            dir=path.parent,
            prefix=f".{path.name}.tmp.",
            text=True,
        )
        tmp_path = Path(tmp_path_str)
        try:
            os.fchmod(fd, 0o600)
            with os.fdopen(fd, "w", encoding="utf-8") as f:
                f.write(content)
                f.flush()
                os.fsync(f.fileno())
            os.replace(str(tmp_path), str(path))
            return True
        except Exception:
            if tmp_path.exists():
                try:
                    tmp_path.unlink()
                except OSError:
                    pass
            raise
