// Copyright 2018 The oksvg Authors. All rights reserved.
//
// created: 2018 by S.R.Wiley
//_
// Implements SVG style matrix transformations.
// https://developer.mozilla.org/en-US/docs/Web/SVG/Attribute/transform
package oksvg

import (
	//"fmt"
	"math"

	"github.com/srwiley/rasterx"
	"golang.org/x/image/math/fixed"
)

type Matrix2D struct {
	A, B, C, D, E, F float64
}

func (a Matrix2D) Mult(b Matrix2D) Matrix2D {
	return Matrix2D{
		A: a.A*b.A + a.C*b.B,
		B: a.B*b.A + a.D*b.B,
		C: a.A*b.C + a.C*b.D,
		D: a.B*b.C + a.D*b.D,
		E: a.A*b.E + a.C*b.F + a.E,
		F: a.B*b.E + a.D*b.F + a.F}
}

var Identity = Matrix2D{1, 0, 0, 1, 0, 0}

// TFixed transforms a fixed.Point26_6 by the matrix
func (m Matrix2D) TFixed(a fixed.Point26_6) (b fixed.Point26_6) {
	b.X = fixed.Int26_6((float64(a.X)*m.A + float64(a.Y)*m.C) + m.E*64)
	b.Y = fixed.Int26_6((float64(a.X)*m.B + float64(a.Y)*m.D) + m.F*64)
	return
}

func (m Matrix2D) Transform(x1, y1 float64) (x2, y2 float64) {
	x2 = x1*m.A + y1*m.C + m.E
	y2 = x1*m.B + y1*m.D + m.F
	return
}

func (a Matrix2D) Scale(x, y float64) Matrix2D {
	return a.Mult(Matrix2D{
		A: x,
		B: 0,
		C: 0,
		D: y,
		E: 0,
		F: 0})
}

func (a Matrix2D) SkewY(theta float64) Matrix2D {
	return a.Mult(Matrix2D{
		A: 1,
		B: math.Tan(theta),
		C: 0,
		D: 1,
		E: 0,
		F: 0})
}

func (a Matrix2D) SkewX(theta float64) Matrix2D {
	return a.Mult(Matrix2D{
		A: 1,
		B: 0,
		C: math.Tan(theta),
		D: 1,
		E: 0,
		F: 0})
}

func (a Matrix2D) Translate(x, y float64) Matrix2D {
	return Matrix2D{
		A: a.A,
		B: a.B,
		C: a.C,
		D: a.D,
		E: a.E + x,
		F: a.F + y}
}

func (a Matrix2D) Rotate(theta float64) Matrix2D {
	return a.Mult(Matrix2D{
		A: math.Cos(theta),
		B: math.Sin(theta),
		C: -math.Sin(theta),
		D: math.Cos(theta),
		E: 0,
		F: 0})
}

type MatrixAdder struct {
	rasterx.Adder
	M Matrix2D
}

func (t *MatrixAdder) Reset() {
	t.M = Identity
}

func (t *MatrixAdder) Start(a fixed.Point26_6) {
	t.Adder.Start(t.M.TFixed(a))
}

// Line adds a linear segment to the current curve.
func (t *MatrixAdder) Line(b fixed.Point26_6) {
	t.Adder.Line(t.M.TFixed(b))
}

// QuadBezier adds a quadratic segment to the current curve.
func (t *MatrixAdder) QuadBezier(b, c fixed.Point26_6) {
	t.Adder.QuadBezier(t.M.TFixed(b), t.M.TFixed(c))
}

// CubeBezier adds a cubic segment to the current curve.
func (t *MatrixAdder) CubeBezier(b, c, d fixed.Point26_6) {
	t.Adder.CubeBezier(t.M.TFixed(b), t.M.TFixed(c), t.M.TFixed(d))
}
