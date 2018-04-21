// Copyright 2017 The oksvg Authors. All rights reserved.
// Use of this source code is governed by your choice of either the
// FreeType License or the GNU General Public License version 2 (or
// any later version), both of which can be found in the LICENSE file.
//
// created: 2/12/2017 by S.R.Wiley
// The oksvg package provides a partial implementation of the SVG 2.0 standard.
// It can perform all SVG2.0 path commands, including arc and miterclip. It also
// has some additional capabilities like arc-clip. Svgdraw does
// not implement all SVG features such as animation or markers, but it can draw
// the many of open source SVG icons correctly. See Readme for
// a list of features.

package oksvg

import (
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/net/html/charset"

	"encoding/xml"
	"errors"
	"image/color"
	"log"

	"github.com/srwiley/rasterx"
	"golang.org/x/image/colornames"
	"golang.org/x/image/math/fixed"
)

type (
	PathStyle struct {
		FillOpacity, LineOpacity          float64
		LineWidth, DashOffset, MiterLimit float64
		Dash                              []float64
		DoFill, DoLine, UseNonZeroWinding bool
		FillColor, LineColor              color.NRGBA
		LineGap                           rasterx.GapFunc
		LeadLineCap                       rasterx.CapFunc // This is used if different than LineCap
		LineCap                           rasterx.CapFunc
		LineJoin                          rasterx.JoinMode
	}

	SvgPath struct {
		PathStyle
		Path rasterx.Path
	}

	SvgIcon struct {
		ViewBox  struct{ X, Y, W, H float64 }
		SVGPaths []SvgPath
	}
)

// DefaultStyle sets the default PathStyle to fill black, winding rule,
// no opacity, no stroke, ButtCap line end and Bevel line connect.
var DefaultStyle = PathStyle{1.0, 1.0, 2.0, 0.0, 4.0, nil, true, false, true,
	color.NRGBA{0x00, 0x00, 0x00, 0xff}, color.NRGBA{0x00, 0x00, 0x00, 0xff},
	nil, nil, rasterx.ButtCap, rasterx.Bevel}

// Draws the compiled SVG icon into the GraphicContext.
// All elements should be contained by the Bounds rectangle of the SvgIcon.
func (s *SvgIcon) Draw(r *rasterx.Dasher, rgbPainter *rasterx.RGBAPainter, opacity float64) {
	for _, svgp := range s.SVGPaths {
		svgp.Draw(r, rgbPainter, opacity)
	}
}

// Draw the compiled SVG icon into the GraphicContext.
// All elements should be contained by the Bounds rectangle of the SvgIcon.
func (svgp *SvgPath) Draw(r *rasterx.Dasher, rgbPainter *rasterx.RGBAPainter, opacity float64) {
	if svgp.DoFill {
		r.Clear()
		ar, g, b, _ := svgp.FillColor.RGBA()
		rgbPainter.SetColor(color.NRGBA{uint8(ar), uint8(g), uint8(b), uint8(svgp.FillOpacity * opacity * 0xFF)})
		// rf will directly call the filler methods for path commands
		rf := &r.Filler

		if svgp.UseNonZeroWinding == false {
			rf.UseNonZeroWinding = false
		}
		svgp.Path.AddTo(rf)
		rf.Rasterize(rgbPainter)
		// default is true
		rf.UseNonZeroWinding = true
	}
	if svgp.DoLine {
		r.Clear()
		lineGap := svgp.LineGap
		if lineGap == nil {
			lineGap = DefaultStyle.LineGap
		}
		lineCap := svgp.LineCap
		if lineCap == nil {
			lineCap = DefaultStyle.LineCap
		}
		leadLineCap := lineCap
		if svgp.LeadLineCap != nil {
			leadLineCap = svgp.LeadLineCap
		}
		r.SetStroke(fixed.Int26_6(svgp.LineWidth*64),
			fixed.Int26_6(svgp.MiterLimit*64), leadLineCap, lineCap,
			lineGap, svgp.LineJoin, svgp.Dash, svgp.DashOffset)
		svgp.Path.AddTo(r)
		ar, g, b, _ := svgp.LineColor.RGBA()
		rgbPainter.SetColor(color.NRGBA{uint8(ar), uint8(g), uint8(b),
			uint8(svgp.LineOpacity * opacity * 0xFF)})
		r.Rasterize(rgbPainter)
	}
}

