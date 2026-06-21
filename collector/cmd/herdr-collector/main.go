// herdr-collector: collect herdr agent status from local/remote hosts
// and publish fleet snapshots via embedded NATS.
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/herdr-deck/herdrdeck/collector/internal/bridge"
	"github.com/herdr-deck/herdrdeck/collector/internal/config"
	"github.com/herdr-deck/herdrdeck/collector/internal/fleet"
	"github.com/herdr-deck/herdrdeck/collector/internal/herdrclient"
	"github.com/herdr-deck/herdrdeck/collector/internal/natsserver"
	"github.com/herdr-deck/herdrdeck/collector/internal/publisher"
	"github.com/herdr-deck/herdrdeck/collector/internal/tunnel"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var (
	store   *fleet.Store
	br      *bridge.Bridge
	pub     *publisher.Publisher
	natsSrv *natsserver.Server
	tunnels []*tunnel.Tunnel

	// Per-connection backoff state
	failCount  = make(map[string]int)
	lastHealth = make(map[string]string) // "online" or "offline" per machine
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "herdr-collector",
		Short: "Collect herdr agent status and publish via NATS",
		Long: `herdr-collector connects to local and remote herdr daemons,
aggregates agent workspace status, and publishes FleetSnapshot on
an embedded NATS server for display processes (herdr-deck, herdr-pet).`,
		RunE: runMain,
	}

	rootCmd.Flags().IntP("nats-port", "n", 4222, "NATS server listen port")
	rootCmd.Flags().BoolP("debug", "d", false, "enable debug logging")

	if err := rootCmd.Execute(); err != nil {
		log.Fatal().Err(err).Msg("collector startup failed")
	}
}

func runMain(cmd *cobra.Command, args []string) error {
	debug, _ := cmd.Flags().GetBool("debug")
	natsPort, _ := cmd.Flags().GetInt("nats-port")

	initLogger(debug)
	log.Info().Bool("debug", debug).Int("nats-port", natsPort).Msg("starting herdr-collector")

	// ── Start embedded NATS ──────────────────────────────────
	srv, err := natsserver.New(natsserver.Options{
		Host:  "127.0.0.1",
		Port:  natsPort,
		Debug: debug,
	})
	if err != nil {
		return fmt.Errorf("nats server: %w", err)
	}
	natsSrv = srv
	defer natsSrv.Shutdown()
	log.Info().Str("url", srv.URL()).Msg("embedded NATS server started")

	// ── Connect publisher ────────────────────────────────────
	pub, err = publisher.New(srv.URL())
	if err != nil {
		return fmt.Errorf("publisher: %w", err)
	}
	defer pub.Close()
	log.Info().Msg("NATS publisher connected")

	// ── Load herdr config ────────────────────────────────────
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Warn().Err(err).Msg("config load issue, using defaults")
	}

	// ── Connect to herdr instances ───────────────────────────
	br = bridge.NewBridge()
	defer func() {
		for _, tun := range tunnels {
			tun.Close()
		}
	}()

	for _, c := range cfg.Connections {
		switch c.Type {
		case "local":
			sock := config.FindLocalSocket()
			if sock == "" {
				log.Warn().Str("name", c.Name).Msg("no socket found for connection")
				continue
			}
			br.AddConnection(c.Name, c.Abbr, c.Color, herdrclient.New(sock))
			log.Debug().Str("name", c.Name).Str("socket", sock).Msg("added herdr connection")

		case "ssh":
			if c.Host == "" || c.RemoteSocket == "" {
				log.Warn().Str("name", c.Name).Msg("ssh connection missing host or remoteSocket")
				continue
			}
			tp := c.LocalPort
			if tp == 0 {
				tp = 19999
			}
			tun := tunnel.NewTunnel(c.Host, c.RemoteSocket, tp)
			tun.SSHPort = c.SSHPort
			if err := tun.Start(); err != nil {
				log.Error().Err(err).Str("name", c.Name).Msg("SSH tunnel start failed")
				continue
			}
			tunnels = append(tunnels, tun)
			if err := tun.WaitReady(10 * time.Second); err != nil {
				log.Error().Err(err).Str("name", c.Name).Msg("SSH tunnel not ready")
				tun.Close()
				continue
			}
			br.AddConnection(c.Name, c.Abbr, c.Color, herdrclient.New(tun.TargetAddr))
			log.Info().Str("name", c.Name).Str("addr", tun.TargetAddr).Msg("added SSH herdr connection")

		default:
			log.Warn().Str("name", c.Name).Str("type", c.Type).Msg("unknown connection type, skipped")
		}
	}

	// ── Fleet store ──────────────────────────────────────────
	store = fleet.NewStore()

	// ── Initial fetch + publish ──────────────────────────────
	refreshAndPublish()

	// ── Event loop ───────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	refreshTick := time.NewTicker(2 * time.Second)
	defer refreshTick.Stop()
	heartbeatTick := time.NewTicker(1 * time.Second)
	defer heartbeatTick.Stop()

	for {
		select {
		case <-quit:
			log.Info().Msg("shutting down...")
			return nil
		case <-refreshTick.C:
			refreshAndPublish()

		case <-heartbeatTick.C:
			if err := pub.PublishHeartbeat(); err != nil {
				log.Error().Err(err).Msg("heartbeat publish failed")
			}
		}
	}
}

func refreshAndPublish() {
	results := br.FetchAll()

	// Backoff + health transition logging
	anyOnline := false
	for i := range results {
		r := &results[i]
		if r.Err == nil {
			failCount[r.ConnName] = 0
			anyOnline = true
			if lastHealth[r.ConnName] == "offline" {
				log.Info().Str("conn", r.ConnName).Msg("machine back online")
			}
			lastHealth[r.ConnName] = "online"
		} else {
			failCount[r.ConnName]++
			// Exponential backoff: skip every Nth tick
			// fail>0:  skip 0 ticks, fail>1: skip 1, fail>2: skip 3, fail>3: skip 7...
			backoffTicks := (1 << min(failCount[r.ConnName], 5)) - 1 // 1, 3, 7, 15, 31
			if failCount[r.ConnName] > 1 && (failCount[r.ConnName]-1)%backoffTicks != 0 {
				continue // skip this tick
			}
			if lastHealth[r.ConnName] != "offline" {
				log.Warn().
					Str("conn", r.ConnName).
					Int("fail_count", failCount[r.ConnName]).
					Err(r.Err).
					Msg("machine went offline, will retry with backoff")
			}
			lastHealth[r.ConnName] = "offline"
		}
	}

	changed := store.ApplyResults(results)
	snap := store.Snapshot()
	if err := pub.PublishSnapshot(&snap); err != nil {
		log.Error().Err(err).Msg("snapshot publish failed")
	}
	if changed {
		online := 0
		offline := 0
		for _, m := range snap.Machines {
			if m.Health == "offline" {
				offline++
			} else {
				online++
			}
		}
		log.Info().
			Uint64("seq", snap.Seq).
			Int("agents", len(snap.Agents)).
			Int("machines", len(snap.Machines)).
			Int("online", online).
			Int("offline", offline).
			Msg("snapshot published")
	}
	if !anyOnline && len(results) > 0 {
		log.Warn().Int("machines", len(results)).Msg("all machines offline")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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
