#!/usr/bin/env python3
"""
scripts/agent_importer.py
Cross-agent conversation migration and project importer for Antigravity Swiss Knife.
Supports native scanning and importing from:
- OpenCode Interpreter / Go (~/.local/share/opencode/opencode.db)
- DeepSeek Harness / DSH (~/.dsh/sessions/)
- Devin (~/.local/share/devin/cli/sessions.db)
- Pi Agent (~/.pi)
- Claude Code (~/.claude/transcripts)
- Custom JSON/JSONL/ZIP files
"""

import sys
import os
import json
import sqlite3
import uuid
import datetime
import urllib.parse
import subprocess
import shutil
import glob
from typing import List, Dict, Any, Optional

ANTIGRAVITY_BASE = os.path.expanduser("~/.gemini/antigravity")
SUMMARIES_DB = os.path.join(ANTIGRAVITY_BASE, "conversation_summaries.db")


def get_known_antigravity_workspaces() -> List[str]:
    """Retrieve list of known workspace filesystem paths from Antigravity."""
    workspaces = set()
    if not os.path.exists(SUMMARIES_DB):
        return list(workspaces)
    try:
        conn = sqlite3.connect(SUMMARIES_DB)
        cur = conn.cursor()
        cur.execute("SELECT workspace_uris FROM conversation_summaries WHERE workspace_uris IS NOT NULL AND workspace_uris != ''")
        for (w_json,) in cur.fetchall():
            try:
                uris = json.loads(w_json)
                for u in uris:
                    if u.startswith("file://"):
                        raw_path = urllib.parse.unquote(u[7:])
                        workspaces.add(os.path.abspath(raw_path))
            except Exception:
                continue
        conn.close()
    except Exception as e:
        sys.stderr.write(f"Warning reading summaries DB: {e}\n")
    return list(workspaces)


def match_workspace_project(detected_path: str, known_workspaces: List[str]) -> tuple:
    """Matches a filesystem path to an Antigravity project."""
    if not detected_path:
        return ("standalone", "Standalone Chats")
    
    clean_path = os.path.abspath(detected_path)
    # 1. Exact path match
    for kw in known_workspaces:
        if os.path.abspath(kw) == clean_path:
            return ("exact", os.path.basename(clean_path))
    
    # 2. Subpath match
    for kw in known_workspaces:
        if clean_path.startswith(kw) or kw.startswith(clean_path):
            return ("heuristic", os.path.basename(kw))
            
    # 3. Basename match
    base_name = os.path.basename(clean_path)
    for kw in known_workspaces:
        if os.path.basename(kw).lower() == base_name.lower():
            return ("heuristic", base_name)

    return ("new", base_name if base_name else "Imported Project")


def scan_opencode() -> List[Dict[str, Any]]:
    candidates = []
    db_path = os.path.expanduser("~/.local/share/opencode/opencode.db")
    if not os.path.exists(db_path):
        return candidates

    known_ws = get_known_antigravity_workspaces()
    conn = sqlite3.connect(db_path)
    cur = conn.cursor()
    query = """
        SELECT s.id, s.title, s.directory, s.agent, s.model, s.tokens_input, s.tokens_output,
               (SELECT count(*) FROM message m WHERE m.session_id = s.id) as msg_count,
               (SELECT count(*) FROM part p WHERE p.session_id = s.id AND p.data LIKE '%tool%') as tool_count
        FROM session s
        ORDER BY s.time_updated DESC, s.time_created DESC
        LIMIT 60
    """
    cur.execute(query)
    rows = cur.fetchall()
    for row in rows:
        sid, title, dpath, agent, model, tin, tout, mcnt, tcnt = row
        tin = tin or 0
        tout = tout or 0
        mcnt = mcnt or 0
        tcnt = tcnt or 0
        tok_est = tin + tout
        if tok_est <= 0:
            tok_est = mcnt * 180

        match_status, target_proj = match_workspace_project(dpath, known_ws)

        display_title = title if title and title.strip() else f"OpenCode Session ({sid[:12]})"
        if len(display_title) > 85:
            display_title = display_title[:82] + "..."

        candidates.append({
            "id": sid,
            "source": "opencode",
            "title": display_title,
            "message_count": mcnt,
            "tool_calls_count": tcnt,
            "token_estimate": tok_est,
            "detected_project_path": dpath or "",
            "target_antigravity_project": target_proj,
            "match_status": match_status,
            "selected": True,
        })
    conn.close()
    return candidates


