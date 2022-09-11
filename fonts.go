package oksvg

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/srwiley/rasterx"
	"golang.org/x/image/math/fixed"
	"golang.org/x/net/html/charset"
)

type (
	Ttx struct {
		Name                   string
		Created, Modified      string
		MaxX, MinX, MaxY, MinY int16
		UnitsPerEm             int16
		GlyphTable             map[string]Glyph
		CodeTable              map[uint16]string
		KernTable              map[string]int16
	}

	Glyph struct {
		Name                    string
		MaxX, MinX, MaxY, MinY  int16
		Width, Lsb, Height, Tsb int16
		Code                    uint16
		Contours                []Contour
	}

	Contour []CPoint

	CPoint struct {
		X, Y int16
		On   bool
	}

	component struct {
		name string
		x, y int16
	}
)

// func SetInt16(id string, attr []xml.Attr, t *int16) (e error) {
// 	i, err := strconv.Atoi(GetAttr(id, attr))
// 	if err != nil {
// 		return err
// 	}
// 	*t = int16(i)
// 	return
// }

//}

// ParseTTX
func ParseTTX(stream io.Reader) (ttx Ttx, err error) {
	decoder := xml.NewDecoder(stream)
	decoder.CharsetReader = charset.NewReaderLabel
	// These are the parser states used by this state-based design pattern parser
	const (
		inNothing = byte(iota)
		inHead
		inHmtx
		inVmtx
		inGlyph
		inContour
		inCMap
		inKern
	)
	state := inNothing

	ttx.GlyphTable = make(map[string]Glyph)
	ttx.CodeTable = make(map[uint16]string)
	ttx.KernTable = make(map[string]int16)
	// local parsing structs
	var g Glyph
	var contour Contour
	var components = make(map[string][]component)

	getAttrT := func(tag string, attr []xml.Attr) (v string) {
		for _, a := range attr {
			if a.Name.Local == "value" {
				return a.Value
			}
		}
		return
	}

	AtoI16 := func(v string) (n int16, err error) {
		var i int
		i, err = strconv.Atoi(v)
		if err != nil {
			return
		}
		n = int16(i)
		return
	}

	setInt16T := func(tag string, attr []xml.Attr) (v int16, err error) {
		return AtoI16(getAttrT(tag, attr))
	}

	setInt16 := func(attr []xml.Attr) (v int16, err error) {
		return setInt16T("value", attr)
	}

	for {
		var t xml.Token
		t, err = decoder.Token()
		if err != nil {
			if err == io.EOF {
				err = nil
				break
			}
			return ttx, err
		}
		switch se := t.(type) {
		case xml.StartElement:
			var i int
			switch state {
			case inNothing:
				switch se.Name.Local {
				case "hmtx":
					state = inHmtx
				case "vmtx":
					state = inVmtx
				case "cmap_format_4":
					state = inCMap
				case "kern":
					state = inKern
				case "TTGlyph":
					state = inGlyph
					g, err = parseGlyphAttrs(se.Attr)
				case "head":
					state = inHead
				}
			case inHead:
				switch se.Name.Local {
				case "created":
					ttx.Created = getAttrT("value", se.Attr)
				case "modified":
					ttx.Modified = getAttrT("value", se.Attr)
				case "unitsPerEm":
					ttx.UnitsPerEm, err = setInt16(se.Attr)
				case "xMin":
					ttx.MinX, err = setInt16(se.Attr)
				case "xMax":
					ttx.MaxX, err = setInt16(se.Attr)
				case "yMax":
					ttx.MaxY, err = setInt16(se.Attr)
				case "yMin":
					ttx.MinY, err = setInt16(se.Attr)
				}
			case inHmtx:
				if se.Name.Local == "mtx" {
					g, err = parseGlyphAttrs(se.Attr)
					gt := ttx.GlyphTable[g.Name]
					gt.Lsb = g.Lsb
					gt.Width = g.Width
					ttx.GlyphTable[g.Name] = gt
				}
			case inVmtx:
				if se.Name.Local == "mtx" {
					g, err = parseGlyphAttrs(se.Attr)
					gt := ttx.GlyphTable[g.Name]
					gt.Tsb = g.Tsb
					gt.Height = g.Height
					ttx.GlyphTable[g.Name] = gt
				}
			case inGlyph:
				if strings.ToLower(se.Name.Local) == "contour" {
					state = inContour
					contour = []CPoint{} // make a new slice
				} else {
					if strings.ToLower(se.Name.Local) == "component" {
						var c component
						for _, a := range se.Attr {
							switch strings.ToLower(a.Name.Local) {
							case "x":
								c.x, err = AtoI16(a.Value)
							case "y":
								c.y, err = AtoI16(a.Value)
							case "glyphname":
								c.name = a.Value
							}
						}
						components[g.Name] = append(components[g.Name], c)
					}
				}
			case inContour:
				if strings.ToLower(se.Name.Local) == "pt" {
					var p CPoint
					for _, a := range se.Attr {
						switch strings.ToLower(a.Name.Local) {
						case "on":
							p.On = a.Value == "1"
						case "x":
							i, err = strconv.Atoi(a.Value)
							p.X = int16(i)
						case "y":
							i, err = strconv.Atoi(a.Value)
							p.Y = int16(i)
						}
					}
					contour = append(contour, p)
				}
			case inKern:
				if strings.ToLower(se.Name.Local) == "pair" {
					ttx.AddPair(se.Attr)
				}
			case inCMap:
				if strings.ToLower(se.Name.Local) == "map" {
					var code uint16
					var name string
					for _, a := range se.Attr {
						switch strings.ToLower(a.Name.Local) {
						case "code":
							var i int64
							i, err = strconv.ParseInt(a.Value[2:], 16, 32)
							code = uint16(i)
						case "name":
							name = a.Value
						}
					}
					ttx.CodeTable[code] = name
				}
			}
		case xml.EndElement:
			switch state {
			case inHead:
				if se.Name.Local == "head" {
					state = inNothing
				}
			case inHmtx:
				if se.Name.Local == "hmtx" {
					state = inNothing
				}
			case inKern:
				if se.Name.Local == "kern" {
					state = inNothing
				}
			case inVmtx:
				if se.Name.Local == "vmtx" {
					state = inNothing
				}
			case inCMap:
				if se.Name.Local == "cmap_format_4" {
					state = inNothing
				}
			case inGlyph:
				if se.Name.Local == "TTGlyph" {
					state = inNothing
					gt := ttx.GlyphTable[g.Name]
					g.Lsb = gt.Lsb
					g.Width = gt.Width
					g.Tsb = gt.Tsb
					g.Height = gt.Height
					ttx.GlyphTable[g.Name] = g
				}
			case inContour:
				if se.Name.Local == "contour" {
					state = inGlyph
					g.Contours = append(g.Contours, contour)
				}
			}
			// case xml.CharData: // Actually not used for this format
		}
		// Error generated by the last xml element?
		if err != nil {
			return ttx, err
		}
	}

	// Initialize the compound glyphs
	// by compiling the contours in the
	// glyphs referenced by the components
	// into the target glyph.
	for k, v := range components {
		g, ok := ttx.GlyphTable[k]
		if !ok {
			continue
		}
		for _, c := range v {
			gc, ok := ttx.GlyphTable[c.name]
			if !ok {
				continue
			}
			g.addComponents(gc, c.x, c.y)
		}
		ttx.GlyphTable[k] = g
	}

	// Add the unicodes to the glyphs records
	for k, v := range ttx.CodeTable {
		g, ok := ttx.GlyphTable[v]
		if !ok {
			continue
		}
		g.Code = k
		ttx.GlyphTable[v] = g
	}
	// Remove .notdefs
	for k := range ttx.GlyphTable {
		if len(k) >= 7 && k[:7] == ".notdef" {
			delete(ttx.GlyphTable, k)
		}
	}
	return
}

