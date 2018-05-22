// Copyright 2017 The oksvg Authors. All rights reserved.
// created: 2/12/2017 by S.R.Wiley

// svgd.go implements translation of an SVG2.0 path into a rasterx Path.

package oksvg

import (
	"errors"
	"log"
	"math"
	"strconv"
	"unicode"

	"github.com/srwiley/rasterx"

	"golang.org/x/image/math/fixed"
)

type (
	ErrorMode  uint8
	PathCursor struct {
		rasterx.Path
		placeX, placeY         float64
		cntlPtX, cntlPtY       float64
		pathStartX, pathStartY fixed.Int26_6
		points                 []float64
		lastKey                uint8
		ErrorMode              ErrorMode
		inPath                 bool
	}
)

var (
	paramMismatchError  = errors.New("Param mismatch")
	commandUnknownError = errors.New("Unknown command")
	zeroLengthIdError   = errors.New("zero length id")
	missingIdError      = errors.New("cannot find id")
)

// MaxDx is the Maximum radians a cubic splice is allowed to span
// in ellipse parametric when approximating an off-axis ellipse.
const MaxDx float64 = math.Pi / 8

const (
	IgnoreErrorMode ErrorMode = iota
	WarnErrorMode
	StrictErrorMode
)

func reflect(px, py, rx, ry float64) (x, y float64) {
	return px*2 - rx, py*2 - ry
}

func (c *PathCursor) valsToAbs(last float64) {
	for i := 0; i < len(c.points); i++ {
		last += c.points[i]
		c.points[i] = last
	}
}

func (c *PathCursor) pointsToAbs(sz int) {
	lastX := c.placeX
	lastY := c.placeY
	for j := 0; j < len(c.points); j += sz {
		for i := 0; i < sz; i += 2 {
			c.points[i+j] += lastX
			c.points[i+1+j] += lastY
		}
		lastX = c.points[(j+sz)-2]
		lastY = c.points[(j+sz)-1]
	}
}

func (c *PathCursor) hasSetsOrMore(sz int, rel bool) bool {
	if !(len(c.points) >= sz && len(c.points)%sz == 0) {
		return false
	}
	if rel {
		c.pointsToAbs(sz)
	}
	return true
}

// ReadFloat reads a floating point value and adds it to the cursor's points slice.
func (c *PathCursor) ReadFloat(numStr string) error {
	f, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return err
	}
	c.points = append(c.points, f)
	return nil
}

// GetPoints reads a set of floating point values from the SVG format number string,
// and add them to the cursor's points slice.
func (c *PathCursor) GetPoints(dataPoints string) error {
	lastIndex := -1
	c.points = c.points[0:0]
	lr := ' '
	for i, r := range dataPoints {
		if unicode.IsNumber(r) == false && r != '.' && !(r == '-' && lr == 'e') && r != 'e' {
			if lastIndex != -1 {
				if err := c.ReadFloat(dataPoints[lastIndex:i]); err != nil {
					return err
				}
			}
			if r == '-' {
				lastIndex = i
			} else {
				lastIndex = -1
			}
		} else if lastIndex == -1 {
			lastIndex = i
		}
		lr = r
	}
	if lastIndex != -1 && lastIndex != len(dataPoints) {
		if err := c.ReadFloat(dataPoints[lastIndex:len(dataPoints)]); err != nil {
			return err
		}
	}
	return nil
}

