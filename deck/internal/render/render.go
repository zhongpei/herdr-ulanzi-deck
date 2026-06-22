// Package render generates SVG strings for each D200X key type.
// Mirrors src/icon-renderer.js
//
// All SVGs use 200×200 viewBox (K14 uses 400×200 for wide key).
// Physical resolution is 196×196 per key slot.
package render

import (
	"encoding/base64"
	"fmt"
	"math"
	"strings"

	"github.com/herdr-deck/herdrdeck/deck/internal/viewmodel"
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
func (r *Renderer) RenderAgentKey(d viewmodel.AgentKeyData) string {
	return r.renderAgentKeySVG(d, r.statusIconFor(d.Status))
}

func (r *Renderer) statusIconFor(status string) string {
	icon := r.statusIcons[status]
	if icon == "" {
		icon = r.statusIcons["unknown"]
	}
	return icon
}

// renderAgentKeySVG is the shared SVG template for agent keys.
// statusIconSVG is the SVG fragment for the status indicator icon.
func (r *Renderer) renderAgentKeySVG(d viewmodel.AgentKeyData, statusIconSVG string) string {
	agentColor := lookupColor(d.AgentType, AgentColors, "#6B7280")
	statusColor := lookupColor(d.Status, StatusColors, "#95A5A6")
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

	durStr := escapeXML(d.StatusDuration)

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
        font-family="sans-serif" font-size="24" font-weight="900">%[7]s</text>
  <text x="150" y="32" text-anchor="middle" fill="white"
        font-family="sans-serif" font-size="24" font-weight="900">%[8]s</text>
  <text x="100" y="90" text-anchor="middle" fill="white"
        font-family="sans-serif" font-size="36" font-weight="700">%[9]s</text>
  %[10]s
  <text x="100" y="140" text-anchor="middle" fill="white"
        font-family="sans-serif" font-size="26" font-weight="700">%[11]s</text>
  <text x="55" y="190" text-anchor="start" fill="white"
        font-family="sans-serif" font-size="20" font-weight="700">%[12]s</text>
</svg>`,
			statusColor,
			agentColor,
			machineColor,
			borderColor, borderWidth,
			boolToInt(d.Focused),
			agentName,
			machineAbbr,
			displayAlias,
			statusIconSVG,
			wsLine1,
			durStr,
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
        font-family="sans-serif" font-size="24" font-weight="900">%[7]s</text>
  <text x="150" y="32" text-anchor="middle" fill="white"
        font-family="sans-serif" font-size="24" font-weight="900">%[8]s</text>
  <text x="100" y="90" text-anchor="middle" fill="white"
        font-family="sans-serif" font-size="36" font-weight="700">%[9]s</text>
  %[10]s
  <text x="100" y="135" text-anchor="middle" fill="white"
        font-family="sans-serif" font-size="22" font-weight="700">%[11]s</text>
  <text x="100" y="160" text-anchor="middle" fill="white"
        font-family="sans-serif" font-size="22" font-weight="700">%[12]s</text>
  <text x="55" y="190" text-anchor="start" fill="white"
        font-family="sans-serif" font-size="20" font-weight="700">%[13]s</text>
</svg>`,
			statusColor,
			agentColor,
			machineColor,
			borderColor, borderWidth,
			boolToInt(d.Focused),
			agentName,
			machineAbbr,
			displayAlias,
			statusIconSVG,
			wsLine1,
			wsLine2,
			durStr,
		)
	}

	return toDataURI(svg)
}

// ─── Animated agent key frames ──────────────────────────────────

// AnimationFrames returns the number of frames and delay in milliseconds
// for a given agent status. Returns (1, 0) for static statuses.
func AnimationFrames(status string) (frames int, delayMs int) {
	switch status {
	case "working":
		return 8, 120
	case "done":
		return 8, 125 // 8 frames × 125ms = 1s pulse cycle
	default:
		return 1, 0
	}
}

