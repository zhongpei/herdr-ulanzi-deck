# Herdr Panel macOS 原生自绘开发文档

## 1. 项目目标

本项目实现一个 macOS 原生信息面板，用于展示 Herdr / Claude / Agent 多进程状态。

它不是传统 GUI 应用，不使用按钮、下拉框、表格控件。所有 UI 都由程序自己绘制。

最终效果是一个菜单栏常驻的 Agent Fleet 信息仪表盘：

```text
╭────────────────────────────────────────────────────────────────────╮
│ HERDR FLEET                                      LIVE · 09:42:18    │
│ ●●●  🔴2 BLOCKED   🟡1 WAITING   🔵8 WORKING   🟢5 IDLE   ✅3 DONE  │
│      CPU 38%  ███████░░░   MEM 62%  ███████████░░░                 │
├────────────────────────────────────────────────────────────────────┤
│  FOCUS        MACHINE FLOW                         PROJECT FLOW     │
│  ACTIVE       ALL  LCL  DEV◉  PRD  RUN             ALL  api◉  web   │
│  show hot     ────●────○────○────○────             ────●────○────   │
├────────────────────────────────────────────────────────────────────┤
│  ATTENTION                                                           │
│  ╭──────────────╮ ╭──────────────╮ ╭──────────────╮                 │
│  │ 🔴 api-3      │ │ 🔴 e2e-1      │ │ 🟡 doc-2      │                 │
│  │ blocked      │ │ blocked      │ │ waiting      │                 │
│  │ DEV / iplist │ │ RUN / tests  │ │ LCL / docs   │                 │
│  │ 03m21s       │ │ 01m08s       │ │ 00m44s       │                 │
│  ╰──────────────╯ ╰──────────────╯ ╰──────────────╯                 │
├────────────────────────────────────────────────────────────────────┤
│  FLEET MATRIX                                                        │
│  DEV  │ 🔴 api-3      🔵 test-2     🔵 api-1      🟢 ui-1            │
│  LCL  │ 🟡 doc-2      ✅ doc-1      🟢 idle-4                        │
│  RUN  │ 🔴 e2e-1      🔵 build-2    ⚫ host-7                         │
│  PRD  │ 🟢 watch-1    🟢 monitor-2                                    │
├────────────────────────────────────────────────────────────────────┤
│  SELECTED  api-3 · claude · DEV / iplist-manager-v2                 │
│  blocked: waiting for permission · double click to focus             │
╰────────────────────────────────────────────────────────────────────╯
```

## 2. 总体架构

当前系统采用三进程架构：

```text
┌────────────────────┐
│ herdr-collector     │  Go
│ - 采集 Herdr 状态   │
│ - 维护 FleetSnapshot│
│ - embedded NATS     │
└─────────┬──────────┘
          │ NATS
          ▼
┌────────────────────┐
│ herdr-deck          │  Go
│ - 订阅 snapshot     │
│ - 渲染 D200X 硬件   │
└────────────────────┘

┌────────────────────┐
│ herdr-panel-mac     │  Swift / AppKit / 自绘
│ - 订阅 snapshot     │
│ - 菜单栏状态入口     │
│ - 原生自绘信息面板   │
└────────────────────┘
```

`herdr-panel-mac` 只做四件事：

```text
1. 连接 collector 内置 NATS
2. 订阅 FleetSnapshot
3. 把 FleetSnapshot 转成 DisplayModel
4. 自绘 Agent Fleet Board
```

它不做这些事情：

```text
不连接 Herdr
不建立 SSH tunnel
不连接 Ulanzi D200X
不 import collector/internal
不 import deck/internal
不生成 SVG
不生成 PNG
不使用传统 GUI 控件
```

## 3. 技术选型

### 3.1 macOS 原生壳

使用 AppKit：

```text
NSApplication
NSStatusItem
NSPanel
NSView
NSTimer / display refresh
```

职责：

```text
AppKit 只负责：
- 应用生命周期
- 菜单栏图标
- 弹出面板窗口
- 鼠标和键盘事件
- 自绘 View 的承载
```

