// Copyright 2017 The oksvg Authors. All rights reserved.
//
// (c) 02/10/2021 by Andrii Raikov

package oksvg

import (
	"encoding/xml"
	"golang.org/x/image/font/gofont/gosmallcaps"
	"golang.org/x/image/font/gofont/gosmallcapsitalic"
	"image"
	"image/color"
	"regexp"
	"strings"

	"github.com/golang/freetype/truetype"

	"github.com/srwiley/rasterx"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gobolditalic"
	"golang.org/x/image/font/gofont/goitalic"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/math/fixed"

	cfp "github.com/raykov/css-font-parser"
)

var (
	textF svgFunc = func(c *IconCursor, attrs []xml.Attr) error {
		c.inText = true

		c.icon.SvgTexts = append(c.icon.SvgTexts, SvgText{Style: attrs, Data: ""})

		return nil
	}

	fontSizeRegexp = regexp.MustCompile("[^0-9\\.]+")
)

type (
	SvgText struct {
		Style []xml.Attr
		Data  string
	}
)

func (svgt *SvgText) DrawTransformed(img *image.RGBA, opacity float64, t rasterx.Matrix2D, classes map[string]styleAttribute) {
	var err error
	var col color.Color = color.Black

	textAnchor := "left"
	x, y := -1.0, -1.0
	fontSize := 10.0
	fontStyle, fontWeight, fontVariant := "", "", ""

	for _, attr := range svgt.Style {

		switch attr.Name.Local {
		case "class":
			cAttrs, ok := classes[strings.TrimSpace(attr.Value)]
			if ok {
				for cAttr, cAttrVal := range cAttrs {
					switch cAttr {
					case "font":
						eFont := cfp.Parse(cAttrVal)
						fontSize, _ = parseFloat(fontSizeRegexp.ReplaceAllString(eFont.Size, ""), 64)
						fontStyle, fontWeight, fontVariant = eFont.Style, eFont.Weight, eFont.Variant
					case "fill":
						col, err = ParseSVGColor(cAttrVal)
						if err != nil {
							col = color.Black
						}
					}
				}
			}
		case "font":
			eFont := cfp.Parse(attr.Value)
			fontSize, _ = parseFloat(fontSizeRegexp.ReplaceAllString(eFont.Size, ""), 64)
			fontStyle, fontWeight, fontVariant = eFont.Style, eFont.Weight, eFont.Variant
			// italic small-caps bold 12px/30px Georgia, serif
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

	var rawTTF []byte
	switch {
	case fontVariant == "small-caps" && fontStyle == "italic":
		rawTTF = gosmallcapsitalic.TTF
	case fontVariant == "small-caps":
		rawTTF = gosmallcaps.TTF
	case fontStyle == "italic" && fontWeight == "bold":
		rawTTF = gobolditalic.TTF
	case fontStyle == "italic":
		rawTTF = goitalic.TTF
	case fontWeight == "bold":
		rawTTF = gobold.TTF
	default:
		rawTTF = goregular.TTF
	}

	ff, err := truetype.Parse(rawTTF)
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
