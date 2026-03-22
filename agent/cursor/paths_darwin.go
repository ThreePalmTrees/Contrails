//go:build darwin

package cursor

import (
	"fmt"
	"os"
	"path/filepath"
)

// globalStorageDir returns the directory containing Cursor's global state.vscdb.
func globalStorageDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Library", "Application Support", "Cursor", "User", "globalStorage")
}

// workspaceStorageDir returns the path to Cursor's workspaceStorage directory.
func workspaceStorageDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	return filepath.Join(
		home, "Library", "Application Support",
		"Cursor", "User", "workspaceStorage",
	), nil
}

// dbPath returns the absolute path to the Cursor state database.
func dbPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	return filepath.Join(
		home, "Library", "Application Support",
		"Cursor", "User", "globalStorage", "state.vscdb",
	), nil
}
