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
	"sync"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

const (
	PluginUUID  = "com.ulanzi.herdr.agentview"
	ActionUUID  = "com.ulanzi.herdr.agentview.monitor"
	DefaultPort = 3906
	DefaultAddr = "127.0.0.1"
)

// Options for creating a new Client.
type Options struct {
	Address string
	Port    int
	Debug   bool
}

// Message types from UlanziDeck.
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
	opts       Options
	keyActions map[string]string // key → actionID
	readyKeys  bool
	mu         sync.RWMutex
	writeMu    sync.Mutex

	onAdd     AddHandler
	onKeyDown KeyDownHandler
}

// New creates a DeckClient.
func New(opts Options, onAdd AddHandler, onKeyDown KeyDownHandler) *Client {
	if opts.Address == "" {
		opts.Address = DefaultAddr
	}
	if opts.Port == 0 {
		opts.Port = DefaultPort
	}
	c := &Client{
		opts:       opts,
		keyActions: make(map[string]string),
		onAdd:      onAdd,
		onKeyDown:  onKeyDown,
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
	addr := fmt.Sprintf("ws://%s:%d", c.opts.Address, c.opts.Port)
	ws, _, err := websocket.DefaultDialer.Dial(addr, nil)
	if err != nil {
		return fmt.Errorf("dial %s: %w", addr, err)
	}

	c.mu.Lock()
	// Close old connection if any
	if c.ws != nil {
		c.ws.Close()
	}
	c.ws = ws
	c.connected = true
	c.mu.Unlock()

	code0 := 0
	c.sendJSON(Message{
		Code:     &code0,
		Cmd:      "connected",
		UUID:     PluginUUID,
		Key:      "",
		ActionID: "",
	})

	log.Info().Msg("connected as main service")
	return nil
}

// ReadPump reads messages from WebSocket and dispatches to handlers.
// Blocks until the connection is closed or an error occurs.
func (c *Client) ReadPump() {
	c.mu.RLock()
	ws := c.ws
	c.mu.RUnlock()
	if ws == nil {
		return
	}

	for {
		_, raw, err := ws.ReadMessage()
		if err != nil {
			log.Error().Err(err).Msg("deck read error")
			c.mu.Lock()
			c.connected = false
			c.mu.Unlock()
			return
		}

		var msg Message
		if err := json.Unmarshal(raw, &msg); err != nil {
			log.Warn().Err(err).Msg("deck parse error")
			continue
		}

		c.handleMessage(msg, raw)
	}
}

func (c *Client) handleMessage(msg Message, raw []byte) {
	// SDK: only ack messages without a code field (bare events, not responses)
	if msg.Cmd != "" && msg.Cmd != "connected" && msg.Code == nil {
		c.sendAck(msg)
	}

	switch msg.Cmd {
	case "connected":
		c.log("[deck] connected: key=%s actionid=%s", msg.Key, msg.ActionID)
	case "add":
		if msg.Key != "" && msg.ActionID != "" {
			c.mu.Lock()
			c.keyActions[msg.Key] = msg.ActionID
			n := len(c.keyActions)
			c.mu.Unlock()
			c.readyKeys = true
			c.onAdd(msg.Key, msg.ActionID)
			c.log("[deck] add: key=%s (total: %d)", msg.Key, n)
		}
	case "clear":
		if msg.Param != nil {
			var items []struct {
				Key string `json:"key"`
			}
			if json.Unmarshal(msg.Param, &items) == nil {
				for _, item := range items {
					c.mu.Lock()
					delete(c.keyActions, item.Key)
					c.mu.Unlock()
					c.log("[deck] clear: key=%s", item.Key)
				}
			}
		}
	case "keydown":
		c.onKeyDown(msg)
	case "keyup", "run", "setactive":
	}
}

func (c *Client) sendAck(msg Message) {
	ack := map[string]any{
		"cmd":      msg.Cmd,
		"code":     0,
		"uuid":     PluginUUID,
		"key":      msg.Key,
		"actionid": msg.ActionID,
	}
	// Echo back all original fields
	data, _ := json.Marshal(msg)
	var raw map[string]any
	if json.Unmarshal(data, &raw) == nil {
		for k, v := range raw {
			ack[k] = v
		}
	}
	ackData, _ := json.Marshal(ack)
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
	c.log("[deck] seeded %d keys from profile", len(kv))
}

// SetKeyImage sends a state command for one key.
func (c *Client) SetKeyImage(key, svgDataURI string, wide bool) error {
	w := 196
	if wide {
		w = 392
	}
	h := 196

	b64 := svgDataURI
	prefixLen := len("data:image/svg+xml;base64,")
	if len(b64) > prefixLen && b64[:prefixLen] == "data:image/svg+xml;base64," {
		b64 = b64[prefixLen:]
	}
	svgData, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return fmt.Errorf("base64 decode: %w", err)
	}

	pngData, err := svgToPNG(svgData, w, h)
	if err != nil {
		return fmt.Errorf("svg→png: %w", err)
	}

	pngBase64 := base64.StdEncoding.EncodeToString(pngData)
	dataURI := "data:image/png;base64," + pngBase64

	c.mu.RLock()
	actionID := c.keyActions[key]
	c.mu.RUnlock()

	if actionID == "" {
		c.log("[deck] SetKeyImage %s: no actionID, skip", key)
		return nil
	}

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
	if firstAction == "" {
		return nil
	}
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

func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// log emits a zerolog debug message when debug mode is on.
func (c *Client) log(format string, args ...any) {
	if c.opts.Debug {
		log.Debug().Msg(fmt.Sprintf(format, args...))
	}
}

// Ensure imports are used
var _ = fmt.Sprintf
