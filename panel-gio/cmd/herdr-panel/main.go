// herdr-panel-gio: desktop Fleet Board panel for herdr agent status.
//
// Receives FleetSnapshot via NATS from herdr-collector and displays
// a full Fleet Board (Top Health, Lens, Attention, Matrix, Selected)
// using Gio immediate-mode GUI.
package main

import (
	"os"

	"github.com/herdr-deck/herdrdeck/displaymodel"
	"github.com/herdr-deck/herdrdeck/panel-gio/internal/alert"
	"github.com/herdr-deck/herdrdeck/panel-gio/internal/board"
	"github.com/herdr-deck/herdrdeck/panel-gio/internal/command"
	"github.com/herdr-deck/herdrdeck/panel-gio/internal/store"
	"github.com/herdr-deck/herdrdeck/panel-gio/internal/subscriber"
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
		Short: "Desktop Fleet Board for herdr agent status",
		Long: `herdr-panel receives FleetSnapshot via NATS and displays a full
Fleet Board with Top Health, Lens tracks, Attention cards, Fleet Matrix,
and Selected agent info using Gio UI.

Replaces the legacy Fyne-based panel.`,
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
	log.Info().Str("nats", natsAddr).Str("alert-on", alertOn).Msg("starting herdr-panel-gio")

	// Parse alert rule
	rule, err := alert.ParseRule(alertOn)
	if err != nil {
		return err
	}
	log.Debug().Strs("watch", rule.WatchStatuses).Msg("alert rule loaded")

	// Init store + displaymodel builder
	st := store.New()
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

	// Connect command publisher (optional)
	cmdPub, err := command.New(natsAddr)
	if err != nil {
		log.Warn().Err(err).Msg("command publisher unavailable, focus disabled")
		cmdPub = nil
	} else {
		defer cmdPub.Close()
		log.Info().Str("nats", natsAddr).Msg("command publisher connected")
	}

	panel := board.New(st, bld, sub, rule, cmdPub, debug)
	return panel.Run()
}

func initLogger(debug bool) {
	output := zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05"}
	level := zerolog.InfoLevel
	if debug {
		level = zerolog.DebugLevel
	}
	log.Logger = zerolog.New(output).Level(level).With().Timestamp().Caller().Logger()
}
