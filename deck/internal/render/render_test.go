package render

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/herdr-deck/herdrdeck/deck/internal/viewmodel"
	"github.com/herdr-deck/herdrdeck/protocol"
)

// decodeSVG decodes a data URI base64 SVG back to plain text for assertion.
func decodeSVG(dataURI string) string {
	b64 := dataURI
	if strings.HasPrefix(b64, "data:image/svg+xml;base64,") {
		b64 = b64[26:]
	}
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return ""
	}
	return string(decoded)
}

func TestRenderAgentKey_Basic(t *testing.T) {
	r := New()
	svg := decodeSVG(r.RenderAgentKey(viewmodel.AgentKeyData{
		AgentType:     "pi",
		Alias:         "review",
		Status:        "working",
		Focused:       true,
		ConnAbbr:      "LCL",
		ConnAbbrColor: "#4ADE80",
		WsLabel:       "main-proj",
	}))
	if !strings.HasPrefix(svg, `<svg`) {
		t.Fatal("expected SVG output")
	}
	if !strings.Contains(svg, "pi") {
		t.Error("expected agent name pi in SVG")
	}
	if !strings.Contains(svg, "LCL") {
		t.Error("expected machine abbr LCL in SVG")
	}
}

func TestRenderAgentKey_Truncation(t *testing.T) {
	r := New()
	svg := decodeSVG(r.RenderAgentKey(viewmodel.AgentKeyData{
		AgentType:     "unknown",
		Alias:         "very-long-agent-alias-name",
		Status:        "idle",
		ConnAbbr:      "LCL",
		ConnAbbrColor: "#888",
		WsLabel:       "super-long-workspace-name",
	}))
	if !strings.Contains(svg, "very") {
		t.Error("alias should be truncated but still contain start")
	}
}

func TestRenderAgentKey_EmptyAlias(t *testing.T) {
	r := New()
	svg := decodeSVG(r.RenderAgentKey(viewmodel.AgentKeyData{
		AgentType: "pi",
		Alias:     "",
		Status:    "done",
		ConnAbbr:  "LCL",
		WsLabel:   "test",
	}))
	if strings.Contains(svg, "pi") {
		t.Log("agent type used as fallback alias (no user alias)")
	}
}

func TestRenderAgentKey_UnknownAgentColor(t *testing.T) {
	r := New()
	svg := decodeSVG(r.RenderAgentKey(viewmodel.AgentKeyData{
		AgentType: "nonexistent-agent",
		Alias:     "test",
		Status:    "unknown",
		ConnAbbr:  "??",
		WsLabel:   "test",
	}))
	if !strings.Contains(svg, "#6B7280") {
		t.Error("unknown agent should get gray fallback color")
	}
}

func TestRenderAgentKey_FocusedBorder(t *testing.T) {
	r := New()
	svg := decodeSVG(r.RenderAgentKey(viewmodel.AgentKeyData{
		AgentType: "pi",
		Alias:     "test",
		Status:    "working",
		Focused:   true,
		ConnAbbr:  "LCL",
		WsLabel:   "test",
	}))
	if !strings.Contains(svg, `stroke="#FFFFFF"`) {
		t.Error("focused agent should have white border")
	}
	if !strings.Contains(svg, `stroke-width="3"`) {
		t.Error("focused border should be 3px")
	}
}

func TestRenderAgentKey_NotFocused(t *testing.T) {
	r := New()
	svg := decodeSVG(r.RenderAgentKey(viewmodel.AgentKeyData{
		AgentType: "pi",
		Alias:     "test",
		Status:    "idle",
		Focused:   false,
		ConnAbbr:  "LCL",
		WsLabel:   "test",
	}))
	if strings.Contains(svg, `stroke="#FFFFFF"`) {
		t.Error("non-focused agent should not have white border")
	}
}

func TestRenderNavAll_Active(t *testing.T) {
	r := New()
	svg := decodeSVG(r.RenderNavAll(viewmodel.NavAllData{Active: true}))
	if !strings.Contains(svg, "#4A90D9") {
		t.Error("active ALL button should have blue background")
	}
	if !strings.Contains(svg, "ALL") {
		t.Error("ALL button should contain ALL text")
	}
	if !strings.Contains(svg, "Go") {
		t.Error("ALL button should show Go marker")
	}
	if !strings.Contains(svg, "#00D084") {
		t.Error("Go marker should use green color")
	}
	// K11 no longer shows CPU/MEM — they moved to K14
	if strings.Contains(svg, "CPU") {
		t.Error("CPU label should NOT be on K11")
	}
}

