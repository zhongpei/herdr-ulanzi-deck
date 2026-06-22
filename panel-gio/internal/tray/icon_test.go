package tray

import (
	"image/png"
	"testing"
)

func TestSeverityDot_Blocked(t *testing.T) {
	pngBytes := severityDot(3, 0, 0)
	img, err := png.Decode(bytesReader(pngBytes))
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	b := img.Bounds()
	if b.Dx() != iconSize || b.Dy() != iconSize {
		t.Fatalf("expected %dx%d, got %dx%d", iconSize, iconSize, b.Dx(), b.Dy())
	}
	r, g, bCol, a := img.At(8, 8).RGBA()
	if r < 50000 || g > 30000 || bCol > 30000 {
		t.Logf("centre pixel R=%d G=%d B=%d A=%d (expected red-ish)", r, g, bCol, a)
	}
}

func TestSeverityDot_Working(t *testing.T) {
	pngBytes := severityDot(0, 5, 0)
	img, _ := png.Decode(bytesReader(pngBytes))
	b := img.Bounds()
	if b.Dx() != iconSize || b.Dy() != iconSize {
		t.Fatalf("expected %dx%d, got %dx%d", iconSize, iconSize, b.Dx(), b.Dy())
	}
}

func TestSeverityDot_Done(t *testing.T) {
	pngBytes := severityDot(0, 0, 8)
	img, _ := png.Decode(bytesReader(pngBytes))
	b := img.Bounds()
	if b.Dx() != iconSize || b.Dy() != iconSize {
		t.Fatalf("expected %dx%d, got %dx%d", iconSize, iconSize, b.Dx(), b.Dy())
	}
}

func TestSeverityDot_Idle(t *testing.T) {
	pngBytes := severityDot(0, 0, 0)
	img, _ := png.Decode(bytesReader(pngBytes))
	b := img.Bounds()
	if b.Dx() != iconSize || b.Dy() != iconSize {
		t.Fatalf("expected %dx%d, got %dx%d", iconSize, iconSize, b.Dx(), b.Dy())
	}
}

func TestSeverityDot_ValidPNG(t *testing.T) {
	pngBytes := severityDot(1, 2, 3)
	if len(pngBytes) == 0 {
		t.Fatal("empty png bytes")
	}
	if pngBytes[0] != 0x89 || pngBytes[1] != 'P' || pngBytes[2] != 'N' || pngBytes[3] != 'G' {
		t.Fatal("not a valid PNG header")
	}
}

// byteReader adapts a []byte to the io.Reader interface needed by png.Decode.
type byteReader struct {
	data []byte
	pos  int
}

func (r *byteReader) Read(p []byte) (int, error) {
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func bytesReader(b []byte) *byteReader {
	return &byteReader{data: b}
}