func parseGlyphAttrs(attrs []xml.Attr) (g Glyph, e error) {
	for _, a := range attrs {
		str := strings.ToLower(a.Name.Local)
		if str == "name" {
			g.Name = a.Value
			continue
		}
		for _, v := range [8]struct {
			name string
			targ *int16
		}{
			{"xmin", &g.MinX}, {"ymin", &g.MinY},
			{"xmax", &g.MaxX}, {"ymax", &g.MaxY},
			{"width", &g.Width}, {"lsb", &g.Lsb},
			{"height", &g.Height}, {"tsb", &g.Tsb},
		} {
			if v.name == str {
				i, err := strconv.Atoi(a.Value)
				if err != nil {
					return g, err
				}
				*v.targ = int16(i)
				break
			}
		}
	}
	return
}

func (ttx *Ttx) AddPair(attr []xml.Attr) (err error) {
	var l, r string
	var v int
	for _, a := range attr {
		switch a.Name.Local {
		case "l":
			l = a.Value
		case "r":
			r = a.Value
		case "v":
			v, err = strconv.Atoi(a.Value)
			if err != nil {
				return err
			}
		}
	}
	ttx.KernTable[l+":"+r] = int16(v)
	return
}

func (g *Glyph) addComponents(gc Glyph, x, y int16) {
	for _, c := range gc.Contours {
		ct := Contour{}
		for _, p := range c {
			ct = append(ct, CPoint{p.X + x, p.Y + y, p.On})
		}
		g.Contours = append(g.Contours, ct)
	}
}

