package oksvg_test

import (
	"fmt"
	"image"
	"os"
	"strings"
	"testing"

	. "github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

func GetFonts(t *testing.T) (fonts []Ttx) {
	fs := []string{
		//"testdata/testfonts/luximr.ttx",
		"testdata/testfonts/luxirr.ttx",
		//"testdata/testfonts/luxisr.ttx",
	}
	for _, fstr := range fs {
		fmt.Println("File: ", fstr)
		file, err := os.Open(fstr)
		if err != nil {
			t.Error(err)
		}
		defer file.Close()
		tx, err := ParseTTX(file)
		if err != nil {
			t.Error(err)
			return
		}
		tx.Name = "luxirr"
		fonts = append(fonts, tx)
	}
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
	fmt.Println(p.ToSVGPath())
	fmt.Fprintf(fout, "%s", p.ToSVGPath())
	fmt.Fprintf(fout, "\" fill = \"black\"  />\n</svg>\n")
	return
}

func SaveIconF(t *testing.T, fonts []Ttx, iconPath string) {
	img := DrawIconF(t, fonts, iconPath)
	if img != nil {
		p := strings.Split(iconPath, "/")
		err := SaveToPngFile(fmt.Sprintf("testdata/%s.png", p[len(p)-1]), img)
		if err != nil {
			t.Error(err)
		}
	}
}

func DrawIconF(t *testing.T, fonts []Ttx, iconPath string) image.Image {
	cursor := NewIconCursor()
	cursor.SetFonts(fonts)

	fin, errf := os.Open(iconPath)
	if errf != nil {
		t.Error(errf)
		return nil
	}
	defer fin.Close()
	icon, errSvg := cursor.ReadIconStream(fin, WarnErrorMode)
	if errSvg != nil {
		t.Error(errSvg)
		return nil
	}
	w, h := int(icon.ViewBox.W), int(icon.ViewBox.H)
	img := image.NewRGBA(image.Rect(0, 0, w, h))

	// Uncomment the next three lines and comment the three after to use ScannerFT
	//	painter := scanFT.NewRGBAPainter(img)
	//	scannerFT := scanFT.NewScannerFT(w, h, painter)
	//	raster := NewDasher(w, h, scannerFT)
	//tb := img.Bounds()
	//tb.Max.X /= 2
	scannerGV := rasterx.NewScannerGV(w, h, img, img.Bounds())
	raster := rasterx.NewDasher(w, h, scannerGV)
	icon.Draw(raster, 1.0)
	return img
}

func TestSVGText(t *testing.T) {
	fonts := GetFonts(t)
	t.Log("loaded fonts ", len(fonts))
	SaveIconF(t, fonts, "testdata/SimpleText.svg")
}

func TestReadTTx(t *testing.T) {
	fs := []string{
		//"testdata/testfonts/luximr.ttx",
		"testdata/testfonts/luxirr.ttx",
		//"testdata/testfonts/luxisr.ttx",
	}
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
		fmt.Println("created ", ttx.Created)
		fmt.Println("modified ", ttx.Modified)
		fmt.Println("units per em ", ttx.UnitsPerEm)
		fmt.Println("mins, maxs ", ttx.MinX, ttx.MinY, ttx.MaxX, ttx.MaxY)
		//q := ttx.GlyphTable[ttx.CodeTable[uint16(0xbf)]]
		//q := ttx.GlyphTable["K"]
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

		// err = q.SaveAsSVG(fstr + ".svg")
		// if err != nil {
		// 	t.Error(err)
		// 	return
		// }

		for k, v := range ttx.CodeTable {
			g, ok := ttx.GlyphTable[v]
			if !ok {
				continue
			}
			if g.Code != k {
				t.Log("warning: code and glyph do not match ", g.Code, g.Name, k)
			}
		}

		ttx.KernTable["q:j"] = int16(105)

		p, _, cons := ttx.StringToPaths("qjclash", rasterx.Identity.Scale(64, 64), true)
		fmt.Println("cons ", cons)
		SaveAsSVG("testdata/testfonts/hello.svg", p)

		for _, q := range ttx.GlyphTable {
			if q.MinX != q.Lsb {
				fmt.Println("minx not equal to lsb ", q.MinX, " ", q.Lsb)
			}
			if q.MaxY != 2033-q.Tsb {
				fmt.Println("maxy not equal to Tsb ", q.MaxY, " ", q.Tsb, q.Name)
			}
		}

	}

}
