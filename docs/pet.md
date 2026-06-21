你这个方向是对的，但这里要注意一个边界：**小精灵不应该复用 `deck/internal/viewmodel` 的包，也不应该重新复制一套 K11/K12/K13/K14 逻辑。**

最佳方案是：**把 K11/K12/K13/K14 的“展示语义”抽成一个轻量共享模块，deck 和 pet 都依赖它；deck 负责把它渲染成 14 个硬件 key，小精灵负责把它渲染成桌面矩阵和控件。**

---

# 1. 现有代码里已经有的语义

你现在的 deck 侧已经把 K11-K14 做得比较清楚：

```text
K1-K10  = agent / empty
K11     = NavAll
K12     = NavMachine
K13     = NavSpace
K14     = Stats
```

`viewmodel` 里已经有 `NavAllData`、`NavMachineData`、`NavSpaceData`、`StatsData` 这些结构。K11 包含 `Label / Active / Filtered / CPU / Memory`；K12 包含当前机器缩写、当前机器颜色、下一个机器缩写；K13 包含空间数量、当前空间、下一个空间；K14 包含 `AgentStats` 和 CPU/MEM。

你的测试也已经覆盖了几个关键行为：

```text
1. 空 fleet 时 K11-K14 仍然存在
2. K12 和 K13 互相独立
3. K12 进入 machine mode 会清空 space
4. K13 进入 space mode 会清空 machine
5. K13 使用全局 space label
6. K14 走 AgentStats + CPU/MEM pipeline
```

这些测试说明：**硬件端的导航语义已经成型，可以作为小精灵的行为标准。**

---

# 2. 现在不能直接复用 deck/internal/viewmodel

当前 `deck/internal/viewmodel/builder.go` 是 deck 子项目内部包。Go 的 `internal` 机制决定了 pet 子项目不能直接 import 它。更重要的是，即使技术上能绕过，也不应该让 pet 依赖 deck 的内部实现。

否则依赖会变成：

```text
pet -> deck/internal/viewmodel
```

这会破坏你三进程拆分的初衷。

正确依赖应该是：

```text
deck -> displaymodel
pet  -> displaymodel
```

而不是：

```text
pet -> deck
deck -> pet
```

---

# 3. 推荐新增一个轻量共享模块：displaymodel

你现在已经有：

```text
protocol/
collector/
deck/
pet/        # 计划新增
```

我建议再加一个非常小的共享模块：

```text
displaymodel/
```

它不是进程，只是共享展示语义库。

最终结构：

```text
herdr-agentview/
  go.work

  protocol/
    snapshot.go
    status.go
    subjects.go
    version.go

  displaymodel/
    go.mod
    state.go
    index.go
    builder.go
    nav.go
    stats.go
    sort.go
    model.go

  collector/
    ...

  deck/
    ...

  pet/
    ...
```

依赖方向：

```text
collector -> protocol

displaymodel -> protocol

deck -> protocol
deck -> displaymodel

pet -> protocol
pet -> displaymodel
```

禁止：

```text
pet -> deck
deck -> pet
displaymodel -> deck
displaymodel -> pet
collector -> displaymodel
```

---

# 4. displaymodel 应该负责什么

它只负责 **“视图状态 + 过滤 + 导航语义 + 统计模型”**。

不要负责：

```text
SVG
PNG
Ebitengine
UlanziDeck
NATS
窗口
鼠标事件
硬件 key mapping
```

它的输入是：

```go
protocol.FleetSnapshot
```

输出是一个通用展示模型：

```go
DisplayModel
```

例如：

```go
type DisplayModel struct {
	Mode ViewMode `json:"mode"`

	Agents []AgentCell `json:"agents"`

	NavAll     NavAllModel     `json:"nav_all"`
	NavMachine NavMachineModel `json:"nav_machine"`
	NavSpace   NavSpaceModel   `json:"nav_space"`
	Stats      StatsModel      `json:"stats"`

	Highlight *AgentCell `json:"highlight,omitempty"`
}
```

这样 deck 和 pet 都能用同一份语义。

---

# 5. K11/K12/K13/K14 在小精灵里的对应关系

## K11：ALL / ACT

硬件上 K11 是 `ALL` / `ACT`，并且有 `Filtered` 状态。你现在的 `NavAllData` 里已经有 `Label`、`Active`、`Filtered`。

小精灵里不要做成硬件按钮，而是做成顶部筛选胶囊：

```text
[ ALL ] / [ ACTIVE ]
```

语义：

```text
ALL:
  显示所有 Agent

ACTIVE:
  只显示需要关注的 Agent
  blocked / waiting_user / working / done
```

