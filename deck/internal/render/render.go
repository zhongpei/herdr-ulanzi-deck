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

// ─── ALL button (K11) with machine color blocks ────────────
func (r *Renderer) RenderNavAll(d viewmodel.NavAllData) string {
	var bg string
	switch {
	case !d.Active:
		bg = "#3a3a3a"
	case d.Filtered:
		bg = "#E67E22"
	default:
		bg = "#4A90D9"
	}

	label := d.Label
	if label == "" {
		label = "ALL"
	}

	// Build machine color blocks
	var blocks strings.Builder
	machines := d.Machines
	if len(machines) > 0 {
		blockW := 56
		blockH := 62
		gap := 8
		totalW := len(machines)*blockW + (len(machines)-1)*gap
		startX := (200 - totalW) / 2

		for i, m := range machines {
			x := startX + i*(blockW+gap)
			y := 18

			machineColor := m.Color
			if machineColor == "" {
				machineColor = "#6B7280"
			}

			abbrColor := "white"
			if m.Health == "offline" {
				abbrColor = "#EF4444"
			}

			count := 0
			if d.AgentCounts != nil {
				count = d.AgentCounts[m.Name]
			}

			blocks.WriteString(fmt.Sprintf(`  <rect x="%d" y="%d" width="%d" height="%d" rx="6" fill="%s"/>
  <text x="%d" y="%d" text-anchor="middle" fill="%s" font-family="sans-serif" font-size="14" font-weight="700">%s</text>
  <text x="%d" y="%d" text-anchor="middle" fill="white" font-family="sans-serif" font-size="24" font-weight="900">%d</text>
`, x, y, blockW, blockH, machineColor,
				x+blockW/2, y+22, abbrColor, escapeXML(m.Abbr),
				x+blockW/2, y+50, count))
		}
	} else {
		blocks.WriteString(`  <text x="100" y="60" text-anchor="middle" fill="#888" font-family="sans-serif" font-size="14" font-weight="400">---</text>`)
	}

	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200">
  <rect width="200" height="200" rx="8" fill="%s"/>
%s
  <text x="100" y="160" text-anchor="middle" fill="white"
        font-family="sans-serif" font-size="36" font-weight="900">%s</text>
</svg>`, bg, blocks.String(), label)

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
	var inner strings.Builder

	// ── Top row: CPU, MEM, status summary ──
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

	inner.WriteString(fmt.Sprintf(`  <text x="15" y="24" fill="white" font-family="sans-serif" font-size="18" font-weight="700">CPU</text>
  <text x="50" y="24" fill="%s" font-family="sans-serif" font-size="20" font-weight="900">%s</text>
  <text x="105" y="24" fill="white" font-family="sans-serif" font-size="18" font-weight="700">MEM</text>
  <text x="140" y="24" fill="%s" font-family="sans-serif" font-size="18" font-weight="900">%s</text>
`, cpuCol, cpuPct, memCol, memPct))

	// Status summary inline (non-zero only)
	xPos := 210
	statusItems := []struct {
		label string
		count int
		color string
	}{
		{"B", d.Stats.Blocked, "#E74C3C"},
		{"D", d.Stats.Done, "#27AE60"},
		{"W", d.Stats.Working, "#F39C12"},
		{"I", d.Stats.Idle, "#7F8C8D"},
		{"?", d.Stats.Unknown, "#95A5A6"},
	}
	for _, item := range statusItems {
		if item.count == 0 {
			continue
		}
		inner.WriteString(fmt.Sprintf(`  <text x="%d" y="24" fill="%s" font-family="sans-serif" font-size="18" font-weight="900">%s</text>
  <text x="%d" y="24" fill="white" font-family="sans-serif" font-size="20" font-weight="900">%d</text>
`, xPos, item.color, item.label, xPos+18, item.count))
		xPos += 48
	}

	inner.WriteString(`  <line x1="0" y1="32" x2="400" y2="32" stroke="#444" stroke-width="1"/>
`)

	// ── Space list (row per space) ──
	spaces := d.Spaces
	if len(spaces) == 0 {
		inner.WriteString(`  <text x="200" y="110" text-anchor="middle" fill="#666" font-family="sans-serif" font-size="22" font-weight="400">---</text>`)
	} else {
		rowY := 48
		rowH := 48
		maxRows := 4

		for i := 0; i < maxRows && i < len(spaces); i++ {
			sp := spaces[i]

			// Space label (left column, ~110px wide)
			label := sp.Label
			if len(label) > 14 {
				label = label[:14] + ".."
			}
			inner.WriteString(fmt.Sprintf(`  <text x="15" y="%d" fill="white" font-family="sans-serif" font-size="18" font-weight="700">%s</text>
`, rowY+16, escapeXML(label)))

			// Machine entries
			mx := 120
			mxEnd := 390
			for mi, mac := range sp.Machines {
				if mi > 0 && mx >= mxEnd {
					break
				}
				if mi > 0 {
					// Add 8px gap + separator before next machine
					mx += 8
					if mx >= mxEnd {
						break
					}
					inner.WriteString(fmt.Sprintf(`  <text x="%d" y="%d" fill="#555" font-family="sans-serif" font-size="16" font-weight="400">|</text>
`, mx-4, rowY+14))
					mx += 6
				}

				macColor := mac.Color
				if macColor == "" {
					macColor = "#6B7280"
				}
				// Colored square + abbreviation + total count
				inner.WriteString(fmt.Sprintf(`  <rect x="%d" y="%d" width="4" height="14" rx="1" fill="%s"/>
  <text x="%d" y="%d" fill="%s" font-family="sans-serif" font-size="14" font-weight="700">%s</text>
  <text x="%d" y="%d" fill="white" font-family="sans-serif" font-size="16" font-weight="900">%d</text>
`, mx, rowY+3, macColor,
					mx+8, rowY+14, macColor, mac.Abbr,
					mx+8+len(mac.Abbr)*8+2, rowY+14, mac.Total))

				badgeX := mx + 8 + len(mac.Abbr)*8 + 30
				badgeOrder := []struct {
					key   string
					label string
					color string
				}{
					{"blocked", "B", "#E74C3C"},
					{"done", "D", "#27AE60"},
					{"working", "W", "#F39C12"},
					{"idle", "I", "#7F8C8D"},
					{"unknown", "?", "#95A5A6"},
				}
				for _, bo := range badgeOrder {
					cnt := mac.Stats[bo.key]
					if cnt == 0 {
						continue
					}
					inner.WriteString(fmt.Sprintf(`  <text x="%d" y="%d" fill="%s" font-family="sans-serif" font-size="13" font-weight="900">%s</text>
  <text x="%d" y="%d" fill="white" font-family="sans-serif" font-size="13" font-weight="400">%d</text>
`, badgeX, rowY+14, bo.color, bo.label, badgeX+14, rowY+14, cnt))
					badgeX += 38
				}
				
			}

			rowY += rowH
		}
	}

	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 400 200">%s
</svg>`, inner.String())
	return toDataURI(svg)
}

