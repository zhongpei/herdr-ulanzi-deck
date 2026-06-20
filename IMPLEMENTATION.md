# 实现方案 — Herdr Agent Status on UlanziDeck

> 基于 DESIGN.md 的详细实现方案，包含伪代码

---

## 一、项目结构

```
ulanzi-deck-herdr/
├── package.json
├── DESIGN.md
├── README.md
│
├── src/
│   ├── index.js                  # 入口：初始化、生命周期
│   ├── config.js                 # 配置读取
│   ├── connection-manager.js     # SSH tunnel 管理
│   ├── herdr-client.js           # Unix socket / TCP JSON-line 客户端
│   ├── state-manager.js          # 多连接状态合并
│   ├── button-mapper.js          # 分页 + 按键映射
│   ├── icon-renderer.js          # SVG 合成（Agent图标+状态+别名）
│   ├── deck-client.js            # UlanziDeck WebSocket 客户端
│   └── icons.js                  # 所有 Agent SVG 路径定义
│
├── manifest.json                 # UlanziDeck 插件声明
└── assets/
    └── icons/                    # SVG 源文件（可选，inline SVG 优先在 icons.js）
```

---

## 二、各模块详细设计与伪代码

---

### 2.1 配置模块 `src/config.js`

**职责：** 读取 `~/.config/herdr-deck/connections.json`，返回配置对象。

```javascript
// ~/.config/herdr-deck/connections.json
//
// {
//   "connections": [
//     { "name": "local", "abbr": "LCL", "type": "local" },
//     { "name": "dev-server", "abbr": "DEV", "type": "ssh",
//       "host": "dev.internal.com",
//       "remoteSocket": "~/.local/share/herdr/herdr.sock" },
//     { "name": "home", "abbr": "HOM", "type": "ssh",
//       "host": "home-server",
//       "remoteSocket": "/home/user/.local/share/herdr/herdr.sock" }
//   ]
// }

import fs from 'fs';
import path from 'path';

const CONFIG_PATH = path.join(
  process.env.HOME || process.env.USERPROFILE,
  '.config', 'herdr-deck', 'connections.json'
);

export function loadConfig() {
  // 读取 CONFIG_PATH
  // 解析 JSON
  // 验证必填字段（name, abbr, type）
  // 验证 type 为 "local" 或 "ssh"
  // SSH 连接必须有 host 和 remoteSocket
  // 返回 { connections: [...] }
}

export function configPath() {
  return CONFIG_PATH;
}
```

---

### 2.2 连接管理 `src/connection-manager.js`

**职责：**

- 为 SSH 连接启动/监控 `ssh -L` 进程
- 返回每个连接可连接的 `{ type, path }`（local 直接返回 socket 路径，SSH 返回 localhost:port）

```javascript
import { spawn } from 'child_process';
import net from 'net';

// SSH tunnel 实例
class SSHTunnel {
  constructor(connConfig) {
    this.name = connConfig.name;
    this.host = connConfig.host;
    this.remoteSocket = connConfig.remoteSocket;
    this.localPort = null;  // 连接后填充
    this.process = null;
    this.ready = false;
  }

  async start() {
    // 1. 找空闲端口：创建临时 server 监听 port=0，获取实际端口后 close
    const localPort = await getFreePort();

    // 2. spawn ssh -L
    //    ssh -L <localPort>:<remoteSocket> <host> -N -o ExitOnForwardFailure=yes
    //    注意 remoteSocket 如果带 ~/，需要展开为完整路径
    const proc = spawn('ssh', [
      '-L', `${localPort}:${expandHome(this.remoteSocket)}`,
      this.host,
      '-N',
      '-o', 'ExitOnForwardFailure=yes'
    ], { stdio: ['ignore', 'pipe', 'pipe'] });

    // 3. 监控 stderr 判断连接就绪
    //    ssh 输出 "Entering interactive session" 或无报错 → 就绪
    //    或等待 3s 后测试连接
    await waitForTunnel(proc, localPort);

    this.localPort = localPort;
    this.process = proc;
    this.ready = true;

    // 4. 监控进程退出 → 自动重连
    proc.on('exit', (code) => {
      this.ready = false;
      this.process = null;
      // 指数退避重连
      scheduleReconnect(this);
    });
  }

  getTarget() {
    // SSH tunnel 连接目标为 TCP localhost
    return { type: 'tcp', host: '127.0.0.1', port: this.localPort };
  }

  async stop() {
    if (this.process) {
      this.process.kill();
      this.process = null;
    }
    this.ready = false;
  }
}

// 本地连接
class LocalConnection {
  constructor(connConfig) {
    this.name = connConfig.name;
    // 默认 socket 路径
    this.socketPath = '~/.local/share/herdr/herdr.sock';
  }

  async start() {
    return; // 无需启动
  }

  getTarget() {
    return { type: 'unix', path: expandHome(this.socketPath) };
  }

  async stop() {}
}

// ConnectionManager：管理多个连接
export class ConnectionManager {
  constructor(config) {
    this.connectors = []; // [{ name, abbr, HerdrClient实例, tunnel/Local实例 }]
  }

  async startAll(config) {
    for (const connCfg of config.connections) {
      // 根据 type 创建 tunnel 或 local
      const transport = connCfg.type === 'ssh'
        ? new SSHTunnel(connCfg)
        : new LocalConnection(connCfg);

      await transport.start();
      const target = transport.getTarget();

      // 每个连接一个 HerdrClient
      const client = new HerdrClient(target);
      await client.connect();

      this.connectors.push({
        name: connCfg.name,
        abbr: connCfg.abbr,
        client,
        transport,
      });
    }
  }

  async stopAll() {
    for (const c of this.connectors) {
      await c.client.disconnect();
      await c.transport.stop();
    }
  }

  getConnectorByName(name) {
    return this.connectors.find(c => c.name === name);
  }

  getAll() {
    return this.connectors;
  }
}
```

