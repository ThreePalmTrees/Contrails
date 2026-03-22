//go:build windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// vscodeWorkspaceStorageDir returns the default VS Code workspace storage directory.
func vscodeWorkspaceStorageDir() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return ""
	}
	return filepath.Join(appData, "Code", "User", "workspaceStorage")
}

// defaultFileManagerName returns the platform's file manager name.
func defaultFileManagerName() string {
	return "File Explorer"
}

// defaultOpenCommand returns the command to open a file/directory with the default handler.
func defaultOpenCommand() string {
	return "explorer"
}

// detectPlatformApps returns IDEOptions found via platform-specific methods
// (e.g., checking common install locations on Windows).
func detectPlatformApps(seen map[string]bool) []IDEOption {
	home, _ := os.UserHomeDir()
	programFiles := os.Getenv("ProgramFiles")

	// Common Windows install locations for IDEs
	appLocations := []struct {
		Name    string
		Command string
		Paths   []string
	}{
		{
			Name:    "VS Code",
			Command: "code",
			Paths: []string{
				filepath.Join(programFiles, "Microsoft VS Code", "Code.exe"),
				filepath.Join(home, "AppData", "Local", "Programs", "Microsoft VS Code", "Code.exe"),
			},
		},
		{
			Name:    "Cursor",
			Command: "cursor",
			Paths: []string{
				filepath.Join(home, "AppData", "Local", "Programs", "cursor", "Cursor.exe"),
				filepath.Join(programFiles, "Cursor", "Cursor.exe"),
			},
		},
	}

	var found []IDEOption
	for _, app := range appLocations {
		if seen[app.Name] {
			continue
		}
		for _, p := range app.Paths {
			if _, err := os.Stat(p); err == nil {
				seen[app.Name] = true
				found = append(found, IDEOption{
					Name:    app.Name,
					Command: app.Command,
				})
				break
			}
		}
	}
	return found
}

// openDirectory executes the given command to open a directory.
func openDirectory(dirPath, command string) error {
	if command == "" {
		command = "explorer"
	}
	return exec.Command("cmd", "/c", command, dirPath).Start()
}

// isPlatformAsset checks if a GitHub release asset name is for Windows.
func isPlatformAsset(name string) bool {
	name = strings.ToLower(name)
	return strings.Contains(name, "windows") || strings.Contains(name, "win64") || strings.Contains(name, "win32")
}