// RenderStatsCarouselFrames returns one SVG frame per active space,
// for encoding into an animated GIF carousel (type:3).
// Each frame shows CPU/MEM/stats summary + one space in large text.
func (r *Renderer) RenderStatsCarouselFrames(d viewmodel.StatsData) []string {
	spaces := d.Spaces
	if len(spaces) == 0 {
		return []string{r.RenderStatsKey(d)}
	}
	frames := make([]string, len(spaces))
	for i, sp := range spaces {
		frames[i] = r.RenderStatsCarouselFrame(d, sp, i, len(spaces))
	}
	return frames
}

// RenderStatsCarouselFrame returns an SVG frame for ONE space,
// with large text suitable for 400×200 viewBox → 392×196 PNG.
func (r *Renderer) RenderStatsCarouselFrame(d viewmodel.StatsData, sp viewmodel.SpaceStats, idx, total int) string {
	var inner strings.Builder

	// ── Top bar: CPU, MEM, status summary ──
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

	inner.WriteString(fmt.Sprintf(`  <text x="15" y="28" fill="white" font-family="sans-serif" font-size="18" font-weight="700">CPU</text>
  <text x="55" y="28" fill="%s" font-family="sans-serif" font-size="20" font-weight="900">%s</text>
  <text x="110" y="28" fill="white" font-family="sans-serif" font-size="18" font-weight="700">MEM</text>
  <text x="150" y="28" fill="%s" font-family="sans-serif" font-size="20" font-weight="900">%s</text>
`, cpuCol, cpuPct, memCol, memPct))

	// Status summary
	xPos := 215
	statusItems := []struct {
		label string
		count int
		color string
	}{
		{"B", d.Stats.Blocked, "#E74C3C"},
		{"D", d.Stats.Done, "#27AE60"},
		{"W", d.Stats.Working, "#F39C12"},
		{"I", d.Stats.Idle, "#7F8C8D"},
		{"?", d.Stats.Unknown, "#95A5A6"},
	}
	for _, item := range statusItems {
		if item.count == 0 {
			continue
		}
		inner.WriteString(fmt.Sprintf(`  <text x="%d" y="28" fill="%s" font-family="sans-serif" font-size="16" font-weight="900">%s</text>
  <text x="%d" y="28" fill="white" font-family="sans-serif" font-size="18" font-weight="900">%d</text>
`, xPos, item.color, item.label, xPos+16, item.count))
		xPos += 42
	}

	inner.WriteString(`  <line x1="0" y1="36" x2="400" y2="36" stroke="#444" stroke-width="1"/>
`)

	// ── Space name (large, centered) ──
	label := sp.Label
	if len(label) > 20 {
		label = label[:20] + ".."
	}
	inner.WriteString(fmt.Sprintf(`  <text x="200" y="72" text-anchor="middle" fill="white" font-family="sans-serif" font-size="36" font-weight="700">%s</text>
`, escapeXML(label)))

	// ── Machine entries ──
	machineY := 90
	for _, mac := range sp.Machines {
		macColor := mac.Color
		if macColor == "" {
			macColor = "#6B7280"
		}

		// Colored square + abbreviation + total
		inner.WriteString(fmt.Sprintf(`  <rect x="15" y="%d" width="6" height="20" rx="2" fill="%s"/>
  <text x="28" y="%d" fill="%s" font-family="sans-serif" font-size="18" font-weight="700">%s</text>
  <text x="%d" y="%d" fill="white" font-family="sans-serif" font-size="18" font-weight="900">%d</text>
`, machineY, macColor, machineY+16, macColor, mac.Abbr, 28+len(mac.Abbr)*10+5, machineY+16, mac.Total))

		// Status badges
		bX := 140
		badgeOrder := []struct {
			key   string
			label string
			color string
		}{
			{"blocked", "B", "#E74C3C"},
			{"done", "D", "#27AE60"},
			{"working", "W", "#F39C12"},
			{"idle", "I", "#7F8C8D"},
			{"unknown", "?", "#95A5A6"},
		}
		for _, bo := range badgeOrder {
			cnt := mac.Stats[bo.key]
			if cnt == 0 {
				continue
			}
			inner.WriteString(fmt.Sprintf(`  <text x="%d" y="%d" fill="%s" font-family="sans-serif" font-size="16" font-weight="900">%s</text>
  <text x="%d" y="%d" fill="white" font-family="sans-serif" font-size="16" font-weight="400">%d</text>
`, bX, machineY+16, bo.color, bo.label, bX+18, machineY+16, cnt))
			bX += 42
		}

		machineY += 30
	}

	// ── Page indicator ──
	pageText := fmt.Sprintf("%d / %d", idx+1, total)
	inner.WriteString(fmt.Sprintf(`  <rect x="170" y="185" width="60" height="12" rx="6" fill="#222"/>
  <text x="200" y="194" text-anchor="middle" fill="#888" font-family="sans-serif" font-size="10" font-weight="400">%s</text>
`, pageText))

	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 400 200">%s
</svg>`, inner.String())
	return toDataURI(svg)
}
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
