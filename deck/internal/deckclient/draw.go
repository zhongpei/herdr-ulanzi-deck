package deckclient

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"strings"

	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/renderers"
	"github.com/tdewolff/canvas/renderers/rasterizer"
)
// SVGFramesToGIF converts multiple SVG frames into an animated GIF.
// Each frame SVG is rendered at the given width×height resolution.
// delaysMs[i] is the per-frame delay in milliseconds.
// Returns the GIF binary data, suitable for sending via type:3 + gifdata.
func SVGFramesToGIF(frames [][]byte, width, height int, delaysMs []int) ([]byte, error) {
	if len(frames) == 0 {
		return nil, fmt.Errorf("no frames to encode")
	}
	if len(delaysMs) != len(frames) {
		return nil, fmt.Errorf("delay count %d must match frame count %d", len(delaysMs), len(frames))
	}

	out := &gif.GIF{}
	for i, svgData := range frames {
		svgStr := string(svgData)
		svgStr = strings.TrimSpace(svgStr)
		svgStr = strings.TrimPrefix(svgStr, `<?xml version="1.0" encoding="UTF-8"?>`)
		svgStr = strings.TrimSpace(svgStr)

		if !strings.HasPrefix(svgStr, "<svg") {
			return nil, fmt.Errorf("frame %d: not an SVG document", i)
		}

		// Parse viewBox for scale
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

		rgba, err := svgToRGBA(svgStr, width, height, scaleX, scaleY)
		if err != nil {
			return nil, fmt.Errorf("frame %d: %w", i, err)
		}

		// Quantize RGBA to paletted (nearest-color, no dithering)
		palettedImg := image.NewPaletted(rgba.Bounds(), palette.Plan9)
		draw.Draw(palettedImg, palettedImg.Bounds(), rgba, image.Point{}, draw.Src)

		out.Image = append(out.Image, palettedImg)
		out.Delay = append(out.Delay, delaysMs[i]/10) // ms → centiseconds
	}

	if len(out.Image) > 0 {
		out.Disposal = make([]byte, len(out.Image))
		out.Disposal[0] = gif.DisposalNone
		for i := 1; i < len(out.Image); i++ {
			out.Disposal[i] = gif.DisposalBackground
		}
	}

	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, out); err != nil {
		return nil, fmt.Errorf("gif encode: %w", err)
	}
	return buf.Bytes(), nil
}

