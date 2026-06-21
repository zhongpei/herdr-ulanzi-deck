// herdr-panel: desktop reminder panel for herdr agent status.
//
// Receives FleetSnapshot via NATS from herdr-collector and displays
// a compact 2×3 card grid of agent statuses in a Fyne window.
// Hides to system tray on close; pops up on alert-worthy status changes.
package main

import (
	"os"

	"github.com/herdr-deck/herdrdeck/displaymodel"
	"github.com/herdr-deck/herdrdeck/panel/internal/alert"
	"github.com/herdr-deck/herdrdeck/panel/internal/app"
	"github.com/herdr-deck/herdrdeck/panel/internal/state"
	"github.com/herdr-deck/herdrdeck/panel/internal/subscriber"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "herdr-panel",
		Short: "Desktop reminder panel for herdr agent status",
		Long: `herdr-panel receives FleetSnapshot via NATS and displays agent
status in a compact 2×3 card grid.

Hides to system tray on window close. Pops up automatically when
agents enter alert-worthy statuses (configurable via --alert-on).`,
		RunE: runMain,
	}

	rootCmd.Flags().StringP("nats", "n", "nats://127.0.0.1:4222", "NATS server URL")
	rootCmd.Flags().BoolP("debug", "d", false, "enable debug logging")
	rootCmd.Flags().StringP("alert-on", "a", "blocked", "comma-separated statuses that trigger window popup")

	if err := rootCmd.Execute(); err != nil {
		log.Fatal().Err(err).Msg("panel startup failed")
	}
}

func runMain(cmd *cobra.Command, args []string) error {
	natsAddr, _ := cmd.Flags().GetString("nats")
	debug, _ := cmd.Flags().GetBool("debug")
	alertOn, _ := cmd.Flags().GetString("alert-on")

	initLogger(debug)
	log.Info().Str("nats", natsAddr).Str("alert-on", alertOn).Msg("starting herdr-panel")

	// Parse alert rule
	rule, err := alert.ParseRule(alertOn)
	if err != nil {
		return err
	}
	log.Debug().Strs("watch", rule.WatchStatuses).Msg("alert rule loaded")

	// Init store + displaymodel builder
	store := state.New()
	bld := displaymodel.NewBuilder()

	// Connect NATS subscriber
	sub, err := subscriber.New(natsAddr)
	if err != nil {
		log.Error().Err(err).Msg("NATS subscriber failed, running in offline mode")
		sub = nil
	} else {
		defer sub.Close()
		log.Info().Str("nats", natsAddr).Msg("NATS subscriber connected")
	}

	// Build Fyne app
	panel, err := app.New(app.Config{
		Store:      store,
		Builder:    bld,
		Subscriber: sub,
		AlertRule:  rule,
		Debug:      debug,
	})
	if err != nil {
		return err
	}

	// Run panel (blocks until Quit from system tray / Cmd+Q / Alt+F4)
	panel.Start()
	return nil
}

func initLogger(debug bool) {
	output := zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05"}
	level := zerolog.InfoLevel
	if debug {
		level = zerolog.DebugLevel
	}
	log.Logger = zerolog.New(output).Level(level).With().Timestamp().Caller().Logger()
}
