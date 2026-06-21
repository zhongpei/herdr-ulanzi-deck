# 三进程重构方案 v2 (修正版)

> 目标: 按 `docs/go-architecture.md` 三进程架构完全重构 Go 部分
> 方案: git worktree + 4 个 Go module (protocol/collector/deck + go.work)
> 分支: `refactor-three-process`

---

## 一、总体架构变更

```
单进程 (现状)                  三进程 (目标)
─────────────                  ────────────
cmd/herdrdeck              ┌─ collector/cmd/herdr-collector  ← 采集 + embedded NATS
  ├─ herdr/*               │
  ├─ state/*               ├─ deck/cmd/herdr-deck             ← 硬件显示 (Ulanzi D200X)
  ├─ mapper/*              │
  ├─ render/*              └─ pet/ (未来预留)
  ├─ deck/*
  ├─ appstate/*                   ↑
  ├─ profile/*                    │ NATS (embedded, 127.0.0.1:4222)
  └─ sysstats/*                   │
                           ┌──────┘
                           │ protocol/  ← 公共模块 (三方依赖)
```

---

## 二、目录结构

```
herdr-agentview/
├── go.work                         ← Go workspace
├── protocol/                       ← 公共协议
│   ├── go.mod
│   ├── snapshot.go                 ← FleetSnapshot, AgentState, AgentStats, MachineInfo
│   ├── status.go                   ← AgentStatus 枚举 + Priority
│   ├── subjects.go                 ← NATS subject 常量
│   └── version.go                  ← 协议版本
├── collector/                      ← 采集平台
│   ├── go.mod
│   ├── Makefile
│   ├── cmd/herdr-collector/main.go ← cobra CLI + 事件循环
│   └── internal/
│       ├── config/config.go        ← LoadConfig, ConnConfig (迁移自 pkg/herdr/config.go)
│       ├── herdrclient/client.go   ← Unix socket JSON-line 客户端 (迁移)
│       ├── tunnel/tunnel.go        ← SSH tunnel (迁移)
│       ├── bridge/bridge.go        ← 多连接数据合并 (改写)
│       ├── fleet/store.go          ← FleetStore: 状态维护 + TTL
│       ├── publisher/publisher.go  ← NATS 发布 (snapshot + heartbeat)
│       └── natsserver/server.go    ← 嵌入式 NATS server
├── deck/                            ← 硬件显示
│   ├── go.mod
│   ├── Makefile
│   ├── cmd/herdr-deck/main.go      ← cobra CLI + 事件循环
│   └── internal/
│       ├── subscriber/subscriber.go ← NATS 订阅 (snapshot + heartbeat)
│       ├── fleet/manager.go        ← 状态管理: sort/filter/stats/duration (改写)
│       ├── viewmodel/builder.go    ← fleet → 14 KeyCommand (迁移自 mapper)
│       ├── viewmodel/types.go      ← AgentKeyData, NavAllData 等 render 类型
│       ├── render/render.go        ← SVG 生成 (迁移)
│       ├── render/colors.go        ← Agent/Status 颜色表 (迁移)
│       ├── render/icons.go         ← Agent/Status 图标路径 (迁移)
│       ├── deckclient/client.go    ← WebSocket → UlanziDeck (迁移)
│       ├── deckclient/draw.go      ← SVG→PNG (迁移)
│       ├── deckclient/keyhash.go   ← 14-key hash 去重 (迁移)
│       ├── controller/store.go     ← dirty flag / snapshot capture / hash
│       ├── controller/controller.go← 事件循环主控 + renderAll
│       ├── profile/manager.go      ← D200X profile (迁移)
│       └── sysstats/sysstats.go    ← 本地 CPU/MEM (迁移)
├── docs/
│   ├── go-architecture.md          ← 架构蓝图
│   ├── refactor-plan.md            ← 本文件
│   └── refactor-todo.md            ← TODO 清单
└── scripts/
    ├── deploy-collector.sh
    ├── deploy-deck.sh
    └── deploy-all.sh
```

**旧目录**: `go/` (重构完成后删除)

---

## 三、protocol 模块定义

### `protocol/snapshot.go`

