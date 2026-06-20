// Package render generates SVG strings for each D200X key type.
// Mirrors src/icon-renderer.js
//
// All SVGs use 200×200 viewBox (K14 uses 400×200 for wide key).
// Physical resolution is 196×196 per key slot.
package render

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/herdr-deck/herdrdeck/pkg/types"
)

// Renderer generates base64-encoded SVG data URIs for each key type.
type Renderer struct {
	agentIcons  map[string]string
	statusIcons map[string]string
}

// New creates a Renderer with built-in agent icon paths.
func New() *Renderer {
	return &Renderer{
		agentIcons:  AgentIcons(),
		statusIcons: StatusIcons(),
	}
}

// ─── Agent key (K1-K10) ──────────────────────────────────────
//
// Layout (200×200 canvas):
//
//	┌──────────────────────┐
//	│ ▓▓▓ PI ▓▓▓  ▓▓ LCL ▓│  ← 48px top bar
//	│──────────────────────│  ← 1px white separator
//	│                      │
//	│       review         │  ← alias (36px BOLD white)
//	│                      │
//	│          W           │  ← status letter (20px)
//	│       main-proj      │  ← workspace name (14px)
//	└──────────────────────┘
//	Remaining bg = status color + black 0.15 overlay
func (r *Renderer) RenderAgentKey(d types.AgentKeyData) string {
	agentColor := lookupColor(d.AgentType, AgentColors, "#6B7280")
	statusColor := lookupColor(d.Status, StatusColors, "#95A5A6")
	statusIcon := r.statusIcons[d.Status]
	if statusIcon == "" {
		statusIcon = r.statusIcons["unknown"]
	}
	alias := escapeXML(d.Alias)
	agentName := escapeXML(d.AgentType)
	machineAbbr := escapeXML(d.ConnAbbr)
	machineColor := d.ConnAbbrColor
	if machineColor == "" {
		machineColor = "#888888"
	}
	wsLabel := escapeXML(d.WsLabel)
	borderColor := "transparent"
	borderWidth := "0"
	if d.Focused {
		borderColor = "#FFFFFF"
		borderWidth = "3"
	}

	displayAlias := truncate(alias, 9)

	// Try two-line workspace label if long enough and has separator
	wsLine1, wsLine2 := smartSplit(wsLabel)
	useTwoLine := wsLine2 != "" && len(wsLabel) > 10
	if !useTwoLine {
		wsLine1 = truncate(wsLabel, 12)
	}

	var svg string
	if !useTwoLine {
		svg = fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200">
  <rect width="200" height="200" rx="8" fill="%[1]s"/>
  <rect width="200" height="200" rx="8" fill="#000" opacity="0.15"/>
  <rect x="0" y="0" width="100" height="48" fill="%[2]s"/>
  <rect x="100" y="0" width="100" height="48" fill="%[3]s"/>
  <rect x="0" y="48" width="200" height="1" fill="#fff" opacity="0.25"/>
  <rect x="2" y="2" width="196" height="196" rx="8"
        fill="none" stroke="%[4]s" stroke-width="%[5]s"
        opacity="%[6]d"/>
  <text x="50" y="32" text-anchor="middle" fill="white"
        font-family="sans-serif" font-size="24" font-weight="800">%[7]s</text>
  <text x="150" y="32" text-anchor="middle" fill="white"
        font-family="sans-serif" font-size="24" font-weight="800">%[8]s</text>
  <text x="100" y="90" text-anchor="middle" fill="white"
        font-family="sans-serif" font-size="36" font-weight="700">%[9]s</text>
  %[10]s
  <text x="100" y="140" text-anchor="middle" fill="white"
        font-family="sans-serif" font-size="26" font-weight="700">%[11]s</text>
</svg>`,
			statusColor,
			agentColor,
			machineColor,
			borderColor, borderWidth,
			boolToInt(d.Focused),
			agentName,
			machineAbbr,
			displayAlias,
			statusIcon,
			wsLine1,
		)
	} else {
		svg = fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200">
  <rect width="200" height="200" rx="8" fill="%[1]s"/>
  <rect width="200" height="200" rx="8" fill="#000" opacity="0.15"/>
  <rect x="0" y="0" width="100" height="48" fill="%[2]s"/>
  <rect x="100" y="0" width="100" height="48" fill="%[3]s"/>
  <rect x="0" y="48" width="200" height="1" fill="#fff" opacity="0.25"/>
  <rect x="2" y="2" width="196" height="196" rx="8"
        fill="none" stroke="%[4]s" stroke-width="%[5]s"
        opacity="%[6]d"/>
  <text x="50" y="32" text-anchor="middle" fill="white"
        font-family="sans-serif" font-size="24" font-weight="800">%[7]s</text>
  <text x="150" y="32" text-anchor="middle" fill="white"
        font-family="sans-serif" font-size="24" font-weight="800">%[8]s</text>
  <text x="100" y="90" text-anchor="middle" fill="white"
        font-family="sans-serif" font-size="36" font-weight="700">%[9]s</text>
  %[10]s
  <text x="100" y="135" text-anchor="middle" fill="white"
        font-family="sans-serif" font-size="22" font-weight="700">%[11]s</text>
  <text x="100" y="160" text-anchor="middle" fill="white"
        font-family="sans-serif" font-size="22" font-weight="700">%[12]s</text>
</svg>`,
			statusColor,
			agentColor,
			machineColor,
			borderColor, borderWidth,
			boolToInt(d.Focused),
			agentName,
			machineAbbr,
			displayAlias,
			statusIcon,
			wsLine1,
			wsLine2,
		)
	}

	return toDataURI(svg)
}

