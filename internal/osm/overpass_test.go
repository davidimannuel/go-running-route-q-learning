package osm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQuery(t *testing.T) {
	res := query(-7.282187, 112.709282, 1000, 180)
	expected := `[out:json][timeout:180];(way["highway"]["area"!~"yes"]["access"!~"private"]["highway"!~"abandoned|bus_guideway|construction|cycleway|motor|no|planned|platform|proposed|raceway|razed|rest_area|services"]["foot"!~"no"]["service"!~"private"]["sidewalk"!~"separate"]["sidewalk:both"!~"separate"]["sidewalk:left"!~"separate"]["sidewalk:right"!~"separate"](-7.29118020,112.70021567,-7.27319380,112.71834833);>;);out body;`
	assert.Equal(t, expected, res)
}
