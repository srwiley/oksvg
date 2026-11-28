// Copyright 2018 The oksvg Authors. All rights reserved.
// created: 2018 by S.R.Wiley
package oksvg

import "testing"

func TestReadFloat(t *testing.T) {
	c := new(PathCursor)

	fStr := "23.4.56"
	c.ReadFloat(fStr)
	if len(c.points) != 2 || c.points[0] != 23.4 || c.points[1] != 0.56 {
		t.Error("read float failed", fStr, c.points)
	}

	c.points = nil

	fStr = "23.4"
	c.ReadFloat(fStr)
	if len(c.points) != 1 || c.points[0] != 23.4 {
		t.Error("read float failed", fStr)
	}
	c.points = nil

	fStr = "23"
	c.ReadFloat(fStr)
	if len(c.points) != 1 || c.points[0] != 23.0 {
		t.Error("read float failed", fStr, c.points)
	}

	c.points = nil
	fStr = ".4"
	c.ReadFloat(fStr)
	if len(c.points) != 1 || c.points[0] != 0.4 {
		t.Error("read float failed", fStr, c.points)
	}

	c.points = nil
	fStr = "23.4.56.67.32"
	c.ReadFloat(fStr)
	if len(c.points) != 4 || c.points[0] != 23.4 || c.points[1] != 0.56 ||
		c.points[2] != 0.67 || c.points[3] != 0.32 {
		t.Error("read float failed", fStr, c.points)
	}

}

// TestParseColorValuePercent covers percentage components in rgb() colors.
// Percentages are floating point numbers per the SVG specification, so a
// value such as 50.5% must parse rather than fail on the decimal point.
func TestParseColorValuePercent(t *testing.T) {
	tests := []struct {
		in   string
		want uint8
	}{
		{"0%", 0},
		{"50.5%", 128},
		{"12.5%", 31},
		{"100%", 255},
		{"128", 128},
		{"300", 255},
	}
	for _, tc := range tests {
		got, err := parseColorValue(tc.in)
		if err != nil {
			t.Errorf("parseColorValue(%q) returned error: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("parseColorValue(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}

	if _, err := parseColorValue("abc%"); err == nil {
		t.Error("parseColorValue(\"abc%\") returned no error")
	}
}
