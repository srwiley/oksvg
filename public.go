// Copyright 2017 The oksvg Authors. All rights reserved.
// created: 2/12/2017 by S.R.Wiley
//
// utils.go implements translation of an SVG2.0 path into a rasterx Path.

package oksvg

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"image/color"
	"io"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/srwiley/rasterx"
	"golang.org/x/image/colornames"
	"golang.org/x/net/html/charset"
)

// ReadIconStream reads the Icon from the given io.Reader.
// This only supports a sub-set of SVG, but
// is enough to draw many icons. If errMode is provided,
// the first value determines if the icon ignores, errors out, or logs a warning
// if it does not handle an element found in the icon file. Ignore warnings is
// the default if no ErrorMode value is provided.
func ReadIconStream(stream io.Reader, errMode ...ErrorMode) (*SvgIcon, error) {
	cursor := NewIconCursor()
	return cursor.ReadIconStream(stream, errMode...)
}

// NewIconCursor returns an IconCursor with the default style and no icon set.
func NewIconCursor() *IconCursor {
	icon := &SvgIcon{Defs: make(map[string][]definition), Grads: make(map[string]*rasterx.Gradient), Transform: rasterx.Identity}
	return &IconCursor{states: []state{{style: DefaultStyle}}, icon: icon}
}

// SetFonts adds a set of fonts.
// Without fonts, text elements are not rendered.
func (c *IconCursor) SetFonts(fonts []Ttx) {
	c.fonts = fonts
}

// ReadIconStream reads the Icon from the stream using the cursor
func (cursor *IconCursor) ReadIconStream(stream io.Reader, errMode ...ErrorMode) (*SvgIcon, error) {
	icon := cursor.icon
	if len(errMode) > 0 {
		cursor.ErrorMode = errMode[0]
	}
	decoder := xml.NewDecoder(stream)
	decoder.CharsetReader = charset.NewReaderLabel
	for {
		t, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return icon, err
		}
		state := cursor.states[len(cursor.states)-1]
		// Inspect the type of the XML token
		switch se := t.(type) {
		case xml.StartElement:
			// Reads all recognized style attributes from the start element
			// and places it on top of the styleStack
			err = cursor.PushState(se.Name.Local, se.Attr)
			if err != nil {
				return icon, err
			}
			err = cursor.readStartElement(se)
			if err != nil {
				return icon, err
			}
			// if se.Name.Local == "style" && state.inDefs {
			// 	cursor.states[len(cursor.states)-1].inDefsStyle = true
			// }
		case xml.EndElement:
			switch se.Name.Local {
			case "g":
				if state.inDefs {
					cursor.currentDef = append(cursor.currentDef, definition{
						Tag: "endg",
					})
				}
			case "title":
				icon.Titles = append(icon.Titles, state.text)
			case "desc":
				//cursor.states[len(cursor.states)-1].inDescText = false
				icon.Descriptions = append(icon.Descriptions, state.text)
			case "style":
				//if state.inStyle {
				icon.classes, err = parseClasses(state.text)
				if err != nil {
					return icon, err
				}
				//}
			case "text":
				//fmt.Printf("Read Text:'%s %d'\n", cursor.text, len(cursor.fonts))
				err := cursor.writeText()
				if err != nil {
					return icon, err
				}
			case "defs":
				if len(cursor.currentDef) > 0 {
					cursor.icon.Defs[cursor.currentDef[0].ID] = cursor.currentDef
					cursor.currentDef = make([]definition, 0)
				}
				//case "radialGradient", "linearGradient":
				//cursor.states[len(cursor.states)-1].inGrad = false
			}
			// pop state
			cursor.states = cursor.states[:len(cursor.states)-1]
		case xml.CharData:
			cursor.states[len(cursor.states)-1].text += string(se)
			// switch {
			// case state.inTitleText:
			// 	fallthrough
			// case state.inText:
			// 	fallthrough
			// case state.inDescText:
			// 	fallthrough
			// case state.inDefsStyle:
			// }
			// if cursor.states[len(cursor.states)-1].inTitleText {
			// 	icon.Titles[len(icon.Titles)-1] += string(se)
			// }
			// if cursor.states[len(cursor.states)-1].inText {
			// 	cursor.text += string(se)
			// }
			// if cursor.states[len(cursor.states)-1].inDescText {
			// 	icon.Descriptions[len(icon.Descriptions)-1] += string(se)
			// }
			// if cursor.states[len(cursor.states)-1].inDefsStyle {
			// 	classInfo = string(se)
			// }
		}
	}
	return icon, nil
}

