package osm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/davidimannuel/go-running-route-q-learning/internal/logger"
)

const (
	// https://github.com/gboeing/osmnx/blob/v2.1.1/osmnx/_overpass.py#L101
	walkFilter = `["highway"]["area"!~"yes"]["access"!~"private"]["highway"!~"abandoned|bus_guideway|construction|cycleway|motor|no|planned|platform|proposed|raceway|razed|rest_area|services"]["foot"!~"no"]["service"!~"private"]["sidewalk"!~"separate"]["sidewalk:both"!~"separate"]["sidewalk:left"!~"separate"]["sidewalk:right"!~"separate"]`
)

func query(lat, lon, radiusM float64, timeoutSec int) string {
	bbox := bboxFromPoint(lat, lon, radiusM)
	return fmt.Sprintf("[out:json][timeout:%d];(way%s(%.8f,%.8f,%.8f,%.8f);>;);out body;",
		timeoutSec, walkFilter,
		bbox.South, bbox.West, bbox.North, bbox.East)
}

type overpassClient struct {
	endpoint string
	httpC    *http.Client
}

func NewOverpassClient() *overpassClient {
	return &overpassClient{
		endpoint: "https://overpass-api.de/api/interpreter",
		httpC:    &http.Client{Timeout: 2 * time.Minute},
	}
}

type FetchParam struct {
	Lat, Long, RadM float64
	TimeoutSec      int
}

// Validate checks map parameters before building an Overpass request.
func (p FetchParam) Validate() error {
	if math.IsNaN(p.Lat) || math.IsInf(p.Lat, 0) || p.Lat <= -90 || p.Lat >= 90 {
		return fmt.Errorf("overpass: latitude must be finite and strictly between -90 and 90 degrees")
	}
	if math.IsNaN(p.Long) || math.IsInf(p.Long, 0) || p.Long < -180 || p.Long > 180 {
		return fmt.Errorf("overpass: longitude must be finite and between -180 and 180 degrees")
	}
	if math.IsNaN(p.RadM) || math.IsInf(p.RadM, 0) || p.RadM <= 0 {
		return fmt.Errorf("overpass: radius must be finite and greater than zero meters")
	}
	if p.TimeoutSec <= 0 {
		return fmt.Errorf("overpass: query timeout must be greater than zero seconds")
	}

	bbox := bboxFromPoint(p.Lat, p.Long, p.RadM)
	// This bbox implementation does not support crossing a pole or the antimeridian.
	if bbox.South < -90 || bbox.North > 90 || bbox.West < -180 || bbox.East > 180 ||
		bbox.South >= bbox.North || bbox.West >= bbox.East {
		return fmt.Errorf("overpass: radius produces an unsupported bounding box")
	}
	return nil
}

const maxAttempt = 3

func (c *overpassClient) Fetch(ctx context.Context, p FetchParam) (OSMData, error) {
	if err := p.Validate(); err != nil {
		return OSMData{}, err
	}
	q := query(p.Lat, p.Long, p.RadM, p.TimeoutSec)
	var lastErr error

	for attempt := 1; attempt <= maxAttempt; attempt++ {
		if err := ctx.Err(); err != nil {
			return OSMData{}, fmt.Errorf("overpass: request canceled: %w", err)
		}
		data, status, err := c.fetchOnce(ctx, q)
		if err == nil {
			return data, nil
		}
		lastErr = err

		switch status {
		case http.StatusTooManyRequests, http.StatusBadGateway,
			http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			// Only these HTTP statuses are retried, as in the original client.
		default:
			return OSMData{}, err
		}
		if attempt == maxAttempt {
			break
		}

		delay := time.Second
		if status == http.StatusTooManyRequests {
			// The public Overpass operator requires at least 30 seconds after a 429.
			delay = 30 * time.Second
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return OSMData{}, fmt.Errorf("overpass: retry canceled: %w", ctx.Err())
		case <-timer.C:
		}
	}

	return OSMData{}, fmt.Errorf("overpass: request failed after %d attempts: %w", maxAttempt, lastErr)
}

// fetchOnce closes the response body before Fetch decides whether to retry.
func (c *overpassClient) fetchOnce(ctx context.Context, q string) (OSMData, int, error) {
	body := strings.NewReader(url.Values{"data": {q}}.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, body)
	if err != nil {
		return OSMData{}, 0, fmt.Errorf("overpass: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "go-running-route-q-learning/0.1 (+https://github.com/davidimannuel/uwika-ta-31123022-2026)")

	resp, err := c.httpC.Do(req)
	if err != nil {
		logger.Error(ctx, "Overpass request failed", logger.Err(err))
		return OSMData{}, 0, fmt.Errorf("overpass: send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return OSMData{}, resp.StatusCode, fmt.Errorf("overpass: read HTTP %d response: %w", resp.StatusCode, err)
		}
		logger.Debug(ctx, "Overpass returned an HTTP error",
			logger.Int64("status", int64(resp.StatusCode)), logger.String("response", string(bodyBytes)))
		return OSMData{}, resp.StatusCode, fmt.Errorf("overpass: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	var data OSMData
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return OSMData{}, resp.StatusCode, fmt.Errorf("overpass: decode response: %w", err)
	}
	if strings.TrimSpace(data.Remark) != "" {
		return OSMData{}, resp.StatusCode, fmt.Errorf("overpass: server remark: %s", data.Remark)
	}
	return data, resp.StatusCode, nil
}
