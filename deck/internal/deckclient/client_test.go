package deckclient

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// testWSServer creates an httptest WebSocket echo server and returns
// the server + a channel of raw messages received.
func testWSServer(t *testing.T) (*httptest.Server, chan []byte) {
	t.Helper()
	msgCh := make(chan []byte, 64)
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("ws upgrade: %v", err)
			return
		}
		defer conn.Close()
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			select {
			case msgCh <- msg:
			default:
			}
		}
	}))
	return s, msgCh
}

// setupClient creates a Client connected to a test WS server with key actions seeded.
func setupClient(t *testing.T) (*Client, chan []byte, func()) {
	t.Helper()
	s, msgCh := testWSServer(t)

	addr := strings.TrimPrefix(s.URL, "http://")
	host := addr
	port := 80
	if colonIdx := strings.LastIndexByte(addr, ':'); colonIdx >= 0 {
		host = addr[:colonIdx]
		fmt.Sscanf(addr[colonIdx+1:], "%d", &port)
	} else {
		port = 80
	}

	opts := Options{
		Address: host,
		Port:    port,
	}
	c := New(opts, nil, nil)
	if err := c.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	c.SeedKeyActions(map[string]string{
		"0_2": "test-action-0_2",
		"1_2": "test-action-1_2",
		"3_2": "test-action-3_2",
	})
	time.Sleep(50 * time.Millisecond)
	drainMsgCh(msgCh)
	return c, msgCh, func() {
		s.Close()
		c.Close()
	}
}

func drainMsgCh(ch chan []byte) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

// countMsgCh drains and returns accumulated message count.
func countMsgCh(ch chan []byte) int {
	n := 0
	for {
		select {
		case <-ch:
			n++
		default:
			return n
		}
	}
}

const testSVG = "data:image/svg+xml;base64," + "PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIxOTYiIGhlaWdodD0iMTk2Ij48cmVjdCB3aWR0aD0iMTk2IiBoZWlnaHQ9IjE5NiIgZmlsbD0iIzJhMmEyYSIvPjwvc3ZnPg=="

// With ImageCache: 2 calls with same key+SVG → 1 WS message (second skips).
func TestSetKeyImage_SameKeySameSVG_SecondCallSkips(t *testing.T) {
	c, msgCh, cleanup := setupClient(t)
	defer cleanup()

	err1 := c.SetKeyImage("0_2", testSVG, false)
	err2 := c.SetKeyImage("0_2", testSVG, false)

	time.Sleep(20 * time.Millisecond)
	total := countMsgCh(msgCh)

	if err1 != nil {
		t.Fatalf("first SetKeyImage: %v", err1)
	}
	if err2 != nil {
		t.Fatalf("second SetKeyImage: %v", err2)
	}
	if total != 1 {
		t.Errorf("expected 1 WS message (cache), got %d", total)
	}
}

// Same SVG, different key → LRU hit, sends cached PNG (no conversion).
func TestSetKeyImage_DiffKeySameSVG_LRUReuse(t *testing.T) {
	c, msgCh, cleanup := setupClient(t)
	defer cleanup()

	// Render on key0_2 first → converts + caches, sends 1 msg
	if err := c.SetKeyImage("0_2", testSVG, false); err != nil {
		t.Fatalf("SetKeyImage 0_2: %v", err)
	}

	// Same SVG on key1_2 → LRU hit, send cached (no conversion), sends 1 msg
	if err := c.SetKeyImage("1_2", testSVG, false); err != nil {
		t.Fatalf("SetKeyImage 1_2: %v", err)
	}

	time.Sleep(20 * time.Millisecond)
	total := countMsgCh(msgCh)
	t.Logf("total messages: %d", total)

	if total != 2 {
		t.Errorf("expected 2 messages (1 convert + 1 cached), got %d", total)
	}
}

// After Reset, same SVG should reconvert (both caches cleared).
func TestSetKeyImage_AfterReset_Reconverts(t *testing.T) {
	c, msgCh, cleanup := setupClient(t)
	defer cleanup()

	if err := c.SetKeyImage("0_2", testSVG, false); err != nil {
		t.Fatalf("first: %v", err)
	}

	c.imageCache.Reset()

	if err := c.SetKeyImage("0_2", testSVG, false); err != nil {
		t.Fatalf("after reset: %v", err)
	}

	time.Sleep(20 * time.Millisecond)
	total := countMsgCh(msgCh)
	t.Logf("total messages: %d", total)

	if total != 2 {
		t.Errorf("expected 2 messages (reset forces reconvert), got %d", total)
	}
}

