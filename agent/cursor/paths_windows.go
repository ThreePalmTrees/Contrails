//go:build windows

package cursor

import (
	"fmt"
	"os"
	"path/filepath"
)

// globalStorageDir returns the directory containing Cursor's global state.vscdb.
func globalStorageDir() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return ""
	}
	return filepath.Join(appData, "Cursor", "User", "globalStorage")
}

// workspaceStorageDir returns the path to Cursor's workspaceStorage directory.
func workspaceStorageDir() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return "", fmt.Errorf("APPDATA environment variable not set")
	}
	return filepath.Join(appData, "Cursor", "User", "workspaceStorage"), nil
}

// dbPath returns the absolute path to the Cursor state database.
func dbPath() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return "", fmt.Errorf("APPDATA environment variable not set")
	}
	return filepath.Join(appData, "Cursor", "User", "globalStorage", "state.vscdb"), nil
}