### 3.2 自绘层

第一版建议使用：

```text
NSView + CoreGraphics + CoreText
```

原因：

```text
1. 对新手友好
2. 不需要一开始写 Metal shader
3. 足够画出圆角卡片、文字、线条、状态点、进度条
4. 便于调试
```

第二版再升级为：

```text
MTKView + Metal
```

只把最需要性能的 `Fleet Matrix` 或动画区域迁到 Metal。

不要第一天就上全 Metal，否则新手开发难度过高。

### 3.3 数据协议

macOS 面板使用 JSON 解码 `FleetSnapshot`。

协议字段以 Go 的 `protocol` 模块为准，Swift 侧复制一份结构体：

```swift
struct FleetSnapshot: Codable {
    let version: Int
    let seq: UInt64
    let updatedAt: String
    let machines: [MachineInfo]
    let agents: [AgentState]
    let stats: AgentStats
}
```

Swift 侧不要直接依赖 Go 源码。
协议边界就是 JSON。

## 4. 项目目录结构

建议新增独立目录：

```text
panel-mac/
  HerdrPanel.xcodeproj

  HerdrPanel/
    App/
      HerdrPanelApp.swift
      AppDelegate.swift
      StatusBarController.swift
      PanelWindowController.swift

    Protocol/
      FleetSnapshot.swift
      AgentState.swift
      AgentStatus.swift
      AgentStats.swift
      MachineInfo.swift
      Subjects.swift

    Data/
      NATSClient.swift
      SnapshotStore.swift
      CollectorHealth.swift
      MockSnapshotProvider.swift

    DisplayModel/
      ViewState.swift
      DisplayModel.swift
      DisplayModelBuilder.swift
      AgentSorter.swift
      StatsBuilder.swift

    Engine/
      BoardView.swift
      RenderContext.swift
      Theme.swift
      Rect.swift
      HitZone.swift
      HitTest.swift
      Animation.swift
      MotionValue.swift

    Scene/
      SceneNode.swift
      SceneBuilder.swift
      TopHealthScene.swift
      LensScene.swift
      AttentionScene.swift
      MatrixScene.swift
      DetailScene.swift

    Actions/
      BoardAction.swift
      ActionDispatcher.swift
      CommandPublisher.swift

    Resources/
      Assets.xcassets
      Fonts/
```

## 5. 模块职责

### 5.1 App 模块

负责 macOS 应用生命周期。

```text
AppDelegate.swift
  - 启动应用
  - 初始化 StatusBarController
  - 初始化 PanelWindowController
  - 初始化 SnapshotStore
  - 初始化 NATSClient
  - 处理退出

StatusBarController.swift
  - 创建菜单栏 item
  - 根据全局状态改变菜单栏图标
  - 点击菜单栏 item 时显示/隐藏面板

PanelWindowController.swift
  - 创建 NSPanel
  - 设置窗口大小
  - 设置窗口位置
  - 挂载 BoardView
```

### 5.2 Protocol 模块

只放协议结构体，不放业务逻辑。

```text
FleetSnapshot.swift
AgentState.swift
AgentStatus.swift
AgentStats.swift
MachineInfo.swift
Subjects.swift
```

示例：

```swift
enum AgentStatus: String, Codable {
    case idle
    case working
    case waitingUser = "waiting_user"
    case blocked
    case done
    case error
    case offline
    case stale
    case unknown
}
```

### 5.3 Data 模块

负责数据输入和状态缓存。

```text
NATSClient.swift
  - 连接 NATS
  - 订阅 herdr.v1.snapshot.full
  - 订阅 herdr.v1.collector.heartbeat
  - 收到 JSON 后解码为 FleetSnapshot

SnapshotStore.swift
  - 保存 latestSnapshot
  - 保存 collector 是否在线
  - 保存最新心跳时间
  - 通知 BoardView 重绘

CollectorHealth.swift
  - 维护 online/offline/stale 状态
```

### 5.4 DisplayModel 模块

