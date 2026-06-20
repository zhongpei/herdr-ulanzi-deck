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
	"os"
	"time"

	"github.com/herdr-deck/herdrdeck/pkg/appstate"
	"github.com/herdr-deck/herdrdeck/pkg/deck"
	"github.com/herdr-deck/herdrdeck/pkg/herdr"
	"github.com/herdr-deck/herdrdeck/pkg/mapper"
	"github.com/herdr-deck/herdrdeck/pkg/profile"
	"github.com/herdr-deck/herdrdeck/pkg/render"
	"github.com/herdr-deck/herdrdeck/pkg/state"
	"github.com/herdr-deck/herdrdeck/pkg/types"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
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
	// Handle agent_N and empty_N — both use the same physical slot
	var idx int
	if n, _ := fmt.Sscanf(keyID, "agent_%d", &idx); n == 1 {
		if idx >= 0 && idx <= 4 {
			return fmt.Sprintf("%d_0", idx)
		}
		if idx >= 5 && idx <= 9 {
			return fmt.Sprintf("%d_1", idx-5)
		}
	}
	if n, _ := fmt.Sscanf(keyID, "empty_%d", &idx); n == 1 {
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
	tunnels  []*herdr.Tunnel
)

// ─── Callbacks (called from ReadPump goroutine) ─────────────
// MUST be lightweight: store mutation only, no I/O, no render.
func onAdd(key, actionID string) {
	log.Debug().Str("key", key).Str("actionID", actionID).Msg("action added")
}

func onKeyDown(msg deck.Message) {
	switch msg.Key {
	case "0_2", "0_3":
		st.SetAll()
		log.Debug().Str("key", msg.Key).Msg("nav: show all")
	case "1_2", "1_3":
		st.NextMachine()
		log.Debug().Str("key", msg.Key).Msg("nav: next machine")
	case "2_2":
		st.NextSpace()
		log.Debug().Str("key", msg.Key).Msg("nav: next space")
	default:
		if idx, ok := keyMap[msg.Key]; ok && idx < 10 {
			kd := bm.RenderAll()
			if idx < len(kd) && kd[idx].Agent != nil {
				a := kd[idx].Agent
				log.Info().
					Str("conn", a.ConnName).
					Str("pane", a.PaneID).
					Str("agent", a.AgentType).
					Msg("focus agent")
			}
		}
	}
}

// ─── Main ──────────────────────────────────────────────────
func main() {
	rootCmd := &cobra.Command{
		Use:   "herdr-deck",
		Short: "Display herdr agent status on Ulanzi D200X",
		Long: `Connects to UlanziDeck (port 3906) and herdr daemon to display
agent workspace status on a D200X stream deck.

Key layout: K1-K10 = agents, K11 = ALL, K12 = machine cycle,
K13 = space cycle, K14 = stats bar.`,
		RunE: runMain,
	}

	rootCmd.Flags().StringP("addr", "a", "127.0.0.1", "UlanziDeck WebSocket address")
	rootCmd.Flags().IntP("port", "p", 3906, "UlanziDeck WebSocket port")
	rootCmd.Flags().BoolP("debug", "d", false, "enable debug logging")

	if err := rootCmd.Execute(); err != nil {
		log.Fatal().Err(err).Msg("startup failed")
	}
}

