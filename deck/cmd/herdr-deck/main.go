// herdr-deck: display herdr agent status on Ulanzi D200X.
//
// Receives FleetSnapshot via NATS from herdr-collector, renders 14 key
// SVG images, and pushes them to the UlanziDeck WebSocket.
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/herdr-deck/herdrdeck/deck/internal/controller"
	"github.com/herdr-deck/herdrdeck/deck/internal/deckclient"
	"github.com/herdr-deck/herdrdeck/deck/internal/fleet"
	"github.com/herdr-deck/herdrdeck/deck/internal/profile"
	"github.com/herdr-deck/herdrdeck/deck/internal/render"
	"github.com/herdr-deck/herdrdeck/deck/internal/subscriber"
	"github.com/herdr-deck/herdrdeck/deck/internal/sysstats"
	"github.com/herdr-deck/herdrdeck/deck/internal/viewmodel"
	"github.com/herdr-deck/herdrdeck/displaymodel"
	"github.com/herdr-deck/herdrdeck/protocol"
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

func keyCommandID(kc viewmodel.KeyCommand) string {
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

// ─── Globals ───────────────────────────────────────────────
var (
	fm       *fleet.Manager
	db       *displaymodel.Builder
	ir       *render.Renderer
	dc       *deckclient.Client
	ctrl     *controller.Controller
	sysColl  *sysstats.Collector
	sub      *subscriber.Subscriber
	lastHash string
)

// ─── Callbacks ─────────────────────────────────────────────
func onAdd(key, actionID string) {
	log.Debug().Str("key", key).Str("actionID", actionID).Msg("action added")
}

func onKeyDown(msg deckclient.Message) {
	switch msg.Key {
	case "0_2", "0_3":
		ctrl.OnK11()
		log.Debug().Str("key", msg.Key).Msg("nav: toggle all/act")

	case "1_2", "1_3":
		ctrl.OnK12()
		log.Debug().Str("key", msg.Key).Msg("nav: next machine")

	case "2_2":
		ctrl.OnK13()
		log.Debug().Str("key", msg.Key).Msg("nav: next space")

	default:
		if idx, ok := keyMap[msg.Key]; ok && idx < 10 {
			m := ctrl.LastModel()
			if m != nil && idx < len(m.Agents) {
				a := m.Agents[idx]
				log.Info().
					Str("conn", a.ConnName).
					Str("pane", a.PaneID).
					Str("agent", a.Agent).
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
		Long: `herdr-deck receives FleetSnapshot via NATS and renders agent
workspace status on a D200X stream deck.

Key layout: K1-K10 = agents, K11 = ALL/ACT, K12 = machine cycle,
K13 = space cycle, K14 = stats bar.`,
		RunE: runMain,
	}

	rootCmd.Flags().StringP("nats", "n", "nats://127.0.0.1:4222", "NATS server URL")
	rootCmd.Flags().StringP("addr", "a", "127.0.0.1", "UlanziDeck WebSocket address")
	rootCmd.Flags().IntP("port", "p", 3906, "UlanziDeck WebSocket port")
	rootCmd.Flags().BoolP("debug", "d", false, "enable debug logging")
	rootCmd.Flags().BoolP("k11-toggle", "k", true, "enable K11 ALL↔ACTIVE toggle")

	if err := rootCmd.Execute(); err != nil {
		log.Fatal().Err(err).Msg("deck startup failed")
	}
}

func runMain(cmd *cobra.Command, args []string) error {
	natsAddr, _ := cmd.Flags().GetString("nats")
	addr, _ := cmd.Flags().GetString("addr")
	port, _ := cmd.Flags().GetInt("port")
	debug, _ := cmd.Flags().GetBool("debug")
	k11Toggle, _ := cmd.Flags().GetBool("k11-toggle")

	initLogger(debug)
	log.Info().Str("nats", natsAddr).Str("addr", addr).Int("port", port).Bool("debug", debug).Msg("starting herdr-deck")

	// ── Connect NATS subscriber ─────────────────────────
	s, err := subscriber.New(natsAddr)
	if err != nil {
		return fmt.Errorf("subscriber: %w", err)
	}
	sub = s
	defer sub.Close()
	log.Info().Str("nats", natsAddr).Msg("NATS subscriber connected")

	// ── Init fleet + displaymodel + controller ─────────
	fm = fleet.NewManager()
	db = displaymodel.NewBuilder()
	ctrl = controller.NewController(fm, db, k11Toggle)
	ir = render.New()
	sysColl = sysstats.New()
	// Warmup: call Collect once to establish baseline so the first
	// ticker-driven collection produces real CPU delta, not 0%%.
	sysColl.Collect()

	// ── Connect to UlanziDeck ────────────────────────────
	dc = deckclient.New(deckclient.Options{
		Address: addr,
		Port:    port,
		Debug:   debug,
	}, onAdd, onKeyDown)
	if err := dc.Connect(); err != nil {
		log.Error().Err(err).Msg("deck connect failed")
	}
	log.Debug().Msg("deck client attached")

	// ── Profile ──────────────────────────────────────────
	pm := profile.New()
	if dir := pm.Ensure("02d04a045u3673881"); dir != "" {
		pm.ActivateProfile("02d04a045u3673881")
		if ka := pm.GetKeyActionMap(); len(ka) > 0 {
			log.Info().Int("keys", len(ka)).Msg("profile ready, key actions seeded")
			dc.SeedKeyActions(ka)
		}
	} else {
		log.Warn().Msg("profile ensure returned empty dir")
	}

	// ── Start message pump ──────────────────────────────
	go messagePump()
	log.Debug().Msg("message pump started")

	// ── Event loop ──────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	renderTick := time.NewTicker(50 * time.Millisecond)
	defer renderTick.Stop()
	sysTick := time.NewTicker(10 * time.Second)
	defer sysTick.Stop()
	healthTick := time.NewTicker(1 * time.Second)
	defer healthTick.Stop()

	for {
		select {
		case <-quit:
			log.Info().Msg("shutting down...")
			return nil
		case snap := <-sub.SnapshotCh():
			fm.ApplySnapshot(snap)
			ctrl.ApplySnapshot(snap)
			ctrl.MarkDirty()
			log.Debug().
				Uint64("seq", snap.Seq).
				Int("agents", len(snap.Agents)).
				Msg("snapshot applied")

		case <-sub.HeartbeatCh():
			fm.MarkHeartbeat(time.Now())

		case <-healthTick.C:
			oldHealth := fm.Health()
			newHealth := fm.CheckHealth()
			if oldHealth != newHealth {
				if newHealth == fleet.HealthOffline {
					log.Warn().Msg("collector went offline")
				} else {
					log.Info().Msg("collector back online")
				}
				ctrl.MarkDirty()
			}

		case <-sysTick.C:
			sysStats, err := sysColl.Collect()
			if err != nil {
				log.Error().Err(err).Msg("sys stats collect failed")
				continue
			}
			fm.SetSysStats(sysStats.CPUPercent, sysStats.MemoryPercent)
			ctrl.MarkDirty()
			log.Debug().
				Float64("cpu", sysStats.CPUPercent).
				Float64("mem", sysStats.MemoryPercent).
				Msg("sys stats updated")

		case <-renderTick.C:
			if !ctrl.IsDirty() {
				continue
			}
			capState := ctrl.Capture()
			ctrl.MarkClean()
			if !capState.ChangedSince(lastHash) {
				continue
			}
			lastHash = capState.Hash()
			renderAll(capState.Model)
		}
	}
}

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

		pm := profile.New()
		if dir := pm.Ensure("02d04a045u3673881"); dir != "" {
			if ka := pm.GetKeyActionMap(); len(ka) > 0 {
				dc.SeedKeyActions(ka)
				log.Debug().Int("keys", len(ka)).Msg("re-seeded key actions")
			}
		}
		dc.ResetImageCache()
		ctrl.MarkDirty()
	}
}

func renderAll(m displaymodel.Model) {
	kd := viewmodel.Adapt(m)
	offline := fm.Health() == fleet.HealthOffline

	for _, k := range kd {
		pk := physKeyFromID(keyCommandID(k))
		if dc == nil || !dc.IsConnected() {
			continue
		}

		switch {
		case k.Agent != nil:
			a := *k.Agent
			if offline {
				a.Status = "offline"
			}
			svg := ir.RenderAgentKey(a)
			if err := dc.SetKeyImage(pk, svg, false); err != nil {
				log.Error().Err(err).Str("key", pk).Msg("set key image failed")
			}
		case k.NavAll != nil:
			if err := dc.SetKeyImage(pk, ir.RenderNavAll(*k.NavAll), false); err != nil {
				log.Error().Err(err).Str("key", pk).Msg("set nav key failed")
			}
		case k.NavMac != nil:
			if err := dc.SetKeyImage(pk, ir.RenderNavMachine(*k.NavMac), false); err != nil {
				log.Error().Err(err).Str("key", pk).Msg("set nav key failed")
			}
		case k.NavSpc != nil:
			if err := dc.SetKeyImage(pk, ir.RenderNavSpace(*k.NavSpc), false); err != nil {
				log.Error().Err(err).Str("key", pk).Msg("set nav key failed")
			}
		case k.Stats != nil:
			svg := ir.RenderStatsKey(*k.Stats)
			if offline {
				s := *k.Stats
				s.Stats = protocol.AgentStats{}
				svg = ir.RenderStatsKey(s)
			}
			if err := dc.SetKeyImage(pk, svg, true); err != nil {
				log.Error().Err(err).Str("key", pk).Msg("set stats key failed")
			}
		default:
			if err := dc.SetKeyImage(pk, ir.RenderEmptyKey(), false); err != nil {
				log.Error().Err(err).Str("key", pk).Msg("set empty key failed")
			}
		}
	}

	stats := fm.ComputeStats()
	healthStr := "online"
	if offline {
		healthStr = "offline"
	}
	log.Info().
		Int("done", stats.Done).
		Int("idle", stats.Idle).
		Int("working", stats.Working).
		Int("blocked", stats.Blocked).
		Int("unknown", stats.Unknown).
		Str("health", healthStr).
		Msg("render cycle complete")
}

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
