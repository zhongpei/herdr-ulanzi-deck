#!/usr/bin/env python3
"""Test 3: Verify logs query - data access, keyword detection, timestamp range"""

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


# Get most recent thread from legacy DB
state_p = Path("~/.codex/state_5.sqlite").expanduser()
uri = f"file:{state_p}?mode=ro&immutable=1"
con = sqlite3.connect(uri, uri=True)
con.row_factory = sqlite3.Row
row = con.execute("""
    SELECT id FROM threads WHERE COALESCE(archived,0)=0
    ORDER BY recency_at_ms DESC LIMIT 1
""").fetchone()
con.close()

if not row:
    raise SystemExit("no threads found")

thread_id = row["id"]
print(f"Using thread: {thread_id}")

# Test logs in both DBs
log_dbs = [
    Path("~/.codex/sqlite/logs_2.sqlite").expanduser(),
    Path("~/.codex/logs_2.sqlite").expanduser(),
]

for lp in log_dbs:
    if not lp.exists():
        continue

    uri = f"file:{lp}?mode=ro&immutable=1"
    con = sqlite3.connect(uri, uri=True)
    con.row_factory = sqlite3.Row

    count = con.execute(
        "SELECT COUNT(*) AS cnt FROM logs WHERE thread_id=?", (thread_id,)
    ).fetchone()["cnt"]

    if count == 0:
        total = con.execute("SELECT COUNT(*) AS cnt FROM logs").fetchone()["cnt"]
        check(False, f"{lp.name}: 0 logs for this thread (has {total} total logs)")
        con.close()
        continue

    check(count > 0, f"{lp.name}: {count} log entries for thread")

    # Check last log timestamp
    r = con.execute(
        """
        SELECT MIN(ts) AS first_ts, MAX(ts) AS last_ts,
               datetime(MIN(ts),'unixepoch','localtime') AS first_at,
               datetime(MAX(ts),'unixepoch','localtime') AS last_at
        FROM logs WHERE thread_id=?
    """,
        (thread_id,),
    ).fetchone()
    check(
        r["last_ts"] > r["first_ts"], f"  ts range: {r['first_at']} -> {r['last_at']}"
    )
    check(r["last_ts"] > 0, f"  last_ts={r['last_ts']} (valid)")

    # Read last 100 logs and scan keywords
    rows = con.execute(
        """
        SELECT feedback_log_body FROM logs WHERE thread_id=?
        ORDER BY ts DESC, ts_nanos DESC, id DESC LIMIT 100
    """,
        (thread_id,),
    ).fetchall()
    full_text = "\n".join(str(r["feedback_log_body"] or "") for r in rows).lower()

    KEY_APPROVAL = ["requestapproval", "approval", "sandbox", "permission"]
    KEY_USER_INPUT = [
        "requestuserinput",
        "waitingonuserinput",
        "needs user",
        "ask user",
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

    found_approval = [kw for kw in KEY_APPROVAL if kw in full_text]
    found_user = [kw for kw in KEY_USER_INPUT if kw in full_text]
    found_done = [kw for kw in KEY_DONE if kw in full_text]

    check(len(found_approval) > 0, f"  approval keywords found: {found_approval}")
    check(len(found_done) > 0, f"  done keywords found: {found_done}")
    print(f"    user_input keywords: {found_user if found_user else '(none)'}")

    con.close()
    print()

print(f"\n=== Result: {PASS} pass, {FAIL} fail ===")
raise SystemExit(0 if FAIL == 0 else 1)
