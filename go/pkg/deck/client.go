// Package deck implements the WebSocket client to UlanziDeck host (port 3906).
// Mirrors src/deck-client.js
//
// The plugin registers as a "main service" via WebSocket, then sends 14 key
// state commands to render images on the D200X buttons.
package deck

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/gorilla/websocket"
)

const (
	PluginUUID  = "com.ulanzi.herdr.agentview"
	ActionUUID  = "com.ulanzi.herdr.agentview.monitor"
	DefaultPort = 3906
	DefaultAddr = "127.0.0.1"
)

// Message types from UlanziDeck
type Message struct {
	Cmd      string          `json:"cmd,omitempty"`
	CmdType  string          `json:"cmdType,omitempty"`
	Code     *int            `json:"code"`
	Key      string          `json:"key"`
	ActionID string          `json:"actionid"`
	UUID     string          `json:"uuid,omitempty"`
	Param    json.RawMessage `json:"param,omitempty"`
}

// AddHandler is called when a key-action mapping is received.
type AddHandler func(key, actionID string)

// KeyDownHandler is called when a key is pressed.
type KeyDownHandler func(msg Message)

// Client manages the WebSocket connection to UlanziDeck.
type Client struct {
	ws         *websocket.Conn
	connected  bool
	address    string
	port       int
	keyActions map[string]string // key → actionID
	keyOrder   []string          // insertion order (JS Map preserves it)
	readyKeys  bool
	logAll     bool

	mu      sync.RWMutex
	writeMu sync.Mutex

	onAdd     AddHandler
	onKeyDown KeyDownHandler
}

// New creates a DeckClient.
func New(onAdd AddHandler, onKeyDown KeyDownHandler) *Client {
	c := &Client{
		address:       DefaultAddr,
		port:          DefaultPort,
		keyActions:    make(map[string]string),

		onAdd:         onAdd,
		onKeyDown:     onKeyDown,
		logAll:        true,
	}
	if c.onAdd == nil {
		c.onAdd = func(_, _ string) {}
	}
	if c.onKeyDown == nil {
		c.onKeyDown = func(_ Message) {}
	}
	return c
}

// Connect establishes the WebSocket connection.
func (c *Client) Connect() error {
	args := os.Args[1:]
	if len(args) >= 2 {
		c.address = args[0]
		fmt.Sscanf(args[1], "%d", &c.port)
	}

	addr := fmt.Sprintf("ws://%s:%d", c.address, c.port)
	ws, _, err := websocket.DefaultDialer.Dial(addr, nil)
	if err != nil {
		return fmt.Errorf("dial %s: %w", addr, err)
	}

	c.mu.Lock()
	c.ws = ws
	c.connected = true
	c.mu.Unlock()

	// Connect as plugin UUID for keydown events
	code0 := 0
	c.sendJSON(Message{
		Code:     &code0,
		Cmd:      "connected",
		UUID:     PluginUUID,
		Key:      "",
		ActionID: "",
	})

	log.Printf("[deck] connected as main service: %s", PluginUUID)
	return nil
}

// ReadPump reads messages from WebSocket and dispatches to handlers.
// Must be called in a goroutine after Connect().
func (c *Client) ReadPump() {
	c.mu.RLock()
	ws := c.ws
	c.mu.RUnlock()
	if ws == nil {
		log.Println("[deck] ReadPump: ws is nil")
		return
	}

	for {
		_, raw, err := ws.ReadMessage()
		if err != nil {
			log.Printf("[deck] read error: %v", err)
			c.mu.Lock()
			c.connected = false
			c.mu.Unlock()
			return
		}

		var msg Message
		if err := json.Unmarshal(raw, &msg); err != nil {
			log.Printf("[deck] parse error: %v", err)
			continue
		}

		if c.logAll {
			log.Printf("[deck] RECV: %s", truncateJSON(raw, 120))
		}

		c.handleMessage(msg)
	}
}

func (c *Client) handleMessage(msg Message) {
	// Per SDK: only ack messages that have NO code field (bare events like add/keydown).
	// Messages WITH code (like state NOTIFY) are the deck's responses — do NOT ack or we loop.
	// SDK check: typeof data.code === "undefined" → this is a real event, not a response.
	if msg.Cmd != "" && msg.Cmd != "connected" && msg.Code == nil {
		c.sendAck(msg)
	}

	switch msg.Cmd {
	case "connected":
		log.Printf("[deck] connected: key=%s actionid=%s", msg.Key, msg.ActionID)

	case "add":
		if msg.Key != "" && msg.ActionID != "" {
			c.mu.Lock()
			c.keyActions[msg.Key] = msg.ActionID
			ready := len(c.keyActions)
			c.mu.Unlock()
			log.Printf("[deck] add: key=%s actionid=%s (total: %d)", msg.Key, msg.ActionID, ready)
			c.readyKeys = true
			c.onAdd(msg.Key, msg.ActionID)
		}

	case "clear":
		if msg.Param != nil {
			var items []struct {
				Key string `json:"key"`
			}
			if err := json.Unmarshal(msg.Param, &items); err == nil {
				for _, item := range items {
					c.mu.Lock()
					delete(c.keyActions, item.Key)
					c.mu.Unlock()
					log.Printf("[deck] clear: key=%s", item.Key)
				}
			}
		}

	case "keydown":
		c.onKeyDown(msg)

	case "keyup", "run", "setactive":
		// no-op
	}
}

