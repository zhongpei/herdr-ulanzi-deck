// Package board provides the Gio-based Fleet Board UI for herdr-panel.
package board

import (
	"image/color"

	"gioui.org/font/gofont"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"github.com/herdr-deck/herdrdeck/protocol"
)

// Theme is the shared material theme used across all board layout functions.
// Must be initialized once before any layout call.
var Theme *material.Theme

func init() {
	Theme = material.NewTheme()
	// NewTheme creates Shaper = &text.Shaper{} (uninitialized zero value).
	// Replace with a real shaper that has fonts loaded.
	Theme.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	// Dark palette for Fleet Board
	Theme.Palette = material.Palette{
		Bg:         ColorBg,
		Fg:         ColorTextPrimary,
		ContrastBg: ColorAccent,
		ContrastFg: ColorTextPrimary,
	}
	Theme.TextSize = 16
}

// ─── Color Palette ───────────────────────────────────────────
// All background colors use alpha=128 (50% opacity) for transparent effect.
// Text colors remain full opacity for readability.

var (
	// Status colors
	ColorBlocked = color.NRGBA{R: 0xE7, G: 0x4C, B: 0x3C, A: 0xFF} // red
	ColorDone    = color.NRGBA{R: 0x27, G: 0xAE, B: 0x60, A: 0xFF} // green
	ColorWorking = color.NRGBA{R: 0xF3, G: 0x9C, B: 0x12, A: 0xFF} // orange
	ColorIdle    = color.NRGBA{R: 0x7F, G: 0x8C, B: 0x8D, A: 0xFF} // gray
	ColorUnknown = color.NRGBA{R: 0x95, G: 0xA5, B: 0xA6, A: 0xFF} // light gray

	// Text colors
	ColorTextPrimary   = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	ColorTextSecondary = color.NRGBA{R: 0xCC, G: 0xCC, B: 0xCC, A: 0xFF}
	ColorTextDim       = color.NRGBA{R: 0x88, G: 0x88, B: 0x88, A: 0xFF}

	// Surface colors
	ColorCardBg  = color.NRGBA{R: 0x2A, G: 0x2A, B: 0x2A, A: 0xFF}
	ColorBg      = color.NRGBA{R: 0x1E, G: 0x1E, B: 0x1E, A: 0xFF}
	ColorDivider = color.NRGBA{R: 0x3A, G: 0x3A, B: 0x3A, A: 0xFF}
	ColorOffline = color.NRGBA{R: 0x55, G: 0x55, B: 0x55, A: 0xFF}

	// Focus/accent
	ColorAccent    = color.NRGBA{R: 0x3B, G: 0x82, B: 0xF6, A: 0xFF}
	ColorHighlight = color.NRGBA{R: 0x4A, G: 0x4A, B: 0x4A, A: 0xFF}
)

// ─── Spacing / Sizing ───────────────────────────────────────

var (
	SpacingTiny  = unit.Dp(2)
	SpacingSmall = unit.Dp(4)
	SpacingMed   = unit.Dp(8)
	SpacingLarge = unit.Dp(12)
	SpacingXl    = unit.Dp(16)

	CornerRadius  = unit.Dp(3)
	CardHeight    = unit.Dp(28)
	RowHeight     = unit.Dp(20)
	BarHeight     = unit.Dp(22)
	MinCardWidth  = unit.Dp(140)
	MinMachineCol = unit.Dp(48)
	ChipRadius    = unit.Dp(2)

	WindowWidth     = unit.Dp(460)
	WindowHeight    = unit.Dp(400) // fallback if estimate fails
	WindowHeightMin = unit.Dp(160) // minimum content height
)

// ─── Status Helpers ─────────────────────────────────────────

// StatusColor returns the display color for a given agent status.
func StatusColor(s protocol.AgentStatus) color.NRGBA {
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

// StatusSymbol returns a unicode symbol for compact display.
func StatusSymbol(s protocol.AgentStatus) string {
	switch s {
	case protocol.StatusBlocked:
		return "●"
	case protocol.StatusDone:
		return "✓"
	case protocol.StatusWorking:
		return "◆"
	case protocol.StatusIdle:
		return "○"
	case protocol.StatusUnknown:
		return "·"
	default:
		return "·"
	}
}