// ReadReplacingCurrentColor replaces currentColor value with specified value and loads SvgIcon as ReadIconStream do.
// currentColor value should be valid hex, rgb or named color value.
func ReadReplacingCurrentColor(stream io.Reader, currentColor string, errMode ...ErrorMode) (icon *SvgIcon, err error) {
	var (
		data []byte
	)

	if data, err = io.ReadAll(stream); err != nil {
		return nil, fmt.Errorf("%w: read data: %v", errParamMismatch, err)
	}

	if currentColor != "" && strings.Contains(string(data), "currentColor") {
		data = []byte(strings.ReplaceAll(string(data), "currentColor", currentColor))
	}

	if icon, err = ReadIconStream(bytes.NewBuffer(data), errMode...); err != nil {
		return nil, fmt.Errorf("%w: load: %v", errParamMismatch, err)
	}

	return icon, nil
}

// ReadIcon reads the Icon from the named file.
// This only supports a sub-set of SVG, but is enough to draw many icons.
// If errMode is provided, the first value determines if the icon ignores, errors out, or logs a warning
// if it does not handle an element found in the icon file.
// Ignore warnings is the default if no ErrorMode value is provided.
// func ReadIcon(iconFile string, errMode ...ErrorMode) (*SvgIcon, error) {
// 	return ReadIconF(iconFile, nil, errMode...)
// }

// ReadIcon reads the Icon from the named file.
// This only supports a sub-set of SVG, but is enough to draw many icons.
// If errMode is provided, the first value determines if the icon ignores, errors out, or logs a warning
// if it does not handle an element found in the icon file.
// Ignore warnings is the default if no ErrorMode value is provided.
func ReadIcon(iconFile string, errMode ...ErrorMode) (*SvgIcon, error) {
	fin, errf := os.Open(iconFile)
	if errf != nil {
		return nil, errf
	}
	defer fin.Close()
	return ReadIconStream(fin, errMode...)
}

