package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

// testClient wraps a WebSocket connection to UlanziDeck.
// Protocol matches UlanzideckApi SDK exactly.
type testClient struct {
	conn       *websocket.Conn
	pluginUUID string
	key        string
	actionID   string
	keyActions map[string]string // key → actionID (populated by ADD events)
	done       chan struct{}
}

// connect dials ws://127.0.0.1:3906 and registers as a main service.
func connect() *testClient {
	u := url.URL{Scheme: "ws", Host: fmt.Sprintf("%s:%d", DefaultAddr, DefaultPort), Path: "/"}
	log.Printf("connecting to %s", u.String())

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatalf("dial: %v", err)
	}

	c := &testClient{
		conn:       conn,
		pluginUUID: PluginUUID,
		keyActions: make(map[string]string),
		done:       make(chan struct{}),
	}

	// Register as main service (matches SDK ulanzideckApi onopen)
	c.sendRaw(map[string]any{
		"code": 0,
		"cmd":  "connected",
		"uuid": PluginUUID,
	})
	log.Printf("registered plugin=%s", PluginUUID)

	go c.readPump()

	// Wait for ADD events to populate key→actionID mappings
	time.Sleep(1 * time.Second)

	// Log what we got
	for k, a := range c.keyActions {
		log.Printf("  key=%s → actionid=%s", k, a)
	}

	return c
}

// readPump processes incoming messages.
// For every message it replies { code:0, cmd:<sameCmd>, ...original }
// (required by the UlanziDeck protocol for main services).
func (c *testClient) readPump() {
	defer close(c.done)
	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			log.Printf("read error: %v", err)
			return
		}
		var evt map[string]any
		if err := json.Unmarshal(raw, &evt); err != nil {
			continue
		}

		cmdStr, _ := evt["cmd"].(string)

		// Required reply: code:0 with original data echoed back
		// matches SDK: if (isMain) this.send(data.cmd, { code:0, ...data })
		if cmdStr != "" && cmdStr != "connected" {
			reply := map[string]any{"code": 0, "cmd": cmdStr}
			for k, v := range evt {
				reply[k] = v
			}
			c.sendRaw(reply)
		}

		switch cmdStr {
		case "add":
			key, _ := evt["key"].(string)
			actID, _ := evt["actionid"].(string)
			if key != "" && actID != "" {
				c.keyActions[key] = actID
				log.Printf("ADD: key=%s actionid=%s", key, actID)
			}
		case "connected":
			log.Printf("server ack: connected")
		default:
			if cmdStr != "" {
				log.Printf("recv: cmd=%s", cmdStr)
			}
		}
	}
}

// sendState sends a state command matching SDK setBaseDataIcon format.
func (c *testClient) sendState(key, dataURI string) {
	actID := c.keyActions[key]
	if actID == "" {
		actID = key + "-default"
		log.Printf("warning: no actionID for key=%s, using fallback", key)
	}

	msg := map[string]any{
		"cmd":      "state",
		"uuid":     c.pluginUUID,
		"key":      key,
		"actionid": actID,
		"param": map[string]any{
			"statelist": []map[string]any{
				{
					"uuid":     ActionUUID,
					"key":      key,
					"actionid": actID,
					"type":     1,
					"data":     dataURI,
					"textData": "",
					"showtext": false,
				},
			},
		},
	}
	c.sendRaw(msg)
}

// sendGifData sends an animated GIF using SDK setGifDataIcon format.
// type:3 + gifdata (NOT type:1 + data).
// gifB64 is the raw base64 of the GIF binary (NOT a data URI).
func (c *testClient) sendGifData(key, gifB64 string) {
	actID := c.keyActions[key]
	if actID == "" {
		actID = key + "-default"
	}

	msg := map[string]any{
		"cmd":      "state",
		"uuid":     c.pluginUUID,
		"key":      key,
		"actionid": actID,
		"param": map[string]any{
			"statelist": []map[string]any{
				{
					"uuid":     ActionUUID,
					"key":      key,
					"actionid": actID,
					"type":     3,
					"gifdata":  gifB64,
					"textData": "",
					"showtext": false,
				},
			},
		},
	}
	c.sendRaw(msg)
}

func (c *testClient) sendRaw(msg map[string]any) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("json marshal error: %v", err)
		return
	}
	if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		log.Printf("send error: %v", err)
	}
}

func (c *testClient) close() {
	c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	c.conn.Close()
}
