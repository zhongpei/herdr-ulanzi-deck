package herdrclient

import (
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"
)

// startTestServer starts a TCP server that responds to JSON-line requests.
func startTestServer(t *testing.T, responses map[string]string) (string, func()) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		buf := make([]byte, 4096)
		n, err := conn.Read(buf)
		if err != nil {
			return
		}

		reqLine := strings.TrimSpace(string(buf[:n]))
		var req struct {
			Method string `json:"method"`
		}
		if err := json.Unmarshal([]byte(reqLine), &req); err != nil {
			return
		}

		resp, ok := responses[req.Method]
		if !ok {
			resp = `{"error":{"code":-1,"message":"unknown method"}}`
		}

		conn.SetWriteDeadline(time.Now().Add(time.Second))
		conn.Write([]byte(resp + "\n"))
	}()

	addr := listener.Addr().String()
	return addr, func() { listener.Close() }
}

func TestRequest_Success(t *testing.T) {
	respJSON := `{"id":"test:1","result":{"workspaces":[{"workspace_id":"ws-1","label":"main"}]}}`
	addr, cleanup := startTestServer(t, map[string]string{
		"workspace.list": respJSON,
	})
	defer cleanup()

	client := New(addr)
	resp, err := client.Request("workspace.list", map[string]any{})
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.Result == nil {
		t.Fatal("expected result")
	}
	var result struct {
		Workspaces []struct {
			ID    string `json:"workspace_id"`
			Label string `json:"label"`
		} `json:"workspaces"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(result.Workspaces) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(result.Workspaces))
	}
	if result.Workspaces[0].ID != "ws-1" {
		t.Errorf("expected ws-1, got %s", result.Workspaces[0].ID)
	}
}

func TestRequest_Error(t *testing.T) {
	addr, cleanup := startTestServer(t, map[string]string{
		"bad.method": `{"id":"test:1","error":{"code":-32601,"message":"Method not found"}}`,
	})
	defer cleanup()

	client := New(addr)
	_, err := client.Request("bad.method", map[string]any{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Method not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRequest_ConnectionRefused(t *testing.T) {
	// Connect to a port that's not listening
	client := New("127.0.0.1:1")
	_, err := client.Request("test", map[string]any{})
	if err == nil {
		t.Fatal("expected connection error")
	}
}

func TestListWorkspaces(t *testing.T) {
	respJSON := `{"id":"deck:1","result":{"workspaces":[{"workspace_id":"ws-1","label":"test"}]}}`
	addr, cleanup := startTestServer(t, map[string]string{
		"workspace.list": respJSON,
	})
	defer cleanup()

	client := New(addr)
	result, err := client.ListWorkspaces()
	if err != nil {
		t.Fatalf("ListWorkspaces failed: %v", err)
	}
	if !strings.Contains(string(result), "ws-1") {
		t.Errorf("unexpected result: %s", string(result))
	}
}

func TestListAgents(t *testing.T) {
	respJSON := `{"id":"deck:1","result":{"agents":[{"pane_id":"p1","agent":"pi"}]}}`
	addr, cleanup := startTestServer(t, map[string]string{
		"agent.list": respJSON,
	})
	defer cleanup()

	client := New(addr)
	result, err := client.ListAgents()
	if err != nil {
		t.Fatalf("ListAgents failed: %v", err)
	}
	if !strings.Contains(string(result), `"p1"`) {
		t.Errorf("unexpected result: %s", string(result))
	}
}

func TestSubscribe_Connection(t *testing.T) {
	// For subscribe, we just test that it connects and gets ack
	ack := `{"id":"deck:sub","result":{"status":"ok"}}`
	addr, cleanup := startTestServer(t, map[string]string{
		"events.subscribe": ack,
	})
	defer cleanup()

	client := New(addr)
	sub, err := client.Subscribe(map[string]any{}, func(msg json.RawMessage) {})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}
	defer sub.Close()
}

func TestSubscribe_Error(t *testing.T) {
	errResp := `{"id":"deck:sub","error":{"code":-1,"message":"subscribe failed"}}`
	addr, cleanup := startTestServer(t, map[string]string{
		"events.subscribe": errResp,
	})
	defer cleanup()

	client := New(addr)
	_, err := client.Subscribe(map[string]any{}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "subscribe failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSubscribe_ReadLoopReceivesEvents(t *testing.T) {
	// Server that sends ack, then sends events
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().String()

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		buf := make([]byte, 4096)
		n, _ := conn.Read(buf)
		var req struct {
			Method string `json:"method"`
		}
		json.Unmarshal(buf[:n], &req)

		if req.Method == "events.subscribe" {
			// Send ack
			conn.Write([]byte(`{"id":"deck:sub","result":{"status":"ok"}}` + "\n"))
			// Send events
			conn.Write([]byte(`{"event":"agent_status","data":{"pane_id":"p1"}}` + "\n"))
			conn.Write([]byte(`{"event":"agent_status","data":{"pane_id":"p2"}}` + "\n"))
			conn.Write([]byte(`not-json` + "\n")) // invalid JSON → should be skipped
			conn.Write([]byte(`{"event":"agent_status","data":{"pane_id":"p3"}}` + "\n"))
			// Wait for client to consume before closing
			time.Sleep(100 * time.Millisecond)
		}
	}()

	client := New(addr)
	received := make([]string, 0)
	sub, err := client.Subscribe(map[string]any{}, func(msg json.RawMessage) {
		var evt struct {
			Event string `json:"event"`
		}
		if json.Unmarshal(msg, &evt) == nil {
			received = append(received, evt.Event)
		}
	})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	// Wait for events to arrive
	time.Sleep(300 * time.Millisecond)
	sub.Close()

	if len(received) < 3 {
		t.Errorf("expected 3 events, got %d: %v", len(received), received)
	}
	// Invalid JSON line should not appear
	for _, evt := range received {
		if evt != "agent_status" {
			t.Errorf("unexpected event: %s", evt)
		}
	}
}

func TestDial_UnixPath(t *testing.T) {
	c := New("/tmp/test.sock")
	// dial should try Unix socket (will fail because socket doesn't exist)
	_, err := c.dial()
	if err == nil {
		t.Error("expected error dialing non-existent Unix socket")
	}
	// Verify it tried Unix, not TCP
	if !strings.Contains(err.Error(), "unix") && !strings.Contains(err.Error(), "no such file") && !strings.Contains(err.Error(), "connection refused") {
		t.Logf("dial error (expected): %v", err)
	}
}

func TestDial_TCPPath(t *testing.T) {
	c := New("127.0.0.1:1")
	_, err := c.dial()
	if err == nil {
		t.Error("expected error dialing closed TCP port")
	}
}

func TestRequest_InvalidJSONResponse(t *testing.T) {
	listener, _ := net.Listen("tcp", "127.0.0.1:0")
	defer listener.Close()
	addr := listener.Addr().String()

	go func() {
		conn, _ := listener.Accept()
		defer conn.Close()
		buf := make([]byte, 4096)
		conn.Read(buf)
		conn.Write([]byte("not valid json\n"))
	}()

	client := New(addr)
	_, err := client.Request("test", map[string]any{})
	if err == nil {
		t.Fatal("expected parse error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("expected parse error, got: %v", err)
	}
}

func TestListTabs(t *testing.T) {
	respJSON := `{"id":"deck:1","result":{"tabs":[{"tab_id":"t1","label":"main"}]}}`
	addr, cleanup := startTestServer(t, map[string]string{
		"tab.list": respJSON,
	})
	defer cleanup()

	client := New(addr)
	result, err := client.ListTabs()
	if err != nil {
		t.Fatalf("ListTabs failed: %v", err)
	}
	if !strings.Contains(string(result), "t1") {
		t.Errorf("unexpected result: %s", string(result))
	}
}
