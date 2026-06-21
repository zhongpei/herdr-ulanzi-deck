#!/usr/bin/env python3
"""
Codex Desktop 旁路监控探测器 — 方案二
======================================
SQLite + logs + JSONL 融合
输出 LIKELY_* 状态
不执行自动调度决策，只提醒人工检查

状态分级（置信度从高到低）：
  LIKELY_WORKING           最近有持续活动
  LIKELY_WAITING_APPROVAL  出现 approval 关键字且未完结
  LIKELY_WAITING_USER      出现 user input 关键字且未完结
  LIKELY_IDLE              存在 thread 但长时间无活动
  UNKNOWN                  DB 与文件不一致 / 无法判断

用法:
  python test_codex_app.py                 单次探测
  python test_codex_app.py --watch         持续监控（每 5s 刷新）
  python test_codex_app.py --watch --interval 10  自定义间隔
  python test_codex_app.py --json          JSON 输出（适合管道）
  python test_codex_app.py --alert-only    只打印需要人工检查的 thread
"""

import argparse
import contextlib
import json
import os
import sqlite3
import sys
import time
from pathlib import Path

# ── 路径探测 ──────────────────────────────────────────────
CODEX_HOME = Path(os.environ.get("CODEX_HOME", "~/.codex")).expanduser()

CANDIDATE_SQLITE_DIRS = [
    Path(os.environ["CODEX_SQLITE_HOME"]).expanduser()
    if os.environ.get("CODEX_SQLITE_HOME") else None,
    CODEX_HOME / "sqlite",
    CODEX_HOME,
]
CANDIDATE_SQLITE_DIRS = [p for p in CANDIDATE_SQLITE_DIRS if p]

# ── 关键字启发式规则 ──────────────────────────────────────
KEY_APPROVAL = [
    "requestapproval",
    "approval",
    "permissions/requestapproval",
    "commandexecution/requestapproval",
    "filechange/requestapproval",
    "sandbox",
    "needs approval",
    "waiting for approval",
]

KEY_USER_INPUT = [
    "requestuserinput",
    "waitingonuserinput",
    "needs user",
    "ask user",
    "question for user",
    "input required",
]

KEY_DONE = [
    "turn/completed",
    "completed",
    "interrupted",
    "failed",
    "serverrequest/resolved",
    "resolved",
    "denied",
    "approved",
    "rejected",
]

KEY_ERROR = [
    "systemerror",
    "has_system_error",
    "critical",
    "panic",
]

# ── 常量 ──────────────────────────────────────────────────
T_WORKING = 30          # 最近 N 秒内有活动 → LIKELY_WORKING
T_MAX_ACTIVITY = 120    # 超过 N 秒无活动 → LIKELY_IDLE
REFRESH_INTERVAL = 5    # --watch 默认间隔


# ══════════════════════════════════════════════════════════
#  DB 层
# ══════════════════════════════════════════════════════════

def ro_connect(path: Path):
    """只读不可变连接（防止写入触发 wal 或锁）"""
    uri = f"file:{path}?mode=ro&immutable=1"
    return sqlite3.connect(uri, uri=True)


def find_all_db(name: str) -> list[Path]:
    """返回所有存在的 DB 路径（可能有 legacy + 新路径并存）"""
    found = []
    seen = set()
    for d in CANDIDATE_SQLITE_DIRS:
        p = d / name
        resolved = p.resolve()
        if p.exists() and resolved not in seen:
            found.append(p)
            seen.add(resolved)
    return found


# ══════════════════════════════════════════════════════════
#  数据读取
# ══════════════════════════════════════════════════════════

