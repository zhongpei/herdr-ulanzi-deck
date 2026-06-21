# herdr-deck

在 **Ulanzi D200X** 上显示 [herdr](https://herdr.dev) AI 编程 Agent 的状态。

> 开发指南: [docs/development-guide.md](./development-guide.md)
> 架构参考: [AGENTS.md](../AGENTS.md)

<p align="center">
  <img src="../assets/deck-photo.jpg" width="600" alt="Herdr 在 Ulanzi D200X 上的效果">
</p>

## 平台支持

**仅支持 macOS 和 Linux。** herdr 本身只在这两个平台运行。作者主要在 **macOS (arm64)** 上测试。

## 功能

- **实时 Agent 状态** — 读取 herdr 数据，显示在 D200X LCD 按键上
- **多机器支持** — 同时连接多台 herdr 实例（本机 + SSH 隧道）
- **优先级排序** — BLOCKED → DONE → WORKING → IDLE → UNKNOWN
- **过滤导航** — K11=ALL/ACT、K12=机器循环、K13=全局 Space 循环
- **品牌色** — 各 Agent 有独立品牌色
- **机器色** — 每台机器有定义色
- **NATS 推送** — Collector 每 2s 轮询 herdr，通过嵌入式 NATS 推送快照；状态不变不重渲染

## 项目结构

三进程架构 + 共享模块：

```
herdr-collector (状态采集 + 内置 NATS)
      │
      │ NATS subjects
      ▼
herdr-deck (Ulanzi D200X 硬件显示)

共享模块:
  protocol/     — 类型定义、状态枚举、NATS subject 常量
  displaymodel/ — 视图状态、过滤、导航、统计模型
```

## 快速开始

### 依赖

- [herdr](https://herdr.dev) 运行中
- [Ulanzi Studio](https://www.ulanzi.com) 3.1.9+
- Ulanzi D200X 设备
- Go 1.26+

### 构建与运行

```bash
# 构建全部模块
cd collector && make build
cd deck      && make build

# 先启动 collector
./build/herdr-collector --debug

# 另一个终端启动 deck
./build/herdr-deck --debug --k11-toggle

# 或使用部署脚本
bash scripts/deploy-all.sh
```

### 配置

Collector 读取 `~/.config/herdr-deck/connections.json`：

```json
{
  "connections": [
    {
      "name": "local",
      "abbr": "LCL",
      "color": "#4ADE80",
      "type": "local"
    },
    {
      "name": "dev-server",
      "abbr": "DEV",
      "color": "#1E3A5F",
      "type": "ssh",
      "host": "user@hostname",
      "remoteSocket": "/home/user/.config/herdr/herdr.sock",
      "localPort": 19999
    }
  ]
}
```

Deck 使用 CLI flags: `--nats`, `--addr`, `--port`, `--k11-toggle`, `--debug`。

## 工作流程

```
herdr-collector (2s 轮询)
  → bridge.FetchAll()
  → fleet.Store
  → publisher.PublishSnapshot (herdr.v1.snapshot.full)
  → embedded NATS
    → herdr-deck subscriber
    → fleet.Manager (时长/健康/系统统计)
    → displaymodel.Builder (ViewState → Model)
    → viewmodel.Adapt (Model → 14 键渲染命令)
    → render (SVG)
    → deckclient (SVG→PNG→WebSocket)
    → Ulanzi D200X
```

## 按键功能 (D200X)

| 按键 | 功能 |
|-----|------|
| K1-K10 | Agent 状态（优先级排序） |
| K11 | **ALL / ACTIVE** — 显示全部或仅 BLOCKED/WORKING/DONE。蓝色=ALL，琥珀色=ACTIVE |
| K12 | **机器循环** — 切换机器，清空 Space 过滤 |
| K13 | **Space 循环（全局）** — 按 workspace 标签全局过滤（不限机器） |
| K14 | **全局统计** — D / I / W / B / ? 计数，CPU/MEM 占用 |

## Agent 状态优先级

1. **BLOCKED** — 最高优先级（红）
2. **DONE** — 完成（绿）
3. **WORKING** — 进行中（黄）
4. **IDLE** — 闲置（灰）
5. **UNKNOWN** — （灰）

## 开发

```bash
# 分模块测试
cd protocol     && go test ./...
cd displaymodel && go test ./...
cd collector    && make test
cd deck         && make test

# 构建
cd collector && make build
cd deck      && make build
```

详见 [docs/development-guide.md](./development-guide.md)。
