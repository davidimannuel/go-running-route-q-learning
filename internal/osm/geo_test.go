package osm

import (
	"math"
	"testing"
)

func TestBBoxFromPoint(t *testing.T) {
	got := bboxFromPoint(-7.282187, 112.709282, 1000)
	want := BBox{South: -7.29118020, West: 112.70021567, North: -7.27319380, East: 112.71834833}
	for _, bound := range []struct {
		name      string
		got, want float64
	}{
		{"south", got.South, want.South},
		{"west", got.West, want.West},
		{"north", got.North, want.North},
		{"east", got.East, want.East},
	} {
		if math.Abs(bound.got-bound.want) > 1e-8 {
			t.Errorf("%s=%.10f; want %.10f", bound.name, bound.got, bound.want)
		}
	}
}
