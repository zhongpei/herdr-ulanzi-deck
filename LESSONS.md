# 开发坑点与教训

> herdr-deck 插件开发过程中踩过的坑及解决思路

---

## 1. 头号教训：不要猜 API，去读真实代码

**问题：** 整个开发过程中，我们花了大量时间"盲猜"UlanziDeck WebSocket 协议的细节——消息格式、必填字段、连接流程。猜了四五轮都不对。

**原因：** 我们没有从一开始就认真读 SDK 的源码。SDK 就在 `UlanziDeckPlugin-SDK/demo/com.ulanzi.APIRequest.ulanziPlugin/plugin/actions/ulanzi-api/libs/ulanzideckApi.js` 里——这是官方 Node.js SDK，**80 行代码就说明了全部协议**。

**教训：** 当你要对接一个已有系统的 API 时：

1. **找到官方 SDK/客户端库**，直接读源码，不要依赖文档
2. 文档可能过时、不完整、有歧义。代码是唯一真相
3. SDK 源码里的 `send()` 方法和 `connect()` 方法就是你的"协议说明书"

**我们在 SDK 里本可以直接看到的答案：**

```javascript
// connect() 发送的是 code:0 + cmd:connected，而不是裸 cmd
{ code: 0, cmd: 'connected', uuid: this.uuid }

// send() 的 outer 层 key/actionid 来自 this.key 和 this.actionid
// 它们由 onmessage 中的 data.uuid == this.uuid 匹配逻辑设置
```

如果我们一开始就读了这 80 行代码，至少省掉 3 轮无效尝试。

---

## 2. 协议调试：日志就是一切

**问题：** `state` 命令发出去后真机没反应，但我们不知道服务端是否收到了、是否拒绝了、为什么拒绝。

**原因：** 我们的 `handleMessage()` 里有一个**致命的消息过滤器**：

```javascript
if (msg.code !== undefined && msg.cmd !== "keydown" && msg.cmd !== "keyup") {
    return; // ← 所有带 code 字段的回复都被静默丢弃！
}
```

服务端对 `state` 命令的回复是：

```json
{"cmd":"state","cmdType":"NOTIFY","code":0,"param":null}
```

这个回复里 `code !== undefined` 且 `cmd !== "keydown"`，所以被我们直接丢了。我们**完全不知道服务端已经成功处理了请求**，于是进入了"盲猜→改→试"的死循环。

**教训：**

1. 调试协议时，**永远先打全量日志**——不要过滤任何消息
2. 加一个全局 `RECV/SEND` 日志开关，能看到每一个字节的往来
3. 确认消息被服务端接收和处理，再排查硬件端问题

**我们的修复：**

```javascript
// 去掉激进过滤，全部打日志
this.ws.on("message", (raw) => {
    const msg = JSON.parse(raw.toString());
    console.log("[deck] RECV:", JSON.stringify(msg)); // 全量
    this.handleMessage(msg);
});
```

---

## 3. 键位格式反转：先确认坐标系

**问题：** 发送 `state` 命令到 `0_0` `0_1` `0_2` `0_3` `0_4`（D200X 不存在这些键），真机永远没反应。

**原因：** 模拟器的 `keys.js` 用了 `col_row` 格式（`0_0` = col 0 row 0），但我们以为它是 `row_col`。我们发到了 `0_3` 和 `0_4`——这在 D200X 上不存在（只有 3 行）。

**教训：**

1. **不要假设坐标系格式**——模拟器的 3×3 看起来像是 `row_col`，但验证发现是 `col_row`
2. **从设备配置文件确认键位**——`config/device_type_source.json` 里明确写了 D200X：

   ```json
   "Columns": 5, "Rows": 3,
   "LargeItem": { "3_2": [2, 1] }
   ```

3. `3_2` 表示 col 3, row 2——这确认了 `col_row` 格式

---

## 4. `code: 0` 是必填字段

**问题：** 连接消息格式错误导致服务端静默忽略。

**原因：** SDK 的 `connect()` 发送 `{ code: 0, cmd: "connected", uuid: "..." }`，但我们只发了 `{ cmd: "connected", uuid: "..." }`。服务端检查 `code` 字段，没有就丢弃整个连接。

**教训：** 协议中的"仪式性"字段（如 `code: 0`）通常不是可选的。读 SDK 源码时要逐字段对比。

---

## 5. Profile 机制：插件应该自建配置，不依赖用户拖拽

**问题：** 期望用户把 Action 拖拽到按键上才能工作——这在开发调试阶段可行，但交付时不可接受。

**原因：** 我们最初设计依赖 `add` 事件来获取 key→actionid 映射。但 `add` 事件只在用户手动拖拽时触发，Profile 加载时不会触发。

**教训：**

1. 阅读 `ProfilesV2/` 目录结构，理解 Profile 文件格式
2. 插件启动时**自己创建 Profile**，把所有 14 个键分配给自己的 Action
3. 从自己创建的 Profile 文件中直接读取 key→actionid 映射，不依赖 `add` 事件
4. 用户只需要在 UlanziStudio 里切换 Profile 即可

**关键代码模式：**

```javascript
// 创建 profile → 写入 manifest + page manifest → 读取 keyActionMap
const pm = new ProfileManager();
pm.ensure("D200X-device-uuid");
const keyActions = pm.getKeyActionMap();
deckClient.seedKeyActions(keyActions);
```

---

## 6. SVG 不是 D200X 固件支持的格式

**问题：** 发送 `data:image/svg+xml;base64,...` 时真机不显示（黑色）。但模拟器里正常。

**原因：** D200X 的 LCD 按键固件不接受 SVG 格式的 base64 数据，只接受 PNG。模拟器在浏览器里渲染（支持 SVG），所以看不出问题。

**教训：**

1. 模拟器 ≠ 真机。模拟器是浏览器，真机是嵌入式固件
2. 嵌入式设备的图片渲染能力有限——优先用 PNG
3. 用 `sharp` 库做 SVG→PNG 转换，输出 196×196（D200X 按键分辨率）

```javascript
const pngBuffer = await sharp(svgBuffer)
    .resize(196, 196, { fit: "contain", background: { r: 0, g: 0, b: 0, alpha: 0 } })
    .png()
    .toBuffer();
```

---

## 7. 流程总结：正确的开发路径

```
1. 读 SDK 源码（不是文档！）
   → 确认 connect/state 消息格式
   → 确认必填字段 (code: 0)
   
2. 架构设计
   → Profile 创建（自包含，不依赖用户操作）
   → 模拟器验证（消息格式、日志输出）
   
3. 真机测试
   → 开全量日志（RECV/SEND）
   → 先验证服务端是否回复（code: 0）
   → 再确认硬件渲染（PNG vs SVG）
   
4. 逐步迭代
   → 每次只改一个变量
   → 看日志确认效果再继续
```

---

## 8. 检查清单

| 步骤 | 检查项 | 当初漏了？ |
|------|--------|-----------|
| 1 | 读 SDK `ulanzideckApi.js` | ✅ |
| 2 | 确认 `connected` 有 `code: 0` | ❌ 猜了 4 轮 |
| 3 | 键位格式 `col_row` | ❌ 猜反了 |
| 4 | 全量 RECV 日志 | ❌ 过滤器挡住了 |
| 5 | 真机 PNG VS SVG | ❌ 模拟器没暴露 |
| 6 | Profile 自建 | ❌ 依赖了 `add` 事件 |