```go
package protocol

type AgentStatus string

const (
    StatusIdle    AgentStatus = "idle"
    StatusWorking AgentStatus = "working"
    StatusBlocked AgentStatus = "blocked"
    StatusDone    AgentStatus = "done"
    StatusUnknown AgentStatus = "unknown"
)

var StatusPriority = map[AgentStatus]int{
    StatusBlocked: 0, StatusDone: 1, StatusWorking: 2,
    StatusIdle: 3, StatusUnknown: 4,
}

type MachineInfo struct {
    Name  string `json:"name"`
    Abbr  string `json:"abbr"`
    Color string `json:"color"`
}

type AgentState struct {
    ID          string      `json:"id"`          // "machine|paneID"
    Machine     string      `json:"machine"`
    Agent       string      `json:"agent"`
    Name        string      `json:"name"`
    Status      AgentStatus `json:"status"`
    Focused     bool        `json:"focused"`
    Workspace   string      `json:"workspace"`
    WorkspaceID string      `json:"workspace_id"`
    TabLabel    string      `json:"tab_label,omitempty"`
    PaneID      string      `json:"pane_id"`
    UpdatedAt   string      `json:"updated_at"`
}

type AgentStats struct {
    Done    int `json:"done"`
    Idle    int `json:"idle"`
    Working int `json:"working"`
    Blocked int `json:"blocked"`
    Unknown int `json:"unknown"`
}

type FleetSnapshot struct {
    Version   int           `json:"version"`
    Seq       uint64        `json:"seq"`
    UpdatedAt string        `json:"updated_at"`
    Machines  []MachineInfo `json:"machines"`
    Agents    []AgentState  `json:"agents"`
    Stats     AgentStats    `json:"stats"`
}
```

### `protocol/subjects.go`

```go
package protocol

const (
    SubjectSnapshot  = "herdr.v1.snapshot.full"
    SubjectHeartbeat = "herdr.v1.collector.heartbeat"
)
```

---

## 四、数据流

```
┌─ collector event loop (2s fetch + 1s heartbeat)
│   bridge.FetchAll() → raw data
│     → fleetStore.Apply(raw) → compute stats, bump seq
│     → publisher.PublishSnapshot(snap)   [herdr.v1.snapshot.full]
│     → publisher.PublishHeartbeat()      [herdr.v1.collector.heartbeat]
│
├─ NATS (embedded, nats://127.0.0.1:4222)
│
└─ deck event loop
    ├─ subscriber → FleetSnapshot channel
    │     → fleet.ApplySnapshot(snap) → dirty=true
    ├─ render tick (50ms) →
    │     controller.Capture(fleet) → hash compare
    │     → renderAll() → 14 SVG → PNG → WebSocket
    ├─ heartbeat check (1s) →
    │     5s 无心跳 → fleet.MarkOffline()
    └─ sys tick (10s) → local CPU/MEM
```

---

## 五、离线状态处理

deck 维护 `ConnectionHealth`:

- **HealthConnected**: 有周期性 snapshot + heartbeat
- **HealthOffline**: 5s 无 heartbeat → agents 灰显，K14 显示 "NATS DISCONNECTED"
- **启动过渡**: subscriber 等待 collector，超时 → 显示 "WAITING COLLECTOR..."

---

## 六、Deck 配置文件

`~/.config/herdr-deck/deck.json` (新建):

```json
{
    "nats_addr": "nats://127.0.0.1:4222",
    "ulanzi_addr": "127.0.0.1",
    "ulanzi_port": 3906,
    "k11_toggle": false
}
```

**K11Toggle 从 `connections.json` 移除**，改为 deck 本地 preference。

---

## 七、完整迁移矩阵

