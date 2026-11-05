package osm

import (
	"bytes"
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCacheKey(t *testing.T) {
	identity := "https://example.test/api/interpreter\n[out:json];node(1);out;"
	const want = "5d62827af9442adadfabdf83309e218dc591438b923bf6fa9741d99527a84c78"
	if got := cacheKey(identity); got != want {
		t.Fatalf("key=%s; want %s", got, want)
	}
}

func TestRequestCacheKey(t *testing.T) {
	client := &overpassClient{endpoint: "https://one.test/api/interpreter"}
	params := validFetchParam()
	key := client.CacheKey(params)
	if client.CacheKey(params) != key {
		t.Fatal("identical requests produced different keys")
	}
	for _, change := range []func(*FetchParam){
		func(p *FetchParam) { p.Lat += 0.001 },
		func(p *FetchParam) { p.Long += 0.001 },
		func(p *FetchParam) { p.RadM++ },
		func(p *FetchParam) { p.TimeoutSec++ },
	} {
		changed := params
		change(&changed)
		if client.CacheKey(changed) == key {
			t.Fatal("changed query reused the same cache key")
		}
	}
	client.endpoint = "https://two.test/api/interpreter"
	if client.CacheKey(params) == key {
		t.Fatal("changed endpoint reused the same cache key")
	}
}

func testCache(t *testing.T) *Cache {
	t.Helper()
	cache, err := NewCache(filepath.Join(t.TempDir(), "cache"))
	if err != nil {
		t.Fatal(err)
	}
	return cache
}

func validCacheData() OSMData {
	return OSMData{Version: 0.6, Generator: "original", Elements: []Element{}}
}

func TestCacheRoundTrip(t *testing.T) {
	cache := testCache(t)
	key := cacheKey("request")
	ctx := context.Background()
	if _, found, err := cache.Get(ctx, key); err != nil || found {
		t.Fatalf("expected cache miss; found=%v, error=%v", found, err)
	}
	data := validCacheData()
	for _, generator := range []string{"first", "second"} {
		data.Generator = generator
		if err := cache.Set(ctx, key, data); err != nil {
			t.Fatal(err)
		}
		got, found, err := cache.Get(ctx, key)
		if err != nil || !found || got.Generator != generator || got.Elements == nil {
			t.Fatalf("data=%+v, found=%v, error=%v", got, found, err)
		}
	}
	entries, err := os.ReadDir(cache.cacheDir)
	if err != nil || len(entries) != 1 || entries[0].Name() != "osm_cache_"+key+".json" {
		t.Fatalf("unexpected cache files: entries=%v, error=%v", entries, err)
	}
}

func TestCacheRejectsInvalidEntries(t *testing.T) {
	for _, body := range []string{
		"not JSON", "null", "{}", `{"version":0.6}`, `{"version":0.6,"elements":null}`,
		`{"version":0.6,"elements":[],"remark":"runtime error"}`,
		`{"version":0.6,"elements":[]} {}`,
	} {
		cache := testCache(t)
		key := cacheKey("request")
		if err := os.MkdirAll(cache.cacheDir, 0o755); err != nil {
			t.Fatal(err)
		}
		path, err := cache.path(key)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		_, found, err := cache.Get(context.Background(), key)
		if found || !errors.Is(err, ErrInvalidCache) {
			t.Fatalf("body=%s, found=%v, error=%v", body, found, err)
		}
	}
}

func TestCacheFailedWritePreservesEntry(t *testing.T) {
	cache := testCache(t)
	key := cacheKey("request")
	ctx := context.Background()
	if err := cache.Set(ctx, key, validCacheData()); err != nil {
		t.Fatal(err)
	}
	path, _ := cache.path(key)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	bad := validCacheData()
	bad.Elements = []Element{{Type: "node", Lat: math.NaN()}}
	if err := cache.Set(ctx, key, bad); err == nil {
		t.Fatal("expected encoding failure")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("previous cache was modified: %v", err)
	}
	entries, err := os.ReadDir(cache.cacheDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary cache was not cleaned up: entries=%v, error=%v", entries, err)
	}
	for _, data := range []OSMData{
		{}, {Version: 0.6}, {Version: 0.6, Elements: []Element{}, Remark: "runtime error"},
	} {
		if err := cache.Set(ctx, key, data); err == nil {
			t.Fatal("invalid response was accepted for caching")
		}
	}
}

func TestCacheCancellation(t *testing.T) {
	cache := testCache(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	key := cacheKey("request")
	if _, _, err := cache.Get(ctx, key); !errors.Is(err, context.Canceled) {
		t.Fatalf("read cancellation was lost: %v", err)
	}
	if err := cache.Set(ctx, key, validCacheData()); !errors.Is(err, context.Canceled) {
		t.Fatalf("write cancellation was lost: %v", err)
	}
}

func TestCacheFileErrors(t *testing.T) {
	file := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(file, []byte("existing file"), 0o600); err != nil {
		t.Fatal(err)
	}
	cache, err := NewCache(file)
	if err != nil {
		t.Fatal(err)
	}
	key := cacheKey("request")
	if _, _, err := cache.Get(context.Background(), key); err == nil || errors.Is(err, ErrInvalidCache) {
		t.Fatalf("expected filesystem error rather than corrupt-cache recovery: %v", err)
	}
	if err := cache.Set(context.Background(), key, validCacheData()); err == nil {
		t.Fatal("expected cache write failure")
	}
	if _, err := NewCache(" "); err == nil {
		t.Fatal("blank directory was accepted")
	}
	for _, key := range []string{"short", strings.Repeat("g", 64), strings.Repeat("../", 22)} {
		if _, _, err := cache.Get(context.Background(), key); err == nil {
			t.Fatalf("invalid key was accepted: %q", key)
		}
	}
}
