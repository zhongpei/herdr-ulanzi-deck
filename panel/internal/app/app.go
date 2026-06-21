// Package app assembles the Fyne application lifecycle: window, tray, NATS
// data pump, and periodic UI refresh.
package app

import (
	"fmt"
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/herdr-deck/herdrdeck/displaymodel"
	"github.com/herdr-deck/herdrdeck/panel/internal/alert"
	"github.com/herdr-deck/herdrdeck/panel/internal/state"
	"github.com/herdr-deck/herdrdeck/panel/internal/subscriber"
	"github.com/herdr-deck/herdrdeck/panel/internal/ui"
	"github.com/herdr-deck/herdrdeck/protocol"
	"github.com/rs/zerolog/log"
)

const (
	appID       = "com.herdr.panel"
	refreshRate = 200 * time.Millisecond
)

type Config struct {
	Store      *state.Store
	Builder    *displaymodel.Builder
	Subscriber *subscriber.Subscriber
	AlertRule  *alert.Rule
	Debug      bool
}

type Panel struct {
	cfg    Config
	app    fyne.App
	window fyne.Window

	// Header widgets
	statsLabels []*widget.Label // 5
	toggleBtn   *widget.Button  // ALL/ACT single button
	machineRow  *fyne.Container // container rebuilt each refresh with machine buttons
	spaceSel    *widget.Select

	// Table
	tableRows   []*rowWidget
	body        *fyne.Container
	footerLabel *widget.Label

	// State
	statusSince map[string]time.Time
	lastSnap    *displaymodel.Model
	lastHB      time.Time
	healthTick  *time.Ticker
	offline     bool
}

type rowWidget struct {
	box   *fyne.Container
	color *canvas.Rectangle
	line  *widget.Label
}

func newRow() *rowWidget {
	bar := canvas.NewRectangle(ui.ColorCardBg)
	bar.SetMinSize(fyne.NewSize(6, 20))
	line := widget.NewLabel("")
	row := container.NewBorder(nil, nil, bar, nil, line)
	return &rowWidget{box: row, color: bar, line: line}
}

func New(cfg Config) (*Panel, error) {
	a := app.NewWithID(appID)
	w := a.NewWindow("Herdr Panel")
	p := &Panel{
		cfg:         cfg,
		app:         a,
		window:      w,
		statusSince: make(map[string]time.Time),
	}
	p.tableRows = make([]*rowWidget, 10)
	for i := range p.tableRows {
		p.tableRows[i] = newRow()
	}
	p.buildUI()
	w.SetCloseIntercept(func() { w.Hide() })
	w.SetPadded(false)
	w.Resize(fyne.NewSize(400, 120))
	if desk, ok := a.(desktop.App); ok {
		menu := fyne.NewMenu("Herdr Panel",
			fyne.NewMenuItem("Show Panel", func() { w.Show(); w.RequestFocus() }),
			fyne.NewMenuItem("Quit", func() { a.Quit() }),
		)
		desk.SetSystemTrayMenu(menu)
	}
	return p, nil
}

func (p *Panel) Start() {
	p.lastHB = time.Now()
	if p.cfg.Subscriber != nil {
		go p.natsPump()
	}
	go p.refreshLoop()
	p.healthTick = time.NewTicker(5 * time.Second)
	go p.healthCheckLoop()
	p.window.Show()
	p.app.Run()
}

func (p *Panel) Stop() {
	if p.healthTick != nil {
		p.healthTick.Stop()
	}
	p.app.Quit()
}

// ─── buildUI ──────────────────────────────────────────────

