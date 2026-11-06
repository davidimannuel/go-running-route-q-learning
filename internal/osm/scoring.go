package osm

import (
	"math"
	"strconv"
	"strings"
)

// These weights and mappings match the active reference rute_lari_core.py,
// section B (direct_signed_unclipped_missing_05_20261103). They are research
// preferences, not measured runner comfort, accessibility, or road safety.
const (
	HighwayWeight = 0.55
	SurfaceWeight = 0.30
	WidthWeight   = 0.15
)

// RoadScores contains the three signed components and their unclipped sum.
type RoadScores struct {
	HighwayScore float64 `json:"highway_score"`
	SurfaceScore float64 `json:"surface_score"`
	WidthScore   float64 `json:"width_score"`
	ComfortScore float64 `json:"comfort_score"`
}

// Scores uses width already parsed during graph loading. No export reparses tags.
// Keeping one mapping function prevents CSV and HTML exports from diverging.
func (e OSMEdge) Scores() RoadScores {
	return ScoreRoad(e.Highway, e.Surface, e.Width, e.WidthValid)
}

// ScoreRoad matches calculate_highway_score, calculate_surface_score, and
// width_score in the reference. Width must come from ParseWidthM at graph loading;
// its validity flag distinguishes unavailable data from a usable measurement.
// Access is a separate journey reward component.
func ScoreRoad(highway, surface string, widthM float64, widthValid bool) RoadScores {
	scores := RoadScores{
		HighwayScore: categoryScore(highway, func(value string) bool {
			switch value {
			case "footway", "pedestrian", "living_street", "residential", "tertiary":
				return true
			}
			return false
		}),
		SurfaceScore: categoryScore(surface, func(value string) bool {
			switch value {
			case "asphalt", "concrete", "paving_stones", "paved":
				return true
			}
			return false
		}),
		WidthScore: -0.5,
	}
	if widthValid && widthM > 0 && !math.IsInf(widthM, 0) {
		scores.WidthScore = -1
		if widthM >= 3 {
			scores.WidthScore = 1
		}
	}
	scores.ComfortScore = HighwayWeight*scores.HighwayScore + SurfaceWeight*scores.SurfaceScore + WidthWeight*scores.WidthScore
	return scores
}

func categoryScore(raw string, priority func(string) bool) float64 {
	text := strings.ToLower(strings.TrimSpace(raw))
	values := []string{text}
	// OSM downloads use scalar strings. Also recognize quoted string lists from
	// reference exports, including Python's single-quoted list representation.
	if parsed, ok := quotedTagList(text); ok {
		values = parsed
	}
	score, found := 1.0, false
	for _, value := range values {
		for part := range strings.SplitSeq(value, ";") {
			part = strings.ToLower(strings.TrimSpace(part))
			if part == "" {
				continue
			}
			found = true
			if !priority(part) {
				score = -1 // Minimum over all non-empty categories, as in the reference.
			}
		}
	}
	if !found {
		return -0.5
	}
	return score
}

// quotedTagList parses only string lists, without evaluating Python expressions.
// Unsupported bracketed values fall back to an unknown scalar category.
func quotedTagList(text string) ([]string, bool) {
	if !strings.HasPrefix(text, "[") || !strings.HasSuffix(text, "]") {
		return nil, false
	}
	text = strings.TrimSpace(text[1 : len(text)-1])
	var values []string
	for text != "" {
		quote := text[0]
		if quote != '\'' && quote != '"' {
			return nil, false
		}
		text = text[1:]
		var value strings.Builder
		for text != "" && text[0] != quote {
			character, _, rest, err := strconv.UnquoteChar(text, quote)
			if err != nil {
				return nil, false
			}
			value.WriteRune(character)
			text = rest
		}
		if text == "" {
			return nil, false
		}
		values = append(values, value.String())
		text = strings.TrimSpace(text[1:])
		if text == "" {
			break
		}
		if text[0] != ',' {
			return nil, false
		}
		text = strings.TrimSpace(text[1:])
	}
	return values, true
}