// ─── ALL button (K11) ──────────────────────────────────────
func (r *Renderer) RenderNavAll(d types.NavAllData) string {
	fill := "#3a3a3a"
	if d.Active {
		fill = "#4A90D9"
	}
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200">
  <rect width="200" height="200" rx="8" fill="%s"/>
  <text x="100" y="115" text-anchor="middle" fill="white"
        font-family="sans-serif" font-size="36" font-weight="800">ALL</text>
  <rect x="155" y="178" width="40" height="18" rx="4" fill="#222" opacity="0.7"/>
  <text x="175" y="192" text-anchor="middle" fill="#00D084"
        font-family="sans-serif" font-size="16" font-weight="700">Go</text>
</svg>`, fill)
	return toDataURI(svg)
}

// ─── Machine cycle button (K12) ────────────────────────────
// Background = machine color when active, dark gray when inactive.
func (r *Renderer) RenderNavMachine(d types.NavMachineData) string {
	bgColor := "#3a3a3a"
	nextColor := "#666"
	if d.Active {
		bgColor = d.CurrentColor
		nextColor = "rgba(255,255,255,0.6)"
	}
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200">
  <rect width="200" height="200" rx="8" fill="%s" opacity="0.85"/>
  <text x="100" y="105" text-anchor="middle" fill="white"
        font-family="sans-serif" font-size="40" font-weight="800">%s</text>
  <text x="100" y="175" text-anchor="middle" fill="%s"
        font-family="sans-serif" font-size="16" font-weight="600">→ %s</text>
</svg>`,
		bgColor,
		escapeXML(d.CurrentAbbr),
		nextColor,
		escapeXML(d.NextAbbr),
	)
	return toDataURI(svg)
}