---

### 2.3 Herdr 客户端 `src/herdr-client.js`

**职责：**

- 通过 Unix socket 或 TCP 连接到 herdr server
- 发送 JSON-line 请求，接收响应
- 支持事件订阅（长连接，持续推送）

**协议验证：**

- 请求格式：`{"id":"<id>","method":"<method>","params":{...}}\n`
- 响应格式：`{"id":"<id>","result":{"type":"<result_type>",...}}\n`
- 错误格式：`{"id":"<id>","error":{"code":"...","message":"..."}}\n`
- 订阅后推送：`{"event":"pane_agent_status_changed","data":{...}}\n`

```javascript
import net from 'net';

export class HerdrClient {
  constructor(target) {
    // target: { type: 'unix', path: '/path/to/sock' }
    //      或 { type: 'tcp', host: '127.0.0.1', port: 12345 }
    this.target = target;
    this.socket = null;
    this.buffer = '';
    this.pending = new Map();  // id -> { resolve, reject }
    this.eventHandlers = [];
    this.requestId = 0;
  }

  async connect() {
    return new Promise((resolve, reject) => {
      const sock = this.target.type === 'unix'
        ? net.createConnection({ path: this.target.path })
        : net.createConnection({ host: this.target.host, port: this.target.port });

      sock.on('connect', () => {
        this.socket = sock;
        resolve();
      });

      sock.on('data', (chunk) => {
        this.buffer += chunk.toString();
        this.processLines();
      });

      sock.on('error', reject);
      sock.on('close', () => this.handleDisconnect());
    });
  }

  processLines() {
    // JSON-line: 每行一个完整 JSON 对象
    const lines = this.buffer.split('\n');
    this.buffer = lines.pop() || '';  // 最后一行可能不完整

    for (const line of lines) {
      if (!line.trim()) continue;

      try {
        const msg = JSON.parse(line);
        this.dispatchMessage(msg);
      } catch (e) {
        console.error('herdr: parse error', line.slice(0, 200), e);
      }
    }
  }

  dispatchMessage(msg) {
    // 1) 如果是订阅事件推送（有 event 字段）
    if (msg.event) {
      // 形如: { "event": "pane_agent_status_changed", "data": { ... } }
      for (const handler of this.eventHandlers) {
        handler(msg);
      }
      return;
    }

    // 2) 如果是请求响应（有 id 字段）
    if (msg.id && this.pending.has(msg.id)) {
      const { resolve, reject } = this.pending.get(msg.id);
      this.pending.delete(msg.id);

      if (msg.error) {
        reject(new Error(msg.error.message));
      } else {
        resolve(msg);  // { id, result: { type, ... } }
      }
      return;
    }

    // 3) 未知消息
    console.warn('herdr: unhandled message', JSON.stringify(msg).slice(0, 200));
  }

  async request(method, params = {}) {
    const id = `herdr-deck:${++this.requestId}`;
    const request = JSON.stringify({ id, method, params }) + '\n';

    return new Promise((resolve, reject) => {
      this.pending.set(id, { resolve, reject });

      if (!this.socket || this.socket.destroyed) {
        reject(new Error('socket not connected'));
        return;
      }

      this.socket.write(request, 'utf-8');

      // 超时 10s
      setTimeout(() => {
        if (this.pending.has(id)) {
          this.pending.delete(id);
          reject(new Error(`request timeout: ${method}`));
        }
      }, 10000);
    });
  }

  // 便捷 API 方法
  async listAgents() {
    // 发送: {"id":"...","method":"agent.list","params":{}}
    // 返回: { result: { type: "agent_list", agents: [...] } }
    const res = await this.request('agent.list', {});
    return res.result.agents;  // AgentInfo[]
  }

  async getAgent(target) {
    // 发送: {"id":"...","method":"agent.get","params":{"target":"..."}}
    const res = await this.request('agent.get', { target });
    return res.result.agent;
  }

  async focusAgent(target) {
    // 发送: {"id":"...","method":"agent.focus","params":{"target":"..."}}
    const res = await this.request('agent.focus', { target });
    return res.result.agent;
  }

  async listWorkspaces() {
    // 发送: {"id":"...","method":"workspace.list","params":{}}
    const res = await this.request('workspace.list', {});
    return res.result.workspaces;  // WorkspaceInfo[]
  }

  async subscribe(params) {
    // 发送: {"id":"...","method":"events.subscribe","params":{subscriptions:[...]}}
    const res = await this.request('events.subscribe', params);
    return res;
  }

  onEvent(handler) {
    this.eventHandlers.push(handler);
  }

  handleDisconnect() {
    // 通知上层连接断开
    this.socket = null;
    // 拒绝所有 pending 请求
    for (const [id, { reject }] of this.pending) {
      reject(new Error('connection closed'));
    }
    this.pending.clear();
  }

  async disconnect() {
    if (this.socket) {
      this.socket.end();
      this.socket = null;
    }
  }
}
```

---

### 2.4 状态管理器 `src/state-manager.js`

**职责：**

- 管理每个连接的局部状态
- 合并为统一状态树（`UnifiedWorkspace[]`）
- 提供查询和订阅通知

