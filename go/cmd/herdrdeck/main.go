// herdr-deck: UlanziDeck plugin for Herdr agent status display.
//
// Entry point. Connects to UlanziDeck (port 3906) and herdr daemon (Unix socket),
// renders agent status on D200X keys.
//
// Usage:
//
//	herdrdeck                    # default: 127.0.0.1:3906
//	herdrdeck 127.0.0.1 3906    # explicit address:port
package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/herdr-deck/herdrdeck/pkg/deck"
	"github.com/herdr-deck/herdrdeck/pkg/mapper"
	"github.com/herdr-deck/herdrdeck/pkg/profile"
	"github.com/herdr-deck/herdrdeck/pkg/render"
	"github.com/herdr-deck/herdrdeck/pkg/state"
	"github.com/herdr-deck/herdrdeck/pkg/types"
)

// Physical key map for D200X (col_row format → index 0-13)
var keyMap = map[string]int{
	"0_0": 0, "1_0": 1, "2_0": 2, "3_0": 3, "4_0": 4,
	"0_1": 5, "1_1": 6, "2_1": 7, "3_1": 8, "4_1": 9,
	"0_2": 10, "1_2": 11, "2_2": 12, "3_2": 13,
}

// Reverse map: key descriptor → physical key
var descriptorMap = map[string]string{
	"nav_all":     "0_2", // K11
	"nav_machine": "1_2", // K12
	"nav_space":   "2_2", // K13
	"stats":       "3_2", // K14
}

func physicalKeyForDescriptor(keyID string) string {
	if physical, ok := descriptorMap[keyID]; ok {
		return physical
	}
	var idx int
	if _, err := fmt.Sscanf(keyID, "agent_%d", &idx); err == nil {
		if idx >= 0 && idx <= 4 {
			return fmt.Sprintf("%d_0", idx) // K1-K5
		}
		if idx >= 5 && idx <= 9 {
			return fmt.Sprintf("%d_1", idx-5) // K6-K10
		}
	}
	return "0_0" // fallback
}

// Global state
var (
	stateManager *state.Manager
	buttonMapper *mapper.Mapper
	iconRenderer *render.Renderer
	deckClient   *deck.Client
)

func main() {
	stateManager = state.NewManager()
	iconRenderer = render.New()
	buttonMapper = mapper.New(stateManager)

	// Init with mock data
	mockData := buildMockData()
	stateManager.Init(mockData)

	// Connect to deck
	deckClient = deck.New(
		func(key, actionID string) {
			log.Printf("[main] action added: key=%s actionid=%s", key, actionID)
			renderAll()
		},
		func(msg deck.Message) {
			handleKeyDown(msg)
		},
	)

	if err := deckClient.Connect(); err != nil {
		log.Printf("[main] failed to connect to deck: %v", err)
		log.Println("[main] falling back to console output for debugging")
	}

	// Start reconnect loop: if ReadPump exits, reconnect after 2s
	go func() {
		for {
			deckClient.ReadPump()
			log.Println("[main] deck disconnected, reconnecting in 2s...")
			time.Sleep(2 * time.Second)
			if err := deckClient.Connect(); err == nil {
				log.Println("[main] reconnected")
			}
		}
	}()

	// Auto-refresh on state change
	stateManager.OnChange(func(event string, data any) {
		renderAll()
	})

	// Seed key actions from profile (before first render)
	pm := profile.New()
	profileDir := pm.Ensure("02d04a045u3673881")
	if profileDir != "" {
		// Activate the profile so UlanziDeck assigns keys to our action
		pm.ActivateProfile("02d04a045u3673881")

		keyActions := pm.GetKeyActionMap()
		if len(keyActions) > 0 {
			log.Printf("[main] profile ready, %d key actions", len(keyActions))
			deckClient.SeedKeyActions(keyActions)
		}
	}

	// Render initial state
	renderAll()

	// Log filter info
	logFilterInfo()

	// Block forever
	select {}
}

// renderAll renders all 14 keys and sends them to the deck.
func renderAll() {
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
}

// keyTypeKeyID extracts the keyId from a KeyCommand.
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

// handleKeyDown processes key press events from the deck.
func handleKeyDown(msg deck.Message) {
	physKey := msg.Key
	log.Printf("[input] keydown: key=%s", physKey)

	switch physKey {
	case "0_2": // K11 — ALL
		log.Println("[nav] ALL pressed")
		buttonMapper.SetAll()
		renderAll()

	case "0_3": // hardware prev page → ALL
		log.Println("[nav] hw prev → ALL")
		buttonMapper.SetAll()
		renderAll()

	case "1_2": // K12 — next machine
		log.Println("[nav] machine cycle pressed")
		buttonMapper.NextMachine()
		renderAll()

	case "1_3": // hardware next page → next machine
		log.Println("[nav] hw next → machine cycle")
		buttonMapper.NextMachine()
		renderAll()

	case "2_2": // K13 — next space
		log.Println("[nav] space cycle pressed")
		buttonMapper.NextSpace()
		renderAll()

	default: // Agent key (K1-K10)
		if idx, ok := keyMap[physKey]; ok && idx < 10 {
			keyData := buttonMapper.RenderAll()
			if idx < len(keyData) && keyData[idx].Agent != nil {
				a := keyData[idx].Agent
				log.Printf("[action] focus: %s/%s", a.ConnName, a.PaneID)
			}
		}
	}
}

// logFilterInfo prints diagnostic information about current state.
func logFilterInfo() {
	machines := stateManager.GetMachines()
	allAgents := stateManager.GetAllAgents()
	stats := stateManager.ComputeStats()

	log.Printf("[info] %d machine(s), %d total agents", len(machines), len(allAgents))
	log.Printf("[info] stats: D%d I%d W%d B%d ?%d",
		stats.Done, stats.Idle, stats.Working, stats.Blocked, stats.Unknown)

	top10 := stateManager.GetFilteredAgents("", "")
	for i, a := range top10 {
		log.Printf("  %d. [%s] %s/%s = %s",
			i+1, a.ConnAbbr, a.Agent, a.Name, a.AgentStatus)
	}
}

// buildMockData creates test data for initial development.
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

func mainWrapper() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println("herdr-deck v0.1.0")
		return
	}
	main()
}
