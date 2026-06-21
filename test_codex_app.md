可以间接做，但要分清 **“可可靠获取的状态”** 和 **“只能启发式推断的状态”**。

## 结论

**读取 `state_5.sqlite` + session JSONL 可以做一个“状态探测器”，但不能强一致地判断 Desktop 当前是否 blocked / waiting approval。**
原因是 Codex 的实时 runtime status 不是直接持久化在 SQLite 里的，而是在 app-server 进程内存中维护。源码里的 `ThreadWatchManager` 用内存字段维护 `running`、`pending_permission_requests`、`pending_user_input_requests`、`has_system_error`，再动态计算 `ThreadStatus`；`waitingOnApproval` 和 `waitingOnUserInput` 都来自这些内存计数。([GitHub][1])

所以：

| 目标                               | 读取 SQLite / JSONL 能不能做 |
| -------------------------------- | ---------------------- |
| 找到 Desktop 里有哪些历史 session        | 可以                     |
| 找到最近活跃/最近更新的 session             | 可以                     |
| 找到 session 对应项目目录、标题、rollout 文件  | 可以                     |
| 判断最近是否还在输出/写日志                   | 可以，启发式                 |
| 判断是否正在 working                   | 可以，启发式                 |
| 判断是否等待 approval                  | 可以，弱启发式                |
| 判断是否等待用户输入                       | 可以，弱启发式                |
| 强一致判断 blocked / waiting approval | 不可靠，除非接入 app-server 事件 |

---

## 1. Codex 本地数据在哪里

官方环境变量文档说明，`CODEX_HOME` 默认是 `~/.codex`，包含 config、auth、logs、sessions、skills 等状态；`CODEX_SQLITE_HOME` 用于设置 SQLite-backed state 的位置，默认跟随 `CODEX_HOME`。([OpenAI开发者][2])

实际你需要检查这些路径：

```bash
~/.codex/state_5.sqlite
~/.codex/logs_2.sqlite
~/.codex/sessions/
~/.codex/archived_sessions/
~/.codex/session_index.jsonl
~/.codex/sqlite/state_5.sqlite       # 新版本/迁移场景可能出现
~/.codex/sqlite/logs_2.sqlite
```

GitHub issue 里也能看到 Desktop 侧确实大量依赖 `state_5.sqlite`，而 CLI/ACP 场景可能写 JSONL session 文件和 `session_index.jsonl`，导致 Desktop UI 与 session 文件不同步。([GitHub][3]) 近期 issue 还提到新路径 `~/.codex/sqlite/state_5.sqlite` 与 legacy/root `~/.codex/state_5.sqlite` 可能并存，并出现 backfill 状态差异。([GitHub][4])

---

## 2. `state_5.sqlite` 能提供什么

源码里 `StateRuntime::get_thread()` 查询 `threads` 表，字段包括：

```sql
id,
rollout_path,
created_at_ms,
updated_at_ms,
recency_at_ms,
source,
thread_source,
model_provider,
model,
cwd,
cli_version,
title,
preview,
sandbox_policy,
approval_mode,
tokens_used,
first_user_message,
archived_at,
git_sha,
git_branch,
git_origin_url
```

这些是 **session 元数据**，不是实时运行状态。源码查询可以直接看到这些字段。([GitHub][5])

你可以用它做 session inventory：

```bash
sqlite3 -readonly ~/.codex/state_5.sqlite '
SELECT
  id,
  datetime(updated_at_ms / 1000, "unixepoch", "localtime") AS updated_at,
  datetime(recency_at_ms / 1000, "unixepoch", "localtime") AS recency_at,
  source,
  cwd,
  title,
  preview,
  rollout_path,
  approval_mode,
  archived
FROM threads
WHERE archived = 0
ORDER BY recency_at_ms DESC
LIMIT 30;
'
```

如果新版本使用 `~/.codex/sqlite/state_5.sqlite`，就优先查这个；如果查不到或 backfill 不完整，再查 legacy root 版本。

---

## 3. 为什么 SQLite 不能强一致判断 blocked

