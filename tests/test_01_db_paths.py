#!/usr/bin/env python3
"""Test 1: Verify Codex DB paths, schemas, and file existence"""

import sqlite3
from pathlib import Path

PASS = 0
FAIL = 0


def check(cond, msg):
    global PASS, FAIL
    if cond:
        print(f"  ✓ {msg}")
        PASS += 1
    else:
        print(f"  ✗ {msg}")
        FAIL += 1


CODEX_HOME = Path("~/.codex").expanduser()

candidates = [
    ("CODEX_HOME/sqlite", CODEX_HOME / "sqlite"),
    ("CODEX_HOME", CODEX_HOME),
]

print("=== DB File Existence ===")
for _, db_name in [("state", "state_5.sqlite"), ("logs", "logs_2.sqlite")]:
    for _, d in candidates:
        p = d / db_name
        exists = p.exists()
        size = p.stat().st_size if exists else 0
        check(exists, f"{p} ({size:,} bytes)")
        if exists:
            uri = f"file:{p}?mode=ro&immutable=1"
            con = sqlite3.connect(uri, uri=True)
            tables = [
                r[0]
                for r in con.execute(
                    "SELECT name FROM sqlite_master WHERE type='table'"
                ).fetchall()
            ]
            check(
                "threads" in tables or "logs" in tables,
                f"  table found: {[t for t in ['threads', 'logs'] if t in tables]}",
            )
            con.close()

print()

print("=== Schema Compatibility ===")
for _, d in candidates:
    p = d / "state_5.sqlite"
    if not p.exists():
        continue
    uri = f"file:{p}?mode=ro&immutable=1"
    con = sqlite3.connect(uri, uri=True)
    cols = {r[1] for r in con.execute("PRAGMA table_info(threads)")}
    check("recency_at_ms" in cols, f"{p}: has recency_at_ms")
    check("updated_at_ms" in cols, f"{p}: has updated_at_ms")
    check("rollout_path" in cols, f"{p}: has rollout_path")
    check("archived" in cols, f"{p}: has archived")
    check("title" in cols, f"{p}: has title")
    check("cwd" in cols, f"{p}: has cwd")
    con.close()

print()

print("=== Logs Schema ===")
for _, d in candidates:
    p = d / "logs_2.sqlite"
    if not p.exists():
        continue
    uri = f"file:{p}?mode=ro&immutable=1"
    con = sqlite3.connect(uri, uri=True)
    cols = {r[1] for r in con.execute("PRAGMA table_info(logs)")}
    check("thread_id" in cols, f"{p}: has thread_id")
    check("ts" in cols, f"{p}: has ts")
    check("level" in cols, f"{p}: has level")
    check("feedback_log_body" in cols, f"{p}: has feedback_log_body")
    con.close()

print(f"\n=== Result: {PASS} pass, {FAIL} fail ===")
raise SystemExit(0 if FAIL == 0 else 1)
