package deck

import (
	"bytes"
	"fmt"
	"image/color"
	"strings"

	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/renderers"
)

// svgToPNG is an internal alias
func svgToPNG(svgData []byte, width, height int) ([]byte, error) {
	return SVGToPNG(svgData, width, height)
}

// SVGToPNG renders SVG to PNG by parsing SVG structure and drawing
// equivalent elements using the canvas API (which handles text correctly).
func SVGToPNG(svgData []byte, width, height int) ([]byte, error) {
	svgStr := string(svgData)
	svgStr = strings.TrimSpace(svgStr)
	svgStr = strings.TrimPrefix(svgStr, `<?xml version="1.0" encoding="UTF-8"?>`)
	svgStr = strings.TrimSpace(svgStr)

	if !strings.HasPrefix(svgStr, "<svg") {
		return nil, fmt.Errorf("not an SVG document")
	}

	// Parse viewBox from SVG to compute correct scale factors.
	// SVG templates use viewBox="0 0 W H" where W,H define the coordinate system.
	svgW := 200.0
	svgH := 200.0
	vb := extractStr(svgStr, "viewBox")
	if vb != "" {
		var x0, y0, vw, vh float64
		if n, _ := fmt.Sscanf(vb, "%f %f %f %f", &x0, &y0, &vw, &vh); n == 4 {
			svgW = vw
			svgH = vh
		}
	}

	scaleX := float64(width) / svgW
	scaleY := float64(height) / svgH

	return renderDirectPNG(svgStr, width, height, scaleX, scaleY)
}

// renderDirectPNG draws key elements directly using canvas API.
// Canvas uses bottom-left origin (y-up). SVG uses top-left origin (y-down).
// We flip Y: canvas_y = height - (svg_y + element_h) * scaleY
func renderDirectPNG(svg string, width, height int, scaleX, scaleY float64) ([]byte, error) {
	c := canvas.New(float64(width), float64(height))

	// Draw rectangles (back to front)
	rects := parseRectElements(svg)
	for _, r := range rects {
		fill := parseHexA(r.fill, r.opacity)
		py := float64(height) - (r.y+r.h)*scaleY
		px := r.x * scaleX
		pw := r.w * scaleX
		ph := r.h * scaleY

		var path *canvas.Path
		if r.rx > 0 {
			prx := r.rx * scaleX
			path = canvas.RoundedRectangle(pw, ph, prx)
		} else {
			path = canvas.Rectangle(pw, ph)
		}
		path = path.Translate(px, py)

		style := canvas.Style{
			Fill: canvas.Paint{Color: fill},
		}
		c.RenderPath(path, style, canvas.Identity)
	}

	// Draw text
	// SVG font-size is in user units (pixels at viewBox native size).
	// Canvas font size is in points. At DPMM(1) = 25.4 DPI:
	//   font_pt = font_px / (DPI / 72) = font_px * 72 / 25.4
	const px2pt = 72.0 / 25.4
	texts := parseTextElements(svg)
	for _, t := range texts {
		px := t.x * scaleX
		py := float64(height) - t.y*scaleY
		fontPt := t.size * scaleX * px2pt

		fillColor := parseHex(t.fill)
		fontStyle := canvas.FontRegular
		var weightVal int
		if _, err := fmt.Sscanf(t.fontWeight, "%d", &weightVal); err == nil && weightVal >= 700 {
			fontStyle = canvas.FontBold
		}
		freshFace := loadFontWithColor(fontPt, fontStyle, fillColor)

		halign := canvas.Left
		if t.anchor == "middle" {
			halign = canvas.Center
		} else if t.anchor == "end" {
			halign = canvas.Right
		}

		text := canvas.NewTextLine(freshFace, t.content, halign)
		c.RenderText(text, canvas.Identity.Translate(px, py))
	}

	var buf bytes.Buffer
	if err := c.Write(&buf, renderers.PNG(canvas.DPMM(1))); err != nil {
		return nil, fmt.Errorf("render png: %w", err)
	}
	return buf.Bytes(), nil
}

// ─── SVG element parsers ─────────────────────────────────────

type rectInfo struct {
	x, y, w, h, rx float64
	fill           string
	opacity        float64
}

type textInfo struct {
	content          string
	x, y, size       float64
	fill             string
	anchor           string
	fontWeight       string
}

func parseRectElements(svg string) []rectInfo {
	var rects []rectInfo
	lines := strings.Split(svg, "\n")

	i := 0
	for i < len(lines) {
		trimmed := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(trimmed, "<rect") {
			i++
			continue
		}

		// Merge multi-line rect elements (attributes can span lines)
		// Only merge lines that do NOT start with < (i.e., continuation lines)
		merged := trimmed
		i++
		for i < len(lines) {
			next := strings.TrimSpace(lines[i])
			if strings.HasPrefix(next, "<") {
				// Next element starts, don't merge
				break
			}
			merged += " " + next
			if strings.HasSuffix(strings.TrimSpace(next), "/>") || strings.HasSuffix(strings.TrimSpace(next), ">") {
				break
			}
			i++
		}

		var r rectInfo
		r.opacity = 1.0
		r.x = extractFloat(merged, "x")
		r.y = extractFloat(merged, "y")
		r.w = extractFloat(merged, "width")
		r.h = extractFloat(merged, "height")
		r.rx = extractFloat(merged, "rx")
		r.fill = extractStr(merged, "fill")
		op := extractFloat(merged, "opacity")
		if op > 0 {
			r.opacity = op
		}
		// Skip rects with fill="none" (stroked borders)
		if r.fill == "none" {
			// i already points to next unprocessed line, don't increment
			continue
		}
		rects = append(rects, r)
		// i already points to next unprocessed line after merge loop
	}
	return rects
}

