package osm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/davidimannuel/go-running-route-q-learning/internal/logger"
)

// ErrInvalidCache identifies corrupt or unsuccessful cached responses.
var ErrInvalidCache = errors.New("invalid OSM cache")

// cacheKey hashes the endpoint and query supplied as a complete request identity.
func cacheKey(data string) string {
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash)
}

// Cache stores successful OSM responses by request identity without expiration.
type Cache struct{ cacheDir string }

func NewCache(cacheDir string) (*Cache, error) {
	if strings.TrimSpace(cacheDir) == "" {
		return nil, fmt.Errorf("cache: directory is required")
	}
	return &Cache{cacheDir: filepath.Clean(cacheDir)}, nil
}

func (c *Cache) path(key string) (string, error) {
	if len(key) != sha256.Size*2 {
		return "", fmt.Errorf("cache: key must be a 64-character hexadecimal SHA-256 digest")
	}
	if _, err := hex.DecodeString(key); err != nil {
		return "", fmt.Errorf("cache: invalid key: %w", err)
	}
	return filepath.Join(c.cacheDir, fmt.Sprintf("osm_cache_%s.json", key)), nil
}

// Get returns a missing cache entry as (OSMData{}, false, nil).
// Callers may recover from ErrInvalidCache by fetching a fresh response.
func (c *Cache) Get(ctx context.Context, key string) (OSMData, bool, error) {
	if err := ctx.Err(); err != nil {
		return OSMData{}, false, fmt.Errorf("cache: read canceled: %w", err)
	}
	path, err := c.path(key)
	if err != nil {
		return OSMData{}, false, err
	}
	logger.Debug(ctx, "Reading OSM cache", logger.String("path", path))
	body, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		logger.Debug(ctx, "OSM cache miss", logger.String("path", path))
		return OSMData{}, false, nil
	}
	if err != nil {
		return OSMData{}, false, fmt.Errorf("cache: read %s: %w", path, err)
	}
	if err := ctx.Err(); err != nil {
		return OSMData{}, false, fmt.Errorf("cache: read canceled: %w", err)
	}
	var data OSMData
	if err := json.Unmarshal(body, &data); err != nil {
		return OSMData{}, false, fmt.Errorf("%w: decode %s: %v", ErrInvalidCache, path, err)
	}
	if err := validateOSMData(data); err != nil {
		return OSMData{}, false, fmt.Errorf("%w: %s: %v", ErrInvalidCache, path, err)
	}
	logger.Debug(ctx, "OSM cache hit", logger.String("path", path), logger.Int("elements", len(data.Elements)))
	return data, true, nil
}

// Set replaces a cache entry only after its new contents are fully written.
func (c *Cache) Set(ctx context.Context, key string, data OSMData) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("cache: write canceled: %w", err)
	}
	path, err := c.path(key)
	if err != nil {
		return err
	}
	if err := validateOSMData(data); err != nil {
		return fmt.Errorf("cache: refuse invalid response: %w", err)
	}
	if err := os.MkdirAll(c.cacheDir, 0o755); err != nil {
		return fmt.Errorf("cache: create directory: %w", err)
	}
	file, err := os.CreateTemp(c.cacheDir, ".osm-cache-*.json")
	if err != nil {
		return fmt.Errorf("cache: create temporary file: %w", err)
	}
	defer func() {
		file.Close()
		os.Remove(file.Name())
	}()
	if err := json.NewEncoder(file).Encode(data); err != nil {
		return fmt.Errorf("cache: encode response: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("cache: flush response: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("cache: close response: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("cache: write canceled: %w", err)
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return fmt.Errorf("cache: replace entry: %w", err)
	}
	logger.Debug(ctx, "Saved OSM cache", logger.String("path", path), logger.Int("elements", len(data.Elements)))
	return nil
}
