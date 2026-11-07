package qlearning

import (
	"math"
	"strconv"
	"strings"

	"github.com/davidimannuel/go-running-route-q-learning/internal/osm"
)

// RoadScores contains the three signed components and their unclipped sum.
type RoadScores struct {
	HighwayScore float64 `json:"highway_score"`
	SurfaceScore float64 `json:"surface_score"`
	WidthScore   float64 `json:"width_score"`
	ComfortScore float64 `json:"comfort_score"`
}

// ScoreEdge uses width already parsed during graph loading. No export reparses tags.
// Keeping one mapping function prevents CSV and HTML exports from diverging.
func ScoreEdge(e osm.OSMEdge) RoadScores {
	return DefaultRewardConfig().Road.ScoreEdge(e)
}

// ScoreRoad matches calculate_highway_score, calculate_surface_score, and
// width_score in the reference. Width must come from osm.ParseWidthM at graph loading;
// its validity flag distinguishes unavailable data from a usable measurement.
// Access is a separate journey reward component.
func ScoreRoad(highway, surface string, widthM float64, widthValid bool) RoadScores {
	return DefaultRewardConfig().Road.ScoreRoad(highway, surface, widthM, widthValid)
}

func (c RoadScoreConfig) ScoreEdge(e osm.OSMEdge) RoadScores {
	return c.ScoreRoad(e.Highway, e.Surface, e.Width, e.WidthValid)
}

func (c RoadScoreConfig) ScoreRoad(highway, surface string, widthM float64, widthValid bool) RoadScores {
	scores := RoadScores{
		HighwayScore: categoryScore(highway, c.Highway, func(value string) bool {
			switch value {
			case "footway", "pedestrian", "living_street", "residential", "tertiary":
				return true
			}
			return false
		}),
		SurfaceScore: categoryScore(surface, c.Surface, func(value string) bool {
			switch value {
			case "asphalt", "concrete", "paving_stones", "paved":
				return true
			}
			return false
		}),
		WidthScore: c.Width.Missing,
	}
	if widthValid && widthM > 0 && !math.IsInf(widthM, 0) {
		scores.WidthScore = c.Width.Other
		if widthM >= c.WidthThresholdM {
			scores.WidthScore = c.Width.Priority
		}
	}
	scores.ComfortScore = c.Highway.Weight*scores.HighwayScore + c.Surface.Weight*scores.SurfaceScore + c.Width.Weight*scores.WidthScore
	return scores
}

func categoryScore(raw string, cfg AttributeScoreConfig, priority func(string) bool) float64 {
	text := strings.ToLower(strings.TrimSpace(raw))
	values := []string{text}
	// OSM downloads use scalar strings. Also recognize quoted string lists from
	// reference exports, including Python's single-quoted list representation.
	if parsed, ok := quotedTagList(text); ok {
		values = parsed
	}
	score, found := math.Inf(1), false
	for _, value := range values {
		for part := range strings.SplitSeq(value, ";") {
			part = strings.ToLower(strings.TrimSpace(part))
			if part == "" {
				continue
			}
			found = true
			valueScore := cfg.Other
			if priority(part) {
				valueScore = cfg.Priority
			}
			score = math.Min(score, valueScore) // Minimum over non-empty categories.

		}
	}
	if !found {
		return cfg.Missing
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