// addSeg decodes an SVG seqment string into equivalent raster path commands saved
// in the cursor's Path
func (c *PathCursor) addSeg(segString string) error {
	// Parse the string describing the numeric points in SVG format
	if err := c.GetPoints(segString[1:]); err != nil {
		return err
	}
	l := len(c.points)
	k := segString[0]
	rel := false
	switch k {
	case 'z':
		fallthrough
	case 'Z':
		if len(c.points) != 0 {
			return paramMismatchError
		}
		if c.inPath {
			c.Path.Stop(true)
			c.inPath = false
		}
	case 'm':
		c.placeX = 0
		c.placeY = 0
		rel = true
		fallthrough
	case 'M':
		if !c.hasSetsOrMore(2, rel) {
			return paramMismatchError
		}
		c.pathStartX, c.pathStartY = fixed.Int26_6(c.points[0]*64), fixed.Int26_6(c.points[1]*64)
		c.inPath = true
		c.Path.Start(fixed.Point26_6{c.pathStartX, c.pathStartY})
		for i := 2; i < l-1; i += 2 {
			c.Path.Line(fixed.Point26_6{
				fixed.Int26_6(c.points[i] * 64),
				fixed.Int26_6(c.points[i+1] * 64)})
		}
		c.placeX = c.points[l-2]
		c.placeY = c.points[l-1]
	case 'l':
		rel = true
		fallthrough
	case 'L':
		if !c.hasSetsOrMore(2, rel) {
			return paramMismatchError
		}
		for i := 0; i < l-1; i += 2 {
			c.Path.Line(fixed.Point26_6{
				fixed.Int26_6(c.points[i] * 64),
				fixed.Int26_6(c.points[i+1] * 64)})
		}
		c.placeX = c.points[l-2]
		c.placeY = c.points[l-1]
	case 'v':
		c.valsToAbs(c.placeY)
		fallthrough
	case 'V':
		if !c.hasSetsOrMore(1, false) {
			return paramMismatchError
		}
		for _, p := range c.points {
			c.Path.Line(fixed.Point26_6{
				fixed.Int26_6(c.placeX * 64),
				fixed.Int26_6(p * 64)})
		}
		c.placeY = c.points[l-1]
	case 'h':
		c.valsToAbs(c.placeX)
		fallthrough
	case 'H':
		if !c.hasSetsOrMore(1, false) {
			return paramMismatchError
		}
		for _, p := range c.points {
			c.Path.Line(fixed.Point26_6{
				fixed.Int26_6(p * 64),
				fixed.Int26_6(c.placeY * 64)})
		}
		c.placeX = c.points[l-1]
	case 'q':
		rel = true
		fallthrough
	case 'Q':
		if !c.hasSetsOrMore(4, rel) {
			return paramMismatchError
		}
		for i := 0; i < l-3; i += 4 {
			c.Path.QuadBezier(
				fixed.Point26_6{
					fixed.Int26_6(c.points[i] * 64),
					fixed.Int26_6(c.points[i+1] * 64)},
				fixed.Point26_6{
					fixed.Int26_6(c.points[i+2] * 64),
					fixed.Int26_6(c.points[i+3] * 64)})
		}
		c.cntlPtX, c.cntlPtY = c.points[l-4], c.points[l-3]
		c.placeX = c.points[l-2]
		c.placeY = c.points[l-1]
	case 't':
		rel = true
		fallthrough
	case 'T':
		if !c.hasSetsOrMore(2, rel) {
			return paramMismatchError
		}
		for i := 0; i < l-1; i += 2 {
			switch c.lastKey {
			case 'q', 'Q', 'T', 't':
				c.cntlPtX, c.cntlPtY = reflect(c.placeX, c.placeY, c.cntlPtX, c.cntlPtY)
			default:
				c.cntlPtX, c.cntlPtY = c.placeX, c.placeY
			}
			c.Path.QuadBezier(
				fixed.Point26_6{
					fixed.Int26_6(c.cntlPtX * 64),
					fixed.Int26_6(c.cntlPtY * 64)},
				fixed.Point26_6{
					fixed.Int26_6(c.points[i] * 64),
					fixed.Int26_6(c.points[i+1] * 64)})
			c.lastKey = k
			c.placeX = c.points[i]
			c.placeY = c.points[i+1]
		}
	case 'c':
		rel = true
		fallthrough
	case 'C':
		if !c.hasSetsOrMore(6, rel) {
			return paramMismatchError
		}
		for i := 0; i < l-5; i += 6 {
			c.Path.CubeBezier(
				fixed.Point26_6{
					fixed.Int26_6(c.points[i] * 64),
					fixed.Int26_6(c.points[i+1] * 64)},
				fixed.Point26_6{
					fixed.Int26_6(c.points[i+2] * 64),
					fixed.Int26_6(c.points[i+3] * 64)},
				fixed.Point26_6{
					fixed.Int26_6(c.points[i+4] * 64),
					fixed.Int26_6(c.points[i+5] * 64)})
		}
		c.cntlPtX, c.cntlPtY = c.points[l-4], c.points[l-3]
		c.placeX = c.points[l-2]
		c.placeY = c.points[l-1]
	case 's':
		rel = true
		fallthrough
	case 'S':
		if !c.hasSetsOrMore(4, rel) {
			return paramMismatchError
		}
		for i := 0; i < l-3; i += 4 {
			switch c.lastKey {
			case 'c', 'C', 's', 'S':
				c.cntlPtX, c.cntlPtY = reflect(c.placeX, c.placeY, c.cntlPtX, c.cntlPtY)
			default:
				c.cntlPtX, c.cntlPtY = c.placeX, c.placeY
			}
			c.Path.CubeBezier(fixed.Point26_6{
				fixed.Int26_6(c.cntlPtX * 64), fixed.Int26_6(c.cntlPtY * 64)},
				fixed.Point26_6{
					fixed.Int26_6(c.points[i] * 64), fixed.Int26_6(c.points[i+1] * 64)},
				fixed.Point26_6{
					fixed.Int26_6(c.points[i+2] * 64), fixed.Int26_6(c.points[i+3] * 64)})
			c.lastKey = k
			c.cntlPtX, c.cntlPtY = c.points[i], c.points[i+1]
			c.placeX = c.points[i+2]
			c.placeY = c.points[i+3]
		}
	case 'a', 'A':
		if !c.hasSetsOrMore(7, false) {
			return paramMismatchError
		}
		for i := 0; i < l-6; i += 7 {
			if k == 'a' {
				c.points[i+5] += c.placeX
				c.points[i+6] += c.placeY
			}
			c.AddArcFromA(c.points[i:])
		}
	default:
		if c.ErrorMode == StrictErrorMode {
			return commandUnknownError
		}
		if c.ErrorMode == WarnErrorMode {
			log.Println("Ignoring svg command " + string(k))
		}
	}
	// So we know how to extend some segment types
	c.lastKey = k
	return nil
}