def read_recent_threads(state_db: Path, limit=30):
    """从 state_5.sqlite 读取最近 thread 元数据"""
    con = ro_connect(state_db)
    con.row_factory = sqlite3.Row

    # 检查 columns（不同版本 schema 有差异）
    cols = {row[1] for row in con.execute("PRAGMA table_info(threads)")}
    has_recency = "recency_at_ms" in cols
    has_preview = "preview" in cols

    order_col = "recency_at_ms" if has_recency else "updated_at_ms"
    preview_col = "preview" if has_preview else "'' AS preview"

    rows = con.execute(f"""
        SELECT
          id, title, cwd, rollout_path,
          source, approval_mode,
          updated_at_ms,
          {order_col} AS recency_at_ms,
          COALESCE(archived, 0) AS archived,
          {preview_col}
        FROM threads
        WHERE COALESCE(archived, 0) = 0
        ORDER BY {order_col} DESC
        LIMIT ?
    """, (limit,)).fetchall()
    con.close()
    return [dict(r) for r in rows]


def read_recent_logs(logs_db: Path | None, thread_id: str, limit=80):
    """从 logs_2.sqlite 读取指定 thread 的最新日志"""
    if not logs_db or not logs_db.exists():
        return []
    try:
        con = ro_connect(logs_db)
        con.row_factory = sqlite3.Row
        rows = con.execute("""
            SELECT ts, ts_nanos, level, target, feedback_log_body AS msg
            FROM logs
            WHERE thread_id = ?
            ORDER BY ts DESC, ts_nanos DESC, id DESC
            LIMIT ?
        """, (thread_id, limit)).fetchall()
        con.close()
        return [dict(r) for r in rows]
    except Exception as e:
        return [{"msg": f"<log read error: {e}>"}]


def tail_lines(path: Path, n=120):
    """从文件尾部读取最后 N 行（高效，不加载全文）"""
    try:
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
    except Exception:
        return []


# ══════════════════════════════════════════════════════════
#  融合判断引擎
# ══════════════════════════════════════════════════════════

def classify(thread: dict, logs: list[dict]) -> dict:
    """
    多信号融合判断 thread 状态。

    信号来源：
      1. state_5.sqlite: recency_at_ms, updated_at_ms
      2. logs_2.sqlite:  日志时间戳、内容关键字
      3. rollout JSONL:  mtime、tail 内容关键字

    返回：
      {
        "status": "LIKELY_WORKING" | "LIKELY_WAITING_APPROVAL" |
                  "LIKELY_WAITING_USER" | "LIKELY_IDLE" | "UNKNOWN",
        "confidence": float (0.0 ~ 1.0),
        "reason": str,           # 判断依据
        "idle_seconds": float,   # 距离最后活动时间
        "alerts": [str],         # 需要人工关注的提示
      }
    """
    now = time.time()
    result = {
        "status": "UNKNOWN",
        "confidence": 0.0,
        "reason": "",
        "idle_seconds": 999999.0,
        "alerts": [],
    }

    # ── 信号 1：rollout JSONL ──
    rollout_path_str = thread.get("rollout_path")
    rollout_path = Path(rollout_path_str).expanduser() if rollout_path_str else None

    rollout_mtime = None
    rollout_tail_text = ""
    if rollout_path and rollout_path.exists():
        with contextlib.suppress(OSError):
            rollout_mtime = rollout_path.stat().st_mtime
        rollout_tail_lines = tail_lines(rollout_path, 120)
        rollout_tail_text = "\n".join(rollout_tail_lines).lower()

    # ── 信号 2：logs ──
    log_text = "\n".join(str(x.get("msg") or "") for x in logs).lower()

    # ── 信号 3：SQLite recency ──
    sqlite_recency = (thread.get("recency_at_ms") or 0) / 1000.0

    # ── 融合时间戳：取最新信号 ──
    timestamps = [t for t in [rollout_mtime, sqlite_recency] if t]
    if logs and logs[0].get("ts"):
        timestamps.append(logs[0]["ts"])
    last_ts = max(timestamps) if timestamps else None

    if last_ts is None:
        result["reason"] = "no activity timestamp available"
        result["confidence"] = 0.2
        return result

    idle_sec = now - last_ts
    result["idle_seconds"] = idle_sec

    # ── 全文本融合（rollout + logs）─
    full = rollout_tail_text + "\n" + log_text

    has_approval = any(k in full for k in KEY_APPROVAL)
    has_user_input = any(k in full for k in KEY_USER_INPUT)
    has_done = any(k in full for k in KEY_DONE)
    has_error = any(k in full for k in KEY_ERROR)

    # ── 优先级判断 ──

    # 1. 错误 / 崩溃
    if has_error and not has_done:
        result["status"] = "LIKELY_ERROR"
        result["confidence"] = 0.65
        result["reason"] = "error/critical keywords detected, no completion found"
        result["alerts"].append("thread may have encountered an error")
        return result

    # 2. 等待 approval（未完结）
    if has_approval and not has_done:
        result["status"] = "LIKELY_WAITING_APPROVAL"
        result["confidence"] = 0.70
        result["reason"] = f"approval keywords found, no resolution; idle={idle_sec:.0f}s"
        result["alerts"].append("可能等待 approval，请检查 Desktop")
        return result

    # 3. 等待用户输入（未完结）
    if has_user_input and not has_done:
        result["status"] = "LIKELY_WAITING_USER"
        result["confidence"] = 0.70
        result["reason"] = f"user input keywords found, no resolution; idle={idle_sec:.0f}s"
        result["alerts"].append("可能等待用户输入，请检查 Desktop")
        return result

    # 4. 工作状态（低 idle + 无 done 标记）
    if idle_sec <= T_WORKING and not has_done:
        result["status"] = "LIKELY_WORKING"
        result["confidence"] = 0.70
        result["reason"] = f"active {idle_sec:.0f}s ago, no completion marker"
        return result

    # 5. 短期无活动但有 done 标记
    if idle_sec <= T_MAX_ACTIVITY and has_done:
        result["status"] = "LIKELY_COMPLETED"
        result["confidence"] = 0.60
        result["reason"] = f"last activity {idle_sec:.0f}s ago, completion marker found"
        return result

    # 6. 短期无活动
    if idle_sec <= T_MAX_ACTIVITY:
        result["status"] = "LIKELY_PAUSED"
        result["confidence"] = 0.55
        result["reason"] = f"idle {idle_sec:.0f}s, no clear markers"
        result["alerts"].append("thread 短期无活动，可能卡住或闲置")
        return result

    # 7. 长时间无活动
    result["status"] = "LIKELY_IDLE"
    result["confidence"] = 0.75
    result["reason"] = f"no activity for {idle_sec:.0f}s (>{T_MAX_ACTIVITY}s)"
    return result


