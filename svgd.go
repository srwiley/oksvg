// Copyright 2017 The oksvg Authors. All rights reserved.
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
	"math"

	"github.com/srwiley/rasterx"
	"golang.org/x/image/colornames"
	"golang.org/x/image/math/fixed"
)

type (
	PathStyle struct {
		FillOpacity, LineOpacity          float64
		LineWidth, DashOffset, MiterLimit float64
		Dash                              []float64
		UseNonZeroWinding                 bool
		fillerColor, linerColor           interface{} // either color.Color or *Gradient
		LineGap                           rasterx.GapFunc
		LeadLineCap                       rasterx.CapFunc // This is used if different than LineCap
		LineCap                           rasterx.CapFunc
		LineJoin                          rasterx.JoinMode
		mAdder                            MatrixAdder // current transform
	}

	SvgPath struct {
		PathStyle
		Path rasterx.Path
	}

	SvgIcon struct {
		ViewBox      struct{ X, Y, W, H float64 }
		Titles       []string // Title elements collect here
		Descriptions []string // Description elements collect here
		Ids          map[string]interface{}
		SVGPaths     []SvgPath
	}

	IconCursor struct {
		PathCursor
		icon                                   *SvgIcon
		StyleStack                             []PathStyle
		grad                                   *Gradient
		inTitleText, inDescText, inGrad, inDef bool
	}
)

// DefaultStyle sets the default PathStyle to fill black, winding rule,
// full opacity, no stroke, ButtCap line end and Bevel line connect.
var DefaultStyle = PathStyle{1.0, 1.0, 2.0, 0.0, 4.0, nil, true,
	color.NRGBA{0x00, 0x00, 0x00, 0xff}, nil,
	nil, nil, rasterx.ButtCap, rasterx.Bevel, MatrixAdder{M: Identity}}

// Draws the compiled SVG icon into the GraphicContext.
// All elements should be contained by the Bounds rectangle of the SvgIcon.
func (s *SvgIcon) Draw(r *rasterx.Dasher, opacity float64) {
	for _, svgp := range s.SVGPaths {
		svgp.Draw(r, opacity)
	}
}

func ApplyOpacity(c color.Color, opacity float64) color.NRGBA {
	r, g, b, _ := c.RGBA()
	return color.NRGBA{uint8(r), uint8(g), uint8(b), uint8(opacity * 0xFF)}
}

// Draw the compiled SvgPath into the GraphicContext.
// All elements should be contained by the Bounds rectangle of the SvgIcon.
func (svgp *SvgPath) Draw(r *rasterx.Dasher, opacity float64) {
	if svgp.fillerColor != nil {
		r.Clear()
		rf := &r.Filler
		rf.SetWinding(svgp.UseNonZeroWinding)
		svgp.mAdder.Adder = rf // This allows transformations to be applied
		svgp.Path.AddTo(&svgp.mAdder)

		switch fillerColor := svgp.fillerColor.(type) {
		case color.Color:
			rf.SetColor(ApplyOpacity(fillerColor, svgp.FillOpacity*opacity))
		case *Gradient:
			if fillerColor.units == ObjectBoundingBox {
				fRect := rf.Scanner.GetPathExtent()
				mnx, mny := float64(fRect.Min.X)/64, float64(fRect.Min.Y)/64
				mxx, mxy := float64(fRect.Max.X)/64, float64(fRect.Max.Y)/64
				fillerColor.bounds.X, fillerColor.bounds.Y = mnx, mny
				fillerColor.bounds.W, fillerColor.bounds.H = mxx-mnx, mxy-mny
			}
			rf.SetColor(fillerColor.GetColorFunction(svgp.FillOpacity * opacity))
		}
		rf.Draw()
		// default is true
		rf.SetWinding(true)
	}
	if svgp.linerColor != nil {
		r.Clear()
		svgp.mAdder.Adder = r
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
		svgp.Path.AddTo(&svgp.mAdder)
		switch linerColor := svgp.linerColor.(type) {
		case color.Color:
			r.SetColor(ApplyOpacity(linerColor, svgp.LineOpacity*opacity))
		case *Gradient:
			if linerColor.units == ObjectBoundingBox {
				fRect := r.Scanner.GetPathExtent()
				mnx, mny := float64(fRect.Min.X)/64, float64(fRect.Min.Y)/64
				mxx, mxy := float64(fRect.Max.X)/64, float64(fRect.Max.Y)/64
				linerColor.bounds.X, linerColor.bounds.Y = mnx, mny
				linerColor.bounds.W, linerColor.bounds.H = mxx-mnx, mxy-mny
			}
			r.SetColor(linerColor.GetColorFunction(svgp.LineOpacity * opacity))
		}
		r.Draw()
	}
}

