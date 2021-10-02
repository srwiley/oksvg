package oksvg

import (
	"encoding/xml"
	"image"
	"image/color"
	"regexp"
	"strings"

	"github.com/golang/freetype/truetype"

	"github.com/srwiley/rasterx"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/math/fixed"
)

var (
	textF svgFunc = func(c *IconCursor, attrs []xml.Attr) error {
		c.inText = true

		c.icon.SvgTexts = append(c.icon.SvgTexts, SvgText{Style: attrs, Data: ""})

		return nil
	}
)

type (
	SvgText struct {
		Style []xml.Attr
		Data  string
	}
)

func (svgt *SvgText) DrawTransformed(img *image.RGBA, opacity float64, t rasterx.Matrix2D) {
	var err error
	var col color.Color

	textAnchor := "left"
	x, y := -1.0, -1.0
	fontSize := 10.0
	//fontFamily := ""
	for _, attr := range svgt.Style {
		switch attr.Name.Local {
		case "font-size":
			reg := regexp.MustCompile("[^0-9\\.]+")
			fontSize, _ = parseFloat(reg.ReplaceAllString(attr.Value, ""), 64)
		case "font-family":
			//fontFamily = attr.Value
		case "text-anchor":
			textAnchor = attr.Value
		case "fill":
			col, err = ParseSVGColor(attr.Value)
			if err != nil {
				col = color.Black
			}
		case "y":
			y, _ = parseFloat(strings.TrimSpace(attr.Value), 64)
		case "x":
			x, _ = parseFloat(strings.TrimSpace(attr.Value), 64)
		}
	}

	if x < 0 || y < 0 {
		return
	}

	point := fixed.Point26_6{X: fixed.Int26_6(x * 64), Y: fixed.Int26_6(y * 64)}

	ff, err := truetype.Parse(goregular.TTF)
	ttf := truetype.NewFace(ff, &truetype.Options{Size: fontSize})

	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: ttf,
		Dot:  point,
	}

	if textAnchor == "middle" {
		w := d.MeasureString(svgt.Data)
		d.Dot.X = fixed.Int26_6(x*64) - w/2
	}

	d.DrawString(svgt.Data)
}