```javascript
export class StateManager {
  constructor() {
    // 每个连接的局部状态
    this.connectionStates = new Map();
    // 统一有序列表
    this.unified = [];
    // 页码缓存
    this.currentPage = 0;
    // 监听器
    this.listeners = [];
  }

  // 某个连接上报初始数据
  initConnection(connName, connAbbr, workspaces, agents) {
    this.connectionStates.set(connName, {
      connName,
      connAbbr,
      workspaces,
      agents,          // Map<pane_id, AgentInfo>
      connected: true,
    });
    this.rebuild();
  }

  // 某个连接的 agent 状态更新
  updateAgentStatus(connName, paneId, newStatus, customStatus, stateLabels) {
    const conn = this.connectionStates.get(connName);
    if (!conn) return;

    const agent = conn.agents.get(paneId);
    if (!agent) return;

    agent.agent_status = newStatus;
    if (customStatus !== undefined) agent.custom_status = customStatus;
    if (stateLabels !== undefined) agent.state_labels = stateLabels;

    this.rebuild();
  }

  // 重新计算统一状态树
  rebuild() {
    // 按配置顺序合并所有连接的 workspaces
    const merged = [];

    for (const [connName, conn] of this.connectionStates) {
      for (const ws of conn.workspaces) {
        const wsAgents = Array.from(conn.agents.values())
          .filter(a => a.workspace_id === ws.workspace_id);

        merged.push({
          connName: conn.connName,
          connAbbr: conn.connAbbr,
          workspace_id: ws.workspace_id,
          label: ws.label,
          number: ws.number,
          agent_status: ws.agent_status,
          tab_count: ws.tab_count,
          pane_count: ws.pane_count,
          agents: wsAgents,
          source: conn,  // 引用，方便后续事件路由
        });
      }
    }

    // 排序：先按配置顺序（connName 在 connections 数组中的 index），再按 workspace.number
    // 实际应该根据 config.connections 的顺序来排序
    // 这里简化：假设 connStates 按配置顺序 add
    this.unified = merged;
    this.notify('stateChanged');
  }

  // 分页计算
  getPage(pageIndex) {
    // 将 unified WS 列表拆成 chunks（每个 WS 拆成 ≤5 agents 的 slice）
    const chunks = [];

    for (const ws of this.unified) {
      const agentSlices = sliceArray(ws.agents, 5);
      for (const slice of agentSlices) {
        chunks.push({
          ...ws,
          agents: slice,
          isPartial: slice.length < ws.agents.length,  // 是否被截断
        });
      }
    }

    // 每页 2 个 chunk
    const pages = sliceArray(chunks, 2);
    this.totalPages = pages.length;

    return {
      page: pageIndex,
      totalPages: pages.length,
      row1: pages[pageIndex]?.[0] || null,  // chunk for row1 (K1-K5)
      row2: pages[pageIndex]?.[1] || null,  // chunk for row2 (K6-K10)
    };
  }

  // 全局统计
  computeStats() {
    const stats = { done: 0, idle: 0, working: 0, blocked: 0, unknown: 0 };
    for (const ws of this.unified) {
      for (const agent of ws.agents) {
        switch (agent.agent_status) {
          case 'done':    stats.done++; break;
          case 'idle':    stats.idle++; break;
          case 'working': stats.working++; break;
          case 'blocked': stats.blocked++; break;
          default:        stats.unknown++; break;
        }
      }
    }
    return stats;
  }

  onChange(fn) {
    this.listeners.push(fn);
  }

  notify(event, data) {
    for (const fn of this.listeners) {
      fn(event, data);
    }
  }
}

function sliceArray(arr, size) {
  const result = [];
  for (let i = 0; i < arr.length; i += size) {
    result.push(arr.slice(i, i + size));
  }
  return result;
}
```

---

### 2.5 按键映射器 `src/button-mapper.js`

**职责：**

- 从 StateManager 获取当前页数据
- 生成 14 个按键的渲染指令

```javascript
export class ButtonMapper {
  constructor(stateManager, config) {
    this.state = stateManager;
    this.config = config;  // 连接配置（用于排序）
    this.currentPage = 0;
  }

  setPage(n) {
    this.currentPage = n;
  }

  // 返回 14 个按键的渲染数据
  renderAll() {
    const pageData = this.state.getPage(this.currentPage);
    const stats = this.state.computeStats();

    return {
      keys: [
        ...this.renderAgentRow(pageData.row1, 0, 4),  // K1-K5 (row1)
        ...this.renderAgentRow(pageData.row2, 5, 9),  // K6-K10 (row2)
        ...this.renderNav(pageData, this.currentPage, this.state.totalPages),
        this.renderStats(stats),                       // K14 (long bar)
      ],
      // 用于编码器显示
      pageInfo: {
        current: this.currentPage + 1,
        total: this.state.totalPages,
      },
    };
  }

  // 渲染一行 agent（K1-K5 或 K6-K10）
  renderAgentRow(wsChunk, startIdx, endIdx) {
    const keys = [];
    const agents = wsChunk ? wsChunk.agents : [];

    for (let i = 0; i < 5; i++) {
      const agent = agents[i];
      if (agent) {
        keys.push({
          keyId: `agent_${startIdx + i}`,
          type: 'agent',
          agentType: agent.agent,          // "pi", "claude"...
          alias: agent.name || agent.agent || '',
          status: agent.agent_status,
          focused: agent.focused,
          paneId: agent.pane_id,
          connName: wsChunk.connName,
          connAbbr: wsChunk.connAbbr,
          customStatus: agent.custom_status,
        });
      } else {
        // 空键位
        keys.push({
          keyId: `agent_${startIdx + i}`,
          type: 'empty',
        });
      }
    }
    return keys;
  }

  // K11-K13: 导航
  renderNav(pageData, pageIdx, totalPages) {
    const getWsLabel = (chunk) => {
      if (!chunk) return '';
      return `${chunk.connAbbr}:${chunk.label}`;
    };

    return [
      // K11 — 上一页（取前一页的首个 WS 名）
      {
        keyId: 'nav_prev',
        type: 'navPrev',
        label: pageIdx > 0 ? getWsLabel(this.state.getPage(pageIdx - 1).row1) : '',
        enabled: pageIdx > 0,
      },
      // K12 — 当前页 WS 信息
      this.renderPageLabel(pageData, pageIdx, totalPages),
      // K13 — 下一页
      {
        keyId: 'nav_next',
        type: 'navNext',
        label: pageIdx < totalPages - 1 ? getWsLabel(this.state.getPage(pageIdx + 1).row1) : '',
        enabled: pageIdx < totalPages - 1,
      },
    ];
  }

  // K12 内容
  renderPageLabel(pageData, pageIdx, totalPages) {
    const row1 = pageData.row1;
    const row2 = pageData.row2;

    if (row1 && !row2) {
      // 单 WS 跨页
      return {
        keyId: 'nav_current',
        type: 'navCurrent',
        singleWs: true,
        label: `${row1.connAbbr}:${row1.label}`,
        sublabel: row1.isPartial ? `WS ${pageIdx + 1}/${this.countWsChunks(row1)}` : '',
      };
    } else if (row1 && row2) {
      // 双 WS 页
      return {
        keyId: 'nav_current',
        type: 'navCurrent',
        singleWs: false,
        rows: [
          { abbr: row1.connAbbr, label: row1.label },
          { abbr: row2.connAbbr, label: row2.label },
        ],
        sublabel: `Page ${pageIdx + 1}/${totalPages}`,
      };
    } else {
      return {
        keyId: 'nav_current',
        type: 'navCurrent',
        singleWs: true,
        label: 'no agents',
        sublabel: '',
      };
    }
  }

  countWsChunks(ws) {
    // 统计该 WS 拆了多少个 chunk
    let count = 0;
    for (const ws2 of this.state.unified) {
      if (ws2.workspace_id === ws.workspace_id && ws2.connName === ws.connName) {
        // 需要从原始 agents 长度算
      }
    }
    // 简化：假设 ws.agents 是原始数据，ws.isPartial 标识是否跨页
    return 1; // 临时
  }

  // K14 — 统计条
  renderStats(stats) {
    return {
      keyId: 'stats',
      type: 'stats',
      stats,
    };
  }

  prevPage() {
    if (this.currentPage > 0) {
      this.currentPage--;
      return true;
    }
    return false;
  }

  nextPage() {
    if (this.currentPage < this.state.totalPages - 1) {
      this.currentPage++;
      return true;
    }
    return false;
  }
}
```