// RenderAgentKeyFrame returns an SVG data URI for a single animation frame
// of an agent key status icon. totalFrames controls the animation cycle length.
// For static statuses (frame count ≤ 1), it delegates to RenderAgentKey.
func (r *Renderer) RenderAgentKeyFrame(d viewmodel.AgentKeyData, frame, totalFrames int) string {
	icon := r.statusIconFor(d.Status)
	animated := rotateStatusIconSVG(d.Status, icon, frame, totalFrames)
	return r.renderAgentKeySVG(d, animated)
}

// RenderAgentKeyFrames returns all animation frame SVGs for an agent key.
// Returns a single-element slice for static statuses.
func (r *Renderer) RenderAgentKeyFrames(d viewmodel.AgentKeyData) []string {
	frames, _ := AnimationFrames(d.Status)
	if frames <= 1 {
		return []string{r.RenderAgentKey(d)}
	}
	result := make([]string, frames)
	for i := 0; i < frames; i++ {
		result[i] = r.RenderAgentKeyFrame(d, i, frames)
	}
	return result
}

// rotateStatusIconSVG returns the SVG status icon content with rotation
// applied for the given animation frame. The rotation is computed as
// coordinate changes in the SVG output (not via SVG transform), since the
// hardware renderer does not support transform attributes.
//
// Currently animated: "working" (snake dots — 3 white dots advance
// around a circle of 8 dots like a snake).
// Other statuses are returned unchanged.
func rotateStatusIconSVG(status, icon string, frame, totalFrames int) string {
	if totalFrames <= 1 {
		return icon
	}

	switch status {
	case "working":
		type dot struct{ x, y float64 }
		dots := []dot{
			{190.0, 180.0}, {187.1, 187.1}, {180.0, 190.0}, {172.9, 187.1},
			{170.0, 180.0}, {172.9, 172.9}, {180.0, 170.0}, {187.1, 172.9},
		}

		head := frame % totalFrames
		var sb strings.Builder
		for i, d := range dots {
			dist := (i - head) % totalFrames
			if dist < 0 {
				dist += totalFrames
			}
			if dist < 3 {
				sb.WriteString(fmt.Sprintf(`<circle cx="%.1f" cy="%.1f" r="3" fill="white"/>
`, d.x, d.y))
			} else {
				sb.WriteString(fmt.Sprintf(`<circle cx="%.1f" cy="%.1f" r="3" fill="#888" opacity="0.3"/>
`, d.x, d.y))
			}
		}
		return sb.String()

	case "done":
		// Sharp pulse: dim (0.10), brief bright (1.00) flash
		// 8 frames × 125ms = 1s cycle — visible like notification LED
		if frame%8 < 4 {
			return `<circle cx="180" cy="180" r="10" fill="white" opacity="0.10"/>`
		}
		return `<circle cx="180" cy="180" r="10" fill="white" opacity="1.00"/>`

	default:
		return icon
	}
}

// rotatePoint rotates point (x, y) around center (cx, cy) by angleDeg degrees.
func rotatePoint(x, y, cx, cy, angleDeg float64) (float64, float64) {
	rad := angleDeg * math.Pi / 180.0
	cos := math.Cos(rad)
	sin := math.Sin(rad)
	dx := x - cx
	dy := y - cy
	return cx + dx*cos - dy*sin, cy + dx*sin + dy*cos
}

