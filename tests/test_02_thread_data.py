#!/usr/bin/env python3
"""Test 2: Verify thread data query - columns, values, rollout file existence"""

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


# Probe both DBs, pick the one with most recent data
db_paths = [
    Path("~/.codex/state_5.sqlite").expanduser(),
    Path("~/.codex/sqlite/state_5.sqlite").expanduser(),
]

for p in db_paths:
    if not p.exists():
        continue

    uri = f"file:{p}?mode=ro&immutable=1"
    con = sqlite3.connect(uri, uri=True)
    con.row_factory = sqlite3.Row
    cols = {r[1] for r in con.execute("PRAGMA table_info(threads)")}

    sort_col = "recency_at_ms" if "recency_at_ms" in cols else "updated_at_ms"
    check(sort_col in cols, f"{p.name}: sort column '{sort_col}' available")

    # Read top 5 threads
    rows = con.execute(f"""
        SELECT id, title, cwd, rollout_path, 
               updated_at_ms, {sort_col} AS sort_val,
               datetime({sort_col}/1000, 'unixepoch', 'localtime') AS sort_at,
               archived, approval_mode, source
        FROM threads
        WHERE COALESCE(archived,0) = 0
        ORDER BY {sort_col} DESC
        LIMIT 5
    """).fetchall()

    check(len(rows) > 0, f"{p.name}: returned {len(rows)} non-archived threads")

    for r in rows:
        tid = r["id"]
        check(
            len(tid) > 10, f"  thread_id='{tid[:20]}...' looks valid ({len(tid)} chars)"
        )
        check(r["sort_val"] > 0, f"  sort_col={r['sort_val']} (>0)")

        rp = Path(r["rollout_path"]).expanduser() if r["rollout_path"] else None
        if rp:
            check(rp.exists(), f"  rollout={rp.name} exists")

        # Verify critical fields are populated
        has_title = bool(r["title"] or r["preview"] if "preview" in r else r["title"])
        check(has_title, "  has title/preview")

    con.close()
    print()

# Overlap check
all_ids = {}
for p in db_paths:
    if not p.exists():
        continue
    uri = f"file:{p}?mode=ro&immutable=1"
    con = sqlite3.connect(uri, uri=True)
    rows = con.execute(
        "SELECT id, rollout_path FROM threads WHERE COALESCE(archived,0)=0"
    ).fetchall()
    for r in rows:
        all_ids.setdefault(r[0], []).append(p.name)
    con.close()

both = [k for k, v in all_ids.items() if len(v) == 2]
legacy_only = [k for k, v in all_ids.items() if v == ["state_5.sqlite"]]
new_only = [k for k, v in all_ids.items() if v == ["state_5.sqlite"]]
check(len(legacy_only) <= 10, f"legacy-only threads: {len(legacy_only)} (expect <=10)")
check(len(new_only) <= 10, f"new-path-only threads: {len(new_only)} (expect <=10)")
check(len(both) > 0, f"threads in both DBs: {len(both)}")

print(f"\n=== Result: {PASS} pass, {FAIL} fail ===")
raise SystemExit(0 if FAIL == 0 else 1)