def scan_dsh() -> List[Dict[str, Any]]:
    candidates = []
    dsh_dir = os.path.expanduser("~/.dsh/sessions")
    if not os.path.exists(dsh_dir):
        return candidates

    known_ws = get_known_antigravity_workspaces()
    zstd_bin = shutil.which("zstd") or "zstd"

    for root, _, files in os.walk(dsh_dir):
        for f in files:
            if not (f.endswith(".zstd") or f.endswith(".jsonl")):
                continue
            full_path = os.path.join(root, f)
            lines = []
            if f.endswith(".zstd"):
                try:
                    res = subprocess.run([zstd_bin, "-dc", full_path], capture_output=True, text=True, timeout=5)
                    lines = [l for l in res.stdout.split("\n") if l.strip()]
                except Exception:
                    continue
            else:
                try:
                    with open(full_path, "r", encoding="utf-8", errors="ignore") as fp:
                        lines = [l.strip() for l in fp if l.strip()]
                except Exception:
                    continue

            if not lines:
                continue

            sid = f.replace(".session.jsonl.zstd", "").replace(".jsonl.zstd", "").replace(".jsonl", "")
            cwd = ""
            title = f"DSH Session ({sid})"
            msg_count = 0
            tool_count = 0

            for idx, line in enumerate(lines):
                try:
                    ev = json.loads(line)
                    ev_type = ev.get("type", "")
                    if ev_type == "session":
                        cwd = ev.get("cwd", cwd)
                        sid = ev.get("id", sid)
                    elif ev_type in ("command/run", "command/done"):
                        tool_count += 1
                        msg_count += 1
                    elif "user" in ev_type or ev.get("source", {}).get("kind") == "user":
                        msg_count += 1
                        txt = ev.get("data", {}).get("text") or ev.get("data", {}).get("args") or ""
                        if txt and title.startswith("DSH Session"):
                            title = txt[:75] + ("..." if len(txt) > 75 else "")
                    elif "agent" in ev_type:
                        msg_count += 1
                except Exception:
                    continue

            if msg_count == 0:
                msg_count = len(lines)

            match_status, target_proj = match_workspace_project(cwd, known_ws)

            candidates.append({
                "id": full_path,
                "source": "dsh",
                "title": title,
                "message_count": msg_count,
                "tool_calls_count": tool_count,
                "token_estimate": msg_count * 210,
                "detected_project_path": cwd,
                "target_antigravity_project": target_proj,
                "match_status": match_status,
                "selected": True,
            })
    return candidates


def scan_devin() -> List[Dict[str, Any]]:
    candidates = []
    db_path = os.path.expanduser("~/.local/share/devin/cli/sessions.db")
    if not os.path.exists(db_path):
        return candidates

    known_ws = get_known_antigravity_workspaces()
    conn = sqlite3.connect(db_path)
    cur = conn.cursor()
    query = """
        SELECT s.id, s.title, s.working_directory, s.model, s.agent_mode,
               (SELECT count(*) FROM message_nodes mn WHERE mn.session_id = s.id) as msg_count,
               (SELECT count(*) FROM tool_call_state tcs WHERE tcs.session_id = s.id) as tool_count
        FROM sessions s
        ORDER BY s.last_activity_at DESC, s.created_at DESC
        LIMIT 60
    """
    cur.execute(query)
    rows = cur.fetchall()
    for row in rows:
        sid, title, wdir, model, mode, mcnt, tcnt = row
        mcnt = mcnt or 0
        tcnt = tcnt or 0
        display_title = title if title and title.strip() else f"Devin Session ({sid})"
        if len(display_title) > 85:
            display_title = display_title[:82] + "..."

        match_status, target_proj = match_workspace_project(wdir, known_ws)

        candidates.append({
            "id": sid,
            "source": "devin",
            "title": display_title,
            "message_count": mcnt,
            "tool_calls_count": tcnt,
            "token_estimate": mcnt * 250,
            "detected_project_path": wdir or "",
            "target_antigravity_project": target_proj,
            "match_status": match_status,
            "selected": True,
        })
    conn.close()
    return candidates