// sendAck sends a required acknowledgment response per the Ulanzi SDK protocol.
// The host expects a response for every message sent to the plugin.
func (c *Client) sendAck(msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return
	}
	raw["code"] = 0

	payload := map[string]any{
		"cmd":      msg.Cmd,
		"uuid":     PluginUUID,
		"key":      msg.Key,
		"actionid": msg.ActionID,
	}
	// Echo back all original fields
	for k, v := range raw {
		payload[k] = v
	}

	ackData, _ := json.Marshal(payload)
	c.sendRaw(ackData)
}

// SeedKeyActions pre-populates key→actionid map from profile (fallback).
func (c *Client) SeedKeyActions(kv map[string]string) {
	c.mu.Lock()
	for k, v := range kv {
		if _, exists := c.keyActions[k]; !exists {
			c.keyActions[k] = v
		}
	}
	c.readyKeys = len(c.keyActions) > 0
	c.mu.Unlock()
	log.Printf("[deck] seeded %d keys from profile", len(kv))
}

// SetKeyImage sends a state command for one key.
// svgDataURI is "data:image/svg+xml;base64,..."
// wide=true for the large key at 3_2 (spans 2 columns, renders at 392×196).
func (c *Client) SetKeyImage(key, svgDataURI string, wide bool) error {
	w := 196
	if wide {
		w = 392
	}
	h := 196

	// Strip data URI prefix, decode base64
	b64 := svgDataURI
	prefixLen := len("data:image/svg+xml;base64,")
	if len(b64) > prefixLen && b64[:prefixLen] == "data:image/svg+xml;base64," {
		b64 = b64[prefixLen:]
	}
	svgData, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return fmt.Errorf("base64 decode: %w", err)
	}

	// Convert SVG → PNG via rasterizer
	pngData, err := svgToPNG(svgData, w, h)
	if err != nil {
		return fmt.Errorf("svg→png failed for %s: %w", key, err)
	}

	pngBase64 := base64.StdEncoding.EncodeToString(pngData)
	dataURI := "data:image/png;base64," + pngBase64

	c.mu.RLock()
	actionID := c.keyActions[key]
	c.mu.RUnlock()

	return c.send("state", map[string]any{
		"param": map[string]any{
			"statelist": []map[string]any{
				{
					"uuid":     ActionUUID,
					"key":      key,
					"actionid": actionID,
					"type":     1,
					"data":     dataURI,
					"textData": "",
					"showtext": false,
				},
			},
		},
	})
}

// ─── Send helpers ─────────────────────────────────────────────

func (c *Client) send(cmd string, params map[string]any) error {
	firstKey, firstAction := c.getFirstKeyAction()
	payload := map[string]any{
		"cmd":      cmd,
		"uuid":     PluginUUID,
		"key":      firstKey,
		"actionid": firstAction,
	}
	for k, v := range params {
		payload[k] = v
	}

	data, _ := json.Marshal(payload)
	return c.sendRaw(data)
}

func (c *Client) sendJSON(msg Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return c.sendRaw(data)
}

func (c *Client) sendRaw(data []byte) error {
	c.mu.RLock()
	ws := c.ws
	connected := c.connected
	c.mu.RUnlock()

	if ws == nil || !connected {
		return nil
	}

	if c.logAll {
		log.Printf("[deck] SEND: %s", truncateJSON(data, 80))
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return ws.WriteMessage(websocket.TextMessage, data)
}

// ─── Internal ────────────────────────────────────────────────

func (c *Client) getFirstKeyAction() (string, string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for k, v := range c.keyActions {
		return k, v
	}
	return "", ""
}

// Close terminates the WebSocket connection.
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ws != nil {
		c.ws.Close()
	}
	c.connected = false
}

// ─── SVG → PNG rasterization (delegated to svg.go) ───────────────

// ─── Helpers ─────────────────────────────────────────────────

func truncateJSON(data []byte, maxLen int) string {
	s := string(data)
	if len(s) > maxLen {
		// Try to keep start and end
		return s[:maxLen/2] + "..." + s[len(s)-maxLen/2:]
	}
	return s
}

// RawSend sends raw bytes (for tests / manual control).
func (c *Client) RawSend(data []byte) error {
	return c.sendRaw(data)
}

// IsConnected returns whether the WebSocket is connected.
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}
