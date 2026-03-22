//go:build darwin

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// vscodeWorkspaceStorageDir returns the default VS Code workspace storage directory.
func vscodeWorkspaceStorageDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Application Support", "Code", "User", "workspaceStorage")
}

// defaultFileManagerName returns the platform's file manager name.
func defaultFileManagerName() string {
	return "Finder"
}

// defaultOpenCommand returns the command to open a file/directory with the default handler.
func defaultOpenCommand() string {
	return "open"
}

// detectPlatformApps returns IDEOptions found via platform-specific methods
// (e.g., .app bundles in /Applications on macOS).
func detectPlatformApps(seen map[string]bool) []IDEOption {
	appBundles := []struct {
		Name    string
		Bundles []string
	}{
		{Name: "VS Code", Bundles: []string{"Visual Studio Code"}},
		{Name: "Cursor", Bundles: []string{"Cursor"}},
		{Name: "Zed", Bundles: []string{"Zed"}},
		{Name: "WebStorm", Bundles: []string{"WebStorm"}},
		{Name: "Antigravity", Bundles: []string{"Antigravity"}},
	}

	var found []IDEOption
	for _, ab := range appBundles {
		if seen[ab.Name] {
			continue
		}
		for _, bundle := range ab.Bundles {
			appPath := filepath.Join("/Applications", bundle+".app")
			if _, err := os.Stat(appPath); err == nil {
				seen[ab.Name] = true
				found = append(found, IDEOption{
					Name:    ab.Name,
					Command: "open -a \"" + bundle + "\"",
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
		command = "open"
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	return exec.Command(shell, "-ic", command+" "+shellescape(dirPath)).Start()
}

// isPlatformAsset checks if a GitHub release asset name is for macOS.
func isPlatformAsset(name string) bool {
	name = strings.ToLower(name)
	return strings.Contains(name, "darwin") || strings.Contains(name, "macos") || strings.Contains(name, "mac")
}

// shellescape wraps a string in single quotes for safe shell usage.
func shellescape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
