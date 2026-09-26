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
//
// Windows has not been checked against a real install; the OpenCode source
// has no Windows-specific override, so the same rule is used there.
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
// database.ts). Without it, the stable opencode.db is used whenever it
// exists. Non-stable channels write opencode-<channel>.db; one of those is
// only used when there is no stable database at all.
func dbPath() (string, error) {
	override := os.Getenv("OPENCODE_DB")
	if filepath.IsAbs(override) || override == ":memory:" {
		return override, nil
	}

	dir, err := dataDir()
	if err != nil {
		return "", err
	}
	if override != "" {
		return filepath.Join(dir, override), nil
	}

	stable := filepath.Join(dir, "opencode.db")
	if _, err := os.Stat(stable); err == nil {
		return stable, nil
	}
	if channel := newestChannelDB(dir); channel != "" {
		return channel, nil
	}
	return stable, nil
}

// newestChannelDB returns the most recently modified opencode-<channel>.db
// in dir, or "" when there is none.
//
// The stable database is deliberately not compared by date: in WAL mode the
// main file's modification time only moves when the WAL is checkpointed, so
// a channel database touched once by a dev build can look newer than the
// database that is in daily use.
func newestChannelDB(dir string) string {
	candidates, err := filepath.Glob(filepath.Join(dir, "opencode-*.db"))
	if err != nil {
		return ""
	}

	var newest string
	var newestMod int64
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}
		if mod := info.ModTime().UnixNano(); newest == "" || mod > newestMod {
			newestMod = mod
			newest = candidate
		}
	}
	return newest
}