你当前硬件代码里 ACTIVE 过滤主要偏向 blocked/working/done，后续小精灵建议把 `waiting_user` 也纳入 ACTIVE，因为这是你桌面提醒的核心状态。

显示形态：

```text
┌────────────────────────────────────────────┐
│ Claude Fleet     [ACT]  🔴2 🟡1 🔵8 🟢5     │
└────────────────────────────────────────────┘
```

操作：

```text
点击 ALL/ACT 胶囊
  -> ToggleActiveOnly()
  -> 重建 DisplayModel
```

---

## K12：机器切换

硬件上 K12 是 machine cycle：显示当前机器、颜色、下一个机器。`NavMachineData` 里已经有 `CurrentAbbr / CurrentColor / NextAbbr / Active`。

小精灵里建议做成机器筛选器：

```text
[ ALL MACHINES ]  [ LCL ] [ DEV ] [ PRD ]
```

但为了复刻 K12 的行为，也要支持“点击一次切到下一台机器”：

```text
点击机器区域：
  ALL -> 第一台机器
  LCL -> DEV
  DEV -> PRD
  PRD -> LCL
```

语义：

```text
NextMachine():
  mode = Machine
  selectedMachine = next
  selectedSpace = ""
```

这个和你现有测试一致：K12 从 space mode 回到 machine mode 时，会清空 `WsLabel`。

显示形态：

```text
Machine: DEV    next PRD
```

或者更适合桌面小精灵：

```text
机器: [DEV]  > PRD
```

---

## K13：项目 / Space 切换

硬件上 K13 是 global space cycle，不再限定当前机器；测试里也明确 K13 的 space 是全局 label，且 K12/K13 互斥。

小精灵里建议做成项目筛选器：

```text
Project: [iplist-manager-v2]  > agentbus
```

语义：

```text
NextSpace():
  mode = Space
  selectedSpace = next global workspace label
  selectedMachine = ""
```

关键点：**K13 用 label，不用 workspace_id。**

因为同一个项目可能在不同机器上运行，workspace UUID 可能不同，但 label 相同。这一点对桌面小精灵非常重要：你想看的通常是“某个项目整体状态”，不是某台机器上的某个 workspace UUID。

---

## K14：统计信息

硬件上 K14 是 stats bar，当前 `StatsData` 包含：

```text
done
idle
working
blocked
unknown
CPUPercent
MemoryPercent
```

并且测试已经验证 K14 和 K11 都能拿到 CPU/MEM。

小精灵里 K14 不应该只是一个小条，而应该成为顶部摘要：

```text
🔴2 blocked   🟡1 waiting   🔵8 working   🟢5 idle   ✅3 done   ⚫1 unknown
CPU 48%   MEM 62%
```

建议拆成两类统计：

```text
Fleet stats:
  Agent 状态数量，来自 protocol.FleetSnapshot.Stats

Local display stats:
  CPU / MEM，来自 pet 进程本机 sysstats
```

如果你希望显示 collector 所在机器的 CPU/MEM，那就应该扩展 protocol，由 collector 发布自己的 system stats。否则默认应该显示 **当前显示端本机资源**。这和 deck 当前逻辑更接近，因为 deck 的 CPU/MEM 是显示进程本地采集后塞入 viewmodel 的。

---

# 6. 小精灵程序的推荐架构

新增：

```text
pet/
  go.mod
  cmd/
    herdr-pet/
      main.go

  internal/
    subscriber/
      subscriber.go       # 订阅 NATS snapshot / heartbeat

    runtime/
      app.go              # 进程生命周期，组装 subscriber + displaymodel + game

    state/
      store.go            # 保存最新 snapshot、view state、health、dirty flag

    view/
      adapter.go          # displaymodel -> pet UI model
      layout.go           # Agent matrix layout
      theme.go            # 状态颜色、优先级、图标选择

    game/
      game.go             # Ebitengine Game
      input.go            # 点击 K11/K12/K13 区域、拖动、穿透切换
      render.go           # 绘制背景、矩阵、小精灵、统计
      assets.go           # PNG / spritesheet 加载
      animation.go        # 小精灵动画状态机

    sysstats/
      sysstats.go         # 可直接复制 deck/internal/sysstats 或抽共享
```

如果想更干净，可以加：

```text
pet/internal/model/
  model.go                # PetModel / AgentCell / Rect
```

---

# 7. 小精灵内部数据流

```text
NATS Snapshot
    │
    ▼
subscriber
    │
    ▼
PetStore.ApplySnapshot(snapshot)
    │
    ▼
displaymodel.Build(snapshot, viewState, localStats)
    │
    ▼
petview.Adapt(DisplayModel)
    │
    ▼
Ebitengine Update()
    │
    ▼
Ebitengine Draw()
```