func parseTextElements(svg string) []textInfo {
	var texts []textInfo
	lines := strings.Split(svg, "\n")

	i := 0
	for i < len(lines) {
		trimmed := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(trimmed, "<text") {
			i++
			continue
		}

		// Merge multi-line text (attributes that span lines)
		// Only merge lines that do NOT start with <
		merged := trimmed
		i++
		for i < len(lines) {
			next := strings.TrimSpace(lines[i])
			if strings.HasPrefix(next, "<") {
				// Next element starts, stop
				break
			}
			merged += " " + next
			if strings.Contains(next, "</text>") {
				break
			}
			i++
		}
		// i now points to next unprocessed line

		var t textInfo
		t.x = extractFloat(merged, "x")
		t.y = extractFloat(merged, "y")
		t.size = extractFloat(merged, "font-size")
		if t.size == 0 {
			t.size = 16
		}
		t.fill = extractStr(merged, "fill")
		t.anchor = extractStr(merged, "text-anchor")
		t.fontWeight = extractStr(merged, "font-weight")

		start := strings.Index(merged, ">")
		end := strings.LastIndex(merged, "</text>")
		if start >= 0 && end > start {
			t.content = merged[start+1 : end]
		}
		if t.content != "" {
			texts = append(texts, t)
		}
		// i already points to next unprocessed line, don't increment
	}
	return texts
}

// ─── Font ─────────────────────────────────────────────────────

var fontCache = make(map[string]*canvas.FontFace)

func loadFont(size float64, style canvas.FontStyle) (*canvas.FontFace, error) {
	key := fmt.Sprintf("%.1f-%d", size, style)
	if f, ok := fontCache[key]; ok {
		return f, nil
	}
	family := canvas.NewFontFamily("sans-serif")
	if err := family.LoadSystemFont("sans-serif", style); err != nil {
		for _, name := range []string{"Helvetica", "Arial", "Liberation Sans"} {
			if err := family.LoadSystemFont(name, style); err == nil {
				break
			}
		}
	}
	face := family.Face(size, color.Black, style)
	fontCache[key] = face
	return face, nil
}

// loadFontWithColor creates a new font face with the specified fill color, uncached.
func loadFontWithColor(size float64, style canvas.FontStyle, fill color.Color) *canvas.FontFace {
	family := canvas.NewFontFamily("sans-serif")
	if err := family.LoadSystemFont("sans-serif", style); err != nil {
		for _, name := range []string{"Helvetica", "Arial", "Liberation Sans"} {
			if err := family.LoadSystemFont(name, style); err == nil {
				break
			}
		}
	}
	return family.Face(size, fill, style)
}

// ─── Color helpers ───────────────────────────────────────────

func parseHex(s string) color.RGBA {
	if s == "" || s[0] != '#' {
		return color.RGBA{255, 255, 255, 255}
	}
	s = s[1:]
	if len(s) == 3 {
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	}
	if len(s) != 6 {
		return color.RGBA{255, 255, 255, 255}
	}
	hex := func(b byte) byte {
		switch {
		case b >= '0' && b <= '9':
			return b - '0'
		case b >= 'A' && b <= 'F':
			return b - 'A' + 10
		case b >= 'a' && b <= 'f':
			return b - 'a' + 10
		}
		return 0
	}
	return color.RGBA{
		R: hex(s[0])<<4 | hex(s[1]),
		G: hex(s[2])<<4 | hex(s[3]),
		B: hex(s[4])<<4 | hex(s[5]),
		A: 255,
	}
}

func parseHexA(s string, opacity float64) color.RGBA {
	c := parseHex(s)
	c.A = uint8(opacity * 255)
	return c
}

// ─── SVG attribute extractors ────────────────────────────────

func extractFloat(line, attr string) float64 {
	s := extractStr(line, attr)
	if s == "" {
		return 0
	}
	var v float64
	fmt.Sscanf(s, "%f", &v)
	return v
}

func extractStr(line, attr string) string {
	search := attr + "="
	idx := -1
	for i := 0; i <= len(line)-len(search); i++ {
		if line[i:i+len(search)] == search {
			if i == 0 || line[i-1] == ' ' || line[i-1] == '"' || line[i-1] == '\'' {
				idx = i
				break
			}
		}
	}
	if idx < 0 {
		return ""
	}
	rest := line[idx+len(search):]
	if len(rest) == 0 {
		return ""
	}
	quote := rest[0]
	if quote != '"' && quote != '\'' {
		return ""
	}
	endIdx := strings.Index(rest[1:], string(quote))
	if endIdx < 0 {
		return ""
	}
	return rest[1 : 1+endIdx]
}
