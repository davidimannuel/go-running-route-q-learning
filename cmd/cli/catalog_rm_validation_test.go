package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCatalogRemovePreservesOtherRouteAndSource(t *testing.T) {
	workdir, directory := catalogFixture(t)
	for _, id := range []string{"first-route", "second-route"} {
		if err := runCatalog(context.Background(), catalogArgs(workdir, directory, catalogSeed), strings.NewReader(id+"\nA route note\n"), io.Discard, io.Discard); err != nil {
			t.Fatal(err)
		}
	}
	preserved := filepath.Join(directory, "routes/second-route")
	before := treeFiles(t, preserved)
	beforeMetadata := readFile(t, filepath.Join(preserved, "metadata.json"))
	source := filepath.Join(workdir, "output/training/b-saved")
	sourceMap := readFile(t, filepath.Join(source, "routes.html"))
	sourceMetadata := readFile(t, filepath.Join(source, "metadata.json"))
	var out bytes.Buffer
	if err := runCatalog(context.Background(), []string{"rm", "--dir", directory, "first-route"}, strings.NewReader(""), &out, io.Discard); err != nil {
		t.Fatal(err)
	}
	manifest, exists, err := loadCatalog(directory)
	if err != nil || !exists || len(manifest.Routes) != 1 || manifest.Routes[0].ID != "second-route" {
		t.Fatalf("manifest=%+v exists=%v err=%v", manifest, exists, err)
	}
	if _, err := os.Stat(filepath.Join(directory, "routes/first-route")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("removed route exists: %v", err)
	}
	if !reflect.DeepEqual(before, treeFiles(t, preserved)) || !bytes.Equal(beforeMetadata, readFile(t, filepath.Join(preserved, "metadata.json"))) {
		t.Fatal("unselected route changed")
	}
	index := string(readFile(t, filepath.Join(directory, "index.html")))
	if strings.Contains(index, "first-route") || !strings.Contains(index, "second-route") || !strings.Contains(out.String(), "Removed catalog route: first-route") {
		t.Fatal("index/output not updated")
	}
	if !bytes.Equal(sourceMap, readFile(t, filepath.Join(source, "routes.html"))) || !bytes.Equal(sourceMetadata, readFile(t, filepath.Join(source, "metadata.json"))) {
		t.Fatal("training source changed")
	}
	if err := runCatalog(context.Background(), []string{"rm", "--dir", directory, "second-route"}, strings.NewReader(""), io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	manifest, _, err = loadCatalog(directory)
	if err != nil || len(manifest.Routes) != 0 || !strings.Contains(string(readFile(t, filepath.Join(directory, "index.html"))), "No routes are available yet.") {
		t.Fatalf("last route removal failed: %v", err)
	}
}

func TestCatalogRemoveRejectsInvalidRequestsWithoutPublishing(t *testing.T) {
	workdir, directory := catalogFixture(t)
	if err := runCatalog(context.Background(), catalogArgs(workdir, directory, catalogSeed), strings.NewReader("keep-route\nA route note\n"), io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	before := treeFiles(t, directory)
	for _, args := range [][]string{
		{"rm"}, {"rm", "--dir", directory}, {"rm", "--dir", directory, "../escape"},
		{"rm", "--dir", directory, "missing"}, {"rm", "--dir", directory, "keep-route", "extra"},
		{"rm", "--unknown"},
	} {
		if err := runCatalog(context.Background(), args, strings.NewReader(""), io.Discard, io.Discard); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := runCatalog(ctx, []string{"rm", "--dir", directory, "keep-route"}, strings.NewReader(""), io.Discard, io.Discard); !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
	unlock, err := lockCatalog(directory)
	if err != nil {
		t.Fatal(err)
	}
	if err := runCatalog(context.Background(), []string{"rm", "--dir", directory, "keep-route"}, strings.NewReader(""), io.Discard, io.Discard); err == nil {
		t.Fatal("ignored active command lock")
	}
	unlock()
	if !reflect.DeepEqual(before, treeFiles(t, directory)) {
		t.Fatal("rejected request changed catalog")
	}
	var help bytes.Buffer
	if err := runCatalog(context.Background(), []string{"rm", "--help"}, strings.NewReader(""), &help, io.Discard); err != nil || !strings.Contains(help.String(), "cli catalog rm") {
		t.Fatalf("help error=%v", err)
	}
}
