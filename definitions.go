package oksvg

import (
	"encoding/xml"
)

// definition is used to store XML-tags of SVG source definitions data.
type definition struct {
	ID, Tag string
	Attrs   []xml.Attr
}
