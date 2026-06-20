# Herdr Agent Status on UlanziDeck — 设计方案

> 版本: v1.0  
> 日期: 2026-06-20  
> 设备: Ulanzi D200X

---

## 1. 系统架构

```
┌──────────────────────────────────────────────────┐
│  Herdr Server (Machine 1)      Herdr Server (M2) │
│  ~/.local/share/herdr/...      (remote via SSH)  │
└────────────┬──────────────────────────┬──────────┘
             │ agent.list / subscribe    │ agent.list / subscribe
             ▼                           ▼
┌──────────────────────────────────────────────────┐
│  SSH Tunnel Manager                               │
│  └─ 自动 spawn `ssh -L <localPort>:<remoteSocket>`│
│     每个远端连接一个本地转发端口                    │
└────────────────────┬─────────────────────────────┘
                     │ localhost:<portN>
                     ▼
┌──────────────────────────────────────────────────┐
│  桥接进程 (Node.js)                               │
│  ├─ ConnectionManager             连接生命周期    │
│  │   ├─ LocalConnection:   直连 Unix socket      │
│  │   └─ SSHTunnelConnection: 连 localhost:<port> │
│  ├─ HerdrClient:         每个连接一个实例          │
│  ├─ DeckClient:          WebSocket 更新 UlanziDeck│
│  ├─ StateManager:        统一状态树(含机器维度)    │
│  └─ ButtonMapper:        分页 + 按键映射          │
└────────────────────┬─────────────────────────────┘
                     │ WebSocket ws://127.0.0.1:3906
                     ▼
┌──────────────────────────────────────────────────┐
│  UlanziDeck 上位机                                │
│  └─ 14× LCD 按键 + 3× Encoder LCD 渲染           │
└──────────────────────────────────────────────────┘
```

### 1.1 连接配置

```json
{
  "connections": [
    {
      "name": "local",
      "abbr": "LCL",
      "type": "local"
    },
    {
      "name": "dev-server",
      "abbr": "DEV",
      "type": "ssh",
      "host": "dev.internal.com",
      "remoteSocket": "~/.local/share/herdr/herdr.sock"
    }
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 内部标识，日志用 |
| `abbr` | string | 是 | 缩写，显示在 K12 按键上（如 `LCL:main-proj`） |
| `type` | `"local"` \| `"ssh"` | 是 | 连接类型 |
| `host` | string | 仅 ssh | SSH 目标（匹配 `~/.ssh/config` 的 Host） |
| `remoteSocket` | string | 仅 ssh | 远端 herdr socket 路径 |

> 注意：`connections` 数组的顺序决定分页排列顺序。

### 1.2 接口协议

| 连接 | 协议 | 方向 |
|------|------|------|
| Herdr ↔ 桥接（直连） | Unix Socket (JSON-line) | 双向请求/响应 + 事件订阅推送 |
| Herdr ↔ 桥接（SSH） | Unix Socket via `ssh -L` 隧道，TCP localhost:<port> | 同上 |
| 桥接 ↔ UlanziDeck | WebSocket (JSON) | 桥接 → 上位机: `state` 命令; 上位机 → 桥接: `keydown`/`keyup` 事件 |

### 1.3 SSH 隧道管理

- 桥接启动时读取配置，为每个 `type: "ssh"` 的连接自动 spawn `ssh -L` 进程
- 本地端口分配：绑定到 `0`（OS 自动分配空闲端口），解析实际绑定端口
- SSH 认证：依赖用户 `~/.ssh/config`，桥接不管理密钥
- 生命周期监控：`ssh -L` 进程意外退出时自动重连
- 重连策略：指数退避（1s → 2s → 4s → ... → 60s max）

### 1.4 数据流

```
[启动]
  │
  ├─ 读取配置文件
  ├─ 启动 SSH tunnel（对每个 type:ssh 的连接）
  ├─ 遍历所有 connections:
  │   ├─ 建立 HerdrClient 连接
  │   ├─ agent.list → 获取该连接下全部 Agent
  │   └─ workspace.list → 获取该连接下全部 Workspace
  ├─ 连接 UlanziDeck WebSocket
  │
  ├─ [统一状态树]
  │   ├─ 所有连接的 WS/Agent 按配置顺序合并
  │   ├─ 每条记录带有 connName 标识来源机器
  │   └─ 分页逻辑在合并后的列表上运行
  │
  ├─ [事件循环]
  │   ├─ 每个连接独立订阅 events.subscribe
  │   ├─ 收到 event → 更新对应连接的局部状态
  │   ├─ 合并到统一状态树 → 重新计算分页 → 渲染按键
  │   └─ keydown 事件 → agent.focus 发送到对应连接
  │
  └─ [断开/重连]
      ├─ 单个连接断开 → 仅该连接重连，不影响其他
      └─ 全部断开时全局等待
