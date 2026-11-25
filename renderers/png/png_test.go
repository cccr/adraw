package png

import (
	"bytes"
	"image"
	stdpng "image/png"
	"testing"

	"github.com/cccr/adraw/pkg/drawing"
	"github.com/cccr/adraw/pkg/render"
)

func v(x, y float64) drawing.Vec2 { return drawing.Vec2{X: x, Y: y} }

func decode(t *testing.T, f drawing.Frame) image.Image {
	t.Helper()
	var buf bytes.Buffer
	if err := New().Render(&buf, f); err != nil {
		t.Fatalf("Render: %v", err)
	}
	img, err := stdpng.Decode(&buf)
	if err != nil {
		t.Fatalf("not a PNG: %v", err)
	}
	return img
}

func dark(img image.Image, mmX, mmY float64) bool {
	r, _, _, _ := img.At(int(mmX*scale), int(mmY*scale)).RGBA()
	return r < 0x8000
}

func TestRegisteredAsPNG(t *testing.T) {
	r, err := render.Get("png")
	if err != nil {
		t.Fatal(err)
	}
	if r.Ext() != "png" {
		t.Errorf("Ext = %q", r.Ext())
	}
}

func TestPNGSizeIsBoundsTimesScale(t *testing.T) {
	img := decode(t, drawing.Frame{Bounds: drawing.Rect{Max: v(30, 20)}})
	if b := img.Bounds(); b.Dx() != int(30*scale) || b.Dy() != int(20*scale) {
		t.Errorf("image is %dx%d, want %dx%d", b.Dx(), b.Dy(), int(30*scale), int(20*scale))
	}
	if dark(img, 15, 10) {
		t.Error("an empty frame is not white")
	}
}

func TestFilledShapeIsSolidAndHolesStayOpen(t *testing.T) {
	outer := drawing.Path{Closed: true, Fill: true, Pts: []drawing.Vec2{v(5, 5), v(25, 5), v(25, 25), v(5, 25)}}
	// the counter is wound the other way, as a font's counters are
	hole := drawing.Path{Closed: true, Fill: true, Pts: []drawing.Vec2{v(10, 10), v(10, 20), v(20, 20), v(20, 10)}}
	img := decode(t, drawing.Frame{Bounds: drawing.Rect{Max: v(30, 30)}, Paths: []drawing.Path{outer, hole}})
	if !dark(img, 7, 7) {
		t.Error("the ring of the shape is not filled")
	}
	if dark(img, 15, 15) {
		t.Error("the counter was filled")
	}
	if dark(img, 2, 2) {
		t.Error("outside the shape is not white")
	}
}

func TestUnfilledPathsAreLines(t *testing.T) {
	line := drawing.Path{Pts: []drawing.Vec2{v(2, 10), v(28, 10)}}
	img := decode(t, drawing.Frame{Bounds: drawing.Rect{Max: v(30, 20)}, Paths: []drawing.Path{line}})
	if !dark(img, 15, 10) {
		t.Error("the line is not drawn")
	}
	if dark(img, 15, 15) {
		t.Error("ink away from the line")
	}
}

func TestHugePageIsCapped(t *testing.T) {
	img := decode(t, drawing.Frame{Bounds: drawing.Rect{Max: v(2000, 2000)}})
	if b := img.Bounds(); b.Dx() > maxSide || b.Dy() > maxSide || b.Dx() < maxSide/2 {
		t.Errorf("image is %dx%d, want the long side capped at %d", b.Dx(), b.Dy(), maxSide)
	}
}

func TestEmptyBoundsRejected(t *testing.T) {
	var buf bytes.Buffer
	if err := New().Render(&buf, drawing.Frame{}); err == nil {
		t.Error("a frame with no extent should be an error")
	}
}
