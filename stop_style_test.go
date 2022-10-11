package oksvg

import (
	"encoding/xml"
	"github.com/srwiley/rasterx"
	"testing"
)

func TestParseStopAttr(t *testing.T) {
	tests := []struct {
		name string
		attr xml.Attr
	}{
		{name: "stopTest1", attr: xml.Attr{
			Name:  xml.Name{Local: "stop-color"},
			Value: "#E24926",
		}},
		{name: "stopTest2", attr: xml.Attr{
			Name:  xml.Name{Local: "stop-color"},
			Value: "#FFF000",
		}},
		{name: "stopTest3", attr: xml.Attr{
			Name:  xml.Name{Local: "stop-opacity"},
			Value: "2",
		}},
		{name: "stopTest4", attr: xml.Attr{
			Name:  xml.Name{Local: "offset"},
			Value: "1",
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stop := rasterx.GradStop{}
			err := ParseStopAttr(&stop, test.attr)
			if err != nil {
				t.Log(err)
				t.FailNow()
			}
			t.Log("stop attr ", stop)
		})
	}
}
