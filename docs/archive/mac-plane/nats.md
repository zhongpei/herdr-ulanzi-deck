这个 macOS 模块不需要重新设计数据通道。它直接接你现有 collector 的 NATS：

```text id="slzrvn"
collector
  -> embedded NATS: nats://127.0.0.1:4222
  -> herdr.v1.snapshot.full
  -> herdr.v1.collector.heartbeat
  -> panel-mac 订阅
```

你现在的 `collector/internal/publisher` 已经发布两类消息：`PublishSnapshot()` 把 `protocol.FleetSnapshot` JSON 发布到 `protocol.SubjectSnapshot`，`PublishHeartbeat()` 把带 `ts` 字段的 heartbeat JSON 发布到 `protocol.SubjectHeartbeat`。
`deck/internal/subscriber` 也已经证明了订阅模式：连接 NATS，订阅 snapshot subject，JSON decode 成 `FleetSnapshot`，再通过 channel 交给渲染侧；heartbeat 也单独订阅。

macOS 版照这个模式实现，只是语言从 Go 换成 Swift。

---

# 1. 数据流

```text id="a1fwuq"
collector publisher
    │
    │ PublishSnapshot(FleetSnapshot)
    │ PublishHeartbeat({ts})
    ▼
embedded NATS
    │
    ├─ herdr.v1.snapshot.full
    │       │
    │       ▼
    │   panel-mac NATSClient
    │       │
    │       ▼
    │   SnapshotStore.apply(snapshot)
    │       │
    │       ▼
    │   DisplayModelBuilder.build()
    │       │
    │       ▼
    │   BoardView.setNeedsDisplay()
    │
    └─ herdr.v1.collector.heartbeat
            │
            ▼
        CollectorHealth.markAlive()
```

核心原则：

```text id="m9c1wy"
NATSClient 只负责收消息
SnapshotStore 只负责保存最新状态
DisplayModelBuilder 负责把状态转成 UI 模型
BoardView 只负责绘制
```

---

# 2. Swift 侧推荐使用 nats.swift

macOS 原生模块可以直接用 `nats-io/nats.swift`。它是 NATS 官方组织下的 Swift client，README 里说明当前支持 Core NATS，并支持通过 Swift Package Manager 添加依赖；示例里使用 `NatsClientOptions().url(...).build()`、`connect()`、`subscribe(subject:)` 和 async sequence 接收消息。([GitHub][1])

在 Xcode 里添加 Package：

```text id="dvv70v"
https://github.com/nats-io/nats.swift.git
```

依赖名：

```text id="otdpnh"
Nats
```

注意：当前需求只需要 Core NATS，不需要 JetStream。nats.swift README 也说明 JetStream、KV、Object Store 还在 roadmap；这不影响你当前的 snapshot/heartbeat 订阅。([GitHub][1])

---

# 3. Swift 协议结构

Swift 侧需要复制 Go `protocol` 的 JSON 结构。字段名要和 JSON 一致。

```swift id="jqh8f2"
import Foundation

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

struct FleetSnapshot: Codable {
    let version: Int
    let seq: UInt64
    let updatedAt: String
    let machines: [MachineInfo]
    let agents: [AgentState]
    let stats: AgentStats

    enum CodingKeys: String, CodingKey {
        case version
        case seq
        case updatedAt = "updated_at"
        case machines
        case agents
        case stats
    }
}

struct MachineInfo: Codable {
    let name: String
    let abbr: String
    let color: String
    let health: String?
    let lastError: String?
    let lastSeenAt: String?

    enum CodingKeys: String, CodingKey {
        case name
        case abbr
        case color
        case health
        case lastError = "last_error"
        case lastSeenAt = "last_seen_at"
    }
}

struct AgentState: Codable, Identifiable {
    let id: String
    let machine: String
    let agent: String
    let name: String
    let status: AgentStatus
    let focused: Bool
    let workspace: String
    let workspaceID: String
    let tabLabel: String?
    let paneID: String
    let updatedAt: String?

    enum CodingKeys: String, CodingKey {
        case id
        case machine
        case agent
        case name
        case status
        case focused
        case workspace
        case workspaceID = "workspace_id"
        case tabLabel = "tab_label"
        case paneID = "pane_id"
        case updatedAt = "updated_at"
    }
}

struct AgentStats: Codable {
    let done: Int
    let idle: Int
    let working: Int
    let waiting: Int?
    let blocked: Int
    let error: Int?
    let offline: Int?
    let stale: Int?
    let unknown: Int
}
```

