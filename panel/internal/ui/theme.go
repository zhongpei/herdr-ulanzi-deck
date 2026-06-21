// Package ui provides Fyne widgets for the herdr-panel.
package ui

import (
	"image/color"

	"github.com/herdr-deck/herdrdeck/protocol"
)

// Status colors for agent state cards.
var (
	ColorBlocked = color.RGBA{R: 0xE7, G: 0x4C, B: 0x3C, A: 0xFF} // red
	ColorDone    = color.RGBA{R: 0x27, G: 0xAE, B: 0x60, A: 0xFF} // green
	ColorWorking = color.RGBA{R: 0xF3, G: 0x9C, B: 0x12, A: 0xFF} // orange
	ColorIdle    = color.RGBA{R: 0x7F, G: 0x8C, B: 0x8D, A: 0xFF} // gray
	ColorUnknown = color.RGBA{R: 0x95, G: 0xA5, B: 0xA6, A: 0xFF} // light gray

	// Card text colors
	ColorTextPrimary   = color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF} // white
	ColorTextSecondary = color.RGBA{R: 0xCC, G: 0xCC, B: 0xCC, A: 0xFF} // light gray
	ColorCardBg        = color.RGBA{R: 0x2A, G: 0x2A, B: 0x2A, A: 0xFF} // dark card background
	ColorBg            = color.RGBA{R: 0x1E, G: 0x1E, B: 0x1E, A: 0xFF} // window background

	// Stats bar colors
	ColorOffline = color.RGBA{R: 0x55, G: 0x55, B: 0x55, A: 0xFF} // gray for offline
)

// StatusColor returns the display color for a given agent status.
func StatusColor(s protocol.AgentStatus) color.Color {
	switch s {
	case protocol.StatusBlocked:
		return ColorBlocked
	case protocol.StatusDone:
		return ColorDone
	case protocol.StatusWorking:
		return ColorWorking
	case protocol.StatusIdle:
		return ColorIdle
	case protocol.StatusUnknown:
		return ColorUnknown
	default:
		return ColorUnknown
	}
}

// StatusLabel returns a short display label for a status.
func StatusLabel(s protocol.AgentStatus) string {
	switch s {
	case protocol.StatusBlocked:
		return "BLOCKED"
	case protocol.StatusDone:
		return "DONE"
	case protocol.StatusWorking:
		return "WORKING"
	case protocol.StatusIdle:
		return "IDLE"
	case protocol.StatusUnknown:
		return "UNKNOWN"
	default:
		return "?"
	}
}

// StatusEmoji returns a status emoji/symbol for compact display.
func StatusEmoji(s protocol.AgentStatus) string {
	switch s {
	case protocol.StatusBlocked:
		return "🔴"
	case protocol.StatusDone:
		return "✅"
	case protocol.StatusWorking:
		return "🔵"
	case protocol.StatusIdle:
		return "🟢"
	case protocol.StatusUnknown:
		return "⚫"
	default:
		return "⚫"
	}
}