| 当前文件 | 新文件 | 变更类型 |
|---------|--------|---------|
| `go/pkg/types/types.go` | 拆为 3 部分: | |
| ↳ AgentStatus/AgentStats | `protocol/snapshot.go` | 重写 (FleetSnapshot) |
| ↳ AgentKeyData 等render类型 | `deck/internal/viewmodel/types.go` | 移 |
| ↳ UnifiedWorkspace 等 | 删除 (collector 内部新 struct) | 删 |
| `go/pkg/herdr/config.go` | `collector/internal/config/config.go` | 移 |
| `go/pkg/herdr/client.go` | `collector/internal/herdrclient/client.go` | 移 |
| `go/pkg/herdr/bridge.go` | `collector/internal/bridge/bridge.go` | 改 |
| `go/pkg/herdr/tunnel.go` | `collector/internal/tunnel/tunnel.go` | 移 |
| `go/pkg/state/state.go` | `deck/internal/fleet/manager.go` | 改 (Init 接口变更) |
| `go/pkg/mapper/mapper.go` | `deck/internal/viewmodel/builder.go` | 改 |
| `go/pkg/render/render.go` | `deck/internal/render/render.go` | 移 |
| `go/pkg/render/colors.go` | `deck/internal/render/colors.go` | 移 |
| `go/pkg/render/icons.go` | `deck/internal/render/icons.go` | 移 |
| `go/pkg/deck/client.go` | `deck/internal/deckclient/client.go` | 移 |
| `go/pkg/deck/draw.go` | `deck/internal/deckclient/draw.go` | 移 |
| `go/pkg/appstate/store.go` | `deck/internal/controller/store.go` | 改 (拆分) |
| `go/pkg/appstate/keyhash.go` | `deck/internal/deckclient/keyhash.go` | 移 |
| `go/pkg/profile/manager.go` | `deck/internal/profile/manager.go` | 移 |
| `go/pkg/sysstats/sysstats.go` | `deck/internal/sysstats/sysstats.go` | 移 |
| `go/cmd/herdrdeck/main.go` | `collector/cmd/herdr-collector/main.go` + `deck/cmd/herdr-deck/main.go` | 拆/新 |

### 测试迁移

| 当前 | 目标 | 说明 |
|------|------|------|
| `go/pkg/appstate/keyhash_test.go` | `deck/internal/deckclient/keyhash_test.go` | 移 |
| `go/pkg/deck/draw_test.go` | `deck/internal/deckclient/draw_test.go` | 移 |
| `go/pkg/herdr/client_test.go` | `collector/internal/herdrclient/client_test.go` | 移 |
| `go/pkg/mapper/mapper_test.go` | `deck/internal/viewmodel/builder_test.go` | 改 (测试数据适配) |
| `go/pkg/render/render_test.go` | `deck/internal/render/render_test.go` | 移 |
| `go/pkg/state/state_test.go` | `deck/internal/fleet/manager_test.go` | 改 (Init 接口变更) |
| `go/pkg/sysstats/sysstats_test.go` | `deck/internal/sysstats/sysstats_test.go` | 移 |

---

## 八、执行顺序

### Phase 0 — 前置

- [ ] git checkout -b refactor-three-process
- [ ] git worktree add ../ulanzi-deck-herdr-refactor
- [ ] 清理 worktree 中的 go/ 目录

### Phase 1 — protocol/

- [ ] 创建 protocol/ 目录 + go.mod
- [ ] snapshot.go (FleetSnapshot + AgentState + AgentStats + MachineInfo)
- [ ] status.go (AgentStatus 枚举)
- [ ] subjects.go (NATS constants)
- [ ] version.go
- [ ] protocol 测试

### Phase 2 — collector/

- [ ] 创建 collector/ 目录 + go.mod + Makefile
- [ ] config/config.go 迁移
- [ ] herdrclient/client.go 迁移
- [ ] tunnel/tunnel.go 迁移
- [ ] bridge/bridge.go 改写
- [ ] fleet/store.go (新建: FleetStore + TTL)
- [ ] publisher/publisher.go (新建: NATS publish)
- [ ] natsserver/server.go (新建: embedded NATS)
- [ ] cmd/herdr-collector/main.go (新建 entry point)
- [ ] collector 测试

### Phase 3 — deck/

- [ ] 创建 deck/ 目录 + go.mod + Makefile
- [ ] subscriber/subscriber.go (新建: NATS subscribe)
- [ ] fleet/manager.go 改写 (Init 接口从 FleetSnapshot)
- [ ] viewmodel/ (迁移 mapper + 新建 types.go)
- [ ] render/ 迁移
- [ ] deckclient/ 迁移 (client + draw + keyhash)
- [ ] profile/manager.go 迁移
- [ ] sysstats/sysstats.go 迁移
- [ ] controller/store.go + controller.go (新建 event loop)
- [ ] cmd/herdr-deck/main.go (新建 entry point)
- [ ] deck 测试

### Phase 4 — 整合

- [ ] go.work 创建
- [ ] scripts/deploy-*.sh
- [ ] 双进程 E2E 验证
- [ ] 删除旧 go/ 目录