// ParseSVGColor parses an SVG color string in all forms
// including all SVG1.1 names, obtained from the image.colornames package
func ParseSVGColor(colorStr string) (color.Color, error) {

	type parseColorFunc func(colorStr string) (color color.Color, done bool, oops error)

	var ( //type parseColorFunc functions that parse different color string formats

		readNone parseColorFunc = func(colorStr string) (color.Color, bool, error) {
			if colorStr == "none" || colorStr == "" {
				// nil color signals that the function (fill or stroke) is off;
				// not the same as black or zero alpha
				return nil, true, nil
			}
			return nil, false, nil
		}

		readNamed parseColorFunc = func(colorStr string) (color.Color, bool, error) {
			cn, ok := colornames.Map[colorStr]
			if ok {
				r, g, b, a := cn.RGBA()
				return color.NRGBA{uint8(r), uint8(g), uint8(b), uint8(a)}, true, nil
			}
			return nil, false, nil
		}

		readHexNum parseColorFunc = func(colorStr string) (color.Color, bool, error) {
			if colorStr[0] != '#' {
				return nil, false, nil
			}
			colorStr = colorStr[1:]
			if len(colorStr) != 6 {
				if len(colorStr) != 3 {
					err := fmt.Errorf("color string %s is not length 3 or 6 as required by SVG specification",
						colorStr)
					return nil, true, err
				}
				// SVG specs say duplicate characters in case of 3 digit hex number
				colorStr = string([]byte{colorStr[0], colorStr[0],
					colorStr[1], colorStr[1], colorStr[2], colorStr[2]})
			}
			puint := func(s string, v *uint8) error {
				t, err := strconv.ParseUint(s, 16, 8)
				*v = uint8(t)
				return err
			}
			var r, g, b uint8
			for _, v := range [3]func() error{
				func() error { return puint(colorStr[0:2], &r) },
				func() error { return puint(colorStr[2:4], &g) },
				func() error { return puint(colorStr[4:6], &b) },
			} {
				if err := v(); err != nil {
					return nil, true, err
				}
			}
			return color.NRGBA{r, g, b, 0xFF}, true, nil
		}

		readRGB parseColorFunc = func(colorStr string) (color.Color, bool, error) {
			cStr := strings.TrimPrefix(colorStr, "rgba(")
			hasAlpha := cStr != colorStr
			if !hasAlpha {
				cStr = strings.TrimPrefix(colorStr, "rgb(")
				if cStr == colorStr {
					return nil, false, nil
				}
			}
			cStr = strings.TrimSuffix(cStr, ")")
			vals := strings.Split(cStr, ",")
			want := 3
			if hasAlpha {
				want = 4
			}
			if len(vals) != want {
				return color.NRGBA{}, true, errParamMismatch
			}
			var cvals [3]uint8
			var err error
			for i := range cvals {
				cvals[i], err = parseColorValue(vals[i])
				if err != nil {
					return nil, true, err
				}
			}
			alpha := uint8(0xFF)
			if hasAlpha {
				alpha, err = parseColorAlpha(vals[3])
				if err != nil {
					return nil, true, err
				}
			}
			return color.NRGBA{cvals[0], cvals[1], cvals[2], alpha}, true, nil
		}

		readHSL parseColorFunc = func(colorStr string) (color.Color, bool, error) {
			cStr := strings.TrimPrefix(colorStr, "hsl(")
			if cStr == colorStr {
				return nil, false, nil
			}
			cStr = strings.TrimSuffix(cStr, ")")
			vals := strings.Split(cStr, ",")
			if len(vals) != 3 {
				return color.NRGBA{}, true, errParamMismatch
			}
			H, err := strconv.ParseInt(strings.TrimSpace(vals[0]), 10, 64)
			if err != nil {
				return color.NRGBA{}, true, fmt.Errorf("invalid hue in hsl: '%s' (%s)", vals[0], err)
			}

			S, err := strconv.ParseFloat(strings.TrimSpace(vals[1][:len(vals[1])-1]), 64)
			if err != nil {
				return color.NRGBA{}, true, fmt.Errorf("invalid saturation in hsl: '%s' (%s)", vals[1], err)
			}
			S = S / 100

			L, err := strconv.ParseFloat(strings.TrimSpace(vals[2][:len(vals[2])-1]), 64)
			if err != nil {
				return color.NRGBA{}, true, fmt.Errorf("invalid lightness in hsl: '%s' (%s)", vals[2], err)
			}
			L = L / 100

			C := (1 - math.Abs((2*L)-1)) * S
			X := C * (1 - math.Abs(math.Mod((float64(H)/60), 2)-1))
			m := L - C/2

			var rp, gp, bp float64
			[6]func(){
				func() { rp, gp, bp = float64(C), float64(X), float64(0) },
				func() { rp, gp, bp = float64(X), float64(C), float64(0) },
				func() { rp, gp, bp = float64(0), float64(C), float64(X) },
				func() { rp, gp, bp = float64(0), float64(X), float64(C) },
				func() { rp, gp, bp = float64(X), float64(0), float64(C) },
				func() { rp, gp, bp = float64(C), float64(0), float64(X) },
			}[(H%360)/60]()

			mround := func(v float64) uint8 {
				r := math.Round((v + m) * 255)
				if r > 255 {
					r = 255
				}
				return uint8(r)
			}
			return color.NRGBA{mround(rp), mround(gp), mround(bp), 0xFF}, true, nil
		}
	)

	parseBinder := func(input string, funcs ...parseColorFunc) (color.Color, error) {
		for _, f := range funcs {
			col, ok, err := f(input)
			if ok { // the function recognized the color string format
				// and either parsed it successfully or errored out.
				return col, err
			}
		}
		// Nothing parsed, format of color was not detected
		return nil, fmt.Errorf("cannot read color string %s", input)
	}

	// try to detect the colorStr format
	// with the following parsing functions in order
	return parseBinder(strings.ToLower(colorStr),
		readNone,
		readHexNum,
		readNamed,
		readRGB,
		readHSL,
	)
}