// ParseSVGColorNum reads the SFG color string e.g. #FBD9BD
func ParseSVGColorNum(colorStr string) (r, g, b uint8, err error) {
	colorStr = strings.TrimPrefix(colorStr, "#")
	var t uint64
	if len(colorStr) != 6 {
		// SVG specs say duplicate characters in case of 3 digit hex number
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
	if strings.HasPrefix(v, "url") { // We are not handling urls
		// and gradients and stuff at this point
		return color.NRGBA{0, 0, 0, 255}, nil
	}
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
				return nil, err
			}
		}
		return color.NRGBA{cvals[0], cvals[1], cvals[2], 0xFF}, nil
	}
	if colorStr[0] == '#' {
		r, g, b, err := ParseSVGColorNum(colorStr)
		if err != nil {
			return nil, err
		}
		return color.NRGBA{r, g, b, 0xFF}, nil
	}
	return nil, paramMismatchError
}

func parseColorValue(v string) (uint8, error) {
	if v[len(v)-1] == '%' {
		n, err := strconv.Atoi(strings.TrimSpace(v[:len(v)-1]))
		if err != nil {
			return 0, err
		}
		return uint8(n * 0xFF / 100), nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if n > 255 {
		n = 255
	}
	return uint8(n), err
}

func (c *IconCursor) parseTransform(v string) (Matrix2D, error) {
	ts := strings.Split(v, ")")
	m1 := c.StyleStack[len(c.StyleStack)-1].mAdder.M
	for _, t := range ts {
		t = strings.TrimSpace(t)
		if len(t) == 0 {
			continue
		}
		d := strings.Split(t, "(")
		if len(d) != 2 || len(d[1]) < 1 {
			return m1, paramMismatchError // badly formed transformation
		}
		err := c.GetPoints(d[1])
		if err != nil {
			return m1, err
		}
		ln := len(c.points)
		switch strings.ToLower(strings.TrimSpace(d[0])) {
		case "rotate":
			if ln == 1 {
				m1 = m1.Rotate(c.points[0] * math.Pi / 180)
			} else if ln == 3 {
				m1 = m1.Translate(c.points[1], c.points[2]).
					Rotate(c.points[0]*math.Pi/180).
					Translate(-c.points[1], -c.points[2])
			} else {
				return m1, paramMismatchError
			}
		case "translate":
			if ln == 1 {
				m1 = m1.Translate(c.points[0], 0)
			} else if ln == 2 {
				m1 = m1.Translate(c.points[0], c.points[1])
			} else {
				return m1, paramMismatchError
			}
		case "skewx":
			if ln == 1 {
				m1 = m1.SkewX(c.points[0] * math.Pi / 180)
			} else {
				return m1, paramMismatchError
			}
		case "skewy":
			if ln == 1 {
				m1 = m1.SkewY(c.points[0] * math.Pi / 180)
			} else {
				return m1, paramMismatchError
			}
		case "scale":
			if ln == 1 {
				m1 = m1.Scale(c.points[0], 0)
			} else if ln == 2 {
				m1 = m1.Scale(c.points[0], c.points[1])
			} else {
				return m1, paramMismatchError
			}
		case "matrix":
			if ln == 6 {
				m1 = m1.Mult(Matrix2D{
					c.points[0],
					c.points[1],
					c.points[2],
					c.points[3],
					c.points[4],
					c.points[5]})
			} else {
				return m1, paramMismatchError
			}
		default:
			return m1, paramMismatchError
		}
	}
	return m1, nil
}

// PushStyle parses the style element, and push it on the style stack. Only color and opacity are supported
// for fill. Note that this parses both the contents of a style attribute plus
// direct fill and opacity attributes.
func (c *IconCursor) PushStyle(se xml.StartElement) error {
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
	curStyle := c.StyleStack[len(c.StyleStack)-1]
	for _, pair := range pairs {
		kv := strings.Split(pair, ":")
		if len(kv) >= 2 {
			k := strings.ToLower(kv[0])
			k = strings.TrimSpace(k)
			v := strings.TrimSpace(kv[1])
			switch k {
			case "fill":
				gradient, err := c.ReadGradUrl(v)
				if err != nil {
					return err
				}
				if gradient != nil {
					curStyle.fillerColor = gradient
					break
				}
				curStyle.fillerColor, err = ParseSVGColor(v)
				if err != nil {
					return err
				}
			case "stroke":
				gradient, err := c.ReadGradUrl(v)
				if err != nil {
					return err
				}
				if gradient != nil {
					curStyle.linerColor = gradient
					break
				}
				col, errc := ParseSVGColor(v)
				if errc != nil {
					return errc
				}
				if col != nil {
					curStyle.linerColor = col.(color.NRGBA)
				} else {
					curStyle.linerColor = nil
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
					return err
				}
				curStyle.MiterLimit = mLimit
			case "stroke-width":
				v = strings.TrimSuffix(v, "px")
				width, err := strconv.ParseFloat(v, 64)
				if err != nil {
					return err
				}
				curStyle.LineWidth = width
			case "stroke-dashoffset":
				dashOffset, err := strconv.ParseFloat(v, 64)
				if err != nil {
					return err
				}
				curStyle.DashOffset = dashOffset
			case "stroke-dasharray":
				if v != "none" {
					dashes := strings.Split(v, ",")
					dList := make([]float64, len(dashes))
					for i, dstr := range dashes {
						d, err := strconv.ParseFloat(strings.TrimSpace(dstr), 64)
						if err != nil {
							return err
						}
						dList[i] = d
					}
					curStyle.Dash = dList
					break
				}
			case "opacity", "stroke-opacity", "fill-opacity":
				op, err := strconv.ParseFloat(v, 64)
				if err != nil {
					return err
				}
				if k != "stroke-opacity" {
					curStyle.FillOpacity *= op
				}
				if k != "fill-opacity" {
					curStyle.LineOpacity *= op
				}
			case "transform":
				m, err := c.parseTransform(v)
				if err != nil {
					return err
				}
				curStyle.mAdder.M = m
			}
		}
	}
	c.StyleStack = append(c.StyleStack, curStyle) // Push style onto stack
	return nil
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

	icon := &SvgIcon{Ids: make(map[string]interface{})}
	cursor := &IconCursor{StyleStack: []PathStyle{DefaultStyle}, icon: icon}
	if len(errMode) > 0 {
		cursor.ErrorMode = errMode[0]
	}
	decoder := xml.NewDecoder(fin)
	decoder.CharsetReader = charset.NewReaderLabel
	for {
		t, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return icon, err
		}
		// Inspect the type of the XML token
		switch se := t.(type) {
		case xml.StartElement:
			// Reads all recognized style attributes from the start element
			// and places it on top of the styleStack
			err = cursor.PushStyle(se)
			if err != nil {
				return icon, err
			}
			//fmt.Println("com", se.Name.Local)
			switch se.Name.Local {
			case "svg":
				icon.ViewBox.X = 0
				icon.ViewBox.Y = 0
				icon.ViewBox.W = 0
				icon.ViewBox.H = 0
				var width, height float64
				for _, attr := range se.Attr {
					switch attr.Name.Local {
					case "viewBox":
						err = cursor.GetPoints(attr.Value)
						if len(cursor.points) != 4 {
							return icon, paramMismatchError
						}
						icon.ViewBox.X = cursor.points[0]
						icon.ViewBox.Y = cursor.points[1]
						icon.ViewBox.W = cursor.points[2]
						icon.ViewBox.H = cursor.points[3]
					case "width":
						wn := strings.TrimSuffix(attr.Value, "cm")
						width, err = strconv.ParseFloat(wn, 64)
					case "height":
						hn := strings.TrimSuffix(attr.Value, "cm")
						height, err = strconv.ParseFloat(hn, 64)
					}
					if err != nil {
						return icon, err
					}
				}
				if icon.ViewBox.W == 0 {
					icon.ViewBox.W = width
				}
				if icon.ViewBox.H == 0 {
					icon.ViewBox.H = height
				}
			case "g": // G does nothing but push the style
			case "rect":
				var x, y, w, h float64
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
					if err != nil {
						return icon, err
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
				cursor.Path.Stop(true)
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
						return icon, err
					}
				}
				if rx == 0 || ry == 0 { // not drawn, but not an error
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
					if err != nil {
						return icon, err
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
							return icon, errors.New("polygon has odd number of points")
						}
					}
					if err != nil {
						return icon, err
					}
				}
				if len(cursor.points) > 4 {
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
					if err != nil {
						return icon, err
					}
				}
			case "desc":
				cursor.inDescText = true
				icon.Descriptions = append(icon.Descriptions, "")
			case "title":
				cursor.inTitleText = true
				icon.Titles = append(icon.Titles, "")
			case "def":
				cursor.inDef = true
			case "linearGradient":
				cursor.inGrad = true
				cursor.grad = &Gradient{points: [5]float64{0, 0, 1, 0, 0},
					isRadial: false, bounds: icon.ViewBox, matrix2D: Identity}
				for _, attr := range se.Attr {
					switch attr.Name.Local {
					case "id":
						id := attr.Value
						if len(id) >= 0 {
							icon.Ids[id] = cursor.grad
						} else {
							return icon, zeroLengthIdError
						}
					case "x1":
						cursor.grad.points[0], err = readFraction(attr.Value)
					case "y1":
						cursor.grad.points[1], err = readFraction(attr.Value)
					case "x2":
						cursor.grad.points[2], err = readFraction(attr.Value)
					case "y2":
						cursor.grad.points[3], err = readFraction(attr.Value)
					default:
						err = cursor.ReadGradAttr(attr)
					}
					if err != nil {
						return icon, err
					}
				}
			case "radialGradient":
				cursor.inGrad = true
				cursor.grad = &Gradient{points: [5]float64{0.5, 0.5, 0.5, 0.5, 0.5},
					isRadial: true, bounds: icon.ViewBox, matrix2D: Identity}
				var setFx, setFy bool
				for _, attr := range se.Attr {
					switch attr.Name.Local {
					case "id":
						id := attr.Value
						if len(id) >= 0 {
							icon.Ids[id] = cursor.grad
						} else {
							return icon, zeroLengthIdError
						}
					case "r":
						cursor.grad.points[4], err = readFraction(attr.Value)
					case "cx":
						cursor.grad.points[0], err = readFraction(attr.Value)
					case "cy":
						cursor.grad.points[1], err = readFraction(attr.Value)
					case "fx":
						setFx = true
						cursor.grad.points[2], err = readFraction(attr.Value)
					case "fy":
						setFy = true
						cursor.grad.points[3], err = readFraction(attr.Value)
					default:
						err = cursor.ReadGradAttr(attr)
					}
					if err != nil {
						return icon, err
					}
				}
				if setFx == false { // set fx to cx by default
					cursor.grad.points[2] = cursor.grad.points[0]
				}
				if setFy == false { // set fy to cy by default
					cursor.grad.points[3] = cursor.grad.points[1]
				}
			case "stop":
				if cursor.inGrad {
					stop := &GradStop{opacity: 1.0}
					for _, attr := range se.Attr {
						switch attr.Name.Local {
						case "offset":
							stop.offset, err = readFraction(attr.Value)
						case "stop-color":
							//todo: add current color inherit
							stop.stopColor, err = ParseSVGColor(attr.Value)
						case "stop-opacity":
							stop.opacity, err = strconv.ParseFloat(attr.Value, 64)
						}
						if err != nil {
							return icon, err
						}
					}
					cursor.grad.stops = append(cursor.grad.stops, stop)
				}

			default:
				errStr := "Cannot process svg element " + se.Name.Local
				if cursor.ErrorMode == StrictErrorMode {
					err = errors.New(errStr)
				} else if cursor.ErrorMode == WarnErrorMode {
					log.Println(errStr)
				}
			}
			if len(cursor.Path) > 0 {
				//The cursor parsed a path from the xml element
				pathCopy := make(rasterx.Path, len(cursor.Path))
				copy(pathCopy, cursor.Path)
				icon.SVGPaths = append(icon.SVGPaths,
					SvgPath{cursor.StyleStack[len(cursor.StyleStack)-1], pathCopy})
				cursor.Path = cursor.Path[:0]
			}
		case xml.EndElement:
			cursor.StyleStack = cursor.StyleStack[:len(cursor.StyleStack)-1]
			switch se.Name.Local {
			case "title":
				cursor.inTitleText = false
			case "desc":
				cursor.inDescText = false
			case "def":
				cursor.inDef = false
			case "radialGradient", "linearGradient":
				cursor.inGrad = false
			}
		case xml.CharData:
			if cursor.inTitleText == true {
				icon.Titles[len(icon.Titles)-1] += string(se)
			}
			if cursor.inDescText == true {
				icon.Descriptions[len(icon.Descriptions)-1] += string(se)
			}
		}
	}
	return icon, nil
}