func TestRenderNavAll_Inactive(t *testing.T) {
	r := New()
	svg := decodeSVG(r.RenderNavAll(viewmodel.NavAllData{Active: false}))
	if !strings.Contains(svg, "#3a3a3a") {
		t.Error("inactive ALL button should have dark background")
	}
}

func TestRenderNavMachine_Active(t *testing.T) {
	r := New()
	svg := decodeSVG(r.RenderNavMachine(viewmodel.NavMachineData{
		CurrentAbbr:  "LCL",
		CurrentColor: "#4ADE80",
		NextAbbr:     "DEV",
		Active:       true,
	}))
	if !strings.Contains(svg, "#4ADE80") {
		t.Error("active machine button should use machine color")
	}
	if !strings.Contains(svg, "LCL") {
		t.Error("should show current abbr")
	}
	if !strings.Contains(svg, "DEV") {
		t.Error("should show next abbr")
	}
}

func TestRenderNavMachine_Inactive(t *testing.T) {
	r := New()
	svg := decodeSVG(r.RenderNavMachine(viewmodel.NavMachineData{
		CurrentAbbr:  "-",
		CurrentColor: "#888",
		NextAbbr:     "-",
		Active:       false,
	}))
	if !strings.Contains(svg, "#3a3a3a") {
		t.Error("inactive machine button should have dark background")
	}
}

func TestRenderNavSpace_Active(t *testing.T) {
	r := New()
	svg := decodeSVG(r.RenderNavSpace(viewmodel.NavSpaceData{
		NextLabel: "main-proj",
		Count:     3,
		Active:    true,
	}))
	if !strings.Contains(svg, "MAIN") && !strings.Contains(svg, "PROJ") {
		t.Error("should show space label split on dash")
	}
	if !strings.Contains(svg, "WS") {
		t.Error("should show WS label at bottom")
	}
}

func TestRenderNavSpace_Inactive(t *testing.T) {
	r := New()
	svg := decodeSVG(r.RenderNavSpace(viewmodel.NavSpaceData{
		Active: false,
	}))
	if !strings.Contains(svg, "...") {
		t.Error("inactive space should show '.' placeholder")
	}
}

func TestRenderNavSpace_SingleLine(t *testing.T) {
	r := New()
	svg := decodeSVG(r.RenderNavSpace(viewmodel.NavSpaceData{
		NextLabel: "SIMPLE",
		Active:    true,
	}))
	if !strings.Contains(svg, "SIMPLE") {
		t.Error("should show simple label in one line")
	}
}

func TestRenderStatsKey_Basic(t *testing.T) {
	r := New()
	svg := decodeSVG(r.RenderStatsKey(viewmodel.StatsData{
		Stats: protocol.AgentStats{
			Done:    3,
			Idle:    2,
			Working: 4,
			Blocked: 1,
			Unknown: 0,
		},
		CPUPercent:    23.5,
		MemoryPercent: 45.2,
	}))
	// Each item is now two <text> elements: colored letter + white number
	if !strings.Contains(svg, ">D<") {
		t.Error("should show Done label D")
	}
	if !strings.Contains(svg, ">3<") {
		t.Error("should show Done count 3")
	}
	if !strings.Contains(svg, ">I<") {
		t.Error("should show Idle label I")
	}
	if !strings.Contains(svg, ">2<") {
		t.Error("should show Idle count 2")
	}
	if !strings.Contains(svg, ">W<") {
		t.Error("should show Working label W")
	}
	if !strings.Contains(svg, ">4<") {
		t.Error("should show Working count 4")
	}
	if !strings.Contains(svg, ">B<") {
		t.Error("should show Blocked label B")
	}
	if !strings.Contains(svg, ">1<") {
		t.Error("should show Blocked count 1")
	}
	// Numbers should be white
	if !strings.Contains(svg, `fill="white"`) {
		t.Error("numbers should be white")
	}
	// Letters should use their status colors
	if !strings.Contains(svg, `fill="#27AE60"`) {
		t.Error("D should use green")
	}
	if !strings.Contains(svg, `fill="#E74C3C"`) {
		t.Error("B should use red")
	}
	// CPU/MEM labels (white) and values should be present
	if !strings.Contains(svg, ">C<") {
		t.Error("K14 should show C label")
	}
	if !strings.Contains(svg, ">M<") {
		t.Error("K14 should show M label")
	}
	if !strings.Contains(svg, "24%") {
		t.Error("K14 should show CPU value 24%%")
	}
	if !strings.Contains(svg, "45%") {
		t.Error("K14 should show MEM value 45%%")
	}
}

