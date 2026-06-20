// herdr-deck: display herdr agent status on Ulanzi D200X.
//
// Event loop architecture:
//
//	ReadPump → store callbacks (lightweight, no render)
//	ticker 50ms → if dirty: Capture → hash compare → render
//	messagePump → reconnect + re-seed
//
// Only one goroutine touches state and render (the event loop).
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/herdr-deck/herdrdeck/pkg/appstate"
	"github.com/herdr-deck/herdrdeck/pkg/deck"
	"github.com/herdr-deck/herdrdeck/pkg/herdr"
	"github.com/herdr-deck/herdrdeck/pkg/mapper"
	"github.com/herdr-deck/herdrdeck/pkg/profile"
	"github.com/herdr-deck/herdrdeck/pkg/render"
	"github.com/herdr-deck/herdrdeck/pkg/state"
	"github.com/herdr-deck/herdrdeck/pkg/types"
)

// ─── Key mapping ───────────────────────────────────────────
var keyMap = map[string]int{
	"0_0": 0, "1_0": 1, "2_0": 2, "3_0": 3, "4_0": 4,
	"0_1": 5, "1_1": 6, "2_1": 7, "3_1": 8, "4_1": 9,
	"0_2": 10, "1_2": 11, "2_2": 12, "3_2": 13,
}

var navKeys = map[string]string{
	"nav_all": "0_2", "nav_machine": "1_2", "nav_space": "2_2", "stats": "3_2",
}

func physKeyFromID(keyID string) string {
	if p, ok := navKeys[keyID]; ok {
		return p
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

func keyCommandID(kc types.KeyCommand) string {
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

// ─── Globals (owned by eventLoop goroutine) ────────────────
var (
	sm       *state.Manager
	bm       *mapper.Mapper
	ir       *render.Renderer
	dc       *deck.Client
	st       *appstate.Store
	lastHash string
)

// ─── Callbacks (called from ReadPump goroutine) ─────────────
// MUST be lightweight: store mutation only, no I/O, no render.
func onAdd(key, actionID string) {
	log.Printf("[main] action added: key=%s", key)
}

func onKeyDown(msg deck.Message) {
	switch msg.Key {
	case "0_2", "0_3":
		st.SetAll()
	case "1_2", "1_3":
		st.NextMachine()
	case "2_2":
		st.NextSpace()
	default:
		if idx, ok := keyMap[msg.Key]; ok && idx < 10 {
			kd := bm.RenderAll()
			if idx < len(kd) && kd[idx].Agent != nil {
				a := kd[idx].Agent
				log.Printf("[action] focus: %s/%s", a.ConnName, a.PaneID)
			}
		}
	}
}

// ─── Main ──────────────────────────────────────────────────
func main() {
	log.SetFlags(log.Ltime | log.Lmsgprefix)
	log.SetPrefix("[herdr-deck] ")

	sm = state.NewManager()
	ir = render.New()
	bm = mapper.New(sm)
	st = appstate.New(sm, bm)

	// ── Load herdr data ─────────────────────────────────────
	cfg, _ := herdr.LoadConfig()
	bridge := herdr.NewBridge()
	for _, c := range cfg.Connections {
		if c.Type == "local" {
			sock := herdr.FindLocalSocket()
			if sock == "" {
				log.Printf("[main] no socket for %q", c.Name)
				continue
			}
			bridge.AddConnection(c.Name, c.Abbr, c.Color, herdr.New(sock))
		}
	}
	unified := bridge.FetchAll()
	if len(unified) == 0 {
		log.Println("[main] no herdr data, using mock")
		unified = buildMockData()
	}
	st.RefreshHerdrData(unified)
	log.Printf("[main] %d ws, %d agents", len(unified), len(sm.GetAllAgents()))

	// ── Connect to deck ─────────────────────────────────────
	dc = deck.New(onAdd, onKeyDown)
	if err := dc.Connect(); err != nil {
		log.Printf("[main] connect failed: %v", err)
	}
	st.SetDeckClient(dc)

	// ── Profile ─────────────────────────────────────────────
	pm := profile.New()
	if dir := pm.Ensure("02d04a045u3673881"); dir != "" {
		pm.ActivateProfile("02d04a045u3673881")
		if ka := pm.GetKeyActionMap(); len(ka) > 0 {
			log.Printf("[main] profile ready, %d keys", len(ka))
			st.SeedKeyActions(ka)
		}
	}

	// ── Start message pump (reconnect loop) ─────────────────
	go messagePump()

	// ── Event loop (single goroutine) ───────────────────────
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		if !st.IsDirty() {
			continue
		}
		snap := st.Capture()
		if !snap.ChangedSince(lastHash) {
			st.MarkClean()
			continue
		}
		lastHash = snap.Hash()
		renderAll()
		st.MarkClean()
	}
}

// ── messagePump: manages WebSocket reconnection ────────────
func messagePump() {
	for {
		dc.ReadPump()
		log.Println("[main] disconnected, reconnecting...")
		time.Sleep(2 * time.Second)
		if err := dc.Connect(); err != nil {
			log.Printf("[main] reconnect failed: %v", err)
			continue
		}
		log.Println("[main] reconnected")

		// Re-seed key actions for the new connection
		pm := profile.New()
		if dir := pm.Ensure("02d04a045u3673881"); dir != "" {
			if ka := pm.GetKeyActionMap(); len(ka) > 0 {
				st.SeedKeyActions(ka)
			}
		}
		// Force re-render with current data
		st.ForceDirty()
	}
}

// ── Render ─────────────────────────────────────────────────
func renderAll() {
	kd := bm.RenderAll()
	for _, k := range kd {
		var svg string
		switch {
		case k.Agent != nil:
			svg = ir.RenderAgentKey(*k.Agent)
		case k.NavAll != nil:
			svg = ir.RenderNavAll(*k.NavAll)
		case k.NavMac != nil:
			svg = ir.RenderNavMachine(*k.NavMac)
		case k.NavSpc != nil:
			svg = ir.RenderNavSpace(*k.NavSpc)
		case k.Stats != nil:
			svg = ir.RenderStatsKey(k.Stats.Stats)
		default:
			svg = ir.RenderEmptyKey()
		}
		pk := physKeyFromID(keyCommandID(k))
		if dc != nil && dc.IsConnected() {
			if err := dc.SetKeyImage(pk, svg, pk == "3_2"); err != nil {
				log.Printf("[render] %s: %v", pk, err)
			}
		}
		if k.Type() != "empty" {
			log.Printf("[render] %s (%s)", pk, k.Type())
		}
	}
	logFilterInfo()
}

func logFilterInfo() {
	all := sm.GetAllAgents()
	stats := sm.ComputeStats()
	log.Printf("[info] %d machines, %d agents | D%d I%d W%d B%d ?%d",
		len(sm.GetMachines()), len(all),
		stats.Done, stats.Idle, stats.Working, stats.Blocked, stats.Unknown)
}

// ── Mock data ──────────────────────────────────────────────
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
