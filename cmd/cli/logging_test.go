package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/davidimannuel/go-running-route-q-learning/internal/osm"
)

func TestDownloadLogging(t *testing.T) {
	for _, tc := range []struct {
		name  string
		flags []string
		debug bool
	}{
		{name: "default info"},
		{name: "debug enabled", flags: []string{"--debug"}, debug: true},
		{name: "debug explicitly disabled", flags: []string{"--debug=false"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			workdir := newWorkdir(t, testConfig)
			args := append([]string{"download", "--workdir", workdir}, tc.flags...)
			fetcher := &fakeFetcher{data: osm.OSMData{Version: 0.6, Elements: []osm.Element{}}}
			for _, source := range []string{"Fetched OSM data from Overpass", "Loaded OSM data from cache"} {
				var stdout, logs bytes.Buffer
				if err := run(context.Background(), args, &stdout, &logs, fetcher); err != nil {
					t.Fatal(err)
				}
				if stdout.Len() != 0 {
					t.Fatalf("progress leaked into stdout: %s", stdout.String())
				}
				debugSeen := false
				scanner := bufio.NewScanner(strings.NewReader(logs.String()))
				for scanner.Scan() {
					var entry map[string]any
					if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
						t.Fatal(err)
					}
					debugSeen = debugSeen || entry["level"] == "DEBUG"
				}
				if err := scanner.Err(); err != nil {
					t.Fatal(err)
				}
				if debugSeen != tc.debug {
					t.Fatalf("debug logs=%v; want %v: %s", debugSeen, tc.debug, logs.String())
				}
				for _, message := range []string{"Map preparation started", source, "Saved OSM data"} {
					if !strings.Contains(logs.String(), message) {
						t.Fatalf("missing progress message %q: %s", message, logs.String())
					}
				}
				if tc.debug && !strings.Contains(logs.String(), "Reading OSM cache") {
					t.Fatalf("cache did not receive the command logger: %s", logs.String())
				}
			}
			if fetcher.calls != 1 {
				t.Fatalf("requests=%d; want one request followed by a cache hit", fetcher.calls)
			}
		})
	}
}

func TestHelpDoesNotWriteLogs(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"download", "--help"}, {"download", "--debug", "--help"}} {
		var stdout, logs bytes.Buffer
		if err := run(context.Background(), args, &stdout, &logs, &fakeFetcher{}); err != nil {
			t.Fatal(err)
		}
		if logs.Len() != 0 || !strings.Contains(stdout.String(), "--debug") {
			t.Fatalf("stdout=%s, logs=%s", stdout.String(), logs.String())
		}
	}
}