func TestRenderStatsKey_ZeroHidden(t *testing.T) {
	r := New()
	svg := decodeSVG(r.RenderStatsKey(viewmodel.StatsData{
		Stats: protocol.AgentStats{
			Done:    1,
			Idle:    0,
			Working: 0,
			Blocked: 0,
			Unknown: 0,
		},
		CPUPercent:    23.5,
		MemoryPercent: 45.2,
	}))
	if !strings.Contains(svg, ">1<") {
		t.Error("should show D count 1")
	}
	if strings.Contains(svg, ">I<") {
		t.Error("should skip I0, no I label")
	}
}

func TestRenderEmptyKey(t *testing.T) {
	r := New()
	svg := decodeSVG(r.RenderEmptyKey())
	if !strings.Contains(svg, "#2a2a2a") {
		t.Error("empty key should have dark color")
	}
}

func TestEscapeXML(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"hello", "hello"},
		{"a&b", "a&amp;b"},
		{"<tag>", "&lt;tag&gt;"},
		{`"quote"`, "&quot;quote&quot;"},
		{"'single'", "&apos;single&apos;"},
	}
	for _, tc := range tests {
		got := escapeXML(tc.input)
		if got != tc.expected {
			t.Errorf("escapeXML(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestToDataURI(t *testing.T) {
	svg := `<svg></svg>`
	uri := toDataURI(svg)
	if !strings.HasPrefix(uri, "data:image/svg+xml;base64,") {
		t.Error("expected data URI prefix")
	}
	// Round-trip: decode and verify
	decoded := decodeSVG(uri)
	if decoded != svg {
		t.Errorf("round-trip: got %q, want %q", decoded, svg)
	}
}

func TestSmartSplit(t *testing.T) {
	tests := []struct {
		input     string
		wantLine1 string
		wantLine2 string
	}{
		{"MAIN-PROJ", "MAIN", "PROJ"},
		{"WEB_APP", "WEB", "APP"},
		{"SIMPLE", "SIMPLE", ""},
		{"BACKEND", "BACKEND", ""},
		{"A-B-C", "A", "B-C"},
		// "TOO-LONG-NAME-HERE" splits on first "-" → "TOO" / "LONG-NAME-HERE"
		// line2 "LONG-NAME-HERE" > 10 chars → truncated to "LONG-NAME…"
		{"TOO-LONG-NAME-HERE", "TOO", "LONG-NAME…"},
		// Periods are NOT separators (pass-through for "..." placeholder)
		{"...", "...", ""},
	}
	for _, tc := range tests {
		l1, l2 := smartSplit(tc.input)
		if l1 != tc.wantLine1 || l2 != tc.wantLine2 {
			t.Errorf("smartSplit(%q) = (%q, %q), want (%q, %q)",
				tc.input, l1, l2, tc.wantLine1, tc.wantLine2)
		}
	}
}

func TestStatusFirstChar(t *testing.T) {
	tests := []struct {
		in  string
		out string
	}{
		{"working", "W"},
		{"blocked", "B"},
		{"done", "D"},
		{"idle", "I"},
		{"unknown", "U"},
		{"", "?"},
	}
	for _, tc := range tests {
		got := statusFirstChar(tc.in)
		if got != tc.out {
			t.Errorf("statusFirstChar(%q) = %q, want %q", tc.in, got, tc.out)
		}
	}
}

func TestRenderAgentKey_StatusIconsAreSVGPrimitives(t *testing.T) {
	// Status icons should be rendered as SVG primitives (polyline/line/circle),
	// NOT as Unicode text — the D200X can't render ✓ ‖ ↻ ⚠ reliably.
	r := New()
	for _, st := range []string{"done", "idle", "working", "blocked", "unknown"} {
		svg := decodeSVG(r.RenderAgentKey(viewmodel.AgentKeyData{
			AgentType: "pi",
			Alias:     "t",
			Status:    st,
			ConnAbbr:  "L",
			WsLabel:   "w",
		}))
		hasPrimitive := strings.Contains(svg, "<polyline") ||
			strings.Contains(svg, "<line ") ||
			strings.Contains(svg, "<circle")
		if !hasPrimitive {
			t.Errorf("status %q: expected SVG primitive (polyline/line/circle) but got: %s", st, svg)
		}
		// Must NOT contain any of the broken Unicode chars.
		for _, bad := range []string{"✓", "‖", "↻", "⚠"} {
			if strings.Contains(svg, bad) {
				t.Errorf("status %q: SVG still contains broken Unicode %q", st, bad)
			}
		}
	}
}

func TestRenderAgentKey_AllStatusColors(t *testing.T) {
	r := New()
	statuses := []string{"done", "idle", "working", "blocked", "unknown"}
	for _, s := range statuses {
		svg := decodeSVG(r.RenderAgentKey(viewmodel.AgentKeyData{
			AgentType: "pi",
			Alias:     "test",
			Status:    s,
			ConnAbbr:  "LCL",
			WsLabel:   "test",
		}))
		color := StatusColors[s]
		if !strings.Contains(svg, color) {
			t.Errorf("status %q should use color %s", s, color)
		}
	}
}

func TestRenderAgentKey_TwoLineWorkspace(t *testing.T) {
	r := New()
	d := viewmodel.AgentKeyData{
		AgentType:      "pi",
		Alias:          "review",
		Status:         "working",
		ConnAbbr:       "LCL",
		ConnAbbrColor:  "#4ADE80",
		WsLabel:        "api-server",
		StatusDuration: "5m",
	}
	svg := decodeSVG(r.RenderAgentKey(d))
	if !strings.Contains(svg, "api") || !strings.Contains(svg, "server") {
		t.Errorf("two-line workspace should show both parts, got: %s", svg)
	}
}

func TestCPUColor_Thresholds(t *testing.T) {
	tests := []struct {
		pct  float64
		want string
	}{
		{0, "#FFFFFF"}, {35, "#FFFFFF"},
		{40, "#F1C40F"}, {69, "#F1C40F"},
		{70, "#E74C3C"}, {99, "#E74C3C"},
	}
	for _, tt := range tests {
		got := cpuColor(tt.pct)
		if got != tt.want {
			t.Errorf("cpuColor(%.0f) = %q, want %q", tt.pct, got, tt.want)
		}
	}
}

func TestMEMColor_Thresholds(t *testing.T) {
	tests := []struct {
		pct  float64
		want string
	}{
		{0, "#FFFFFF"}, {49, "#FFFFFF"},
		{50, "#F1C40F"}, {79, "#F1C40F"},
		{80, "#E74C3C"}, {99, "#E74C3C"},
	}
	for _, tt := range tests {
		got := memColor(tt.pct)
		if got != tt.want {
			t.Errorf("memColor(%.0f) = %q, want %q", tt.pct, got, tt.want)
		}
	}
}

func TestRenderAgentKey_OfflineStatus(t *testing.T) {
	r := New()
	d := viewmodel.AgentKeyData{
		AgentType:      "pi",
		Alias:          "offline-agent",
		Status:         "offline",
		ConnAbbr:       "LCL",
		ConnAbbrColor:  "#4ADE80",
		WsLabel:        "proj",
		StatusDuration: "0m",
	}
	svg := decodeSVG(r.RenderAgentKey(d))
	if !strings.Contains(svg, "#95A5A6") {
		t.Error("offline status falls back to unknown gray")
	}
}

func TestRenderNavAll_InactiveState(t *testing.T) {
	r := New()
	d := viewmodel.NavAllData{
		KeyID: "nav_all", Type: "navAll", Label: "ALL", Active: false,
	}
	svg := decodeSVG(r.RenderNavAll(d))
	if !strings.Contains(svg, "#3a3a3a") {
		t.Error("inactive NavAll should have dark gray background")
	}
}

func TestRenderNavAll_FilteredState(t *testing.T) {
	r := New()
	d := viewmodel.NavAllData{
		KeyID: "nav_all", Type: "navAll", Label: "ACT", Active: true, Filtered: true,
	}
	svg := decodeSVG(r.RenderNavAll(d))
	if !strings.Contains(svg, "#E67E22") {
		t.Error("filtered NavAll should have amber background")
	}
}