func runMain(cmd *cobra.Command, args []string) error {
	addr, _ := cmd.Flags().GetString("addr")
	port, _ := cmd.Flags().GetInt("port")
	debug, _ := cmd.Flags().GetBool("debug")

	// ── Init logger ──────────────────────────────────────────
	initLogger(debug)
	log.Info().Str("addr", addr).Int("port", port).Bool("debug", debug).Msg("starting herdr-deck")

	sm = state.NewManager()
	ir = render.New()
	bm = mapper.New(sm)
	st = appstate.New(sm, bm)

	// ── Load herdr data ─────────────────────────────────────
	cfg, err := herdr.LoadConfig()
	if err != nil {
		log.Warn().Err(err).Msg("config load issue, using defaults")
	}
	bridge := herdr.NewBridge()

	// Cleanup SSH tunnels on exit
	defer func() {
		for _, tun := range tunnels {
			tun.Close()
		}
	}()

	for _, c := range cfg.Connections {
		switch c.Type {
		case "local":
			sock := herdr.FindLocalSocket()
			if sock == "" {
				log.Warn().Str("name", c.Name).Msg("no socket found for connection")
				continue
			}
			bridge.AddConnection(c.Name, c.Abbr, c.Color, herdr.New(sock))
			log.Debug().Str("name", c.Name).Str("socket", sock).Msg("added herdr connection")

		case "ssh":
			if c.Host == "" || c.RemoteSocket == "" {
				log.Warn().Str("name", c.Name).Msg("ssh connection missing host or remoteSocket")
				continue
			}
			tp := c.LocalPort
			if tp == 0 {
				tp = 19999 // default fallback
			}
			tun := herdr.NewTunnel(c.Host, c.RemoteSocket, tp)
			if err := tun.Start(); err != nil {
				log.Error().Err(err).Str("name", c.Name).Msg("SSH tunnel start failed")
				continue
			}
			tunnels = append(tunnels, tun)
			log.Debug().Str("name", c.Name).Str("host", c.Host).Int("localPort", tp).Msg("SSH tunnel started, waiting for ready...")
			if err := tun.WaitReady(10 * time.Second); err != nil {
				log.Error().Err(err).Str("name", c.Name).Msg("SSH tunnel not ready")
				tun.Close()
				continue
			}
			bridge.AddConnection(c.Name, c.Abbr, c.Color, herdr.New(tun.TargetAddr))
			log.Info().Str("name", c.Name).Str("addr", tun.TargetAddr).Msg("added SSH herdr connection")

		default:
			log.Warn().Str("name", c.Name).Str("type", c.Type).Msg("unknown connection type, skipped")
		}
	}
	unified := bridge.FetchAll()
	if len(unified) == 0 {
		log.Warn().Msg("no herdr data, using mock data")
		unified = buildMockData()
	}
	st.RefreshHerdrData(unified)
	allAgents := sm.GetAllAgents()
	log.Info().
		Int("workspaces", len(unified)).
		Int("agents", len(allAgents)).
		Msg("herdr data loaded")
	log.Debug().
		Int("machines", len(sm.GetMachines())).
		Int("done", sm.ComputeStats().Done).
		Int("idle", sm.ComputeStats().Idle).
		Int("working", sm.ComputeStats().Working).
		Int("blocked", sm.ComputeStats().Blocked).
		Int("unknown", sm.ComputeStats().Unknown).
		Msg("state summary")

	// ── Connect to deck ─────────────────────────────────────
	dc = deck.New(deck.Options{
		Address: addr,
		Port:    port,
		Debug:   debug,
	}, onAdd, onKeyDown)
	if err := dc.Connect(); err != nil {
		log.Error().Err(err).Msg("deck connect failed")
	}
	st.SetDeckClient(dc)
	log.Debug().Msg("deck client attached to store")

	// ── Profile ─────────────────────────────────────────────
	pm := profile.New()
	if dir := pm.Ensure("02d04a045u3673881"); dir != "" {
		pm.ActivateProfile("02d04a045u3673881")
		if ka := pm.GetKeyActionMap(); len(ka) > 0 {
			log.Info().Int("keys", len(ka)).Msg("profile ready, key actions seeded")
			st.SeedKeyActions(ka)
		}
	} else {
		log.Warn().Msg("profile ensure returned empty dir")
	}

	// ── Start message pump (reconnect loop) ─────────────────
	go messagePump()
	log.Debug().Msg("message pump started")

	// ── Event loop (single goroutine) ───────────────────────
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	loopIter := 0
	for range ticker.C {
		loopIter++
		if loopIter%2000 == 0 {
			log.Debug().Int("iter", loopIter).Msg("event loop heartbeat")
		}

		if !st.IsDirty() {
			continue
		}
		snap := st.Capture()
		st.MarkClean()
		if !snap.ChangedSince(lastHash) {
			log.Debug().Msg("state unchanged, skipping render")
			continue
		}
		lastHash = snap.Hash()
		log.Debug().Int("mode", int(snap.Mode)).Msg("state changed, rendering")
		renderAll()
	}
	return nil
}