```

---

## 2. D200X 物理按键布局

```
┌────────────────────────────────────────────────────┐
│                                                    │
│  ┌────┬────┬────┬────┬────┐                        │
│  │ K1 │ K2 │ K3 │ K4 │ K5 │  行1: WS-A Agents(≤5) │
│  ├────┼────┼────┼────┼────┤                        │
│  │ K6 │ K7 │ K8 │ K9 │ K10│  行2: WS-B Agents(≤5) │
│  ├────┼────┼────┼────┼────┤                        │
│  │K11 │K12 │K13 │ K14(长条形)  │  行3: 导航+统计   │
│  └────┴────┴────┴──────────────┘                    │
│                                                    │
│  ┌──────────────────────────────────────────────┐  │
│  │  Encoder 1   │  Encoder 2   │  Encoder 3     │  │
│  └──────────────┴──────────────┴──────────────┘  │
│                                                    │
└────────────────────────────────────────────────────┘
```

### 2.1 按键功能分配

| 键位 | 数量 | 功能 | 说明 |
|------|------|------|------|
| K1-K10 | 10 | Agent 状态键 | 按状态优先级排序显示前 10 个 Agent。排序规则：BLOCKED > DONE > WORKING > IDLE > UNKNOWN，同状态内按机器分组 |
| K11 | 1 | ALL 模式 | 切换到 ALL 模式，显示全部机器的 Agent |
| K12 | 1 | 上一台机器 | 切换到前一台机器的 Agent（单机间循环：LCL → DEV → PRD → LCL...） |
| K13 | 1 | 下一个 Space | 在当前选中的机器内，切换到下一个 Space（K12 和 K13 取交集） |
| K14 | 1 (长条形) | 全局统计栏 | 跨所有 Workspace 的全部 Agent 状态统计，始终固定不变 |

### 2.2 Encoder 旋钮功能分配

| 旋钮 | 功能 | LCD 显示内容 | 点击动作 |
|------|------|-------------|---------|
| Encoder 1 | 当前页所选 Agent 的 Tab 切换 | 当前 Tab 名称 | 切换到选中的 Tab |
| Encoder 2 | 选中 Agent 详情浏览 | Agent 自定义状态 / state_labels / custom_status | 聚焦到该 Agent（herdr agent focus） |
| Encoder 3 | 翻页 | 页码 "Page 2/5" | 无 |

---

## 3. 按键视觉设计

### 3.1 Agent 键（K1-K10） — 二段式布局

#### 最终设计

```
┌──────────────────────┐
│ ▓▓▓ PI ▓▓▓  ▓▓ LCL ▓│  ← 48px 顶栏: 左=Agent品牌色+名, 右=机器色+缩写
│▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁│  ← 1px 白分隔线
│                      │
│       review         │  ← 别名 36px BOLD (白色)
│                      │
│          W           │  ← 状态首字母 20px BOLD (白色)
│      main-proj       │  ← WS名称 26px BOLD (白色)
│                      │
└──────────────────────┘
  ↑ 全背景 = 工作状态色 (DONE绿/IDLE灰/WORKING琥珀/BLOCKED红/UNKNOWN灰)
     + 黑遮罩 0.15 (保证白字在所有状态下可读)