def scan_pi() -> List[Dict[str, Any]]:
    candidates = []
    pi_dir = os.path.expanduser("~/.pi")
    if not os.path.exists(pi_dir):
        return candidates

    known_ws = get_known_antigravity_workspaces()
    for root, _, files in os.walk(pi_dir):
        for f in files:
            if f.endswith(".json") or f.endswith(".jsonl") or f.endswith(".md"):
                full_path = os.path.join(root, f)
                size = os.path.getsize(full_path)
                match_status, target_proj = match_workspace_project(root, known_ws)
                candidates.append({
                    "id": full_path,
                    "source": "pi",
                    "title": f"Pi Agent Record ({f})",
                    "message_count": max(1, int(size / 300)),
                    "tool_calls_count": 0,
                    "token_estimate": max(100, int(size / 4)),
                    "detected_project_path": root,
                    "target_antigravity_project": target_proj,
                    "match_status": match_status,
                    "selected": True,
                })
    return candidates


def scan_claude_code() -> List[Dict[str, Any]]:
    candidates = []
    dir_path = os.path.expanduser("~/.claude/transcripts")
    if not os.path.exists(dir_path):
        return candidates

    known_ws = get_known_antigravity_workspaces()
    for entry in sorted(os.listdir(dir_path), reverse=True):
        if not entry.endswith(".jsonl"):
            continue
        full_path = os.path.join(dir_path, entry)
        msg_count = 0
        tool_count = 0
        title = f"Claude Session ({entry})"
        try:
            with open(full_path, "r", encoding="utf-8", errors="ignore") as f:
                for line in f:
                    msg_count += 1
                    if '"type":"tool_use"' in line:
                        tool_count += 1
                    if title.startswith("Claude Session") and '"type":"user"' in line:
                        try:
                            p = json.loads(line)
                            c = p.get("content", "")
                            if c:
                                title = c[:75] + ("..." if len(c) > 75 else "")
                        except Exception:
                            pass
        except Exception:
            continue

        size = os.path.getsize(full_path)
        tok_est = max(msg_count * 150, int(size / 4))
        curr_cwd = os.getcwd()
        proj_id, proj_name = match_workspace_project(curr_cwd, known_ws)
        candidates.append({
            "id": entry,
            "source": "claude-code",
            "title": title,
            "message_count": msg_count,
            "tool_calls_count": tool_count,
            "token_estimate": tok_est,
            "detected_project_path": curr_cwd,
            "target_antigravity_project": proj_name,
            "match_status": "exact" if proj_id != "standalone" else "fallback",
            "selected": True,
        })
        if len(candidates) >= 30:
            break
    return candidates