---

### 2.6 SVG 图标渲染器 `src/icon-renderer.js`

**职责：**

- 根据按键渲染数据生成 base64 SVG
- 通过 `setBaseDataIcon` 发送到 UlanziDeck

```javascript
export class IconRenderer {
  constructor() {
    // Agent 图标路径定义（SVG path data，不依赖文件）
    this.agentIcons = getAgentIcons();
  }

  // 渲染单个 agent 键（返回 base64 SVG data URL）
  renderAgentKey(keyData) {
    const { agentType, alias, status, focused } = keyData;
    const bgColor = STATUS_COLORS[status] || '#555';
    const iconPath = this.agentIcons[agentType] || this.agentIcons['unknown'];
    const statusEmoji = STATUS_EMOJI[status] || '?';
    const borderColor = focused ? '#FFFFFF' : 'transparent';
    const borderWidth = focused ? '4' : '0';

    // SVG: 200x200 canvas
    // 背景圆角矩形（颜色=状态色）
    // Agent 图标（居中，~60% size）
    // 别名文字（底部偏上）
    // 状态图标（右下角）
    // 聚焦边框（可选）

    const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200">
      <defs>
        <clipPath id="bg"><rect width="200" height="200" rx="8"/></clipPath>
      </defs>
      <!-- 背景 -->
      <rect width="200" height="200" rx="8" fill="${bgColor}" opacity="0.85"/>
      <!-- 暗色遮罩 -->
      <rect width="200" height="200" rx="8" fill="rgba(0,0,0,0.15)"/>
      <!-- 聚焦边框 -->
      <rect x="2" y="2" width="196" height="196" rx="8"
            fill="none" stroke="${borderColor}" stroke-width="${borderWidth}"
            opacity="${focused ? 1 : 0}"/>
      <!-- Agent 图标：从 path 数据渲染 -->
      <g transform="translate(40, 30) scale(0.6)" fill="white" opacity="0.9">
        ${iconPath}
      </g>
      <!-- 别名 -->
      <text x="100" y="150" text-anchor="middle" fill="white"
            font-family="sans-serif" font-size="22" font-weight="500">
        ${escapeXml(alias)}
      </text>
      <!-- 状态 Emoji -->
      <text x="170" y="185" text-anchor="end" font-size="24">
        ${statusEmoji}
      </text>
    </svg>`;

    return 'data:image/svg+xml;base64,' + Buffer.from(svg).toString('base64');
  }

  // 渲染 K11/K13 导航键
  renderNavKey(type, label, enabled) {
    const arrow = type === 'navPrev' ? '◀' : '▶';
    const opacity = enabled ? '1' : '0.3';

    const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200">
      <rect width="200" height="200" rx="8" fill="#3a3a3a"/>
      <!-- 箭头 -->
      <text x="100" y="70" text-anchor="middle" fill="white"
            font-size="36" opacity="${opacity}">${arrow}</text>
      <!-- WS 名 -->
      <text x="100" y="130" text-anchor="middle" fill="#aaa"
            font-family="sans-serif" font-size="20" opacity="${opacity}">
        ${escapeXml(label)}
      </text>
    </svg>`;

    return 'data:image/svg+xml;base64,' + Buffer.from(svg).toString('base64');
  }

  // 渲染 K12 — WS 信息
  renderCurrentKey(labelData) {
    // singleWs: { connAbbr, wsLabel, sublabel }
    // dualWs: [{ abbr, label }, { abbr, label }], sublabel
    let svg;
    if (labelData.singleWs) {
      svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200">
        <rect width="200" height="200" rx="8" fill="#2a2a2a"/>
        <text x="100" y="90" text-anchor="middle" fill="white"
              font-family="sans-serif" font-size="22" font-weight="600">
          ${escapeXml(labelData.label)}
        </text>
        <text x="100" y="140" text-anchor="middle" fill="#888"
              font-family="sans-serif" font-size="18">
          ${escapeXml(labelData.sublabel || '')}
        </text>
      </svg>`;
    } else {
      const row1 = labelData.rows[0];
      const row2 = labelData.rows[1];
      svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200">
        <rect width="200" height="200" rx="8" fill="#2a2a2a"/>
        <text x="100" y="60" text-anchor="middle" fill="white"
              font-family="sans-serif" font-size="20">
          ${escapeXml(`${row1.abbr}:${row1.label}`)}
        </text>
        <text x="100" y="95" text-anchor="middle" fill="#aaa"
              font-family="sans-serif" font-size="18">
          · ${escapeXml(`${row2.abbr}:${row2.label}`)}
        </text>
        <text x="100" y="145" text-anchor="middle" fill="#666"
              font-family="sans-serif" font-size="16">
          ${escapeXml(labelData.sublabel || '')}
        </text>
      </svg>`;
    }
    return 'data:image/svg+xml;base64,' + Buffer.from(svg).toString('base64');
  }

  // 渲染 K14 — 全局统计（长条形）
  renderStatsKey(stats) {
    // 长条形尺寸不同，用 400x200
    const icons = [
      { emoji: '✅', count: stats.done, color: '#27AE60' },
      { emoji: '⏸', count: stats.idle, color: '#7F8C8D' },
      { emoji: '⏳', count: stats.working, color: '#F39C12' },
      { emoji: '❌', count: stats.blocked, color: '#E74C3C' },
      { emoji: '❓', count: stats.unknown, color: '#95A5A6' },
    ];

    let items = '';
    const spacing = 400 / icons.length;
    icons.forEach((item, i) => {
      const cx = spacing * i + spacing / 2;
      items += `
        <text x="${cx}" y="115" text-anchor="middle" font-size="40">${item.emoji}</text>
        <text x="${cx}" y="165" text-anchor="middle" fill="white"
              font-family="sans-serif" font-size="32" font-weight="700">
          ${item.count}
        </text>`;
    });

    const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 400 200">
      <rect width="400" height="200" rx="8" fill="#222" opacity="0.9"/>
      ${items}
    </svg>`;

    return 'data:image/svg+xml;base64,' + Buffer.from(svg).toString('base64');
  }

  // 空键
  renderEmptyKey() {
    const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200">
      <rect width="200" height="200" rx="8" fill="#2a2a2a" opacity="0.3"/>
    </svg>`;
    return 'data:image/svg+xml;base64,' + Buffer.from(svg).toString('base64');
  }
}

// 状态颜色映射
const STATUS_COLORS = {
  done: '#27AE60',
  idle: '#7F8C8D',
  working: '#F39C12',
  blocked: '#E74C3C',
  unknown: '#95A5A6',
};

const STATUS_EMOJI = {
  done: '✅',
  idle: '⏸',
  working: '⏳',
  blocked: '❌',
  unknown: '❓',
};

function escapeXml(str) {
  if (!str) return '';
  return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;').replace(/'/g, '&apos;');
}

// Agent 图标 SVG path 数据（简化 line-art 风格）
function getAgentIcons() {
  return {
    pi: `<path d="M100 40 L100 160 M60 160 L140 160" 
              stroke="white" stroke-width="10" fill="none"/>`,
    claude: `<ellipse cx="100" cy="80" rx="30" ry="25" fill="none" stroke="white" stroke-width="8"/>
             <path d="M70 90 Q85 140 100 160 Q115 140 130 90" fill="none" stroke="white" stroke-width="8"/>
             <path d="M60 60 Q80 20 100 40 Q120 20 140 60" fill="none" stroke="white" stroke-width="6"/>`,
    cursor: `<path d="M60 40 L60 170 L120 120 L160 160" fill="none" stroke="white" stroke-width="10" stroke-linejoin="round"/>`,
    gemini: `<path d="M100 30 Q130 80 170 100 Q130 120 100 170 Q70 120 30 100 Q70 80 100 30Z" 
                  fill="none" stroke="white" stroke-width="8"/>`,
    copilot: `<path d="M60 100 Q60 50 100 40 Q140 50 140 100 Q140 150 100 160 Q60 150 60 100Z"
                    fill="none" stroke="white" stroke-width="8"/>
              <path d="M80 85 L100 105 L120 85" fill="none" stroke="white" stroke-width="6" stroke-linecap="round"/>`,
    claude: `<path d="M70 50 L100 120 L130 50" fill="none" stroke="white" stroke-width="8" stroke-linecap="round"/>
             <circle cx="100" cy="140" r="15" fill="none" stroke="white" stroke-width="6"/>`,
    cline: `<path d="M50 140 L90 60 L130 140 L170 60" fill="none" stroke="white" stroke-width="8" stroke-linejoin="round"/>`,
    codex: `<path d="M50 60 L90 60 L90 140 L50 140" fill="none" stroke="white" stroke-width="6" rx="4"/>
            <path d="M110 80 L130 100 L110 120" fill="none" stroke="white" stroke-width="6" stroke-linecap="round"/>
            <path d="M140 80 L160 100 L140 120" fill="none" stroke="white" stroke-width="6" stroke-linecap="round"/>`,
    devin: `<path d="M60 80 Q100 40 140 80" fill="none" stroke="white" stroke-width="8" stroke-linecap="round"/>
            <path d="M80 100 L80 160 M120 100 L120 160" fill="none" stroke="white" stroke-width="8"/>
            <rect x="60" y="140" width="80" height="6" rx="3" fill="white"/>`,
    grok: `<ellipse cx="100" cy="90" rx="35" ry="25" fill="none" stroke="white" stroke-width="8"/>
           <circle cx="80" cy="85" r="6" fill="white"/>
           <circle cx="120" cy="85" r="6" fill="white"/>
           <path d="M75 105 Q100 125 125 105" fill="none" stroke="white" stroke-width="5" stroke-linecap="round"/>`,
    kimi: `<path d="M100 30 Q50 90 100 150" fill="none" stroke="white" stroke-width="10" stroke-linecap="round"/>
           <path d="M100 30 Q150 90 100 150" fill="none" stroke="white" stroke-width="10" stroke-linecap="round"/>
           <circle cx="100" cy="40" r="8" fill="white"/>`,
    kilo: `<text x="100" y="130" text-anchor="middle" fill="white" font-size="100" font-weight="bold">K</text>`,
    kiro: `<path d="M60 30 L140 100 L80 100 L140 170" fill="none" stroke="white" stroke-width="10" stroke-linejoin="round"/>`,
    opencode: `<text x="100" y="135" text-anchor="middle" fill="white" font-size="100" font-weight="bold">{</text>`,
    qodercli: `<text x="100" y="130" text-anchor="middle" fill="white" font-size="80" font-weight="bold">&gt;_</text>`,
    amp: `<circle cx="100" cy="100" r="50" fill="none" stroke="white" stroke-width="8"/>
          <path d="M80 70 L120 100 L80 100 L120 130" fill="none" stroke="white" stroke-width="8" stroke-linejoin="round"/>`,
    antigravity: `<path d="M100 160 L100 60 M70 90 L100 60 L130 90" fill="none" stroke="white" stroke-width="10" stroke-linecap="round" stroke-linejoin="round"/>`,
    droid: `<rect x="65" y="50" width="70" height="70" rx="15" fill="none" stroke="white" stroke-width="8"/>
            <circle cx="80" cy="85" r="5" fill="white"/>
            <circle cx="120" cy="85" r="5" fill="white"/>
            <rect x="85" y="120" width="30" height="30" rx="5" fill="none" stroke="white" stroke-width="6"/>
            <line x1="75" y1="115" x2="70" y2="145" stroke="white" stroke-width="6" stroke-linecap="round"/>
            <line x1="125" y1="115" x2="130" y2="145" stroke="white" stroke-width="6" stroke-linecap="round"/>`,
    hermes: `<path d="M50 60 L100 110 L150 60" fill="none" stroke="white" stroke-width="8" stroke-linecap="round" stroke-linejoin="round"/>
             <rect x="45" y="60" width="110" height="80" rx="8" fill="none" stroke="white" stroke-width="8"/>
             <path d="M50 100 L95 125 L100 130 L105 125 L150 100" fill="none" stroke="white" stroke-width="6" stroke-linejoin="round"/>`,
    unknown: `<circle cx="100" cy="80" r="30" fill="none" stroke="white" stroke-width="8"/>
              <text x="100" y="145" text-anchor="middle" fill="white" font-size="50">?</text>`,
  };
}
```

---

### 2.7 Deck 客户端 `src/deck-client.js`

**职责：**

- WebSocket 连接到 UlanziDeck 上位机
- 接收按键事件（keydown/keyup）
- 发送按键渲染（state 命令）

```javascript
import WebSocket from 'ws';

const ULANZI_PORT = 3906;
const ULANZI_ADDR = '127.0.0.1';
const PLUGIN_UUID = 'com.ulanzi.herdr.agentview';

export class DeckClient {
  constructor(onKeyDown, onKeyUp) {
    this.ws = null;
    this.connected = false;
    this.key = '';     // 分配给我们的键位 ID（由上位机分配）
    this.actionid = '';
    this.onKeyDown = onKeyDown || (() => {});
    this.onKeyUp = onKeyUp || (() => {});
  }

  async connect() {
    return new Promise((resolve, reject) => {
      // 从 argv 获取端口和地址，和 SDK 一致的逻辑
      const [,,, address, port] = process.argv;  // node app.js 127.0.0.1 3906
      const wsAddr = address || ULANZI_ADDR;
      const wsPort = port || ULANZI_PORT;

      this.ws = new WebSocket(`ws://${wsAddr}:${wsPort}`);

      this.ws.on('open', () => {
        // 发送 connected 事件注册插件
        this.send('connected', { uuid: PLUGIN_UUID });
        this.connected = true;
        resolve();
      });

      this.ws.on('message', (data) => {
        try {
          const msg = JSON.parse(data.toString());
          this.handleMessage(msg);
        } catch (e) {
          console.error('deck: parse error', e);
        }
      });

      this.ws.on('close', () => {
        this.connected = false;
        // 自动重连
        setTimeout(() => this.connect(), 1000);
      });

      this.ws.on('error', reject);
    });
  }

  handleMessage(msg) {
    // 忽略有 code 属性的内部系统消息
    if (msg.code !== undefined && msg.cmd !== 'keydown' && msg.cmd !== 'keyup') return;

    const cmd = msg.cmd;
    const context = msg.context;  // SDK 自动拼接的 uuid___key___actionid

    switch (cmd) {
      case 'connected':
        // 存储分配的 key/actionid
        this.key = msg.key || '';
        this.actionid = msg.actionid || '';
        break;

      case 'keydown':
        this.onKeyDown(msg);
        break;

      case 'keyup':
        this.onKeyUp(msg);
        break;

      case 'run':
        // 按键首次加载时触发
        break;

      default:
        break;
    }
  }

  // 更新某个按键的图标
  setKeyImage(key, base64SvgData) {
    // 使用 setBaseDataIcon 等效协议
    // 注：key 在这里是物理键位（如 "0_0"），不是 SDK 的 context
    // 我们需要确保上位机知道要更新哪个物理按键
    
    // UlanziDeck 的 state 命令格式：
    // { cmd: "state", param: { statelist: [{ uuid, key, actionid, type, data }] } }
    this.send('state', {
      param: {
        statelist: [{
          uuid: PLUGIN_UUID,
          key: key,            // 物理键位 ID: "0_0", "1_2", "3_3"
          actionid: this.actionid,
          type: 1,             // 1 = base64 图片
          data: base64SvgData,
          showtext: false,
        }],
      },
    });
  }

  setKeyImageWithText(key, base64SvgData, text) {
    this.send('state', {
      param: {
        statelist: [{
          uuid: PLUGIN_UUID,
          key: key,
          actionid: this.actionid,
          type: 1,
          data: base64SvgData,
          textData: text || '',
          showtext: !!text,
        }],
      },
    });
  }

  send(cmd, params) {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) return;
    this.ws.send(JSON.stringify({
      cmd,
      uuid: PLUGIN_UUID,
      key: this.key,
      actionid: this.actionid,
      ...params,
    }));
  }

  async disconnect() {
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }
}
```

---

### 2.8 Agent 图标库 `src/icons.js`

**职责：** Agent 图标 SVG path 定义（已在 icon-renderer 内联，此为独立导出文件供复用）

```javascript
// 导出 agentIcons 映射表
// (内容与 icon-renderer.js 中 getAgentIcons() 一致)
export { getAgentIcons } from './icon-renderer.js';
```

---

### 2.9 入口 `src/index.js`

**职责：** 整合所有模块，事件循环

```javascript
import { loadConfig, configPath } from './config.js';
import { ConnectionManager } from './connection-manager.js';
import { StateManager } from './state-manager.js';
import { ButtonMapper } from './button-mapper.js';
import { IconRenderer } from './icon-renderer.js';
import { DeckClient } from './deck-client.js';

