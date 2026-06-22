package main

import (
	"encoding/base64"
	"fmt"
	"log"
	"time"
)

// toDataURI base64-encodes SVG and returns a data URI.
func toDataURI(svg string) string {
	return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svg))
}

// testAgentSVG generates an agent-key-style SVG.
//   status    — background color + icon (working/done/idle/blocked/unknown)
//   label     — center text
//   notchFrame — rotation frame 0-7 for the working spinner notch
func testAgentSVG(status, label string, notchFrame int) string {
	bgColor := statusColors[status]
	if bgColor == "" {
		bgColor = "#6B7280"
	}

	var statusIcon string
	switch status {
	case "working":
		angleDeg := float64(notchFrame) * 45.0
		cs, sn := cosDeg(angleDeg), sinDeg(angleDeg)
		x1 := 180.0 + 0.0*cs - (-10.0)*sn
		y1 := 180.0 + 0.0*sn + (-10.0)*cs
		x2 := 180.0 + 0.0*cs - (-6.0)*sn
		y2 := 180.0 + 0.0*sn + (-6.0)*cs
		statusIcon = fmt.Sprintf(
			`<circle cx="180" cy="180" r="8" fill="none" stroke="white" stroke-width="3"/>
<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="white" stroke-width="3" stroke-linecap="round"/>`,
			x1, y1, x2, y2)
	case "blocked":
		statusIcon = `<polyline points="180,168 192,190 168,190 180,168" fill="none" stroke="white" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"/>
<line x1="180" y1="178" x2="180" y2="184" stroke="white" stroke-width="3" stroke-linecap="round"/>
<circle cx="180" cy="187.5" r="1.5" fill="white"/>`
	case "done":
		statusIcon = `<polyline points="168,180 176,188 192,170" fill="none" stroke="white" stroke-width="4" stroke-linecap="round" stroke-linejoin="round"/>`
	default:
		statusIcon = `<line x1="174" y1="170" x2="174" y2="194" stroke="white" stroke-width="4" stroke-linecap="round"/>
<line x1="186" y1="170" x2="186" y2="194" stroke="white" stroke-width="4" stroke-linecap="round"/>`
	}

	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200">
  <rect width="200" height="200" rx="8" fill="%[1]s"/>
  <rect width="200" height="200" rx="8" fill="#000" opacity="0.15"/>
  <rect x="0" y="0" width="100" height="48" fill="#10B981"/>
  <rect x="100" y="0" width="100" height="48" fill="#4ADE80"/>
  <rect x="0" y="48" width="200" height="1" fill="#fff" opacity="0.25"/>
  <text x="50" y="32" text-anchor="middle" fill="white" font-family="sans-serif" font-size="24" font-weight="900">TEST</text>
  <text x="150" y="32" text-anchor="middle" fill="white" font-family="sans-serif" font-size="24" font-weight="900">DEV</text>
  <text x="100" y="90" text-anchor="middle" fill="white" font-family="sans-serif" font-size="36" font-weight="700">%[2]s</text>
  %[3]s
  <text x="100" y="140" text-anchor="middle" fill="white" font-family="sans-serif" font-size="26" font-weight="700">herdr-test</text>
  <text x="55" y="190" text-anchor="start" fill="white" font-family="sans-serif" font-size="20" font-weight="700">frame %[4]d</text>
</svg>`, bgColor, label, statusIcon, notchFrame)
}

// testNavSVG generates a simple navigation key SVG.
func testNavSVG(label, subLabel string) string {
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200">
  <rect width="200" height="200" rx="8" fill="#3a3a3a"/>
  <text x="100" y="90" text-anchor="middle" fill="white" font-family="sans-serif" font-size="36" font-weight="700">%s</text>
  <text x="100" y="140" text-anchor="middle" fill="#aaa" font-family="sans-serif" font-size="20">%s</text>
</svg>`, label, subLabel)
}

// testStatsSVG generates the wide stats key SVG (400x200 viewBox).
func testStatsSVG() string {
	return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 400 200">
  <rect width="400" height="200" rx="8" fill="#1a1a2e"/>
  <text x="30" y="55" fill="white" font-family="sans-serif" font-size="22" font-weight="700">D:3 I:2 W:1 B:0</text>
  <text x="30" y="100" fill="#4ADE80" font-family="sans-serif" font-size="28" font-weight="900">ONLINE</text>
  <text x="30" y="150" fill="#888" font-family="sans-serif" font-size="18">CPU:23% MEM:45%</text>
  <text x="280" y="100" fill="white" font-family="sans-serif" font-size="48" font-weight="900">TEST</text>
</svg>`
}

// ── sin/cos via Taylor (no math import needed) ────────────────

func cosDeg(deg float64) float64 {
	rad := deg * 3.141592653589793 / 180.0
	x2 := rad * rad
	return 1 - x2/2 + x2*x2/24 - x2*x2*x2/720 + x2*x2*x2*x2/40320
}

func sinDeg(deg float64) float64 {
	rad := deg * 3.141592653589793 / 180.0
	x3 := rad * rad * rad
	return rad - x3/6 + x3*rad/120 - x3*x3/5040 + x3*x3*rad/362880
}

// runSendSVG sends a static SVG to one key.
func runSendSVG(key string) {
	c := connect()
	defer c.close()

	svg := testAgentSVG("working", "SVG", 0)
	c.sendState(key, toDataURI(svg))
	log.Printf("sent SVG to key=%s", key)
	time.Sleep(1 * time.Second)
}
