package tray

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
)

// Severity colours matching the Fleet Board palette.
var (
	dotBlocked = color.NRGBA{R: 0xE7, G: 0x4C, B: 0x3C, A: 0xFF} // red
	dotWorking = color.NRGBA{R: 0xF3, G: 0x9C, B: 0x12, A: 0xFF} // gold
	dotDone    = color.NRGBA{R: 0x27, G: 0xAE, B: 0x60, A: 0xFF} // green
	dotIdle    = color.NRGBA{R: 0x7F, G: 0x8C, B: 0x8D, A: 0xFF} // gray
)

const iconSize = 16

// severityDot picks a colour based on the most severe status and returns a
// 16x16 PNG with a filled circle in that colour.
//
// Used only on Windows (macOS uses attributedTitle, Linux uses plain text).
// Priority: blocked > working > done > idle.
func severityDot(blocked, working, done int) []byte {
	col := pickColour(blocked, working, done)

	img := image.NewNRGBA(image.Rect(0, 0, iconSize, iconSize))
	draw.Draw(img, img.Bounds(), image.Transparent, image.Point{}, draw.Src)

	cx, cy := iconSize/2, iconSize/2
	r := iconSize/2 - 1
	r2 := r * r

	for y := 0; y < iconSize; y++ {
		for x := 0; x < iconSize; x++ {
			dx, dy := x-cx, y-cy
			if dx*dx+dy*dy <= r2 {
				dist := math.Sqrt(float64(dx*dx + dy*dy))
				alpha := 1.0
				if dist > float64(r)-0.5 {
					alpha = float64(r) + 0.5 - dist
					if alpha < 0 {
						alpha = 0
					}
				}
				c := color.NRGBA{
					R: col.R,
					G: col.G,
					B: col.B,
					A: uint8(float64(col.A) * alpha),
				}
				img.SetNRGBA(x, y, c)
			}
		}
	}

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func pickColour(blocked, working, done int) color.NRGBA {
	switch {
	case blocked > 0:
		return dotBlocked
	case working > 0:
		return dotWorking
	case done > 0:
		return dotDone
	default:
		return dotIdle
	}
}