# ══════════════════════════════════════════════════════════
#  输出格式化
# ══════════════════════════════════════════════════════════

STATUS_EMOJI = {
    "LIKELY_WORKING":          "🟢",
    "LIKELY_WAITING_APPROVAL": "🟡",
    "LIKELY_WAITING_USER":     "🟡",
    "LIKELY_ERROR":            "🔴",
    "LIKELY_PAUSED":           "🔵",
    "LIKELY_COMPLETED":        "⚪",
    "LIKELY_IDLE":             "⚫",
    "UNKNOWN":                 "❓",
}

STATUS_LABEL = {
    "LIKELY_WORKING":          "WORKING",
    "LIKELY_WAITING_APPROVAL": "WAITING_APPROVAL",
    "LIKELY_WAITING_USER":     "WAITING_USER_INPUT",
    "LIKELY_ERROR":            "ERROR",
    "LIKELY_PAUSED":           "PAUSED",
    "LIKELY_COMPLETED":        "COMPLETED",
    "LIKELY_IDLE":             "IDLE",
    "UNKNOWN":                 "UNKNOWN",
}

DISCLAIMER = (
    "⚠  旁路监控模式 — 状态为启发式推断，非 app-server 确认状态\n"
    "   仅用于人工参考，不执行自动调度决策"
)