// svgToRGBA renders SVG content to an RGBA image using the canvas rasterizer.
func svgToRGBA(svg string, width, height int, scaleX, scaleY float64) (*image.RGBA, error) {
	c := canvas.New(float64(width), float64(height))

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

		text := canvas.NewTextLine(freshFace, emojiFilter(t.content), halign)

		if fontStyle == canvas.FontBold {
			c.RenderText(text, canvas.Identity.Translate(px-1, py))
			c.RenderText(text, canvas.Identity.Translate(px+1, py))
		} else {
			c.RenderText(text, canvas.Identity.Translate(px, py))
		}
	}

	polylines := parsePolylineElements(svg)
	for _, p := range polylines {
		if len(p.points) < 4 {
			continue
		}
		path := &canvas.Path{}
		px0 := p.points[0] * scaleX
		py0 := float64(height) - p.points[1]*scaleY
		path.MoveTo(px0, py0)
		for i := 2; i+1 < len(p.points); i += 2 {
			px := p.points[i] * scaleX
			py := float64(height) - p.points[i+1]*scaleY
			path.LineTo(px, py)
		}
		style := buildStrokeStyle(p.stroke, p.fill, p.opacity, p.strokeWidth*scaleX, p.linecap, p.linejoin)
		c.RenderPath(path, style, canvas.Identity)
	}

	lines := parseLineElements(svg)
	for _, l := range lines {
		path := &canvas.Path{}
		path.MoveTo(l.x1*scaleX, float64(height)-l.y1*scaleY)
		path.LineTo(l.x2*scaleX, float64(height)-l.y2*scaleY)
		style := buildStrokeStyle(l.stroke, "none", 1.0, l.strokeWidth*scaleX, l.linecap, "")
		c.RenderPath(path, style, canvas.Identity)
	}

	circles := parseCircleElements(svg)
	for _, ci := range circles {
		px := ci.cx * scaleX
		py := float64(height) - ci.cy*scaleY
		pr := ci.r * scaleX
		path := canvas.Circle(pr).Translate(px, py)
		style := buildStrokeStyle(ci.stroke, ci.fill, 1.0, ci.strokeWidth*scaleX, "", "")
		c.RenderPath(path, style, canvas.Identity)
	}

	return rasterizer.Draw(c, canvas.DPMM(1), canvas.DefaultColorSpace), nil
}

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

		text := canvas.NewTextLine(freshFace, emojiFilter(t.content), halign)

		// Bold text: render twice with 1px horizontal offset at canvas level
		// to create visible weight regardless of system bold font availability.
		if fontStyle == canvas.FontBold {
			c.RenderText(text, canvas.Identity.Translate(px-1, py))
			c.RenderText(text, canvas.Identity.Translate(px+1, py))
		} else {
			c.RenderText(text, canvas.Identity.Translate(px, py))
		}
	}

	// Draw polylines (used by status icons like checkmark, triangle, bars)
	polylines := parsePolylineElements(svg)
	for _, p := range polylines {
		if len(p.points) < 4 {
			continue
		}
		path := &canvas.Path{}
		// First point: MoveTo (in flipped canvas coords)
		px0 := p.points[0] * scaleX
		py0 := float64(height) - p.points[1]*scaleY
		path.MoveTo(px0, py0)
		// Remaining points: LineTo
		for i := 2; i+1 < len(p.points); i += 2 {
			px := p.points[i] * scaleX
			py := float64(height) - p.points[i+1]*scaleY
			path.LineTo(px, py)
		}
		style := buildStrokeStyle(p.stroke, p.fill, p.opacity, p.strokeWidth*scaleX, p.linecap, p.linejoin)
		c.RenderPath(path, style, canvas.Identity)
	}

	// Draw lines (used by status icons like pause bars, exclamation)
	lines := parseLineElements(svg)
	for _, l := range lines {
		path := &canvas.Path{}
		path.MoveTo(l.x1*scaleX, float64(height)-l.y1*scaleY)
		path.LineTo(l.x2*scaleX, float64(height)-l.y2*scaleY)
		style := buildStrokeStyle(l.stroke, "none", 1.0, l.strokeWidth*scaleX, l.linecap, "")
		c.RenderPath(path, style, canvas.Identity)
	}

	// Draw circles (used by status icons like unknown badge, working spinner)
	circles := parseCircleElements(svg)
	for _, ci := range circles {
		px := ci.cx * scaleX
		py := float64(height) - ci.cy*scaleY
		pr := ci.r * scaleX
		path := canvas.Circle(pr).Translate(px, py)
		style := buildStrokeStyle(ci.stroke, ci.fill, 1.0, ci.strokeWidth*scaleX, "", "")
		c.RenderPath(path, style, canvas.Identity)
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
	content    string
	x, y, size float64
	fill       string
	anchor     string
	fontWeight string
}

type polylineInfo struct {
	points      []float64 // x,y,x,y,...
	stroke      string
	strokeWidth float64
	fill        string
	opacity     float64
	linecap     string
	linejoin    string
}

type circleInfo struct {
	cx, cy, r   float64
	fill        string
	stroke      string
	strokeWidth float64
}