```

#### 各区域说明

| 区域 | 位置 | 内容 | 字号 |
|------|------|------|------|
| 顶栏左半 | 0-100, 0-48 | Agent 品牌色背景 + Agent 名 | 24px BOLD 白 |
| 顶栏右半 | 100-200, 0-48 | 机器缩写色背景 + 机器缩写 | 24px BOLD 白 |
| 分隔线 | y=48 | 1px 白色线, opacity=0.25 | - |
| 背景区 | 49-200 | 工作状态色 + 黑遮罩 0.15 | - |
| 别名 | 居中 | 用户自定义别名 (agent.name) | **36px BOLD 白** |
| 状态字母 | 居中偏下 | 状态首字母 (D/I/W/B/?) | 20px BOLD 白 |
| WS 名 | 底部 | 当前 Workspace label | **26px BOLD 白** |
| 聚焦边框 | 边缘 | 2-3px 白色发光边框 | focused=true 时显示 |

#### Agent 品牌色 + 机器色

Agent 品牌色用于顶栏左半（标识 Agent 类型），机器色用于顶栏右半（标识机器来源）。
品牌色和状态背景色之间有白分隔线隔离，防止颜色混淆。

**Agent 品牌色表：**

| Agent | 品牌色 |
|-------|--------|
| Pi | `#7C3AED` (紫) |
| Claude | `#D97757` (暖棕) |
| Cursor | `#00B884` (青绿) |
| Cline | `#2563EB` (蓝) |
| Codex | `#1E293B` (暗灰) |
| Gemini | `#4285F4` (Google 蓝) |
| Copilot | `#8957E5` (紫) |
| Devin | `#FF6B35` (橙) |
| Grok | `#1DA1F2` (Twitter 蓝) |
| Kimi | `#FF6B6B` (珊瑚) |
| Kilo | `#10B981` (翠绿) |
| Kiro | `#F59E0B` (琥珀) |
| OpenCode | `#6366F1` (靛蓝) |
| QoderCLI | `#8B5CF6` (紫罗兰) |
| Amp | `#EC4899` (粉) |
| AntiGravity | `#06B6D4` (青) |
| Droid | `#84CC16` (莱姆) |
| Hermes | `#F97316` (橙) |
| Unknown | `#6B7280` (灰) |

**机器缩写色：** 在配置文件中定义，每个机器缩写对应一个唯一颜色。
例如：`{ "abbr": "LCL", "color": "#4ADE80" }`

#### 工作状态色（全背景）

| 状态 | 背景色 |
|------|--------|
| DONE | `#27AE60` (绿) |
| IDLE | `#7F8C8D` (灰) |
| WORKING | `#F39C12` (琥珀) |
| BLOCKED | `#E74C3C` (红) |
| UNKNOWN | `#95A5A6` (灰) |

#### 聚焦态

focused=true 时，显示 3px 白色发光边框。

### 3.2 导航键（K11, K13）

**K11 — ALL 模式（全部机器）：**

```
┌──────────────┐
│              │
│    ALL       │  ← 大号 "ALL" 文字
│              │
└──────────────┘
```

- 白色文字，深色背景
- 点击后显示全部机器的 Agent

**K13 — 下一个 Machine/Space：**

```
┌──────────────┐
│              │
│  LCL  → PRD │  ← 左侧当前机器/space，右侧下一个
│              │
└──────────────┘
```

- 显示当前过滤条件和下一个选项

### 3.3 机器循环键（K12）

**K12 — 在单机间循环（整键背景=机器色）：**

```
┌──────────────┐
│ ▓▓▓ LCL ▓▓▓▓▓│  ← 整键背景 = 当前机器的颜色 (如 LCL=#4ADE80)
│              │
│   → DEV      │  ← 底部小字显示下一个机器
└──────────────┘
```

- 按键背景使用当前机器的定义色（来自 `abbrColor`）
- 空闲模式（ALL 模式）下背景为灰色
- 点击切换到下一台机器
- 通过背景色一眼识别当前选中哪台机器

### 3.4 全局统计条（K14）— 长条形按键

长条形可容纳一行横排信息：

```
┌──────────────────────────────────────────────┐
│  ✅3    ⏸2    ⏳4    ❌1    ❓0             │
└──────────────────────────────────────────────┘
```

- 每个状态一个图标 + 数字计数
- 统计所有 Workspace 下所有 Agent 的汇总
- 始终显示，不随翻页变化
- 背景深色半透明，图标彩色

**颜色方案：**

| 状态 | 图标颜色 |
|------|---------|
| ✅ Done | `#27AE60` |
| ⏸ Idle | `#7F8C8D` |
| ⏳ Working | `#F39C12` |
| ❌ Blocked | `#E74C3C` |
| ❓ Unknown | `#95A5A6` |

---

## 4. Agent 图标库（SVG 手绘）

需要手绘 18 个 Agent 的 SVG 图标（白色/单色，200×200 viewBox，适合叠加背景色）：

