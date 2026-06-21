我认为你这个 **三进程方案是更好的最终架构**，比“一个主进程 + 多 Display 插件”更适合你现在的目标。

原因很明确：你现在已经不是单纯加一个 UI，而是要把 **采集平台、硬件显示、桌面小精灵** 三类依赖彻底拆开。三进程能把依赖、测试、崩溃边界、发布边界都分清楚。

---

# 1. 结论

推荐改成：

```text
Process 1：herdr-collector
  采集平台 / 状态中心 / 内置 NATS Server

Process 2：herdr-deck
  硬件显示进程，只负责 Ulanzi D200X

Process 3：herdr-pet
  桌面小精灵进程，只负责 Ebitengine GUI
```

数据流：

```text
Herdr / Claude / AgentBus / Remote Hosts
        │
        ▼
┌──────────────────────────┐
│ herdr-collector           │
│ - 采集状态                │
│ - 状态归一化              │
│ - FleetSnapshot           │
│ - 内置 NATS Server        │
└───────────┬──────────────┘
            │ NATS subjects
            ▼
   ┌────────────────┬────────────────┐
   ▼                ▼                ▼
herdr-deck       herdr-pet        future clients
硬件显示          桌面小精灵       TUI/Web/Logger
```

这比 HTTP 更适合你的场景，因为你不是 request/response，而是 **状态事件流 / 快照流 / 多消费者订阅**。

---

# 2. 为什么三进程比单进程更适合你

你的判断是对的。三进程有几个实际优势。

## 2.1 依赖彻底隔离

现在你的代码里硬件依赖已经侵入状态层。例如 `appstate.Store` 里面持有 `deck.Client`，并提供 `SetDeckClient()`、`SeedKeyActions()`，这说明状态层已经知道硬件设备的存在。这个耦合在单进程继续扩展时会越来越严重。

三进程后依赖边界变成：

```text
herdr-collector:
  依赖 herdr / ssh / nats / protocol
  不依赖 deck
  不依赖 ebitengine

herdr-deck:
  依赖 nats client / deck client / svg renderer
  不依赖 herdr
  不依赖 ebitengine

herdr-pet:
  依赖 nats client / ebitengine / assets
  不依赖 herdr
  不依赖 deck
```

这个边界非常干净。

---

## 2.2 崩溃隔离

桌面小精灵是 GUI / 游戏循环，硬件推送是 WebSocket + SVG→PNG，采集平台是远程连接和状态聚合。它们的故障模式完全不同。

三进程后：

```text
herdr-pet 崩溃：
  不影响采集
  不影响硬件显示

herdr-deck 崩溃：
  不影响采集
  不影响桌宠

herdr-collector 崩溃：
  两个显示端进入 disconnected/offline
```

这比一个进程里所有东西一起崩溃稳定。

---

## 2.3 测试更清楚

你现在的 `renderAll()` 同时做了 mapper、SVG renderer、per-key hash、物理 key 映射、deck client 推送。这个函数天然应该属于硬件显示进程，不应该属于采集平台。

三进程后测试会变清楚：

```text
collector 测试：
  输入 Herdr/Claude mock
  输出 FleetSnapshot / AgentEvent

deck 测试：
  输入 FleetSnapshot
  输出 14-key ViewModel / SVG / PNG / deck command

pet 测试：
  输入 FleetSnapshot
  输出 PetViewModel / layout / animation state
```

这对 Go 项目非常友好。

---

# 3. NATS 放在哪里

我同意你的方向：**采集平台内置 NATS 是合理的。**

NATS 官方文档说明，如果应用是 Go，并且适合部署场景，可以把 NATS server 嵌入到应用里。([NATS 文档][1])
JetStream 是 NATS 的内置持久化引擎，可以存储消息并支持后续 replay；和 Core NATS 必须有活跃订阅不同，JetStream 可以让消息被捕获并在之后被消费者重放。([NATS 文档][2])

所以你的架构可以是：

```text
herdr-collector
  ├─ 状态采集器
  ├─ FleetStore
  ├─ SnapshotBuilder
  └─ embedded NATS Server
        ├─ Core NATS subjects
        └─ 可选 JetStream
```

