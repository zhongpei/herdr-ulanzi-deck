package board

import (
	"fmt"
	"time"

	"gioui.org/app"
	"gioui.org/op"
	"gioui.org/unit"
	"github.com/herdr-deck/herdrdeck/displaymodel"
	"github.com/herdr-deck/herdrdeck/panel-gio/internal/alert"
	"github.com/herdr-deck/herdrdeck/panel-gio/internal/command"
	"github.com/herdr-deck/herdrdeck/panel-gio/internal/config"
	"github.com/herdr-deck/herdrdeck/panel-gio/internal/store"
	"github.com/herdr-deck/herdrdeck/panel-gio/internal/subscriber"
	"github.com/herdr-deck/herdrdeck/panel-gio/internal/tray"
	"github.com/herdr-deck/herdrdeck/protocol"
	"github.com/rs/zerolog/log"
)

const (
	refreshInterval = 200 * time.Millisecond
	healthTimeout   = 5 * time.Second
	resizeDebounce  = 3 * time.Second
)

type App struct {
	store       *store.Store
	builder     *displaymodel.Builder
	alertRule   *alert.Rule
	sub         *subscriber.Subscriber
	cmdPub      *command.Publisher
	tray        *tray.Tray
	trayView    uintptr
	debug       bool
	lastHB      time.Time
	offline     bool
	lastSnap    *displaymodel.Model
	statusSince map[string]time.Time
	input       *InputState
	anim        *AnimationState
	saveTimer   *time.Timer
	curW        unit.Dp
	curH        unit.Dp
}

func New(st *store.Store, bld *displaymodel.Builder, sub *subscriber.Subscriber, rule *alert.Rule, cmdPub *command.Publisher, debug bool) *App {
	return &App{
		store:       st,
		builder:     bld,
		sub:         sub,
		alertRule:   rule,
		cmdPub:      cmdPub,
		tray:        tray.New(st),
		debug:       debug,
		lastHB:      time.Now(),
		statusSince: make(map[string]time.Time),
		input:       NewInputState(),
	}
}

func (a *App) Run() error {
	log.Info().Msg("starting Gio Fleet Board")

	// Start system tray before app.Main() so NSStatusItem is registered
	// before the Cocoa event loop begins on darwin.
	a.tray.Start()
	defer a.tray.Stop()

	go a.gioLoop()
	app.Main()
	return nil
}

func (a *App) gioLoop() {
	cfg := config.Load(float32(WindowWidth), float32(WindowHeightMin))
	if cfg.Height < float32(WindowHeightMin) {
		cfg.Height = float32(WindowHeightMin)
	}
	savedW := unit.Dp(cfg.Width)
	snap := a.store.Snapshot()
	a.builder.SetState(a.store.ViewState())
	initM := a.builder.Build(snap, displaymodel.LocalStats{}, nil)
	estH := estimateContentHeight(initM)
	if estH < int(WindowHeightMin) {
		estH = int(WindowHeightMin)
	}
	initH := cfg.Height
	if float32(estH) > initH {
		initH = float32(estH)
	}
	a.curW = savedW
	a.curH = unit.Dp(initH)

	win := new(app.Window)
	win.Option(
		app.Title("Herdr Fleet Board"),
		app.Size(a.curW, a.curH),
		app.Decorated(false),
	)

	if a.sub != nil {
		go a.natsPump()
	}
	go a.refreshPump(win)
	go a.healthCheckPump(win)

	var ops op.Ops
	a.anim = NewAnimationState(time.Now())

	for {
		switch e := win.Event().(type) {
		case app.DestroyEvent:
			log.Debug().Msg("window destroyed")
			if a.saveTimer != nil {
				a.saveTimer.Stop()
			}
			config.Save(float32(a.curW), float32(a.curH))
			return

		case app.ViewEvent:
			if n := nativeViewHandle(e); n != 0 {
				a.trayView = n
				a.tray.SetView(n)
			}

		case app.FrameEvent:
			now := time.Now()
			gtx := app.NewContext(&ops, e)

			wDp := e.Metric.PxToDp(e.Size.X)
			hDp := e.Metric.PxToDp(e.Size.Y)
			if wDp != a.curW || hDp != a.curH {
				a.curW, a.curH = wDp, hDp
				a.scheduleSave()
			}

			phase := a.anim.Advance(now)

			snap := a.store.Snapshot()
			a.input.SyncMachines(snap)
			a.input.SyncSpaces(snap)
			snap = a.filterHiddenMachines(snap)
			a.builder.SetState(a.store.ViewState())
			durations := a.buildDurations(snap, now)
			m := a.builder.Build(snap, displaymodel.LocalStats{}, durations)

			a.input.HideWindow = func() {
				a.tray.Hide()
			}
			a.input.Clear()
			actions := HandleClicks(gtx, a.input, a.store, a.builder)
			actions = append(actions, HandleKeys(gtx, a.input, a.store, a.builder)...)
			a.handleActions(actions)

			LayoutBoard(gtx, m, snap, now, a.offline, phase, a.input, a.store.HiddenMachines())

			if a.alertRule != nil && a.lastSnap != nil {
				a.checkAlert(a.lastSnap.Agents, m.Agents, win)
			}
			a.lastSnap = &m

			e.Frame(gtx.Ops)
		}
	}
}

