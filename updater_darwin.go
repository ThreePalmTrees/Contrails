//go:build darwin

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ApplyUpdate downloads the new .app bundle, replaces the current one, and relaunches.
// This is the full .app bundle replacement flow:
// 1. Download zip from GitHub Release
// 2. Extract to temp dir (on same volume)
// 3. Strip quarantine attribute
// 4. Rename current .app to .app.old
// 5. Rename new .app into place
// 6. Spawn relaunch command
// 7. Exit current process
func ApplyUpdate(downloadURL string) error {
	if downloadURL == "" {
		return fmt.Errorf("no download URL provided")
	}

	// Find our own .app bundle path
	bundlePath, err := findBundlePath()
	if err != nil {
		return fmt.Errorf("cannot determine app bundle path: %w", err)
	}

	bundleDir := filepath.Dir(bundlePath)
	bundleName := filepath.Base(bundlePath)

	// Download the zip to a temp file on the same volume
	tmpZip := filepath.Join(bundleDir, ".contrails-update.zip")
	defer os.Remove(tmpZip)

	if err := downloadFile(tmpZip, downloadURL); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	// Extract to a temp dir on the same volume (required for atomic rename)
	tmpDir := filepath.Join(bundleDir, ".contrails-update-tmp")
	os.RemoveAll(tmpDir)
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return fmt.Errorf("cannot create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := extractZip(tmpZip, tmpDir); err != nil {
		return fmt.Errorf("extraction failed: %w", err)
	}

	// Find the .app inside the extracted dir
	newAppPath, err := findAppInDir(tmpDir)
	if err != nil {
		return fmt.Errorf("no .app found in archive: %w", err)
	}

	// Strip quarantine attribute (critical for macOS Gatekeeper)
	_ = exec.Command("xattr", "-dr", "com.apple.quarantine", newAppPath).Run()

	// Atomic swap: rename current → .old, rename new → current
	oldPath := bundlePath + ".old"
	os.RemoveAll(oldPath) // Clean up any previous failed update

	if err := os.Rename(bundlePath, oldPath); err != nil {
		return fmt.Errorf("cannot move current app aside: %w", err)
	}

	targetPath := filepath.Join(bundleDir, bundleName)
	if err := os.Rename(newAppPath, targetPath); err != nil {
		// Rollback: restore old app
		_ = os.Rename(oldPath, bundlePath)
		return fmt.Errorf("cannot place new app: %w", err)
	}

	// Clean up old version
	os.RemoveAll(oldPath)

	// Relaunch: spawn detached process that opens the new bundle, then exit
	cmd := exec.Command("/usr/bin/open", "-n", targetPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("relaunch failed: %w", err)
	}

	os.Exit(0)
	return nil // unreachable
}

// CleanupOldUpdate removes any .app.old left from a previous update.
// Call during startup.
func CleanupOldUpdate() {
	bundlePath, err := findBundlePath()
	if err != nil {
		return
	}
	os.RemoveAll(bundlePath + ".old")
}

// findBundlePath returns the path to the running .app bundle.
// e.g., /Applications/Contrails.app
func findBundlePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}

	// Resolve symlinks
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", err
	}

	// exe is typically /path/to/Contrails.app/Contents/MacOS/contrails
	// Walk up to find the .app directory
	dir := exe
	for i := 0; i < 5; i++ {
		dir = filepath.Dir(dir)
		if strings.HasSuffix(dir, ".app") {
			return dir, nil
		}
	}

	return "", fmt.Errorf("not running inside a .app bundle: %s", exe)
}

// findAppInDir finds a .app directory inside the given directory.
func findAppInDir(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	for _, entry := range entries {
		if entry.IsDir() && strings.HasSuffix(entry.Name(), ".app") {
			return filepath.Join(dir, entry.Name()), nil
		}
	}

	// Check one level deeper (some zips have a top-level folder)
	for _, entry := range entries {
		if entry.IsDir() {
			subEntries, err := os.ReadDir(filepath.Join(dir, entry.Name()))
			if err != nil {
				continue
			}
			for _, sub := range subEntries {
				if sub.IsDir() && strings.HasSuffix(sub.Name(), ".app") {
					return filepath.Join(dir, entry.Name(), sub.Name()), nil
				}
			}
		}
	}

	return "", fmt.Errorf("no .app found")
}