type lineInfo struct {
	x1, y1, x2, y2 float64
	stroke         string
	strokeWidth    float64
	linecap        string
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

// sharedFontFamily is initialized once at package init and reused
// for all font face creation. FontFamily.LoadSystemFont is expensive
// (CoreText font loading on macOS), so doing it once instead of per-
// render cycle eliminates the 1.5GB+ physical footprint on macOS.
var sharedFontFamily *canvas.FontFamily

func init() {
	sharedFontFamily = canvas.NewFontFamily("sans-serif")
	// Load all 4 style combinations: regular, bold, italic, bold-italic.
	// FontFamily.fonts is a map[Style]*Font, so each LoadSystemFont
	// replaces the previous font for that style. The LAST font loaded
	// for each style is used for glyph lookups.
	styles := []canvas.FontStyle{
		canvas.FontRegular,
		canvas.FontBold,
		canvas.FontItalic,
		canvas.FontBold | canvas.FontItalic,
	}
	for _, style := range styles {
		for _, name := range fontNames() {
			_ = sharedFontFamily.LoadSystemFont(name, style)
		}
	}
}

var fontCache = make(map[string]*canvas.FontFace)

func loadFont(size float64, style canvas.FontStyle) (*canvas.FontFace, error) {
	key := fmt.Sprintf("%.1f-%d", size, style)
	if f, ok := fontCache[key]; ok {
		return f, nil
	}
	face := sharedFontFamily.Face(size, color.Black, style)
	fontCache[key] = face
	return face, nil
}

var fontColorCache = make(map[string]*canvas.FontFace)

// loadFontWithColor creates a new font face with the specified fill color.
// Cached by size + style + RGBA to avoid repeated Face allocations.
func loadFontWithColor(size float64, style canvas.FontStyle, fill color.Color) *canvas.FontFace {
	rgba := color.RGBAModel.Convert(fill).(color.RGBA)
	key := fmt.Sprintf("%.1f-%d-%02x%02x%02x%02x", size, style, rgba.R, rgba.G, rgba.B, rgba.A)
	if f, ok := fontColorCache[key]; ok {
		return f
	}
	face := sharedFontFamily.Face(size, fill, style)
	fontColorCache[key] = face
	return face
}

// fontNames returns system font names to try, in priority order.
// FontFamily.fonts is a map (one font per style), so the LAST successfully
// loaded font is the one used for all glyph lookups. Names are ordered so
// the font with the broadest Unicode coverage (especially CJK) comes last.
//
// All status/agent icon glyphs (✓, ‖, ↻, ⚠, ?) are drawn with SVG primitives
// in icons.go — no Unicode text — so Apple Symbols is not needed here.
func fontNames() []string {
	return []string{
		// Generic Latin fonts (loaded first, overwritten by subsequent fonts)
		"Helvetica",
		"Arial",
		"Liberation Sans",
		"DejaVu Sans",
		"Verdana",
		"Tahoma",
		// CJK-capable fonts for Linux (Noto Sans CJK, WenQuanYi)
		"Noto Sans CJK SC",
		"Noto Sans CJK JP",
		"WenQuanYi Micro Hei",
		"WenQuanYi Zen Hei",
		// Broad Unicode coverage — loaded LAST so it wins
		// Arial Unicode MS covers Latin, CJK, Cyrillic, Greek, Arabic, Hebrew, etc.
		"Arial Unicode MS",
	}
}

// emojiFilter replaces emoji codepoints with "?" to avoid .notdef boxes.
// tdewolff/canvas uses outline font rendering and cannot render color emoji
// (sbix/CBDT/COLR tables). Common emoji ranges are replaced.
func emojiFilter(s string) string {
	if !hasEmoji(s) {
		return s
	}
	runes := []rune(s)
	for i, r := range runes {
		if isEmoji(r) {
			runes[i] = '?'
		}
	}
	return string(runes)
}

func hasEmoji(s string) bool {
	for _, r := range s {
		if isEmoji(r) {
			return true
		}
	}
	return false
}

func isEmoji(r rune) bool {
	// SMP emoji blocks — these are safely all-emoji, no CJK or Latin overlap.
	// Miscellaneous Symbols and Pictographs
	if r >= 0x1F300 && r <= 0x1FAFF {
		return true
	}
	// Supplemental Symbols and Pictographs (extended emoji)
	if r >= 0x1FA00 && r <= 0x1FA6F {
		return true
	}
	// Emoticons (faces)
	if r >= 0x1F600 && r <= 0x1F64F {
		return true
	}
	// Transport and Map Symbols
	if r >= 0x1F680 && r <= 0x1F6FF {
		return true
	}
	// Enclosed Ideographic Supplement (emoji variants of CJK)
	if r >= 0x1F200 && r <= 0x1F2FF {
		return true
	}
	// Regional Indicator Symbols (flag letters)
	if r >= 0x1F1E6 && r <= 0x1F1FF {
		return true
	}
	// Variation Selectors (emoji presentation selectors)
	if r >= 0xFE00 && r <= 0xFE0F {
		return true
	}
	// Combining Enclosing Keycap
	if r == 0x20E3 {
		return true
	}
	return false
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

// extractPoints parses an SVG points attribute (e.g. "8,76 16,84 32,66" or
// "8,76,16,84,32,66") into a flat []float64 of x,y pairs. Skips tokens that
// don't parse as numbers.
func extractPoints(s string) []float64 {
	var out []float64
	f := func(tok string) {
		var v float64
		if _, err := fmt.Sscanf(tok, "%f", &v); err == nil {
			out = append(out, v)
		}
	}
	// Split on whitespace and commas simultaneously by replacing commas with spaces.
	for _, tok := range strings.Fields(strings.ReplaceAll(s, ",", " ")) {
		f(tok)
	}
	return out
}

func parsePolylineElements(svg string) []polylineInfo {
	var out []polylineInfo
	lines := strings.Split(svg, "\n")
	i := 0
	for i < len(lines) {
		trimmed := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(trimmed, "<polyline") {
			i++
			continue
		}
		merged := trimmed
		i++
		for i < len(lines) {
			next := strings.TrimSpace(lines[i])
			if strings.HasPrefix(next, "<") {
				break
			}
			merged += " " + next
			if strings.HasSuffix(strings.TrimSpace(next), "/>") || strings.HasSuffix(strings.TrimSpace(next), ">") {
				break
			}
			i++
		}
		var p polylineInfo
		p.opacity = 1.0
		p.points = extractPoints(extractStr(merged, "points"))
		p.stroke = extractStr(merged, "stroke")
		p.fill = extractStr(merged, "fill")
		p.linecap = extractStr(merged, "stroke-linecap")
		p.linejoin = extractStr(merged, "stroke-linejoin")
		p.strokeWidth = extractFloat(merged, "stroke-width")
		if p.strokeWidth == 0 {
			p.strokeWidth = 1
		}
		if op := extractFloat(merged, "opacity"); op > 0 {
			p.opacity = op
		}
		if len(p.points) >= 4 {
			out = append(out, p)
		}
	}
	return out
}

func parseLineElements(svg string) []lineInfo {
	var out []lineInfo
	lines := strings.Split(svg, "\n")
	i := 0
	for i < len(lines) {
		trimmed := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(trimmed, "<line") {
			i++
			continue
		}
		merged := trimmed
		i++
		for i < len(lines) {
			next := strings.TrimSpace(lines[i])
			if strings.HasPrefix(next, "<") {
				break
			}
			merged += " " + next
			if strings.HasSuffix(strings.TrimSpace(next), "/>") || strings.HasSuffix(strings.TrimSpace(next), ">") {
				break
			}
			i++
		}
		var l lineInfo
		l.x1 = extractFloat(merged, "x1")
		l.y1 = extractFloat(merged, "y1")
		l.x2 = extractFloat(merged, "x2")
		l.y2 = extractFloat(merged, "y2")
		l.stroke = extractStr(merged, "stroke")
		l.linecap = extractStr(merged, "stroke-linecap")
		l.strokeWidth = extractFloat(merged, "stroke-width")
		if l.strokeWidth == 0 {
			l.strokeWidth = 1
		}
		out = append(out, l)
	}
	return out
}

func parseCircleElements(svg string) []circleInfo {
	var out []circleInfo
	lines := strings.Split(svg, "\n")
	i := 0
	for i < len(lines) {
		trimmed := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(trimmed, "<circle") {
			i++
			continue
		}
		merged := trimmed
		i++
		for i < len(lines) {
			next := strings.TrimSpace(lines[i])
			if strings.HasPrefix(next, "<") {
				break
			}
			merged += " " + next
			if strings.HasSuffix(strings.TrimSpace(next), "/>") || strings.HasSuffix(strings.TrimSpace(next), ">") {
				break
			}
			i++
		}
		var ci circleInfo
		ci.cx = extractFloat(merged, "cx")
		ci.cy = extractFloat(merged, "cy")
		ci.r = extractFloat(merged, "r")
		ci.fill = extractStr(merged, "fill")
		ci.stroke = extractStr(merged, "stroke")
		ci.strokeWidth = extractFloat(merged, "stroke-width")
		if ci.r > 0 {
			out = append(out, ci)
		}
	}
	return out
}

// buildStrokeStyle assembles a canvas.Style honoring fill, stroke, opacity,
// stroke-width, and optional stroke-linecap / stroke-linejoin.
// fill="none" suppresses the fill paint; otherwise fill is applied.
func buildStrokeStyle(stroke, fill string, opacity, strokeWidth float64, linecap, linejoin string) canvas.Style {
	style := canvas.Style{
		StrokeWidth: strokeWidth,
	}
	if fill != "" && fill != "none" {
		style.Fill = canvas.Paint{Color: parseHexA(fill, opacity)}
	}
	if stroke != "" && stroke != "none" {
		style.Stroke = canvas.Paint{Color: parseHexA(stroke, opacity)}
		style.StrokeCapper = parseLineCap(linecap)
		style.StrokeJoiner = parseLineJoin(linejoin)
	}
	return style
}

func parseLineCap(s string) canvas.Capper {
	switch s {
	case "round":
		return canvas.RoundCap
	case "square":
		return canvas.SquareCap
	default:
		return canvas.ButtCap
	}
}

func parseLineJoin(s string) canvas.Joiner {
	switch s {
	case "round":
		return canvas.RoundJoin
	case "bevel":
		return canvas.BevelJoin
	default:
		return canvas.MiterJoin
	}
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
