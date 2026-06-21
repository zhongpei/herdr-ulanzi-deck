// Package herdrclient implements the Unix socket JSON-line protocol client for herdr.
// Mirrors src/herdr-client.js
//
// Each request opens a new connection (matching herdr's Rust client behavior).
package herdrclient

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"time"
)

var reqID atomic.Int32

// Client communicates with a herdr daemon via Unix socket or TCP.
type Client struct {
	target string // Unix socket path, or "host:port" for TCP
}

// New creates a herdr client for the given target.
// For Unix socket: "/path/to/herdr.sock"
// For TCP: "host:port"
func New(target string) *Client {
	return &Client{target: target}
}

// Response from herdr daemon.
type Response struct {
	ID     string          `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Request sends a JSON-line request and waits for one response.
func (c *Client) Request(method string, params map[string]any) (*Response, error) {
	id := fmt.Sprintf("deck:%d", reqID.Add(1))
	req := map[string]any{
		"id":     id,
		"method": method,
		"params": params,
	}
	reqData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	conn, err := c.dial()
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", c.target, err)
	}
	defer conn.Close()

	// Set deadline
	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return nil, fmt.Errorf("set deadline: %w", err)
	}

	// Send request
	if _, err := conn.Write(append(reqData, '\n')); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

	// Read response (one JSON line)
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("read: %w", err)
		}
		return nil, errors.New("connection closed before response")
	}

	line := scanner.Bytes()
	var resp Response
	if err := json.Unmarshal(line, &resp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if resp.Error != nil {
		return &resp, fmt.Errorf("herdr error: %s", resp.Error.Message)
	}

	return &resp, nil
}

// ListWorkspaces calls workspace.list.
func (c *Client) ListWorkspaces() (json.RawMessage, error) {
	resp, err := c.Request("workspace.list", map[string]any{})
	if err != nil {
		return nil, err
	}
	return resp.Result, nil
}

// ListAgents calls agent.list.
func (c *Client) ListAgents() (json.RawMessage, error) {
	resp, err := c.Request("agent.list", map[string]any{})
	if err != nil {
		return nil, err
	}
	return resp.Result, nil
}

// ListPanes calls pane.list.
func (c *Client) ListPanes() (json.RawMessage, error) {
	resp, err := c.Request("pane.list", map[string]any{})
	if err != nil {
		return nil, err
	}
	return resp.Result, nil
}

// ListTabs calls tab.list and returns the raw result.
func (c *Client) ListTabs() (json.RawMessage, error) {
	resp, err := c.Request("tab.list", map[string]any{})
	if err != nil {
		return nil, err
	}
	return resp.Result, nil
}

// Subscribe establishes a long-lived subscription connection.
// The caller must close the Subscriber when done.
func (c *Client) Subscribe(params map[string]any, onEvent func(json.RawMessage)) (*Subscriber, error) {
	conn, err := c.dial()
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}

	req := map[string]any{
		"id":     "deck:sub",
		"method": "events.subscribe",
		"params": params,
	}
	reqData, err := json.Marshal(req)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("marshal: %w", err)
	}

	if _, err := conn.Write(append(reqData, '\n')); err != nil {
		conn.Close()
		return nil, fmt.Errorf("write: %w", err)
	}

	// Read subscription ack
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)
	if !scanner.Scan() {
		conn.Close()
		return nil, errors.New("connection closed before ack")
	}

	line := scanner.Bytes()
	var ack Response
	if err := json.Unmarshal(line, &ack); err != nil {
		conn.Close()
		return nil, fmt.Errorf("parse ack: %w", err)
	}
	if ack.Error != nil {
		conn.Close()
		return nil, fmt.Errorf("subscribe error: %s", ack.Error.Message)
	}

	sub := &Subscriber{
		conn:    conn,
		scanner: scanner,
		onEvent: onEvent,
	}
	go sub.readLoop()
	return sub, nil
}

// dial connects to the target (Unix socket or TCP).
func (c *Client) dial() (net.Conn, error) {
	if len(c.target) > 0 && c.target[0] == '/' {
		return net.DialTimeout("unix", c.target, 5*time.Second)
	}
	return net.DialTimeout("tcp", c.target, 5*time.Second)
}

// Subscriber manages a long-lived event subscription.
type Subscriber struct {
	conn    net.Conn
	scanner *bufio.Scanner
	onEvent func(json.RawMessage)
}

// readLoop reads events and dispatches to onEvent.
func (s *Subscriber) readLoop() {
	defer s.conn.Close()
	for s.scanner.Scan() {
		line := s.scanner.Bytes()
		var msg json.RawMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			continue
		}
		s.onEvent(msg)
	}
}

// Close terminates the subscription.
func (s *Subscriber) Close() error {
	return s.conn.Close()
}

// ensure io.EOF isn't unused
var _ = io.EOF