完整图：

```text
┌─────────────────────────────────────────────┐
│ herdr-pet                                   │
│                                             │
│  NATS Subscriber                            │
│    - snapshot                               │
│    - heartbeat                              │
│                                             │
│  PetStore                                   │
│    - latest FleetSnapshot                   │
│    - ViewState                              │
│    - collector health                       │
│    - local CPU/MEM                          │
│                                             │
│  displaymodel.Builder                       │
│    - K11 ALL/ACT semantics                  │
│    - K12 machine cycle                      │
│    - K13 global space cycle                 │
│    - K14 stats model                        │
│                                             │
│  PetView Adapter                            │
│    - Agent matrix                           │
│    - top summary                            │
│    - highlight event                        │
│    - clickable regions                      │
│                                             │
│  Ebitengine Game                            │
│    - transparent overlay                    │
│    - sprite animation                       │
│    - draw matrix                            │
│    - handle clicks                          │
└─────────────────────────────────────────────┘
```

---

# 8. displaymodel 的核心结构

建议把当前 deck 的 `FilterMode`、`NavAllData`、`NavMachineData`、`NavSpaceData`、`StatsData` 升级成通用模型。

```go
package displaymodel

type ViewMode int

const (
	ModeAll ViewMode = iota
	ModeMachine
	ModeSpace
)

type ViewState struct {
	Mode            ViewMode
	SelectedMachine string
	SelectedSpace   string
	ActiveOnly      bool
}

type LocalStats struct {
	CPUPercent    float64
	MemoryPercent float64
}

type Model struct {
	Mode ViewMode

	Agents []AgentCell

	NavAll     NavAll
	NavMachine NavMachine
	NavSpace   NavSpace
	Stats      Stats

	Highlight *AgentCell
}

type NavAll struct {
	Label    string
	Active   bool
	Filtered bool
}

type NavMachine struct {
	CurrentName  string
	CurrentAbbr  string
	CurrentColor string
	NextName     string
	NextAbbr     string
	Active       bool
}

type NavSpace struct {
	Count        int
	CurrentLabel string
	NextLabel    string
	Active       bool
}

type Stats struct {
	AgentStats    protocol.AgentStats
	CPUPercent    float64
	MemoryPercent float64
}

type AgentCell struct {
	ID        string
	Machine   string
	Agent     string
	Name      string
	Status    protocol.AgentStatus
	Workspace string
	PaneID    string
	Focused   bool
}
```

---

# 9. displaymodel 的行为接口

```go
type Builder struct {
	state ViewState
}

func NewBuilder() *Builder

func (b *Builder) State() ViewState
func (b *Builder) SetState(s ViewState)

func (b *Builder) SetAll()
func (b *Builder) ToggleActiveOnly()
func (b *Builder) NextMachine(snapshot *protocol.FleetSnapshot)
func (b *Builder) NextSpace(snapshot *protocol.FleetSnapshot)

func (b *Builder) Build(
	snapshot *protocol.FleetSnapshot,
	local LocalStats,
) Model
```

这样 deck 和 pet 的事件处理都变得一致。

deck 里：

```go
case K11:
	builder.SetAll()
	builder.ToggleActiveOnly()

case K12:
	builder.NextMachine(snapshot)

case K13:
	builder.NextSpace(snapshot)
```

pet 里：

```go
case clickNavAll:
	builder.SetAll()
	builder.ToggleActiveOnly()

case clickMachine:
	builder.NextMachine(snapshot)

case clickSpace:
	builder.NextSpace(snapshot)
```

同一份语义，不会分叉。

---

# 10. 小精灵 UI 推荐布局

默认小精灵应该是：

```text
┌────────────────────────────────────────────────────┐
│ 🐾 Herdr Pet   [ACT]  M: DEV > PRD  P: api > web   │
│ 🔴2  🟡1  🔵8  🟢5  ✅3  ⚫1   CPU 48% MEM 62%       │
├────────────────────────────────────────────────────┤
│ 🔴 api-3   🔴 e2e-1   🟡 doc-2   🔵 api-1           │
│ 🔵 test-2  🟢 ui-1    ✅ doc-1   ⚫ host-7           │
├────────────────────────────────────────────────────┤
│ api-3 blocked · waiting for user confirmation       │
└────────────────────────────────────────────────────┘
```

对应关系：

```text
顶部第一行：
  K11 + K12 + K13

顶部第二行：
  K14 stats

中间：
  K1-K10 的扩展版 Agent Matrix

底部：
  最高优先级事件 / 当前选中 Agent
```

