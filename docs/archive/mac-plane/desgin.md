对。既然 UI 完全自绘，就不应该再套“按钮、下拉框、表格”这些传统控件模型。应该把它设计成一个 **Agent Fleet 信息仪表盘**：核心目标不是“能操作”，而是 **一眼发现异常、零学习切换维度、最少动作定位 Agent**。

你现有硬件语义可以继续保留：K1-K10 是 Agent 状态，K11 是 ALL/ACTIVE，K12 是机器切换，K13 是全局项目/space 切换，K14 是全局统计。代码和文档里已经把这个物理布局和语义定义清楚了：K11 控制 ALL/ACTIVE，K12 切机器，K13 按 workspace label 全局切项目，K14 显示全局统计。
同时测试里也已经确认 K12/K13 是互斥的：进入机器模式会清空 space，进入 space 模式会清空 machine；K13 的项目切换是全局 space label，而不是单机 workspace ID。

---

# 1. 设计原则

这个面板不再模拟“14 个按钮”，而是复刻它们的 **信息语义**。

```text
K11 -> 视野范围：ALL / ACTIVE
K12 -> 机器维度：全部机器 / 某台机器
K13 -> 项目维度：全部项目 / 某个项目
K14 -> 全局健康统计
K1-K10 -> 当前最值得看的 Agent
```

新的 UI 不叫“按钮面板”，而是：

```text
Herdr Fleet Board
```

核心交互模型：

```text
看状态：不用点
切维度：一击/滚轮
定位异常：异常永远浮到最前
深入详情：点 Agent 卡片
执行聚焦：双击 Agent 卡片
```

---

# 2. 推荐主界面：Fleet Board

这是默认展开态，宽度建议 680–760px，高度 420–520px。

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

这个布局把 K14 放到顶部，把 K11/K12/K13 放到第二层，把 K1-K10 扩展成“Attention + Matrix”。

---

# 3. 信息层级

这个面板要分 5 层，不能平均用力。

## 第 1 层：全局健康

顶部第一眼只回答一个问题：

```text
现在有没有需要我处理的事？
```

所以 K14 不再是一个角落统计块，而是顶部主标题区：

```text
🔴2 BLOCKED   🟡1 WAITING   🔵8 WORKING   🟢5 IDLE   ✅3 DONE
CPU 38%       MEM 62%
```

如果有 blocked，整个顶部出现红色脉冲边线。
如果只有 waiting，顶部出现黄色细线。
如果 collector offline，整个顶部变灰。

---

## 第 2 层：视野控制

K11/K12/K13 不再是按钮，而是三条“视野轨道”。

```text
FOCUS        MACHINE FLOW                         PROJECT FLOW
ACTIVE       ALL  LCL  DEV◉  PRD  RUN             ALL  api◉  web
show hot     ────●────○────○────○────             ────●────○────
```

语义：

```text
FOCUS        = K11，ALL / ACTIVE
MACHINE FLOW = K12，机器维度
PROJECT FLOW = K13，项目维度
```

这里没有下拉框。
所有候选项直接铺出来，滚轮/点击/快捷键切换。

---

## 第 3 层：Attention

这一层只放最重要的 Agent。

```text
ATTENTION
╭──────────────╮ ╭──────────────╮ ╭──────────────╮
│ 🔴 api-3      │ │ 🔴 e2e-1      │ │ 🟡 doc-2      │
│ blocked      │ │ blocked      │ │ waiting      │
│ DEV / iplist │ │ RUN / tests  │ │ LCL / docs   │
│ 03m21s       │ │ 01m08s       │ │ 00m44s       │
╰──────────────╯ ╰──────────────╯ ╰──────────────╯
```

规则：

```text
blocked/error 永远进入 Attention
waiting_user 永远进入 Attention
working 超过阈值进入 Attention
offline/stale 进入 Attention
idle/done 不进入 Attention，除非被选中
```

这比硬件 K1-K10 更适合桌面：硬件只能放 10 个 key，桌面可以先放“需要看”的，再放全量矩阵。

---

## 第 4 层：Fleet Matrix

这是全局态势图，不是普通表格。

```text
FLEET MATRIX
DEV  │ 🔴 api-3      🔵 test-2     🔵 api-1      🟢 ui-1
LCL  │ 🟡 doc-2      ✅ doc-1      🟢 idle-4
RUN  │ 🔴 e2e-1      🔵 build-2    ⚫ host-7
PRD  │ 🟢 watch-1    🟢 monitor-2
```

默认按机器分行。
如果切到 Project Flow，则按项目分行：

```text
FLEET MATRIX · PROJECT VIEW
api      │ 🔴 api-3      🔵 api-1      🟢 api-doc
tests    │ 🔴 e2e-1      🔵 test-2     🔵 build-2
docs     │ 🟡 doc-2      ✅ doc-1
monitor  │ 🟢 watch-1    ⚫ host-7
```

这样 K12/K13 不只是过滤，也会改变矩阵的主轴。

---

## 第 5 层：Selected / Detail

底部永远显示当前选中 Agent 或最高优先级事件：

```text
SELECTED  api-3 · claude · DEV / iplist-manager-v2
blocked: waiting for permission · double click to focus
```

如果没有选中，则显示最高优先级事件：

```text
TOP EVENT  e2e-1 blocked for 01m08s · RUN / tests
```

---

# 4. 极简操作设计

