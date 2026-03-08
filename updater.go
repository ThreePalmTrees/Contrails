package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// GitHubRepo is the owner/repo for update checks.
const GitHubRepo = "ThreePalmTrees/Contrails"

// UpdateInfo holds information about an available update.
type UpdateInfo struct {
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	ReleaseURL     string `json:"releaseURL"`
	DownloadURL    string `json:"downloadURL"`
	ReleaseNotes   string `json:"releaseNotes"`
}

// githubRelease is the subset of GitHub API response we need.
type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Body    string `json:"body"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// CheckForUpdate queries GitHub Releases API and returns update info if a newer version exists.
// Returns nil, nil if already up to date or if the check fails silently.
func CheckForUpdate() (*UpdateInfo, error) {
	if Version == "dev" {
		return nil, nil
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", GitHubRepo))
	if err != nil {
		return nil, nil // Network failure — silent
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil // API error — silent
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, nil
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	currentVersion := strings.TrimPrefix(Version, "v")

	if !isNewerVersion(latestVersion, currentVersion) {
		return nil, nil // Up to date
	}

	// Find the macOS zip asset
	downloadURL := ""
	for _, asset := range release.Assets {
		name := strings.ToLower(asset.Name)
		if strings.Contains(name, "darwin") || strings.Contains(name, "macos") || strings.Contains(name, "mac") {
			if strings.HasSuffix(name, ".zip") {
				downloadURL = asset.BrowserDownloadURL
				break
			}
		}
	}

	// Fallback: if only one .zip asset, use it
	if downloadURL == "" {
		for _, asset := range release.Assets {
			if strings.HasSuffix(strings.ToLower(asset.Name), ".zip") {
				downloadURL = asset.BrowserDownloadURL
				break
			}
		}
	}

	return &UpdateInfo{
		CurrentVersion: currentVersion,
		LatestVersion:  latestVersion,
		ReleaseURL:     release.HTMLURL,
		DownloadURL:    downloadURL,
		ReleaseNotes:   release.Body,
	}, nil
}

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

// downloadFile downloads a URL to a local file path.
func downloadFile(dest, url string) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}

// extractZip extracts a zip file to a destination directory.
func extractZip(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		target := filepath.Join(destDir, f.Name) //nolint:gosec

		// Prevent zip slip
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path in zip: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, f.Mode()); err != nil {
				return err
			}
			continue
		}

		// Ensure parent dir exists
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}

		outFile, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()
		if err != nil {
			return err
		}
	}
	return nil
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

// isNewerVersion compares two semver strings (without "v" prefix).
// Returns true if latest > current.
func isNewerVersion(latest, current string) bool {
	latestParts := parseSemver(latest)
	currentParts := parseSemver(current)

	for i := 0; i < 3; i++ {
		if latestParts[i] > currentParts[i] {
			return true
		}
		if latestParts[i] < currentParts[i] {
			return false
		}
	}
	return false
}

// parseSemver parses "1.2.3" into [1, 2, 3]. Returns [0,0,0] on failure.
func parseSemver(v string) [3]int {
	var parts [3]int
	n := 0
	current := 0
	for _, ch := range v {
		if ch == '.' {
			if n < 3 {
				parts[n] = current
			}
			n++
			current = 0
		} else if ch >= '0' && ch <= '9' {
			current = current*10 + int(ch-'0')
		} else {
			break // Stop at prerelease tags like -beta
		}
	}
	if n < 3 {
		parts[n] = current
	}
	return parts
}
