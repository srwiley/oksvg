// Copyright 2018 The oksvg Authors. All rights reserved.
//
// created: 5/12/2018 by S.R.Wiley
package oksvg

import (
	"image/color"
	"math"
	"sort"

	"github.com/srwiley/rasterx"
)

const (
	ObjectBoundingBox GradientUnits = iota
	UserSpaceOnUse
)

const (
	PadSpread SpreadMethod = iota
	ReflectSpread
	RepeatSpread
)

const epsilonF = 1e-5

type (
	SpreadMethod  byte
	GradientUnits byte

	GradStop struct {
		stopColor color.Color
		offset    float64
		opacity   float64
	}

	Gradient struct {
		points   [5]float64
		stops    []*GradStop
		bounds   struct{ X, Y, W, H float64 }
		matrix2D Matrix2D
		spread   SpreadMethod
		units    GradientUnits
		isRadial bool
	}
)

// tColor takes the paramaterized value along the gradient's stops and
// returns a color depending on the spreadMethod value of the gradient and
// the gradient's slice of stop values.
func (g *Gradient) tColor(t, opacity float64) color.Color {
	d := len(g.stops)
	// These cases can be taken care of early on
	if t >= 1.0 && g.spread == PadSpread {

		s := g.stops[d-1]
		//fmt.Println("pad 0 op", s.opacity, opacity, ApplyOpacity(s.stopColor, s.opacity*opacity))
		return ApplyOpacity(s.stopColor, s.opacity*opacity)
	}
	if t <= 0.0 && g.spread == PadSpread {
		return ApplyOpacity(g.stops[0].stopColor, g.stops[0].opacity*opacity)
	}
	var modRange float64 = 1.0
	if g.spread == ReflectSpread {
		modRange = 2.0
	}
	mod := math.Mod(t, modRange)
	if mod < 0 {
		mod += modRange
	}

	place := 0 // Advance to place where mod is greater than the indicated stop
	for place != len(g.stops) && mod > g.stops[place].offset {
		place++
	}
	switch g.spread {
	case RepeatSpread:
		var s1, s2 *GradStop
		switch place {
		case 0, d:
			s1, s2 = g.stops[d-1], g.stops[0]
		default:
			s1, s2 = g.stops[place-1], g.stops[place]
		}
		return g.blendStops(mod, opacity, s1, s2, false)
	case ReflectSpread:
		switch place {
		case 0:
			//fmt.Println("rf 0 op", g.stops[0].opacity*opacity, ApplyOpacity(g.stops[0].stopColor, g.stops[0].opacity*opacity))
			return ApplyOpacity(g.stops[0].stopColor, g.stops[0].opacity*opacity)
		case d:
			// Advance to place where mod-1 is greater than the stop indicated by place in reverseof the stop slice.
			// Since this is the reflect spead mode, the mod interval is two, allowing the stop list to be
			// iterated in reverse before repeating the sequence.
			for place != d*2 && mod-1 > (1-g.stops[d*2-place-1].offset) {
				place++
			}
			switch place {
			case d:
				s := g.stops[d-1]
				return ApplyOpacity(s.stopColor, s.opacity*opacity)
			case d * 2:
				return ApplyOpacity(g.stops[0].stopColor, g.stops[0].opacity*opacity)
			default:
				return g.blendStops(mod-1, opacity,
					g.stops[d*2-place], g.stops[d*2-place-1], true)
			}
		default:
			return g.blendStops(mod, opacity,
				g.stops[place-1], g.stops[place], false)
		}
	default: // PadSpread
		switch place {
		case 0:
			return ApplyOpacity(g.stops[0].stopColor, g.stops[0].opacity*opacity)
		case len(g.stops):
			s := g.stops[len(g.stops)-1]
			return ApplyOpacity(s.stopColor, s.opacity*opacity)
		default:
			return g.blendStops(mod, opacity, g.stops[place-1], g.stops[place], false)
		}
	}
}

func (g *Gradient) blendStops(t, opacity float64, s1, s2 *GradStop, flip bool) color.Color {
	s1off := s1.offset
	if s1.offset > s2.offset && !flip { // happens in repeat spread mode
		s1off -= 1
		if t > 1 {
			t -= 1
		}
	}
	if s2.offset == s1off {
		return ApplyOpacity(s2.stopColor, s2.opacity)
	}
	if flip {
		t = 1 - t
	}
	tp := (t - s1off) / (s2.offset - s1off)
	r1, g1, b1, _ := s1.stopColor.RGBA()
	r2, g2, b2, _ := s2.stopColor.RGBA()

	return ApplyOpacity(color.RGBA{
		uint8((float64(r1)*(1-tp) + float64(r2)*tp) / 256),
		uint8((float64(g1)*(1-tp) + float64(g2)*tp) / 256),
		uint8((float64(b1)*(1-tp) + float64(b2)*tp) / 256),
		0xFF}, (s1.opacity*(1-tp)+s2.opacity*tp)*opacity)
}

