package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"
)

// runAnimateKey cycles visible animation patterns on a single key.
// Patterns mimic the Uptime Monitor plugin:
//   blink — full-key flash inversion (like DOWN state)
//   pulse — sine-wave opacity pulse on text (like SLOW state)
func runAnimateKey(key string, fps int) {
	delay := time.Second / time.Duration(fps)
	log.Printf("animating key=%s at %d fps (%dms per frame)", key, fps, delay/time.Millisecond)
	log.Printf("pattern: blink — full-key color flash every 450ms")

	c := connect()
	defer c.close()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	ticker := time.NewTicker(delay)
	defer ticker.Stop()

	frame := 0
	for {
		select {
		case <-sig:
			log.Printf("stopped after %d frames", frame)
			return
		case <-ticker.C:
			// blinkOn toggles every 3 frames = 450ms at 6fps
			blinkOn := (frame/3)%2 == 1
			// slowPulse: sine wave opacity
			slowPulse := 0.65 + 0.35*sin(float64(frame)*0.25)
			svg := animatedTestSVG(frame, blinkOn, slowPulse)
			c.sendState(key, toDataURI(svg))
			frame++
			if frame%100 == 0 {
				log.Printf("sent %d frames", frame)
			}
		}
	}
}

// runAnimateAll cycles all 10 agent keys with visible animation.
func runAnimateAll(fps int) {
	agentKeys := []string{"0_0", "1_0", "2_0", "3_0", "4_0", "0_1", "1_1", "2_1", "3_1", "4_1"}
	delay := time.Second / time.Duration(fps)
	log.Printf("animating %d agent keys at %d fps (%dms)", len(agentKeys), fps, delay/time.Millisecond)
	log.Printf("pattern: blink — full-key color flash")

	c := connect()
	defer c.close()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	ticker := time.NewTicker(delay)
	defer ticker.Stop()

	frame := 0
	for {
		select {
		case <-sig:
			log.Printf("stopped after %d frames", frame)
			return
		case <-ticker.C:
			blinkOn := (frame/3)%2 == 1
			slowPulse := 0.65 + 0.35*sin(float64(frame)*0.25)
			for _, key := range agentKeys {
				svg := animatedTestSVG(frame, blinkOn, slowPulse)
				c.sendState(key, toDataURI(svg))
			}
			frame++
			if frame%50 == 0 {
				log.Printf("sent %d animation cycles", frame)
			}
		}
	}
}

// sin via Taylor expansion.
func sin(x float64) float64 {
	x3 := x * x * x
	return x - x3/6 + x3*x/120 - x3*x3/5040 + x3*x3*x/362880
}

// animatedTestSVG generates a test SVG with visible animation effects.
//
// Mimics Uptime Monitor's two animation modes:
//   blinkOn=true  — full-key color inversion (red ↔ dark, like DOWN state)
//   blinkOn=false — normal amber background (like UP state)
//   slowPulse     — opacity of status text pulses 0.65-1.0 (like SLOW state)
//
// Layout:
//
//	┌──────────────────────┐
//	│ ▓▓▓ TEST ▓▓▓ ▓▓ DEV ▓│
//	│──────────────────────│
//	│       WORKING        │  ← status text with slowPulse opacity
//	│                      │
//	│    ⬤  ← status dot   │  ← green dot, blinks when blinkOn
//	│       herdr-test     │
//	└──────────────────────┘
func animatedTestSVG(frame int, blinkOn bool, slowPulse float64) string {
	bg, textColor := "#F39C12", "white"
	dotColor := "#4ADE80"
	statusText := "WORKING"

	if blinkOn {
		// Full inversion: red background, dark text (like uptime DOWN flash)
		bg, textColor = "#E74C3C", "#1a1a1a"
		dotColor = "#E74C3C"
		statusText = "DOWN"
	}

	// Frame counter for reference
	frameLabel := fmt.Sprintf("F%03d", frame)

	// slowPulse on status text opacity
	opacity := fmt.Sprintf("%.2f", slowPulse)

	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200">
  <rect width="200" height="200" rx="8" fill="%s"/>
  <rect width="200" height="200" rx="8" fill="#000" opacity="0.10"/>
  <rect x="0" y="0" width="100" height="48" fill="#1a1a2e"/>
  <rect x="100" y="0" width="100" height="48" fill="#2d2d44"/>
  <rect x="0" y="48" width="200" height="1" fill="#fff" opacity="0.20"/>
  <text x="50" y="32" text-anchor="middle" fill="white" font-family="sans-serif" font-size="20" font-weight="900">TEST</text>
  <text x="150" y="32" text-anchor="middle" fill="white" font-family="sans-serif" font-size="20" font-weight="900">DEV</text>
  <text x="100" y="90" text-anchor="middle" fill="%s" font-family="sans-serif" font-size="32" font-weight="700" opacity="%s">%s</text>
  <circle cx="100" cy="130" r="10" fill="%s"/>
  <text x="100" y="175" text-anchor="middle" fill="#aaa" font-family="sans-serif" font-size="18">herdr-test</text>
  <text x="55" y="195" text-anchor="start" fill="%s" font-family="sans-serif" font-size="16">%s</text>
</svg>`, bg, textColor, opacity, statusText, dotColor, textColor, frameLabel)
}