def import_opencode_session(sid: str, match_mode: str = "auto") -> Optional[str]:
    db_path = os.path.expanduser("~/.local/share/opencode/opencode.db")
    if not os.path.exists(db_path):
        return None

    conn = sqlite3.connect(db_path)
    cur = conn.cursor()
    cur.execute("SELECT id, title, directory, agent, model, tokens_input, tokens_output, time_created FROM session WHERE id = ?", (sid,))
    s_row = cur.fetchone()
    if not s_row:
        conn.close()
        return None

    _, title, directory, agent, model_raw, tin, tout, created_ts = s_row
    directory = directory or os.getcwd()

    # Fetch messages
    cur.execute("SELECT id, time_created, data FROM message WHERE session_id = ? ORDER BY time_created ASC", (sid,))
    msg_rows = cur.fetchall()

    # Fetch parts
    cur.execute("SELECT message_id, data FROM part WHERE session_id = ? ORDER BY time_created ASC", (sid,))
    parts_by_msg: Dict[str, List[Dict[str, Any]]] = {}
    for mid, p_data in cur.fetchall():
        try:
            p_obj = json.loads(p_data)
            parts_by_msg.setdefault(mid, []).append(p_obj)
        except Exception:
            continue

    conn.close()

    cid = str(uuid.uuid4())
    steps = []
    preview = "Imported conversation session"

    for idx, (mid, m_ts, m_data_str) in enumerate(msg_rows):
        role = "user"
        try:
            m_data = json.loads(m_data_str)
            role = m_data.get("role", "user")
        except Exception:
            pass

        # Text parts
        m_parts = parts_by_msg.get(mid, [])
        text_content = ""
        for p in m_parts:
            if p.get("type") == "text":
                text_content += p.get("text", "") + "\n"
            elif p.get("type") == "tool":
                text_content += f"\n[Tool Use: {p.get('tool', 'tool')}]\n{json.dumps(p.get('state', {}))}\n"

        if not text_content.strip():
            text_content = f"[{role.capitalize()} Step]"

        if role == "user" and preview == "Imported conversation session":
            preview = text_content[:150].strip()

        step_source = "USER_EXPLICIT" if role == "user" else "MODEL"
        step_type = "USER_INPUT" if role == "user" else "PLANNER_RESPONSE"
        ts_iso = datetime.datetime.fromtimestamp(m_ts / 1000.0 if m_ts > 1e11 else m_ts).isoformat() + "Z"

        steps.append({
            "step_index": idx,
            "source": step_source,
            "type": step_type,
            "status": "DONE",
            "created_at": ts_iso,
            "content": text_content.strip()
        })

    if not steps:
        steps.append({
            "step_index": 0,
            "source": "USER_EXPLICIT",
            "type": "USER_INPUT",
            "status": "DONE",
            "created_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
            "content": title or "OpenCode Session"
        })

    # Write transcript to brain
    brain_dir = os.path.join(ANTIGRAVITY_BASE, "brain", cid, ".system_generated", "logs")
    os.makedirs(brain_dir, exist_ok=True)
    transcript_path = os.path.join(brain_dir, "transcript.jsonl")
    with open(transcript_path, "w", encoding="utf-8") as tf:
        for st in steps:
            tf.write(json.dumps(st) + "\n")

    # Insert into conversation_summaries.db
    ws_uri = "file://" + urllib.parse.quote(directory, safe="/")
    ws_json = json.dumps([ws_uri])
    proj_name = os.path.basename(directory)
    now_iso = datetime.datetime.now(datetime.timezone.utc).isoformat()
    full_title = f"[OpenCode] {title}" if title else f"OpenCode Session ({sid[:8]})"

    sconn = sqlite3.connect(SUMMARIES_DB)
    scur = sconn.cursor()
    scur.execute("""
        INSERT OR REPLACE INTO conversation_summaries
        (conversation_id, title, preview, step_count, last_modified_time, workspace_uris, status, source, project_id, agent_name, last_user_input_time, last_user_input_step_index)
        VALUES (?, ?, ?, ?, ?, ?, 'DONE', 'OPENCODE_IMPORT', ?, ?, ?, 0)
    """, (cid, full_title, preview, len(steps), now_iso, ws_json, proj_name, agent or "OpenCode", now_iso))
    sconn.commit()
    sconn.close()

    return cid


