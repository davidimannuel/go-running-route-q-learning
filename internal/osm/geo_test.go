package osm

import (
	"math"
	"strings"
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

func TestGreatCircleDistanceM(t *testing.T) {
	// Expected values come from known central angles (R * angle), independently
	// of the Haversine implementation. Small arcs also check sub-meter precision.
	for _, tc := range []struct {
		name                   string
		lat1, lon1, lat2, lon2 float64
		wantM                  float64
	}{
		{name: "identical points", lat1: -7.282187, lon1: 112.709282, lat2: -7.282187, lon2: 112.709282},
		{name: "short equatorial arc", lon2: 0.001, wantM: 111.19508372419142},
		{name: "short meridian arc", lat2: 0.002, wantM: 222.39016744838284},
		{name: "sub-meter arc", lon2: 0.000000001, wantM: 0.00011119508372419142},
		{name: "quarter circumference", lon2: 90, wantM: 10007557.535177227},
		{name: "opposite meridians at sixty degrees north", lat1: 60, lat2: 60, lon2: 180, wantM: 6671705.023451485},
		{name: "antipodes", lon2: 180, wantM: 20015115.070354454},
		{name: "near antipodes", lat2: 0.00000001, lon2: 180, wantM: 20015115.069242503},
		{name: "both poles", lat1: 90, lat2: -90, wantM: 20015115.070354454},
		{name: "crossing antimeridian", lon1: 179.999, lon2: -179.999, wantM: 222.39016744838284},
		{name: "equivalent longitude boundaries", lon1: -180, lon2: 180},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := greatCircleDistanceM(tc.lat1, tc.lon1, tc.lat2, tc.lon2)
			if err != nil {
				t.Fatal(err)
			}
			toleranceM := math.Max(1e-8, tc.wantM*1e-10)
			if math.IsNaN(got) || math.IsInf(got, 0) || got < 0 || math.Abs(got-tc.wantM) > toleranceM {
				t.Fatalf("distance=%.12f m; want %.12f m within %.12f m", got, tc.wantM, toleranceM)
			}
			reverse, err := greatCircleDistanceM(tc.lat2, tc.lon2, tc.lat1, tc.lon1)
			if err != nil || math.Abs(reverse-got) > toleranceM {
				t.Fatalf("reverse distance=%.12f m, forward=%.12f m, error=%v", reverse, got, err)
			}
		})
	}
}

func TestGreatCircleDistanceMRejectsInvalidCoordinates(t *testing.T) {
	for index, coordinate := range []struct {
		name, point, axis string
		limit             float64
	}{
		{name: "lat1", point: "origin", axis: "latitude", limit: 90},
		{name: "lon1", point: "origin", axis: "longitude", limit: 180},
		{name: "lat2", point: "destination", axis: "latitude", limit: 90},
		{name: "lon2", point: "destination", axis: "longitude", limit: 180},
	} {
		for _, invalid := range []struct {
			name  string
			value float64
		}{
			{name: "NaN", value: math.NaN()},
			{name: "positive infinity", value: math.Inf(1)},
			{name: "negative infinity", value: math.Inf(-1)},
			{name: "above upper bound", value: coordinate.limit + 0.001},
			{name: "below lower bound", value: -coordinate.limit - 0.001},
		} {
			t.Run(coordinate.name+"/"+invalid.name, func(t *testing.T) {
				var points [4]float64
				points[index] = invalid.value
				_, err := greatCircleDistanceM(points[0], points[1], points[2], points[3])
				if err == nil || !strings.Contains(err.Error(), coordinate.point+" "+coordinate.axis) {
					t.Fatalf("error=%v; want invalid %s %s", err, coordinate.point, coordinate.axis)
				}
			})
		}
	}
}
