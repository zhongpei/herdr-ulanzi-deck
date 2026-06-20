// herdr-deck: UlanziDeck plugin for Herdr agent status display.
//
// Architecture:
//
//	WebSocket ─── messagePump (goroutine) ──msgChan──┐
//	                                                  │
//	ticker(50ms) ─────────────────────────────────────┤
//	                                                  ▼
//	                                        appLoop (single goroutine)
//	                                            │
//	                                            ├─ handleMessage → update state
//	                                            ├─ set needRender=true
//	                                            ├─ on tick: if needRender && sigChanged → renderAll()
//	                                            └─ never concurrent
package main

import (
	"crypto/sha256"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/herdr-deck/herdrdeck/pkg/deck"
	"github.com/herdr-deck/herdrdeck/pkg/herdr"
	"github.com/herdr-deck/herdrdeck/pkg/mapper"
	"github.com/herdr-deck/herdrdeck/pkg/profile"
	"github.com/herdr-deck/herdrdeck/pkg/render"
	"github.com/herdr-deck/herdrdeck/pkg/state"
	"github.com/herdr-deck/herdrdeck/pkg/types"
)

// ─── Physical key map ────────────────────────────────────────
var keyMap = map[string]int{
	"0_0": 0, "1_0": 1, "2_0": 2, "3_0": 3, "4_0": 4,
	"0_1": 5, "1_1": 6, "2_1": 7, "3_1": 8, "4_1": 9,
	"0_2": 10, "1_2": 11, "2_2": 12, "3_2": 13,
}

var descriptorMap = map[string]string{
	"nav_all": "0_2", "nav_machine": "1_2", "nav_space": "2_2", "stats": "3_2",
}

func physicalKeyForDescriptor(keyID string) string {
	if phys, ok := descriptorMap[keyID]; ok {
		return phys
	}
	var idx int
	if _, err := fmt.Sscanf(keyID, "agent_%d", &idx); err == nil {
		if idx >= 0 && idx <= 4 {
			return fmt.Sprintf("%d_0", idx)
		}
		if idx >= 5 && idx <= 9 {
			return fmt.Sprintf("%d_1", idx-5)
		}
	}
	return "0_0"
}

// ─── Global state (single goroutine) ─────────────────────────
var (
	stateManager  *state.Manager
	buttonMapper  *mapper.Mapper
	iconRenderer  *render.Renderer
	deckClient    *deck.Client
	lastRenderSig string // render dedup: skip if unchanged
	needRender    bool   // flag set by event handlers, consumed by tick
)