负责把原始 snapshot 转成可绘制模型。

它不关心 AppKit，不关心绘图 API。

输入：

```text
FleetSnapshot
ViewState
CollectorHealth
```

输出：

```text
DisplayModel
```

核心结构：

```swift
struct ViewState {
    var focusLens: FocusLens
    var axis: BoardAxis
    var selectedMachine: String?
    var selectedProject: String?
    var selectedAgentID: String?
    var boardMode: BoardMode
}

enum FocusLens {
    case all
    case active
}

enum BoardAxis {
    case global
    case machine
    case project
}

enum BoardMode {
    case mini
    case fleet
    case incident
    case detail
}
```

`DisplayModel` 示例：

```swift
struct DisplayModel {
    let title: String
    let nowText: String
    let collectorHealth: CollectorHealthState
    let stats: StatsModel
    let lens: LensModel
    let machineFlow: FlowModel
    let projectFlow: FlowModel
    let attentionAgents: [AgentCardModel]
    let matrixRows: [MatrixRowModel]
    let selected: SelectedModel?
}
```

### 5.5 Engine 模块

负责自绘和交互。

```text
BoardView.swift
  - 继承 NSView
  - override draw(_ dirtyRect:)
  - override mouseDown
  - override mouseMoved
  - override scrollWheel
  - override keyDown

RenderContext.swift
  - 封装 CGContext
  - 提供 drawText / drawRoundedRect / drawLine / drawDot 等方法

Theme.swift
  - 颜色
  - 字体
  - 间距
  - 圆角
  - 状态色

HitZone.swift
  - 保存可点击区域
  - 每次绘制时生成
  - 鼠标事件根据 hit zone 转成 BoardAction

Animation.swift
  - 管理动效
  - blocked pulse
  - working breathe
  - card transition
```

### 5.6 Scene 模块

负责把 DisplayModel 布局成可绘制节点。

```text
SceneBuilder.swift
  - 输入 DisplayModel + panel size
  - 输出 SceneNode 列表

TopHealthScene.swift
  - 绘制顶部 K14 健康条

LensScene.swift
  - 绘制 K11/K12/K13 视野轨道

AttentionScene.swift
  - 绘制重点 Agent 卡片

MatrixScene.swift
  - 绘制 Fleet Matrix

DetailScene.swift
  - 绘制底部 Selected / Top Event
```

## 6. UI 信息结构

最终主面板分五层：

```text
╭──────────────────────────────────────────────╮
│ K14: Global Health / Stats                    │
├──────────────────────────────────────────────┤
│ K11 Focus Lens | K12 Machine Flow | K13 Space │
├──────────────────────────────────────────────┤
│ Attention: blocked / waiting / stale          │
├──────────────────────────────────────────────┤
│ Fleet Matrix: all visible agents              │
├──────────────────────────────────────────────┤
│ Selected Agent / Top Event                    │
╰──────────────────────────────────────────────╯
```

### 6.1 顶部 K14：Global Health

负责回答：

```text
现在有没有需要处理的事？
```

显示内容：

```text
🔴2 BLOCKED
🟡1 WAITING
🔵8 WORKING
🟢5 IDLE
✅3 DONE
⚫1 UNKNOWN
CPU 38%
MEM 62%
```

如果 `blocked > 0`：

```text
顶部红色边线
状态点轻微脉冲
菜单栏图标变红
```

如果 `waiting_user > 0` 且没有 blocked：

```text
顶部黄色边线
菜单栏图标变黄
```

如果 collector offline：

```text
顶部变灰
显示 COLLECTOR OFFLINE
```

### 6.2 K11：Focus Lens

K11 不画成按钮，画成视野状态：

```text
FOCUS
ACTIVE
show hot
```

两种状态：

```text
ALL
  显示全部 Agent

ACTIVE
  只显示需要关注的 Agent：
  - blocked
  - error
  - waiting_user
  - stale
  - offline
  - working
```

交互：

```text
点击 FOCUS 区域：ALL / ACTIVE 切换
按 A：ALL / ACTIVE 切换
```