def import_devin_session(sid: str, match_mode: str = "auto") -> Optional[str]:
    db_path = os.path.expanduser("~/.local/share/devin/cli/sessions.db")
    if not os.path.exists(db_path):
        return None

    conn = sqlite3.connect(db_path)
    cur = conn.cursor()
    cur.execute("SELECT id, title, working_directory, model, agent_mode, created_at FROM sessions WHERE id = ?", (sid,))
    s_row = cur.fetchone()
    if not s_row:
        conn.close()
        return None

    _, title, wdir, model, mode, created_ts = s_row
    wdir = wdir or os.getcwd()

    cur.execute("SELECT node_id, chat_message, created_at FROM message_nodes WHERE session_id = ? ORDER BY node_id ASC", (sid,))
    nodes = cur.fetchall()
    conn.close()

    cid = str(uuid.uuid4())
    steps = []
    preview = "Imported Devin conversation session"

    for idx, (node_id, chat_msg_str, c_ts) in enumerate(nodes):
        if not chat_msg_str:
            continue
        try:
            m_obj = json.loads(chat_msg_str)
            role = m_obj.get("role", "assistant")
            content = m_obj.get("content", "")
            if isinstance(content, list):
                # multi-part content
                parts_text = []
                for p in content:
                    if isinstance(p, dict) and "text" in p:
                        parts_text.append(p["text"])
                    elif isinstance(p, str):
                        parts_text.append(p)
                content = "\n".join(parts_text)
            elif not isinstance(content, str):
                content = json.dumps(content)

            if role == "system":
                continue # Skip raw internal system prompts to keep conversation clean

            if role == "user" and preview == "Imported Devin conversation session":
                preview = content[:150].strip()

            step_source = "USER_EXPLICIT" if role == "user" else "MODEL"
            step_type = "USER_INPUT" if role == "user" else "PLANNER_RESPONSE"
            ts_iso = datetime.datetime.fromtimestamp(c_ts if c_ts < 1e11 else c_ts / 1000.0).isoformat() + "Z"

            steps.append({
                "step_index": len(steps),
                "source": step_source,
                "type": step_type,
                "status": "DONE",
                "created_at": ts_iso,
                "content": content.strip()
            })
        except Exception:
            continue

    if not steps:
        steps.append({
            "step_index": 0,
            "source": "USER_EXPLICIT",
            "type": "USER_INPUT",
            "status": "DONE",
            "created_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
            "content": title or "Devin Session"
        })

    brain_dir = os.path.join(ANTIGRAVITY_BASE, "brain", cid, ".system_generated", "logs")
    os.makedirs(brain_dir, exist_ok=True)
    transcript_path = os.path.join(brain_dir, "transcript.jsonl")
    with open(transcript_path, "w", encoding="utf-8") as tf:
        for st in steps:
            tf.write(json.dumps(st) + "\n")

    ws_uri = "file://" + urllib.parse.quote(wdir, safe="/")
    ws_json = json.dumps([ws_uri])
    proj_name = os.path.basename(wdir)
    now_iso = datetime.datetime.now(datetime.timezone.utc).isoformat()
    full_title = f"[Devin] {title}" if title else f"Devin Session ({sid})"

    sconn = sqlite3.connect(SUMMARIES_DB)
    scur = sconn.cursor()
    scur.execute("""
        INSERT OR REPLACE INTO conversation_summaries
        (conversation_id, title, preview, step_count, last_modified_time, workspace_uris, status, source, project_id, agent_name, last_user_input_time, last_user_input_step_index)
        VALUES (?, ?, ?, ?, ?, ?, 'DONE', 'DEVIN_IMPORT', ?, 'Devin Agent', ?, 0)
    """, (cid, full_title, preview, len(steps), now_iso, ws_json, proj_name, now_iso))
    sconn.commit()
    sconn.close()

    return cid


