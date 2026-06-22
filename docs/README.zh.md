# herdr-agentview

在 **Ulanzi D200X** 硬件面板和 **桌面 Fleet Board** 上显示 [herdr](https://herdr.dev) AI 编程 Agent 的状态。

> 架构参考: [AGENTS.md](../AGENTS.md)
> 开发指南: [docs/development-guide.md](./development-guide.md)

## 平台支持

**macOS、Linux、Windows。** 桌面面板 (panel-gio) 基于 Gio UI，支持全平台。collector 和 deck 仅在 macOS/Linux 运行。

## 功能

- **实时 Agent 状态** — 读取 herdr 数据，同时显示在 D200X 和桌面面板上
- **多机器支持** — 同时连接多台 herdr 实例（本机 + SSH 隧道）
- **优先级排序** — BLOCKED → DONE → WORKING → IDLE → UNKNOWN
- **桌面 Fleet Board** — 5 区域信息面板（状态统计、机器过滤、异常卡片、Agent 矩阵、选中信息）
- **NATS 推送** — Collector 每 2s 轮询 herdr，通过嵌入式 NATS 推送快照

## 项目结构

三进程架构 + 共享模块：

```
herdr-collector (状态采集 + 内置 NATS)
      │
      │ NATS subjects
      ▼
herdr-deck (Ulanzi D200X 硬件显示)
herdr-panel (桌面 Fleet Board, Gio UI)

共享模块:
  protocol/     — 类型定义、状态枚举、NATS subject 常量
  displaymodel/ — 视图状态、过滤、导航、统计模型
```

## 快速开始

### 依赖

- [herdr](https://herdr.dev) 运行中
- Go 1.26+

### 构建与运行

```bash
# 构建 collector
cd collector && make build
./build/herdr-collector --debug

# 新终端，启动桌面面板
cd panel-gio && make build
./build/herdr-panel --debug

# 或一键启动全部
bash scripts/deploy-all.sh
```

## 桌面面板 (panel-gio)

### 界面区域

```
┌─────────────────────────────────────────────────────────┐
│ TOTAL 42 │ 3 blocked 8 working 5 idle 3 done 1 unknown ● LIVE · 14:30:00 │
├─────────────────────────────────────────────────────────┤
│ FOCUS  MACHINE  SPACES                                  │
│ ALL    LCL DEV  PRD RUN  api web  docs  golang spot     │
├─────────────────────────────────────────────────────────┤
│ ATTENTION                                               │
│ ╭──────╮ ╭──────╮ ╭──────╮                             │
│ │ ●api │ │ ✓doc │ │ ◆ui  │                             │
│ │block │ │done  │ │work  │                             │
│ │iplist│ │web   │ │DEV   │                             │
│ ╰──────╯ ╰──────╯ ╰──────╯                             │
├─────────────────────────────────────────────────────────┤
│ AGENT GRID                                              │
│ DEV  ◆ ui  ✓ test  ● api                                │
│ LCL  ◆ build ○ idle                                     │
├─────────────────────────────────────────────────────────┤
│ SELECTED api · claude · DEV / iplist                    │
└─────────────────────────────────────────────────────────┘
```

### 键盘快捷键

| 键 | 功能 |
|----|------|
| `A` | **ALL / ACTIVE** — 全部显示 / 仅显示活跃 agent |
| `M` | **机器过滤** — 点击切换机器显隐，多选 |
| `P` | **Space 过滤** — 点击切换项目显隐，多选 |
| `R` | **重置** — 清空所有过滤，显示全部 |
| `1-9` | **选中 Attention** — 选中 Attention 区域卡片 |
| `Enter` | **Focus** — 发送 focus 命令到 collector |
| `Esc` | **清空选中** — 取消 Attention 选中 |

详情见 [AGENTS.md](../AGENTS.md)。

## Deck 按钮功能 (D200X)

| 按键 | 功能 |
|-----|------|
| K1-K10 | Agent 状态（优先级排序） |
| K11 | **ALL / ACTIVE** — 显示全部或仅 BLOCKED/WORKING/DONE |
| K12 | **机器循环** — 切换机器，清空 Space 过滤 |
| K13 | **Space 循环（全局）** — 按 workspace 标签全局过滤 |
| K14 | **全局统计** — D / I / W / B / ? 计数，CPU/MEM 占用 |

## Agent 状态优先级

1. **BLOCKED** — 最高优先级（红）
2. **DONE** — 完成（绿）
3. **WORKING** — 进行中（黄）
4. **IDLE** — 闲置（灰）
5. **UNKNOWN** — （灰）

## 开发

```bash
# 分模块测试
cd protocol     && go test ./...
cd displaymodel && go test ./...
cd collector    && make test
cd deck         && make test
cd panel-gio    && make test

# 构建
cd collector && make build
cd deck      && make build
cd panel-gio && make build
```

详见 [docs/development-guide.md](./development-guide.md)。

## 变更历史

### 2026-06-22 — Deck ImageCache + 共享 FontFamily

**问题:** `herdr-deck` 在 macOS 上物理足迹达 1.5GB (RSS 仅 23MB)，根因是每次
SVG→PNG 转换都创建新的 `canvas.FontFamily`，每次加载 11 种系统字体
(CoreText)，累计约 1980 次字体加载。

**修复 (方案 A: per-key render cache):**

1. **`ImageCache`** (`deck/internal/deckclient/render_cache.go`) — 3 层缓存:
   - `latestByKey`: 同物理键同 SVG 跳过发送 (硬件已有)
   - `LRU` (64 entry): 不同键同 SVG 复用缓存 PNG，零转换
   - 未命中: SVG→PNG 转换，结果入缓存
2. **共享 `FontFamily`** (`draw.go` init()) — 只创建 1 个 FontFamily，
   加载 4 种字重 × 11 字体，代替原来每次渲染创建新的 Family
3. **移除 `KeyHashTracker`** — 被 ImageCache 替代

**效果:**

- 稳定态 0 次 canvas 调用/cycle (之前平均 2-5 次)
- macOS footprint 1.5GB → ~200MB
- `go test ./... -race` 218 用例全过

详见 [debugging.md](./debugging.md) 第 6 节。