官方 app-server 文档说明，`thread/list` / `thread/read` 返回的 `thread` 会带 `status`，但这个 `status` 是 runtime 状态；`thread/loaded/list` 才是当前内存中 loaded 的 thread id。文档明确说 `thread/loaded/list` 用来检查哪些 session active，而不用扫描磁盘 rollout。([GitHub][6])

源码也说明实时状态是这样算的：

```text
pending_permission_requests > 0  => waitingOnApproval
pending_user_input_requests > 0  => waitingOnUserInput
running == true                  => active
has_system_error == true         => systemError
otherwise                        => idle
```

这套状态在 app-server 的 `ThreadWatchManager` 内存中，不是 `threads` 表字段。([GitHub][1])

因此只读 SQLite 时，你最多能判断：

```text
这个 session 最近有更新
这个 session 的 rollout 文件最近有写入
这个 session 的 logs 最近有输出
这个 session 最后一条记录不像 completed
```

但不能 100% 判断：

```text
当前 Desktop UI 正停在 approval dialog
当前 turn 还没结束
当前正在等用户输入
```

---

## 4. `logs_2.sqlite` 可以增强判断

`logs_2.sqlite` 里有 `logs` 表，源码写入字段包括：

```sql
ts,
ts_nanos,
level,
target,
feedback_log_body,
thread_id,
process_uuid,
module_path,
file,
line,
estimated_bytes
```

查询接口也是按 `thread_id` / `process_uuid` / 时间倒序查日志。([GitHub][7])

你可以用它判断“最近是否仍在活动”：

```bash
sqlite3 -readonly ~/.codex/logs_2.sqlite '
SELECT
  id,
  datetime(ts, "unixepoch", "localtime") AS ts,
  level,
  target,
  thread_id,
  substr(feedback_log_body, 1, 300) AS msg
FROM logs
WHERE thread_id IS NOT NULL
ORDER BY ts DESC, ts_nanos DESC, id DESC
LIMIT 100;
'
```

针对某个 thread：

```bash
THREAD_ID="thr_xxx"

sqlite3 -readonly ~/.codex/logs_2.sqlite "
SELECT
  datetime(ts, 'unixepoch', 'localtime') AS ts,
  level,
  target,
  substr(feedback_log_body, 1, 500) AS msg
FROM logs
WHERE thread_id = '$THREAD_ID'
ORDER BY ts DESC, ts_nanos DESC, id DESC
LIMIT 50;
"
```

然后做启发式：

| 日志/文件现象                                                                                        | 推断                    |
| ---------------------------------------------------------------------------------------------- | --------------------- |
| 最近 5–10 秒持续有 log 或 rollout 写入                                                                  | working               |
| 最近出现 approval / requestApproval / permissions / sandbox / waiting 字样，之后没有 resolved / completed | 可能 waiting approval   |
| 最近出现 user input / requestUserInput / question 字样，之后没有 completed                                | 可能 waiting user input |
| 最近出现 completed / interrupted / error / failed                                                  | 可能已结束或失败              |
| 几分钟无新日志且 rollout mtime 不变                                                                      | 多半 idle 或卡死           |

注意：日志内容不是稳定 API，版本变化会影响关键字。

---

## 5. session JSONL 能做什么

session JSONL 的主要价值是补充 `state_5.sqlite`：

```text
state_5.sqlite
  -> 找 thread id、cwd、title、rollout_path

rollout_path / sessions/*.jsonl
  -> tail 最新事件
  -> 判断最近是否有 assistant delta、tool call、command execution、completion
```

官方 README 也说明 archive 操作移动的是 persisted rollout，也就是磁盘 JSONL 文件。([GitHub][6])

你可以这样从 SQLite 找 rollout 文件：

```bash
sqlite3 -readonly ~/.codex/state_5.sqlite '
SELECT id, rollout_path
FROM threads
WHERE archived = 0
ORDER BY recency_at_ms DESC
LIMIT 20;
'
```

然后 tail：

```bash
tail -n 80 "$ROLLOUT_PATH"
```

建议不要写死 JSONL schema，而是做宽松模式：

```text
1. 解析每行 JSON
2. 提取 type / item.type / event / msg / status / role 等常见字段
3. 保留原始 JSON fallback
4. 只做低置信度判断
```

---

## 6. 推荐的间接监控算法

