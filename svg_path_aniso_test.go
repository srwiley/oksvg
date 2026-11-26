package oksvg_test

import (
	"image"
	"strings"
	"testing"

	. "github.com/srwiley/oksvg"
	. "github.com/srwiley/rasterx"
)

const anisoSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="128" height="128" viewBox="0 0 128 128">
  <g transform="scale(0.5, 2.0)">
    <rect x="32" y="8" width="64" height="48" style="fill:#3b82f6;stroke:#1e3a8a;stroke-width:12;stroke-linejoin:round;" />
  </g>
</svg>`

func anisoStroke(c interface{ RGBA() (r, g, b, a uint32) }) bool {
	r, g, b, a := c.RGBA()
	if a < 0x8000 {
		return false
	}
	return r>>8 < 45 && g>>8 < 75 && b>>8 < 160
}

// runs returns the lengths of the first two stroke runs along a scan line.
func runs(n int, at func(i int) interface{ RGBA() (r, g, b, a uint32) }) (first, second int) {
	count := 0
	seen := false
	for i := 0; i < n; i++ {
		if anisoStroke(at(i)) {
			count++
			seen = true
		} else if seen {
			if first == 0 {
				first = count
			} else {
				return first, count
			}
			count = 0
			seen = false
		}
	}
	return
}

func checkWidths(t *testing.T, got [4]int, want [4]int) {
	t.Helper()
	names := [4]string{"left vertical", "right vertical", "top horizontal", "bottom horizontal"}
	for i := range names {
		if got[i] != want[i] {
			t.Errorf("%s stroke thickness = %d, want %d", names[i], got[i], want[i])
		}
	}
}

func TestAnisotropicStrokeWidth(t *testing.T) {
	icon, err := ReadIconStream(strings.NewReader(anisoSVG), WarnErrorMode)
	if err != nil {
		t.Fatal(err)
	}
	w, h := int(icon.ViewBox.W), int(icon.ViewBox.H)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	scanner := NewScannerGV(w, h, img, img.Bounds())
	raster := NewDasher(w, h, scanner)
	icon.Draw(raster, 1.0)

	left, right := runs(w, func(x int) interface{ RGBA() (r, g, b, a uint32) } {
		return img.At(x, 64)
	})
	top, bottom := runs(h, func(y int) interface{ RGBA() (r, g, b, a uint32) } {
		return img.At(32, y)
	})

	// scale(0.5, 2.0) with stroke-width 12 gives 6px vertical and 24px horizontal strokes.
	checkWidths(t, [4]int{left, right, top, bottom}, [4]int{6, 6, 24, 24})
}

func TestAnisotropicStrokeWidthSetTarget(t *testing.T) {
	icon, err := ReadIconStream(strings.NewReader(anisoSVG), WarnErrorMode)
	if err != nil {
		t.Fatal(err)
	}
	// Stretch the 128x128 viewBox to 64x256 (0.5/2.0), which composes with the
	// element's own scale(0.5,2.0) to give 0.25/4.0. Stroke-width 12 then
	// becomes 3px vertically and 48px horizontally.
	w, h := 64, 256
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	scanner := NewScannerGV(w, h, img, img.Bounds())
	raster := NewDasher(w, h, scanner)
	icon.SetTarget(0, 0, float64(w), float64(h))
	icon.Draw(raster, 1.0)

	left, right := runs(w, func(x int) interface{ RGBA() (r, g, b, a uint32) } {
		return img.At(x, 128)
	})
	top, bottom := runs(h, func(y int) interface{ RGBA() (r, g, b, a uint32) } {
		return img.At(16, y)
	})
	checkWidths(t, [4]int{left, right, top, bottom}, [4]int{3, 3, 48, 48})
}

func TestAnisotropicStrokeWidthShear(t *testing.T) {
	const shearSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="128" height="128" viewBox="0 0 128 128">
  <g transform="matrix(1,0,0.5,1,0,0)">
    <rect x="32" y="8" width="64" height="48" style="fill:#3b82f6;stroke:#1e3a8a;stroke-width:12;" />
  </g>
</svg>`
	icon, err := ReadIconStream(strings.NewReader(shearSVG), WarnErrorMode)
	if err != nil {
		t.Fatal(err)
	}
	w, h := int(icon.ViewBox.W), int(icon.ViewBox.H)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	scanner := NewScannerGV(w, h, img, img.Bounds())
	raster := NewDasher(w, h, scanner)
	icon.Draw(raster, 1.0)

	// A pure shear preserves stroke thickness at constant y, so both stay 12;
	// its signature is the slanted edge, which shifts by 0.5 per unit of y.
	firstStrokeX := func(y int) int {
		for x := 0; x < w; x++ {
			if anisoStroke(img.At(x, y)) {
				return x
			}
		}
		return -1
	}
	x16, x48 := firstStrokeX(16), firstStrokeX(48)
	if x16 < 0 || x48 < 0 {
		t.Fatalf("no stroke found on shear scan lines: y=16 -> %d, y=48 -> %d", x16, x48)
	}
	if shift := x48 - x16; shift != 16 {
		t.Errorf("left edge shifted %dpx between y=16 and y=48, want 16", shift)
	}
}