// renderSig returns a hash of state that determines if output changed.
func renderSig() string {
	agents := stateManager.GetFilteredAgents(buttonMapper.ConnName, buttonMapper.WsID)
	h := sha256.New()
	for _, a := range agents {
		// Include all fields that affect visual output
		fmt.Fprintf(h, "%s|%s|%s|%v|%s|%s|%s\n",
			a.PaneID, a.Agent, a.AgentStatus, a.Focused, a.ConnName, a.Name, a.WsLabel)
	}
	fmt.Fprintf(h, "mode=%d conn=%s ws=%s\n", buttonMapper.Mode, buttonMapper.ConnName, buttonMapper.WsID)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func main() {
	stateManager = state.NewManager()
	iconRenderer = render.New()
	buttonMapper = mapper.New(stateManager)

	// ── Load config + herdr data ───────────────────────────────
	cfg, err := herdr.LoadConfig()
	if err != nil {
		log.Printf("[main] config warning: %v", err)
	}
	bridge := herdr.NewBridge()
	for _, c := range cfg.Connections {
		if c.Type == "local" {
			socketPath := herdr.FindLocalSocket()
			if socketPath == "" {
				log.Printf("[main] no local socket for %q", c.Name)
				continue
			}
			bridge.AddConnection(c.Name, c.Abbr, c.Color, herdr.New(socketPath))
		}
	}
	unified := bridge.FetchAll()
	if len(unified) == 0 {
		log.Println("[main] no herdr data, using mock")
		unified = buildMockData()
	}
	stateManager.Init(unified)
	log.Printf("[main] %d workspaces, %d agents", len(unified), len(stateManager.GetAllAgents()))

	// ── Connect to deck ────────────────────────────────────────
	deckClient = deck.New(onAdd, onKeyDown)
	if err := deckClient.Connect(); err != nil {
		log.Printf("[main] connect failed: %v, falling back to console", err)
	}

	// ── Profile setup ──────────────────────────────────────────
	pm := profile.New()
	profileDir := pm.Ensure("02d04a045u3673881")
	if profileDir != "" {
		pm.ActivateProfile("02d04a045u3673881")
		if ka := pm.GetKeyActionMap(); len(ka) > 0 {
			log.Printf("[main] profile ready, %d keys", len(ka))
			deckClient.SeedKeyActions(ka)
		}
	}

	// ── Event loop (single goroutine) ──────────────────────────
	msgChan := make(chan interface{}, 64)
	go messagePump(msgChan)

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	needRender = true // render on first tick
	for {
		select {
		case raw := <-msgChan:
			switch m := raw.(type) {
			case deck.Message:
				handleMessage(m)
			case func():
				m()
			}

		case <-ticker.C:
			if needRender {
				sig := renderSig()
				if sig != lastRenderSig {
					lastRenderSig = sig
					doRender()
				}
				needRender = false
			}
		}
	}
}

// ─── messagePump: receives WebSocket messages, pushes to channel ──
func messagePump(msgChan chan<- interface{}) {
	for {
		deckClient.ReadPump()
		log.Println("[main] deck disconnected, reconnecting in 2s...")
		time.Sleep(2 * time.Second)
		for {
			if err := deckClient.Connect(); err == nil {
				log.Println("[main] reconnected")
				msgChan <- func() { needRender = true }
				break
			}
			time.Sleep(2 * time.Second)
		}
	}
}

// ─── Handle messages (runs in event loop goroutine) ───────────
func handleMessage(msg deck.Message) {
	switch msg.Cmd {
	case "connected":
		log.Printf("[deck] connected: key=%s actionid=%s", msg.Key, msg.ActionID)
	case "setactive":
		// no-op, deck sends these after add
	default:
		log.Printf("[deck] unhandled: %s", msg.Cmd)
	}
}

func onAdd(key, actionID string) {
	log.Printf("[main] action added: key=%s", key)
	needRender = true
}

func onKeyDown(msg deck.Message) {
	switch msg.Key {
	case "0_2", "0_3": // K11 or hw prev
		buttonMapper.SetAll()
		needRender = true
	case "1_2", "1_3": // K12 or hw next
		buttonMapper.NextMachine()
		needRender = true
	case "2_2": // K13
		buttonMapper.NextSpace()
		needRender = true
	default:
		if idx, ok := keyMap[msg.Key]; ok && idx < 10 {
			keyData := buttonMapper.RenderAll()
			if idx < len(keyData) && keyData[idx].Agent != nil {
				a := keyData[idx].Agent
				log.Printf("[action] focus: %s/%s", a.ConnName, a.PaneID)
			}
		}
	}
}

// ── doRender: single-goroutine, no concurrency ────────────────
func doRender() {
	keyData := buttonMapper.RenderAll()
	for _, kd := range keyData {
		var svg string
		switch {
		case kd.Agent != nil:
			svg = iconRenderer.RenderAgentKey(*kd.Agent)
		case kd.NavAll != nil:
			svg = iconRenderer.RenderNavAll(*kd.NavAll)
		case kd.NavMac != nil:
			svg = iconRenderer.RenderNavMachine(*kd.NavMac)
		case kd.NavSpc != nil:
			svg = iconRenderer.RenderNavSpace(*kd.NavSpc)
		case kd.Stats != nil:
			svg = iconRenderer.RenderStatsKey(kd.Stats.Stats)
		default:
			svg = iconRenderer.RenderEmptyKey()
		}
		physKey := physicalKeyForDescriptor(keyTypeKeyID(kd))
		if deckClient != nil && deckClient.IsConnected() {
			isWide := physKey == "3_2"
			if err := deckClient.SetKeyImage(physKey, svg, isWide); err != nil {
				log.Printf("[render] %s failed: %v", physKey, err)
			}
		}
		if kd.Type() != "empty" {
			log.Printf("[render] %s (%s)", physKey, kd.Type())
		}
	}
	logFilterInfo()
}

func keyTypeKeyID(kc types.KeyCommand) string {
	switch {
	case kc.Agent != nil:
		return kc.Agent.KeyID
	case kc.NavAll != nil:
		return kc.NavAll.KeyID
	case kc.NavMac != nil:
		return kc.NavMac.KeyID
	case kc.NavSpc != nil:
		return kc.NavSpc.KeyID
	case kc.Stats != nil:
		return kc.Stats.KeyID
	case kc.Empty != nil:
		return kc.Empty.KeyID
	default:
		return ""
	}
}

func logFilterInfo() {
	machines := stateManager.GetMachines()
	allAgents := stateManager.GetAllAgents()
	stats := stateManager.ComputeStats()
	log.Printf("[info] %d machine(s), %d agents | D%d I%d W%d B%d ?%d",
		len(machines), len(allAgents),
		stats.Done, stats.Idle, stats.Working, stats.Blocked, stats.Unknown)
}

// ── Mock data (fallback) ─────────────────────────────────────
func buildMockData() []types.UnifiedWorkspace {
	return []types.UnifiedWorkspace{
		{
			ConnName: "local", ConnAbbr: "LCL", ConnAbbrColor: "#4ADE80",
			WorkspaceID: "ws-1", Label: "main-proj",
			Agents: []types.AgentInfo{
				{PaneID: "p1", Agent: "pi", Name: "review", AgentStatus: types.StatusWorking, Focused: true},
				{PaneID: "p2", Agent: "cursor", Name: "fix-bug", AgentStatus: types.StatusBlocked},
				{PaneID: "p3", Agent: "pi", Name: "idle", AgentStatus: types.StatusIdle},
			},
		},
		{
			ConnName: "local", ConnAbbr: "LCL", ConnAbbrColor: "#4ADE80",
			WorkspaceID: "ws-2", Label: "web-app",
			Agents: []types.AgentInfo{
				{PaneID: "p4", Agent: "claude", Name: "api-done", AgentStatus: types.StatusDone},
				{PaneID: "p5", Agent: "pi", Name: "feat-auth", AgentStatus: types.StatusWorking},
			},
		},
		{
			ConnName: "dev-server", ConnAbbr: "DEV", ConnAbbrColor: "#60A5FA",
			WorkspaceID: "ws-3", Label: "backend",
			Agents: []types.AgentInfo{
				{PaneID: "p6", Agent: "gemini", Name: "waiting", AgentStatus: types.StatusIdle},
				{PaneID: "p7", Agent: "copilot", Name: "deploy", AgentStatus: types.StatusWorking},
				{PaneID: "p8", Agent: "devin", Name: "test-fail", AgentStatus: types.StatusBlocked},
				{PaneID: "p9", Agent: "cursor", Name: "done", AgentStatus: types.StatusDone},
				{PaneID: "p10", Agent: "cline", Name: "unknown-act", AgentStatus: types.StatusUnknown},
			},
		},
		{
			ConnName: "dev-server", ConnAbbr: "DEV", ConnAbbrColor: "#60A5FA",
			WorkspaceID: "ws-4", Label: "infra",
			Agents: []types.AgentInfo{
				{PaneID: "p11", Agent: "cline", Name: "tf-plan", AgentStatus: types.StatusWorking},
			},
		},
		{
			ConnName: "prod", ConnAbbr: "PRD", ConnAbbrColor: "#F87171",
			WorkspaceID: "ws-5", Label: "prod-site",
			Agents: []types.AgentInfo{
				{PaneID: "p12", Agent: "pi", Name: "deployed", AgentStatus: types.StatusDone},
				{PaneID: "p13", Agent: "cursor", Name: "monitor", AgentStatus: types.StatusWorking},
				{PaneID: "p14", Agent: "pi", Name: "standby", AgentStatus: types.StatusIdle},
			},
		},
		{
			ConnName: "prod", ConnAbbr: "PRD", ConnAbbrColor: "#F87171",
			WorkspaceID: "ws-6", Label: "monitoring",
			Agents: []types.AgentInfo{
				{PaneID: "p15", Agent: "grok", Name: "alert-45", AgentStatus: types.StatusBlocked},
			},
		},
	}
}

func init() {
	log.SetFlags(log.Ltime | log.Lmsgprefix)
	log.SetPrefix("[herdr-deck] ")
}
