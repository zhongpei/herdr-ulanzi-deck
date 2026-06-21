#!/usr/bin/env python3
"""Test 5: Verify fusion classify logic with known scenarios"""

import time

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


# ── Scenario templates ──
KEY_APPROVAL = [
    "requestapproval",
    "approval",
    "sandbox",
    "needs approval",
    "waiting for approval",
    "permission",
]
KEY_USER_INPUT = [
    "requestuserinput",
    "waitingonuserinput",
    "needs user",
    "ask user",
    "input required",
    "question for user",
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
KEY_ERROR = ["systemerror", "has_system_error", "critical", "panic"]
T_WORKING = 30
T_MAX_ACTIVITY = 120


def classify_sim(age_sec, text):
    full = text.lower()
    has_approval = any(k in full for k in KEY_APPROVAL)
    has_user_input = any(k in full for k in KEY_USER_INPUT)
    has_done = any(k in full for k in KEY_DONE)
    has_error = any(k in full for k in KEY_ERROR)

    if age_sec is None:
        return {"status": "UNKNOWN", "confidence": 0.2}
    if has_error and not has_done:
        return {"status": "LIKELY_ERROR", "confidence": 0.65}
    if has_approval and not has_done:
        return {"status": "LIKELY_WAITING_APPROVAL", "confidence": 0.70}
    if has_user_input and not has_done:
        return {"status": "LIKELY_WAITING_USER", "confidence": 0.70}
    if age_sec <= T_WORKING and not has_done:
        return {"status": "LIKELY_WORKING", "confidence": 0.70}
    if age_sec <= T_MAX_ACTIVITY and has_done:
        return {"status": "LIKELY_COMPLETED", "confidence": 0.60}
    if age_sec <= T_MAX_ACTIVITY:
        return {"status": "LIKELY_PAUSED", "confidence": 0.55}
    return {"status": "LIKELY_IDLE", "confidence": 0.75}


now = time.time()

print("=== Scenario Tests ===\n")

# Scenario 1: Recently active, no markers
r = classify_sim(5, "some random log content here")
check(r["status"] == "LIKELY_WORKING", f"S1 active 5s: {r['status']}")

# Scenario 2: Approval + not done
r = classify_sim(5, "user requested approval for file change")
check(r["status"] == "LIKELY_WAITING_APPROVAL", f"S2 waiting approval: {r['status']}")

# Scenario 3: Approval + done → should NOT be WAITING_APPROVAL
# has_done=true → approval/done signals produce COMPLETED, not WAITING or WORKING
r = classify_sim(5, "request approval for ... completed")
check(
    r["status"] != "LIKELY_WAITING_APPROVAL",
    f"S3 approval+done: not WAITING ({r['status']})",
)
check(r["status"] == "LIKELY_COMPLETED", f"S3 approval+done: COMPLETED ({r['status']})")

# Scenario 4: User input waiting
r = classify_sim(5, "waitingonuserinput for question")
check(r["status"] == "LIKELY_WAITING_USER", f"S4 waiting user: {r['status']}")

# Scenario 5: Error
r = classify_sim(5, "critical system error detected")
check(r["status"] == "LIKELY_ERROR", f"S5 error: {r['status']}")

# Scenario 6: Idle 60s, no done
r = classify_sim(60, "some old log")
check(r["status"] == "LIKELY_PAUSED", f"S6 paused 60s: {r['status']}")

# Scenario 7: Idle 60s with done
r = classify_sim(60, "turn completed successfully")
check(r["status"] == "LIKELY_COMPLETED", f"S7 completed 60s: {r['status']}")

# Scenario 8: Very old thread
r = classify_sim(999999, "some old content")
check(r["status"] == "LIKELY_IDLE", f"S8 idle days: {r['status']}")

# Scenario 9: No timestamp signals at all
r = classify_sim(None, "")
check(r["status"] == "UNKNOWN", f"S9 no ts: {r['status']}")

# Scenario 10: Error + done → not ERROR
r = classify_sim(5, "systemerror but then completed")
check(r["status"] != "LIKELY_ERROR", f"S10 error+done: not ERROR ({r['status']})")

print("\n=== Edge Cases ===")

# Edge: Mixed signals - approval should win over working
r = classify_sim(5, "approval request for sandbox step")
check(r["status"] == "LIKELY_WAITING_APPROVAL", "E1 approval beats working")

# Edge: User input beats idle
r = classify_sim(9999, "needs user input for question")
check(r["status"] == "LIKELY_WAITING_USER", "E2 user_input beats idle")

# Edge: Case insensitivity
r = classify_sim(5, "REQUEST APPROVAL PENDING")
check(r["status"] == "LIKELY_WAITING_APPROVAL", "E3 case insensitive")

# Edge: Partial word match (sandbox is in KEY_APPROVAL)
r = classify_sim(5, "entering sandbox environment")
check(r["status"] == "LIKELY_WAITING_APPROVAL", "E4 sandbox triggers approval")

print(f"\n=== Result: {PASS} pass, {FAIL} fail ===")
raise SystemExit(0 if FAIL == 0 else 1)