显示端只需要连接：

```text
nats://127.0.0.1:4222
```

不需要 HTTP。

---

# 4. NATS Core 还是 JetStream

这里要分清楚。你的状态显示有两类数据。

## 4.1 实时状态事件：用 NATS Core

例如：

```text
agent 状态变化
blocked 事件
waiting_user 事件
working 心跳
```

这些适合 Core NATS：

```text
herdr.agent.event
herdr.agent.heartbeat
herdr.collector.status
```

优点：

```text
低延迟
简单
适合实时 UI
```

缺点：

```text
消费者离线时会丢消息
```

但这对实时状态事件可以接受，因为 UI 最终要看的是最新状态，不是每一条历史事件。

---

## 4.2 最新全量快照：建议用 JetStream 或 KV

桌宠和硬件进程启动时，最重要的是马上拿到当前全局状态，而不是等下一次事件。

所以需要一个“最新状态快照”：

```text
herdr.fleet.snapshot
```

这里有两个选择。

### 方案 A：Core NATS 周期广播 snapshot

```text
collector 每 1-2 秒 publish 一次全量 snapshot
deck/pet 启动后等下一帧
```

优点：

```text
最简单
不需要 JetStream
```

缺点：

```text
显示端刚启动时可能要等 1-2 秒
collector 如果只在状态变化时发，显示端可能拿不到初始状态
```

### 方案 B：JetStream 保存最新 snapshot

JetStream 的 stream 是 message store，可以定义消息如何存储、限制和保留；官方也建议使用 JetStream publish，因为服务器会确认消息已被存储。([NATS 文档][3])
JetStream consumer 是 stream 的有状态视图，并且可以提供 at-least-once delivery。([NATS 文档][4])

适合这样：

```text
collector 写入 latest snapshot
deck 启动后读取 latest snapshot
pet 启动后读取 latest snapshot
之后再订阅实时事件
```

我建议正式方案用：

```text
Core NATS：实时事件
JetStream：最新快照 / 重要状态
```

---

# 5. 最推荐的数据流

## 总体数据流

```text
┌────────────────────────────────────────────────────┐
│ herdr-collector                                     │
│                                                    │
│ Sources                                            │
│  - Herdr local socket                              │
│  - SSH remote Herdr                                │
│  - Claude push / AgentBus future                   │
│                                                    │
│ Normalizer                                         │
│  - UnifiedWorkspace                                │
│  - AgentState                                      │
│                                                    │
│ FleetStore                                         │
│  - all agents                                      │
│  - all workspaces                                  │
│  - stats                                           │
│  - ttl/offline                                     │
│                                                    │
│ Publisher                                          │
│  - NATS Core events                                │
│  - JetStream latest snapshot                       │
└──────────────────────┬─────────────────────────────┘
                       │
                       │ nats://127.0.0.1:4222
                       ▼
       ┌───────────────┴────────────────┐
       ▼                                ▼
┌──────────────────┐          ┌───────────────────┐
│ herdr-deck        │          │ herdr-pet          │
│                  │          │                   │
│ NATS Subscriber   │          │ NATS Subscriber    │
│ DeckViewModel     │          │ PetViewModel       │
│ SVG Renderer      │          │ Ebitengine State   │
│ SVG -> PNG        │          │ Desktop Overlay    │
│ Ulanzi WS Client  │          │                   │
└──────────────────┘          └───────────────────┘
```

---

# 6. 三个子项目的组织方式

我建议不是一个大 Go module 里三个 cmd，而是 **一个 workspace + 三个独立 module + 一个 shared protocol module**。

```text
herdr-agentview/
  go.work

  protocol/
    go.mod
    snapshot.go
    event.go
    status.go
    subjects.go
    version.go

  collector/
    go.mod
    cmd/herdr-collector/main.go
    internal/source/herdr/
    internal/source/push/
    internal/fleet/
    internal/natsserver/
    internal/publisher/

  deck/
    go.mod
    cmd/herdr-deck/main.go
    internal/subscriber/
    internal/viewmodel/
    internal/render/
    internal/deckclient/
    internal/profile/

  pet/
    go.mod
    cmd/herdr-pet/main.go
    internal/subscriber/
    internal/viewmodel/
    internal/game/
    internal/assets/
    internal/layout/
```