// ParseSVGColorNum reads the SFG color string e.g. #FBD9BD
func ParseSVGColorNum(colorStr string) (r, g, b uint8, err error) {
	colorStr = strings.TrimPrefix(colorStr, "#")
	var t uint64
	if len(colorStr) != 6 { // SVG specs say duplicate characters in case of 3 digit hex number
		colorStr = string([]byte{colorStr[0], colorStr[0],
			colorStr[1], colorStr[1], colorStr[2], colorStr[2]})
	}
	for _, v := range []struct {
		c *uint8
		s string
	}{
		{&r, colorStr[0:2]},
		{&g, colorStr[2:4]},
		{&b, colorStr[4:6]}} {
		t, err = strconv.ParseUint(v.s, 16, 8)
		if err != nil {
			return
		}
		*v.c = uint8(t)
	}
	return
}

// ParseSVGColor parses an SVG color string in all forms
// including all SVG1.1 names, obtained from the colornames package
func ParseSVGColor(colorStr string) (color.Color, error) {
	//_, _, _, a := curColor.RGBA()
	v := strings.ToLower(colorStr)
	switch v {
	case "none":
		// nil signals that the function (fill or stroke) is off;
		// not the same as black
		return nil, nil
	default:
		cn, ok := colornames.Map[v]
		if ok {
			r, g, b, a := cn.RGBA()
			return color.NRGBA{uint8(r), uint8(g), uint8(b), uint8(a)}, nil
		}
	}
	cStr := strings.TrimPrefix(colorStr, "rgb(")
	if cStr != colorStr {
		cStr := strings.TrimSuffix(cStr, ")")
		vals := strings.Split(cStr, ",")
		if len(vals) != 3 {
			return color.NRGBA{}, paramMismatchError
		}
		var cvals [3]uint8
		var err error
		for i := range cvals {
			cvals[i], err = parseColorValue(vals[i])
			if err != nil {
				return color.NRGBA{0, 0, 0, 0}, err
			}
		}
		return color.NRGBA{cvals[0], cvals[1], cvals[2], 0xFF}, nil
	}
	if colorStr[0] == '#' {
		r, g, b, err := ParseSVGColorNum(colorStr)
		if err != nil {
			return color.NRGBA{0, 0, 0, 0}, err
		}
		return color.NRGBA{r, g, b, 0xFF}, nil
	}
	return color.NRGBA{0, 0, 0, 0}, paramMismatchError
}

func parseColorValue(v string) (uint8, error) {
	if v[len(v)-1] == '%' {
		n, err := strconv.Atoi(v[:len(v)-1])
		if err != nil {
			return 0, err
		}
		return uint8(n * 0xFF / 100), nil
	}
	n, err := strconv.Atoi(v)
	return uint8(n), err
}

