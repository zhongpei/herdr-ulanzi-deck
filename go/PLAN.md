# Go 版重构方案

## 一、当前问题

| # | 问题 | 后果 |
|---|------|------|
| 1 | ReadPump goroutine 直接调 `renderAll()`，main goroutine 也调 | 多 goroutine 并发渲染 → state 命令乱序 → K1 黑闪 |
| 2 | 14 个 add 事件各触发一次全量渲染 | 196 条命令涌入 deck |
| 3 | 渲染结果相同也照渲不误 | 浪费，加剧闪烁 |
| 4 | `keyActions` 用 Go map（无序），`getFirstKeyAction()` 随机 | 外层 actionid 不确定 |
| 5 | 重连逻辑嵌在 ReadPump defer 里，旧 ws 未关闭 | 多连接冲突 |
| 6 | sendAck 条件判断依赖 `*int` 指针 | 易错 |

---

## 二、目标架构

### 三层分离

```
┌─────────────────────────────────────────────────────────┐
│ Layer 1: Event Sources                                  │
│                                                         │
│  deck WebSocket ──→ ReadPump ──→ callback (轻量操作)    │
│  herdr socket   ──→ (未来 feature)                      │
│  ticker 50ms    ──→ eventLoop                           │
└──────────────────────┬──────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────┐
│ Layer 2: State Store (appstate.Store)                   │
│                                                         │
│  业务方法:        内部操作:        状态:                  │
│  Store.SetAll()    → mapper.SetAll()    dirty flag      │
│  Store.NextMachine → mapper.NextMachine dirty           │
│  Store.RefreshData → bridge.FetchAll()  dirty           │
│  Store.SyncKey(id) → keyActions[id]=v   (无视觉影响)    │
│                                                         │
│  Capture(): 从 mapper + stateManager 生成快照 + hash    │
│                                                         │
│  所有状态变更必须经过 Store，不留外部直接修改的路径       │
└──────────────────────┬──────────────────────────────────┘
                       │ ticker 查询 dirty + hash 对比
                       ▼
┌─────────────────────────────────────────────────────────┐
│ Layer 3: Render                                         │
│                                                         │
│  eventLoop goroutine (唯一)                              │
│    select {                                             │
│      case <-ticker.C:                                   │
│        if !dirty → continue                             │
│        snap = store.Capture()                           │
│        if snap.hash == lastHash → continue（去重）       │
│        renderAll(snap)                                   │
│        lastHash = snap.hash                             │
│        dirty = false                                    │
│    }                                                    │
│                                                         │
│  并发归零，渲染去重，50ms 合并一次                        │
└─────────────────────────────────────────────────────────┘
```

### 数据流

```
事件 → callback（ReadPump goroutine）
        │
        ├─ "add":  store.SyncKey(key, actionID)
        │          (仅写 keyActions map，不设 dirty)
        │
        ├─ keydown K11: store.SetAll()
        │               dirty = true
        ├─ keydown K12: store.NextMachine()
        │               dirty = true
        ├─ keydown K13: store.NextSpace()
        │               dirty = true
        │
        └─ herdr (未来):
                      bridge.FetchAll()
                      stateManager.Init(unified)
                      dirty = true

ticker 50ms ──→ eventLoop:
                  if dirty:
                    snap = store.Capture()
                    if snap.hash != lastHash:
                      renderAll(snap)
                      lastHash = snap.hash
                    dirty = false
```

---

## 三、State Store 设计

### 位置

新的 `go/pkg/appstate/store.go` 包。

### 接口

```go
package appstate

type Store struct {
    sm        *state.Manager    // 引用，不拥有
    mapper    *mapper.Mapper    // 引用
    keyMap    map[string]string // key→actionID（有序）
    keyOrder  []string         // 插入顺序
    dirty     bool
}

// 业务方法
func (s *Store) SetAll()
func (s *Store) NextMachine()
func (s *Store) NextSpace()
func (s *Store) SyncHerdrData(unified []types.UnifiedWorkspace)
func (s *Store) SyncKeyAction(key, actionID string)

// 快照（供 render 使用）
func (s *Store) Capture() *Snapshot
func (s *Store) IsDirty() bool
func (s *Store) MarkClean()
```

### Snapshot

```go
type Snapshot struct {
    TopAgents []types.AgentInfo  // 排序后 top 10（含 ConnName/Abbr 等丰富信息）
    Mode      mapper.FilterMode
    ConnName  string
    WsID      string
    Stats     types.AgentStats
    hash      string
}

func (s *Snapshot) VisualHash() string
```

VisualHash 包含：

- 每个 agent 的 `PaneID + Agent + AgentStatus + Focused + Name + ConnName`
- `Mode + ConnName + WsID`
- `Stats (D/I/W/B/?)`
- keyActions 的版本号（只影响是否要重发 state 命令，不影响视觉）

### keyActions 有序

用 `map[string]string` + `[]string` 组合保证插入顺序：

```go
type Store struct {
    keyActions map[string]string // key → actionID
    keyOrder   []string          // 插入顺序
}
```

`GetFirstKeyAction()` 返回 `keyOrder[0]`（总是 "0_0"）。

---

## 四、并发模型

```
goroutine A（ReadPump）:
    deck.WebSocket.ReadMessage()
    → 轻量 callback（调 store.SetAll() 等 + dirty=true）

goroutine B（messagePump，管理重连）:
    for { ReadPump(); Sleep(2s); Connect(); dirty=true }

goroutine C（eventLoop）:
    select {
    case <-ticker.C: 渲染
    case <-msgChan:  处理消息（未来 herdr 事件）
    }
```

不需要锁——写 store 的几种操作：

- `SetAll()`：从 ReadPump 调，eventLoop 不会同时读
- `Capture()`：从 eventLoop 调，ReadPump 不会同时写
- ticker 和消息处理在同一个 goroutine(C) 里，串行

唯一需要注意：`keyActions` map 在 ReadPump（写）和 SetKeyImage（读）之间共享。但 SetKeyImage 只在 eventLoop 里调，ReadPump 写 keyActions 时 eventLoop 不会同时读。无竞争。

---

## 五、重连机制

```
messagePump goroutine:
    loop:
        deckClient.ReadPump()    // 阻塞，断开后返回
        log.Println("disconnected")
        time.Sleep(2s)
        deckClient.Connect()
        // 重连后重新 seed keyActions
        for k, v := range store.GetAllKeyActions() {
            deckClient.SeedKeyAction(k, v)
        }
        store.SetDirty()
```

`ReadPump` 自身不再有 defer reconnect。重连完全由 messagePump 管理。

`Connect()` 内部只做：Dial + send connected + 创建新的 ReadPump goroutine。旧的 ReadPump goroutine 已经退出了。

---

## 六、修改清单

| 文件 | 改动 |
|------|------|
| `go/pkg/appstate/store.go` | **新建**：Store + Snapshot |
| `go/cmd/herdrdeck/main.go` | **重写**：用 Store + eventLoop + ticker |
| `go/pkg/deck/client.go` | 简化：去掉 callback 的重渲染调用；ReadPump 去掉 defer reconnect；KeyActions 用 store |
| `go/pkg/mapper/mapper.go` | 无改动 |
| `go/pkg/state/state.go` | 无改动 |
| `go/PLAN.md` | 本文档 |

---

## 七、验收标准

1. K1-K10 颜色正确，filter 切换正常
2. K11-K13 导航不闪黑
3. 启动时 ≤3 次渲染（初始 + 去重）
4. 重连后自动恢复
5. 零并发调用 renderAll