def format_human(thread, classification, show_disclaimer=True):
    """人类可读输出"""
    status = classification["status"]
    emoji = STATUS_EMOJI.get(status, "❓")
    label = STATUS_LABEL.get(status, status)
    conf = classification["confidence"]
    title = thread.get("title") or thread.get("preview") or "(no title)"
    cwd = thread.get("cwd") or "?"
    tid = thread["id"]
    idle = classification["idle_seconds"]
    reason = classification["reason"]
    alerts = classification.get("alerts", [])

    lines = []
    if show_disclaimer:
        lines.append(DISCLAIMER)
        lines.append("")
    lines.append(f"{emoji}  {label:22s}  conf={conf:.2f}  idle={idle:6.0f}s")
    lines.append(f"    id={tid}")
    lines.append(f"    cwd={cwd}")
    lines.append(f"    title={title}")
    lines.append(f"    reason={reason}")
    for a in alerts:
        lines.append(f"    ⚠  {a}")
    return "\n".join(lines)


def format_json(threads_results):
    """机器可读 JSON 输出"""
    return json.dumps({
        "monitor_mode": "bypass_heuristic",
        "disclaimer": (
            "Heuristic inference only. Not app-server confirmed state. "
            "Not for automatic scheduling decisions."
        ),
        "timestamp": time.time(),
        "threads": [
            {
                "id": tr["thread"]["id"],
                "title": tr["thread"].get("title") or tr["thread"].get("preview"),
                "cwd": tr["thread"].get("cwd"),
                "rollout_path": tr["thread"].get("rollout_path"),
                "status": tr["result"]["status"],
                "confidence": tr["result"]["confidence"],
                "reason": tr["result"]["reason"],
                "idle_seconds": tr["result"]["idle_seconds"],
                "alerts": tr["result"].get("alerts", []),
            }
            for tr in threads_results
        ],
    }, indent=2, ensure_ascii=False)


def format_alert_only(thread, classification):
    """只输出需要人工检查的 thread（简洁格式）"""
    status = classification["status"]
    alert_stati = {"LIKELY_WAITING_APPROVAL", "LIKELY_WAITING_USER", "LIKELY_ERROR", "LIKELY_PAUSED"}
    if status not in alert_stati:
        return None

    emoji = STATUS_EMOJI.get(status, "❓")
    title = thread.get("title") or thread.get("preview") or "(no title)"
    cwd = thread.get("cwd") or "?"
    tid = thread["id"]
    reason = classification["reason"]

    return f"{emoji} [{STATUS_LABEL.get(status, status)}] {title}  |  {cwd}  |  {tid}\n   {reason}"


# ══════════════════════════════════════════════════════════
#  Main
# ══════════════════════════════════════════════════════════

def find_databases():
    """查找所有 state_5.sqlite 和 logs_2.sqlite"""
    state_dbs = find_all_db("state_5.sqlite")
    logs_dbs = find_all_db("logs_2.sqlite")
    if not state_dbs:
        print("ERROR: state_5.sqlite not found in any candidate dir:")
        for d in CANDIDATE_SQLITE_DIRS:
            print(f"  {d}")
        raise SystemExit(1)
    return state_dbs, logs_dbs


def probe(state_dbs, logs_dbs, limit=30):
    """
    单次探测：遍历所有 state DB 读取 thread，logs DB 自动匹配，融合判断。
    可能多个 state DB 并存（legacy + 新路径），按 thread id 去重。
    """
    seen_ids = set()
    results = []

    if not isinstance(state_dbs, list):
        state_dbs = [state_dbs]
    if not isinstance(logs_dbs, list):
        logs_dbs = [logs_dbs] if logs_dbs else []

    for sdb in state_dbs:
        threads = read_recent_threads(sdb, limit=limit * 2)
        for t in threads:
            if t["id"] in seen_ids:
                continue
            seen_ids.add(t["id"])

            # 找匹配的 logs DB（优先同目录）
            logs_for_thread = []
            for ldb in logs_dbs:
                logs = read_recent_logs(ldb, t["id"], limit=80)
                logs_for_thread.extend(logs)
                if logs:
                    break

            result = classify(t, logs_for_thread)
            results.append({"thread": t, "logs": logs_for_thread, "result": result})

    # 按最近活动排序
    results.sort(key=lambda r: r["result"]["idle_seconds"])
    return results[:limit]


