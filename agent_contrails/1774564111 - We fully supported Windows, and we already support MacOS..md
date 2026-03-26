- **Session ID:** `00bef500-32bf-4f00-a4fc-eb144f69fe68`
- **Created:** 2026-03-26 22:28:31
- **Last Message:** 2026-03-26 22:34:28
- **User:** User
- **Agent:** Claude Code
- **Model:** claude-opus-4-6

---

## 🧑 User - 2026-03-26 22:29:42

We fully supported Windows, and we already support MacOS.
Now we need to research what needs to be done in order to support Linux as well.
Please read the previous spike we did for supporting Windows, and research what's needed for supporting Linux.
This is mainly a detailed research task rather than actual implementation (will follow later based on this analysis)

@"agent_contrails/1772990949 - Right now we support Mac only (universal, both apple silicon and intel).md"

---

## 🤖 Assistant - 2026-03-26 22:29:48

*Model: claude-opus-4-6*

I'll research Linux support requirements by examining the current cross-platform code and researching Wails v2 Linux specifics in parallel.

<details>
<summary>Tool Calls</summary>

- **Agent**: `Agent: Find platform-specific code paths`
- **Agent**: `Agent: Research Wails v2 Linux support`
- **WebSearch**: `Search `Wails v2 Linux requirements webkit2gtk dependencies installation``
- **WebSearch**: `Search `Wails v2 Linux build guide distribution packaging AppImage deb rpm``
- **WebSearch**: `Search `Wails v2 Linux known issues limitations compared macOS Windows``
- **WebSearch**: `Search `Wails v2 Linux CI GitHub Actions ubuntu webkit2gtk build``
- **WebSearch**: `Search `Linux XDG Base Directory specification app config data path``
- **WebSearch**: `Search `Linux desktop app auto-update strategy AppImage Electron alternative``
- **WebSearch**: `Search `Wails v2 Linux window options frameless transparent titlebar styling``
- **WebSearch**: `Search `Wails v2 nfpm linux packaging deb rpm configuration``
- **WebSearch**: `Search `Cursor editor VS Code Linux config path ~/.config location``
- **WebFetch**: `Fetch `https://github.com/dAppServer/wails-build-action``
- **WebFetch**: `Fetch `https://deepwiki.com/wailsapp/wails/2.1-installation``
- **WebFetch**: `Fetch `https://docs.appimage.org/packaging-guide/optional/updates.html``
- **WebSearch**: `Search `Wails v2 Linux distro support page supported distributions``
- **WebSearch**: `Search `wails v2 linux appimage packaging guide tutorial``
- **WebFetch**: `Fetch `https://github.com/wailsapp/wails/pull/4481``
- **WebFetch**: `Fetch `https://github.com/wailsapp/wails/issues/1923``
- **WebFetch**: `Fetch `https://blog.jamesreed.dev/packaging-apps-for-linux-with-appimage``
- **WebSearch**: `Search `Wails v2 Wayland support issues linux window manager``
- **WebSearch**: `Search `Go os.UserConfigDir os.UserCacheDir Linux XDG path``

</details>

Perfect! Now I have enough information. Let me create a comprehensive report of the cross-platform work done in the codebase:

## Comprehensive Cross-Platform Analysis Report

I've thoroughly explored the Contrails codebase. Here's a complete breakdown of all cross-platform work already implemented for Windows and what needs to be done for Linux support:

### FILES WITH BUILD TAGS (Platform-Specific)

