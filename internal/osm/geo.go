package osm

import (
	"fmt"
	"math"
)

const (
	earthRadiusM = 6_371_009 // meters
)

type BBox struct {
	South, West, North, East float64
}

// bboxFromPoint uses the spherical-Earth approximation from OSMnx.
// Reference: https://github.com/gboeing/osmnx/blob/v2.1.1/osmnx/utils_geo.py#L371
func bboxFromPoint(lat, lon, distanceM float64) BBox {
	latRad := lat * math.Pi / 180
	deltaLat := (distanceM / earthRadiusM) * 180 / math.Pi
	deltaLon := deltaLat / math.Cos(latRad)

	south := lat - deltaLat
	west := lon - deltaLon
	north := lat + deltaLat
	east := lon + deltaLon
	return BBox{South: south, West: west, North: north, East: east}
}

// greatCircleDistanceM returns the spherical distance between two lat/lon points.
// Inputs are decimal degrees; the result is meters without rounding. Identical
// coordinates are valid and return zero. Coordinates must be finite and in range.
// This matches OSMnx's Haversine model and mean Earth radius, not an ellipsoidal
// geodesic or a distance that accounts for elevation.
// Reference: https://github.com/gboeing/osmnx/blob/v2.1.1/osmnx/distance.py#L68-L112
func greatCircleDistanceM(lat1, lon1, lat2, lon2 float64) (float64, error) {
	for _, point := range [...]struct {
		name     string
		lat, lon float64
	}{
		{name: "origin", lat: lat1, lon: lon1},
		{name: "destination", lat: lat2, lon: lon2},
	} {
		if math.IsNaN(point.lat) || math.IsInf(point.lat, 0) || point.lat < -90 || point.lat > 90 {
			return 0, fmt.Errorf("great-circle distance: %s latitude must be finite and between -90 and 90 degrees", point.name)
		}
		if math.IsNaN(point.lon) || math.IsInf(point.lon, 0) || point.lon < -180 || point.lon > 180 {
			return 0, fmt.Errorf("great-circle distance: %s longitude must be finite and between -180 and 180 degrees", point.name)
		}
	}

	// Trigonometric functions expect radians, while OSM coordinates use degrees.
	lat1Rad, lat2Rad := lat1*math.Pi/180, lat2*math.Pi/180
	lon1Rad, lon2Rad := lon1*math.Pi/180, lon2*math.Pi/180
	sinHalfLat := math.Sin((lat2Rad - lat1Rad) / 2)
	sinHalfLon := math.Sin((lon2Rad - lon1Rad) / 2)
	a := sinHalfLat*sinHalfLat + math.Cos(lat1Rad)*math.Cos(lat2Rad)*sinHalfLon*sinHalfLon

	// Haversine's intermediate value is in [0,1]; clamp floating-point drift so
	// the square root and inverse sine stay defined, including near antipodes.
	a = math.Max(0, math.Min(1, a))
	arc := 2 * math.Asin(math.Sqrt(a))
	return earthRadiusM * arc, nil
}

// GreatCircleDistanceM exposes the graph loader's validated Haversine calculation
// for start snapping and direction rewards. Coordinates are degrees, result meters.
func GreatCircleDistanceM(lat1, lon1, lat2, lon2 float64) (float64, error) {
	return greatCircleDistanceM(lat1, lon1, lat2, lon2)
}