func (g *Gradient) GetColorFunction(opacity float64) interface{} {
	switch len(g.stops) {
	case 0:
		return ApplyOpacity(color.RGBA{255, 0, 255, 255}, opacity) // default color for gradient w/o stops.
	case 1:
		return ApplyOpacity(g.stops[0].stopColor, opacity) // Illegal, I think, should really should not happen.
	}

	// sort by offset in ascending order
	sort.Slice(g.stops, func(i, j int) bool {
		return g.stops[i].offset < g.stops[j].offset
	})

	w, h := float64(g.bounds.W), float64(g.bounds.H)
	oriX, oriY := float64(g.bounds.X), float64(g.bounds.Y)
	gradT := Identity.Translate(oriX, oriY).Scale(w, h).
		Mult(g.matrix2D).Scale(1/w, 1/h).Translate(-oriX, -oriY).Invert()

	if g.isRadial {

		cx := g.bounds.X + g.bounds.W*g.points[0]
		cy := g.bounds.Y + g.bounds.H*g.points[1]
		rx := g.bounds.W * g.points[4]
		ry := g.bounds.H * g.points[4]

		if g.points[0] == g.points[2] && g.points[1] == g.points[3] {
			// When the focus and center are the same things are much simpler;
			// t is just distance from center
			// scaled by the bounds aspect ratio times r
			return rasterx.ColorFunc(func(xi, yi int) color.Color {
				x, y := gradT.Transform(float64(xi)+0.5, float64(yi)+0.5)
				dx := float64(x) - cx
				dy := float64(y) - cy
				return g.tColor(math.Sqrt(dx*dx/(rx*rx)+dy*dy/(ry*ry)), opacity)
			})
		} else {
			fx := g.bounds.X + g.bounds.W*g.points[2]
			fy := g.bounds.Y + g.bounds.H*g.points[3]

			//Scale
			fx /= rx
			fy /= ry
			cx /= rx
			cy /= ry

			dfx := fx - cx
			dfy := fy - cy

			if dfx*dfx+dfy*dfy > 1 { // Focus outside of circle; use intersection
				// point of line from center to focus and circle as per SVG specs.
				nfx, nfy, intersects := rasterx.RayCircleIntersectionF(fx, fy, cx, cy, cx, cy, 1.0-epsilonF)
				fx, fy = nfx, nfy
				if intersects == false {
					return color.RGBA{255, 255, 0, 255} // should not happen
				}
			}
			return rasterx.ColorFunc(func(xi, yi int) color.Color {
				x, y := gradT.Transform(float64(xi)+0.5, float64(yi)+0.5)

				ex := float64(x) / rx
				ey := float64(y) / ry

				t1x, t1y, intersects := rasterx.RayCircleIntersectionF(ex, ey, fx, fy, cx, cy, 1.0)
				if intersects == false { //In this case, use the last stop color
					s := g.stops[len(g.stops)-1]
					return ApplyOpacity(s.stopColor, s.opacity*opacity)
				}
				tdx, tdy := t1x-fx, t1y-fy
				dx, dy := ex-fx, ey-fy
				if tdx*tdx+tdy*tdy < epsilonF {
					s := g.stops[len(g.stops)-1]
					return ApplyOpacity(s.stopColor, s.opacity*opacity)
				}
				return g.tColor(math.Sqrt(dx*dx+dy*dy)/math.Sqrt(tdx*tdx+tdy*tdy), opacity)
			})
		}
	} else {

		p1x := g.bounds.X + g.bounds.W*g.points[0]
		p1y := g.bounds.Y + g.bounds.H*g.points[1]
		p2x := g.bounds.X + g.bounds.W*g.points[2]
		p2y := g.bounds.Y + g.bounds.H*g.points[3]

		dx := p2x - p1x
		dy := p2y - p1y
		d := (dx*dx + dy*dy) // self inner prod
		return rasterx.ColorFunc(func(xi, yi int) color.Color {
			x, y := gradT.Transform(float64(xi)+0.5, float64(yi)+0.5)
			dfx := x - p1x
			dfy := y - p1y
			return g.tColor((dx*dfx+dy*dfy)/d, opacity)
		})
	}
}
