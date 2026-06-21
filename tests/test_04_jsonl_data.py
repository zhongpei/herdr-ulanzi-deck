#!/usr/bin/env python3
"""Test 4: Verify rollout JSONL access - tail, parse, keyword detection, mtime"""

import json
import os
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


def tail_lines(path, n=120):
    with path.open("rb") as f:
        f.seek(0, os.SEEK_END)
        size = f.tell()
        block = 8192
        data = b""
        while size > 0 and data.count(b"\n") <= n:
            step = min(block, size)
            size -= step
            f.seek(size)
            data = f.read(step) + data
        return data.decode("utf-8", errors="replace").splitlines()[-n:]


# Get threads + rollout paths from both DBs
db_paths = [
    Path("~/.codex/state_5.sqlite").expanduser(),
    Path("~/.codex/sqlite/state_5.sqlite").expanduser(),
]

all_rows = []
for p in db_paths:
    if not p.exists():
        continue
    uri = f"file:{p}?mode=ro&immutable=1"
    con = sqlite3.connect(uri, uri=True)
    con.row_factory = sqlite3.Row
    cols = {r[1] for r in con.execute("PRAGMA table_info(threads)")}
    sort_col = "recency_at_ms" if "recency_at_ms" in cols else "updated_at_ms"
    rows = con.execute(f"""
        SELECT id, rollout_path, {sort_col} AS sort_val
        FROM threads WHERE COALESCE(archived,0)=0
        ORDER BY {sort_col} DESC LIMIT 10
    """).fetchall()
    all_rows.extend(rows)
    con.close()

print(f"=== Testing {len(all_rows)} recent threads ===\n")

parsed_ok = 0
mtime_valid = 0
keyword_results = {}

for r in all_rows:
    rp = Path(r["rollout_path"]).expanduser()
    if not rp.exists():
        check(False, f"rollout missing: {rp}")
        continue

    mtime = rp.stat().st_mtime
    check(mtime > 0, f"{rp.name}: mtime={mtime} (valid)")

    lines = tail_lines(rp, 100)
    check(len(lines) > 0, f"{rp.name}: {len(lines)} lines read")

    # Parse last 10 lines
    for l in lines[-10:]:
        try:
            obj = json.loads(l)
            check(
                "type" in obj or "event" in obj,
                f"  parsed JSON: type={obj.get('type', '?')}",
            )
            parsed_ok += 1
        except json.JSONDecodeError:
            check(False, f"  invalid JSON line: {l[:60]}...")

    # Keyword scan across full tail
    full_text = "\n".join(lines).lower()

    KEY_APPROVAL = [
        "requestapproval",
        "approval",
        "permissions/requestapproval",
        "sandbox",
        "needs approval",
    ]
    KEY_USER_INPUT = [
        "requestuserinput",
        "waitingonuserinput",
        "needs user",
        "ask user",
        "input required",
    ]
    KEY_DONE = [
        "turn/completed",
        "completed",
        "interrupted",
        "failed",
        "resolved",
        "denied",
        "approved",
        "rejected",
    ]

    fa = [kw for kw in KEY_APPROVAL if kw in full_text]
    fu = [kw for kw in KEY_USER_INPUT if kw in full_text]
    fd = [kw for kw in KEY_DONE if kw in full_text]
    keyword_results[r["id"][:12]] = {"approval": fa, "user_input": fu, "done": fd}

print("\n=== Keyword Summary (first 10 chars of thread_id) ===")
check_found = {"approval": 0, "user_input": 0, "done": 0}
for tid, hits in keyword_results.items():
    for cat in ["approval", "user_input", "done"]:
        if hits[cat]:
            check_found[cat] += 1
            check(True, f"  {tid}: {cat} → {hits[cat][:3]}")

print(
    f"\nThreads with approval keywords: {check_found['approval']}/{len(keyword_results)}"
)
print(
    f"Threads with user_input keywords: {check_found['user_input']}/{len(keyword_results)}"
)
print(f"Threads with done keywords: {check_found['done']}/{len(keyword_results)}")

print(f"\n=== Result: {PASS} pass, {FAIL} fail ===")
raise SystemExit(0 if FAIL == 0 else 1)
