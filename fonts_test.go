package oksvg_test

import (
	"fmt"
	"os"
	"testing"

	. "github.com/srwiley/oksvg"
)

func TestReadTTx(t *testing.T) {
	fs := []string{"testdata/testfonts/luximr.ttx", "testdata/testfonts/luxirr.ttx", "testdata/testfonts/luxisr.ttx"}
	for _, fstr := range fs {
		fmt.Println("File: ", fstr)
		file, err := os.Open(fstr)
		if err != nil {
			t.Error(err)
		}
		defer file.Close()
		ttx, err := ParseTTX(file)
		if err != nil {
			t.Error(err)
			return
		}
		//fmt.Println("hmtx ", len(ttx.MtxTable))
		fmt.Println("glyphs ", len(ttx.GlyphTable))
		q := ttx.GlyphTable[ttx.CodeTable[uint16(0xbf)]]
		//fmt.Println(" width ", q.Width, " ", q.MaxX-q.MinX, " ", q.Lsb, " ", q.Name)
		//q := ttx.GlyphTable["A"]
		//fmt.Println("q mx", q.MaxX, q.MinX, q.MaxY, q.MinY)
		//fmt.Println("q contour len ", len(q.Contours))
		//fmt.Println("q contour1 len ", len(q.Contours[1]))
		//fmt.Println("q contour1 0 pt ", q.Contours[0][0])
		//fmt.Println("q contour1 1 pt ", q.Contours[0][1])
		//fmt.Println("q contour1 Lsb, width  ", q.Lsb, q.Width)
		//v := uint16(0x23)
		//fmt.Println("code ", v, " ", ttx.CodeTable[v])
		//fmt.Println("Path ", q.Paths[0].ToSVGPath())
		//fmt.Println("Path ", q.Paths[1].ToSVGPath())

		fmt.Println("kern A to V  ", ttx.KernTable["A"+":"+"V"])

		err = q.SaveAsSVG(fstr + ".svg")
		if err != nil {
			t.Error(err)
			return
		}

		for k, v := range ttx.CodeTable {
			g, ok := ttx.GlyphTable[v]
			if !ok {
				continue
			}
			if g.Code != k {
				t.Log("warning: code and glyph do not match ", g.Code, g.Name, k)
			}
		}

		// for _, q := range ttx.GlyphTable {
		// 	if q.Width < q.MaxX-q.MinX+q.Lsb {
		// 		fmt.Println("advange less than width plus lsb ", q.Width, q.MaxX-q.MinX, q.Lsb, q.MaxX-q.MinX+q.Lsb, " ", q.Name)
		// 	}
		// }

	}

}