**Platform-Specific Files Found:**
1. `/Users/admin/Documents/projects/contrails/platform_darwin.go` (//go:build darwin)
2. `/Users/admin/Documents/projects/contrails/platform_windows.go` (//go:build windows)
3. `/Users/admin/Documents/projects/contrails/updater_darwin.go` (//go:build darwin)
4. `/Users/admin/Documents/projects/contrails/updater_windows.go` (//go:build windows)
5. `/Users/admin/Documents/projects/contrails/agent/cursor/paths_darwin.go` (//go:build darwin)
6. `/Users/admin/Documents/projects/contrails/agent/cursor/paths_windows.go` (//go:build windows)
7. `/Users/admin/Documents/projects/contrails/agent/claudecode/hook_command_darwin.go` (//go:build darwin)
8. `/Users/admin/Documents/projects/contrails/agent/claudecode/hook_command_windows.go` (//go:build windows)

**No Linux-specific files exist yet.**

---

### 1. PLATFORM-SPECIFIC PATH RESOLUTION CODE

#### Cursor Agent Paths

**macOS** (`paths_darwin.go`):
- `globalStorageDir()`: Returns `~/Library/Application Support/Cursor/User/globalStorage`
- `workspaceStorageDir()`: Returns `~/Library/Application Support/Cursor/User/workspaceStorage`
- `dbPath()`: Returns `~/Library/Application Support/Cursor/User/globalStorage/state.vscdb`

**Windows** (`paths_windows.go`):
- `globalStorageDir()`: Returns `%APPDATA%/Cursor/User/globalStorage` (reads APPDATA env var)
- `workspaceStorageDir()`: Returns `%APPDATA%/Cursor/User/workspaceStorage` (reads APPDATA env var)
- `dbPath()`: Returns `%APPDATA%/Cursor/User/globalStorage/state.vscdb`

**Linux Needs:**
- Linux Cursor typically stores in `~/.config/Cursor/User/globalStorage` and `~/.config/Cursor/User/workspaceStorage` (XDG Base Directory Specification)

#### Main App Platform Detection Functions (`platform_*.go`)

**macOS** (`platform_darwin.go`):
- `vscodeWorkspaceStorageDir()`: `~/Library/Application Support/Code/User/workspaceStorage`
- `defaultFileManagerName()`: Returns "Finder"
- `defaultOpenCommand()`: Returns "open"
- `detectPlatformApps()`: Searches `/Applications` for `.app` bundles (VS Code, Cursor, Zed, WebStorm, Antigravity)
- `openDirectory()`: Uses `/bin/sh -ic` with `open` command
- `isPlatformAsset()`: Matches asset names containing "darwin", "macos", or "mac"
- `shellescape()`: Wraps strings in single quotes for safe shell usage

**Windows** (`platform_windows.go`):
- `vscodeWorkspaceStorageDir()`: `%APPDATA%/Code/User/workspaceStorage`
- `defaultFileManagerName()`: Returns "File Explorer"
- `defaultOpenCommand()`: Returns "explorer"
- `detectPlatformApps()`: Checks `ProgramFiles` and `AppData/Local/Programs` for VS Code and Cursor executables
- `openDirectory()`: Uses `cmd /c explorer` with directory path
- `isPlatformAsset()`: Matches asset names containing "windows", "win64", or "win32"

**Linux Needs:**
- `vscodeWorkspaceStorageDir()`: `~/.config/Code/User/workspaceStorage` (XDG spec)
- `defaultFileManagerName()`: "File Manager" (generic) or detect (nautilus, nemo, dolphin, thunar, pcmanfm, caja, etc.)
- `defaultOpenCommand()`: "xdg-open" (standard Linux command)
- `detectPlatformApps()`: Check `~/.local/share/applications`, `/usr/share/applications`, and common install paths (`/usr/bin`, `~/.local/bin`, `~/.nix-profile/bin`)
- `openDirectory()`: Use `bash -c "xdg-open"` or direct xdg-open invocation
- `isPlatformAsset()`: Match "linux" or "linux-x64" or "linux-arm64" (depending on packaging)

---

### 2. UPDATER PLATFORM-SPECIFIC CODE

#### macOS Updater (`updater_darwin.go`)

**Key Implementation:**
- `ApplyUpdate(downloadURL)`: Downloads .zip, extracts to temp dir on same volume, strips quarantine attribute with `xattr -dr com.apple.quarantine`, performs atomic rename swap (current .app → .app.old, new .app into place), relaunches with `/usr/bin/open -n`
- `CleanupOldUpdate()`: Removes `.app.old` from previous update
- `findBundlePath()`: Walks up from `os.Executable()` resolving symlinks, finds directory ending in `.app`
- `findAppInDir()`: Searches for `.app` directory (checks 2 levels deep for wrapped zips)

**Key macOS Considerations:**
- Atomic rename on same volume required
- Quarantine flag stripping is critical for Gatekeeper
- Detached process launch with `-n` flag

#### Windows Updater (`updater_windows.go`)

**Key Implementation:**
- `ApplyUpdate(downloadURL)`: Downloads .zip, extracts to temp dir next to exe, Windows allows renaming running exe, so current exe → .old, new exe into place, relaunches with `CreationFlags: 0x00000008` (DETACHED_PROCESS flag)
- `CleanupOldUpdate()`: Removes `.exe.old`
- `findExeInDir()`: Prefers matching the expected exe name, searches for any `.exe` file (checks 2 levels deep)

**Key Windows Considerations:**
- Windows allows renaming a running executable (unlike Unix)
- Detached process flag prevents console window appearance
- No quarantine attribute equivalent

**Linux Needs:**
- `.exe` → needs to be just the binary name (e.g., `contrails`, not `contrails.exe`)
- Cannot rename running executable on Linux → must handle symlinks or move to temp, setup update script, exit, then run update
- Or: Download to temp, extract, move new to place, exit, systemd timer or shell script handles the rest
- Standard approach: Use a shell script wrapper or systemd service for updates
- `isPlatformAsset()`: Need to handle Linux asset naming (e.g., "Contrails-linux.zip", "Contrails-linux-x64.zip", "Contrails-linux-arm64.zip")

---

### 3. CLAUDE CODE HOOK COMMAND (Platform-Specific)

These are shell commands that get injected into `.claude/settings.local.json` to capture session end signals.

**macOS** (`hook_command_darwin.go`):
```bash
cat > ~/contrails/hook-signals/$(date +%s)_$$.json
```
- Uses `date +%s` for Unix epoch seconds
- `$$` for PID (shell variable)
- Uses standard `cat` to write stdin to file
- File extension is `.json`

**Windows** (`hook_command_windows.go`):
```powershell
powershell -NoProfile -Command "[Console]::In.ReadToEnd() | Set-Content -Encoding utf8 ($env:USERPROFILE + '\contrails\hook-signals\' + [DateTimeOffset]::UtcNow.ToUnixTimeSeconds() + '_' + $PID + '.json')"
```
- Uses PowerShell because Windows default shell doesn't have `date` equivalent easily
- Uses `[DateTimeOffset]::UtcNow.ToUnixTimeSeconds()` for Unix timestamp
- `$PID` for process ID
- Uses `Set-Content -Encoding utf8` to write stdin
- Uses `%USERPROFILE%` equivalent (`$env:USERPROFILE`)

**Linux Needs:**
```bash
cat > ~/contrails/hook-signals/$(date +%s)_$$.json
```
- Same as macOS (Linux has standard `date` and `cat`)
- Can be identical to Darwin

---

### 4. WAILS CONFIGURATION (main.go)

**Current Setup:**
```go
Mac: &mac.Options{
    TitleBar: &mac.TitleBar{
        TitlebarAppearsTransparent: true,
        HideTitle: true,
        HideTitleBar: false,
        FullSizeContent: true,
        UseToolbar: false,
        HideToolbarSeparator: true,
    },
    WebviewIsTransparent: true,
    WindowIsTranslucent: true,
    // ...
}

Windows: &windows.Options{
    WebviewIsTransparent: true,
}

// Menu only on macOS:
Menu: func() *menu.Menu {
    if runtime.GOOS == "darwin" {
        return menu.NewMenuFromItems(menu.AppMenu(), menu.EditMenu(), menu.WindowMenu())
    }
    return nil
}(),
```

**Linux Needs:**
- Wails supports Linux with GTK backend
- May need different webview transparency/translucency settings
- Menu handling via Wails (different from native menu bar)
- Possibly need: `Linux: &linux.Options{...}` if supported by Wails v2.11.0

---

### 5. BUILD SCRIPTS

**macOS** (`buildMacOS.sh`):
```bash
wails build -platform darwin/universal -ldflags "$LDFLAGS"
codesign --force --deep -s - build/bin/contrails.app
```
- Builds for both Apple Silicon and Intel (darwin/universal)
- Code signs with self-signed cert (`-s -`)

**Windows** (`buildWindows.sh`):
```bash
wails build -platform windows/amd64 -webview2 embed -ldflags "$LDFLAGS"
```
- Builds for Windows x64
- Embeds WebView2 runtime

**Linux Needs:**
```bash
wails build -platform linux/amd64 -ldflags "$LDFLAGS"
# Optionally: wails build -platform linux/arm64
```
- Can build for AMD64 and/or ARM64 (Raspberry Pi)
- May need GTK development libraries: `libgtk-3-dev` on Debian/Ubuntu

---

### 6. CI/CD WORKFLOW (.github/workflows/release.yml)

**Current:**
- `build-macos` job: Runs on `macos-latest`, builds with `buildMacOS.sh`, creates `Contrails-macos.zip`
- `build-windows` job: Runs on `windows-latest`, builds with `buildWindows.sh`, creates `Contrails-windows.zip`
- `release` job: Downloads both artifacts, creates GitHub Release with both

**Linux Additions Needed:**
- Add `build-linux` job: Runs on `ubuntu-latest`, builds with `buildLinux.sh`, creates `Contrails-linux.zip` (and optionally `Contrails-linux-arm64.zip`)
- Update `release` job: Include `build-linux` in `needs:` array, add `Contrails-linux.zip` to file list

---

### 7. RUNTIME.GOOS REFERENCES

Found in:
- `main.go` line 53: Runtime menu detection `if runtime.GOOS == "darwin"`
- `analytics.go` lines 104-105, 131-132, 203-204: Runtime telemetry tracking OS and architecture

**Linux Handling:**
- All references already use proper if/else patterns
- `runtime.GOOS == "linux"` will work automatically when needed

---

### 8. AGENT-SPECIFIC PLATFORM CODE

#### VS Code Agent
- Path resolution: Uses `vscodeWorkspaceStorageDir()` from `platform_*.go` files
- **No platform-specific VS Code code exists** (uses shared `vscode/` driver)
- Works identically on all platforms via abstraction

#### Cursor Agent
- **Has platform-specific path files** (`paths_darwin.go`, `paths_windows.go`)
- Reads from platform-specific Cursor storage locations
- **Needs `paths_linux.go`** following same pattern

#### Claude Code Agent
- Hook command differs by platform (captured in `hook_command_*.go`)
- Signal directory: Always `~/contrails/hook-signals` (handled by `EnsureSignalDirectory()` using `os.UserHomeDir()`)
- **Needs `hook_command_linux.go`** identical to macOS version

---

### 9. WAILS.JSON

Current minimal config:
```json
{
  "$schema": "https://wails.io/schemas/config.v2.json",
  "name": "contrails",
  "outputfilename": "contrails",
  // ... frontend build commands
}
```

**Note:** Wails v2.11.0 handles platform-specific builds via CLI flags (`-platform`), not config file.

---

### SUMMARY OF WHAT NEEDS TO BE ADDED FOR LINUX SUPPORT

1. **Create `platform_linux.go`**
   - `vscodeWorkspaceStorageDir()` → `~/.config/Code/User/workspaceStorage`
   - `defaultFileManagerName()` → detect or default to "File Manager"
   - `defaultOpenCommand()` → `xdg-open`
   - `detectPlatformApps()` → Search common Linux install locations
   - `openDirectory()` → Use `xdg-open` or `bash -c "xdg-open"`
   - `isPlatformAsset()` → Match "linux" in asset name

2. **Create `updater_linux.go`**
   - `ApplyUpdate()` → Handle non-running-exe-renamable scenario
   - `CleanupOldUpdate()` → Cleanup old binary
   - Helper functions for finding binary in extracted zip

3. **Create `agent/cursor/paths_linux.go`**
   - `globalStorageDir()` → `~/.config/Cursor/User/globalStorage`
   - `workspaceStorageDir()` → `~/.config/Cursor/User/workspaceStorage`
   - `dbPath()` → `~/.config/Cursor/User/globalStorage/state.vscdb`

4. **Create `agent/claudecode/hook_command_linux.go`**
   - `var hookCommand = "cat > ~/contrails/hook-signals/$(date +%s)_$$.json"` (identical to macOS)

5. **Create `buildLinux.sh`**
   - Build command for Linux x64 (and optionally ARM64)

6. **Update `.github/workflows/release.yml`**
   - Add `build-linux` job
   - Update `release` job to include Linux artifact

7. **Update `main.go`** (possibly, if Wails supports Linux options)
   - May need platform detection for menu or window options

8. **Update README.md**
   - Add Linux installation instructions
   - Update "Supported Agents" table with Linux support note

---

### ARCHITECTURE NOTES

The codebase uses excellent abstraction patterns:
- Platform functions exported as simple functions (not methods/interfaces)
- Build tags handle compile-time selection
- Main business logic is platform-agnostic
- Only I/O, paths, and shell commands differ by platform

All path resolution functions follow a consistent naming pattern and are called from platform-agnostic code, making Linux support straightforward to add.

<details>
<summary>Tool Calls</summary>

- **WebSearch**: `Search `wails-build-action GitHub Actions linux ubuntu 24.04 workflow yaml example``

</details>

I now have all the information needed. Here is the comprehensive research report.

---

# Wails v2 Linux Support -- Comprehensive Research Report

## 1. Linux Requirements and Dependencies

Wails v2 on Linux requires three core dependencies:

- **GCC / Build Tools** -- C compiler required because Wails uses CGO for GTK bindings
- **GTK3+ Development Libraries** (`libgtk-3-dev`)
- **WebKit2GTK** -- the web rendering engine (Linux equivalent of WebView2 on Windows / WKWebView on macOS)

### WebKit2GTK Version Matrix

| Distribution | Package | Build Tag Required |
|---|---|---|
| Ubuntu 20.04/22.04, Debian 11/12 | `libwebkit2gtk-4.0-dev` | (none) |
| Ubuntu 24.04+, Debian 13+ | `libwebkit2gtk-4.1-dev` | `-tags webkit2_41` |
| Fedora 38+, Arch Linux | `webkit2gtk4.1-devel` / `webkit2gtk-4.1` | `-tags webkit2_41` |

This is the single biggest gotcha: **Ubuntu 24.04 dropped `libwebkit2gtk-4.0-dev`** entirely. You must install `libwebkit2gtk-4.1-dev` and build with `-tags webkit2_41`. Multiple GitHub issues document this: [#3581](https://github.com/wailsapp/wails/issues/3581), [#3513](https://github.com/wailsapp/wails/issues/3513), [#3587](https://github.com/wailsapp/wails/issues/3587).

Additional build tags exist for specific WebKit2GTK feature versions:
- `-tags webkit2_36` (WebKit2GTK 2.36+ features)
- `-tags webkit2_40` (WebKit2GTK 2.40+ features)

### Other Build Requirements
- **Go** (with CGO enabled -- `CGO_ENABLED=1` is mandatory)
- **gcc** and **pkg-config** (for C bindings)
- **Node.js/npm/yarn** (frontend build)

The `wails doctor` command auto-detects the distribution and suggests the correct package install commands.

## 2. Supported Distributions

Wails does not maintain a fixed list. Instead, it uses package-manager detection logic in `v2/internal/system/packagemanager/`. It supports distributions that use:
- **apt** (Debian, Ubuntu, Linux Mint, Pop!_OS, etc.)
- **dnf/yum** (Fedora, RHEL, CentOS, Rocky, AlmaLinux)
- **pacman** (Arch Linux, Manjaro, EndeavourOS)
- **zypper** (openSUSE)
- **emerge** (Gentoo)
- **nix** (NixOS)

If a distro is not recognized, users can add detection rules via the [Add Linux Distro guide](https://wails.io/docs/guides/linux-distro-support/).

## 3. Linux Build Configuration

A `buildLinux.sh` equivalent for this project would look like:

```sh
#!/bin/sh
set -e
LDFLAGS=""
[ -n "$1" ] && LDFLAGS="$LDFLAGS -X main.Version=$1"
[ -n "$2" ] && LDFLAGS="$LDFLAGS -X main.PostHogAPIKey=$2"

# For Ubuntu 24.04+: add -tags webkit2_41
TAGS="-tags webkit2_41"

if [ -n "$LDFLAGS" ]; then
  wails build -platform linux/amd64 $TAGS -ldflags "$LDFLAGS"
else
  wails build -platform linux/amd64 $TAGS
fi
```

Key differences from macOS/Windows builds:
- No code signing step (unlike macOS `codesign`)
- No WebView2 embedding (unlike Windows `-webview2 embed`)
- Output is a single ELF binary (no `.app` bundle or `.exe`)
- Requires the webkit2_41 build tag on modern distros

## 4. Known Issues and Limitations vs macOS/Windows

### Wayland Support
- Wails v2 runs on Wayland via **XWayland compatibility**, not native Wayland ([#1420](https://github.com/wailsapp/wails/issues/1420))
- **GNOME on Wayland** does not display window icons; a `.desktop` file is required
- Window max width/height constraints may not work correctly on Wayland ([#2431](https://github.com/wailsapp/wails/issues/2431))
- Some users on Arch Linux / GNOME Wayland reported blank windows

### Window Management
- **Window translucency** may be ignored by some window managers or produce a black window (requires a compositor)
- Transparent windows work (added in PR [#1926](https://github.com/wailsapp/wails/pull/1926)) but without blur/frosting effects (would need GTK4)
- **Frameless windows** work but custom drag regions need `CSSDragProperty` and `CSSDragValue` options

### Titlebar / Styling
- Linux has no equivalent to macOS `TitleBar` options (transparent titlebar, hidden title, etc.)
- The `linux.Options{}` struct provides: `Icon`, `WindowIsTranslucent`, `Messages`, `WebviewGpuPolicy`, `ProgramName`
- No native vibrancy/translucent material like macOS
- The project's current `main.go` has `Mac:` and `Windows:` options but would need a `Linux:` section

### Menu Bar
- The current project conditionally renders `Menu` only on macOS (line 53 of `main.go`: `runtime.GOOS == "darwin"`). On Linux, GTK apps can have menus but Wails renders them inside the window (same as Windows). The current `nil` return for non-darwin is fine.

### Cross-compilation
- Cross-compiling *to* Linux from macOS/Windows is difficult due to CGO/GTK dependencies
- Cross-compiling *from* Linux to Windows/macOS is also problematic ([#1921](https://github.com/wailsapp/wails/issues/1921))
- Best approach: build each platform on its native runner in CI

## 5. Packaging Options

### AppImage (Recommended for universal distribution)
- Single-file, runs on most Linux distros without installation
- Structure: AppDir containing the binary, a `.desktop` file, and an icon
- Built using `appimagetool` or `linuxdeploy`
- Self-update support via embedded update information (zsync protocol)
- No signing required
- Example: `appimagetool -u "zsync|https://example.com/app.AppImage.zsync" AppDir/`

### .deb (Debian/Ubuntu)
- Built via **nFPM** (used by Wails v3, usable with v2 too)
- Runtime dependencies: `libgtk-3-0`, `libwebkit2gtk-4.1-0` (NOT the `-dev` packages)
- Can be hosted in a personal APT repository for auto-updates

### .rpm (Fedora/RHEL)
- Also built via nFPM
- Runtime dependencies: `gtk3`, `webkit2gtk4.1`

### Flatpak / Snap
- Not directly supported by Wails tooling but possible with manual configuration
- Flatpak has sandbox restrictions that may affect filesystem access (relevant for reading `~/.claude/` and `~/.config/`)
- Snap has `strict` confinement that would require `home` and `personal-files` plugs

### Recommendation for Contrails
AppImage is the most practical option for initial Linux release:
- No repository hosting needed
- Single download, similar UX to current macOS (.zip) and Windows (.zip) distribution
- Can be distributed as `Contrails-linux.AppImage` in GitHub Releases

## 6. Window Options and Styling Differences

The project's `main.go` currently configures:

```go
Mac: &mac.Options{ ... }      // Transparent titlebar, vibrancy
Windows: &windows.Options{ ... } // Transparent webview
```

A Linux options block would be:

```go
Linux: &linux.Options{
    Icon:                icon,  // Window icon (required for taskbar)
    WindowIsTranslucent: false, // Usually false; compositor-dependent
    WebviewGpuPolicy:    linux.WebviewGpuPolicyAlways,
    ProgramName:         "contrails",
}
```

Key styling differences:
- The custom titlebar in the frontend (already used for Windows) would also work for Linux
- `.desktop` file needed for proper icon display in GNOME/Wayland
- No system-level dark/light theme integration (the app already handles its own theming)

## 7. CI/CD Considerations

### GitHub Actions Configuration

The existing `release.yml` would need a new `build-linux` job:

```yaml
build-linux:
  runs-on: ubuntu-24.04  # or ubuntu-latest (currently 24.04)
  steps:
    - uses: actions/checkout@v4
    - name: Install Linux dependencies
      run: sudo apt-get update && sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev
    - name: Set up Go
      uses: actions/setup-go@v5
      with:
        go-version: "1.26"
    - name: Set up Node
      uses: actions/setup-node@v4
      with:
        node-version: "24.11.1"
    - name: Install Yarn
      run: npm install -g yarn
    - name: Install Wails
      run: go install github.com/wailsapp/wails/v2/cmd/wails@v2.11.0
    - name: Build
      run: ./buildLinux.sh "$VERSION" "$POSTHOG_KEY"
```

Important notes:
- `ubuntu-latest` on GitHub Actions is currently Ubuntu 24.04, which requires `webkit2_41`
- If you need Ubuntu 22.04 for broader compatibility, use `runs-on: ubuntu-22.04` with `libwebkit2gtk-4.0-dev`
- The [wails-build-action](https://github.com/dAppServer/wails-build-action) can automate dependency detection and build tag selection
- Building on Ubuntu 22.04 produces binaries compatible with more distros (older glibc)

### Alternative: wails-build-action
```yaml
- uses: dAppServer/wails-build-action@v3
  with:
    build-name: contrails
    build-platform: linux/amd64
```
This action automatically detects the Ubuntu version and adds `-tags webkit2_41` when needed.

## 8. How Other Wails Apps Handle Linux Distribution

Based on the Wails ecosystem:
- Most distribute a raw binary or AppImage via GitHub Releases
- Some use nFPM for .deb/.rpm alongside AppImage
- The [wails-build-action](https://github.com/marketplace/actions/wails-action-ci-cd) is widely used for CI
- Few use Flatpak or Snap due to complexity

## 9. Linux App Data Paths (XDG Base Directory)

### XDG Specification

| Variable | Default | Purpose |
|---|---|---|
| `$XDG_CONFIG_HOME` | `~/.config` | User configuration |
| `$XDG_DATA_HOME` | `~/.local/share` | User data |
| `$XDG_STATE_HOME` | `~/.local/state` | User state (logs, history) |
| `$XDG_CACHE_HOME` | `~/.cache` | Non-essential cached data |

### Go Standard Library Support
- `os.UserConfigDir()` returns `$XDG_CONFIG_HOME` or `~/.config` on Linux
- `os.UserCacheDir()` returns `$XDG_CACHE_HOME` or `~/.cache` on Linux
- `os.UserHomeDir()` returns `$HOME`

### Contrails-Specific Path Mapping

The project needs `platform_linux.go` with these paths:

| Function | macOS | Windows | Linux (needed) |
|---|---|---|---|
| `vscodeWorkspaceStorageDir()` | `~/Library/Application Support/Code/User/workspaceStorage` | `%APPDATA%/Code/User/workspaceStorage` | `~/.config/Code/User/workspaceStorage` |
| Cursor `globalStorageDir()` | `~/Library/Application Support/Cursor/User/globalStorage` | `%APPDATA%/Cursor/User/globalStorage` | `~/.config/Cursor/User/globalStorage` |
| Cursor `workspaceStorageDir()` | `~/Library/Application Support/Cursor/User/workspaceStorage` | `%APPDATA%/Cursor/User/workspaceStorage` | `~/.config/Cursor/User/workspaceStorage` |
| Cursor `dbPath()` | `~/Library/Application Support/Cursor/User/globalStorage/state.vscdb` | `%APPDATA%/Cursor/User/globalStorage/state.vscdb` | `~/.config/Cursor/User/globalStorage/state.vscdb` |
| `configDir` (Contrails settings) | `~/Library/Application Support` (`os.UserConfigDir()`) | `%APPDATA%` | `~/.config` (via `os.UserConfigDir()`) |
| Claude Code projects | `~/.claude/projects/` | `~/.claude/projects/` | `~/.claude/projects/` (same) |
| Hook signals | `~/contrails/hook-signals/` | `%USERPROFILE%/contrails/hook-signals/` | `~/contrails/hook-signals/` (same) |

### Files That Need Linux Variants

These files currently exist only for `darwin` and `windows` and need `linux` build-tagged equivalents:

1. `/Users/admin/Documents/projects/contrails/platform_darwin.go` -> `platform_linux.go`
2. `/Users/admin/Documents/projects/contrails/agent/cursor/paths_darwin.go` -> `paths_linux.go`
3. `/Users/admin/Documents/projects/contrails/agent/claudecode/hook_command_darwin.go` -> `hook_command_linux.go`

The `hook_command_linux.go` would use the same `cat` command as macOS:
```
cat > ~/contrails/hook-signals/$(date +%s)_$$.json
```
(Linux `date` supports `%N` for nanoseconds, unlike BSD date, but `%s_$$` is sufficient.)

The `isPlatformAsset()` function in `platform_linux.go` would match asset names containing "linux".

The `defaultOpenCommand()` would return `"xdg-open"`, and `defaultFileManagerName()` would return `"Files"` (or `"File Manager"`).

## 10. Linux Auto-Update Strategies

### Option A: AppImage with zsync (Recommended)
- Embed update info: `appimagetool -u "zsync|https://github.com/.../latest/Contrails-linux.AppImage.zsync"`
- Users can update via `AppImageUpdate` tool
- Delta updates (only downloads changed bytes)
- No server-side infrastructure needed beyond GitHub Releases

### Option B: GitHub Release Check (Current approach)
- The project already has `isPlatformAsset()` per platform
- On Linux, check GitHub Releases API for assets matching "linux"
- Download new binary / AppImage and replace
- Simple but requires full re-download each time

### Option C: Package Repository
- Host APT/YUM repo for .deb/.rpm auto-updates
- More complex infrastructure
- Best UX for users of those package managers
- Not recommended for initial release

### Recommendation
Start with GitHub Release checking (already partially implemented via `isPlatformAsset`), distributing an AppImage. This matches the current macOS/Windows distribution model (zip download from GitHub Releases).

---

## Summary of Work Required for Linux Support

1. **New files needed**: `platform_linux.go`, `agent/cursor/paths_linux.go`, `agent/claudecode/hook_command_linux.go`
2. **Modified files**: `main.go` (add `Linux:` options + possibly menu logic), `release.yml` (add `build-linux` job)
3. **New file**: `buildLinux.sh` (build script with `-tags webkit2_41`)
4. **New asset**: `.desktop` file and icon for AppImage packaging
5. **Optional**: AppImage packaging script/Makefile target

The Go standard library's `os.UserConfigDir()` and `os.UserHomeDir()` already handle XDG paths correctly, so the Contrails settings storage (`configDir`) and Claude Code project scanning (`~/.claude/projects/`) will work on Linux without changes. The platform-specific files are the main code effort.

### Sources
- [Wails Installation Docs](https://wails.io/docs/gettingstarted/installation/)
- [Wails Linux Distro Support](https://wails.io/docs/guides/linux-distro-support/)
- [Wails Linux Guide](https://wails.io/docs/guides/linux/)
- [Ubuntu 24.04 webkit2gtk issue #3581](https://github.com/wailsapp/wails/issues/3581)
- [Ubuntu 24.04 webkit2gtk issue #3513](https://github.com/wailsapp/wails/issues/3513)
- [Wails Wayland support #1420](https://github.com/wailsapp/wails/issues/1420)
- [Wails Linux transparent windows #1923](https://github.com/wailsapp/wails/issues/1923)
- [wails-build-action](https://github.com/dAppServer/wails-build-action)
- [Wails DeepWiki Installation](https://deepwiki.com/wailsapp/wails/2.1-installation)
- [nFPM packaging](https://nfpm.goreleaser.com/)
- [Wails nfpm.yaml fix PR #4481](https://github.com/wailsapp/wails/pull/4481)
- [V3 Linux packaging PR #3909](https://github.com/wailsapp/wails/pull/3909)
- [AppImage update documentation](https://docs.appimage.org/packaging-guide/optional/updates.html)
- [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir/latest/)
- [XDG ArchWiki](https://wiki.archlinux.org/title/XDG_Base_Directory)
- [Go adrg/xdg package](https://github.com/adrg/xdg)
- [Wails Options Reference](https://wails.io/docs/reference/options/)
- [Wails Frameless Guide](https://wails.io/docs/guides/frameless/)
- [Linux AppImage packaging blog](https://blog.jamesreed.dev/packaging-apps-for-linux-with-appimage)
- [Electron Linux auto-update discussion](https://github.com/electron-userland/electron-builder/issues/6330)

## Linux Support Research Report

Based on exploring the current codebase (which already has `darwin` + `windows` platform files) and researching Wails v2 Linux specifics:

---

### 1. Core Dependencies

Linux uses **WebKit2GTK** (via GTK3) as its webview engine. Unlike Windows (pure Go), Linux **requires CGO** (like macOS).

**Build-time dependencies:**
- `gcc`, `pkg-config`
- `libgtk-3-dev`
- `libwebkit2gtk-4.0-dev` (Ubuntu 22.04) **OR** `libwebkit2gtk-4.1-dev` (Ubuntu 24.04+)

**Runtime dependencies:**
- `libgtk-3-0`, `libwebkit2gtk-4.0-0` or `libwebkit2gtk-4.1-0`

**Critical gotcha:** Ubuntu 24.04 dropped `webkit2gtk-4.0` entirely. You must use `4.1` and build with **`-tags webkit2_41`**. This is the single biggest Linux-specific concern. ([#3581](https://github.com/wailsapp/wails/issues/3581), [#3513](https://github.com/wailsapp/wails/issues/3513))

---

### 2. Files to Create (Matching Existing Pattern)

The codebase already has clean `_darwin.go` / `_windows.go` splits. Linux needs these 4 new files + 2 build/CI files:

| New File | Based On | Notes |
|---|---|---|
| `platform_linux.go` | `platform_darwin.go` | Paths, open command, app detection, asset matching |
| `updater_linux.go` | `updater_windows.go` | Binary replacement (no `.app` bundle) |
| `agent/cursor/paths_linux.go` | `paths_darwin.go` | XDG-based paths |
| `agent/claudecode/hook_command_linux.go` | `hook_command_darwin.go` | **Identical** to macOS (`cat > ~/contrails/...`) |
| `buildLinux.sh` | `buildWindows.sh` | Add `-tags webkit2_41` |
| `.github/workflows/release.yml` | Add `build-linux` job | |

---

### 3. Platform-Specific Path Mapping

| Function | macOS | Windows | Linux |
|---|---|---|---|
| `vscodeWorkspaceStorageDir()` | `~/Library/Application Support/Code/...` | `%APPDATA%/Code/...` | `~/.config/Code/User/workspaceStorage` |
| Cursor `globalStorageDir()` | `~/Library/Application Support/Cursor/...` | `%APPDATA%/Cursor/...` | `~/.config/Cursor/User/globalStorage` |
| Cursor `workspaceStorageDir()` | same pattern | same pattern | `~/.config/Cursor/User/workspaceStorage` |
| Cursor `dbPath()` | same pattern | same pattern | `~/.config/Cursor/User/globalStorage/state.vscdb` |
| `defaultOpenCommand()` | `open` | `explorer` | **`xdg-open`** |
| `defaultFileManagerName()` | `Finder` | `File Explorer` | `File Manager` |
| `isPlatformAsset()` | "darwin"/"macos"/"mac" | "windows"/"win64"/"win32" | **"linux"** |

All use XDG Base Directory spec (`~/.config` = `$XDG_CONFIG_HOME` default). Go's `os.UserConfigDir()` already returns this correctly on Linux.

---

### 4. `platform_linux.go` Details

- **`detectPlatformApps()`**: Search `/usr/bin`, `/usr/local/bin`, `~/.local/bin`, and `.desktop` files in `/usr/share/applications` and `~/.local/share/applications` for VS Code (`code`), Cursor (`cursor`), etc.
- **`openDirectory()`**: Use `xdg-open <path>` (standard freedesktop.org method)
- **`shellescape()`**: Same as macOS (POSIX shell)

---

### 5. Updater Considerations

The updater is the most complex piece. Key differences from macOS/Windows:

| Concern | macOS | Windows | Linux |
|---|---|---|---|
| App format | `.app` bundle (dir) | `.exe` (file) | ELF binary (file) |
| Rename running binary? | N/A (bundle) | Yes (Windows allows it) | **No** (Linux locks the inode) |
| Update strategy | Rename `.app` → `.app.old` | Rename `.exe` → `.exe.old` | **Overwrite works** — Linux locks inode, not path. You can `os.Rename` or `os.Remove` + write new file while the old process runs |
| Relaunch | `/usr/bin/open -n` | `os.StartProcess` (DETACHED) | `exec.Command(exePath).Start()` or `syscall.Exec` |
| Quarantine | `xattr` removal | N/A | N/A |
| Asset matching | "darwin"/"macos" | "windows"/"win" | "linux" |

**Key insight:** On Linux, you *can* replace/unlink a running binary — the kernel keeps the old inode in memory until the process exits. So the update flow is simpler than Windows: download new binary → replace file → relaunch.

---

### 6. Wails Window Options (`main.go`)

```go
Linux: &linux.Options{
    Icon:                icon,     // Required for taskbar/Wayland
    WindowIsTranslucent: false,    // Compositor-dependent, unreliable
    WebviewGpuPolicy:    linux.WebviewGpuPolicyAlways,
    ProgramName:         "contrails",
}
```

- **No equivalent** to macOS transparent titlebar / vibrancy — the custom titlebar CSS (already used for Windows) will work
- The current `Menu: nil` for non-darwin is correct for Linux too

---

### 7. Known Limitations & Wayland

| Issue | Impact |
|---|---|
| Runs via **XWayland**, not native Wayland | Works but not pixel-perfect on Wayland-only setups |
| GNOME/Wayland doesn't show window icon without `.desktop` file | Need to ship a `.desktop` file |
| Window translucency unreliable | Depends on compositor; keep it `false` |
| Max width/height constraints may fail on Wayland | Minor UX issue |
| No frosted glass / vibrancy | CSS-only styling (already the approach for Windows) |

---

### 8. Packaging & Distribution

**Recommended: AppImage** (single-file, runs on most distros, no install needed)
- Similar UX to current `.zip` distribution for macOS/Windows
- Built with `linuxdeploy` + `appimagetool`
- Supports delta updates via zsync
- GitHub Release asset: `Contrails-linux.AppImage`

**Alternative/future:** `.deb` + `.rpm` via nFPM for apt/yum users.

**Avoid initially:** Flatpak/Snap (sandbox restrictions conflict with reading `~/.claude/`, `~/.config/Code/`, etc.)

---

### 9. CI/CD (`release.yml`)

```yaml
build-linux:
  runs-on: ubuntu-24.04
  steps:
    - uses: actions/checkout@v4
    - name: Install deps
      run: sudo apt-get update && sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev
    - name: Setup Go
      uses: actions/setup-go@v5
    - name: Setup Node
      uses: actions/setup-node@v4
    - name: Install Wails
      run: go install github.com/wailsapp/wails/v2/cmd/wails@v2.11.0
    - name: Build
      run: ./buildLinux.sh "$VERSION" "$POSTHOG_KEY"
```

**Note:** Building on Ubuntu 22.04 (with `webkit2gtk-4.0`) produces binaries with older glibc, compatible with more distros. Building on 24.04 (with `webkit2_41`) targets newer distros. Trade-off to consider.

---

### 10. Supported Distributions

Wails supports any distro using: **apt** (Debian/Ubuntu/Mint/Pop), **dnf** (Fedora/RHEL), **pacman** (Arch/Manjaro), **zypper** (openSUSE), **emerge** (Gentoo), **nix** (NixOS).

Architecture: **amd64** (primary target, covers ~99% of Linux desktop users). ARM64 can be added later for Raspberry Pi / Asahi Linux users.

---

### Summary: Effort Assessment

| Area | Complexity | Notes |
|---|---|---|
| `platform_linux.go` | **Low** | Straightforward XDG paths + `xdg-open` |
| `agent/cursor/paths_linux.go` | **Trivial** | 3 functions, `~/.config/Cursor/...` |
| `hook_command_linux.go` | **Trivial** | Copy from macOS verbatim |
| `updater_linux.go` | **Medium** | Simpler than macOS/Windows (can overwrite running binary) |
| `buildLinux.sh` | **Low** | One `wails build` command + `-tags webkit2_41` |
| CI/CD `release.yml` | **Low** | Add parallel job, install GTK/WebKit deps |
| `main.go` Linux options | **Low** | Add `Linux:` block with icon + program name |
| AppImage packaging | **Medium** | New tooling (`linuxdeploy`, `.desktop` file, icon) |
| **Total** | **Moderate** | Less work than Windows was (no `.app` bundle complexity) |

---