class HerdrDeckApp {
  constructor() {
    this.config = null;
    this.connManager = new ConnectionManager();
    this.stateManager = new StateManager();
    this.iconRenderer = new IconRenderer();
    this.buttonMapper = null;
    this.deckClient = null;
    this.currentPage = 0;
  }

  async start() {
    // 1. 加载配置
    this.config = loadConfig();
    console.log(`config loaded: ${this.config.connections.length} connections`);

    // 2. 启动所有连接（local + SSH tunnels）
    await this.connManager.startAll(this.config);

    // 3. 初始化状态管理器
    this.stateManager = new StateManager();

    // 4. 从每个连接获取初始数据
    for (const conn of this.connManager.getAll()) {
      const client = conn.client;
      try {
        const [workspaces, agents] = await Promise.all([
          client.listWorkspaces(),
          client.listAgents(),
        ]);

        // 构建 Agent Map
        const agentMap = new Map();
        for (const agent of agents) {
          agentMap.set(agent.pane_id, agent);
        }

        this.stateManager.initConnection(conn.name, conn.abbr, workspaces, agentMap);
      } catch (err) {
        console.error(`failed to init ${conn.name}:`, err.message);
      }
    }

    // 5. 创建按键映射器
    this.buttonMapper = new ButtonMapper(this.stateManager, this.config);

    // 6. 连接 UlanziDeck
    this.deckClient = new DeckClient(
      (msg) => this.handleKeyDown(msg),
      (msg) => this.handleKeyUp(msg)
    );
    await this.deckClient.connect();

    // 7. 首次渲染所有按键
    this.renderAll();

    // 8. 注册事件订阅（每个连接独立）
    for (const conn of this.connManager.getAll()) {
      const client = conn.client;

      // 订阅所有需要的事件
      await client.subscribe({
        subscriptions: [
          { type: 'pane.agent_status_changed' },
          { type: 'pane.agent_detected' },
          { type: 'workspace.created' },
          { type: 'workspace.closed' },
          { type: 'workspace.focused' },
          { type: 'workspace.renamed' },
        ],
      });

      // 事件处理器（每个连接独立）
      client.onEvent((msg) => this.handleHerdrEvent(conn.name, msg));
    }

    // 9. 状态变更自动刷新
    this.stateManager.onChange(() => this.renderAll());
  }