| # | Agent | id (manifest) | 图标设计 |
|---|-------|---------------|---------|
| 1 | Pi | `pi` | 希腊字母 π |
| 2 | Claude | `claude` | 咖啡杯 ☕ 简化线稿 |
| 3 | Cursor | `cursor` | 光标箭头 |
| 4 | Cline | `cline` | 山脉/折线 |
| 5 | Codex | `codex` | 卷轴/代码括号 |
| 6 | Gemini | `gemini` | 双子星/双菱形 |
| 7 | Copilot | `copilot` | 翅膀/翼 |
| 8 | Devin | `devin` | 螺丝刀/扳手 |
| 9 | Grok | `grok` | 眼睛/眼球 |
| 10 | Kimi | `kimi` | 月牙/弯月 |
| 11 | Kilo | `kilo` | 字母 K |
| 12 | Kiro | `kiro` | 闪电 ⚡ |
| 13 | OpenCode | `opencode` | 打开的花括号 `{` |
| 14 | QoderCLI | `qodercli` | 终端提示符 `>_` |
| 15 | Amp | `amp` | 安培/闪电+圆 |
| 16 | AntiGravity | `antigravity` | 反重力箭头 ↑ |
| 17 | Droid | `droid` | 机器人头 |
| 18 | Hermes | `hermes` | 信封/飞鸟 |

### 4.1 SVG 规格要求

```svg
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200" fill="#FFFFFF">
  <!-- 图标路径 -->
</svg>
```

- viewBox: 0 0 200 200
- 填充色: `#FFFFFF`（白色），渲染时桥接程序动态替换为背景色
- 线条宽度: 8-12px（在 200px 画布上清晰可见）
- 居中对齐

---

## 5. 工作流程

### 5.1 启动初始化流程

```
1. 桥接进程启动
2. 读取连接配置文件 `~/.config/herdr-deck/connections.json`
3. 为每个 `type: "ssh"` 的连接启动 SSH tunnel
   ├─ 执行 `ssh -L 0:<remoteSocket> <host> -N`
   ├─ 解析输出获取实际绑定的本地端口
   └─ 等待 tunnel 就绪
4. 遍历所有 connections:
   ├─ 建立 HerdrClient 连接
   │   ├─ local → Unix Socket (~/.local/share/herdr/herdr.sock)
   │   └─ ssh  → TCP localhost:<port>
   ├─ 发送: { id: "<conn>:workspace:list", method: "workspace.list", params: {} }
   ├─ 发送: { id: "<conn>:agent:list", method: "agent.list", params: {} }
   └─ 存入该连接对应的局部状态
5. 合并所有连接数据到统一状态树
   ├─ 每个 WS 带 connName/connAbbr 标识来源
   └─ 按配置顺序排列
6. 连接 UlanziDeck WebSocket (ws://127.0.0.1:3906)
   └─ 发送 { cmd: "connected", uuid: "com.ulanzi.herdr.agentview" }
7. 每个连接单独注册事件订阅
   └─ pane.agent_status_changed / pane.agent_detected / workspace.*
8. 首次渲染所有按键
   ├─ 计算分页：统一 WS 列表按 ≤5 Agent 拆分为 chunks
   ├─ 每页取 2 个 chunks（行1 + 行2）
   ├─ 确定当前页
   ├─ 渲染 K1-K5: 行1 WS 的 agents（带机器缩写）
   ├─ 渲染 K6-K10: 行2 WS 的 agents
   ├─ 渲染 K11-K13: 翻页导航
   └─ 渲染 K14: 全局统计
```

### 5.2 事件更新流程

```
收到 Herdr Event → PaneAgentStatusChanged:
  ├─ 确定该 event 来自哪个连接（connName）
  ├─ 更新该连接局部状态中的 AgentInfo.agent_status
  ├─ 合并到统一状态树 → 重新排序
  ├─ 按当前过滤模式（ALL/单机/单space）重新取前10
  ├─ 重新渲染 K1-K10 + 更新 K14 全局统计
  └─ 如果状态变为 Blocked → 发送 Toast 通知

收到 Herdr Event → WorkspaceFocused:
  ├─ 确定该 event 来自哪个连接
  ├─ 更新该连接的 WS 状态
  ├─ 重新排序并过滤
  ├─ 更新 K1-K10 + K11-K13
  └─ 重新渲染
```

### 5.3 按键交互流程