// ─── ALL button (K11) ──────────────────────────────────────
func (r *Renderer) RenderNavAll(d viewmodel.NavAllData) string {
	var fill string
	switch {
	case !d.Active:
		fill = "#3a3a3a" // inactive
	case d.Filtered:
		fill = "#E67E22" // active + filtered → amber
	default:
		fill = "#4A90D9" // active + unfiltered → blue
	}
	label := d.Label
	if label == "" {
		label = "ALL"
	}
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200">
  <rect width="200" height="200" rx="8" fill="%s"/>
  <text x="100" y="115" text-anchor="middle" fill="white"
        font-family="sans-serif" font-size="36" font-weight="900">%s</text>
  <rect x="155" y="178" width="40" height="18" rx="4" fill="#222" opacity="0.7"/>
  <text x="175" y="192" text-anchor="middle" fill="#00D084"
        font-family="sans-serif" font-size="16" font-weight="700">Go</text>
</svg>`, fill, label)
	return toDataURI(svg)
}

// RenderNavAllFrames returns 8 animated SVG frames for K11 machine status.
func (r *Renderer) RenderNavAllFrames(d viewmodel.NavAllData) []string {
	frames := make([]string, 8)
	for i := 0; i < 8; i++ {
		frames[i] = r.RenderNavAllFrame(d, i, 8)
	}
	return frames
}

// RenderNavAllFrame returns one animation frame of K11 machine status.
// Online machines get a green breathing dot (sine-wave opacity 0.3↔1.0).
// Offline machines get a red blinking dot (square-wave 0.1↔1.0).
func (r *Renderer) RenderNavAllFrame(d viewmodel.NavAllData, frame, totalFrames int) string {
	var fill string
	switch {
	case !d.Active:
		fill = "#3a3a3a"
	case d.Filtered:
		fill = "#E67E22"
	default:
		fill = "#4A90D9"
	}

	label := d.Label
	if label == "" {
		label = "ALL"
	}

	var body strings.Builder
	machines := d.Machines

	if len(machines) == 0 {
		body.WriteString(`<text x="100" y="55" text-anchor="middle" fill="#888" font-family="sans-serif" font-size="14" font-weight="400">---</text>`)
	} else {
		spacing := 60
		totalW := len(machines) * spacing
		startX := (200-totalW)/2 + 15

		for i, m := range machines {
			x := startX + i*spacing - 15
			tx := x + 20

			opacity := 0.3
			switch m.Health {
			case "online":
				phase := float64(frame) * 2.0 * math.Pi / float64(totalFrames)
				opacity = 0.3 + 0.7*(math.Sin(phase)+1.0)/2.0
			case "offline":
				if frame < totalFrames/2 {
					opacity = 1.0
				} else {
					opacity = 0.1
				}
			}

			dotColor := "#4ADE80"
			if m.Health == "offline" {
				dotColor = "#EF4444"
			}

			body.WriteString(fmt.Sprintf(
				`<circle cx="%d" cy="55" r="5" fill="%s" opacity="%.2f"/>
<text x="%d" y="60" fill="white" font-family="sans-serif" font-size="14" font-weight="700">%s</text>
`, x, dotColor, opacity, tx, escapeXML(m.Abbr)))
		}
	}

	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200">
  <rect width="200" height="200" rx="8" fill="%[1]s"/>
  %[2]s
  <text x="100" y="140" text-anchor="middle" fill="white"
        font-family="sans-serif" font-size="36" font-weight="900">%[3]s</text>
</svg>`, fill, body.String(), label)

	return toDataURI(svg)
}