func (p *Panel) buildUI() {
	// Stats labels (K14) — use colored canvas.Text for status counts
	s1 := widget.NewLabel("B0") // blocked
	s2 := widget.NewLabel("D0") // done
	s3 := widget.NewLabel("W0") // working
	s4 := widget.NewLabel("I0") // idle
	s5 := widget.NewLabel("?0") // unknown
	p.statsLabels = []*widget.Label{s1, s2, s3, s4, s5}

	// ALL/ACT toggle
	p.toggleBtn = widget.NewButton("ALL", func() {
		wasAct := p.cfg.Builder.State().ActiveOnly
		p.cfg.Builder.SetAll()
		p.cfg.Builder.SetActiveOnly(!wasAct)
		p.cfg.Store.SetViewState(p.cfg.Builder.State())
	})

	// Machine buttons container (rebuilt each refresh)
	p.machineRow = container.NewHBox()

	// Space dropdown
	p.spaceSel = widget.NewSelect([]string{"ALL"}, func(sel string) {
		label := stripPrefix(sel) // remove emoji prefix
		if label == "" || label == "ALL" {
			p.cfg.Builder.SetAll()
		} else {
			p.cfg.Builder.SetState(displaymodel.ViewState{
				Mode:          displaymodel.ModeSpace,
				SelectedSpace: label,
				ActiveOnly:    p.cfg.Builder.State().ActiveOnly,
			})
		}
		p.cfg.Store.SetViewState(p.cfg.Builder.State())
	})
	p.spaceSel.SetSelected("ALL")

	// Header: stats + toggle + machines + space
	header := container.NewHBox(
		s1, s2, s3, s4, s5,
		widget.NewLabel(" "),
		p.toggleBtn,
		widget.NewLabel(" "),
		p.machineRow,
		layout.NewSpacer(),
		p.spaceSel,
	)

	p.body = container.NewVBox()
	p.footerLabel = widget.NewLabel("connecting...")
	p.footerLabel.Alignment = fyne.TextAlignCenter
	p.footerLabel.TextStyle = fyne.TextStyle{Italic: true}

	main := container.NewBorder(header, p.footerLabel, nil, nil, p.body)
	p.window.SetContent(main)
}

// ─── refreshUI ────────────────────────────────────────────

func (p *Panel) refreshUI() {
	rawSnap := p.cfg.Store.Snapshot()
	vs := p.cfg.Store.ViewState()

	// Apply hidden-machines filter before displaymodel
	snap := p.filterHiddenMachines(rawSnap)

	bld := p.cfg.Builder
	bld.SetState(vs)
	now := time.Now()
	durations := p.buildDurations(snap, now)
	model := bld.Build(snap, displaymodel.LocalStats{}, durations)

	// ── Stats ──
	stats := model.Stats.AgentStats
	p.statsLabels[0].SetText(fmt.Sprintf("B%d", stats.Blocked))
	p.statsLabels[1].SetText(fmt.Sprintf("D%d", stats.Done))
	p.statsLabels[2].SetText(fmt.Sprintf("W%d", stats.Working))
	p.statsLabels[3].SetText(fmt.Sprintf("I%d", stats.Idle))
	p.statsLabels[4].SetText(fmt.Sprintf("?%d", stats.Unknown))

	// ── ALL/ACT toggle ──
	if vs.ActiveOnly {
		p.toggleBtn.SetText("ACT")
		p.toggleBtn.Importance = widget.HighImportance
	} else {
		p.toggleBtn.SetText("ALL")
		p.toggleBtn.Importance = widget.MediumImportance
	}
	p.toggleBtn.Refresh()

	// ── Machine buttons ──
	p.machineRow.RemoveAll()
	if snap != nil {
		for _, m := range snap.Machines {
			mName := m.Name
			hidden := p.cfg.Store.IsMachineHidden(mName)
			btn := widget.NewButton(m.Abbr, func() {
				p.cfg.Store.ToggleMachine(mName)
			})
			if hidden {
				btn.Importance = widget.LowImportance
			} else {
				btn.Importance = widget.HighImportance
			}
			p.machineRow.Add(btn)
		}
	}
	p.machineRow.Refresh()

	// ── Space dropdown with status dots ──
	if snap != nil {
		wsOpts := buildWsOptions(snap.Agents)
		allOpts := make([]string, 0, len(wsOpts)+1)
		allOpts = append(allOpts, "ALL")
		allOpts = append(allOpts, wsOpts...)
		p.spaceSel.Options = allOpts
		if vs.Mode == displaymodel.ModeSpace && vs.SelectedSpace != "" {
			// Find the option with matching label (after stripping prefix)
			sel := findWsOption(allOpts, vs.SelectedSpace)
			if sel != "" {
				p.spaceSel.SetSelected(sel)
			} else {
				p.spaceSel.SetSelected("ALL")
			}
		} else {
			p.spaceSel.SetSelected("ALL")
		}
		p.spaceSel.Refresh()
	}

	// ── Table rows ──
	agents := model.Agents
	showCount := len(agents)
	if showCount > 10 {
		showCount = 10
	}
	p.body.RemoveAll()
	for i := 0; i < showCount; i++ {
		p.updateRow(i, agents[i])
		p.body.Add(p.tableRows[i].box)
	}
	p.body.Refresh()
	p.resizeToFit(showCount)

	// ── Footer ──
	health := p.cfg.Store.Health()
	if health == state.HealthOffline {
		p.footerLabel.SetText("⛔ offline")
		p.offline = true
	} else if len(agents) == 0 {
		p.footerLabel.SetText("waiting...")
		p.offline = false
	} else {
		p.footerLabel.SetText(fmt.Sprintf("%d agents", len(agents)))
		p.offline = false
	}

	// ── Alert ──
	if p.cfg.AlertRule != nil && p.lastSnap != nil {
		p.checkAlert(p.lastSnap.Agents, model.Agents)
	}
	p.lastSnap = &model
}

