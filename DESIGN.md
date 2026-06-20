# Herdr Agent Status on UlanziDeck — 设计方案

> 版本: v1.1 (同步代码)
> 设备: Ulanzi D200X

---

## 1. 系统架构

```
Herdr Server (local)          Herdr Server (remote via SSH)
Unix Socket                     Unix Socket
       │                              │
       │ JSON-line API                │ SSH -L (Unix socket forward)
       ▼                              ▼
┌──────────────────────────────────────────────┐
│  connection-manager.js                        │
│  ├─ local: 直连 Unix Socket                  │
│  └─ ssh:   spawn ssh -L <localSock>:<remote> │
├──────────────────────────────────────────────┤
│  herdr-client.js     (每个连接一个实例)        │
│  herdr-bridge.js     (合并多机数据)           │
│  state-manager.js    (统一状态树 + 排序过滤)   │
│  button-mapper.js    (过滤模式 + 14键映射)     │
│  icon-renderer.js    (SVG→PNG 渲染)          │
├──────────────────────────────────────────────┤
│  deck-client.js      (WebSocket → D200X)     │
└────────────────────┬─────────────────────────┘
                     │ state 命令 (PNG base64)
                     ▼
              UlanziDeck 上位机 (port 3906)
                     │
                     ▼
              D200X 14× LCD 按键
```

### 1.1 连接配置

`~/.config/herdr-deck/connections.json`：

```json
{
  "connections": [
    { "name": "local",      "abbr": "LCL", "color": "#4ADE80", "type": "local" },
    { "name": "dev-server", "abbr": "DEV", "color": "#1E3A5F", "type": "ssh",
      "host": "user@host", "remoteSocket": "/home/user/.config/herdr/herdr.sock" }
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 内部标识 |
| `abbr` | string | 是 | 缩写，显示在 K12 按键上 |
| `color` | string | 是 | 机器色，K12 按钮背景色 |
| `type` | `"local"` \| `"ssh"` | 是 | 连接类型 |
| `host` | string | 仅 ssh | SSH 目标（匹配 `~/.ssh/config`） |
| `remoteSocket` | string | 仅 ssh | 远端 herdr socket 绝对路径 |

### 1.2 接口协议

| 连接 | 协议 |
|------|------|
| Herdr ↔ 桥接 | Unix Socket JSON-line (每个请求独立连接) |
| Herdr ↔ 桥接(SSH) | `ssh -L <localUnixSocket>:<remoteSocket>` 转发 |
| 桥接 ↔ UlanziDeck | WebSocket JSON (port 3906) |

### 1.3 SSH 隧道

- `ssh -L /tmp/herdr-<name>.sock:<remoteSocket> <host> -N`
- 本地 Unix socket → 远端 Unix socket 直通
- SSH 认证依赖 `~/.ssh/config`

---

## 2. D200X 物理按键布局

```
       col0   col1   col2   col3   col4
row0:  K1     K2     K3     K4     K5      ← Agents (K1-K10)
row1:  K6     K7     K8     K9     K10
row2:  K11    K12    K13    [K14  wide]    ← 过滤 + 统计
```

### 2.1 按键功能分配

| 键位 | 功能 | 说明 |
|------|------|------|
| K1-K10 | Agent 状态键 | 按状态优先级排序显示前 10 个 Agent。BLOCKED > DONE > WORKING > IDLE > UNKNOWN |
| K11 | ALL 模式 | 切换到 ALL 模式，显示全部机器的 Agent |
| K12 | 机器循环 | 切换到下一台机器的 Agent（LCL→DEV→PRD→LCL...），清除 space 过滤 |
| K13 | Space 循环 | 在当前机器内切换到下一个 Space（取交集）。ALL 模式下无效 |
| K14 | 全局统计栏 | 跨所有机器所有 Agent 的全局统计 |

### 2.2 Encoder 旋钮

（预留，未实现）

---

## 3. 按键视觉设计

### 3.1 Agent 键（K1-K10）

```
┌──────────────────────┐
│ ▓▓▓▓ PI ▓▓▓  ▓▓ LCL ▓│  ← 48px 顶栏: 左=Agent品牌色+名, 右=机器色+缩写
│▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁│  ← 1px 白分隔线
│                      │
│       review         │  ← 别名 36px BOLD (白色)
│                      │
│          W           │  ← 状态首字母 20px BOLD (白色)
│      main-proj       │  ← WS名称 26px BOLD (白色)
│                      │
└──────────────────────┘
  ↑ 背景 = 工作状态色 + 黑遮罩 0.15