### 6.3 K12：Machine Flow

K12 不画成下拉框，画成机器轨道：

```text
MACHINE FLOW
ALL  LCL  DEV◉  PRD  RUN
────●────○────○────○────
```

语义：

```text
当前选中某台机器时：
  只显示这台机器相关 Agent

切换机器时：
  清空 selectedProject
  axis = machine
```

交互：

```text
点击某个机器：选中机器
滚轮：上一台 / 下一台机器
按 M：下一台机器
Shift+M：上一台机器
双击机器轨道：回到 ALL machines
```

### 6.4 K13：Project Flow

K13 不画成下拉框，画成项目轨道：

```text
PROJECT FLOW
ALL  api◉  web  tests  docs
────●────○────○────○────
```

语义：

```text
Project 使用 workspace label 聚合
不是 workspace_id
不绑定单台机器

切换项目时：
  清空 selectedMachine
  axis = project
```

交互：

```text
点击某个项目：选中项目
滚轮：上一个 / 下一个项目
按 P：下一个项目
Shift+P：上一个项目
双击项目轨道：回到 ALL projects
```

### 6.5 Attention 区

只显示最重要的 Agent。

进入 Attention 的条件：

```text
blocked
error
waiting_user
offline
stale
working 超过阈值
```

排序规则：

```text
error
blocked
waiting_user
offline
stale
long_running_working
working
done
idle
unknown
```

卡片格式：

```text
╭──────────────╮
│ 🔴 api-3      │
│ blocked      │
│ DEV / iplist │
│ 03m21s       │
╰──────────────╯
```

### 6.6 Fleet Matrix

默认按机器分组：

```text
FLEET MATRIX
DEV  │ 🔴 api-3      🔵 test-2     🔵 api-1      🟢 ui-1
LCL  │ 🟡 doc-2      ✅ doc-1      🟢 idle-4
RUN  │ 🔴 e2e-1      🔵 build-2    ⚫ host-7
PRD  │ 🟢 watch-1    🟢 monitor-2
```

切到 project axis 时按项目分组：

```text
FLEET MATRIX · PROJECT VIEW
api      │ 🔴 api-3      🔵 api-1      🟢 api-doc
tests    │ 🔴 e2e-1      🔵 test-2     🔵 build-2
docs     │ 🟡 doc-2      ✅ doc-1
monitor  │ 🟢 watch-1    ⚫ host-7
```

### 6.7 底部 Selected / Top Event

有选中 Agent 时：

```text
SELECTED  api-3 · claude · DEV / iplist-manager-v2
blocked: waiting for permission · double click to focus
```

没有选中时：

```text
TOP EVENT  e2e-1 blocked for 01m08s · RUN / tests
```

## 7. 交互规则

### 7.1 鼠标

```text
点击 FOCUS 区        -> ALL / ACTIVE 切换
滚轮 FOCUS 区        -> ALL / ACTIVE 切换

点击 MACHINE FLOW    -> 选中机器
滚轮 MACHINE FLOW    -> 下一台 / 上一台机器
双击 MACHINE FLOW    -> 回到 ALL machines

点击 PROJECT FLOW    -> 选中项目
滚轮 PROJECT FLOW    -> 下一项目 / 上一项目
双击 PROJECT FLOW    -> 回到 ALL projects

点击 Agent 卡片      -> 选中 Agent
双击 Agent 卡片      -> focus agent
右键 Agent 卡片      -> copy info / pin / hide

点击顶部统计         -> 进入 Incident Mode
双击顶部统计         -> reset all filters
```

### 7.2 键盘

```text
A       ALL / ACTIVE
M       下一台机器
Shift+M 上一台机器
P       下一个项目
Shift+P 上一个项目
R       reset filters
Enter   focus selected agent
Esc     关闭详情 / 收起面板
/       搜索 agent/project/machine
1-9     快速选择 Attention Agent
```

## 8. Action 设计

所有输入事件最终都转换成 `BoardAction`。

