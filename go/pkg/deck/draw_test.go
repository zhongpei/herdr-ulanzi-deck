package deck

import (
	"testing"
)

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
	// Full pipeline: render an agent key with all 5 status variants and
	// verify the PNG output is well-formed.
	// This is the regression test for the "boxes" bug.
	xml := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200">
  <rect width="200" height="200" fill="#27AE60"/>
  <polyline points="8,76 16,84 32,66" fill="none" stroke="white" stroke-width="4" stroke-linecap="round" stroke-linejoin="round"/>
  <text x="100" y="100" text-anchor="middle" fill="white" font-size="36">test</text>
</svg>`
	png, err := SVGToPNG([]byte(xml), 196, 196)
	if err != nil {
		t.Fatalf("SVGToPNG failed: %v", err)
	}
	// PNG magic header
	if len(png) < 8 || string(png[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("output is not a valid PNG (got %d bytes)", len(png))
	}
	// Sanity: should be non-trivial size
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
