# Hardware Debugging Guide

> Ulanzi D200X integration — WebSocket protocol, key mapping, and common pitfalls.
> Platform: macOS, Linux

---

## 1. Read the SDK Source, Not the Docs

The UlanziDeck WebSocket protocol is documented in the official Node.js SDK:

```
UlanziDeckPlugin-SDK/demo/com.ulanzi.APIRequest.ulanziPlugin/
  plugin/actions/ulanzi-api/libs/ulanzideckApi.js
```

80 lines that describe every message format and connection flow. The SDK's `send()` and `connect()` are the protocol specification.

**Rule:** When integrating with an existing system, find the official SDK source first. Documentation is always secondary.

---

## 2. Log Everything During Debugging

When a `state` command fails silently:

1. **Enable debug logging** on the deck process: `./build/herdr-deck --debug`
2. **Do not filter messages.** The deckclient's `handleMessage` was discarding responses with a `code` field — the server's `{"cmd":"state","code":0}` reply was silently dropped.
3. Confirm the server received and responded before checking hardware.

---

## 3. Key Coordinate Format: `col_row`

D200X keys use `col_row` format (column first, then row). From the device config:

```json
"Columns": 5, "Rows": 3,
"LargeItem": { "3_2": [2, 1] }
```

`3_2` = col 3, row 2. This matches the key map in `deck/cmd/herdr-deck/main.go`:

```go
var keyMap = map[string]int{
    "0_0": 0, "1_0": 1, "2_0": 2, "3_0": 3, "4_0": 4,
    "0_1": 5, "1_1": 6, "2_1": 7, "3_1": 8, "4_1": 9,
    "0_2": 10, "1_2": 11, "2_2": 12, "3_2": 13,
}
```

**Never send to non-existent keys** like `0_3` or `0_4` (D200X has 5 cols × 3 rows).

---

## 4. `code: 0` Is Required

The `connected` message must include `"code": 0`. The SDK sends:

```javascript
{ code: 0, cmd: "connected", uuid: "...", ... }
```

Omitting `code` causes the server to silently ignore the connection. This is handled in `deck/internal/deckclient/client.go` in the `Connect()` method.

---

## 5. Profile Must Be Self-Created

The `add` event only fires when a user manually drags an action onto a key. Profile load does NOT trigger it.

The deck auto-creates all 14 key actions in `deck/internal/profile/manager.go`:

- `Ensure(deviceUUID)` creates a fresh profile
- `GetKeyActionMap()` reads the key→actionID mapping from the profile file
- `SeedKeyActions()` pre-populates the deckclient's key map

This ensures the plugin works without any user configuration.

---

## 6. D200X Requires PNG, Not SVG

The D200X LCD firmware only accepts PNG images. SVG renders fine in the simulator (browser-based) but produces a black screen on the real device.

The deck pipeline: SVG → PNG via `tdewolff/canvas` → base64 → WebSocket.

This happens in `deck/internal/deckclient/draw.go` (`svgToPNG`) and `client.go` (`SetKeyImage`).

---

## 7. Recommended Development Flow

```
1. Read SDK source (not docs)
   → Confirm connect/state message format
   → Confirm required fields (code: 0)

2. Architecture
   → Profile auto-create (self-contained, no user setup)
   → Verify message format + logging in simulator

3. Hardware test
   → Enable full debug logging (RECV/SEND)
   → Verify server response (code: 0)
   → Confirm hardware rendering (PNG, not SVG)

4. Iterate
   → One variable change at a time
   → Check logs before assuming hardware issue
```

---

## 8. Quick Checklist

| Step | Check |
|------|-------|
| 1 | Read SDK `ulanzideckApi.js` |
| 2 | `connected` includes `code: 0` |
| 3 | Key format `col_row` (not `row_col`) |
| 4 | Full RECV log enabled |
| 5 | Device gets PNG, not SVG |
| 6 | Profile self-created |