// ─── Space cycle button (K13) ─────────────────────────────
// Bold uppercase space name, auto line-break on separators.
func (r *Renderer) RenderNavSpace(d types.NavSpaceData) string {
	raw := "..."
	if d.Active && d.NextLabel != "" {
		raw = escapeXML(d.NextLabel)
	}
	upper := strings.ToUpper(raw)
	line1, line2 := smartSplit(upper)

	textY := "112"
	line2El := ""
	if line2 != "" {
		textY = "95"
		line2El = fmt.Sprintf(
			`<text x="100" y="130" text-anchor="middle" fill="white"
        font-family="sans-serif" font-size="26" font-weight="800">%s</text>`,
			line2,
		)
	}

	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200">
  <rect width="200" height="200" rx="8" fill="#333"/>
  <text x="100" y="%s" text-anchor="middle" fill="white"
        font-family="sans-serif" font-size="28" font-weight="800">%s</text>
  %s
  <text x="100" y="178" text-anchor="middle" fill="#888"
        font-family="sans-serif" font-size="14" font-weight="600">WS</text>
</svg>`,
		textY,
		line1,
		line2El,
	)
	return toDataURI(svg)
}

// ─── Stats bar (K14 - wide key) with CPU/MEM overlay ──────
// Compact agent-status stats on the right side of the bottom row.
// CPU/MEM percentages displayed at top-right with color thresholds:
//   CPU: <40% white, 40-70% yellow, >=70% red
//   MEM: <50% white, 50-80% yellow, >=80% red
func (r *Renderer) RenderStatsKey(d types.StatsData) string {
	stats := d.Stats
	items := []struct {
		Label string
		Count int
		Color string
	}{
		{"D", stats.Done, "#27AE60"},
		{"I", stats.Idle, "#7F8C8D"},
		{"W", stats.Working, "#F39C12"},
		{"B", stats.Blocked, "#E74C3C"},
		{"?", stats.Unknown, "#95A5A6"},
	}

	var inner strings.Builder
	x := 370
	step := 65
	numGap := 4
	for i := len(items) - 1; i >= 0; i-- {
		item := items[i]
		if item.Count == 0 && item.Label != "D" {
			continue
		}
		labelLine := fmt.Sprintf(`<text x="%d" y="185" text-anchor="end" fill="%s" font-family="sans-serif" font-size="28" font-weight="800">%s</text>`, x, item.Color, item.Label)
		numLine := fmt.Sprintf(`<text x="%d" y="185" text-anchor="start" fill="white" font-family="sans-serif" font-size="28" font-weight="800">%d</text>`, x+numGap, item.Count)
		inner.WriteString("\n  " + labelLine + "\n  " + numLine)
		x -= step
	}

	// CPU/MEM at top-right
	cpuPct := formatPct(d.CPUPercent)
	cpuCol := cpuColor(d.CPUPercent)
	if d.CPUPercent <= 0.01 {
		cpuPct = "--"
		cpuCol = "#555"
	}
	memPct := formatPct(d.MemoryPercent)
	memCol := memColor(d.MemoryPercent)
	if d.MemoryPercent <= 0.01 {
		memPct = "--"
		memCol = "#555"
	}

	// CPU/MEM row at top-right: "CPU 45%  MEM 62%"
	// Spaced to avoid overlap: labels ~28px, values ~32px at respective font sizes
	inner.WriteString("\n  ")
	inner.WriteString(fmt.Sprintf(`<text x="240" y="50" text-anchor="start" fill="white" font-family="sans-serif" font-size="14" font-weight="800">CPU</text>`))
	inner.WriteString("\n  ")
	inner.WriteString(fmt.Sprintf(`<text x="278" y="50" text-anchor="start" fill="%s" font-family="sans-serif" font-size="18" font-weight="800">%s</text>`, cpuCol, cpuPct))
	inner.WriteString("\n  ")
	inner.WriteString(fmt.Sprintf(`<text x="320" y="50" text-anchor="start" fill="white" font-family="sans-serif" font-size="14" font-weight="800">MEM</text>`))
	inner.WriteString("\n  ")
	inner.WriteString(fmt.Sprintf(`<text x="358" y="50" text-anchor="start" fill="%s" font-family="sans-serif" font-size="18" font-weight="800">%s</text>`, memCol, memPct))

	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 400 200">%s
</svg>`, inner.String())
	return toDataURI(svg)
}

// ─── Empty key ───────────────────────────────────────────────
func (r *Renderer) RenderEmptyKey() string {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200">
  <rect width="200" height="200" rx="8" fill="#2a2a2a" opacity="0.25"/>
</svg>`
	return toDataURI(svg)
}

// ─── Helpers ─────────────────────────────────────────────────

func toDataURI(svg string) string {
	return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svg))
}

// formatPct formats a percentage value as an integer with "%" suffix.
func formatPct(v float64) string {
	return fmt.Sprintf("%.0f", v) + "%"
}

// cpuColor returns the display color for a CPU percentage.
// <40% white, 40-70% yellow, >=70% red.
func cpuColor(pct float64) string {
	if pct >= 70 {
		return "#E74C3C"
	}
	if pct >= 40 {
		return "#F1C40F"
	}
	return "#FFFFFF"
}

// memColor returns the display color for a memory percentage.
// <50% white, 50-80% yellow, >=80% red.
func memColor(pct float64) string {
	if pct >= 80 {
		return "#E74C3C"
	}
	if pct >= 50 {
		return "#F1C40F"
	}
	return "#FFFFFF"
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

func lookupColor(key string, table map[string]string, fallback string) string {
	if v, ok := table[key]; ok {
		return v
	}
	return fallback
}

func statusFirstChar(s string) string {
	if s == "" {
		return "?"
	}
	return strings.ToUpper(s[:1])
}

func truncate(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen-1] + "…"
	}
	return s
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// smartSplit splits workspace label on dash/underscore/space for two-line display.
// Does NOT split on period ("..." placeholder must pass through intact).
func smartSplit(s string) (string, string) {
	// Don't split short labels or placeholders
	if len(s) <= 1 {
		return s, ""
	}
	sepIdx := -1
	for _, sep := range []string{"-", "_", " "} {
		if idx := strings.Index(s, sep); idx >= 0 {
			if sepIdx < 0 || idx < sepIdx {
				sepIdx = idx
			}
		}
	}
	if sepIdx < 0 {
		// Truncate if still too long
		if len(s) > 10 {
			return s[:9] + "…", ""
		}
		return s, ""
	}
	line1 := s[:sepIdx]
	line2 := s[sepIdx+1:]
	if len(line1) > 10 {
		line1 = line1[:9] + "…"
		line2 = ""
	}
	if len(line2) > 10 {
		line2 = line2[:9] + "…"
	}
	return line1, line2
}