依赖方向：

```text
collector -> protocol
deck      -> protocol
pet       -> protocol
```

禁止：

```text
collector -> deck
collector -> pet
deck      -> collector
pet       -> collector
deck      -> pet
pet       -> deck
```

这就是你说的“依赖关系完全分清”。

---

# 7. protocol 模块必须很小

`protocol` 不能塞业务逻辑，只放三类东西：

```text
状态枚举
消息结构
NATS subject 常量
```

例如：

```go
package protocol

const (
	SubjectSnapshot = "herdr.fleet.snapshot"
	SubjectEventAll = "herdr.agent.event.*"
	SubjectHeartbeat = "herdr.collector.heartbeat"
)

type AgentStatus string

const (
	StatusIdle        AgentStatus = "idle"
	StatusWorking     AgentStatus = "working"
	StatusBlocked     AgentStatus = "blocked"
	StatusWaitingUser AgentStatus = "waiting_user"
	StatusDone        AgentStatus = "done"
	StatusUnknown     AgentStatus = "unknown"
	StatusOffline     AgentStatus = "offline"
)

type FleetSnapshot struct {
	Version   int          `json:"version"`
	Seq       uint64       `json:"seq"`
	UpdatedAt string       `json:"updated_at"`
	Stats     AgentStats   `json:"stats"`
	Agents    []AgentState `json:"agents"`
}

type AgentState struct {
	ID        string      `json:"id"`
	Source    string      `json:"source"`
	Host      string      `json:"host"`
	Project   string      `json:"project"`
	Workspace string      `json:"workspace"`
	Agent     string      `json:"agent"`
	Name      string      `json:"name"`
	Status    AgentStatus `json:"status"`
	Message   string      `json:"message,omitempty"`
	UpdatedAt string      `json:"updated_at"`
	TTLMS     int         `json:"ttl_ms"`
}
```

当前代码里的 `UnifiedWorkspace` 已经包含 connection metadata、workspace metadata 和 agents；`AgentStats` 已经有 done/idle/working/blocked/unknown 统计，这些都适合转成 protocol 里的 `FleetSnapshot`。

---

# 8. collector 的职责

`collector` 是唯一状态中心。

```text
collector/
  负责：
    1. 连接本机 Herdr socket
    2. 建立远程 SSH tunnel
    3. 拉取多主机 workspace/agent 状态
    4. 接收未来 Claude/Agent push
    5. 归一化状态
    6. 维护 FleetStore
    7. 计算 stats
    8. 发布 NATS event/snapshot
    9. 内置 NATS server
```

它不负责：

```text
不生成 SVG
不生成 PNG
不连接 UlanziDeck
不启动 Ebitengine
不关心桌宠布局
```

现有主程序里 local/SSH Herdr 初始化、bridge.AddConnection、bridge.FetchAll 这些逻辑应该移到 collector。

---

# 9. deck 的职责

`deck` 只做硬件显示。

```text
deck/
  负责：
    1. 连接 NATS
    2. 订阅 FleetSnapshot
    3. 转 DeckViewModel
    4. 生成 14-key SVG
    5. SVG -> PNG
    6. 推送 UlanziDeck WebSocket
    7. 处理硬件按键
```

它不负责：

```text
不连接 Herdr
不建立 SSH tunnel
不维护全局 FleetStore
不启动 NATS server
不依赖 Ebitengine
```

现有 `KeyHashTracker` 是 14-key 专用，注释里明确按 `mapper.RenderAll()` 的 0..13 顺序对应 K1-K10、K11-K14，所以应该留在 deck 子项目里。

---

# 10. pet 的职责

`pet` 只做桌面小精灵。

```text
pet/
  负责：
    1. 连接 NATS
    2. 订阅 FleetSnapshot / AgentEvent
    3. 转 PetViewModel
    4. 维护本地 UI 状态
    5. Ebitengine 渲染桌宠
    6. 显示 Agent 状态矩阵
```

它不负责：

```text
不连接 Herdr
不连接 UlanziDeck
不生成 SVG
不关心 deck key mapping
不启动 NATS server
```