如果你当前 Go protocol 还没有 `waiting/error/offline/stale` 字段，Swift 侧可以先设成 optional。这样协议升级时不会崩。

---

# 4. Subjects 常量

Swift 侧不要到处写字符串，集中放一个文件：

```swift id="xuhv4l"
enum NATSSubjects {
    static let snapshot = "herdr.v1.snapshot.full"
    static let heartbeat = "herdr.v1.collector.heartbeat"
    static let focusCommand = "herdr.v1.command.agent.focus"
}
```

如果你 Go 侧现在 subject 名字不是这个，而是 `protocol.SubjectSnapshot` / `protocol.SubjectHeartbeat` 定义的其他值，Swift 侧必须跟 Go 的 `protocol/subjects.go` 完全一致。你现在 Go deck 订阅的就是 `protocol.SubjectSnapshot` 和 `protocol.SubjectHeartbeat`，collector 也发布到这两个 subject。

---

# 5. NATSClient.swift

这个类只做连接、订阅、解码前的数据分发。不要在这里做 UI 过滤。

```swift id="okepyd"
import Foundation
import Nats

final class HerdrNATSClient {
    private let url: URL
    private let store: SnapshotStore
    private var nats: NatsClient?

    init(url: URL, store: SnapshotStore) {
        self.url = url
        self.store = store
    }

    func start() {
        Task.detached(priority: .background) {
            await self.runForever()
        }
    }

    private func runForever() async {
        while true {
            do {
                try await connectAndSubscribe()
            } catch {
                await MainActor.run {
                    self.store.markNATSDisconnected(error.localizedDescription)
                }
                try? await Task.sleep(nanoseconds: 2_000_000_000)
            }
        }
    }

    private func connectAndSubscribe() async throws {
        let client = NatsClientOptions()
            .url(url)
            .build()

        self.nats = client
        try await client.connect()

        await MainActor.run {
            self.store.markNATSConnected()
        }

        let snapshotSub = try await client.subscribe(subject: NATSSubjects.snapshot)
        let heartbeatSub = try await client.subscribe(subject: NATSSubjects.heartbeat)

        async let snapshotTask: Void = consumeSnapshots(snapshotSub)
        async let heartbeatTask: Void = consumeHeartbeats(heartbeatSub)

        _ = try await (snapshotTask, heartbeatTask)
    }

    private func consumeSnapshots(_ subscription: NatsSubscription) async throws {
        for try await msg in subscription {
            guard let payload = msg.payload else {
                continue
            }

            do {
                let snapshot = try JSONDecoder().decode(FleetSnapshot.self, from: payload)
                await MainActor.run {
                    self.store.apply(snapshot)
                }
            } catch {
                await MainActor.run {
                    self.store.recordDecodeError(error.localizedDescription)
                }
            }
        }
    }

    private func consumeHeartbeats(_ subscription: NatsSubscription) async throws {
        for try await _ in subscription {
            await MainActor.run {
                self.store.markCollectorAlive()
            }
        }
    }

    func stop() {
        Task {
            try? await nats?.close()
        }
    }
}
```

上面代码里的 `NatsSubscription` 类型名可能需要按你引入的 `nats.swift` 版本调整；关键结构是：

```text id="u9nbmv"
connect
subscribe snapshot
subscribe heartbeat
for await message
decode JSON
MainActor 更新 SnapshotStore
```

