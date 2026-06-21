package board

import (
	"fmt"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"
	"github.com/herdr-deck/herdrdeck/displaymodel"
	"github.com/herdr-deck/herdrdeck/panel-gio/internal/store"
	"github.com/herdr-deck/herdrdeck/protocol"
)

// AgentRef holds minimal agent info for focus commands.
type AgentRef struct {
	PaneID  string
	Machine string
	Name    string
}

// InputState tracks keyboard focus, mouse clicks, and current selection.
type InputState struct {
	// HideWindow is called when the user clicks the minimize button.
	HideWindow func()
	HideBtn    widget.Clickable

	// ALL/ACT toggle
	BtnActive widget.Clickable

	// Per-machine toggle buttons (multi-select)
	MachineClicks map[string]*widget.Clickable
	MachineOrder  []string // stable order from snapshot

	// SPACES: per-space toggle buttons (multi-select)
	SpaceClicks  map[string]*widget.Clickable
	SpaceOrder   []string
	HiddenSpaces map[string]bool

	// Currently selected agent index (0 = first attention card)
	SelectedIdx int
	SelectedTag string
	AgentTags   []string
	Agents      []AgentRef
}

// NewInputState creates an empty input state.
func NewInputState() *InputState {
	return &InputState{
		SelectedIdx:   -1,
		MachineClicks: make(map[string]*widget.Clickable),
		SpaceClicks:   make(map[string]*widget.Clickable),
		HiddenSpaces:  make(map[string]bool),
	}
}

// SyncMachines ensures clickable exists for each machine in snapshot.
func (is *InputState) SyncMachines(snap *protocol.FleetSnapshot) {
	if snap == nil {
		return
	}
	// Add new machines
	for _, m := range snap.Machines {
		if _, ok := is.MachineClicks[m.Name]; !ok {
			is.MachineClicks[m.Name] = &widget.Clickable{}
		}
	}
	// Update order
	is.MachineOrder = is.MachineOrder[:0]
	for _, m := range snap.Machines {
		is.MachineOrder = append(is.MachineOrder, m.Name)
	}
}

// RegisterAgent adds an agent for keyboard selection.
func (is *InputState) RegisterAgent(tag string, ref AgentRef) {
	is.AgentTags = append(is.AgentTags, tag)
	is.Agents = append(is.Agents, ref)
}

// Clear resets per-frame state.
func (is *InputState) Clear() {
	is.AgentTags = is.AgentTags[:0]
	is.Agents = is.Agents[:0]
}

// SyncSpaces ensures clickable exists for each space in snapshot.
func (is *InputState) SyncSpaces(snap *protocol.FleetSnapshot) {
	if snap == nil {
		return
	}
	seen := map[string]bool{}
	is.SpaceOrder = is.SpaceOrder[:0]
	for _, a := range snap.Agents {
		if a.Workspace == "" || seen[a.Workspace] {
			continue
		}
		seen[a.Workspace] = true
		if _, ok := is.SpaceClicks[a.Workspace]; !ok {
			is.SpaceClicks[a.Workspace] = &widget.Clickable{}
			// Default: visible (not hidden)
			is.HiddenSpaces[a.Workspace] = false
		}
		is.SpaceOrder = append(is.SpaceOrder, a.Workspace)
	}
}

// HandleClicks checks clickable buttons and returns action strings.
func HandleClicks(gtx layout.Context, is *InputState, st *store.Store, bld *displaymodel.Builder) []string {
	var actions []string

	// ALL/ACT toggle
	if is.BtnActive.Clicked(gtx) {
		vs := bld.State()
		vs.ActiveOnly = !vs.ActiveOnly
		bld.SetState(vs)
		st.SetViewState(vs)
		actions = append(actions, "active-toggle")
	}

	// Per-machine toggle
	for name, click := range is.MachineClicks {
		if click.Clicked(gtx) {
			st.ToggleMachine(name)
			actions = append(actions, "machine-toggle-"+name)
		}
	}

	// SPACES: per-space toggle
	for name, click := range is.SpaceClicks {
		if click.Clicked(gtx) {
			is.HiddenSpaces[name] = !is.HiddenSpaces[name]
			actions = append(actions, "space-toggle-"+name)
		}
	}

	// Hide-to-tray button
	if is.HideBtn.Clicked(gtx) {
		if is.HideWindow != nil {
			is.HideWindow()
		}
		actions = append(actions, "hide")
	}

	return actions
}

// HandleKeys processes keyboard input and returns view state mutations.
func HandleKeys(gtx layout.Context, is *InputState, st *store.Store, bld *displaymodel.Builder) []string {
	var actions []string

	event.Op(gtx.Ops, is)

	for {
		e, ok := gtx.Source.Event(
			key.Filter{Name: "A"},
			key.Filter{Name: "M"},
			key.Filter{Name: "P"},
			key.Filter{Name: "R"},
			key.Filter{Name: key.NameEscape},
			key.Filter{Name: key.NameReturn},
			key.Filter{Name: key.NameEnter},
			key.Filter{Name: "1"},
			key.Filter{Name: "2"},
			key.Filter{Name: "3"},
			key.Filter{Name: "4"},
			key.Filter{Name: "5"},
			key.Filter{Name: "6"},
			key.Filter{Name: "7"},
			key.Filter{Name: "8"},
			key.Filter{Name: "9"},
		)
		if !ok {
			break
		}
		switch ev := e.(type) {
		case key.Event:
			if ev.State != key.Press {
				continue
			}
			switch ev.Name {
			case "A":
				vs := bld.State()
				vs.ActiveOnly = !vs.ActiveOnly
				bld.SetState(vs)
				st.SetViewState(vs)
				actions = append(actions, "active-toggle")
			case "M":
				bld.NextMachine(st.Snapshot())
				st.SetViewState(bld.State())
				actions = append(actions, "next-machine")
			case "P":
				bld.NextSpace(st.Snapshot())
				st.SetViewState(bld.State())
				actions = append(actions, "next-space")
			case "R":
				bld.SetAll()
				st.SetViewState(bld.State())
				is.SelectedIdx = -1
				actions = append(actions, "reset")
			case key.NameEscape:
				is.SelectedIdx = -1
				actions = append(actions, "escape")
			case key.NameReturn, key.NameEnter:
				if is.SelectedIdx >= 0 && is.SelectedIdx < len(is.AgentTags) {
					is.SelectedTag = is.AgentTags[is.SelectedIdx]
					actions = append(actions, "focus-selected")
				}
			default:
				if len(ev.Name) == 1 && ev.Name[0] >= '1' && ev.Name[0] <= '9' {
					idx := int(ev.Name[0] - '1')
					if idx < len(is.AgentTags) {
						is.SelectedIdx = idx
						is.SelectedTag = is.AgentTags[idx]
						actions = append(actions, fmt.Sprintf("select-%d", idx))
					}
				}
			}
		}
	}

	return actions
}

// ShortcutHint returns a display string for keyboard shortcuts.
func ShortcutHint() string {
	return "A:ALL/ACT  M:Machine  P:Space  R:Reset  1-9:Select  Enter:Focus  Esc:Clear"
}
