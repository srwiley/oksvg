package oksvg_test

import (
	"bufio"
	"fmt"
	"image"
	"os"

	"image/png"
	"strings"
	"testing"

	. "github.com/srwiley/oksvg"
	. "github.com/srwiley/rasterx"
	"golang.org/x/image/math/fixed"
)

const testArco = `M150,350 l 50,-55 
           a25,25 -30 0,1 50,-25 l 50,-25 
           a25,50 -30 0,1 50,-25 l 50,-25 
           a25,75 -30 0,1 50,-25 l 50,-25 
           a25,100 -30 0,1 50,-25 l 50,15z`

const testArco2 = `M150,350 l 50,-55 
           a35,25 -30 0,0 50,-25 l 50,-25 
           a25,50 -30 0,1 50,-25 l 50,-25 
           a25,75 -30 0,1 50,-25 l 50,-25 
           a25,100 -30 0,1 50,-25, l 50,15z`

const testArcoS = `M150,350 l 50,-55 
           a35,25 -30 0,0 50,-25,
           25,50 -30 0,1 50,-25
           a25,75 -30 0,1 50,-25 l 50,-25 
           a25,100 -30 0,1 50,-25 l 50,15,0,25,-15,-15  z`

// Explicitly call each command in abs and rel mode and concatenated forms
const testSVG0 = `m20,20,0,400,400,0z`
const testSVG1 = `M20,20 L500,800 L800,200z`
const testSVG2 = `M20,20 Q200,800 800,800z`
const testSVG3 = `M20,50 C200,200 800,200 800,500z`
const testSVG4 = `M20,50 S200,1400 400,500 S700,800 800,400z`
const testSVG5 = `M50,20 Q 800,500 500,800z`
const testSVG6 = `M20,50 c200,200 800,200 400,300z`
const testSVG7 = `M20,20 c0,500 500,0 500,500z`
const testSVG8 = `M20,50 c200,200 800,200 400,300c200,200 800,200 400,300z`
const testSVG9 = `M20,50 c200,200 800,200 400,300,200,200 800,200 400,300z`
const testSVG10 = `M20,50 c200,200 800,200 400,300,200,200 800,200 400,300s500,300 200,200s600,300 200,200z`
const testSVG11 = `M20,50 c200,200 800,200 400,300,200,200 800,200 400,300s500,300 200,200,600,300 200,200z`
const testSVG12 = `M100,100 Q400,100 250,250 T400,400z`
const testSVG13 = `M100,100 Q400,100 250,250 t150,150,150,150z`

func DrawIcon(t *testing.T, iconPath string) {
	icon, errSvg := ReadIcon(iconPath, WarnErrorMode)
	if errSvg != nil {
		t.Error(errSvg)
		return
	}
	img := image.NewRGBA(image.Rect(0, 0, int(icon.ViewBox.W), int(icon.ViewBox.H)))
	painter := NewRGBAPainter(img)
	raster := NewDasher(int(icon.ViewBox.W), int(icon.ViewBox.H))
	icon.Draw(raster, painter, 1.0)
	p := strings.Split(iconPath, "/")
	err := SaveToPngFile(fmt.Sprintf("testdata/%s.png", p[len(p)-1]), img)
	if err != nil {
		t.Error(err)
	}
}

func SaveToPngFile(filePath string, m image.Image) error {
	// Create the file
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	// Create Writer from file
	b := bufio.NewWriter(f)
	// Write the image into the buffer
	err = png.Encode(b, m)
	if err != nil {
		return err
	}
	err = b.Flush()
	if err != nil {
		return err
	}
	return nil
}

func _TestSvgPathsStroke(t *testing.T) {
	for _, v := range []string{"fill", "stroke"} {
		for i, p := range []string{testArco, testArco2, testArcoS,
			testSVG0, testSVG1, testSVG2, testSVG3, testSVG4, testSVG5,
			testSVG6, testSVG7, testSVG8, testSVG9, testSVG10,
			testSVG11, testSVG12, testSVG13,
		} {
			//t.Log(p)
			img := image.NewRGBA(image.Rect(0, 0, 1600, 1600))
			painter := NewRGBAPainter(img)
			raster := NewDasher(1600, 1600)
			c := &SvgCursor{}
			d := DefaultStyle
			if v == "stroke" {
				d.DoFill = false
				d.DoLine = true
			}
			icon := SvgIcon{}

			err := c.CompilePath(p)
			if err != nil {
				t.Error(err)
			}
			icon.SVGPaths = append(icon.SVGPaths, SvgPath{d, c.Path})
			icon.Draw(raster, painter, 1)

			err = SaveToPngFile(fmt.Sprintf("testdata/%s%d.png", v, i), img)
			if err != nil {
				t.Error(err)
			}
		}

	}
}

func _TestLandscapeIcons(t *testing.T) {
	for _, p := range []string{
		"beach", "cape", "iceberg", "island",
		"mountains", "sea", "trees", "village"} {
		t.Log("reading ", p)
		DrawIcon(t, "testdata/landscapeIcons/"+p+".svg")
	}
}

func TestTestIcons(t *testing.T) {
	for _, p := range []string{
		"astronaut", "jupiter", "lander", "school-bus", "telescope", "diagram"} {
		t.Log("reading ", p)
		DrawIcon(t, "testdata/testIcons/"+p+".svg")
	}
}

func TestStrokeIcons(t *testing.T) {
	for _, p := range []string{
		"OpacityStrokeDashTest.svg",
		"OpacityStrokeDashTest2.svg",
		"OpacityStrokeDashTest3.svg",
		"TestShapes.svg",
		"TestShapes2.svg",
		"TestShapes3.svg",
		"TestShapes4.svg",
		"TestShapes5.svg",
	} {
		t.Log("reading ", p)
		DrawIcon(t, "testdata/"+p)
	}
}

func TestCircleLineIntersect(t *testing.T) {
	a := fixed.Point26_6{30 * 64, 55 * 64}
	b := fixed.Point26_6{40 * 64, 40 * 64}
	c := fixed.Point26_6{40 * 64, 40 * 64}
	r := fixed.Int26_6(10 * 64)
	x1, touching := RayCircleIntersection(a, b, c, r)
	t.Log("x1, t ", x1, touching)
}
