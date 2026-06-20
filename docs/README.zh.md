# herdr-deck

在 **Ulanzi D200X** 上显示 [herdr](https://herdr.dev) AI 编程 Agent 的状态。

> 开发指南: [docs/development-guide.md](./docs/development-guide.md)
> 架构文档: [docs/architecture.md](./docs/architecture.md)

<p align="center">
  <img src="../assets/deck-photo.jpg" width="600" alt="Herdr 在 Ulanzi D200X 上的效果">
</p>

## 平台支持

**仅支持 macOS 和 Linux。** herdr 本身只在这两个平台运行。作者主要在 **macOS (arm64)** 上测试。

## 功能

- **实时 Agent 状态** — 读取 herdr 数据，显示在 D200X LCD 按键上
- **多机器支持** — 同时连接多台 herdr 实例（本机 + SSH 隧道）
- **优先级排序** — BLOCKED → DONE → WORKING → IDLE → UNKNOWN
- **过滤导航** — K11=全部、K12=机器循环、K13=Space 循环
- **品牌色** — 各 Agent 有独立品牌色
- **机器色** — 每台机器有定义色
- **自动刷新** — herdr 数据每 2 秒轮询一次，状态不变时不重渲染

## 架构

提供两个实现版本：

### Go 版（推荐）— `go/`

```
独立二进制，无需 Node.js。
编译为单一可执行文件。

Go binary → WebSocket → UlanziDeck D200X
```

- **单文件二进制**: ~15MB，零运行时依赖
- **无需插件目录**: 任意位置运行
- **无需 npm install**: 内置 SVG→PNG 渲染
- **语言**: Go 1.25+
- **依赖**: gorilla/websocket, tdewolff/canvas, zerolog, cobra

### JS 版（原始）— `src/`

```
Node.js 插件 → WebSocket → UlanziDeck D200X
```

- 需要 Node.js 20+ 和 npm 依赖（ws, sharp）
- 需要拷贝到插件目录运行

## 快速开始（Go）

```bash
# 构建
cd go && make build
./build/herdrdeck --addr 127.0.0.1 --port 3906

# 或带调试日志
./build/herdrdeck -d

# 全量部署脚本（杀旧进程、构建、启动）
bash scripts/deploy-go.sh
```

### 依赖（Go）

- [herdr](https://herdr.dev) 运行中
- [Ulanzi Studio](https://www.ulanzi.com) 3.1.9+
- Ulanzi D200X 设备
- Go 1.25+（构建用）

## 快速开始（JS）

```bash
cp -r herdr-deck \
  ~/Library/Application\ Support/Ulanzi/UlanziDeck/Plugins/com.ulanzi.herdr.agentview.ulanziPlugin
cd ~/Library/Application\ Support/Ulanzi/UlanziDeck/Plugins/com.ulanzi.herdr.agentview.ulanziPlugin
npm install
node src/index.js 127.0.0.1 3906 zh_CN
```

### 依赖（JS）

- Node.js 20+、npm

## 配置

创建 `~/.config/herdr-deck/connections.json`:

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

## 按键功能（D200X）

| 按键 | 功能 |
|-----|------|
| K1-K10 | Agent 状态（优先级排序） |
| K11 | **ALL** — 全部机器 |
| K12 | **机器循环** — 切换机器 |
| K13 | **Space 循环** — 切换 Space |
| K14 | **全局统计** — D / I / W / B / ? |

### K14 统计栏

宽键显示各状态计数。**字母**用状态色（D=绿, I=灰, W=黄, B=红），**数字**用白色，项目间有间距。

## 实现对比

| 方面 | Go | JavaScript |
|--------|-----|-----------|
| 运行环境 | 编译二进制 | Node.js 20+ |
| 依赖 | 零运行时 | ws, sharp |
| SVG→PNG | tdewolff/canvas（纯 Go） | sharp（C++ 原生） |
| CLI | cobra (`--addr`, `--port`, `--debug`) | 位置参数 |
| 部署 | 任意路径运行 | 必须放插件目录 |
| 体积 | ~15MB | 184MB+（含 node_modules） |
| 插件清单 | 存根 stub | 完整 plugin |

## 开发

```bash
# Go 测试
cd go && make test

# 构建并运行
cd go && make run

# 部署 Go 版本
bash scripts/deploy-go.sh

# 部署 JS 版本
bash scripts/deploy-and-run.sh
```

详见 [docs/development-guide.md](./docs/development-guide.md)。