  // Herdr 事件处理
  handleHerdrEvent(connName, msg) {
    const event = msg.event;
    const data = msg.data;

    switch (event) {
      case 'pane_agent_status_changed':
        this.stateManager.updateAgentStatus(
          connName,
          data.pane_id,
          data.agent_status,
          data.custom_status,
          data.state_labels
        );
        break;

      case 'pane_agent_detected':
        // Agent 类型变化（如从普通终端变为 AI agent）
        this.refreshConnectionData(connName);
        break;

      case 'workspace_focused':
        // 其他 WS 聚焦时，刷新当前页回到该 WS
        this.refreshConnectionData(connName);
        break;

      case 'workspace_created':
      case 'workspace_closed':
      case 'workspace_renamed':
        this.refreshConnectionData(connName);
        break;
    }
  }

  // 刷新某个连接的完整状态（workspace 增删时）
  async refreshConnectionData(connName) {
    const conn = this.connManager.getConnectorByName(connName);
    if (!conn) return;

    try {
      const [workspaces, agents] = await Promise.all([
        conn.client.listWorkspaces(),
        conn.client.listAgents(),
      ]);

      const agentMap = new Map();
      for (const agent of agents) {
        agentMap.set(agent.pane_id, agent);
      }

      this.stateManager.initConnection(connName, conn.abbr, workspaces, agentMap);
    } catch (err) {
      console.error(`refresh ${connName} failed:`, err.message);
    }
  }

