//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// ApplyUpdate downloads the new .exe, replaces the current one, and relaunches.
// Windows update flow:
// 1. Download zip from GitHub Release
// 2. Extract to temp dir next to current exe
// 3. Rename current exe to .old (Windows allows renaming a running exe)
// 4. Move new exe into place
// 5. Spawn relaunch command
// 6. Exit current process
func ApplyUpdate(downloadURL string) error {
	if downloadURL == "" {
		return fmt.Errorf("no download URL provided")
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine executable path: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("cannot resolve executable path: %w", err)
	}

	exeDir := filepath.Dir(exePath)
	exeName := filepath.Base(exePath)

	// Download the zip to a temp file next to the current exe
	tmpZip := filepath.Join(exeDir, ".contrails-update.zip")
	defer os.Remove(tmpZip)

	if err := downloadFile(tmpZip, downloadURL); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	// Extract to a temp dir
	tmpDir := filepath.Join(exeDir, ".contrails-update-tmp")
	os.RemoveAll(tmpDir)
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return fmt.Errorf("cannot create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := extractZip(tmpZip, tmpDir); err != nil {
		return fmt.Errorf("extraction failed: %w", err)
	}

	// Find the new exe in the extracted dir
	newExePath, err := findExeInDir(tmpDir, exeName)
	if err != nil {
		return fmt.Errorf("no executable found in archive: %w", err)
	}

	// Windows allows renaming a running executable.
	// Rename current exe → .old, then move new exe into place.
	oldPath := exePath + ".old"
	os.Remove(oldPath) // Clean up any previous failed update

	if err := os.Rename(exePath, oldPath); err != nil {
		return fmt.Errorf("cannot rename current executable: %w", err)
	}

	if err := os.Rename(newExePath, exePath); err != nil {
		// Rollback
		_ = os.Rename(oldPath, exePath)
		return fmt.Errorf("cannot place new executable: %w", err)
	}

	// Relaunch the new executable
	cmd := exec.Command(exePath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x00000008, // DETACHED_PROCESS
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("relaunch failed: %w", err)
	}

	os.Exit(0)
	return nil // unreachable
}

// CleanupOldUpdate removes leftover files from a previous update.
// Call during startup.
func CleanupOldUpdate() {
	exePath, err := os.Executable()
	if err != nil {
		return
	}
	exePath, _ = filepath.EvalSymlinks(exePath)
	exeDir := filepath.Dir(exePath)

	os.Remove(exePath + ".old")
	os.Remove(filepath.Join(exeDir, ".contrails-update.zip"))
	os.RemoveAll(filepath.Join(exeDir, ".contrails-update-tmp"))
}

// findExeInDir finds an executable file in the given directory, preferring
// one that matches the expected name.
func findExeInDir(dir, preferredName string) (string, error) {
	// First look for the preferred name
	preferred := filepath.Join(dir, preferredName)
	if _, err := os.Stat(preferred); err == nil {
		return preferred, nil
	}

	// Look for any .exe file
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".exe" {
			return filepath.Join(dir, entry.Name()), nil
		}
	}

	// Check one level deeper
	for _, entry := range entries {
		if entry.IsDir() {
			subEntries, err := os.ReadDir(filepath.Join(dir, entry.Name()))
			if err != nil {
				continue
			}
			for _, sub := range subEntries {
				if !sub.IsDir() && filepath.Ext(sub.Name()) == ".exe" {
					return filepath.Join(dir, entry.Name(), sub.Name()), nil
				}
			}
		}
	}

	return "", fmt.Errorf("no .exe found")
}
