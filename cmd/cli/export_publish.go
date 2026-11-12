package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// legacyExportSite only accepts the old export layout. Unknown files are kept
// rather than treating a directory named site as disposable user data.
func legacyExportSite(output, workdir string) (bool, error) {
	directory := filepath.Join(output, "site")
	info, err := os.Lstat(directory)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf("site is not a regular directory")
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return false, err
	}
	defer root.Close()
	manifest, err := readExportMetadata(root, "export.json")
	if err != nil {
		return false, fmt.Errorf("keep site: cannot verify old export: %w", err)
	}
	if manifest["command"] != "export" || manifest["schema_version"] != json.Number("1") || manifest["workdir"] != workdir {
		return false, fmt.Errorf("keep site: export manifest does not match this workdir")
	}
	allowed := map[string]bool{".": true, "index.html": true, "export.json": true}
	sources, ok := manifest["sources"].([]any)
	if !ok || len(sources) == 0 {
		return false, fmt.Errorf("keep site: export has no source list")
	}
	for _, raw := range sources {
		source, _ := raw.(map[string]any)
		name, _ := source["file"].(string)
		name = filepath.FromSlash(name)
		if !filepath.IsLocal(name) || filepath.Clean(name) != name {
			return false, fmt.Errorf("keep site: invalid exported path")
		}
		if name == "road-analysis.html" {
			allowed[name], allowed["analysis.html"] = true, true
		} else if filepath.Base(name) == "routes.html" && (filepath.Dir(name) == "training" || filepath.Dir(filepath.Dir(name)) == "training") {
			allowed[name], allowed[filepath.Join(filepath.Dir(name), "report.html")] = true, true
		} else {
			return false, fmt.Errorf("keep site: unexpected export source %s", name)
		}
		for parent := filepath.Dir(name); parent != "."; parent = filepath.Dir(parent) {
			allowed[parent] = true
		}
	}
	err = filepath.WalkDir(directory, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name, err := filepath.Rel(directory, path)
		if err != nil {
			return err
		}
		if !allowed[name] || entry.Type()&os.ModeSymlink != 0 || (!entry.IsDir() && !entry.Type().IsRegular()) {
			return fmt.Errorf("keep site: unexpected file %s; move it before exporting", name)
		}
		return nil
	})
	return err == nil, err
}

// publishExport replaces only prepared HTML/export.json. Backups allow ordinary
// publication errors or cancellation to roll back without touching research data.
func publishExport(ctx context.Context, output, stage string, removeSite bool) error {
	var names []string
	if err := filepath.WalkDir(stage, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			name, err := filepath.Rel(stage, path)
			if err != nil {
				return err
			}
			names = append(names, name)
		}
		return nil
	}); err != nil {
		return err
	}
	root, err := os.OpenRoot(output)
	if err != nil {
		return err
	}
	defer root.Close()
	backup, err := os.MkdirTemp(output, ".export-backup-*")
	if err != nil {
		return err
	}
	backupName, stageName := filepath.Base(backup), filepath.Base(stage)
	var replaced, saved []string
	rollback := func(cause error) error {
		var restoreErr error
		for i := len(replaced) - 1; i >= 0; i-- {
			restoreErr = errors.Join(restoreErr, root.Remove(replaced[i]))
		}
		for i := len(saved) - 1; i >= 0; i-- {
			name := saved[i]
			restoreErr = errors.Join(restoreErr, root.Rename(filepath.Join(backupName, name), name))
		}
		if restoreErr != nil {
			return errors.Join(cause, fmt.Errorf("restore export failed; remaining backups in %s: %w", backup, restoreErr))
		}
		return errors.Join(cause, os.RemoveAll(backup))
	}
	// Move each old file aside before its replacement. Only the verified legacy
	// site directory may be removed; it is restored too if publication fails.
	if removeSite {
		if err := root.Rename("site", filepath.Join(backupName, "site")); err != nil {
			return rollback(err)
		}
		saved = append(saved, "site")
	}
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return rollback(err)
		}
		info, err := root.Lstat(name)
		if err == nil {
			if !info.Mode().IsRegular() {
				return rollback(fmt.Errorf("destination %s is not a regular file", name))
			}
			if err := root.MkdirAll(filepath.Join(backupName, filepath.Dir(name)), 0o755); err != nil {
				return rollback(err)
			}
			if err := root.Rename(name, filepath.Join(backupName, name)); err != nil {
				return rollback(err)
			}
			saved = append(saved, name)
		} else if !errors.Is(err, os.ErrNotExist) {
			return rollback(err)
		}
		if err := root.Rename(filepath.Join(stageName, name), name); err != nil {
			return rollback(err)
		}
		replaced = append(replaced, name)
	}
	if err := ctx.Err(); err != nil {
		return rollback(err)
	}
	return os.RemoveAll(backup)
}