func readFraction(v string) (f float64, err error) {
	v = strings.TrimSpace(v)
	d := 1.0
	if strings.HasSuffix(v, "%") {
		d = 100
		v = strings.TrimSuffix(v, "%")
	}
	f, err = strconv.ParseFloat(v, 64)
	f /= d
	if f > 1 {
		f = 1
	} else if f < 0 {
		f = 0
	}
	return
}

func (c *IconCursor) ReadGradUrl(v string) (grad *Gradient, err error) {
	if strings.HasPrefix(v, "url(") && strings.HasSuffix(v, ")") {
		urlStr := strings.TrimSpace(v[4 : len(v)-1])
		if strings.HasPrefix(urlStr, "#") {
			switch grad := c.icon.Ids[urlStr[1:]].(type) {
			case *Gradient:
				return grad, nil
			default:
				return nil, nil //missingIdError
			}

		}
	}
	return nil, nil // not a gradient url, and not an error
}

func (cursor *IconCursor) ReadGradAttr(attr xml.Attr) (err error) {
	switch attr.Name.Local {
	case "gradientTransform":
		cursor.grad.matrix2D, err = cursor.parseTransform(attr.Value)
	case "gradientUnits":
		switch strings.TrimSpace(attr.Value) {
		case "userSpaceOnUse":
			cursor.grad.units = UserSpaceOnUse
		case "objectBoundingBox":
			cursor.grad.units = ObjectBoundingBox
		}
	case "spreadMethod":
		switch strings.TrimSpace(attr.Value) {
		case "pad":
			cursor.grad.spread = PadSpread
		case "reflect":
			cursor.grad.spread = ReflectSpread
		case "repeat":
			cursor.grad.spread = RepeatSpread
		}
	}
	return nil
}