这能保证 `pet` 的依赖只包含：

```text
protocol
nats.go
ebitengine
assets/layout 相关包
```

---

# 11. NATS subject 设计

建议先固定这些 subject：

```text
herdr.fleet.snapshot
herdr.agent.event
herdr.collector.heartbeat
herdr.collector.status
```

更细一点：

```text
herdr.v1.snapshot.full
herdr.v1.agent.upsert
herdr.v1.agent.remove
herdr.v1.collector.heartbeat
herdr.v1.collector.status
```

第一版建议只做两个：

```text
herdr.v1.snapshot.full
herdr.v1.collector.heartbeat
```

理由：简单、稳定、不容易错。

---

# 12. 发布策略

collector 每次状态变化时发布 snapshot，同时周期性补发。

```text
状态变化：
  立即发布 herdr.v1.snapshot.full

无状态变化：
  每 2 秒补发一次 snapshot 或 heartbeat

collector 正常：
  heartbeat 每 1 秒

collector 退出：
  deck/pet 超时后显示 offline
```

deck/pet 启动后：

```text
1. 连接 NATS
2. 读取 latest snapshot，如果使用 JetStream
3. 订阅后续 snapshot
4. 如果 5 秒没有 heartbeat，显示 collector offline
```

---

# 13. 是否必须 JetStream

我的建议：

## 第一阶段：可以只用 Core NATS

最小可用：

```text
collector 内置 NATS
collector 每 2 秒广播 full snapshot
deck/pet 订阅 snapshot
```

这已经能跑。

## 第二阶段：加 JetStream latest snapshot

加 JetStream 后解决：

```text
deck/pet 启动时不用等下一轮广播
deck/pet 短暂断开后能拿到最新快照
重要事件可保留
```

正式版建议加 JetStream，但不是第一天必须做。

---

# 14. 三进程方案的主要风险

这个方案是对的，但要注意几个风险。

## 风险 1：collector 内置 NATS 后，collector 变成总线单点

如果 collector 挂了，NATS 也挂了。

这是否能接受？我认为当前可以接受，因为 collector 本身就是状态源。collector 挂了，显示端本来也没有新状态。

如果未来你要让总线独立存在，就变成：

```text
Process 1：nats-server
Process 2：collector
Process 3：deck
Process 4：pet
```

但这不是你当前想要的三进程方案。

---

## 风险 2：NATS embedded 生命周期要管理好

collector 需要做到：

```text
启动 embedded NATS
等待 ready
再启动状态采集
再发布 snapshot
退出时优雅 shutdown
```

否则 deck/pet 可能连接不上或反复重连。

---

## 风险 3：协议版本必须固定

因为三进程后协议就是边界。必须有：

```text
version
seq
updated_at
schema_version
```

不要让 deck/pet 直接 import collector 内部 types。

---

# 15. 最终推荐方案

我建议你定成：

```text
三进程 + shared protocol + collector 内置 NATS
```

最终结构：

```text
herdr-agentview/
  protocol/       # 公共协议，三方都依赖
  collector/      # 采集平台 + embedded NATS
  deck/           # Ulanzi D200X 显示进程
  pet/            # Ebitengine 桌面小精灵
```

最终数据流：

```text
collector
  -> embedded NATS
  -> snapshot/event subjects
      -> deck
      -> pet
```

一句话判断：

**你的三进程方案比单进程插件方案更稳，也更适合长期维护；采集平台内置 NATS 可以解决事件推流问题，但第一版建议只做 full snapshot 广播，稳定后再加 JetStream 保存 latest snapshot。**

[1]: https://docs.nats.io/running-a-nats-service/clients?utm_source=chatgpt.com "NATS Server Clients | NATS Docs"
[2]: https://docs.nats.io/nats-concepts/jetstream?utm_source=chatgpt.com "JetStream - NATS Docs"
[3]: https://docs.nats.io/nats-concepts/jetstream/streams?utm_source=chatgpt.com "Streams - NATS Docs"
[4]: https://docs.nats.io/nats-concepts/jetstream/consumers?utm_source=chatgpt.com "Consumers - NATS Docs"