// ParseSVGColor parses an SVG color string in all forms
// including all SVG1.1 names, obtained from the image.colornames package
// func ParseSVGColorO(colorStr string) (color.Color, error) {
// 	// _, _, _, a := curColor.RGBA()
// 	v := strings.ToLower(colorStr)
// 	if strings.HasPrefix(v, "url") { // We are not handling urls
// 		// and gradients and stuff at this point
// 		return color.NRGBA{0, 0, 0, 255}, nil
// 	}
// 	switch v {
// 	case "none", "":
// 		// nil signals that the function (fill or stroke) is off;
// 		// not the same as black
// 		return nil, nil
// 	default:
// 		cn, ok := colornames.Map[v]
// 		if ok {
// 			r, g, b, a := cn.RGBA()
// 			return color.NRGBA{uint8(r), uint8(g), uint8(b), uint8(a)}, nil
// 		}
// 	}
// 	cStr := strings.TrimPrefix(colorStr, "rgb(")
// 	if cStr != colorStr {
// 		cStr := strings.TrimSuffix(cStr, ")")
// 		vals := strings.Split(cStr, ",")
// 		if len(vals) != 3 {
// 			return color.NRGBA{}, errParamMismatch
// 		}
// 		var cvals [3]uint8
// 		var err error
// 		for i := range cvals {
// 			cvals[i], err = parseColorValue(vals[i])
// 			if err != nil {
// 				return nil, err
// 			}
// 		}
// 		return color.NRGBA{cvals[0], cvals[1], cvals[2], 0xFF}, nil
// 	}

// 	cStr = strings.TrimPrefix(colorStr, "hsl(")
// 	if cStr != colorStr {
// 		cStr := strings.TrimSuffix(cStr, ")")
// 		vals := strings.Split(cStr, ",")
// 		if len(vals) != 3 {
// 			return color.NRGBA{}, errParamMismatch
// 		}

// 		H, err := strconv.ParseInt(strings.TrimSpace(vals[0]), 10, 64)
// 		if err != nil {
// 			return color.NRGBA{}, fmt.Errorf("invalid hue in hsl: '%s' (%s)", vals[0], err)
// 		}

// 		S, err := strconv.ParseFloat(strings.TrimSpace(vals[1][:len(vals[1])-1]), 64)
// 		if err != nil {
// 			return color.NRGBA{}, fmt.Errorf("invalid saturation in hsl: '%s' (%s)", vals[1], err)
// 		}
// 		S = S / 100

// 		L, err := strconv.ParseFloat(strings.TrimSpace(vals[2][:len(vals[2])-1]), 64)
// 		if err != nil {
// 			return color.NRGBA{}, fmt.Errorf("invalid lightness in hsl: '%s' (%s)", vals[2], err)
// 		}
// 		L = L / 100

// 		C := (1 - math.Abs((2*L)-1)) * S
// 		X := C * (1 - math.Abs(math.Mod((float64(H)/60), 2)-1))
// 		m := L - C/2

// 		var rp, gp, bp float64
// 		if H < 60 {
// 			rp, gp, bp = float64(C), float64(X), float64(0)
// 		} else if H < 120 {
// 			rp, gp, bp = float64(X), float64(C), float64(0)
// 		} else if H < 180 {
// 			rp, gp, bp = float64(0), float64(C), float64(X)
// 		} else if H < 240 {
// 			rp, gp, bp = float64(0), float64(X), float64(C)
// 		} else if H < 300 {
// 			rp, gp, bp = float64(X), float64(0), float64(C)
// 		} else {
// 			rp, gp, bp = float64(C), float64(0), float64(X)
// 		}

// 		r, g, b := math.Round((rp+m)*255), math.Round((gp+m)*255), math.Round((bp+m)*255)
// 		if r > 255 {
// 			r = 255
// 		}
// 		if g > 255 {
// 			g = 255
// 		}
// 		if b > 255 {
// 			b = 255
// 		}

// 		return color.NRGBA{
// 			uint8(r),
// 			uint8(g),
// 			uint8(b),
// 			0xFF,
// 		}, nil
// 	}

// 	if colorStr[0] == '#' {
// 		r, g, b, err := ParseSVGColorNum(colorStr)
// 		if err != nil {
// 			return nil, err
// 		}
// 		return color.NRGBA{r, g, b, 0xFF}, nil
// 	}
// 	return nil, errParamMismatch
// }
