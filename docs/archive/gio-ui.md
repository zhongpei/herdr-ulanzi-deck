可以继续优化，但要分清两条路：

1. **继续用 Fyne 的高级特性优化**：能改善，但上限有限。
2. **保留 Go，换成 Gio 自绘 UI**：这是我现在认为的最佳方案。

如果你的目标是接近我们上面设计的 **Agent Fleet Board**，也就是这种形态：

```text
K14 全局健康条
K11 视野范围
K12 机器轨道
K13 项目轨道
Attention 异常卡片
Fleet Matrix 状态矩阵
底部 Selected / Top Event
```

那么最佳方案不是继续堆 Fyne 标准组件，而是：**用 Gio 重写 panel 进程，做 Go 版自绘信息面板。**

---

# 1. 结论

推荐方案：

```text
collector      Go，保持不变
deck           Go，保持不变
protocol       Go，保持不变
panel-gio      Go + Gio，自绘 Agent Fleet Board
```

也就是：

```text
collector
  -> embedded NATS
  -> FleetSnapshot
      -> herdr-deck
      -> herdr-panel-gio
```

你当前代码已经有清晰的三进程基础：`collector` 发布 NATS snapshot，`deck/internal/subscriber` 订阅 `protocol.SubjectSnapshot` 和 `protocol.SubjectHeartbeat`，并把 `FleetSnapshot` 通过 channel 交给渲染侧，这条数据链路可以直接复用到新的 Go GUI 面板里。

---

# 2. Fyne 还能不能优化？

能，但不是最佳路线。

Fyne 官方支持自定义 widget。它的文档说明 widget 本质上是一个 stateful canvas object，并通过 `CreateRenderer()` 提供渲染定义；`WidgetRenderer` 负责 widget 的具体渲染行为。([Fyne 文档][1])

Fyne 也有 canvas 层，可以使用 `Rectangle`、`Text`、`Circle`、`Line`、`Gradient` 等基础绘图对象。官方文档说明 Fyne 的所有可绘制对象都是 `CanvasObject`，并且 canvas primitives 是构建更高层 UI 的基础。([Fyne 文档][2])

同时 Fyne 允许自定义 Theme，主题接口可以控制颜色、字体、图标和尺寸。([Fyne 文档][3])

所以 Fyne 的优化路线是：

```text
不用 Button / Select / Table
改用 Custom Widget + CanvasObject
自己画卡片、轨道、状态点、统计条
自己处理点击区域
自己做主题
```

这能把界面从“默认控件很丑”提升到“定制化面板”。

但 Fyne 仍然有几个上限：

```text
1. 它是 retained-mode widget toolkit，不是为完全自绘仪表盘设计的
2. 自定义动画、hit-test、复杂状态过渡会越来越别扭
3. 大量 CanvasObject 组合后，渲染和状态管理会复杂
4. 视觉细节仍会受到 Fyne 渲染体系约束
5. 做成我们设计的 Fleet Board 会变成“绕开 Fyne 做一套自己的 UI”
```

所以我不建议继续把精力投入 Fyne 标准组件。如果保留 Fyne，只把它当作 **窗口壳 + 自定义 Canvas 面板**，不要用它的默认控件。

---

# 3. 为什么我现在推荐 Gio

Gio 官方定位就是 Go 的 cross-platform immediate-mode GUI，支持 macOS、Windows、Linux、Android、iOS、FreeBSD、OpenBSD 和 WebAssembly。([Gio UI][4])

更关键的是，Gio 的模型非常接近我们现在想做的 UI：在 `FrameEvent` 到来时，程序根据当前状态生成一组 operation list 并重新绘制；官方文档也说明，FrameEvent 触发后程序负责调用 `e.Frame`，用新的状态生成显示操作，这正是 immediate-mode 的核心。([Gio UI][5])

这对你的场景非常合适，因为你的 UI 本身就是：

```text
NATS 最新 Snapshot
  -> ViewState
  -> DisplayModel
  -> 重新 layout
  -> 重新 draw
```

这不是传统表单应用，而是仪表盘。Gio 比 Fyne 更接近这个模型。

---

# 4. 最佳方案：Go + Gio 自绘 Panel

最终结构：