```
用户按下 Agent 键 (K1-K10):
  ├─ 获取该 Agent 的 pane_id
  ├─ 发送: { method: "agent.focus", params: { target: pane_id } }
  └─ Herdr 聚焦到该 Agent 的 pane

用户按下 K11 (ALL 模式):
  ├─ 切换到 ALL 模式
  ├─ 取消 machine 和 space 过滤
  ├─ 显示全部机器的 Agent（取前 10）
  └─ 更新全部按键

用户按下 K12 (机器循环):
  ├─ 切换到下一台机器的 Agent
  ├─ 清除 space 过滤
  ├─ 按新机器过滤 Agent → 取前 10
  └─ 更新全部按键

用户按下 K13 (Space 循环):
  ├─ 在当前机器内切换到下一个 Space
  ├─ 与当前机器过滤取交集
  ├─ 按交集过滤 Agent → 取前 10
  └─ 更新全部按键

用户旋转 Encoder 1:
  ├─ 获取当前 WS 的 Tab 列表
  ├─ 向左/右旋转 → 修改选中的 Tab index
  ├─ 更新 Encoder 1 LCD 显示当前 Tab 名
  └─ 点击按压 → tab.focus

用户旋转 Encoder 2:
  ├─ 在当前 Agent 列表上滚动（K1-K10 对应的 agents）
  ├─ 更新 Encoder 2 LCD 显示该 Agent 的完整状态
  │   Agent: pi (review)
  │   Status: WORKING
  │   Custom: reviewing file ../src/main.rs
  └─ 点击按压 → agent.focus
```

---

## 6. 内部数据结构

### 6.1 连接配置

```typescript
interface ConnectionConfig {
  name: string;       // 内部标识，日志用
  abbr: string;       // 缩写，显示在 K12（如 "LCL"、"DEV"）
  type: "local" | "ssh";
  host?: string;      // SSH 目标（匹配 ~/.ssh/config）
  remoteSocket?: string;  // 远端 socket 路径（仅 ssh）
}

interface AppConfig {
  connections: ConnectionConfig[];
}
```

### 6.2 状态树

```typescript
interface AppState {
  connections: ConnectionState[];
  unifiedWorkspaces: UnifiedWorkspace[];  // 合并后的 WS 有序列表
  currentPage: number;
  totalPages: number;
  agentStats: AgentStats;  // 全部 WS 的统计
}

interface ConnectionState {
  connName: string;
  connAbbr: string;
  status: "connecting" | "connected" | "disconnected" | "error";
  workspaces: WorkspaceInfo[];
  agents: Map<string, AgentInfo>;
  client: HerdrClient | null;
}

interface UnifiedWorkspace {
  connName: string;       // 来源机器
  connAbbr: string;       // 机器缩写
  workspace_id: string;
  label: string;
  number: number;
  agent_status: AgentStatus;
  tab_count: number;
  pane_count: number;
  agents: AgentInfo[];    // 该 WS 下的 agents
}

interface WorkspaceInfo {
  workspace_id: string;
  label: string;
  number: number;
  focused: boolean;
  agent_status: AgentStatus;
  tab_count: number;
  active_tab_id: string;
  pane_count: number;
}

interface AgentInfo {
  pane_id: string;
  terminal_id: string;
  workspace_id: string;
  tab_id: string;
  agent: string | null;       // detected agent type: "pi", "claude"...
  name: string | null;        // custom alias
  agent_status: AgentStatus;
  custom_status: string | null;
  state_labels: Record<string, string>;
  title: string | null;
  display_agent: string | null;
  focused: boolean;
  revision: number;
}

interface AgentStats {
  done: number;
  idle: number;
  working: number;
  blocked: number;
  unknown: number;
}

enum AgentStatus {
  Idle = "idle",
  Working = "working", 
  Blocked = "blocked",
  Done = "done",
  Unknown = "unknown",
}
```

### 6.2 按键状态映射

```typescript
interface ButtonState {
  keyId: string;        // "0_0", "1_2", "3_3" (row_col)
  type: ButtonType;     // agent | navPrev | navCurrent | navNext | stats
  content: ButtonContent;
}

interface ButtonContent {
  svgBase64: string;    // 渲染好的按键图像 (base64 SVG)
  // OR 分解字段:
  agentIcon?: string;   // Agent 类型图标 SVG path
  alias?: string;       // 自定义别名
  statusIcon?: string;  // 状态图标 (✅⏸⏳❌❓)
  statusColor?: string; // 状态对应颜色
}
```

---

## 7. 实现路径

### 阶段 1 — 核心桥接（优先级最高）

