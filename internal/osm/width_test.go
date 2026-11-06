package osm

import (
	"strings"
	"testing"
)

func TestParseWidthM(t *testing.T) {
	for raw, want := range map[string]float64{
		"3": 3, " 3 M ": 3, "2,5 metres": 2.5, "3.00 meters": 3,
		"0.25 metre": 0.25, "2meter": 2, "04": 4,
	} {
		if got, ok := ParseWidthM(raw); !ok || got != want {
			t.Errorf("%q = %v, %v; want %v", raw, got, ok, want)
		}
	}
	for _, raw := range []string{"", "0", "-3", "+3", ".5", "3.", "3;4", "[3]", "['3']", "3 ft", "NaN", "Inf", "3e0", "1,2,3", strings.Repeat("9", 400)} {
		if got, ok := ParseWidthM(raw); ok || got != 0 {
			t.Errorf("%q unexpectedly parsed as %v", raw, got)
		}
	}
}
