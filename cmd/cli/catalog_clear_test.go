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

func TestCatalogClearResetsCompleteAndBrokenCatalogs(t *testing.T) {
	for _, state := range []string{"complete", "missing manifest", "broken manifest", "missing map", "empty", "missing directory"} {
		t.Run(state, func(t *testing.T) {
			workdir, directory := catalogFixture(t)
			source := treeFiles(t, filepath.Join(workdir, "output"))
			if state != "missing directory" {
				if err := runCatalog(context.Background(), catalogArgs(workdir, directory, catalogSeed), strings.NewReader("old-route\nOld note\n"), io.Discard, io.Discard); err != nil {
					t.Fatal(err)
				}
				switch state {
				case "missing manifest":
					if err := os.Remove(filepath.Join(directory, "catalog.json")); err != nil {
						t.Fatal(err)
					}
				case "broken manifest":
					if err := os.WriteFile(filepath.Join(directory, "catalog.json"), []byte("{broken"), 0600); err != nil {
						t.Fatal(err)
					}
				case "missing map":
					if err := os.Remove(filepath.Join(directory, "routes/old-route/index.html")); err != nil {
						t.Fatal(err)
					}
				case "empty":
					if err := os.RemoveAll(directory); err != nil {
						t.Fatal(err)
					}
					if err := os.Mkdir(directory, 0755); err != nil {
						t.Fatal(err)
					}
				}
			}
			var out bytes.Buffer
			if err := runCatalog(context.Background(), []string{"clear", "--dir", directory}, strings.NewReader(""), &out, io.Discard); err != nil {
				t.Fatal(err)
			}
			manifest, exists, err := loadCatalog(directory)
			if err != nil || !exists || len(manifest.Routes) != 0 {
				t.Fatalf("manifest=%+v exists=%v err=%v", manifest, exists, err)
			}
			html := string(readFile(t, filepath.Join(directory, "index.html")))
			if !strings.Contains(html, "No routes are available yet.") || strings.Contains(html, "old-route") || !strings.Contains(out.String(), "Catalog cleared") {
				t.Fatal("index or output did not reset")
			}
			if !reflect.DeepEqual(source, treeFiles(t, filepath.Join(workdir, "output"))) {
				t.Fatal("clear changed training results")
			}
			entries, err := os.ReadDir(filepath.Join(directory, "routes"))
			if err != nil || len(entries) != 0 {
				t.Fatalf("routes=%v err=%v", entries, err)
			}
			if _, err := os.Stat(filepath.Join(directory, ".nojekyll")); err != nil {
				t.Fatal(err)
			}
			// A cleared catalog must accept a new route, including a previously used ID.
			if err := runCatalog(context.Background(), catalogArgs(workdir, directory, catalogSeed), strings.NewReader("old-route\nNew note\n"), io.Discard, io.Discard); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCatalogClearProtectsOtherDirectoriesAndActiveCommands(t *testing.T) {
	for _, kind := range []string{"repository", "workdir", "nested data", "symlink", "lock", "canceled"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("keep"), 0600); err != nil {
				t.Fatal(err)
			}
			target := dir
			ctx := context.Background()
			switch kind {
			case "repository":
				if err := os.Mkdir(filepath.Join(dir, ".git"), 0755); err != nil {
					t.Fatal(err)
				}
			case "workdir":
				if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("keep"), 0600); err != nil {
					t.Fatal(err)
				}
			case "nested data":
				if err := os.MkdirAll(filepath.Join(dir, "routes/old/input"), 0755); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				target = filepath.Join(t.TempDir(), "catalog-link")
				if err := os.Symlink(dir, target); err != nil {
					t.Fatal(err)
				}
			case "lock":
				if err := os.WriteFile(filepath.Join(filepath.Dir(dir), "."+filepath.Base(dir)+".lock"), nil, 0600); err != nil {
					t.Fatal(err)
				}
			case "canceled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			err := runCatalog(ctx, []string{"clear", "--dir", target}, strings.NewReader(""), io.Discard, io.Discard)
			if err == nil {
				t.Fatal("unsafe clear succeeded")
			}
			if kind == "canceled" && !errors.Is(err, context.Canceled) {
				t.Fatalf("err=%v", err)
			}
			if string(readFile(t, filepath.Join(dir, "index.html"))) != "keep" {
				t.Fatal("failed clear changed files")
			}
		})
	}
}
