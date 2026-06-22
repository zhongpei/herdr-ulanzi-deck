package deckclient

import (
	"bytes"
	"image/gif"
	"image/color"
	"testing"

	"github.com/tdewolff/canvas"
)

// ─── SVG element parser tests ──────────────────────────────

func TestParsePolylineElements(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200">
  <polyline points="8,76 16,84 32,66" fill="none" stroke="white" stroke-width="4" stroke-linecap="round"/>
</svg>`
	got := parsePolylineElements(svg)
	if len(got) != 1 {
		t.Fatalf("expected 1 polyline, got %d", len(got))
	}
	want := []float64{8, 76, 16, 84, 32, 66}
	if len(got[0].points) != len(want) {
		t.Fatalf("expected %d points, got %d", len(want), len(got[0].points))
	}
	for i, v := range want {
		if got[0].points[i] != v {
			t.Errorf("point[%d] = %v, want %v", i, got[0].points[i], v)
		}
	}
	if got[0].stroke != "white" {
		t.Errorf("stroke = %q, want white", got[0].stroke)
	}
	if got[0].strokeWidth != 4 {
		t.Errorf("stroke-width = %v, want 4", got[0].strokeWidth)
	}
	if got[0].linecap != "round" {
		t.Errorf("linecap = %q, want round", got[0].linecap)
	}
}

func TestParsePolylineElements_SpaceSeparatedPoints(t *testing.T) {
	svg := `<polyline points="0 0 10 10 20 0" stroke="white"/>`
	got := parsePolylineElements(svg)
	if len(got) != 1 || len(got[0].points) != 6 {
		t.Fatalf("unexpected parse result: %+v", got)
	}
}

func TestParseCircleElements(t *testing.T) {
	svg := `<svg>
  <circle cx="20" cy="76" r="11" fill="none" stroke="white" stroke-width="3"/>
  <circle cx="20" cy="83.5" r="1.5" fill="white"/>
</svg>`
	got := parseCircleElements(svg)
	if len(got) != 2 {
		t.Fatalf("expected 2 circles, got %d", len(got))
	}
	if got[0].cx != 20 || got[0].cy != 76 || got[0].r != 11 {
		t.Errorf("circle[0] = %+v, want cx=20 cy=76 r=11", got[0])
	}
	if got[0].stroke != "white" {
		t.Errorf("circle[0].stroke = %q", got[0].stroke)
	}
	if got[1].fill != "white" {
		t.Errorf("circle[1].fill = %q, want white (filled)", got[1].fill)
	}
}

func TestParseLineElements(t *testing.T) {
	svg := `<svg>
  <line x1="14" y1="66" x2="14" y2="86" stroke="white" stroke-width="4" stroke-linecap="round"/>
  <line x1="26" y1="66" x2="26" y2="86" stroke="white" stroke-width="4" stroke-linecap="round"/>
</svg>`
	got := parseLineElements(svg)
	if len(got) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(got))
	}
	if got[0].x1 != 14 || got[0].y2 != 86 {
		t.Errorf("line[0] = %+v", got[0])
	}
	if got[0].linecap != "round" {
		t.Errorf("line[0].linecap = %q", got[0].linecap)
	}
}

func TestSVGToPNG_RendersStatusIcons(t *testing.T) {
	xml := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200">
  <rect width="200" height="200" fill="#27AE60"/>
  <polyline points="8,76 16,84 32,66" fill="none" stroke="white" stroke-width="4" stroke-linecap="round" stroke-linejoin="round"/>
  <text x="100" y="100" text-anchor="middle" fill="white" font-size="36">test</text>
</svg>`
	png, err := SVGToPNG([]byte(xml), 196, 196)
	if err != nil {
		t.Fatalf("SVGToPNG failed: %v", err)
	}
	if len(png) < 8 || string(png[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("output is not a valid PNG (got %d bytes)", len(png))
	}
	if len(png) < 500 {
		t.Errorf("PNG suspiciously small: %d bytes", len(png))
	}
}

func TestExtractPoints_HandlesCommasAndSpaces(t *testing.T) {
	cases := []struct {
		in   string
		want []float64
	}{
		{"8,76 16,84", []float64{8, 76, 16, 84}},
		{"8,76,16,84,32,66", []float64{8, 76, 16, 84, 32, 66}},
		{"8 76 16 84", []float64{8, 76, 16, 84}},
		{"", nil},
	}
	for _, tc := range cases {
		got := extractPoints(tc.in)
		if len(got) != len(tc.want) {
			t.Errorf("extractPoints(%q) = %d vals, want %d", tc.in, len(got), len(tc.want))
			continue
		}
		for i, v := range tc.want {
			if got[i] != v {
				t.Errorf("extractPoints(%q)[%d] = %v, want %v", tc.in, i, got[i], v)
			}
		}
	}
}

// ─── Font color cache tests ──────────────────────────────────

func TestLoadFontWithColor_CacheHit(t *testing.T) {
	face1 := loadFontWithColor(12.0, canvas.FontRegular, color.RGBA{255, 255, 255, 255})
	face2 := loadFontWithColor(12.0, canvas.FontRegular, color.RGBA{255, 255, 255, 255})
	if face1 != face2 {
		t.Error("same params should return cached face (pointer equality)")
	}
}

func TestLoadFontWithColor_CacheHitAcrossCalls(t *testing.T) {
	faceA := loadFontWithColor(14.0, canvas.FontBold, color.RGBA{0, 255, 0, 255})
	_ = loadFontWithColor(10.0, canvas.FontRegular, color.RGBA{255, 0, 0, 255})
	faceB := loadFontWithColor(14.0, canvas.FontBold, color.RGBA{0, 255, 0, 255})
	if faceA != faceB {
		t.Error("cache should persist across interleaved calls")
	}
}

func TestLoadFontWithColor_DifferentSizeMiss(t *testing.T) {
	faceA := loadFontWithColor(10.0, canvas.FontRegular, color.RGBA{255, 255, 255, 255})
	faceB := loadFontWithColor(20.0, canvas.FontRegular, color.RGBA{255, 255, 255, 255})
	if faceA == faceB {
		t.Error("different sizes should produce different cache entries")
	}
}

func TestLoadFontWithColor_DifferentStyleMiss(t *testing.T) {
	faceA := loadFontWithColor(12.0, canvas.FontRegular, color.RGBA{255, 255, 255, 255})
	faceB := loadFontWithColor(12.0, canvas.FontBold, color.RGBA{255, 255, 255, 255})
	if faceA == faceB {
		t.Error("regular vs bold should produce different cache entries")
	}
}

func TestLoadFontWithColor_DifferentColorMiss(t *testing.T) {
	faceA := loadFontWithColor(12.0, canvas.FontRegular, color.RGBA{255, 255, 255, 255})
	faceB := loadFontWithColor(12.0, canvas.FontRegular, color.RGBA{0, 0, 0, 255})
	if faceA == faceB {
		t.Error("white vs black fill should produce different cache entries")
	}
}

func TestLoadFontWithColor_ColorNearEdges(t *testing.T) {
	faceA := loadFontWithColor(12.0, canvas.FontRegular, color.RGBA{255, 255, 255, 255})
	faceB := loadFontWithColor(12.0, canvas.FontRegular, color.RGBA{255, 255, 255, 255})
	if faceA != faceB {
		t.Error("max white should cache hit")
	}

	faceC := loadFontWithColor(12.0, canvas.FontRegular, color.RGBA{0, 0, 0, 0})
	faceD := loadFontWithColor(12.0, canvas.FontRegular, color.RGBA{0, 0, 0, 0})
	if faceC != faceD {
		t.Error("transparent black should cache hit")
	}

	if faceA == faceC {
		t.Error("white and transparent black must be different cache entries")
	}
}

func TestLoadFontWithColor_SizeVariations(t *testing.T) {
	sizes := []float64{0, 1, 8, 12, 16, 24, 36, 72, 144, 999}
	faces := make(map[float64]*canvas.FontFace)

	for _, sz := range sizes {
		faces[sz] = loadFontWithColor(sz, canvas.FontRegular, color.RGBA{255, 255, 255, 255})
	}

	visited := make(map[*canvas.FontFace]float64)
	for sz, face := range faces {
		if existingSz, ok := visited[face]; ok {
			t.Errorf("size %.0f collided with size %.0f: same face pointer", sz, existingSz)
		}
		visited[face] = sz
	}
}

func TestLoadFontWithColor_StyleCombinations(t *testing.T) {
	styles := []struct {
		name  string
		style canvas.FontStyle
	}{
		{"regular", canvas.FontRegular},
		{"bold", canvas.FontBold},
		{"italic", canvas.FontItalic},
		{"bold-italic", canvas.FontBold | canvas.FontItalic},
	}

	faces := make(map[string]*canvas.FontFace)
	for _, s := range styles {
		faces[s.name] = loadFontWithColor(16.0, s.style, color.RGBA{255, 255, 255, 255})
	}

	visited := make(map[*canvas.FontFace]string)
	for name, face := range faces {
		if existingName, ok := visited[face]; ok {
			t.Errorf("style %q collided with %q: same face pointer", name, existingName)
		}
		visited[face] = name
	}
}

func TestLoadFontWithColor_ColorPalette(t *testing.T) {
	colors := []color.RGBA{
		{255, 255, 255, 255},
		{0, 0, 0, 255},
		{255, 0, 0, 255},
		{0, 255, 0, 255},
		{0, 0, 255, 255},
		{128, 128, 128, 255},
		{231, 76, 60, 255},
		{241, 196, 15, 255},
		{39, 174, 96, 255},
		{127, 140, 141, 255},
	}

	faces := make(map[int]*canvas.FontFace)
	for i, c := range colors {
		faces[i] = loadFontWithColor(16.0, canvas.FontRegular, c)
	}

	visited := make(map[*canvas.FontFace]int)
	for i, face := range faces {
		if existingIdx, ok := visited[face]; ok {
			t.Errorf("color[%d] collided with color[%d]: same face pointer", i, existingIdx)
		}
		visited[face] = i
	}
}

func TestLoadFontWithColor_ReturnsValidFace(t *testing.T) {
	face := loadFontWithColor(12.0, canvas.FontRegular, color.RGBA{255, 255, 255, 255})
	if face == nil {
		t.Fatal("loadFontWithColor returned nil")
	}
	_ = face.Metrics()
}

func TestLoadFont_SeparateCache(t *testing.T) {
	faceA := loadFontWithColor(12.0, canvas.FontRegular, color.RGBA{255, 255, 255, 255})
	faceB, err := loadFont(12.0, canvas.FontRegular)
	if err != nil {
		t.Fatalf("loadFont failed: %v", err)
	}
	if faceA == faceB {
		t.Error("loadFontWithColor(white) and loadFont(black) should be different")
	}

	faceC, err := loadFont(12.0, canvas.FontRegular)
	if err != nil {
		t.Fatalf("loadFont failed: %v", err)
	}
	if faceB != faceC {
		t.Error("loadFont cache miss on repeated call")
	}
}

// ─── SVGFramesToGIF ──────────────────────────────────────────────

const testGIFSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200">
<rect width="200" height="200" rx="8" fill="#4ADE80"/>
<rect width="200" height="200" rx="8" fill="#000" opacity="0.15"/>
<text x="100" y="100" text-anchor="middle" fill="white" font-family="sans-serif" font-size="36" font-weight="700">test</text>
</svg>`

func TestSVGFramesToGIF_Basic(t *testing.T) {
	frames := [][]byte{[]byte(testGIFSVG), []byte(testGIFSVG)}
	delays := []int{120, 120}

	data, err := SVGFramesToGIF(frames, 196, 196, delays)
	if err != nil {
		t.Fatalf("SVGFramesToGIF failed: %v", err)
	}

	// Verify GIF89a header
	if len(data) < 6 || string(data[:6]) != "GIF89a" {
		t.Errorf("not a valid GIF: got header %q", data[:min(6, len(data))])
	}

	// Verify size well under 300KB
	if len(data) > 100*1024 {
		t.Errorf("GIF too large: %d bytes", len(data))
	}

	// Verify 2 frames
	r := bytes.NewReader(data)
	g, err := gif.DecodeAll(r)
	if err != nil {
		t.Fatalf("gif decode: %v", err)
	}
	if len(g.Image) != 2 {
		t.Errorf("frame count: got %d, want 2", len(g.Image))
	}
	if len(g.Delay) != 2 {
		t.Errorf("delay count: got %d, want 2", len(g.Delay))
	}
}

func TestSVGFramesToGIF_Delays(t *testing.T) {
	frames := [][]byte{[]byte(testGIFSVG), []byte(testGIFSVG)}
	// 120ms → 12cs, 2000ms → 200cs
	delays := []int{120, 2000}

	data, err := SVGFramesToGIF(frames, 196, 196, delays)
	if err != nil {
		t.Fatalf("SVGFramesToGIF failed: %v", err)
	}

	r := bytes.NewReader(data)
	g, err := gif.DecodeAll(r)
	if err != nil {
		t.Fatalf("gif decode: %v", err)
	}

	if g.Delay[0] != 12 {
		t.Errorf("delay[0]: got %d cs, want 12 cs", g.Delay[0])
	}
	if g.Delay[1] != 200 {
		t.Errorf("delay[1]: got %d cs, want 200 cs", g.Delay[1])
	}
}

func TestSVGFramesToGIF_Dimensions(t *testing.T) {
	frames := [][]byte{[]byte(testGIFSVG)}
	data, err := SVGFramesToGIF(frames, 196, 196, []int{100})
	if err != nil {
		t.Fatalf("SVGFramesToGIF failed: %v", err)
	}

	r := bytes.NewReader(data)
	g, err := gif.DecodeAll(r)
	if err != nil {
		t.Fatalf("gif decode: %v", err)
	}

	bounds := g.Image[0].Bounds()
	if bounds.Dx() != 196 || bounds.Dy() != 196 {
		t.Errorf("dimensions: got %dx%d, want 196x196", bounds.Dx(), bounds.Dy())
	}
}

func TestSVGFramesToGIF_Errors(t *testing.T) {
	if _, err := SVGFramesToGIF([][]byte{}, 196, 196, []int{}); err == nil {
		t.Error("expected error for empty frames")
	}
	if _, err := SVGFramesToGIF([][]byte{[]byte(testGIFSVG)}, 196, 196, []int{100, 200}); err == nil {
		t.Error("expected error for delay count mismatch")
	}
	if _, err := SVGFramesToGIF([][]byte{[]byte("not svg")}, 196, 196, []int{100}); err == nil {
		t.Error("expected error for invalid SVG")
	}
}