不要做传统按钮。所有操作变成热区 + 快捷键 + 滚轮。

## 鼠标操作

```text
点击 FOCUS 区        -> ALL / ACTIVE 切换
滚轮 FOCUS 区        -> ALL / ACTIVE 切换

点击 MACHINE FLOW    -> 选中最近机器
滚轮 MACHINE FLOW    -> 下一台 / 上一台机器
双击 MACHINE FLOW    -> 回到 ALL machines

点击 PROJECT FLOW    -> 选中最近项目
滚轮 PROJECT FLOW    -> 下一个 / 上一个项目
双击 PROJECT FLOW    -> 回到 ALL projects

点击 Agent 卡片      -> 选中，底部显示详情
双击 Agent 卡片      -> 发送 agent.focus
右键 Agent 卡片      -> pin / hide / copy info

点击顶部统计         -> 切到异常优先视图
双击顶部统计         -> reset all filters
```

## 键盘操作

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
1-9     快速选择 Attention 区 Agent
```

这比按钮/下拉效率更高。

---

# 5. 三种显示状态

## 状态 A：Mini Bar

菜单栏点击后先展开这个，适合快速看。

```text
╭────────────────────────────────────────────╮
│ HERDR  🔴2  🟡1  🔵8  🟢5  ✅3   LIVE       │
│ ACTIVE · DEV · api              09:42:18   │
╰────────────────────────────────────────────╯
```

点击 Mini Bar 下半区展开到 Fleet Board。

---

## 状态 B：Fleet Board

默认工作态，就是上面的大面板。

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

## 状态 C：Incident Mode

当 blocked/waiting 很多时，自动切成事故视图。

```text
╭────────────────────────────────────────────────────────────────────╮
│ INCIDENT MODE                                      🔴4  🟡3  ⚫1     │
│ Only actionable agents are shown · press A to show all              │
├────────────────────────────────────────────────────────────────────┤
│  NOW                                                                 │
│  01  🔴 api-3     DEV / iplist     waiting permission      03m21s    │
│  02  🔴 e2e-1     RUN / tests      command failed          01m08s    │
│  03  🔴 build-4   RUN / release    blocked by test         00m55s    │
│  04  🟡 doc-2     LCL / docs       needs review            00m44s    │
│  05  🟡 ui-5      LCL / panel      asks confirmation       00m33s    │
├────────────────────────────────────────────────────────────────────┤
│  BY MACHINE                                                          │
│  DEV  🔴1  🟡0  🔵2       RUN  🔴2  🟡0  🔵3       LCL  🔴0  🟡2     │
├────────────────────────────────────────────────────────────────────┤
│  1-9 select · Enter focus · M machine · P project · R reset          │
╰────────────────────────────────────────────────────────────────────╯
```

这个模式比传统表格更适合运维：异常即列表，按编号快速处理。

---

# 6. Agent 卡片设计

不要做普通行。Agent 是一个信息块。

```text
╭──────────────╮
│ 🔴 api-3      │  第一行：状态 + 短名
│ blocked      │  第二行：状态文本
│ DEV / iplist │  第三行：机器 / 项目
│ 03m21s       │  第四行：持续时间
╰──────────────╯
```

不同状态的视觉：

```text
🔴 blocked/error  : 红色左缘 + 轻微脉冲
🟡 waiting_user   : 黄色边框 + 慢闪
🔵 working        : 蓝色进度呼吸
🟢 idle           : 低亮绿色
✅ done           : 低亮白/绿
⚫ offline/stale  : 灰色颗粒感
```

---

# 7. 交互不是“按钮”，而是“状态机”

UI 内部可以设计成这几个状态：

```text
ViewState
  FocusLens:
    ALL | ACTIVE

  Axis:
    Global | Machine | Project

  SelectedMachine:
    "" | "LCL" | "DEV" | "PRD"

  SelectedProject:
    "" | "api" | "web" | "tests"

  SelectedAgent:
    "" | agentID

  BoardMode:
    Mini | Fleet | Incident | Detail
```

热区触发的是 action：

```text
Action
  ToggleFocusLens
  NextMachine
  PrevMachine
  ClearMachine
  NextProject
  PrevProject
  ClearProject
  SelectAgent(agentID)
  FocusAgent(agentID)
  ResetFilters
  ExpandDetail
  Collapse
```

你自绘 UI 只需要维护 hit map：

```text
HitZone
  Rect
  Cursor
  Action
  HoverState
```

这就是你的“组件系统”，但不是传统 GUI 组件。

---

# 8. 推荐最终视觉方向

我建议定为：

```text
黑色半哑光底
低饱和状态色
圆角卡片
细线分割
红/黄只用于告警
蓝色只表示 working
绿色降权显示
```

不要做太花。这个工具是状态面板，不是玩具。

视觉关键词：

```text
terminal dashboard
stream deck semantics
mission control
agent fleet board
low-noise alert surface
```

---

# 9. 最终 UI 定案建议

我建议你先确认这个主方案：

```text
顶部：K14 全局健康条
第二层：K11/K12/K13 三条视野轨道
第三层：Attention 异常卡片
第四层：Fleet Matrix 状态矩阵
底部：Selected / Top Event 详情条
```

也就是：

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

一句话：**不要做“按钮式面板”，做“Agent Fleet 雷达面板”。K11/K12/K13 是视野切换，K14 是健康仪表，K1-K10 升级成 Attention + Matrix。**

