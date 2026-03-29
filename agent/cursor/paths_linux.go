//go:build linux

package cursor

import (
	"fmt"
	"os"
	"path/filepath"
)

// globalStorageDir returns the directory containing Cursor's global state.vscdb.
func globalStorageDir() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(configDir, "Cursor", "User", "globalStorage")
}

// workspaceStorageDir returns the path to Cursor's workspaceStorage directory.
func workspaceStorageDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolving config directory: %w", err)
	}
	return filepath.Join(configDir, "Cursor", "User", "workspaceStorage"), nil
}

// dbPath returns the absolute path to the Cursor state database.
func dbPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolving config directory: %w", err)
	}
	return filepath.Join(configDir, "Cursor", "User", "globalStorage", "state.vscdb"), nil
}
