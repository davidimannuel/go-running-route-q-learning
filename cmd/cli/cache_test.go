package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidimannuel/go-running-route-q-learning/internal/osm"
)

func cacheTestData(generator string) osm.OSMData {
	return osm.OSMData{Version: 0.6, Generator: generator, Elements: []osm.Element{}}
}

func TestDownloadUsesCachedResponse(t *testing.T) {
	workdir := newWorkdir(t, testConfig)
	fetcher := &fakeFetcher{data: cacheTestData("cached")}
	args := []string{"download", "--workdir", workdir}
	var out bytes.Buffer
	if err := run(context.Background(), args, &out, &out, fetcher); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(workdir, "output", "osm.json")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	fetcher.err = errors.New("network must not be called on a cache hit")
	out.Reset()
	if err := run(context.Background(), args, &out, &out, fetcher); err != nil {
		t.Fatal(err)
	}
	if fetcher.calls != 1 || !strings.Contains(out.String(), "Loaded OSM data from cache") ||
		!bytes.Contains(readFile(t, path), []byte("cached")) {
		t.Fatalf("requests=%d, output=%s", fetcher.calls, out.String())
	}
}

func TestDownloadForceRefreshesCache(t *testing.T) {
	workdir := newWorkdir(t, testConfig)
	fetcher := &fakeFetcher{data: cacheTestData("original")}
	args := []string{"download", "--workdir", workdir}
	for _, force := range []bool{false, true} {
		runArgs := append([]string{}, args...)
		if force {
			fetcher.data.Generator = "refreshed"
			runArgs = append(runArgs, "--force")
		}
		if err := run(context.Background(), runArgs, &bytes.Buffer{}, &bytes.Buffer{}, fetcher); err != nil {
			t.Fatal(err)
		}
	}
	if fetcher.calls != 2 {
		t.Fatalf("requests=%d; want 2", fetcher.calls)
	}
	path := filepath.Join(workdir, "output", "osm.json")
	cachePath := filepath.Join(workdir, "cache", "osm_cache_"+fetcher.CacheKey(fetcher.params)+".json")
	previousOutput := readFile(t, path)
	previousCache := readFile(t, cachePath)
	fetcher.err = errors.New("refresh failed")
	if err := run(context.Background(), args, &bytes.Buffer{}, &bytes.Buffer{}, fetcher); err != nil || fetcher.calls != 2 {
		t.Fatalf("refreshed cache was not reused: requests=%d, error=%v", fetcher.calls, err)
	}
	if !bytes.Contains(readFile(t, path), []byte("refreshed")) {
		t.Fatal("cached result did not contain the refreshed data")
	}
	err := run(context.Background(), append(args, "--force"), &bytes.Buffer{}, &bytes.Buffer{}, fetcher)
	if !errors.Is(err, fetcher.err) || fetcher.calls != 3 {
		t.Fatalf("requests=%d, error=%v", fetcher.calls, err)
	}
	if !bytes.Equal(previousOutput, readFile(t, path)) || !bytes.Equal(previousCache, readFile(t, cachePath)) {
		t.Fatal("failed refresh modified an existing result")
	}
}

func TestDownloadCacheFollowsQueryRatherThanOutputDirectory(t *testing.T) {
	workdir := newWorkdir(t, testConfig)
	fetcher := &fakeFetcher{data: cacheTestData("original-query")}
	args := []string{"download", "--workdir", workdir}
	if err := run(context.Background(), args, &bytes.Buffer{}, &bytes.Buffer{}, fetcher); err != nil {
		t.Fatal(err)
	}
	changed := strings.ReplaceAll(testConfig, "radius_m: 3025", "radius_m: 1500")
	if err := os.WriteFile(filepath.Join(workdir, "config.yaml"), []byte(changed), 0o600); err != nil {
		t.Fatal(err)
	}
	fetcher.data.Generator = "new-query"
	if err := run(context.Background(), args, &bytes.Buffer{}, &bytes.Buffer{}, fetcher); err != nil {
		t.Fatal(err)
	}
	if fetcher.calls != 2 {
		t.Fatalf("changed query reused the old cache: requests=%d", fetcher.calls)
	}
	if err := os.WriteFile(filepath.Join(workdir, "config.yaml"), []byte(testConfig+"output_dir: other-output\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run(context.Background(), args, &bytes.Buffer{}, &bytes.Buffer{}, fetcher); err != nil {
		t.Fatal(err)
	}
	if fetcher.calls != 2 || !bytes.Contains(readFile(t, filepath.Join(workdir, "other-output", "osm.json")), []byte("original-query")) {
		t.Fatalf("original query cache was not reused: requests=%d", fetcher.calls)
	}
	entries, err := os.ReadDir(filepath.Join(workdir, "cache"))
	if err != nil || len(entries) != 2 {
		t.Fatalf("expected separate cache entries: entries=%v, error=%v", entries, err)
	}
}

func TestDownloadRepairsCorruptCache(t *testing.T) {
	workdir := newWorkdir(t, testConfig)
	fetcher := &fakeFetcher{data: cacheTestData("original")}
	args := []string{"download", "--workdir", workdir}
	if err := run(context.Background(), args, &bytes.Buffer{}, &bytes.Buffer{}, fetcher); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(workdir, "cache", "osm_cache_"+fetcher.CacheKey(fetcher.params)+".json")
	if err := os.WriteFile(path, []byte("broken JSON"), 0o600); err != nil {
		t.Fatal(err)
	}
	fetcher.data.Generator = "repaired"
	var out bytes.Buffer
	if err := run(context.Background(), args, &out, &out, fetcher); err != nil {
		t.Fatal(err)
	}
	var saved osm.OSMData
	if err := json.Unmarshal(readFile(t, path), &saved); err != nil {
		t.Fatal(err)
	}
	if fetcher.calls != 2 || saved.Generator != "repaired" || !strings.Contains(out.String(), "Ignoring invalid cache") {
		t.Fatalf("requests=%d, saved=%+v, output=%s", fetcher.calls, saved, out.String())
	}
}

func TestDownloadCustomCacheDirectory(t *testing.T) {
	workdir := newWorkdir(t, testConfig+"cache_dir: artifacts/osm-cache\n")
	fetcher := &fakeFetcher{data: cacheTestData("custom-cache")}
	args := []string{"download", "--workdir", workdir}
	if err := run(context.Background(), args, &bytes.Buffer{}, &bytes.Buffer{}, fetcher); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(workdir, "artifacts", "osm-cache"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("configured cache directory was not used: entries=%v, error=%v", entries, err)
	}
	if _, err := os.Stat(filepath.Join(workdir, "cache")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unexpected default cache directory: %v", err)
	}
}

func TestDownloadCacheReadFailureSkipsHTTP(t *testing.T) {
	workdir := newWorkdir(t, testConfig)
	if err := os.WriteFile(filepath.Join(workdir, "cache"), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	fetcher := &fakeFetcher{data: cacheTestData("unused")}
	err := run(context.Background(), []string{"download", "--workdir", workdir}, &bytes.Buffer{}, &bytes.Buffer{}, fetcher)
	if err == nil || fetcher.calls != 0 {
		t.Fatalf("expected cache access error before HTTP: requests=%d, error=%v", fetcher.calls, err)
	}
}
