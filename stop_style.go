package oksvg

import (
	"encoding/xml"
	"github.com/srwiley/rasterx"
)

//ParseStopAttr stop Attr contain offset, stop-color, stop-opacity
func ParseStopAttr(stop *rasterx.GradStop, attr xml.Attr) (err error) {
	if stop == nil {
		return nil
	}

	switch attr.Name.Local {
	case "offset":
		stop.Offset, err = readFraction(attr.Value)
	case "stop-color":
		//todo: add current color inherit
		stop.StopColor, err = ParseSVGColor(attr.Value)
	case "stop-opacity":
		stop.Opacity, err = parseFloat(attr.Value, 64)
	}
	if err != nil {
		return err
	}
	return nil

}
