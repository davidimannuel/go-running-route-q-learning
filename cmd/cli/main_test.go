package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidimannuel/go-running-route-q-learning/internal/osm"
)

const testConfig = `schema_version: 1
map:
  latitude: -7.282187
  longitude: 112.709282
  radius_m: 3025
  timeout_sec: 90
`

type fakeFetcher struct {
	calls  int
	params osm.FetchParam
	data   osm.OSMData
	err    error
}

func (f *fakeFetcher) Fetch(ctx context.Context, params osm.FetchParam) (osm.OSMData, error) {
	f.calls++
	f.params = params
	if err := ctx.Err(); err != nil {
		return osm.OSMData{}, err
	}
	return f.data, f.err
}

func newWorkdir(t *testing.T, config string) string {
	t.Helper()
	workdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workdir, "config.yaml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	return workdir
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestDownload(t *testing.T) {
	for _, tc := range []struct{ name, config, output string }{
		{"default output", testConfig, "output"},
		{"custom output", testConfig + "output_dir: results/baseline\n", filepath.Join("results", "baseline")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			workdir := newWorkdir(t, tc.config)
			fetcher := &fakeFetcher{data: osm.OSMData{Version: 0.6, Elements: []osm.Element{
				{Type: "node", ID: 17, Lat: -7.282187, Lon: 112.709282},
				{Type: "way", ID: 21, Nodes: []int{17, 18}, Tags: map[string]string{"highway": "residential"}},
			}}}
			var out bytes.Buffer
			if err := run(context.Background(), []string{"download", "--workdir", workdir}, &out, fetcher); err != nil {
				t.Fatal(err)
			}
			want := osm.FetchParam{Lat: -7.282187, Long: 112.709282, RadM: 3025, TimeoutSec: 90}
			if fetcher.calls != 1 || fetcher.params != want {
				t.Fatalf("requests=%d, params=%+v", fetcher.calls, fetcher.params)
			}
			path := filepath.Join(workdir, tc.output, "osm.json")
			var saved osm.OSMData
			if err := json.Unmarshal(readFile(t, path), &saved); err != nil {
				t.Fatal(err)
			}
			if len(saved.Elements) != 2 || len(saved.Elements[1].Nodes) != 2 || saved.Elements[1].Nodes[0] != 17 {
				t.Fatalf("unexpected saved OSM elements: %+v", saved.Elements)
			}
			if !strings.Contains(out.String(), path) {
				t.Fatalf("output does not identify the result path: %s", out.String())
			}
			if _, err := os.Stat(filepath.Join(workdir, "input")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("unexpected input directory: %v", err)
			}
		})
	}
}

func TestDownloadOverwritesOnlyMap(t *testing.T) {
	workdir := newWorkdir(t, testConfig)
	fetcher := &fakeFetcher{data: osm.OSMData{Generator: "first"}}
	args := []string{"download", "--workdir", workdir}
	if err := run(context.Background(), args, &bytes.Buffer{}, fetcher); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(workdir, "output", "training.csv")
	if err := os.WriteFile(other, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	fetcher.data.Generator = "second"
	if err := run(context.Background(), args, &bytes.Buffer{}, fetcher); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(readFile(t, filepath.Join(workdir, "output", "osm.json")), []byte("second")) || string(readFile(t, other)) != "keep" {
		t.Fatal("expected only osm.json to be replaced")
	}
	entries, err := os.ReadDir(filepath.Join(workdir, "output"))
	if err != nil || len(entries) != 2 {
		t.Fatalf("temporary output was not cleaned up: entries=%v, error=%v", entries, err)
	}
}

func TestDownloadFailurePreservesPreviousResult(t *testing.T) {
	for _, stage := range []string{"fetch", "encoding", "cancellation"} {
		t.Run(stage, func(t *testing.T) {
			workdir := newWorkdir(t, testConfig)
			output := filepath.Join(workdir, "output")
			if err := os.Mkdir(output, 0o755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(output, "osm.json")
			if err := os.WriteFile(path, []byte("previous result"), 0o600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			sentinel := errors.New("download unavailable")
			fetcher := &fakeFetcher{}
			switch stage {
			case "fetch":
				fetcher.err = sentinel
			case "encoding":
				fetcher.data.Version = math.NaN()
			case "cancellation":
				cancel()
			}
			err := run(ctx, []string{"download", "--workdir", workdir}, &bytes.Buffer{}, fetcher)
			if err == nil {
				t.Fatal("expected an error")
			}
			if stage == "fetch" && !errors.Is(err, sentinel) {
				t.Fatalf("original fetch error was lost: %v", err)
			}
			if stage == "cancellation" && !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation was lost: %v", err)
			}
			if string(readFile(t, path)) != "previous result" {
				t.Fatal("previous result was modified")
			}
			entries, err := os.ReadDir(output)
			if err != nil || len(entries) != 1 {
				t.Fatalf("temporary output was not cleaned up: entries=%v, error=%v", entries, err)
			}
		})
	}
}

func TestCommandValidation(t *testing.T) {
	for _, args := range [][]string{
		nil,
		{"unknown"},
		{"download"},
		{"download", "--unknown"},
		{"download", "--workdir", t.TempDir(), "extra"},
		{"download", "--workdir", t.TempDir()},
		{"download", "--workdir", newWorkdir(t, "schema_version: 1\nouput_dir: output\n")},
	} {
		fetcher := &fakeFetcher{}
		if err := run(context.Background(), args, &bytes.Buffer{}, fetcher); err == nil || fetcher.calls != 0 {
			t.Fatalf("args=%v, error=%v, requests=%d", args, err, fetcher.calls)
		}
	}
}

func TestCommandHelp(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"download", "--help"}} {
		var out bytes.Buffer
		fetcher := &fakeFetcher{}
		err := run(context.Background(), args, &out, fetcher)
		if err != nil || fetcher.calls != 0 || !strings.Contains(out.String(), "--workdir") {
			t.Fatalf("args=%v, output=%q, error=%v, requests=%d", args, out.String(), err, fetcher.calls)
		}
	}
}