// ── Logger init ───────────────────────────────────────────
func initLogger(debug bool) {
	output := zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: "15:04:05",
		NoColor:    false,
	}
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	level := zerolog.InfoLevel
	if debug {
		level = zerolog.DebugLevel
	}

	log.Logger = zerolog.New(output).
		Level(level).
		With().
		Timestamp().
		Caller().
		Logger()
}

// ── messagePump: manages WebSocket reconnection ────────────
func messagePump() {
	for {
		dc.ReadPump()
		log.Warn().Msg("deck disconnected, reconnecting in 2s...")
		time.Sleep(2 * time.Second)
		if err := dc.Connect(); err != nil {
			log.Error().Err(err).Msg("reconnect failed")
			continue
		}
		log.Info().Msg("deck reconnected")

		// Re-seed key actions for the new connection
		pm := profile.New()
		if dir := pm.Ensure("02d04a045u3673881"); dir != "" {
			if ka := pm.GetKeyActionMap(); len(ka) > 0 {
				st.SeedKeyActions(ka)
				log.Debug().Int("keys", len(ka)).Msg("re-seeded key actions")
			}
		}
		// Force re-render with current data
		st.ForceDirty()
		log.Debug().Msg("store forced dirty for re-render")
	}
}

// ── Render ─────────────────────────────────────────────────
func renderAll() {
	kd := bm.RenderAll()
	log.Debug().Int("keys", len(kd)).Msg("rendering all keys")
	for _, k := range kd {
		var svg string
		var kt string
		switch {
		case k.Agent != nil:
			svg = ir.RenderAgentKey(*k.Agent)
			kt = "agent " + k.Agent.AgentType + "/" + k.Agent.Status
			log.Debug().
				Str("agent", k.Agent.AgentType).
				Str("status", k.Agent.Status).
				Str("alias", k.Agent.Alias).
				Msg("render agent key")
		case k.NavAll != nil:
			svg = ir.RenderNavAll(*k.NavAll)
			kt = "navAll"
		case k.NavMac != nil:
			svg = ir.RenderNavMachine(*k.NavMac)
			kt = "navMachine " + k.NavMac.CurrentAbbr
		case k.NavSpc != nil:
			svg = ir.RenderNavSpace(*k.NavSpc)
			kt = "navSpace"
		case k.Stats != nil:
			svg = ir.RenderStatsKey(k.Stats.Stats)
			kt = "stats"
			log.Debug().
				Int("done", k.Stats.Stats.Done).
				Int("idle", k.Stats.Stats.Idle).
				Int("working", k.Stats.Stats.Working).
				Int("blocked", k.Stats.Stats.Blocked).
				Int("unknown", k.Stats.Stats.Unknown).
				Msg("render stats key")
		default:
			svg = ir.RenderEmptyKey()
			kt = "empty"
		}
		pk := physKeyFromID(keyCommandID(k))
		if dc != nil && dc.IsConnected() {
			if err := dc.SetKeyImage(pk, svg, pk == "3_2"); err != nil {
				log.Error().Err(err).Str("key", pk).Str("type", kt).Msg("set key image failed")
			} else {
				log.Debug().Str("key", pk).Str("type", kt).Msg("key image set OK")
			}
		} else {
			log.Warn().Str("key", pk).Str("type", kt).Msg("key image skipped (disconnected)")
		}
	}
	logFilterInfo()
}

func logFilterInfo() {
	all := sm.GetAllAgents()
	stats := sm.ComputeStats()
	machines := sm.GetMachines()
	log.Info().
		Int("machines", len(machines)).
		Int("agents", len(all)).
		Int("done", stats.Done).
		Int("idle", stats.Idle).
		Int("working", stats.Working).
		Int("blocked", stats.Blocked).
		Int("unknown", stats.Unknown).
		Msg("render cycle complete")
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
