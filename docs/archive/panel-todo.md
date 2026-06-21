# herdr-panel 实现清单

## Phase 0：项目结构

- [ ] 新建 `panel/` 目录
- [ ] `panel/go.mod`（module github.com/herdr-deck/herdrdeck/panel）
- [ ] `panel/cmd/herdr-panel/main.go`（入口，cobra CLI）
- [ ] 注册 panel 到 `go.work`（`use ( ./panel )`）
- [ ] `go work sync` 验证依赖解析

## Phase 1：NATS 数据订阅

- [ ] `panel/internal/subscriber/subscriber.go`
  - 复制 deck/internal/subscriber，简化为 panel 所需
  - 只保留 Snapshot 和 Heartbeat 订阅
  - 去掉 deckclient/profile 相关逻辑
- [ ] `panel/internal/subscriber/subscriber_test.go`
  - 连接测试
  - 消息接收测试

## Phase 2：Store

- [ ] `panel/internal/state/store.go`
  - 持最新 protocol.FleetSnapshot
  - 持 displaymodel.ViewState
  - 持 collector health 状态
  - dirty flag 触发 UI 刷新（通过 channel 通知 app 刷新）
- [ ] `panel/internal/state/store_test.go`
  - ApplySnapshot
  - ViewState 读写
  - Health 跟踪

## Phase 3：Fyne UI 骨架

- [ ] `panel/internal/app/app.go`
  - Fyne app 初始化
  - 组装 store + builder + subscriber + alert
  - 后台 goroutine 定时轮询 store.dirty（200ms），dirty 则调用 `canvas.Refresh(container)` 重建 UI
  - channel 通知 vs 定时轮询二选一：用 time.Ticker 200ms 轮询 dirty flag，避免 channel 竞争
- [ ] `panel/internal/ui/main_window.go`
  - Fyne 主窗口
  - 标题 "Herdr Panel"
  - 可拖动、可缩放
  - 关闭按钮 → 隐藏到托盘（非退出）
  - 默认位置：屏幕右下角
  - 窗口位置/尺寸持久化（fyne.Preferences）
- [ ] `panel/internal/ui/main_window_test.go`

## Phase 4：顶部控件

- [ ] `panel/internal/ui/stats_bar.go`
  - K14：状态颜色块 + 数字（🔴🟡🔵🟢✅⚫）
  - 纯 agent 计数，无 CPU/MEM
- [ ] `panel/internal/ui/toolbar.go`
  - K11：[ALL] [ACT] 并排按钮，高亮当前
  - K12：Machine 下拉框 [DEV ▾]
  - K13：Space 下拉框 [api ▾]
  - 事件 → displaymodel.Builder 更新 ViewState → MarkDirty
- [ ] `panel/internal/ui/toolbar_test.go`

## Phase 5：卡片网格

- [ ] `panel/internal/ui/card_grid.go`
  - 2×3 固定网格，无滚动条
  - 每格：状态色块背景 + agent名 + 机器缩写 + 持续时间
  - 按优先级取前 6（blocked > done > working > idle > unknown）
  - 点击卡片 → 可选：显示 agent 详情
- [ ] `panel/internal/ui/theme.go`
  - 状态颜色映射
  - 字体、间距、圆角
- [ ] `panel/internal/ui/card_grid_test.go`

## Phase 6：系统托盘

- [ ] `panel/internal/ui/tray.go`
  - 托盘常驻图标
  - 左键单击 → 显示/隐藏窗口
  - 右键菜单：
    - "Show Panel" / "Hide Panel"
    - "Quit"（真正退出）
  - 图标随状态变色（collector offline → 灰；有 blocked → 红；全部正常 → 绿）
  - 不包含 ALL/ACT 切换（窗口内有按钮）
- [ ] `panel/internal/ui/tray_test.go`

## Phase 7：告警规则

- [ ] `panel/internal/alert/rules.go`
  - AlertRule 结构：WatchStatuses []protocol.AgentStatus
  - 默认：`[blocked]`
  - 通过 CLI 参数 `--alert-on` 设置，如 `--alert-on blocked,done,working`
  - 解析逗号分隔的状态字符串
- [ ] `panel/internal/alert/monitor.go`
  - 对比新旧 snapshot 的 agent 状态
  - 检测到 WatchStatuses 中的状态出现
  - 触发行为：
    1. 窗口弹出到前台
    2. 对应卡片闪烁/高亮（Fyne canvas.NewRectangle + 定时透明度动画）
    3. 托盘图标变色（已有）
- [ ] `panel/internal/alert/monitor_test.go`

## Phase 8：集成 & 构建

- [ ] `panel/internal/app/app.go` 完整集成
  - subscriber → store → displaymodel.Builder → UI 刷新
  - alert monitor 挂载
  - 无 CPU/MEM 采集
- [ ] `panel/cmd/herdr-panel/main.go` CLI
  - `--nats` 默认 `nats://127.0.0.1:4222`
  - `--debug` debug 日志
  - `--alert-on` 默认 `blocked`
  - 传递给 alert/rules 模块
- [ ] `panel/Makefile`
  - build、test、clean
- [ ] 更新 `scripts/deploy-all.sh` 加入 panel

## Phase 9：UI 细节打磨

- [ ] 告警卡片闪烁动画（Fyne canvas 定时器，透明度交替 1.0↔0.3）
- [ ] 窗口最小尺寸限制
- [ ] collector offline 状态显示（窗口顶部 banner 或整体变灰）
- [ ] 首次启动无数据时的空态显示（"Waiting for collector..."）
- [ ] 点击卡片 → 详情弹窗（dialog.ShowCustom）
- [ ] 暗色主题（Fyne 内置 theme.DarkTheme()，无需自绘）
- [ ] 高 DPI 验证（Fyne 原生支持，在 macOS Retina 上测文字和卡片清晰度）

## 测试要求

- 每个 internal 包有单元测试
- subscriber 测试 NATS 消息流
- displaymodel 测试共享过滤排序（已有 54 测试，复用）
- alert/monitor 测试状态变化检测
- `go vet ./...` 无错误
- `go test ./...` 全部通过