// ─── Machine cycle button (K12) ────────────────────────────
// Background = machine color when active, dark gray when inactive.
func (r *Renderer) RenderNavMachine(d viewmodel.NavMachineData) string {
	bgColor := "#3a3a3a"
	nextColor := "#666"
	if d.Active {
		bgColor = d.CurrentColor
		nextColor = "rgba(255,255,255,0.6)"
	}
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200">
  <rect width="200" height="200" rx="8" fill="%s" opacity="0.85"/>
  <text x="100" y="105" text-anchor="middle" fill="white"
        font-family="sans-serif" font-size="40" font-weight="900">%s</text>
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
func (r *Renderer) RenderNavSpace(d viewmodel.NavSpaceData) string {
	// Current space label (main text) — defaults to "..." when empty or "-".
	current := d.CurrentLabel
	if current == "" || current == "-" {
		current = "..."
	}
	if !d.Active {
		current = "..."
	}
	upper := strings.ToUpper(escapeXML(current))
	line1, line2 := smartSplit(upper)

	// Layout: two-line (y=74/110, 30/26px) or single-line (y=86, 30px).
	// Next hint at y=168/158, 24px, #BBB.
	textY := "86"
	mainSize := "30"
	line2El := ""
	if line2 != "" {
		textY = "74"
		mainSize = "30"
		line2El = fmt.Sprintf(
			`<text x="100" y="110" text-anchor="middle" fill="white"
        font-family="sans-serif" font-size="26" font-weight="900">%s</text>`,
			line2,
		)
	}

	// Next space hint at bottom (like K12 "→ DEV")
	hintY := "158"
	if line2 != "" {
		hintY = "168"
	}
	hint := ""
	if d.Active && d.NextLabel != "" {
		hint = fmt.Sprintf(
			`<text x="100" y="%s" text-anchor="middle" fill="#BBB"
        font-family="sans-serif" font-size="24" font-weight="700">→ %s</text>`,
			hintY,
			escapeXML(d.NextLabel),
		)
	} else {
		hint = fmt.Sprintf(`<text x="100" y="%s" text-anchor="middle" fill="#BBB"
        font-family="sans-serif" font-size="24" font-weight="700">WS</text>`,
			hintY)
	}

	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200">
  <rect width="200" height="200" rx="8" fill="#333"/>
  <text x="100" y="%s" text-anchor="middle" fill="white"
        font-family="sans-serif" font-size="%s" font-weight="900">%s</text>
  %s
  %s
</svg>`,
		textY,
		mainSize,
		line1,
		line2El,
		hint,
	)
	return toDataURI(svg)
}

// ─── Stats bar (K14 - wide key) with CPU/MEM overlay ──────
// Compact agent-status stats on the right side of the bottom row.
// CPU/MEM percentages displayed at top-right with color thresholds:
//
//	CPU: <40% white, 40-70% yellow, >=70% red
//	MEM: <50% white, 50-80% yellow, >=80% red
func (r *Renderer) RenderStatsKey(d viewmodel.StatsData) string {
	stats := d.Stats
	items := []struct {
		Label string
		Count int
		Color string
	}{
		{"B", stats.Blocked, "#E74C3C"},
		{"D", stats.Done, "#27AE60"},
		{"W", stats.Working, "#F39C12"},
		{"I", stats.Idle, "#7F8C8D"},
		{"?", stats.Unknown, "#95A5A6"},
	}

	var inner strings.Builder
	x := 365
	step := 65
	numGap := 4
	for i := len(items) - 1; i >= 0; i-- {
		item := items[i]
		if item.Count == 0 && item.Label != "D" {
			continue
		}
		labelLine := fmt.Sprintf(`<text x="%d" y="185" text-anchor="end" fill="%s" font-family="sans-serif" font-size="28" font-weight="900">%s</text>`, x, item.Color, item.Label)
		numLine := fmt.Sprintf(`<text x="%d" y="185" text-anchor="start" fill="white" font-family="sans-serif" font-size="28" font-weight="900">%d</text>`, x+numGap, item.Count)
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

	// CPU/MEM row at top: "C 45%  M 62%" — abbreviated single-char labels
	// so the clock on K14 doesn't clip the text. Font: labels 20pt bold white,
	// values 24pt bold colored (approaching the 28pt stats-bar font below).
	inner.WriteString("\n  ")
	inner.WriteString(fmt.Sprintf(`<text x="220" y="50" text-anchor="start" fill="white" font-family="sans-serif" font-size="20" font-weight="900">C</text>`))
	inner.WriteString("\n  ")
	inner.WriteString(fmt.Sprintf(`<text x="245" y="50" text-anchor="start" fill="%s" font-family="sans-serif" font-size="24" font-weight="900">%s</text>`, cpuCol, cpuPct))
	inner.WriteString("\n  ")
	inner.WriteString(fmt.Sprintf(`<text x="315" y="50" text-anchor="start" fill="white" font-family="sans-serif" font-size="20" font-weight="900">M</text>`))
	inner.WriteString("\n  ")
	inner.WriteString(fmt.Sprintf(`<text x="340" y="50" text-anchor="start" fill="%s" font-family="sans-serif" font-size="24" font-weight="900">%s</text>`, memCol, memPct))

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