你的需求不是控制 Codex，只是判断状态，所以可以接受“多信号融合”。

### 状态分级

```text
CONFIRMED_ACTIVE:
  app-server 能返回 active

LIKELY_WORKING:
  最近 N 秒 logs_2.sqlite 有该 thread 新日志
  或 rollout_path mtime 持续变化
  或 JSONL tail 末尾是未完成的 tool/model 输出

LIKELY_WAITING_APPROVAL:
  最近事件/日志出现 approval/requestApproval/permissions/sandbox
  且之后没有 resolved/completed/denied/approved/turn completed

LIKELY_WAITING_USER:
  最近事件/日志出现 requestUserInput/question/needs input
  且之后没有 user response/turn completed

LIKELY_IDLE:
  state_5.sqlite 存在 thread
  但最近无日志、无 rollout 更新
  tail 最后为 completed/interrupted/error

UNKNOWN:
  DB 与 JSONL 不一致
  rollout_path 不存在
  Desktop migration/backfill 状态异常
```

### 探测流程

```text
1. 找 SQLite home
   - CODEX_SQLITE_HOME
   - ~/.codex/sqlite
   - ~/.codex

2. 查 state_5.sqlite.threads
   - id
   - cwd
   - title
   - rollout_path
   - updated_at_ms
   - recency_at_ms
   - source
   - approval_mode
   - archived

3. 对最近 N 个 thread：
   - stat rollout_path
   - tail JSONL 最新 100 行
   - 查 logs_2.sqlite 最新 100 行

4. 合并判断：
   - active_score
   - approval_score
   - user_input_score
   - idle_score

5. 输出状态和置信度：
   - working / likely
   - waiting_approval / likely
   - waiting_user / likely
   - idle / likely
   - unknown
```

---

## 7. 一个可用的 Python 探测脚本思路

```python
#!/usr/bin/env python3
import json
import os
import sqlite3
import time
from pathlib import Path

CODEX_HOME = Path(os.environ.get("CODEX_HOME", "~/.codex")).expanduser()

CANDIDATE_SQLITE_DIRS = [
    Path(os.environ["CODEX_SQLITE_HOME"]).expanduser()
    if os.environ.get("CODEX_SQLITE_HOME") else None,
    CODEX_HOME / "sqlite",
    CODEX_HOME,
]
CANDIDATE_SQLITE_DIRS = [p for p in CANDIDATE_SQLITE_DIRS if p]

KEY_APPROVAL = [
    "requestapproval",
    "approval",
    "permissions/requestapproval",
    "commandexecution/requestapproval",
    "filechange/requestapproval",
    "sandbox",
]
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
    "serverrequest/resolved",
    "resolved",
]


def ro_connect(path: Path):
    uri = f"file:{path}?mode=ro&immutable=1"
    return sqlite3.connect(uri, uri=True)


def find_db(name: str) -> Path | None:
    for d in CANDIDATE_SQLITE_DIRS:
        p = d / name
        if p.exists():
            return p
    return None


def tail_lines(path: Path, n=120):
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


def read_recent_threads(state_db: Path, limit=30):
    con = ro_connect(state_db)
    con.row_factory = sqlite3.Row
    rows = con.execute("""
        SELECT
          id, title, preview, cwd, rollout_path,
          source, approval_mode, updated_at_ms, recency_at_ms, archived
        FROM threads
        WHERE COALESCE(archived, 0) = 0
        ORDER BY recency_at_ms DESC
        LIMIT ?
    """, (limit,)).fetchall()
    con.close()
    return [dict(r) for r in rows]


def read_recent_logs(logs_db: Path | None, thread_id: str, limit=80):
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
    except Exception:
        return []


def classify(thread, logs):
    now = time.time()
    rollout_path = Path(thread["rollout_path"]).expanduser() if thread.get("rollout_path") else None

    mtime = None
    tail_text = ""
    if rollout_path and rollout_path.exists():
        mtime = rollout_path.stat().st_mtime
        tail_text = "\n".join(tail_lines(rollout_path, 120)).lower()

    log_text = "\n".join(str(x.get("msg") or "") for x in logs).lower()
    full = tail_text + "\n" + log_text

    last_activity = max(
        [x for x in [
            mtime,
            (logs[0]["ts"] if logs else None),
            (thread.get("recency_at_ms") or 0) / 1000,
        ] if x]
    )

    age = now - last_activity if last_activity else 999999

    has_approval = any(k in full for k in KEY_APPROVAL)
    has_user_input = any(k in full for k in KEY_USER_INPUT)
    has_done = any(k in full for k in KEY_DONE)

    if age <= 15 and not has_done:
        base = "likely_working"
        confidence = 0.70
    elif age <= 60:
        base = "recent_activity"
        confidence = 0.55
    else:
        base = "likely_idle"
        confidence = 0.60

    if has_approval and not has_done:
        return "likely_waiting_approval", max(confidence, 0.65), age

    if has_user_input and not has_done:
        return "likely_waiting_user_input", max(confidence, 0.65), age

    return base, confidence, age


def main():
    state_db = find_db("state_5.sqlite")
    logs_db = find_db("logs_2.sqlite")

    if not state_db:
        raise SystemExit("state_5.sqlite not found")

    threads = read_recent_threads(state_db, limit=30)

    print(f"state_db={state_db}")
    print(f"logs_db={logs_db}")
    print()

    for t in threads:
        logs = read_recent_logs(logs_db, t["id"], limit=80)
        status, conf, age = classify(t, logs)
        print(f"{status:28} conf={conf:.2f} age={age:7.1f}s")
        print(f"  id={t['id']}")
        print(f"  cwd={t.get('cwd')}")
        print(f"  title={t.get('title') or t.get('preview')}")
        print(f"  rollout={t.get('rollout_path')}")
        print()


if __name__ == "__main__":
    main()
```

