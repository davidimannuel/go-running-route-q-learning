package osm

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

var widthPattern = regexp.MustCompile(`^([0-9]+(?:\.[0-9]+)?)\s*(?:m|meter|metre|meters|metres)?$`)

// ParseWidthM accepts a single positive number, decimal commas, and recognized
// meter units. Lists, semicolon combinations, feet, non-positive values, and
// malformed numbers are not averaged or guessed. The original tag is preserved.
func ParseWidthM(value string) (float64, bool) {
	text := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(value)), ",", ".")
	match := widthPattern.FindStringSubmatch(text)
	if match == nil {
		return 0, false
	}
	meters, err := strconv.ParseFloat(match[1], 64)
	if err != nil || meters <= 0 || math.IsInf(meters, 0) {
		return 0, false
	}
	return meters, true
}