// filterHiddenMachines removes agents from machines that are toggled off.
func (p *Panel) filterHiddenMachines(snap *protocol.FleetSnapshot) *protocol.FleetSnapshot {
	hidden := p.cfg.Store.HiddenMachines()
	if snap == nil || len(hidden) == 0 {
		return snap
	}
	filtered := *snap
	filtered.Agents = make([]protocol.AgentState, 0, len(snap.Agents))
	for _, a := range snap.Agents {
		if !hidden[a.Machine] {
			filtered.Agents = append(filtered.Agents, a)
		}
	}
	filtered.Stats = recalcStats(filtered.Agents)
	return &filtered
}

func recalcStats(agents []protocol.AgentState) protocol.AgentStats {
	var s protocol.AgentStats
	for _, a := range agents {
		switch a.Status {
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
	return s
}

type wsInfo struct {
	label                           string
	hasBlocked, hasDone, hasWorking bool
}

// buildWsOptions returns workspace option strings with status emoji prefix,
// sorted: blocked first, done second, working third, rest last.
func buildWsOptions(agents []protocol.AgentState) []string {
	wsMap := make(map[string]*wsInfo)
	var order []string
	for _, a := range agents {
		if a.Workspace == "" {
			continue
		}
		if _, ok := wsMap[a.Workspace]; !ok {
			wsMap[a.Workspace] = &wsInfo{label: a.Workspace}
			order = append(order, a.Workspace)
		}
		info := wsMap[a.Workspace]
		switch a.Status {
		case protocol.StatusBlocked:
			info.hasBlocked = true
		case protocol.StatusDone:
			info.hasDone = true
		case protocol.StatusWorking:
			info.hasWorking = true
		}
	}
	// Sort: blocked first, done second, working third, rest
	sorted := make([]*wsInfo, 0, len(order))
	for _, l := range order {
		sorted = append(sorted, wsMap[l])
	}
	// Simple bubble sort by category priority
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if wsScore(sorted[i]) > wsScore(sorted[j]) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	out := make([]string, len(sorted))
	for i, info := range sorted {
		prefix := ""
		if info.hasBlocked {
			prefix = "[B]"
		} else if info.hasDone {
			prefix = "[D]"
		} else if info.hasWorking {
			prefix = "[W]"
		}
		out[i] = prefix + info.label
	}
	return out
}

func wsScore(info *wsInfo) int {
	if info.hasBlocked {
		return 0
	}
	if info.hasDone {
		return 1
	}
	if info.hasWorking {
		return 2
	}
	return 3
}

func stripPrefix(s string) string {
	if len(s) >= 3 && (s[:3] == "[B]" || s[:3] == "[D]" || s[:3] == "[W]") {
		return s[3:]
	}
	return s
}

func findWsOption(opts []string, target string) string {
	for _, o := range opts {
		if stripPrefix(o) == target {
			return o
		}
	}
	return ""
}

func (p *Panel) updateRow(idx int, agent displaymodel.AgentCell) {
	r := p.tableRows[idx]
	if r == nil {
		return
	}
	r.color.FillColor = ui.StatusColor(agent.Status)
	r.color.Refresh()
	name := agent.Name
	mach := agent.ConnAbbr
	if mach == "" {
		mach = "?"
	}
	ws := agent.WsLabel
	if ws == "" {
		ws = "-"
	}
	dur := agent.StatusDuration
	if dur == "" {
		dur = "-"
	}
	line := fmt.Sprintf("%-18s  %-5s  %-12s  %5s",
		truncate(name, 18), mach, truncate(ws, 12), dur)
	r.line.SetText(line)
	r.box.Show()
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) > n {
		return string(runes[:n-1]) + "…"
	}
	return s
}

// ─── durations ────────────────────────────────────────────

func (p *Panel) buildDurations(snap *protocol.FleetSnapshot, now time.Time) map[string]string {
	if snap == nil {
		return nil
	}
	durations := make(map[string]string, len(snap.Agents))
	for _, a := range snap.Agents {
		key := a.ID
		if _, exists := p.statusSince[key]; !exists {
			p.statusSince[key] = now
		}
		dur := now.Sub(p.statusSince[key])
		durations[key] = formatDuration(dur)
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
	days := hours / 24
	hours = hours % 24
	return fmt.Sprintf("%dd%dh", days, hours)
}

// ─── resize ───────────────────────────────────────────────

func (p *Panel) resizeToFit(count int) {
	const windowW float32 = 400
	const rowH float32 = 22
	const headerH float32 = 32
	const footerH float32 = 18
	const pad float32 = 4
	h := headerH + rowH*float32(count) + footerH + pad
	if h < 80 {
		h = 80
	}
	if h > 320 {
		h = 320
	}
	p.window.Resize(fyne.NewSize(windowW, h))
}

// ─── NATS ─────────────────────────────────────────────────

func (p *Panel) natsPump() {
	sub := p.cfg.Subscriber
	store := p.cfg.Store
	for {
		select {
		case snap := <-sub.SnapshotCh():
			store.ApplySnapshot(snap)
			p.lastHB = time.Now()
		case <-sub.HeartbeatCh():
			p.lastHB = time.Now()
			store.MarkHeartbeat()
		}
	}
}

func (p *Panel) healthCheckLoop() {
	for range p.healthTick.C {
		if time.Since(p.lastHB) > 5*time.Second {
			p.cfg.Store.MarkOffline()
		}
	}
}

// ─── Refresh loop ─────────────────────────────────────────

func (p *Panel) refreshLoop() {
	ticker := time.NewTicker(refreshRate)
	defer ticker.Stop()
	for range ticker.C {
		if !p.cfg.Store.IsDirty() {
			continue
		}
		fyne.Do(func() { p.refreshUI() })
	}
}

// ─── Alert ────────────────────────────────────────────────

func (p *Panel) checkAlert(oldAgents, newAgents []displaymodel.AgentCell) {
	oldMap := make(map[string]displaymodel.AgentCell)
	for _, a := range oldAgents {
		oldMap[a.PaneID] = a
	}
	triggered := false
	for _, a := range newAgents {
		old, exists := oldMap[a.PaneID]
		if !exists {
			if p.cfg.AlertRule.ShouldAlert(a.Status) {
				triggered = true
				break
			}
			continue
		}
		if old.Status != a.Status && p.cfg.AlertRule.ShouldAlert(a.Status) {
			triggered = true
			break
		}
	}
	if triggered {
		p.window.Show()
		p.window.RequestFocus()
		log.Info().Msg("alert triggered: window popped up")
	}
}

// ─── unused import anchors ────────────────────────────────

var _ = color.Color(nil)