def import_dsh_session(session_path: str, match_mode: str = "auto") -> Optional[str]:
    if not os.path.exists(session_path):
        return None

    zstd_bin = shutil.which("zstd") or "zstd"

    lines = []
    if session_path.endswith(".zstd"):
        try:
            res = subprocess.run([zstd_bin, "-dc", session_path], capture_output=True, text=True, timeout=10)
            lines = [l for l in res.stdout.split("\n") if l.strip()]
        except Exception:
            return None
    else:
        try:
            with open(session_path, "r", encoding="utf-8", errors="ignore") as fp:
                lines = [l.strip() for l in fp if l.strip()]
        except Exception:
            return None

    if not lines:
        return None

    cid = str(uuid.uuid4())
    steps = []
    cwd = os.getcwd()
    title = "DSH Conversation"
    preview = "Imported DSH session"

    for idx, line in enumerate(lines):
        try:
            ev = json.loads(line)
            ev_type = ev.get("type", "")
            if ev_type == "session":
                cwd = ev.get("cwd", cwd)
            elif "user" in ev_type or ev.get("source", {}).get("kind") == "user":
                txt = ev.get("data", {}).get("text") or ev.get("data", {}).get("args") or str(ev.get("data", ""))
                if title == "DSH Conversation" and txt:
                    title = txt[:75] + ("..." if len(txt) > 75 else "")
                    preview = txt[:150]
                steps.append({
                    "step_index": len(steps),
                    "source": "USER_EXPLICIT",
                    "type": "USER_INPUT",
                    "status": "DONE",
                    "created_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
                    "content": txt
                })
            elif "command/run" in ev_type or "agent" in ev_type:
                txt = json.dumps(ev.get("data", {}))
                steps.append({
                    "step_index": len(steps),
                    "source": "MODEL",
                    "type": "PLANNER_RESPONSE",
                    "status": "DONE",
                    "created_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
                    "content": txt
                })
        except Exception:
            continue

    if not steps:
        steps.append({
            "step_index": 0,
            "source": "USER_EXPLICIT",
            "type": "USER_INPUT",
            "status": "DONE",
            "created_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
            "content": title
        })

    brain_dir = os.path.join(ANTIGRAVITY_BASE, "brain", cid, ".system_generated", "logs")
    os.makedirs(brain_dir, exist_ok=True)
    transcript_path = os.path.join(brain_dir, "transcript.jsonl")
    with open(transcript_path, "w", encoding="utf-8") as tf:
        for st in steps:
            tf.write(json.dumps(st) + "\n")

    ws_uri = "file://" + urllib.parse.quote(cwd, safe="/")
    ws_json = json.dumps([ws_uri])
    proj_name = os.path.basename(cwd)
    now_iso = datetime.datetime.now(datetime.timezone.utc).isoformat()
    full_title = f"[DSH] {title}"

    sconn = sqlite3.connect(SUMMARIES_DB)
    scur = sconn.cursor()
    scur.execute("""
        INSERT OR REPLACE INTO conversation_summaries
        (conversation_id, title, preview, step_count, last_modified_time, workspace_uris, status, source, project_id, agent_name, last_user_input_time, last_user_input_step_index)
        VALUES (?, ?, ?, ?, ?, ?, 'DONE', 'DSH_IMPORT', ?, 'DeepSeek Harness', ?, 0)
    """, (cid, full_title, preview, len(steps), now_iso, ws_json, proj_name, now_iso))
    sconn.commit()
    sconn.close()

    return cid


def main():
    if len(sys.argv) < 2:
        print(json.dumps({"error": "missing command (scan or import)"}))
        sys.exit(1)

    cmd = sys.argv[1]

    if cmd == "scan":
        source = "opencode"
        for i, arg in enumerate(sys.argv):
            if arg == "--source" and i + 1 < len(sys.argv):
                source = sys.argv[i + 1]

        candidates = []
        if source == "opencode":
            candidates = scan_opencode()
        elif source == "dsh":
            candidates = scan_dsh()
        elif source == "devin":
            candidates = scan_devin()
        elif source == "pi":
            candidates = scan_pi()
        elif source == "claude-code":
            candidates = scan_claude_code()
        else:
            candidates = scan_opencode()

        print(json.dumps({
            "success": True,
            "source": source,
            "count": len(candidates),
            "candidates": candidates
        }))

    elif cmd == "import":
        source = "opencode"
        ids = []
        match_mode = "auto"

        for i, arg in enumerate(sys.argv):
            if arg == "--source" and i + 1 < len(sys.argv):
                source = sys.argv[i + 1]
            elif arg == "--ids" and i + 1 < len(sys.argv):
                ids = sys.argv[i + 1].split(",")
            elif arg == "--mode" and i + 1 < len(sys.argv):
                match_mode = sys.argv[i + 1]

        imported = []
        for sid in ids:
            sid = sid.strip()
            if not sid:
                continue
            new_cid = None
            if source == "opencode":
                new_cid = import_opencode_session(sid, match_mode)
            elif source == "devin":
                new_cid = import_devin_session(sid, match_mode)
            elif source == "dsh":
                new_cid = import_dsh_session(sid, match_mode)
            if new_cid:
                imported.append(new_cid)

        print(json.dumps({
            "success": True,
            "source": source,
            "imported_count": len(imported),
            "imported_ids": imported,
            "message": f"Successfully imported {len(imported)} conversations into Antigravity!"
        }))


if __name__ == "__main__":
    main()