```swift
enum BoardAction {
    case toggleFocusLens
    case selectMachine(String)
    case nextMachine
    case previousMachine
    case clearMachine

    case selectProject(String)
    case nextProject
    case previousProject
    case clearProject

    case selectAgent(String)
    case focusAgent(String)

    case resetFilters
    case enterIncidentMode
    case collapsePanel
}
```

`ActionDispatcher` 负责执行 action：

```text
toggleFocusLens:
  修改 ViewState.focusLens

selectMachine:
  selectedMachine = machine
  selectedProject = nil
  axis = .machine

selectProject:
  selectedProject = project
  selectedMachine = nil
  axis = .project

focusAgent:
  通过 NATS command subject 发送 focus 命令给 collector
```

## 9. Hit Test 设计

自绘 UI 没有按钮，所以必须自己维护点击区域。

每次绘制时生成 `HitZone`：

```swift
struct HitZone {
    let id: String
    let rect: CGRect
    let action: BoardAction
    let hoverCursor: CursorKind
}
```

绘制流程：

```text
1. 清空 hitZones
2. 绘制顶部统计，同时注册顶部统计 hit zone
3. 绘制 FOCUS，同时注册 FOCUS hit zone
4. 绘制 MACHINE FLOW，每个机器注册一个 hit zone
5. 绘制 PROJECT FLOW，每个项目注册一个 hit zone
6. 绘制 Agent 卡片，每个 Agent 注册一个 hit zone
7. mouseDown 时遍历 hitZones，找到命中的 action
```

## 10. 动画设计

不要使用真正物理引擎。
只实现简单 Motion Engine。

### 10.1 AnimatedValue

```swift
struct AnimatedValue {
    var current: CGFloat
    var target: CGFloat
    var velocity: CGFloat
    var stiffness: CGFloat
    var damping: CGFloat
}
```

用途：

```text
卡片位置过渡
卡片透明度变化
blocked 脉冲
working 呼吸动画
hover 放大 2%
```

### 10.2 每帧更新

```text
display tick
  -> update animations
  -> setNeedsDisplay()
```

第一版可以不用 60 FPS。
建议：

```text
普通状态：10 FPS
有动画/hover/状态变化：30 FPS
```

## 11. 渲染实现路线

### 第一阶段：CoreGraphics 自绘

`BoardView` 继承 `NSView`：

```swift
final class BoardView: NSView {
    private var displayModel: DisplayModel?
    private var hitZones: [HitZone] = []

    override var acceptsFirstResponder: Bool { true }

    override func draw(_ dirtyRect: NSRect) {
        guard let ctx = NSGraphicsContext.current?.cgContext else { return }
        let render = RenderContext(ctx: ctx, scale: window?.backingScaleFactor ?? 2.0)
        drawBoard(render)
    }

    override func mouseDown(with event: NSEvent) {
        let point = convert(event.locationInWindow, from: nil)
        dispatchHit(at: point, clickCount: event.clickCount)
    }

    override func scrollWheel(with event: NSEvent) {
        let point = convert(event.locationInWindow, from: nil)
        dispatchScroll(at: point, delta: event.scrollingDeltaY)
    }

    override func keyDown(with event: NSEvent) {
        dispatchKey(event)
    }
}
```

### 第二阶段：局部 Metal 化

当 CoreGraphics 版功能稳定后，把 `FleetMatrix` 替换为 `MTKView`：

```text
BoardView
  TopHealth: CoreGraphics
  Lens: CoreGraphics
  Attention: CoreGraphics
  FleetMatrix: Metal
  Detail: CoreGraphics
```

不建议第一版直接全 Metal。

## 12. NATS 数据接入

### 12.1 订阅 subjects

```text
herdr.v1.snapshot.full
herdr.v1.collector.heartbeat
```

### 12.2 Snapshot 接收流程

```text
NATSClient 收到 message
  -> JSONDecoder.decode(FleetSnapshot.self)
  -> SnapshotStore.apply(snapshot)
  -> DisplayModelBuilder.build()
  -> BoardView.update(model)
  -> BoardView.setNeedsDisplay()
```