func (c *PathCursor) ElipseAt(cx, cy, rx, ry float64) {
	c.placeX, c.placeY = cx+rx, cy
	c.points = c.points[0:0]
	c.points = append(c.points, rx, ry, 0.0, 1.0, 0.0, c.placeX, c.placeY)
	c.Path.Start(fixed.Point26_6{
		fixed.Int26_6(c.placeX * 64),
		fixed.Int26_6(c.placeY * 64)})
	c.AddArcFromAC(c.points, cx, cy)
	c.Path.Stop(true)
}

func (c *PathCursor) AddArcFromA(points []float64) {
	cx, cy := FindEllipseCenter(&points[0], &points[1], points[2]*math.Pi/180, c.placeX,
		c.placeY, points[5], points[6], points[4] == 0, points[3] == 0)
	c.AddArcFromAC(points, cx, cy)
}

func (c *PathCursor) AddArcFromAC(points []float64, cx, cy float64) {
	rotX := points[2] * math.Pi / 180 // Convert degress to radians
	largeArc := points[3] != 0
	sweep := points[4] != 0
	startAngle := math.Atan2(c.placeY-cy, c.placeX-cx) - rotX
	endAngle := math.Atan2(points[6]-cy, points[5]-cx) - rotX
	deltaTheta := endAngle - startAngle
	arcBig := math.Abs(deltaTheta) > math.Pi

	// Approximate ellipse using cubic bezeir splines
	etaStart := math.Atan2(math.Sin(startAngle)/points[1], math.Cos(startAngle)/points[0])
	etaEnd := math.Atan2(math.Sin(endAngle)/points[1], math.Cos(endAngle)/points[0])
	deltaEta := etaEnd - etaStart
	if (arcBig && !largeArc) || (!arcBig && largeArc) { // Go has no boolean XOR
		if deltaEta < 0 {
			deltaEta += math.Pi * 2
		} else {
			deltaEta -= math.Pi * 2
		}
	}
	// This check migth be needed if the center point of the elipse is
	// at the midpoint of the start and end lines.
	if deltaEta < 0 && sweep {
		deltaEta += math.Pi * 2
	} else if deltaEta >= 0 && !sweep {
		deltaEta -= math.Pi * 2
	}

	// Round up to determine number of cubic splines to approximate bezier curve
	segs := int(math.Abs(deltaEta)/MaxDx) + 1
	dEta := deltaEta / float64(segs) // span of each segment
	// Approximate the ellipse using a set of cubic bezier curves by the method of
	// L. Maisonobe, "Drawing an elliptical arc using polylines, quadratic
	// or cubic Bezier curves", 2003
	// https://www.spaceroots.org/documents/elllipse/elliptical-arc.pdf
	tde := math.Tan(dEta / 2)
	alpha := math.Sin(dEta) * (math.Sqrt(4+3*tde*tde) - 1) / 3 // Math is fun!
	lx, ly := c.placeX, c.placeY
	sinTheta, cosTheta := math.Sin(rotX), math.Cos(rotX)
	ldx, ldy := ellipsePrime(points[0], points[1], sinTheta, cosTheta, etaStart, cx, cy)
	for i := 1; i <= segs; i++ {
		eta := etaStart + dEta*float64(i)
		var px, py float64
		if i == segs {
			px, py = points[5], points[6] // Just makes the end point exact; no roundoff error
		} else {
			px, py = ellipsePointAt(points[0], points[1], sinTheta, cosTheta, eta, cx, cy)
		}
		dx, dy := ellipsePrime(points[0], points[1], sinTheta, cosTheta, eta, cx, cy)

		c.Path.CubeBezier(fixed.Point26_6{
			fixed.Int26_6((lx + alpha*ldx) * 64), fixed.Int26_6((ly + alpha*ldy) * 64)},
			fixed.Point26_6{
				fixed.Int26_6((px - alpha*dx) * 64), fixed.Int26_6((py - alpha*dy) * 64)},
			fixed.Point26_6{
				fixed.Int26_6(px * 64), fixed.Int26_6(py * 64)})
		lx, ly, ldx, ldy = px, py, dx, dy
	}
	c.placeX, c.placeY = lx, ly
}