```

| 区域 | 内容 | 字号 |
|------|------|------|
| 顶栏左半 | Agent 品牌色背景 + Agent 名 | 24px BOLD 白 |
| 顶栏右半 | 机器缩写色背景 + 机器缩写 | 24px BOLD 白 |
| 分隔线 | y=48, 1px 白色, opacity=0.25 | - |
| 别名 | 用户自定义别名 (agent.name) | **36px BOLD 白** |
| 状态字母 | D / I / W / B / ? | 20px BOLD 白 |
| WS 名 | 当前 Workspace label | **26px BOLD 白** |
| 聚焦边框 | 3px 白色发光边框 | focused=true 时显示 |

**Agent 品牌色表：**

| Agent | 色值 | Agent | 色值 |
|-------|------|-------|------|
| Pi | `#7C3AED` 紫 | Claude | `#D97757` 暖棕 |
| Cursor | `#00B884` 青绿 | Cline | `#2563EB` 蓝 |
| Codex | `#1E293B` 暗灰 | Gemini | `#4285F4` Google蓝 |
| Copilot | `#8957E5` 紫 | Devin | `#FF6B35` 橙 |
| Grok | `#1DA1F2` 蓝 | Kimi | `#FF6B6B` 珊瑚 |
| Kilo | `#10B981` 翠绿 | Kiro | `#F59E0B` 琥珀 |
| OpenCode | `#6366F1` 靛蓝 | QoderCLI | `#8B5CF6` 紫罗兰 |
| Amp | `#EC4899` 粉 | AntiGravity | `#06B6D4` 青 |
| Droid | `#84CC16` 莱姆 | Hermes | `#F97316` 橙 |
| Unknown | `#6B7280` 灰 | | |

**工作状态色（全背景）：**

| 状态 | 背景色 |
|------|--------|
| DONE | `#27AE60` 绿 |
| IDLE | `#7F8C8D` 灰 |
| WORKING | `#F39C12` 琥珀 |
| BLOCKED | `#E74C3C` 红 |
| UNKNOWN | `#95A5A6` 灰 |

### 3.2 导航键（K11-K13）

**K11 — ALL 模式：**

```
┌──────────┐
│          │
│   ALL    │  ← 36px BOLD 白字 + 蓝底 (active)
│          │    灰底 (inactive)
└──────────┘
```

**K12 — 机器循环：**

```
┌──────────┐
│ ▓▓ LCL ▓▓│  ← 背景 = 机器色 (LCL=绿, DEV=深蓝)
│  → DEV   │  ← 底部箭头 + 下一个机器缩写
└──────────┘
```

**K13 — Space 循环：**

```
┌──────────┐
│   MAIN   │  ← Space 名 28px BOLD 大写, 自动换行
│   PROJ   │  (在 "-" / "_" / "." 处分行)
│    WS    │  ← 小字提示
└──────────┘
```

### 3.3 全局统计（K14 — 宽键）

```
┌──────────────────────────────────┐
│                    D3 I2 W4 B1 ?0│  ← 右下角紧凑, 颜色=状态色
└──────────────────────────────────┘
```

---

## 4. 过滤逻辑

### 三种模式

| 模式 | K11(ALL) | K12(机器) | K13(Space) | 显示内容 |
|------|----------|-----------|------------|---------|
| ALL | 蓝色高亮 | 灰色 | 灰色 | 全部机器排序前10 |
| Machine | - | 背景=机器色 | 灰色 | 该机器排序前10 |
| Space | - | 背景=机器色 | 显示Space名 | 该机器+该space前10 |

### 排序

1. 第一层：状态优先级 `BLOCKED(0) > DONE(1) > WORKING(2) > IDLE(3) > UNKNOWN(4)`
2. 第二层：同状态内按机器顺序分组
3. K1-K10 取前 10 个，超出截断

---

## 5. 数据流

```
启动:
  1. 读 config → connections.json
  2. connection-manager.startAll()
     ├─ local: 找本地 Unix socket → herdr-client
     └─ ssh:   ssh -L tunnel → 找本地转发的 socket → herdr-client
  3. herdr-bridge.fetchAll()
     ├─ 每个连接: listWorkspaces() + listAgents()
     └─ 合并 → UnifiedWorkspace[]
  4. 创建 D200X Profile (profile-manager)
  5. 连接 UlanziDeck WebSocket
  6. 渲染全部 14 键

按键交互:
  K1-K10  →  agent.focus(connName, paneId)
  K11     →  setAll()
  K12     →  nextMachine()
  K13     →  nextSpace()
```

---

## 6. 文件清单

| 文件 | 说明 |
|------|------|
| `src/index.js` | 入口：生命周期、事件路由 |
| `src/config.js` | 读 `~/.config/herdr-deck/connections.json` |
| `src/connection-manager.js` | SSH tunnel + 多连接管理 |
| `src/herdr-client.js` | Unix Socket JSON-line 客户端 |
| `src/herdr-bridge.js` | 多连接数据合并 → UnifiedWorkspace |
| `src/state-manager.js` | 状态树 + 排序过滤 |
| `src/button-mapper.js` | 过滤模式 + 14 键映射 |
| `src/icon-renderer.js` | SVG 合成 → PNG |
| `src/deck-client.js` | WebSocket → D200X |
| `src/profile-manager.js` | 自动创建 D200X Profile |
| `src/mock-data.js` | 测试用 mock（fallback） |
| `scripts/deploy-and-run.sh` | 一键部署 |
| `manifest.json` | UlanziDeck 插件声明 |
| `tests/filter-buttons.test.js` | 过滤按钮单元测试 |
| `AGENTS.md` | 开发规则 |
