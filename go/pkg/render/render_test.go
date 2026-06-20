package render

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/herdr-deck/herdrdeck/pkg/types"
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
	svg := decodeSVG(r.RenderAgentKey(types.AgentKeyData{
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
	svg := decodeSVG(r.RenderAgentKey(types.AgentKeyData{
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
	svg := decodeSVG(r.RenderAgentKey(types.AgentKeyData{
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
	svg := decodeSVG(r.RenderAgentKey(types.AgentKeyData{
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
	svg := decodeSVG(r.RenderAgentKey(types.AgentKeyData{
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
	svg := decodeSVG(r.RenderAgentKey(types.AgentKeyData{
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
	svg := decodeSVG(r.RenderNavAll(types.NavAllData{Active: true}))
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
}

func TestRenderNavAll_Inactive(t *testing.T) {
	r := New()
	svg := decodeSVG(r.RenderNavAll(types.NavAllData{Active: false}))
	if !strings.Contains(svg, "#3a3a3a") {
		t.Error("inactive ALL button should have dark background")
	}
}

func TestRenderNavMachine_Active(t *testing.T) {
	r := New()
	svg := decodeSVG(r.RenderNavMachine(types.NavMachineData{
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
	svg := decodeSVG(r.RenderNavMachine(types.NavMachineData{
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
	svg := decodeSVG(r.RenderNavSpace(types.NavSpaceData{
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
	svg := decodeSVG(r.RenderNavSpace(types.NavSpaceData{
		Active: false,
	}))
	if !strings.Contains(svg, "...") {
		t.Error("inactive space should show '.' placeholder")
	}
}

func TestRenderNavSpace_SingleLine(t *testing.T) {
	r := New()
	svg := decodeSVG(r.RenderNavSpace(types.NavSpaceData{
		NextLabel: "SIMPLE",
		Active:    true,
	}))
	if !strings.Contains(svg, "SIMPLE") {
		t.Error("should show simple label in one line")
	}
}

func TestRenderStatsKey_Basic(t *testing.T) {
	r := New()
	svg := decodeSVG(r.RenderStatsKey(types.AgentStats{
		Done:    3,
		Idle:    2,
		Working: 4,
		Blocked: 1,
		Unknown: 0,
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
}

func TestRenderStatsKey_ZeroHidden(t *testing.T) {
	r := New()
	svg := decodeSVG(r.RenderStatsKey(types.AgentStats{
		Done:    1,
		Idle:    0,
		Working: 0,
		Blocked: 0,
		Unknown: 0,
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

func TestRenderAgentKey_AllStatusColors(t *testing.T) {
	r := New()
	statuses := []string{"done", "idle", "working", "blocked", "unknown"}
	for _, s := range statuses {
		svg := decodeSVG(r.RenderAgentKey(types.AgentKeyData{
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