func (a *App) scheduleSave() {
	if a.saveTimer != nil {
		a.saveTimer.Stop()
	}
	a.saveTimer = time.AfterFunc(resizeDebounce, func() {
		config.Save(float32(a.curW), float32(a.curH))
	})
}

func (a *App) filterHiddenMachines(snap *protocol.FleetSnapshot) *protocol.FleetSnapshot {
	if snap == nil {
		return nil
	}
	hiddenMach := a.store.HiddenMachines()
	hiddenSp := a.input.HiddenSpaces
	filtered := *snap
	filtered.Agents = make([]protocol.AgentState, 0, len(snap.Agents))
	for _, ag := range snap.Agents {
		if hiddenMach[ag.Machine] {
			continue
		}
		if hiddenSp[ag.Workspace] {
			continue
		}
		filtered.Agents = append(filtered.Agents, ag)
	}
	var s protocol.AgentStats
	for _, ag := range filtered.Agents {
		switch ag.Status {
		case protocol.StatusDone:
			s.Done++
		case protocol.StatusIdle:
			s.Idle++
		case protocol.StatusWorking:
			s.Working++
		case protocol.StatusBlocked:
			s.Blocked++
		default:
			s.Unknown++
		}
	}
	filtered.Stats = s
	return &filtered
}

func estimateContentHeight(m displaymodel.Model) int {
	h := 10 + 24 + 44
	att := 0
	for _, a := range m.Agents {
		if a.Status == protocol.StatusBlocked || a.Status == protocol.StatusWorking {
			att++
		}
	}
	if att > 0 {
		h += 60
	} else {
		h += 22
	}
	machSet := map[string]struct{}{}
	for _, a := range m.Agents {
		machSet[a.ConnName] = struct{}{}
	}
	rows := 22 + len(machSet)*18
	if rows > 300 {
		rows = 300
	}
	h += rows + 30
	return h
}

func (a *App) handleActions(actions []string) {
	for _, action := range actions {
		if action == "focus-selected" && a.input.SelectedIdx >= 0 && a.input.SelectedIdx < len(a.input.Agents) && a.cmdPub != nil {
			ref := a.input.Agents[a.input.SelectedIdx]
			if err := a.cmdPub.PublishFocus(ref.PaneID, ref.Machine, ref.PaneID); err != nil {
				log.Error().Err(err).Str("agent", ref.Name).Msg("focus command failed")
			}
		}
	}
}

func (a *App) natsPump() {
	log.Debug().Msg("NATS pump started")
	for {
		select {
		case snap := <-a.sub.SnapshotCh():
			a.store.ApplySnapshot(snap)
			a.lastHB = time.Now()
			a.offline = false
		case <-a.sub.HeartbeatCh():
			a.lastHB = time.Now()
			a.store.MarkHeartbeat()
		}
	}
}

func (a *App) refreshPump(w *app.Window) {
	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()
	for range ticker.C {
		w.Invalidate()
	}
}

func (a *App) healthCheckPump(w *app.Window) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for range ticker.C {
		if time.Since(a.lastHB) > healthTimeout && !a.offline {
			a.offline = true
			a.store.MarkOffline()
			w.Invalidate()
			log.Warn().Msg("collector offline")
		}
	}
}

func (a *App) checkAlert(oldAgents, newAgents []displaymodel.AgentCell, w *app.Window) {
	oldMap := make(map[string]displaymodel.AgentCell)
	for _, ag := range oldAgents {
		oldMap[ag.PaneID] = ag
	}
	for _, ag := range newAgents {
		old, exists := oldMap[ag.PaneID]
		if (!exists && a.alertRule.ShouldAlert(ag.Status)) || (exists && old.Status != ag.Status && a.alertRule.ShouldAlert(ag.Status)) {
			w.Invalidate()
			return
		}
	}
}

func (a *App) buildDurations(snap *protocol.FleetSnapshot, now time.Time) map[string]string {
	if snap == nil {
		return nil
	}
	durations := make(map[string]string, len(snap.Agents))
	for _, ag := range snap.Agents {
		key := ag.ID
		if _, exists := a.statusSince[key]; !exists {
			a.statusSince[key] = now
		}
		durations[key] = formatDuration(now.Sub(a.statusSince[key]))
	}
	return durations
}

func formatDuration(d time.Duration) string {
	totalMin := int(d.Minutes())
	if totalMin < 1 {
		return "0m"
	}
	if totalMin < 60 {
		return fmt.Sprintf("%dm", totalMin)
	}
	hours := totalMin / 60
	mins := totalMin % 60
	if hours < 24 {
		return fmt.Sprintf("%dh%02dm", hours, mins)
	}
	return fmt.Sprintf("%dd%dh", hours/24, hours%24)
}