// PushStyle parses the style element, and push it on the style stack. Only color and opacity are supported
// for fill. Note that this parses both the contents of a style attribute plus
// direct fill and opacity attributes.
func PushStyle(se xml.StartElement, stack []PathStyle) ([]PathStyle, error) {
	var pairs []string
	for _, attr := range se.Attr {
		switch strings.ToLower(attr.Name.Local) {
		case "style":
			pairs = append(pairs, strings.Split(attr.Value, ";")...)
		default:
			pairs = append(pairs, attr.Name.Local+":"+attr.Value)
		}
	}
	// Make a copy of the top style
	curStyle := stack[len(stack)-1]
	//var errc error
	for _, pair := range pairs {
		kv := strings.Split(pair, ":")
		if len(kv) >= 2 {
			k := strings.ToLower(kv[0])
			v := strings.Trim(kv[1], " ")
			switch k {
			case "fill":
				col, errc := ParseSVGColor(v)
				if errc != nil {
					return stack, errc
				}
				//fmt.Println("do fill ", col)
				if curStyle.DoFill = col != nil; curStyle.DoFill {
					curStyle.FillColor = col.(color.NRGBA)
				}
			case "stroke":
				col, errc := ParseSVGColor(v)
				if errc != nil {
					return stack, errc
				}
				if curStyle.DoLine = col != nil; curStyle.DoLine {
					curStyle.LineColor = col.(color.NRGBA)
				}
			case "stroke-linegap":
				switch v {
				case "flat":
					curStyle.LineGap = rasterx.FlatGap
				case "round":
					curStyle.LineGap = rasterx.RoundGap
				case "cubic":
					curStyle.LineGap = rasterx.CubicGap
				case "quadratic":
					curStyle.LineGap = rasterx.QuadraticGap
				}
			case "stroke-leadlinecap":
				switch v {
				case "butt":
					curStyle.LeadLineCap = rasterx.ButtCap
				case "round":
					curStyle.LeadLineCap = rasterx.RoundCap
				case "square":
					curStyle.LeadLineCap = rasterx.SquareCap
				case "cubic":
					curStyle.LeadLineCap = rasterx.CubicCap
				case "quadratic":
					curStyle.LeadLineCap = rasterx.QuadraticCap
				}
			case "stroke-linecap":
				switch v {
				case "butt":
					curStyle.LineCap = rasterx.ButtCap
				case "round":
					curStyle.LineCap = rasterx.RoundCap
				case "square":
					curStyle.LineCap = rasterx.SquareCap
				case "cubic":
					curStyle.LineCap = rasterx.CubicCap
				case "quadratic":
					curStyle.LineCap = rasterx.QuadraticCap
				}
			case "stroke-linejoin":
				switch v {
				case "miter":
					curStyle.LineJoin = rasterx.Miter
				case "miter-clip":
					curStyle.LineJoin = rasterx.MiterClip
				case "arc-clip":
					curStyle.LineJoin = rasterx.ArcClip
				case "round":
					curStyle.LineJoin = rasterx.Round
				case "arc":
					curStyle.LineJoin = rasterx.Arc
				case "bevel":
					curStyle.LineJoin = rasterx.Bevel
				}
			case "stroke-miterlimit":
				mLimit, err := strconv.ParseFloat(v, 64)
				if err != nil {
					return stack, err
				}
				curStyle.MiterLimit = mLimit
			case "stroke-width":
				v = strings.TrimSuffix(v, "px")
				width, err := strconv.ParseFloat(v, 64)
				if err != nil {
					return stack, err
				}
				curStyle.LineWidth = width
			case "stroke-dashoffset":
				dashOffset, err := strconv.ParseFloat(v, 64)
				if err != nil {
					return stack, err
				}
				curStyle.DashOffset = dashOffset
			case "stroke-dasharray":
				if v != "none" {
					dashes := strings.Split(v, ",")
					dList := make([]float64, len(dashes))
					for i, dstr := range dashes {
						d, err := strconv.ParseFloat(strings.Trim(dstr, " "), 64)
						if err != nil {
							return stack, err
						}
						dList[i] = d
					}
					curStyle.Dash = dList
					break
				}
			case "opacity", "stroke-opacity", "fill-opacity":
				op, err := strconv.ParseFloat(v, 64)
				if err != nil {
					return stack, err
				}
				if k != "stroke-opacity" {
					curStyle.FillOpacity *= op
				}
				if k != "fill-opacity" {
					curStyle.LineOpacity *= op
				}
			}
		}
	}
	stack = append(stack, curStyle) // Push style onto stack
	return stack, nil
}

