//go:build linux

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// ApplyUpdate downloads the new binary, replaces the current one, and relaunches.
// Linux update flow:
// 1. Download zip from GitHub Release
// 2. Extract to temp dir next to current binary
// 3. Replace the current binary (Linux allows unlinking/overwriting a running binary's path)
// 4. Spawn relaunch command
// 5. Exit current process
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

	// Download the zip to a temp file next to the current binary
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

	// Find the new binary in the extracted dir
	newBinPath, err := findBinaryInDir(tmpDir, exeName)
	if err != nil {
		return fmt.Errorf("no executable found in archive: %w", err)
	}

	// Linux allows removing/overwriting a running binary's path — the kernel
	// keeps the old inode in memory until the process exits.
	// Remove current binary first, then move new one into place.
	oldPath := exePath + ".old"
	os.Remove(oldPath)

	if err := os.Rename(exePath, oldPath); err != nil {
		return fmt.Errorf("cannot move current binary aside: %w", err)
	}

	if err := os.Rename(newBinPath, exePath); err != nil {
		// Rollback
		_ = os.Rename(oldPath, exePath)
		return fmt.Errorf("cannot place new binary: %w", err)
	}

	// Ensure the new binary is executable
	_ = os.Chmod(exePath, 0755)

	// Clean up old binary
	os.Remove(oldPath)

	// Relaunch
	cmd := exec.Command(exePath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
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

// findBinaryInDir finds an executable file in the given directory, preferring
// one that matches the expected name.
func findBinaryInDir(dir, preferredName string) (string, error) {
	// First look for the preferred name
	preferred := filepath.Join(dir, preferredName)
	if info, err := os.Stat(preferred); err == nil && !info.IsDir() {
		return preferred, nil
	}

	// Look for any executable file
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.Mode()&0111 != 0 {
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
				if sub.IsDir() {
					continue
				}
				info, err := sub.Info()
				if err != nil {
					continue
				}
				if info.Mode()&0111 != 0 {
					return filepath.Join(dir, entry.Name(), sub.Name()), nil
				}
			}
		}
	}

	return "", fmt.Errorf("no executable found")
}
