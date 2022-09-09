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
		GlyphTable map[string]Glyph
		CodeTable  map[uint16]string
		KernTable  map[string]int16
	}

	Glyph struct {
		Name                    string
		MaxX, MinX, MaxY, MinY  int16
		Width, Lsb, Height, Tsb int16
		Code                    uint16
		Contours                []Contour
		Paths                   []rasterx.Path
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
			}
		}
	}
	return
}

// ParseTTX
func ParseTTX(stream io.Reader) (ttx Ttx, err error) {
	decoder := xml.NewDecoder(stream)
	decoder.CharsetReader = charset.NewReaderLabel
	// These are the parser states used by this state-based design pattern parser
	const (
		inNothing = byte(iota)
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

	for {
		t, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return ttx, err
		}
		switch se := t.(type) {
		case xml.StartElement:
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
					if err != nil {
						return ttx, err
					}
				}
			case inHmtx:
				if se.Name.Local == "mtx" {
					g, err := parseGlyphAttrs(se.Attr)
					if err != nil {
						return ttx, err
					}
					gt := ttx.GlyphTable[g.Name]
					gt.Lsb = g.Lsb
					gt.Width = g.Width
					ttx.GlyphTable[g.Name] = gt
				}
			case inVmtx:
				if se.Name.Local == "mtx" {
					g, err := parseGlyphAttrs(se.Attr)
					if err != nil {
						return ttx, err
					}
					gt := ttx.GlyphTable[g.Name]
					gt.Tsb = g.Tsb
					gt.Height = g.Height
					ttx.GlyphTable[g.Name] = gt
				}
			case inGlyph:
				if strings.ToLower(se.Name.Local) == "contour" {
					state = inContour
					contour = []CPoint{} // make a new slice
				}
				if strings.ToLower(se.Name.Local) == "component" {
					var c component
					for _, a := range se.Attr {
						switch strings.ToLower(a.Name.Local) {
						case "x":
							i, err := strconv.Atoi(a.Value)
							c.x = int16(i)
							if err != nil {
								return ttx, err
							}
						case "y":
							i, err := strconv.Atoi(a.Value)
							c.y = int16(i)
							if err != nil {
								return ttx, err
							}
						case "glyphname":
							c.name = a.Value
						}
					}
					components[g.Name] = append(components[g.Name], c)
				}
			case inContour:
				if strings.ToLower(se.Name.Local) == "pt" {
					var p CPoint
					for _, a := range se.Attr {
						switch strings.ToLower(a.Name.Local) {
						case "on":
							p.On = a.Value == "1"
						case "x":
							i, err := strconv.Atoi(a.Value)
							p.X = int16(i)
							if err != nil {
								return ttx, err
							}
						case "y":
							i, err := strconv.Atoi(a.Value)
							p.Y = int16(i)
							if err != nil {
								return ttx, err
							}
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
							i, err := strconv.ParseInt(a.Value[2:], 16, 32)
							code = uint16(i)
							if err != nil {
								return ttx, err
							}
						case "name":
							name = a.Value
						}
					}
					ttx.CodeTable[code] = name
				}
			}
		case xml.EndElement:
			switch state {
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
					g.Paths = append(g.Paths, contour.contourToPath(0, 0))
				}
			}
		case xml.CharData: // Actually not used for this format

		}
	}
	// Create the compound characters
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
		g.Paths = append(g.Paths, ct.contourToPath(0, 0))
	}
}

func (g *Glyph) SaveAsSVG(filename string) (err error) {
	fout, errf := os.Create(filename)
	if errf != nil {
		return errf
	}
	defer fout.Close()
	fmt.Fprintf(fout, "<svg width=\"8cm\" height=\"10cm\" viewBox=\"%d %d %d %d\" ",
		g.MinX, -g.MaxY, g.MaxX-g.MinX, g.MaxY-g.MinY)
	fmt.Fprintf(fout, "xmlns=\"http://www.w3.org/2000/svg\" version=\"1.1\">\n")
	fmt.Fprintf(fout, "<title>ttx letter %s</title>", g.Name)
	fmt.Fprintf(fout, "<path d=\"")
	for _, p := range g.Paths {
		fmt.Fprintf(fout, "%s", p.ToSVGPath())
	}
	fmt.Fprintf(fout, "\" fill = \"black\"  />\n</svg>\n")
	return
}

// contourToPath returns a path represenation
// of the contour.
func (c Contour) contourToPath(x, y int16) (p rasterx.Path) {
	if len(c) == 0 {
		return
	}
	lastPt := c[0]
	lastPt.X += x
	lastPt.Y += y
	// if !c[0].On {
	// 	fmt.Println("Off curve first point")
	// }
	p.Start(fixed.Point26_6{X: fixed.Int26_6(lastPt.X) * 64,
		Y: fixed.Int26_6(-lastPt.Y) * 64})
	for i := 1; i < len(c); i++ {
		pt := c[i]
		pt.X += x
		pt.Y += y
		if pt.On {
			if lastPt.On {
				p.Line(fixed.Point26_6{X: fixed.Int26_6(pt.X) * 64,
					Y: fixed.Int26_6(-pt.Y) * 64})
			} else {
				p.QuadBezier(fixed.Point26_6{X: fixed.Int26_6(lastPt.X) * 64,
					Y: fixed.Int26_6(-lastPt.Y) * 64},
					fixed.Point26_6{X: fixed.Int26_6(pt.X) * 64,
						Y: fixed.Int26_6(-pt.Y) * 64})
			}
		} else {
			if !lastPt.On {
				midPt := CPoint{(pt.X + lastPt.X) / 2, (pt.Y + lastPt.Y) / 2, true}
				p.QuadBezier(fixed.Point26_6{X: fixed.Int26_6(lastPt.X) * 64,
					Y: fixed.Int26_6(-lastPt.Y) * 64},
					fixed.Point26_6{X: fixed.Int26_6(midPt.X) * 64,
						Y: fixed.Int26_6(-midPt.Y) * 64})
			}
		}
		lastPt = pt
	}
	if !lastPt.On {
		p.QuadBezier(fixed.Point26_6{X: fixed.Int26_6(lastPt.X) * 64,
			Y: fixed.Int26_6(-lastPt.Y) * 64},
			fixed.Point26_6{X: fixed.Int26_6(c[0].X+x) * 64,
				Y: fixed.Int26_6(-c[0].Y+y) * 64})
	} else {
		p.Line(fixed.Point26_6{X: fixed.Int26_6(c[0].X+x) * 64,
			Y: fixed.Int26_6(-c[0].Y+y) * 64})
	}
	p.Stop(true)
	return
}