  // 按键按下处理
  handleKeyDown(msg) {
    // msg.context 格式: uuid___key___actionid
    // msg.key 是物理键位（如 "0_0"）
    const physicalKey = msg.key;
    const pageData = this.buttonMapper.renderAll();

    // 根据物理键位判断功能
    // K1-K10 (key_0_0 to key_2_1): agent 聚焦
    // K11 (key_3_0): 上一页
    // K13 (key_3_2): 下一页
    // K14 (key_3_3): 无交互（仅统计展示）

    if (physicalKey === '3_0') {
      // K11 — 上一页
      if (this.buttonMapper.prevPage()) {
        this.renderAll();
      }
    } else if (physicalKey === '3_2') {
      // K13 — 下一页
      if (this.buttonMapper.nextPage()) {
        this.renderAll();
      }
    } else {
      // Agent 键 — 聚焦到对应 agent
      const keyIndex = this.getKeyIndex(physicalKey);
      if (keyIndex >= 0 && keyIndex < 10) {
        const agentKey = pageData.keys[keyIndex];
        if (agentKey && agentKey.type === 'agent') {
          this.focusAgent(agentKey.connName, agentKey.paneId);
        }
      }
    }
  }

  handleKeyUp(msg) {
    // 暂不需要处理
  }

  // 聚焦到特定 agent
  async focusAgent(connName, paneId) {
    const conn = this.connManager.getConnectorByName(connName);
    if (!conn) return;

    try {
      await conn.client.focusAgent(paneId);
    } catch (err) {
      console.error(`focus agent failed: ${connName}/${paneId}`, err.message);
    }
  }