```text
panel-gio/
  go.mod

  cmd/
    herdr-panel/
      main.go

  internal/
    subscriber/
      subscriber.go          # 复制/改造 deck/internal/subscriber

    store/
      store.go               # latest snapshot + heartbeat + view state

    displaymodel/
      state.go               # K11/K12/K13/K14 语义
      builder.go             # snapshot -> DisplayModel
      sort.go                # severity sort
      stats.go               # stats model

    board/
      app.go                 # Gio event loop
      layout.go              # Fleet Board 布局
      render.go              # 自绘
      input.go               # 点击、滚轮、快捷键
      hitzone.go             # hit-test 区域
      theme.go               # 颜色、字体、尺寸
      motion.go              # 动画

    command/
      publisher.go           # focus agent command
```

依赖方向：

```text
panel-gio -> protocol
panel-gio -> nats.go
panel-gio -> gio

panel-gio 不依赖 deck/internal
panel-gio 不依赖 collector/internal
```

---

# 5. 新数据流

```text
collector
  -> NATS
  -> herdr.v1.snapshot.full
  -> panel-gio subscriber
  -> Store.ApplySnapshot()
  -> DisplayModelBuilder.Build()
  -> Gio FrameEvent
  -> Board.Layout()
  -> Board.Draw()
```

和现有 deck 进程保持同级：

```text
collector
  -> NATS
      -> deck subscriber
      -> gio panel subscriber
```

你现在 `deck/internal/subscriber` 已经实现了 snapshot/heartbeat 订阅，并且采用“drain 未消费 snapshot，只保留最新”的策略，这个策略非常适合 UI 面板，也应该复制到 `panel-gio/internal/subscriber`。

---

# 6. Gio 版 UI 如何实现我们设计的 Fleet Board

## 6.1 不使用传统组件

Gio 里不要用 Material Button、List、Table。
直接做自绘：

```text
Board.Render(gtx)
  drawBackground()
  drawTopHealth()
  drawLensTracks()
  drawAttentionCards()
  drawFleetMatrix()
  drawSelectedBar()
```

布局结构：

```text
╭────────────────────────────────────────────────────────────────────╮
│ HERDR FLEET                                      LIVE · 09:42:18    │
│ ●●●  🔴2 BLOCKED   🟡1 WAITING   🔵8 WORKING   🟢5 IDLE   ✅3 DONE  │
├────────────────────────────────────────────────────────────────────┤
│  FOCUS        MACHINE FLOW                         PROJECT FLOW     │
│  ACTIVE       ALL  LCL  DEV◉  PRD  RUN             ALL  api◉  web   │
├────────────────────────────────────────────────────────────────────┤
│  ATTENTION                                                           │
│  ╭──────────────╮ ╭──────────────╮ ╭──────────────╮                 │
│  │ 🔴 api-3      │ │ 🔴 e2e-1      │ │ 🟡 doc-2      │                 │
│  │ blocked      │ │ blocked      │ │ waiting      │                 │
│  │ DEV / iplist │ │ RUN / tests  │ │ LCL / docs   │                 │
│  ╰──────────────╯ ╰──────────────╯ ╰──────────────╯                 │
├────────────────────────────────────────────────────────────────────┤
│  FLEET MATRIX                                                        │
│  DEV  │ 🔴 api-3      🔵 test-2     🔵 api-1      🟢 ui-1            │
│  LCL  │ 🟡 doc-2      ✅ doc-1      🟢 idle-4                        │
│  RUN  │ 🔴 e2e-1      🔵 build-2    ⚫ host-7                         │
├────────────────────────────────────────────────────────────────────┤
│  SELECTED  api-3 · claude · DEV / iplist-manager-v2                 │
│  blocked: waiting for permission · double click to focus             │
╰────────────────────────────────────────────────────────────────────╯
```

---

## 6.2 Gio 里的“组件”其实是函数

你不要创建传统 widget。
直接写纯函数：

```go
func LayoutBoard(gtx layout.Context, m DisplayModel) layout.Dimensions
func DrawTopHealth(gtx layout.Context, r image.Rectangle, m StatsModel)
func DrawLens(gtx layout.Context, r image.Rectangle, m LensModel)
func DrawAttention(gtx layout.Context, r image.Rectangle, agents []AgentCard)
func DrawMatrix(gtx layout.Context, r image.Rectangle, rows []MatrixRow)
func DrawSelected(gtx layout.Context, r image.Rectangle, selected *SelectedModel)
```

这和我们前面说的“完全自绘”一致。

---

# 7. Gio 相比 Fyne 的核心优势