func (g *Glyph) SaveAsSVG(filename string) (err error) {
	fout, errf := os.Create(filename)
	if errf != nil {
		return errf
	}
	defer fout.Close()
	fmt.Fprintf(fout, "<svg width=\"2cm\" height=\"2cm\" viewBox=\"%d %d %d %d\" ",
		g.MinX, -g.MaxY, g.MaxX-g.MinX, g.MaxY-g.MinY)
	fmt.Fprintf(fout, "xmlns=\"http://www.w3.org/2000/svg\" version=\"1.1\">\n")
	fmt.Fprintf(fout, "<title>ttx letter %s</title>", g.Name)
	fmt.Fprintf(fout, "<path d=\"")
	for _, p := range g.Contours {
		fmt.Fprintf(fout, "%s", p.ContourToPath(fixed.Point26_6{X: 0, Y: 0}, 8).ToSVGPath())
	}
	fmt.Fprintf(fout, "\" fill = \"black\"  />\n</svg>\n")
	return
}

func SaveAsSVG(filename string, p rasterx.Path) (err error) {
	fout, errf := os.Create(filename)
	if errf != nil {
		return errf
	}
	defer fout.Close()
	fmt.Fprintf(fout, "<svg width=\"15cm\" height=\"8cm\" viewBox=\"%d %d %d %d\" ",
		//g.MinX, -g.MaxY, g.MaxX-g.MinX, g.MaxY-g.MinY)
		-400, -700, 8000, 1000)
	fmt.Fprintf(fout, "xmlns=\"http://www.w3.org/2000/svg\" version=\"1.1\">\n")
	fmt.Fprintf(fout, "<title>ttx letter %s</title>", "hello")
	fmt.Fprintf(fout, "<circle cx=\"0\" cy=\"0\" r=\"35\" fill = \"red\"/>\n")
	fmt.Fprintf(fout, "<path d=\"")
	//for _, pth := range p {
	fmt.Fprintf(fout, "%s", p.ToSVGPath())
	//}
	fmt.Fprintf(fout, "\" fill = \"black\"  />\n</svg>\n")
	return
}

func (ttx *Ttx) GlyphForRune(tk rune) (g Glyph, ok bool) {
	var name string
	name, ok = ttx.CodeTable[uint16(tk)]
	if !ok {
		return
	}
	g, ok = ttx.GlyphTable[name]
	return
}

func (ttx *Ttx) StringToPaths(text string, origin fixed.Point26_6, useKerning bool) (p rasterx.Path, advance, consumed int) {
	//px, py := int16(0), int16(0)
	//adv := int16(0)
	var lastName string
	for _, tk := range text {
		g, ok := ttx.GlyphForRune(tk)
		if !ok {
			continue
		}
		if len(lastName) > 0 && useKerning {
			kern, ok := ttx.KernTable[lastName+":"+g.Name]
			if ok {
				origin.X += fixed.Int26_6(kern) * 64
			}
		}
		consumed++
		for _, c := range g.Contours {
			p = append(p, c.ContourToPath(origin, 8)...)
		}
		origin.X += fixed.Int26_6(g.Width) * 64
		lastName = g.Name
	}
	advance = int(origin.X / 64)
	return
}

// ContourToPath returns a path represenation of the contour.
func (c Contour) ContourToPath(ori fixed.Point26_6, scale int) (p rasterx.Path) {

	toFixedP := func(x, y int16) (p fixed.Point26_6) {
		p = fixed.Point26_6{
			X: fixed.Int26_6(int(x)*scale) + ori.X,
			Y: fixed.Int26_6(int(y)*scale) + ori.Y}
		return
	}
	if len(c) <= 1 {
		return // Single point doth not make a glyph
	}
	lastPt := c[0]
	//lastPt.X += x
	//lastPt.Y += y
	// if !c[0].On {
	// 	fmt.Println("Off curve first point")
	// }
	p.Start(toFixedP(lastPt.X, -lastPt.Y))
	for i := 1; i < len(c); i++ {
		pt := c[i]
		if pt.On {
			if lastPt.On {
				p.Line(toFixedP(pt.X, -pt.Y))
			} else {
				p.QuadBezier(
					toFixedP(lastPt.X, -lastPt.Y),
					toFixedP(pt.X, -pt.Y))
			}
		} else {
			if !lastPt.On {
				p.QuadBezier(
					toFixedP(lastPt.X, -lastPt.Y),
					toFixedP((pt.X+lastPt.X)/2, -(pt.Y+lastPt.Y)/2))
			}
		}
		lastPt = pt
	}
	if !lastPt.On {
		p.QuadBezier(
			toFixedP(lastPt.X, -lastPt.Y),
			toFixedP(c[0].X, -c[0].Y))
	} else {
		p.Line(toFixedP(c[0].X, -c[0].Y))
	}
	p.Stop(true)
	return
}