def run_single(args):
    """单次探测并输出"""
    state_dbs, logs_dbs = find_databases()
    results = probe(state_dbs, logs_dbs, limit=args.limit)

    if args.json:
        print(format_json(results))
        return

    if args.alert_only:
        for r in results:
            line = format_alert_only(r["thread"], r["result"])
            if line:
                print(line)
        return

    # 默认：人类可读输出
    for sdb in state_dbs:
        print(f"state_db={sdb}")
    for ldb in logs_dbs:
        print(f"logs_db={ldb}")
    print(f"threads={len(results)}")
    print()

    for r in results:
        print(format_human(r["thread"], r["result"], show_disclaimer=False))
        print()

    # 汇总 alert
    alerts = [r for r in results if r["result"].get("alerts")]
    if alerts:
        print(f"\n{'='*60}")
        print(f"⚠  {len(alerts)} thread(s) 需要人工检查：")
        print(f"{'='*60}")
        for r in alerts:
            s = r["result"]["status"]
            emoji = STATUS_EMOJI.get(s, "❓")
            label = STATUS_LABEL.get(s, s)
            title = r["thread"].get("title") or r["thread"].get("preview") or "(no title)"
            print(f"  {emoji} [{label}] {title}")


def run_watch(args):
    """持续监控模式"""
    interval = args.interval or REFRESH_INTERVAL
    state_dbs, logs_dbs = find_databases()

    print(f"旁路监控启动 — 每 {interval}s 刷新 (Ctrl+C 停止)")
    print(DISCLAIMER)
    print()

    try:
        while True:
            # 清屏
            sys.stderr.write("\033[2J\033[H")
            sys.stderr.flush()

            now_str = time.strftime("%Y-%m-%d %H:%M:%S")
            results = probe(state_dbs, logs_dbs, limit=args.limit)

            print(f"=== Codex Desktop 状态探测器 [{now_str}] ===")
            print()

            # 输出需要关注的 thread
            alerts = [r for r in results if r["result"].get("alerts")]
            working = [r for r in results if r["result"]["status"] == "LIKELY_WORKING"]

            if alerts:
                print(f"⚠  需人工检查 ({len(alerts)}):")
                print("-" * 50)
                for r in alerts:
                    line = format_alert_only(r["thread"], r["result"])
                    if line:
                        print(f"  {line}")
                print()

            if working:
                print(f"🟢 运行中 ({len(working)}):")
                print("-" * 50)
                for r in working:
                    title = r["thread"].get("title") or r["thread"].get("preview") or "(no title)"
                    cwd = r["thread"].get("cwd") or "?"
                    idle = r["result"]["idle_seconds"]
                    print(f"  🟢 {title:40s}  idle={idle:5.0f}s  {cwd}")
                print()

            # 其他 thread 合计
            others = len(results) - len(alerts) - len(working)
            if others > 0:
                print(f"(其他 {others} thread 无异常)")

            print()
            print("--- 按 Ctrl+C 退出 ---")

            time.sleep(interval)
    except KeyboardInterrupt:
        print("\n监控停止")


def main():
    parser = argparse.ArgumentParser(
        description="Codex Desktop 旁路监控探测器 — 方案二",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__,
    )
    parser.add_argument(
        "--watch", action="store_true",
        help="持续监控模式（默认：单次探测）"
    )
    parser.add_argument(
        "--interval", type=int, default=REFRESH_INTERVAL,
        help=f"监控刷新间隔（秒，默认 {REFRESH_INTERVAL}）"
    )
    parser.add_argument(
        "--limit", type=int, default=30,
        help="最多检查 thread 数（默认 30）"
    )
    parser.add_argument(
        "--json", action="store_true",
        help="JSON 输出（默认：人类可读）"
    )
    parser.add_argument(
        "--alert-only", action="store_true",
        help="只输出需要人工检查的 thread"
    )

    args = parser.parse_args()

    if args.watch:
        run_watch(args)
    else:
        run_single(args)


if __name__ == "__main__":
    main()
