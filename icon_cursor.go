// Copyright 2017 The oksvg Authors. All rights reserved.
// created: 2/12/2017 by S.R.Wiley
//

package oksvg

import (
	"encoding/xml"
	"errors"
	"fmt"
	"image/color"
	"log"
	"math"
	"strings"

	"github.com/srwiley/rasterx"
)

type (
	// state saves parsing state information
	state struct {
		name  string // element name of the state
		style PathStyle
		attr  []xml.Attr // Stored for some tags
		text  string     // Text accumulates here

		// FontSize                          string // These are actually stored in the attrs stack
		// FontFamily                        string
		// FontKerning                       bool

		inGrad, inDefs, inText bool
	}

	// definition is used to store XML-tags of SVG source definitions data.
	definition struct {
		ID, Tag string
		Attrs   []xml.Attr
	}

	// IconCursor is used while parsing SVG files.
	IconCursor struct {
		PathCursor
		icon       *SvgIcon
		states     []state
		grad       *rasterx.Gradient
		fonts      []Ttx
		currentDef []definition
	}
)

// ReadGradURL reads an SVG format gradient url
// Since the context of the gradient can affect the colors
// the current fill or line color is passed in and used in
// the case of a nil stopClor value
func (c *IconCursor) ReadGradURL(v string, defaultColor interface{}) (grad rasterx.Gradient, ok bool) {
	if strings.HasPrefix(v, "url(") && strings.HasSuffix(v, ")") {
		urlStr := strings.TrimSpace(v[4 : len(v)-1])
		if strings.HasPrefix(urlStr, "#") {
			var g *rasterx.Gradient
			g, ok = c.icon.Grads[urlStr[1:]]
			if ok {
				grad = localizeGradIfStopClrNil(g, defaultColor)
			}
		}
	}
	return
}

// ReadGradAttr reads an SVG gradient attribute.
func (c *IconCursor) ReadGradAttr(attr xml.Attr) (err error) {
	switch attr.Name.Local {
	case "gradientTransform":
		c.grad.Matrix, err = c.parseTransform(attr.Value)
	case "gradientUnits":
		switch strings.TrimSpace(attr.Value) {
		case "userSpaceOnUse":
			c.grad.Units = rasterx.UserSpaceOnUse
		case "objectBoundingBox":
			c.grad.Units = rasterx.ObjectBoundingBox
		}
	case "spreadMethod":
		switch strings.TrimSpace(attr.Value) {
		case "pad":
			c.grad.Spread = rasterx.PadSpread
		case "reflect":
			c.grad.Spread = rasterx.ReflectSpread
		case "repeat":
			c.grad.Spread = rasterx.RepeatSpread
		}
	}
	return
}

// PushState parses the style element, and push it on the style stack. Only color and opacity are supported
// for fill. Note that this parses both the contents of a style attribute plus
// direct fill and opacity attributes.
func (c *IconCursor) PushState(name string, attrs []xml.Attr) error {
	var pairs []string
	className := ""
	for _, attr := range attrs {
		switch strings.ToLower(attr.Name.Local) {
		case "style":
			pairs = append(pairs, strings.Split(attr.Value, ";")...)
		case "class":
			className = attr.Value
		default:
			pairs = append(pairs, attr.Name.Local+":"+attr.Value)
		}
	}
	// Make a copy of the top state
	state := c.states[len(c.states)-1]
	state.text = "" // Text does not pass though to the new item
	state.name = name
	for _, pair := range pairs {
		kv := strings.Split(pair, ":")
		if len(kv) >= 2 {
			k := strings.ToLower(kv[0])
			k = strings.TrimSpace(k)
			v := strings.TrimSpace(kv[1])
			err := c.readStyleAttr(&(state.style), k, v)
			if err != nil {
				return err
			}
		}
	}
	c.adaptClasses(&(state.style), className)
	c.states = append(c.states, state) // Push style onto stack
	return nil
}

func (c *IconCursor) readTransformAttr(m1 rasterx.Matrix2D, k string) (rasterx.Matrix2D, error) {
	ln := len(c.points)
	switch k {
	case "rotate":
		if ln == 1 {
			m1 = m1.Rotate(c.points[0] * math.Pi / 180)
		} else if ln == 3 {
			m1 = m1.Translate(c.points[1], c.points[2]).
				Rotate(c.points[0]*math.Pi/180).
				Translate(-c.points[1], -c.points[2])
		} else {
			return m1, errParamMismatch
		}
	case "translate":
		if ln == 1 {
			m1 = m1.Translate(c.points[0], 0)
		} else if ln == 2 {
			m1 = m1.Translate(c.points[0], c.points[1])
		} else {
			return m1, errParamMismatch
		}
	case "skewx":
		if ln == 1 {
			m1 = m1.SkewX(c.points[0] * math.Pi / 180)
		} else {
			return m1, errParamMismatch
		}
	case "skewy":
		if ln == 1 {
			m1 = m1.SkewY(c.points[0] * math.Pi / 180)
		} else {
			return m1, errParamMismatch
		}
	case "scale":
		if ln == 1 {
			m1 = m1.Scale(c.points[0], 0)
		} else if ln == 2 {
			m1 = m1.Scale(c.points[0], c.points[1])
		} else {
			return m1, errParamMismatch
		}
	case "matrix":
		if ln == 6 {
			m1 = m1.Mult(rasterx.Matrix2D{
				A: c.points[0],
				B: c.points[1],
				C: c.points[2],
				D: c.points[3],
				E: c.points[4],
				F: c.points[5]})
		} else {
			return m1, errParamMismatch
		}
	default:
		return m1, errParamMismatch
	}
	return m1, nil
}