// ReadIcon reads the Icon from the named file
// This only supports a sub-set of SVG, but
// is enough to draw many icons. If errMode is provided,
// the first value determines if the icon ignores, errors out, or logs a warning
// if it does not handle an element found in the iconFile. Ignore warnings is
// the default if no ErrorMode value is provided.
func ReadIcon(iconFile string, errMode ...ErrorMode) (*SvgIcon, error) {
	fin, errf := os.Open(iconFile)
	if errf != nil {
		return nil, errf
	}
	defer fin.Close()

	icon := SvgIcon{}
	cursor := &SvgCursor{StyleStack: []PathStyle{DefaultStyle}}
	if len(errMode) > 0 {
		cursor.ErrorMode = errMode[0]
	}
	decoder := xml.NewDecoder(fin)
	decoder.CharsetReader = charset.NewReaderLabel
	for {
		cursor.init()
		t, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return &icon, err
		}
		// Inspect the type of the XML token
		switch se := t.(type) {
		case xml.StartElement:
			// Reads all recognized style attributes from the start element
			// and places it on top of the styleStack
			cursor.StyleStack, err = PushStyle(se, cursor.StyleStack)
			if err != nil {
				return &icon, err
			}
			switch se.Name.Local {
			case "svg":
				for _, attr := range se.Attr {
					switch attr.Name.Local {
					case "viewBox":
						err = cursor.GetPoints(attr.Value)
						if len(cursor.points) != 4 {
							return &icon, paramMismatchError
						}
						icon.ViewBox.X = cursor.points[0]
						icon.ViewBox.Y = cursor.points[1]
						icon.ViewBox.W = cursor.points[2]
						icon.ViewBox.H = cursor.points[3]
					}
				}
			case "g": // G does nothing but push the style
			case "rect":
				var x, y, w, h float64 //= math.NaN(),math.NaN(),math.NaN(),math.NaN()
				for _, attr := range se.Attr {
					switch attr.Name.Local {
					case "x":
						x, err = strconv.ParseFloat(attr.Value, 64)
					case "y":
						y, err = strconv.ParseFloat(attr.Value, 64)
					case "width":
						w, err = strconv.ParseFloat(attr.Value, 64)
					case "height":
						h, err = strconv.ParseFloat(attr.Value, 64)
					}
				}
				if w == 0 || h == 0 {
					break
				}

				startPt := fixed.Point26_6{
					fixed.Int26_6(x * 64),
					fixed.Int26_6(y * 64)}
				cursor.Path.Start(startPt)
				cursor.Path.Line(fixed.Point26_6{
					fixed.Int26_6((x + w) * 64),
					fixed.Int26_6(y * 64)})
				cursor.Path.Line(fixed.Point26_6{
					fixed.Int26_6((x + w) * 64),
					fixed.Int26_6((y + h) * 64)})
				cursor.Path.Line(fixed.Point26_6{
					fixed.Int26_6(x * 64),
					fixed.Int26_6((y + h) * 64)})
				cursor.Path.Line(startPt)
			case "circle", "ellipse":
				var cx, cy, rx, ry float64
				for _, attr := range se.Attr {
					switch attr.Name.Local {
					case "cx":
						cx, err = strconv.ParseFloat(attr.Value, 64)
					case "cy":
						cy, err = strconv.ParseFloat(attr.Value, 64)
					case "r":
						rx, err = strconv.ParseFloat(attr.Value, 64)
						ry = rx
					case "rx":
						rx, err = strconv.ParseFloat(attr.Value, 64)
					case "ry":
						ry, err = strconv.ParseFloat(attr.Value, 64)
					}
					if err != nil {
						return &icon, err
					}
				}
				if rx == 0 || ry == 0 { // not drawn
					break
				}
				cursor.ElipseAt(cx, cy, rx, ry)
			case "line":
				var x1, x2, y1, y2 float64
				for _, attr := range se.Attr {
					switch attr.Name.Local {
					case "x1":
						x1, err = strconv.ParseFloat(attr.Value, 64)
					case "x2":
						x2, err = strconv.ParseFloat(attr.Value, 64)
					case "y1":
						y1, err = strconv.ParseFloat(attr.Value, 64)
					case "y2":
						y2, err = strconv.ParseFloat(attr.Value, 64)
					}
				}
				cursor.Path.Start(fixed.Point26_6{
					fixed.Int26_6(x1 * 64),
					fixed.Int26_6(y1 * 64)})
				cursor.Path.Line(fixed.Point26_6{
					fixed.Int26_6(x2 * 64),
					fixed.Int26_6(y2 * 64)})
			case "polygon", "polyline":
				for _, attr := range se.Attr {
					switch attr.Name.Local {
					case "points":
						err = cursor.GetPoints(attr.Value)
						if len(cursor.points)%2 != 0 {
							return &icon, errors.New("polygon has odd number of points")
						}
					}
				}
				if len(cursor.points) > 4 {
					//cursor.Path.MoveTo(cursor.points[0], cursor.points[1])
					cursor.Path.Start(fixed.Point26_6{
						fixed.Int26_6(cursor.points[0] * 64),
						fixed.Int26_6(cursor.points[1] * 64)})
					for i := 2; i < len(cursor.points)-1; i += 2 {
						cursor.Path.Line(fixed.Point26_6{
							fixed.Int26_6(cursor.points[i] * 64),
							fixed.Int26_6(cursor.points[i+1] * 64)})
					}
					if se.Name.Local == "polygon" { // SVG spec sez polylines dont have close
						cursor.Path.Stop(true)
					}
				}
			case "path":
				for _, attr := range se.Attr {
					switch attr.Name.Local {
					case "d":
						err = cursor.CompilePath(attr.Value)
					}
				}
			default:
				errStr := "Cannot process svg element " + se.Name.Local
				if cursor.ErrorMode == StrictErrorMode {
					err = errors.New(errStr)
				}
				if cursor.ErrorMode == WarnErrorMode {
					log.Println(errStr)
				}
			}
			if err != nil {
				return &icon, err
			}
			if len(cursor.Path) > 0 {
				//The cursor parsed a path from the xml element
				pathCopy := make(rasterx.Path, len(cursor.Path))
				copy(pathCopy, cursor.Path)
				icon.SVGPaths = append(icon.SVGPaths,
					SvgPath{cursor.StyleStack[len(cursor.StyleStack)-1], pathCopy})
			}
		case xml.EndElement:
			cursor.StyleStack = cursor.StyleStack[:len(cursor.StyleStack)-1]
		}
	}
	return &icon, nil
}