// SVG rendered at wide/N-wide width → different cache entries.
func TestSetKeyImage_WideAndRegular_SeparateCache(t *testing.T) {
	c, msgCh, cleanup := setupClient(t)
	defer cleanup()

	// Same SVG, regular width
	if err := c.SetKeyImage("0_2", testSVG, false); err != nil {
		t.Fatalf("regular: %v", err)
	}
	// Same SVG, wide version → different hash (width in hash)
	if err := c.SetKeyImage("3_2", testSVG, true); err != nil {
		t.Fatalf("wide: %v", err)
	}
	// Same key, repeat regular → should be cached
	if err := c.SetKeyImage("0_2", testSVG, false); err != nil {
		t.Fatalf("regular repeat: %v", err)
	}

	time.Sleep(20 * time.Millisecond)
	total := countMsgCh(msgCh)
	t.Logf("total messages: %d", total)

	// 3 calls: regular(conv) + wide(conv) + regular(cache hit) = 2 messages
	if total != 2 {
		t.Errorf("expected 2 messages (regular+wide, 3rd cached), got %d", total)
	}
}

func TestSetKeyImage_ConcurrentSafe(t *testing.T) {
	c, msgCh, cleanup := setupClient(t)
	defer cleanup()

	done := make(chan struct{}, 2)
	go func() {
		for i := 0; i < 10; i++ {
			_ = c.SetKeyImage("0_2", testSVG, false)
		}
		done <- struct{}{}
	}()
	go func() {
		for i := 0; i < 10; i++ {
			_ = c.SetKeyImage("1_2", testSVG, false)
		}
		done <- struct{}{}
	}()

	<-done
	<-done

	time.Sleep(20 * time.Millisecond)
	total := countMsgCh(msgCh)
	t.Logf("total messages from concurrent calls: %d", total)

	// At most: first key converts + first other key LRU reuse = 2 (or more if race)
	// At least: both keys should send at least 1 message each = 2
	if total < 2 {
		t.Errorf("expected at least 2 messages from 2 keys, got %d", total)
	}
	if total > 10 {
		t.Errorf("too many messages from concurrent calls (cache issue): %d", total)
	}
}

// Different SVG on same key → cache miss → new conversion + send.
func TestSetKeyImage_DifferentSVG_Reconverts(t *testing.T) {
	c, msgCh, cleanup := setupClient(t)
	defer cleanup()

	svgA := testSVG
	svgB := "data:image/svg+xml;base64," + "PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIxOTYiIGhlaWdodD0iMTk2Ij48cmVjdCB3aWR0aD0iMTk2IiBoZWlnaHQ9IjE5NiIgZmlsbD0iIzMzMzMzMyIvPjwvc3ZnPg=="

	if err := c.SetKeyImage("0_2", svgA, false); err != nil {
		t.Fatalf("svgA: %v", err)
	}
	if err := c.SetKeyImage("0_2", svgB, false); err != nil {
		t.Fatalf("svgB: %v", err)
	}

	time.Sleep(20 * time.Millisecond)
	total := countMsgCh(msgCh)
	t.Logf("total messages: %d", total)

	// Both different → 2 messages
	if total != 2 {
		t.Errorf("expected 2 messages (different SVGs), got %d", total)
	}
}

// ─── SetKeyGIFImage ──────────────────────────────────────────────

var testGIFFrames = []string{
	"data:image/svg+xml;base64," + "PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIxOTYiIGhlaWdodD0iMTk2Ij48cmVjdCB3aWR0aD0iMTk2IiBoZWlnaHQ9IjE5NiIgZmlsbD0iIzJhMmEyYSIvPjwvc3ZnPg==",
	"data:image/svg+xml;base64," + "PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIxOTYiIGhlaWdodD0iMTk2Ij48cmVjdCB3aWR0aD0iMTk2IiBoZWlnaHQ9IjE5NiIgZmlsbD0iIzMzMzMzMyIvPjwvc3ZnPg==",
}

func TestSetKeyGIFImage_SendsGIF(t *testing.T) {
	c, msgCh, cleanup := setupClient(t)
	defer cleanup()

	delays := []int{120, 120}
	if err := c.SetKeyGIFImage("0_2", testGIFFrames, delays, false); err != nil {
		t.Fatalf("SetKeyGIFImage: %v", err)
	}

	time.Sleep(20 * time.Millisecond)
	select {
	case raw := <-msgCh:
		msgStr := string(raw)
		if !strings.Contains(msgStr, `"type":3`) && !strings.Contains(msgStr, `"type":3`) {
			t.Errorf("message should have type:3, got: %s", msgStr)
		}
		if !strings.Contains(msgStr, `"gifdata"`) {
			t.Errorf("message should have gifdata field, got: %s", msgStr)
		}
	default:
		t.Error("expected at least one WS message")
	}
}

func TestSetKeyGIFImage_DifferentFrames_Reconverts(t *testing.T) {
	c, msgCh, cleanup := setupClient(t)
	defer cleanup()

	delays := []int{120, 120}
	if err := c.SetKeyGIFImage("0_2", testGIFFrames, delays, false); err != nil {
		t.Fatalf("first: %v", err)
	}

	// Different frames
	other := []string{testGIFFrames[0]}
	if err := c.SetKeyGIFImage("0_2", other, []int{120}, false); err != nil {
		t.Fatalf("different: %v", err)
	}

	time.Sleep(20 * time.Millisecond)
	if total := countMsgCh(msgCh); total != 2 {
		t.Errorf("expected 2 messages (different frames), got %d", total)
	}
}