  // 物理键位 Key ID → 索引
  // D200X 布局：
  //   0_0 0_1 0_2 0_3 0_4 → K1-K5   (index 0-4)
  //   1_0 1_1 1_2 1_3 1_4 → K6-K10 (index 5-9)
  //   2_0 2_1 2_2 2_3     → K11-K14 (index 10-13)
  getKeyIndex(physicalKey) {
    const map = {
      '0_0': 0, '0_1': 1, '0_2': 2, '0_3': 3, '0_4': 4,
      '1_0': 5, '1_1': 6, '1_2': 7, '1_3': 8, '1_4': 9,
      '2_0': 10, '2_1': 11, '2_2': 12, '2_3': 13,
    };
    return map[physicalKey] ?? -1;
  }

  // 渲染所有按键到 Deck
  renderAll() {
    const pageData = this.buttonMapper.renderAll();

    for (const keyData of pageData.keys) {
      let base64Svg;

      switch (keyData.type) {
        case 'agent':
          base64Svg = this.iconRenderer.renderAgentKey(keyData);
          break;
        case 'navPrev':
        case 'navNext':
          base64Svg = this.iconRenderer.renderNavKey(keyData.type, keyData.label, keyData.enabled);
          break;
        case 'navCurrent':
          base64Svg = this.iconRenderer.renderCurrentKey(keyData);
          break;
        case 'stats':
          base64Svg = this.iconRenderer.renderStatsKey(keyData.stats);
          break;
        default:
          base64Svg = this.iconRenderer.renderEmptyKey();
      }

      // 物理键位映射: agent 键根据 keyId 推断物理位置
      const physicalKey = this.getPhysicalKey(keyData.keyId, keyData.type);
      this.deckClient.setKeyImage(physicalKey, base64Svg);
    }
  }

  // 根据 keyId 和 type 推断物理键位
  getPhysicalKey(keyId, type) {
    // agent_0 ~ agent_4 → 0_0 ~ 0_4
    // agent_5 ~ agent_9 → 1_0 ~ 1_4
    // nav_prev → 2_0
    // nav_current → 2_1
    // nav_next → 2_2
    // stats → 2_3 (长条)
    const map = {
      'nav_prev': '2_0',
      'nav_current': '2_1',
      'nav_next': '2_2',
      'stats': '2_3',
    };

    if (map[keyId]) return map[keyId];

    // agent 键
    if (keyId.startsWith('agent_')) {
      const idx = parseInt(keyId.split('_')[1]);
      if (idx >= 0 && idx <= 4) return `0_${idx}`;
      if (idx >= 5 && idx <= 9) return `1_${idx - 5}`;
    }

    return '0_0'; // fallback
  }

  async stop() {
    await this.deckClient.disconnect();
    for (const conn of this.connManager.getAll()) {
      await conn.client.disconnect();
    }
    await this.connManager.stopAll();
  }
}

// 启动
const app = new HerdrDeckApp();
app.start().catch(err => {
  console.error('fatal:', err);
  process.exit(1);
});
```

---

## 三、测试策略

| 层次 | 测试内容 | 工具 |
|------|---------|------|
| HerdrClient 单体 | mock socket，验证请求/响应/重连 | Node.js `net` + 临时 socket |
| Config 解析 | 加载有效/无效配置文件 | 单元测试 |
| StateManager | 多连接数据合并、分页逻辑 | 单元测试 |
| ButtonMapper | 各种 WS/Agent 组合的按键输出 | 单元测试 |
| IconRenderer | SVG 生成（尺寸、颜色、文本） | 快照测试 |
| 集成测试 | UlanziDeckSimulator 模拟器 | WebSocket + 视觉确认 |
| E2E | 真机 + 实际 herdr | herdr 运行态 |

---

## 四、实现顺序

```
Step 1: src/herdr-client.js        ← 可独立测试，mock herdr socket
Step 2: src/config.js              ← 简单文件读取
Step 3: src/connection-manager.js  ← 依赖 config
Step 4: src/state-manager.js       ← 可独立测试
Step 5: src/icon-renderer.js       ← SVG 生成，可独立验证
Step 6: src/button-mapper.js       ← 依赖 state-manager, icon-renderer
Step 7: src/deck-client.js         ← 依赖 UlanziDeck Simulator 测试
Step 8: src/index.js               ← 整合全部
Step 9: manifest.json + 插件打包   ← 部署
```

---

## 五、关键风险点

| 风险 | 影响 | 缓解 |
|------|------|------|
| SSH tunnel 端口解析不稳定 | 连接失败 | 增加 TCP 连接健康检查，失败回退重试 |
| Unix socket 并发写 | 消息交错 | 每个 HerdrClient 串行写，等待 response |
| UlanziDeck 按键 key 映射不对 | 图标渲染到错误键位 | 用模拟器确认各 D200X 物理键位 ID |
| 事件订阅推送频率高 | 过度刷新 | 状态变化 200ms 内节流，批量刷新 |
| `~/.ssh/config` 中 host 别名找不到 | SSH 失败 | 启动时提前 `ssh -G <host>` 验证 |