### 12.3 Collector 离线判断

```text
每 1 秒检查 lastHeartbeatAt

如果超过 5 秒没有 heartbeat：
  collectorHealth = offline
  BoardView 变灰
  菜单栏 icon 变灰
```

## 13. Focus Agent 命令

双击 Agent 卡片时，不直接连接 Herdr，而是发 NATS command 给 collector。

Subject：

```text
herdr.v1.command.agent.focus
```

Payload：

```json
{
  "version": 1,
  "command_id": "uuid",
  "machine": "dev-server",
  "pane_id": "p8",
  "source": "panel-mac",
  "created_at": "2026-06-21T10:00:00Z"
}
```

collector 收到后调用 Herdr focus。

## 14. 菜单栏图标逻辑

菜单栏图标只表达最高严重等级。

优先级：

```text
collector offline -> gray
blocked/error     -> red
waiting_user      -> yellow
working           -> blue
normal            -> green
```

文字可选：

```text
H 🔴2
H 🟡1
H 🔵8
H ✓
```

第一版可以只用文字，不需要图标资源。

## 15. 开发步骤

### 第 0 步：准备环境

```text
1. 安装 Xcode
2. 创建 macOS App 项目
3. Language 选择 Swift
4. Interface 不依赖 SwiftUI 主界面
5. 使用 AppKit AppDelegate 生命周期
```

项目命名：

```text
HerdrPanel
```

### 第 1 步：创建空白菜单栏应用

目标：

```text
运行后菜单栏出现 H
点击 H 弹出空白面板
再次点击 H 隐藏面板
```

需要实现：

```text
AppDelegate
StatusBarController
PanelWindowController
```

验收：

```text
菜单栏有入口
Dock 不一定要显示
面板能弹出和关闭
```

### 第 2 步：创建 BoardView

目标：

```text
面板里显示自绘黑色背景和标题 HERDR FLEET
```

需要实现：

```text
BoardView
RenderContext
Theme
```

验收：

```text
无任何传统控件
所有内容由 draw(_:) 绘制
```

### 第 3 步：实现 MockSnapshot

目标：

```text
不用 NATS，也能看到完整 UI
```

需要实现：

```text
MockSnapshotProvider
FleetSnapshot
AgentState
AgentStats
DisplayModelBuilder
```

验收：

```text
能画出：
- 顶部统计
- FOCUS
- MACHINE FLOW
- PROJECT FLOW
- Attention 卡片
- Fleet Matrix
- Selected 底部详情
```

### 第 4 步：实现 HitZone

目标：

```text
点击不同区域能改变 ViewState
```

需要实现：

```text
HitZone
HitTest
ActionDispatcher
```

验收：

```text
点击 FOCUS：ALL/ACTIVE 切换
点击 Machine：切换机器
点击 Project：切换项目
点击 Agent：底部详情变化
```

### 第 5 步：接入 NATS

目标：

```text
从 collector 实时读取 FleetSnapshot
```

需要实现：

```text
NATSClient
SnapshotStore
CollectorHealth
```

验收：

```text
启动 collector 后 panel-mac 能显示真实 Agent 状态
关闭 collector 后 panel-mac 变灰并显示 offline
```

### 第 6 步：实现 focus agent 命令

目标：

```text
双击 Agent 卡片能请求 collector focus 对应 pane
```

需要实现：

```text
CommandPublisher
FocusAgentCommand
```

验收：

```text
双击卡片后 NATS 发出 command
collector 收到 command
对应 Herdr agent 被 focus
```

### 第 7 步：实现动画

目标：

```text
状态变化有轻量动画，但不影响可读性
```

需要实现：

```text
MotionValue
AnimationManager
```

验收：

```text
blocked 卡片轻微 pulse
working 状态点呼吸
hover 卡片轻微高亮
```

### 第 8 步：打包

目标：

```text
生成可双击运行的 macOS App
```

需要处理：

```text
Bundle ID
图标
签名
自动启动可选
配置 NATS 地址
```