| 项目                  | Fyne 高级用法 | Gio |
| ------------------- | --------: | --: |
| 适合默认控件              |         强 |   中 |
| 适合完全自绘面板            |         中 |   强 |
| 自定义布局               |         中 |   强 |
| 自定义绘制               |         中 |   强 |
| 交互 hit-test         |        别扭 |  自然 |
| 动画                  |        中弱 | 更自然 |
| 状态驱动重绘              |         中 |   强 |
| 接近我们设计的 Fleet Board |        勉强 |  合适 |
| 是否仍然纯 Go            |         是 |   是 |

结论：

```text
如果只是“让 Fyne 不那么丑”，继续 Fyne custom widget。
如果要接近我们设计的 Agent Fleet Board，换 Gio。
```

---

# 8. Fyne 保留方案：只适合短期救火

如果你不想立刻迁移，Fyne 可以这样优化：

```text
1. 删除所有默认 Button / Select / Table
2. 新建一个 BoardWidget
3. BoardWidget.CreateRenderer() 返回一个自定义 renderer
4. renderer 里维护：
   - background rectangle
   - top stats canvas objects
   - lens track canvas objects
   - agent cards canvas objects
   - matrix text/circle objects
5. BoardWidget 实现 Tapped / Scrolled / MouseMoved
6. Theme 里统一字体、颜色、尺寸
```

包结构：

```text
panel-fyne/
  internal/
    board/
      widget.go
      renderer.go
      theme.go
      hitzone.go
      displaymodel.go
```

但这只是过渡方案，不是最佳终局。

Fyne 的官方文档也暗示了这条路：canvas primitive 本身是非交互的，如果需要交互，通常应该把它们用在 custom widget 里。([Go Packages][6])

---

# 9. 推荐落地路线

## 第一阶段：保留现有 Go 三进程，新增 `panel-gio`

不要动 collector/deck。

```text
collector/
deck/
protocol/
panel-gio/
```

`panel-gio` 先只做 mock 数据。

验收：

```text
能画出 Top Health
能画出 K11/K12/K13
能画出 Attention
能画出 Matrix
能点击切换 ViewState
```

---

## 第二阶段：接入 NATS

直接复制 deck subscriber 思路：

```text
Subscribe protocol.SubjectSnapshot
Subscribe protocol.SubjectHeartbeat
只保留 latest snapshot
5 秒无 heartbeat -> offline
```

验收：

```text
collector 启动后 panel-gio 实时显示
collector 停止后 panel-gio 变灰
```

---

## 第三阶段：实现交互

```text
A       ALL / ACTIVE
M       下一台机器
P       下一个项目
R       reset
1-9     选择 Attention agent
Enter   focus selected
```

双击 agent 通过 NATS command 发回 collector。

---

## 第四阶段：做视觉优化

```text
1. 自定义深色主题
2. 状态颜色体系
3. 圆角卡片
4. 轻微动画
5. working 呼吸
6. blocked pulse
7. hover highlight
```

---

# 10. 最终推荐

我的最终判断：

```text
不建议继续深度投资 Fyne 标准组件。
可以短期用 Fyne Custom Widget 救火。
最佳 Go-only 方案是 Gio。
```

最终方案：

```text
Go collector 继续负责采集
Go deck 继续负责硬件
Go protocol 保持共享协议
新增 Go Gio panel 负责高级自绘 UI
```

一句话：

**如果坚持 Go，又要把丑界面优化到接近我们设计的 Fleet Board，最佳方案是迁移到 Gio，用 immediate-mode 自绘方式实现整个信息面板；Fyne 只能作为短期过渡。**

[1]: https://docs.fyne.io/extend/custom-widget/?utm_source=chatgpt.com "Writing a Custom Widget | Fyne Documentation"
[2]: https://docs.fyne.io/explore/canvas/?utm_source=chatgpt.com "Canvas and CanvasObject - Fyne Documentation"
[3]: https://docs.fyne.io/extend/custom-theme/?utm_source=chatgpt.com "Creating a Custom Theme - Fyne Documentation"
[4]: https://gioui.org/?utm_source=chatgpt.com "Gio UI"
[5]: https://gioui.org/doc/architecture/input?utm_source=chatgpt.com "Input"
[6]: https://pkg.go.dev/fyne.io/fyne/v2/canvas?utm_source=chatgpt.com "canvas package - fyne.io/fyne/v2/canvas"