| 模块 | 文件 | 描述 |
|------|------|------|
| 连接管理器 | `src/connection-manager.js` | 配置读取、SSH tunnel 生命周期、连接池管理 |
| Herdr 客户端 | `src/herdr-client.js` | Unix socket/TCP 连接，请求/响应，事件订阅（每连接一个实例） |
| Deck 客户端 | `src/deck-client.js` | WebSocket 连接，按键渲染，事件接收 |
| 状态管理器 | `src/state-manager.js` | 多连接状态合并、统一状态树维护 |
| 按键映射器 | `src/button-mapper.js` | 分页计算、状态→按键布局映射 |
| 图标渲染器 | `src/icon-renderer.js` | SVG 合成（图标+机器缩写+状态） |
| 入口文件 | `src/index.js` | 启动、重连、生命周期 |

### 阶段 2 — 图标资产

| 文件 | 内容 |
|------|------|
| `assets/icons/pi.svg` | Pi Agent 图标 |
| `assets/icons/claude.svg` | Claude Agent 图标 |
| `assets/icons/cursor.svg` | Cursor Agent 图标 |
| `...` | 其余 15 个 Agent 图标 |
| `assets/icons/status-done.svg` | Done 状态图标 |
| `assets/icons/status-idle.svg` | Idle 状态图标 |
| `assets/icons/status-working.svg` | Working 状态图标 |
| `assets/icons/status-blocked.svg` | Blocked 状态图标 |
| `assets/icons/status-unknown.svg` | Unknown 状态图标 |

### 阶段 3 — 插件打包

| 文件 | 内容 |
|------|------|
| `manifest.json` | UlanziDeck 插件声明 |
| `plugin/app.js` | 主服务入口（Node.js） |
| `property-inspector/` | 配置页面（可选） |

### 阶段 4 — 高级功能

| 功能 | 描述 |
|------|------|
| Encoder 支持 | 旋钮交互逻辑 |
| 翻页 | 超过 10 个 Agent 时的分页 |
| Toast 通知 | Blocked Agent 自动告警 |
| 状态历史 | K14 统计 Mini 趋势 |

---

## 8. 分支决策树（已确认）

```
硬件设备
  └─ D200X → 14键 + 3 Encoder

连接方式
  ├─ local: 直连 Unix Socket
  ├─ ssh: 桥接 spawn `ssh -L` tunnel → 连 localhost:<port>
  ├─ 端口: OS 自动分配空闲端口
  └─ 认证: 依赖 ~/.ssh/config，桥接不管理密钥

显示方案
  └─ 方案 B: 图标 + 状态文字叠加 (英文)

键位分布
  ├─ K1-K10: 排序后的 Agent 状态键（取前 10）
  ├─ K11: ALL 模式（显示全部机器）
  ├─ K12: 机器循环（在单机间切换）
  ├─ K13: Space 循环（在当前机器内切换 Space）
  └─ K14: 全局统计栏 (长条, 始终固定)

过滤逻辑
  ├─ 三种模式：ALL（全部）/ Machine（单机）/ Space（单space）
  ├─ K12 和 K13 取交集
  ├─ K12 切换机器时清除 K13 的 space 过滤
  └─ K11 清除所有过滤

排序逻辑
  ├─ 第一层：状态优先级 BLOCKED(0) > DONE(1) > WORKING(2) > IDLE(3) > UNKNOWN(4)
  ├─ 第二层：同状态内按机器顺序分组
  ├─ K1-K10 仅显示前 10 个
  └─ 超过 10 个时截断

导航方式
  └─ K11/K12/K13 切换过滤模式（无翻页）

连接断开行为
  ├─ 单连接断开 → 仅该连接重连，显示其最后已知状态（标记为 stale）
  └─ 新连接接入 → 动态插入到 Unified WS 列表，重新计算分页

状态图标
  ├─ ✅ Done = 绿色
  ├─ ⏸ Idle = 灰色
  ├─ ⏳ Working = 琥珀色
  ├─ ❌ Blocked = 红色
  └─ ❓ Unknown = 灰色

K14 统计
  └─ 纯图标 + 数字横排 (所有 WS 汇总)

Encoder 分配
  ├─ E1: Tab 切换
  ├─ E2: Agent 详情浏览
  └─ E3: 翻页

布局层级
  └─ 行1(5键) + 行2(5键) + 行3(4键,最后1个长条)

图标方案
  └─ SVG 手绘 18 个 Agent + 5 个状态图标
```