nats.swift README 示例展示的就是 `connect()` 后 `subscribe(subject:)`，再用 `for try await msg in subscription` 处理消息。([GitHub][1])

---

# 6. SnapshotStore.swift

`SnapshotStore` 是 UI 和 NATS 之间的缓冲层。它必须在主线程更新，因为后面会触发 `BoardView` 重绘。

```swift id="b8luc9"
import Foundation
import AppKit

@MainActor
final class SnapshotStore {
    private(set) var latestSnapshot: FleetSnapshot?
    private(set) var natsConnected: Bool = false
    private(set) var collectorAlive: Bool = false
    private(set) var lastHeartbeatAt: Date?
    private(set) var lastError: String?

    weak var boardView: BoardView?

    func apply(_ snapshot: FleetSnapshot) {
        latestSnapshot = snapshot
        collectorAlive = true
        lastError = nil
        rebuildAndRedraw()
    }

    func markCollectorAlive() {
        collectorAlive = true
        lastHeartbeatAt = Date()
        rebuildAndRedraw()
    }

    func markNATSConnected() {
        natsConnected = true
        lastError = nil
        rebuildAndRedraw()
    }

    func markNATSDisconnected(_ error: String) {
        natsConnected = false
        collectorAlive = false
        lastError = error
        rebuildAndRedraw()
    }

    func recordDecodeError(_ error: String) {
        lastError = "decode: \(error)"
        rebuildAndRedraw()
    }

    func checkHeartbeatTimeout() {
        guard let last = lastHeartbeatAt else {
            collectorAlive = false
            rebuildAndRedraw()
            return
        }

        if Date().timeIntervalSince(last) > 5 {
            collectorAlive = false
            rebuildAndRedraw()
        }
    }

    private func rebuildAndRedraw() {
        guard let view = boardView else { return }

        let model = DisplayModelBuilder.build(
            snapshot: latestSnapshot,
            viewState: view.viewState,
            collectorAlive: collectorAlive,
            natsConnected: natsConnected,
            lastError: lastError
        )

        view.updateDisplayModel(model)
    }
}
```

注意：这里不直接绘制，只生成 `DisplayModel` 后交给 `BoardView`。

---

# 7. BoardView 如何接收更新

```swift id="6jxvvu"
final class BoardView: NSView {
    var viewState = ViewState()
    private var displayModel: DisplayModel?

    func updateDisplayModel(_ model: DisplayModel) {
        self.displayModel = model
        self.needsDisplay = true
    }

    override func draw(_ dirtyRect: NSRect) {
        guard let model = displayModel else {
            drawEmptyState()
            return
        }

        drawBoard(model)
    }
}
```

数据链路就是：

```text id="hg3f0d"
NATS message
  -> SnapshotStore.apply()
  -> DisplayModelBuilder.build()
  -> BoardView.updateDisplayModel()
  -> BoardView.draw()
```

---

# 8. 启动流程

`AppDelegate` 中组装：

```swift id="pszyq3"
final class AppDelegate: NSObject, NSApplicationDelegate {
    private var statusBar: StatusBarController!
    private var panelWindow: PanelWindowController!
    private var store: SnapshotStore!
    private var natsClient: HerdrNATSClient!
    private var heartbeatTimer: Timer?

    func applicationDidFinishLaunching(_ notification: Notification) {
        store = SnapshotStore()

        panelWindow = PanelWindowController(store: store)
        store.boardView = panelWindow.boardView

        statusBar = StatusBarController(panelWindow: panelWindow, store: store)

        natsClient = HerdrNATSClient(
            url: URL(string: "nats://127.0.0.1:4222")!,
            store: store
        )
        natsClient.start()

        heartbeatTimer = Timer.scheduledTimer(
            withTimeInterval: 1.0,
            repeats: true
        ) { [weak self] _ in
            Task { @MainActor in
                self?.store.checkHeartbeatTimeout()
            }
        }
    }

    func applicationWillTerminate(_ notification: Notification) {
        natsClient.stop()
        heartbeatTimer?.invalidate()
    }
}
```