这个脚本的定位是 **旁路观察器**，不是严格状态源。

---

## 8. 最佳工程方案

如果你只是想“发现可能卡住的 Desktop session”，可以用：

```text
state_5.sqlite + logs_2.sqlite + rollout JSONL tail
```

但 UI 上要明确标注：

```text
LIKELY_WORKING
LIKELY_WAITING_APPROVAL
LIKELY_WAITING_USER_INPUT
LIKELY_IDLE
UNKNOWN
```

不要标成强确定状态。

如果你需要强确定：

```text
working
blocked
waiting approval
waiting user input
```

仍然应该优先接 app-server，因为官方协议直接提供 `thread/loaded/list`、`thread/status/changed`，并且状态可以包含 `activeFlags`。([GitHub][6])

最终建议：

```text
方案 1：强一致
  app-server 事件流作为主状态源
  SQLite / JSONL 只做恢复和补充

方案 2：旁路监控 Desktop
  SQLite / logs / JSONL 融合
  输出 likely 状态
  不执行自动调度决策，只提醒人工检查

方案 3：混合
  能连 app-server 就用 app-server
  连不上或不是同一 Desktop runtime 时，降级为 SQLite/JSONL 旁路推断
```

你的场景如果只是“看到哪些 Codex Desktop 会话可能卡住，需要用户处理”，方案 2 足够。但如果后续要自动调度、自动接管、自动中断，SQLite/JSONL 方案不够可靠。

[1]: https://raw.githubusercontent.com/openai/codex/main/codex-rs/app-server/src/thread_status.rs "raw.githubusercontent.com"
[2]: https://developers.openai.com/codex/environment-variables "Environment variables – Codex | OpenAI Developers"
[3]: https://github.com/openai/codex/issues/16385 "ACP-spawned sessions don't appear in the Codex desktop app · Issue #16385 · openai/codex · GitHub"
[4]: https://github.com/openai/codex/issues/28087 "Codex Desktop can get stuck on sqlite state backfill with partial new index while legacy state is complete · Issue #28087 · openai/codex · GitHub"
[5]: https://raw.githubusercontent.com/openai/codex/main/codex-rs/state/src/runtime/threads.rs "raw.githubusercontent.com"
[6]: https://github.com/openai/codex/blob/main/codex-rs/app-server/README.md "codex/codex-rs/app-server/README.md at main · openai/codex · GitHub"
[7]: https://raw.githubusercontent.com/openai/codex/main/codex-rs/state/src/runtime/logs.rs "raw.githubusercontent.com"

