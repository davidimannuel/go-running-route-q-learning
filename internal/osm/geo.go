package osm

import "math"

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