func (c *IconCursor) parseTransform(v string) (rasterx.Matrix2D, error) {
	ts := strings.Split(v, ")")
	m1 := c.states[len(c.states)-1].style.mAdder.M
	for _, t := range ts {
		t = strings.TrimSpace(t)
		if len(t) == 0 {
			continue
		}
		d := strings.Split(t, "(")
		if len(d) != 2 || len(d[1]) < 1 {
			return m1, errParamMismatch // badly formed transformation
		}
		err := c.GetPoints(d[1])
		if err != nil {
			return m1, err
		}
		m1, err = c.readTransformAttr(m1, strings.ToLower(strings.TrimSpace(d[0])))
		if err != nil {
			return m1, err
		}
	}
	return m1, nil
}

func (c *IconCursor) readStyleAttr(curStyle *PathStyle, k, v string) error {
	switch k {
	case "fill":
		gradient, ok := c.ReadGradURL(v, curStyle.fillerColor)
		if ok {
			curStyle.fillerColor = gradient
			break
		}
		var err error
		curStyle.fillerColor, err = ParseSVGColor(v)
		return err
	case "stroke":
		gradient, ok := c.ReadGradURL(v, curStyle.linerColor)
		if ok {
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
		mLimit, err := parseFloat(v, 64)
		if err != nil {
			return err
		}
		curStyle.MiterLimit = mLimit
	case "stroke-width":
		width, err := parseFloat(v, 64)
		if err != nil {
			return err
		}
		curStyle.LineWidth = width
	case "stroke-dashoffset":
		dashOffset, err := parseFloat(v, 64)
		if err != nil {
			return err
		}
		curStyle.DashOffset = dashOffset
	case "stroke-dasharray":
		if v != "none" {
			dList, err := ReadFloatList(v)
			if err != nil {
				return err
			}
			curStyle.Dash = dList
			break
		}
	case "opacity", "stroke-opacity", "fill-opacity":
		op, err := parseFloat(v, 64)
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
		//case "font-family":
		//curStyle.FontFamily = v
		//case "font-kerning":
		//curStyle.FontKerning = true
		// if v == "false" || v == "no" || v == "0" {
		// 	curStyle.FontKerning = false
		// }
		//case "font-size":
		//curStyle.FontSize = v
	}
	return nil
}

func (c *IconCursor) readStartElement(se xml.StartElement) (err error) {
	var skipDef bool
	if se.Name.Local == "radialGradient" || se.Name.Local == "linearGradient" || c.states[len(c.states)-1].inGrad {
		skipDef = true
	}
	if c.states[len(c.states)-1].inDefs && !skipDef {
		ID := ""
		for _, attr := range se.Attr {
			if attr.Name.Local == "id" {
				ID = attr.Value
			}
		}
		if ID != "" && len(c.currentDef) > 0 {
			c.icon.Defs[c.currentDef[0].ID] = c.currentDef
			c.currentDef = make([]definition, 0)
		}
		c.currentDef = append(c.currentDef, definition{
			ID:    ID,
			Tag:   se.Name.Local,
			Attrs: se.Attr,
		})
		return nil
	}
	df, ok := drawFuncs[se.Name.Local]
	if !ok {
		errStr := "Cannot process svg element " + se.Name.Local
		if c.returnError(errStr) {
			return errors.New(errStr)
		}
		return nil
	}
	err = df(c, se.Attr)
	if err != nil {
		e := fmt.Sprintf("error during processing svg element %s: %s", se.Name.Local, err.Error())
		if c.returnError(e) {
			return errors.New(e)
		}
		err = nil
	}

	if len(c.Path) > 0 {
		//The cursor parsed a path from the xml element
		pathCopy := make(rasterx.Path, len(c.Path))
		copy(pathCopy, c.Path)
		c.icon.SVGPaths = append(c.icon.SVGPaths,
			SvgPath{c.states[len(c.states)-1].style, pathCopy})
		c.Path = c.Path[:0]
	}
	return
}

func (c *IconCursor) adaptClasses(pathStyle *PathStyle, className string) {
	if className == "" || len(c.icon.classes) == 0 {
		return
	}
	for k, v := range c.icon.classes[className] {
		c.readStyleAttr(pathStyle, k, v)
	}
}

func (c *IconCursor) returnError(errMsg string) bool {
	if c.ErrorMode == StrictErrorMode {
		return true
	}
	if c.ErrorMode == WarnErrorMode {
		log.Println(errMsg)
	}

	return false
}
