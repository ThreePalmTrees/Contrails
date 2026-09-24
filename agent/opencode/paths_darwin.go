//go:build darwin

package opencode

import (
	"fmt"
	"os"
	"path/filepath"
)

// dataDir returns the OpenCode data directory. OpenCode resolves this the
// same way on every platform: $XDG_DATA_HOME if set, else ~/.local/share
// (packages/core/src/global.ts). Note it is not ~/.opencode, which only
// holds bin/.
func dataDir() (string, error) {
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return filepath.Join(v, "opencode"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	return filepath.Join(home, ".local", "share", "opencode"), nil
}

// dbPath returns the absolute path to the OpenCode database.
//
// OPENCODE_DB overrides the path: an absolute value or ":memory:" is used
// as-is, a relative value is joined to the data directory (matching
// database.ts). Without it, the stable filename is opencode.db; non-stable
// channels use opencode-<channel>.db, so every matching candidate is probed
// and the newest file wins.
func dbPath() (string, error) {
	if v := os.Getenv("OPENCODE_DB"); v != "" {
		if filepath.IsAbs(v) || v == ":memory:" {
			return v, nil
		}
		dir, err := dataDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, v), nil
	}

	dir, err := dataDir()
	if err != nil {
		return "", err
	}
	return newestDBFile(dir), nil
}

// newestDBFile returns the most recently modified opencode database in dir,
// or the stable opencode.db path when no candidate exists yet.
func newestDBFile(dir string) string {
	stable := filepath.Join(dir, "opencode.db")
	candidates, err := filepath.Glob(filepath.Join(dir, "opencode-*.db"))
	if err != nil || len(candidates) == 0 {
		return stable
	}
	candidates = append(candidates, stable)

	newest := stable
	var newestMod int64 = -1
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err != nil {
			continue
		}
		if mod := info.ModTime().UnixNano(); mod > newestMod {
			newestMod = mod
			newest = candidate
		}
	}
	return newest
}
