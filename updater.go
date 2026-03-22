package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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

	// Find the platform-specific zip asset
	downloadURL := ""
	for _, asset := range release.Assets {
		name := strings.ToLower(asset.Name)
		if isPlatformAsset(name) && strings.HasSuffix(name, ".zip") {
			downloadURL = asset.BrowserDownloadURL
			break
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

// ApplyUpdate is defined in updater_darwin.go / updater_windows.go
// CleanupOldUpdate is defined in updater_darwin.go / updater_windows.go
// isPlatformAsset is defined in platform_darwin.go / platform_windows.go

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