---

# 9. 关键设计点：只保留最新 snapshot

你原来的 deck subscriber 有一个重要策略：snapshot channel 容量很小，并且收到新 snapshot 时会 drain 掉旧 snapshot，只保留最新的，避免 UI 渲染落后于真实状态。

macOS 版也应该遵守同样原则：

```text id="pkdrqi"
不要排队渲染每一帧 snapshot
只保存 latestSnapshot
每次 UI draw 都使用最新状态
```

Swift 侧的 `SnapshotStore.latestSnapshot` 天然就是这个模式。

不要这样做：

```text id="a5ele8"
收到 100 条 snapshot
排队渲染 100 次
```

要这样做：

```text id="jiz7xv"
收到 100 条 snapshot
latestSnapshot 被覆盖 100 次
UI 下一帧只画最新的
```

---

# 10. Focus 命令如何走 NATS

双击 Agent 卡片时，macOS 不应该直接连 Herdr。它应该向 collector 发 command。

```text id="kzn5bl"
BoardView double click
  -> ActionDispatcher.focusAgent(agentID)
  -> CommandPublisher.publishFocusCommand()
  -> NATS subject: herdr.v1.command.agent.focus
  -> collector 收到
  -> br.FocusAgent(machine, paneID)
```

Swift payload：

```swift id="g90ae7"
struct FocusAgentCommand: Codable {
    let version: Int
    let commandID: String
    let machine: String
    let paneID: String
    let source: String
    let createdAt: String

    enum CodingKeys: String, CodingKey {
        case version
        case commandID = "command_id"
        case machine
        case paneID = "pane_id"
        case source
        case createdAt = "created_at"
    }
}
```

发布：

```swift id="uewnsz"
func publishFocusCommand(agent: AgentState) async throws {
    let cmd = FocusAgentCommand(
        version: 1,
        commandID: UUID().uuidString,
        machine: agent.machine,
        paneID: agent.paneID,
        source: "panel-mac",
        createdAt: ISO8601DateFormatter().string(from: Date())
    )

    let data = try JSONEncoder().encode(cmd)
    try await nats.publish(data, subject: NATSSubjects.focusCommand)
}
```

---

# 11. 最小开发顺序

给普通应届生的实现顺序：

```text id="puvxtk"
1. 先用 MockSnapshotProvider 把 UI 画出来
2. 复制 Swift 版 FleetSnapshot / AgentState / AgentStats
3. 加 nats.swift 依赖
4. 写 HerdrNATSClient
5. 只订阅 snapshot，打印 seq 和 agents.count
6. 确认能收到 collector 发布的数据
7. 接到 SnapshotStore.apply()
8. 触发 BoardView 重绘
9. 再订阅 heartbeat
10. 实现 collector offline 灰色状态
11. 最后实现 focus command publish
```

---

# 12. 当前方案的最终答案

这个模块从原有 NATS 订阅数据的方式是：

```text id="4f9859"
连接 collector 内置 NATS:
  nats://127.0.0.1:4222

订阅:
  protocol.SubjectSnapshot    -> FleetSnapshot JSON
  protocol.SubjectHeartbeat   -> collector health

收到 snapshot:
  JSON decode
  覆盖 latestSnapshot
  build DisplayModel
  BoardView 重绘

收到 heartbeat:
  更新 lastHeartbeatAt
  collectorAlive = true

5 秒无 heartbeat:
  collectorAlive = false
  UI 进入 offline
```

也就是说，macOS 面板和现在的 `herdr-deck` 是同级消费者：

```text id="migxuu"
collector
  -> NATS
      -> deck subscriber
      -> mac panel subscriber
```

不要让 macOS 面板走 HTTP，也不要让它调用 deck。它只消费 `FleetSnapshot` 和 `Heartbeat` 两个 subject。

[1]: https://github.com/nats-io/nats.swift "GitHub - nats-io/nats.swift: Swift client for NATS, the cloud native messaging system. · GitHub"