## 16. 新手开发注意事项

### 16.1 不要一开始接真实 NATS

先用 mock 数据把 UI 画出来。
否则调试 UI 时会被数据链路干扰。

### 16.2 不要一开始写 Metal

第一版用 CoreGraphics 自绘。
只要结构设计正确，后续可以替换成 Metal。

### 16.3 不要做传统控件

不要使用：

```text
NSButton
NSComboBox
NSTableView
SwiftUI Picker
SwiftUI Table
```

本项目所有交互都应通过：

```text
HitZone
BoardAction
ViewState
```

### 16.4 不要让 View 直接处理业务逻辑

错误做法：

```text
BoardView 里直接过滤 agents
BoardView 里直接拼状态统计
BoardView 里直接判断 active agents
```

正确做法：

```text
SnapshotStore 保存数据
DisplayModelBuilder 生成模型
BoardView 只负责绘制模型
ActionDispatcher 只负责修改 ViewState
```

## 17. 最小可用版本定义

MVP 必须包含：

```text
1. 菜单栏入口
2. 弹出 NSPanel
3. 自绘 BoardView
4. MockSnapshot UI
5. NATS snapshot 订阅
6. K11 ALL/ACTIVE
7. K12 Machine Flow
8. K13 Project Flow
9. K14 Global Stats
10. Attention 卡片
11. Fleet Matrix
12. 点击 Agent 显示详情
```

MVP 不要求：

```text
Metal
复杂动画
搜索
右键菜单
自动启动
高级设置页
```

## 18. 最终验收标准

### 功能验收

```text
collector 启动后，panel-mac 能实时显示 Agent 状态
collector 关闭后，panel-mac 进入 offline 状态
K11 能切换 ALL/ACTIVE
K12 能按机器切换
K13 能按项目切换
K12/K13 互斥
K14 能显示状态统计
点击 Agent 能显示详情
双击 Agent 能发送 focus 命令
```

### 架构验收

```text
panel-mac 不依赖 collector/internal
panel-mac 不依赖 deck/internal
panel-mac 只依赖协议 JSON
UI 不使用传统控件
绘制逻辑和数据逻辑分离
HitZone 统一管理交互
```

### 视觉验收

```text
blocked 一眼可见
waiting_user 一眼可见
working 不抢过 blocked/waiting
idle/done 低亮显示
当前机器/项目过滤状态明确
顶部统计能 1 秒内看懂全局健康
```

## 19. 推荐开发顺序清单

给新手的实际任务顺序：

```text
任务 1：创建 macOS App 空项目
任务 2：做菜单栏 H 图标
任务 3：点击 H 弹出 NSPanel
任务 4：在 NSPanel 里挂载 BoardView
任务 5：BoardView 画黑色背景和标题
任务 6：写 Swift 版 FleetSnapshot 协议结构
任务 7：写 MockSnapshotProvider
任务 8：写 DisplayModelBuilder
任务 9：画顶部 K14 统计区
任务 10：画 K11/K12/K13 视野轨道
任务 11：画 Attention 卡片
任务 12：画 Fleet Matrix
任务 13：实现 HitZone 点击
任务 14：实现 ViewState 切换
任务 15：接 NATS snapshot
任务 16：接 heartbeat offline
任务 17：实现双击 focus command
任务 18：加简单动画
任务 19：优化字体、颜色、间距
任务 20：打包 app
```

## 20. 一句话总结

这个 macOS 面板不是 GUI 控件项目，而是一个原生自绘 Agent Fleet 仪表盘。

架构核心是：

```text
NATS Snapshot
  -> SnapshotStore
  -> DisplayModelBuilder
  -> SceneBuilder
  -> BoardView 自绘
  -> HitZone 交互
```

视觉核心是：

```text
K14 = 全局健康
K11 = 视野范围
K12 = 机器维度
K13 = 项目维度
K1-K10 = Attention + Fleet Matrix
```

开发策略是：

```text
先 CoreGraphics 自绘 MVP
再优化动画
最后再考虑 Metal
```