也就是说，小精灵不是原样画 14 个按键，而是 **复刻 14 个按键的语义**。

---

# 11. 小精灵的三种显示模式

## 模式 1：Mini

```text
🐾 🔴2 🟡1 🔵8 [ACT]
```

只显示 K14 摘要 + K11 当前过滤状态。

## 模式 2：Matrix

```text
[ACT]  M:DEV>P  P:api>web
🔴2 🟡1 🔵8 🟢5
🔴 api-3  🟡 doc-2  🔵 test-1  🟢 ui-1
```

这是默认模式。

## 模式 3：Detail

点击某个 agent：

```text
Agent: claude / test-fail
Machine: dev-server
Project: iplist-manager-v2
Status: blocked
Updated: 09:42:18
Message: waiting for permission
```

---

# 12. PetStore 不要承担过滤逻辑

小精灵内部建议这样分工：

```text
PetStore:
  保存最新 snapshot
  保存 ViewState
  保存 collector health
  保存 local sysstats

displaymodel:
  根据 snapshot + ViewState 计算过滤后的 agents、K11/K12/K13/K14

layout:
  根据窗口大小计算每个 cell 的矩形

game:
  接收鼠标点击，修改 ViewState
  Draw 当前 model
```

不要让 `game.Draw()` 里面现场过滤 agents。
不要让 `subscriber` 里面处理 K11/K12/K13。

---

# 13. 和 deck 现有代码的关系

现有 deck 可以逐步改成：

```text
deck/internal/viewmodel
  ↓
displaymodel
```

然后 deck 侧保留一个适配层：

```text
deck/internal/viewmodel/
  adapter.go
```

职责：

```text
displaymodel.Model
  -> []deck KeyCommand
```

也就是说：

```text
displaymodel 负责语义
deck viewmodel 负责 14-key 物理投影
pet viewmodel 负责桌面布局投影
```

数据流：

```text
protocol.FleetSnapshot
      │
      ▼
displaymodel.Model
      │
      ├── deck adapter -> 14 KeyCommand -> SVG -> PNG -> D200X
      │
      └── pet adapter  -> Agent Matrix -> Ebitengine
```

---

# 14. 最终推荐架构图

```text
protocol.FleetSnapshot
        │
        ▼
┌──────────────────────────┐
│ displaymodel              │
│                          │
│ ViewState                 │
│ - ModeAll                 │
│ - ModeMachine             │
│ - ModeSpace               │
│ - ActiveOnly              │
│                          │
│ Build()                   │
│ - filter agents           │
│ - sort agents             │
│ - build K11 semantics     │
│ - build K12 semantics     │
│ - build K13 semantics     │
│ - build K14 semantics     │
└────────────┬─────────────┘
             │
     ┌───────┴────────┐
     ▼                ▼
┌──────────────┐  ┌──────────────────┐
│ herdr-deck    │  │ herdr-pet         │
│              │  │                  │
│ DeckAdapter   │  │ PetAdapter        │
│ 14 KeyCommand │  │ AgentMatrix       │
│ SVG Renderer  │  │ Ebitengine Draw   │
│ D200X         │  │ Desktop Overlay   │
└──────────────┘  └──────────────────┘
```

---

# 15. 开发顺序

建议这样做：

```text
1. 新建 displaymodel 模块
2. 从 deck/internal/viewmodel 抽出：
   - FilterMode
   - ViewState
   - SetAll
   - NextMachine
   - NextSpace
   - ToggleActiveOnly
   - filter/sort agents
   - NavAll/NavMachine/NavSpace/Stats model

3. deck/internal/viewmodel 改成 adapter：
   displaymodel.Model -> []KeyCommand

4. 保证 deck 所有测试通过

5. 新建 pet 项目：
   - subscriber
   - PetStore
   - sysstats
   - displaymodel.Builder
   - Ebitengine Game

6. pet 第一版只做静态矩阵，不做复杂动画

7. pet 第二版再加小精灵动画、透明置顶、鼠标穿透
```

---

# 最终判断

小精灵应该复刻的是 **K11/K12/K13/K14 的交互语义**，不是复刻 14 个硬件按钮的外观。

最终设计应是：

```text
K11 -> ALL / ACTIVE 筛选胶囊
K12 -> Machine cycle 控件
K13 -> Project / Space cycle 控件
K14 -> 顶部状态统计栏
K1-K10 -> Agent 状态矩阵
```

而程序架构上，最重要的是新增：

```text
displaymodel/
```

让 deck 和 pet 共用一套导航与过滤语义。这样小精灵不会和硬件版行为分叉，也不会破坏三进程依赖边界。

