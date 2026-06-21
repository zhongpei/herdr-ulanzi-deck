package board

import (
	"image/color"
	"math"
	"time"

	"gioui.org/unit"
)

// AnimationState tracks per-agent animation progress.
type AnimationState struct {
	// Phase is a time-based value [0, 2π) for sine-wave animation.
	Phase float64
	// lastUpdate tracks the last frame time for delta calculation.
	lastUpdate time.Time
}

// NewAnimationState creates an animation state starting at the given time.
func NewAnimationState(now time.Time) *AnimationState {
	return &AnimationState{
		Phase:      0,
		lastUpdate: now,
	}
}

// Advance progresses the animation phase by the time delta from now.
// Returns the updated phase for use in animation calculations.
func (a *AnimationState) Advance(now time.Time) float64 {
	if a.lastUpdate.IsZero() {
		a.lastUpdate = now
		return 0
	}
	dt := now.Sub(a.lastUpdate).Seconds()
	a.lastUpdate = now
	// ~1 full cycle per 2 seconds
	a.Phase += dt * math.Pi
	if a.Phase > 2*math.Pi {
		a.Phase -= 2 * math.Pi
	}
	return a.Phase
}

// Animation rate multipliers (higher = faster flash).
const (
	AnimRateBreath = 1.0 // working: slow breathing
	AnimRateFlash  = 2.5 // done: medium flash
	AnimRatePulse  = 4.0 // blocked: fastest rapid pulse
)

// BreathOpacity returns [0.6, 1.0] — slow breathing for working.
func BreathOpacity(phase float64) float64 {
	return 0.6 + 0.4*float64(math.Sin(phase*AnimRateBreath)*0.5+0.5)
}

// FlashOpacity returns [0.3, 1.0] — medium flash for done.
func FlashOpacity(phase float64) float64 {
	return 0.3 + 0.7*float64(math.Sin(phase*AnimRateFlash)*0.5+0.5)
}

// PulseOpacity returns [0.4, 1.0] — fast rapid pulse for blocked.
func PulseOpacity(phase float64) float64 {
	return 0.4 + 0.6*float64(math.Sin(phase*AnimRatePulse)*0.5+0.5)
}

// AnimateColor applies an opacity multiplier to a color for breathing/pulse.
func AnimateColor(c color.NRGBA, opacity float64) color.NRGBA {
	return color.NRGBA{
		R: uint8(float64(c.R) * opacity),
		G: uint8(float64(c.G) * opacity),
		B: uint8(float64(c.B) * opacity),
		A: c.A,
	}
}

// LerpDp interpolates between two Dp values by factor t [0,1].
func LerpDp(a, b unit.Dp, t float64) unit.Dp {
	return a + unit.Dp(float64(b-a)*t)
}