// ellipsePrime gives tangent vectors for parameterized elipse; a, b, radii, eta parameter, center cx, cy
func ellipsePrime(a, b, sinTheta, cosTheta, eta, cx, cy float64) (px, py float64) {
	bCosEta := b * math.Cos(eta)
	aSinEta := a * math.Sin(eta)
	px = -aSinEta*cosTheta - bCosEta*sinTheta
	py = -aSinEta*sinTheta + bCosEta*cosTheta
	return
}

// ellipsePointAt gives points for parameterized elipse; a, b, radii, eta parameter, center cx, cy
func ellipsePointAt(a, b, sinTheta, cosTheta, eta, cx, cy float64) (px, py float64) {
	aCosEta := a * math.Cos(eta)
	bSinEta := b * math.Sin(eta)
	px = cx + aCosEta*cosTheta - bSinEta*sinTheta
	py = cy + aCosEta*sinTheta + bSinEta*cosTheta
	return
}

// FindEllipseCenter locates the center of the Ellipse if it exists. If it does not exist,
// the radius values will be increased minimally for a solution to be possible
// while preserving the ra to rb ratio.  ra and rb arguments are pointers that can be
// checked after the call to see if the values changed. This method uses coordinate transformations
// to reduce the problem to finding the center of a circle that includes the origin
// and an arbitrary point. The center of the circle is then transformed
// back to the original coordinates and returned.
func FindEllipseCenter(ra, rb *float64, rotX, startX, startY, endX, endY float64, sweep, smallArc bool) (cx, cy float64) {
	cos, sin := math.Cos(rotX), math.Sin(rotX)

	// Move origin to start point
	nx, ny := endX-startX, endY-startY

	// Rotate ellipse x-axis to coordinate x-axis
	nx, ny = nx*cos+ny*sin, -nx*sin+ny*cos
	// Scale X dimension so that ra = rb
	nx *= *rb / *ra // Now the ellipse is a circle radius rb; therefore foci and center coincide

	midX, midY := nx/2, ny/2
	midlenSq := midX*midX + midY*midY

	var hr float64 = 0.0
	if *rb**rb < midlenSq {
		// Requested ellipse does not exist; scale ra, rb to fit. Length of
		// span is greater than max width of ellipse, must scale *ra, *rb
		nrb := math.Sqrt(midlenSq)
		if *ra == *rb {
			*ra = nrb // prevents roundoff
		} else {
			*ra = *ra * nrb / *rb
		}
		*rb = nrb
	} else {
		hr = math.Sqrt(*rb**rb-midlenSq) / math.Sqrt(midlenSq)
	}
	// Notice that if hr is zero, both answers are the same.
	if (sweep && smallArc) || (!sweep && !smallArc) {
		cx = midX + midY*hr
		cy = midY - midX*hr
	} else {
		cx = midX - midY*hr
		cy = midY + midX*hr
	}

	// reverse scale
	cx *= *ra / *rb
	//Reverse rotate and translate back to original coordinates
	return cx*cos - cy*sin + startX, cx*sin + cy*cos + startY
}

func (c *PathCursor) init() {
	c.placeX = 0.0
	c.placeY = 0.0
	c.points = c.points[0:0]
	c.lastKey = ' '
	c.Path.Clear()
	c.inPath = false
}

// CompilePath translates the svgPath description string into a rasterx path.
// All valid SVG path elements are interpreted to draw2d equivalents. Ellipses tilted relative
// the x-axis as defined by the SVG 'a' and 'A' elements are approximated
// with cubic bezier splines since draw2d has no off-axis ellipse type.
// The resulting path element is stored in the SvgCursor.
func (c *PathCursor) CompilePath(svgPath string) error {
	c.init()
	lastIndex := -1
	for i, v := range svgPath {
		if unicode.IsLetter(v) && v != 'e' {
			if lastIndex != -1 {
				if err := c.addSeg(svgPath[lastIndex:i]); err != nil {
					return err
				}
			}
			lastIndex = i
		}
	}
	if lastIndex != -1 {
		if err := c.addSeg(svgPath[lastIndex:]); err != nil {
			return err
		}
	}
	return nil
}
