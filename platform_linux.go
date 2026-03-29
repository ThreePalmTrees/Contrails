//go:build linux

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// vscodeWorkspaceStorageDir returns the default VS Code workspace storage directory.
func vscodeWorkspaceStorageDir() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(configDir, "Code", "User", "workspaceStorage")
}

// defaultFileManagerName returns the platform's file manager name.
func defaultFileManagerName() string {
	return "File Manager"
}

// defaultOpenCommand returns the command to open a file/directory with the default handler.
func defaultOpenCommand() string {
	return "xdg-open"
}

// detectPlatformApps returns IDEOptions found via platform-specific methods
// (e.g., checking common install locations and .desktop files on Linux).
func detectPlatformApps(seen map[string]bool) []IDEOption {
	appLocations := []struct {
		Name    string
		Command string
		Paths   []string
	}{
		{
			Name:    "VS Code",
			Command: "code",
			Paths: []string{
				"/usr/bin/code",
				"/usr/local/bin/code",
				"/snap/bin/code",
			},
		},
		{
			Name:    "Cursor",
			Command: "cursor",
			Paths: []string{
				"/usr/bin/cursor",
				"/usr/local/bin/cursor",
			},
		},
		{
			Name:    "Zed",
			Command: "zed",
			Paths: []string{
				"/usr/bin/zed",
				"/usr/local/bin/zed",
			},
		},
	}

	home, _ := os.UserHomeDir()
	if home != "" {
		for i := range appLocations {
			appLocations[i].Paths = append(appLocations[i].Paths,
				filepath.Join(home, ".local", "bin", appLocations[i].Command),
			)
		}
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
		command = "xdg-open"
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	return exec.Command(shell, "-c", command+" "+shellescape(dirPath)).Start()
}

// isPlatformAsset checks if a GitHub release asset name is for Linux.
func isPlatformAsset(name string) bool {
	name = strings.ToLower(name)
	return strings.Contains(name, "linux")
}

// shellescape wraps a string in single quotes for safe shell usage.
func shellescape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
