# herdr-panel — 桌面提醒面板

## 定位

herdr-panel 是 herdr-agentview 三进程架构中的桌面组件，替代原"小精灵/桌宠"方案。

**它不是仪表盘**，用户会在终端检查细节。它是一个**提醒器**——一眼告诉用户"需不需要去检查"，以及去哪检查。

## 产品形态

```
┌───────────────────────────────────┐
│ 2 blocked  1 working  5 idle     │
│      [ALL]  [ACT]                 │
│ M:[DEV ▾]  P:[api ▾]             │
├───────────────────────────────────┤
│ ┌──────┐ ┌──────┐ ┌──────┐       │
│ │ 🔴   │ │ 🔴   │ │ 🟡   │       │
│ │api-3 │ │e2e-1 │ │doc-2 │       │
│ │dev-sv│ │dev-sv│ │local │       │
│ │  5m  │ │  2m  │ │  3m  │       │
│ └──────┘ └──────┘ └──────┘       │
│ ┌──────┐ ┌──────┐ ┌──────┐       │
│ │ 🔵   │ │ 🔵   │ │ 🟢   │       │
│ │test-1│ │build │ │ui-1  │       │
│ │run-01│ │dev-sv│ │local │       │
│ │ 12m  │ │  8m  │ │  1h  │       │
│ └──────┘ └──────┘ └──────┘       │
└───────────────────────────────────┘
```

- 2×3 卡片网格
- 无滚动条
- 超出 6 个 agent 时按优先级取前 6（blocked > done > working > idle > unknown）
- 每张卡片：状态色块背景 + agent名 + 机器缩写 + 持续时间

## 需求汇总

- 跨平台（macOS / Windows / Linux）
- Fyne GUI（非游戏引擎）
- 普通窗口（非透明、非 overlay）
- 系统托盘常驻（关闭窗口→隐藏到托盘，右键菜单退出）
- 默认位置：屏幕右下角
- 记忆窗口位置和尺寸（持久化）
- 告警规则：
  - 全局设置，定义哪些 agent 状态变化时弹出窗口
  - 触发时：窗口自动弹出 + 对应卡片闪烁/高亮 + 托盘图标变色
- 可手动隐藏/关闭

## 导航控件（K11/K12/K13/K14）

| 硬件Key | 桌面控件 |
|---------|----------|
| K11 ALL/ACT | 并排按钮 [ALL] [ACT]，高亮当前模式 |
| K12 Machine | 下拉框 [DEV ▾] |
| K13 Space | 下拉框 [api ▾] |
| K14 Stats | 顶部状态条（色块+数字） |

## 技术选型

| 项 | 选择 | 原因 |
|----|------|------|
| GUI 框架 | **Fyne** | 纯 Go 跨平台、原生窗口、系统托盘支持 |
| 渲染 | Fyne 原生 | 不需要自定义 OpenGL/游戏引擎 |
| 窗口类型 | 普通窗口 | 不透明、可拖动、可缩放 |
| 系统托盘 | Fyne Desktop systray | v2.2.0+ 支持 macOS/Win/Linux |
| 主题 | Fyne 内置 DarkTheme | 无需自绘，暗色适合信息面板 |
| 高 DPI | Fyne 原生支持 | macOS Retina 自动适配 |
| 数据订阅 | NATS | 复用现有协议和 collector |

不使用：Ebitengine / raylib-go / GLFW / WebView

## 程序架构

```
protocol.FleetSnapshot
        │
        ▼
┌──────────────────────────────┐
│ displaymodel                  │  ← 共享模块，已有
│ Builder.Build()               │
└──────────┬───────────────────┘
           │
           ▼
┌──────────────────────────────┐
│ herdr-panel                   │
│                               │
│  subscriber/                  │  ← NATS snapshot + heartbeat
│    subscriber.go              │
│                               │
│  state/                       │
│    store.go                   │  ← 最新 snapshot + ViewState + health
│                               │
│  app/                         │
│    app.go                     │  ← Fyne 生命周期组装
│                               │
│  ui/                          │
│    main_window.go             │  ← 主窗口
│    toolbar.go                 │  ← K11 ALL/ACT + K12 dropdown + K13 dropdown
│    stats_bar.go               │  ← K14 统计条
│    card_grid.go               │  ← 2×3 Agent 卡片网格
│    tray.go                    │  ← 系统托盘菜单
│    theme.go                   │  ← 状态颜色、字体
│                               │
│  alert/                       │
│    rules.go                   │  ← 告警规则解析
│    monitor.go                 │  ← 状态变化检测 + 触发弹出
└──────────────────────────────┘
```

## 依赖方向

```
panel -> protocol
panel -> displaymodel
panel -> fyne/v2
panel -> fyne.io/systray

panel 不依赖 deck
panel 不依赖 collector/internal
```

## 模块结构

```
panel/
  go.mod
  cmd/
    herdr-panel/
      main.go
  internal/
    subscriber/
    state/
    app/
    ui/
    alert/
```

## 告警规则

```go
type AlertRule struct {
    // 哪些状态变化时需要弹出窗口
    WatchStatuses []protocol.AgentStatus
}
```

规则只检查 agent 状态变化，不按 machine/workspace 细分。

触发的行为：

1. 窗口从隐藏状态弹出到前台
2. 对应卡片闪烁/高亮
3. 托盘图标变色（由 collector health + alert 状态决定）

## 开发顺序

1. 新建 panel/ 模块（go.mod、cmd）
2. subscriber 从 NATS 接收 FleetSnapshot
3. state/store 持最新 snapshot + ViewState
4. displaymodel.Builder 集成（已有）
5. Fyne 主窗口骨架（静窗口、可拖动、可缩放、可关闭→隐藏）
6. 顶部 K11/K12/K13/K14 控件
7. 2×3 卡片网格（按优先级取前 6）
8. 系统托盘（显示/隐藏/退出）
9. 告警规则模块（状态变化检测 + 弹出）
10. 窗口位置/尺寸持久化
11. 系统托盘图标随状态变色
12. 窗口默认位置右下角
