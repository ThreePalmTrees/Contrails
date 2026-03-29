- **Session ID:** `00bef500-32bf-4f00-a4fc-eb144f69fe68`
- **Created:** 2026-03-26 22:28:31
- **Last Message:** 2026-03-26 22:39:49
- **User:** User
- **Agent:** Claude Code
- **Model:** claude-opus-4-6

## 🧑 User - 2026-03-26 22:29:42

We fully supported Windows, and we already support MacOS.
Now we need to research what needs to be done in order to support Linux as well.
Please read the previous spike we did for supporting Windows, and research what's needed for supporting Linux.
This is mainly a detailed research task rather than actual implementation (will follow later based on this analysis)

@"agent_contrails/1772990949 - Right now we support Mac only (universal, both apple silicon and intel).md"

## 🤖 Assistant - 2026-03-26 22:29:48

*Model: claude-opus-4-6*

I'll research Linux support requirements by examining the current cross-platform code and researching Wails v2 Linux specifics in parallel.

<details>
<summary>Tool Calls (2)</summary>

- **Agent**: `Agent: Find platform-specific code paths`

<details>
<summary>Subagent Activity (29 tool calls)</summary>

Explore the codebase at /Users/admin/Documents/projects/contrails thoroughly. I need to find:

1. All files with build tags (//go:build darwin, //go:build windows, //go:build linux, or any platform-specific build constraints)
2. All platform-specific path resolution code (paths_darwin.go, paths_windows.go, etc.)
3. All updater platform-specific code (updater_darwin.go, updater_windows.go, etc.)
4. All references to runtime.GOOS or platform detection
5. The main.go file - specifically the Wails options including Mac: and Windows: sections
6. The hook command code in agent/claudecode/hook.go - how it handles platform differences
7. Build scripts (buildMacOS.sh, buildWindows.sh or similar)
8. CI/CD workflow files in .github/workflows/
9. The wails.json config
10. Any Linux-specific code that may already exist

Be very thorough - read the full contents of all platform-specific files. I need to understand exactly what cross-platform work has already been done for Windows so I can assess what's needed for Linux.

<details>
<summary>Bash: `Ran `find /Users/admin/Documents/projects/contrails -type f -name "*.go" | head -20``</summary>

```
/Users/admin/Documents/projects/contrails/app_test.go
/Users/admin/Documents/projects/contrails/updater.go
/Users/admin/Documents/projects/contrails/runtime.go
/Users/admin/Documents/projects/contrails/parser_test.go
/Users/admin/Documents/projects/contrails/types.go
/Users/admin/Documents/projects/contrails/agent/contrail.go
/Users/admin/Documents/projects/contrails/agent/cursor/scanner.go
/Users/admin/Documents/projects/contrails/agent/cursor/parser_test.go
/Users/admin/Documents/projects/contrails/agent/cursor/types.go
/Users/admin/Documents/projects/contrails/agent/cursor/paths_windows.go
/Users/admin/Documents/projects/contrails/agent/cursor/parser.go
/Users/admin/Documents/projects/contrails/agent/cursor/paths_darwin.go
/Users/admin/Documents/projects/contrails/agent/cursor/driver.go
/Users/admin/Documents/projects/contrails/agent/claudecode/scanner.go
/Users/admin/Documents/projects/contrails/agent/claudecode/parser_test.go
/Users/admin/Documents/projects/contrails/agent/claudecode/types.go
/Users/admin/Documents/projects/contrails/agent/claudecode/signal_watcher.go
/Users/admin/Documents/projects/contrails/agent/claudecode/hook.go
/Users/admin/Documents/projects/contrails/agent/claudecode/hook_enforcer.go
/Users/admin/Documents/projects/contrails/agent/claudecode/hook_test.go
```

</details>
<details>
<summary>Bash: `Ran `find /Users/admin/Documents/projects/contrails -type f \( -name "*_windows.go" -o -name "*_darwin.go" -o -name "*_linux.go" \)``</summary>

```
/Users/admin/Documents/projects/contrails/agent/cursor/paths_windows.go
/Users/admin/Documents/projects/contrails/agent/cursor/paths_darwin.go
/Users/admin/Documents/projects/contrails/agent/claudecode/hook_command_windows.go
/Users/admin/Documents/projects/contrails/agent/claudecode/hook_command_darwin.go
/Users/admin/Documents/projects/contrails/platform_windows.go
/Users/admin/Documents/projects/contrails/updater_windows.go
/Users/admin/Documents/projects/contrails/platform_darwin.go
/Users/admin/Documents/projects/contrails/updater_darwin.go
```

</details>
<details>
<summary>Bash: `Ran `find /Users/admin/Documents/projects/contrails -type f \( -name "main.go" -o -name "wails.json" -o -name "*.sh" -o -name "*.yml" -o -name "*.yaml" \) | grep -E "(main\.go|wails\.json|\.sh$|workflows)" | head -20``</summary>

```
/Users/admin/Documents/projects/contrails/wails.json
/Users/admin/Documents/projects/contrails/buildWindows.sh
/Users/admin/Documents/projects/contrails/buildMacOS.sh
/Users/admin/Documents/projects/contrails/.github/workflows/release.yml
/Users/admin/Documents/projects/contrails/release.sh
/Users/admin/Documents/projects/contrails/main.go
```

</details>
<details>
<summary>Bash: `Ran `grep -r "runtime\.GOOS\|runtime\.GOARCH\|//go:build" /Users/admin/Documents/projects/contrails --include="*.go" 2>/dev/null | head -50``</summary>

```
/Users/admin/Documents/projects/contrails/agent/cursor/paths_windows.go://go:build windows
/Users/admin/Documents/projects/contrails/agent/cursor/paths_darwin.go://go:build darwin
/Users/admin/Documents/projects/contrails/agent/claudecode/hook_command_windows.go://go:build windows
/Users/admin/Documents/projects/contrails/agent/claudecode/hook_command_darwin.go://go:build darwin
/Users/admin/Documents/projects/contrails/platform_windows.go://go:build windows
/Users/admin/Documents/projects/contrails/updater_windows.go://go:build windows
/Users/admin/Documents/projects/contrails/platform_darwin.go://go:build darwin
/Users/admin/Documents/projects/contrails/updater_darwin.go://go:build darwin
/Users/admin/Documents/projects/contrails/main.go:			if runtime.GOOS == "darwin" {
/Users/admin/Documents/projects/contrails/analytics.go:	props.Set("os", runtime.GOOS)
/Users/admin/Documents/projects/contrails/analytics.go:	props.Set("arch", runtime.GOARCH)
/Users/admin/Documents/projects/contrails/analytics.go:	props.Set("os", runtime.GOOS)
/Users/admin/Documents/projects/contrails/analytics.go:	props.Set("arch", runtime.GOARCH)
/Users/admin/Documents/projects/contrails/analytics.go:		"os":          runtime.GOOS,
/Users/admin/Documents/projects/contrails/analytics.go:		"arch":        runtime.GOARCH,
```

</details>
- **Read**: `Read `/Users/admin/Documents/projects/contrails/main.go``
- **Read**: `Read `/Users/admin/Documents/projects/contrails/platform_darwin.go``
- **Read**: `Read `/Users/admin/Documents/projects/contrails/platform_windows.go``
- **Read**: `Read `/Users/admin/Documents/projects/contrails/updater_darwin.go``
- **Read**: `Read `/Users/admin/Documents/projects/contrails/updater_windows.go``
- **Read**: `Read `/Users/admin/Documents/projects/contrails/agent/cursor/paths_darwin.go``
- **Read**: `Read `/Users/admin/Documents/projects/contrails/agent/cursor/paths_windows.go``
- **Read**: `Read `/Users/admin/Documents/projects/contrails/agent/claudecode/hook_command_darwin.go``
- **Read**: `Read `/Users/admin/Documents/projects/contrails/agent/claudecode/hook_command_windows.go``
- **Read**: `Read `/Users/admin/Documents/projects/contrails/wails.json``
- **Read**: `Read `/Users/admin/Documents/projects/contrails/buildMacOS.sh``
- **Read**: `Read `/Users/admin/Documents/projects/contrails/buildWindows.sh``
- **Read**: `Read `/Users/admin/Documents/projects/contrails/.github/workflows/release.yml``
- **Read**: `Read `/Users/admin/Documents/projects/contrails/agent/claudecode/hook.go``
<details>
<summary>Bash: `Ran `find /Users/admin/Documents/projects/contrails -type f -name "*.go" | grep -E "(updater|platform)" | grep -v test``</summary>

```
/Users/admin/Documents/projects/contrails/updater.go
/Users/admin/Documents/projects/contrails/platform_windows.go
/Users/admin/Documents/projects/contrails/updater_windows.go
/Users/admin/Documents/projects/contrails/platform_darwin.go
/Users/admin/Documents/projects/contrails/updater_darwin.go
```

</details>
- **Read**: `Read `/Users/admin/Documents/projects/contrails/updater.go``
- **Read**: `Read `/Users/admin/Documents/projects/contrails/analytics.go``
<details>
<summary>Bash: `Ran `head -100 /Users/admin/Documents/projects/contrails/README.md``</summary>

```
# Contrails ✈️

**Preserve your coding agent trails.**

<i>Contrails, short for "condensation trails", are the trails left behind by aircrafts at high altitudes.</i>

Contrails is a macOS and Windows app that watches your coding agent sessions (VS Code Copilot, Claude Code, and Cursor), parses them into readable Markdown, and saves them into your project repositories.
This way you keep the reasoning that led to fixing a bug or implementing a feature.

Built with [Wails v2](https://wails.io/) (Go + React + TypeScript).

![Contrails Screenshot](screenshot.png)

## Why

Coding agents forget everything between sessions. The reasoning that led to a fix, the wrong approaches that were tried, the self-corrections — all of it vanishes. Contrails watches agent session files in real-time and outputs clean, human-readable Markdown into a `contrails/` directory in your project, making your agent conversations part of your repo history.

When working on a related feature in the future, you can reference relevant contrails to help the agent remember its previous reasoning.

## Supported Agents
...
```

</details>
<details>
<summary>Bash: `Ran `grep -r "linux\|Linux\|GOOS == \"linux\"\|runtime\.GOOS" /Users/admin/Documents/projects/contrails --include="*.go" 2>/dev/null | grep -i linux``</summary>

```
(Bash completed with no output)
```

</details>
<details>
<summary>Bash: `Ran `find /Users/admin/Documents/projects/contrails -type f -name "*.go" | sort``</summary>

```
/Users/admin/Documents/projects/contrails/agent/claudecode/driver.go
/Users/admin/Documents/projects/contrails/agent/claudecode/hook_command_darwin.go
/Users/admin/Documents/projects/contrails/agent/claudecode/hook_command_windows.go
/Users/admin/Documents/projects/contrails/agent/claudecode/hook_enforcer_test.go
/Users/admin/Documents/projects/contrails/agent/claudecode/hook_enforcer.go
/Users/admin/Documents/projects/contrails/agent/claudecode/hook_test.go
/Users/admin/Documents/projects/contrails/agent/claudecode/hook.go
/Users/admin/Documents/projects/contrails/agent/claudecode/parser_test.go
/Users/admin/Documents/projects/contrails/agent/claudecode/parser.go
/Users/admin/Documents/projects/contrails/agent/claudecode/scanner.go
/Users/admin/Documents/projects/contrails/agent/claudecode/signal_watcher.go
/Users/admin/Documents/projects/contrails/agent/claudecode/types.go
/Users/admin/Documents/projects/contrails/agent/contrail.go
/Users/admin/Documents/projects/contrails/agent/cursor/driver.go
/Users/admin/Documents/projects/contrails/agent/cursor/parser_test.go
/Users/admin/Documents/projects/contrails/agent/cursor/parser.go
/Users/admin/Documents/projects/contrails/agent/cursor/paths_darwin.go
/Users/admin/Documents/projects/contrails/agent/cursor/paths_windows.go
/Users/admin/Documents/projects/contrails/agent/cursor/scanner.go
/Users/admin/Documents/projects/contrails/agent/cursor/types.go
...
```

</details>
<details>
<summary>Bash: `Ran `ls -la /Users/admin/Documents/projects/contrails/agent_contrails/ 2>/dev/null | head -30``</summary>

```
total 600
drwxr-xr-x@ 11 admin  staff     352 Mar 26 23:19 .
drwx------@ 40 admin  staff    1280 Mar 26 23:18 ..
-rw-r--r--@  1 admin  staff   24319 Mar 25 17:20 1772990949 - Right now we support Mac only (universal, both apple silicon and intel).md
-rw-r--r--@  1 admin  staff   15935 Mar 25 23:14 1774090224 - This is a reflection task. Research the codebase, see the main README, the agent_contrails, anything.md
-rw-r--r--@  1 admin  staff    4501 Mar 25 23:15 1774167064 - In App.tsx, inside the titlebar, I want to show that indicator only for dev builds (check how in the.md
-rw-r--r--@  1 admin  staff   11145 Mar 25 17:23 1774455706 - I want to allow searching the sections (processed, partially processed, not yet processed, ignored).md
-rw-r--r--@  1 admin  staff   12210 Mar 25 17:36 1774455977 - I want to explore fixing performance-UX, when “ignoring” or “processing”, the list re-comput.md
-rw-r--r--@  1 admin  staff  122115 Mar 25 23:59 1774456978 - I'd like to allow categorizing contrails that are already processed. (create category, add to catego.md
-rw-r--r--@  1 admin  staff   75704 Mar 26 12:35 1774522519 - For the windows build, I just tried it on a windows machine and this is my feedback-.md
-rw-r--r--@  1 admin  staff   14245 Mar 26 22:52 1774561440 - If “No chats yet…”, don’t display the “Process All Now” button nor the “Process now”.md
-rw-r--r--@  1 admin  staff   13356 Mar 26 23:26 1774563416 - We now have fully supported Windows (and we already support MacOS).md
```

</details>
- **Read**: `Read `/Users/admin/Documents/projects/contrails/agent_contrails/1774522519 - For the windows build, I just tried it on a windows machine and this is my feedback-.md``
- **Read**: `Read `/Users/admin/Documents/projects/contrails/agent_contrails/1774522519 - For the windows build, I just tried it on a windows machine and this is my feedback-.md``
- **Read**: `Read `/Users/admin/Documents/projects/contrails/agent_contrails/1774563416 - We now have fully supported Windows (and we already support MacOS).md``
<details>
<summary>Bash: `Ran `head -100 /Users/admin/Documents/projects/contrails/app.go``</summary>

```
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"contrails/agent"
	"contrails/agent/claudecode"
	"contrails/agent/cursor"
	"contrails/agent/vscode"

	"github.com/google/uuid"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
...
```

</details>

</details>

- **Agent**: `Agent: Research Wails v2 Linux support`

<details>
<summary>Subagent Activity (50 tool calls)</summary>

Research Wails v2 Linux support comprehensively. This is a RESEARCH ONLY task - do not write any code.

Search the web for:
1. Wails v2 Linux requirements and dependencies (webkit2gtk, etc.)
2. Wails v2 Linux supported distributions and versions
3. Wails v2 Linux build requirements (CGO, gcc, pkg-config, etc.)
4. Wails v2 Linux known issues and limitations compared to macOS/Windows
5. Wails v2 Linux packaging options (AppImage, .deb, .rpm, Flatpak, Snap)
6. Wails v2 Linux window options and styling differences
7. Wails v2 Linux CI/CD considerations (GitHub Actions runners, dependencies)
8. How other Wails v2 apps handle Linux distribution
9. Linux equivalents for app data paths (XDG Base Directory specification)
10. Linux auto-updater strategies for desktop apps (AppImage update, etc.)

Try these searches and web fetches:
- Search: "Wails v2 Linux requirements webkit2gtk dependencies"
- Search: "Wails v2 Linux build guide distribution packaging"
- Search: "Wails v2 Linux known issues limitations"
- Search: "Wails v2 Linux AppImage packaging"
- Search: "Linux XDG Base Directory VS Code config path"
- Search: "Linux desktop app auto-update strategy AppImage"
- Fetch: https://wails.io/docs/gettingstarted/installation
- Fetch: https://wails.io/docs/guides/linux/
- Search: "Wails v2 Linux CI GitHub Actions ubuntu webkit2gtk"
- Search: "Cursor editor Linux config path location"

Compile all findings into a detailed report.

- **Bash**: `Ran `curl -s -L "https://wails.io/docs/gettingstarted/installation" 2>/dev/null | head -3000``
- **Bash**: `Ran `curl -s -L "https://wails.io/docs/guides/linux/" 2>/dev/null | head -3000``
<details>
<summary>Bash: `Ran `curl -s -L "https://wails.io/docs/reference/project-config" 2>/dev/null | head -3000``</summary>

```
<!DOCTYPE html><html lang="en-US"><head><title>Just a moment...</title><meta http-equiv="Content-Type" content="text/html; charset=UTF-8"><meta http-equiv="X-UA-Compatible" content="IE=Edge"><meta name="robots" content="noindex,nofollow"><meta name="viewport" content="width=device-width,initial-scale=1"><style>*{box-sizing:border-box;margin:0;padding:0}html{line-height:1.15;-webkit-text-size-adjust:100%;color:#313131;font-family:system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,"Helvetica Neue",Arial,"Noto Sans",sans-serif,"Apple Color Emoji","Segoe UI Emoji","Segoe UI Symbol","Noto Color Emoji"}body{display:flex;flex-direction:column;height:100vh;min-height:100vh}.main-content{margin:8rem auto;padding-left:1.5rem;max-width:60rem}@media (width <= 720px){.main-content{margin-top:4rem}}#challenge-error-text{background-image:url("data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIzMiIgaGVpZ2h0PSIzMiIgZmlsbD0ibm9uZSI+PHBhdGggZmlsbD0iI0IyMEYwMyIgZD0iTTE2IDNhMTMgMTMgMCAxIDAgMTMgMTNBMTMuMDE1IDEzLjAxNSAwIDAgMCAxNiAzbTAgMjRhMTEgMTEgMCAxIDEgMTEtMTEgMTEuMDEgMTEuMDEgMCAwIDEtMTEgMTEiLz48cGF0aCBmaWxsPSIjQjIwRjAzIiBkPSJNMTcuMDM4IDE4LjYxNUgxNC44N0wxNC41NjMgOS41aDIuNzgzem0tMS4wODQgMS40MjdxLjY2IDAgMS4wNTcuMzg4LjQwNy4zODkuNDA3Ljk5NCAwIC41OTYtLjQwNy45ODQtLjM5Ny4zOS0xLjA1Ny4zODktLjY1IDAtMS4wNTYtLjM4OS0uMzk4LS4zODktLjM5OC0uOTg0IDAtLjU5Ny4zOTgtLjk4NS40MDYtLjM5NyAxLjA1Ni0uMzk3Ii8+PC9zdmc+");background-repeat:no-repeat;background-size:contain;padding-left:34px}</style><meta http-equiv="refresh" content="360"></head><body><div class="main-wrapper" role="main"><div class="main-content"><noscript><div class="h2"><span id="challenge-error-text">Enable JavaScript and cookies to continue</span></div></noscript></div></div><script>(function(){window._cf_chl_opt = {cvId: '3',cZone: 'wails.io',cType: 'managed',cRay: '9e299a074d90d592',cH: 'rijK1LqgwZIu4CyY4nYy6Is5LvPWt0XGnNp2VMDx5TY-1774564245-1.2.1.1-qVLN7srW.TYwBj44CC.uaW2H1RuBuevGzOpd.eaxE2AmHA_u4oqP__It4MK85Q4J',cUPMDTk:"/docs/gettingstarted/installation?__cf_chl_tk=lBsNXijGkbla46qF63YzI6qaWbeve6PYbjxkpRn4o1w-1774564245-1.0.1.1-Cynax2BaYb8nR070_uXnBdLveaQxMjc12I2iMTXXW.g",cFPWv: 'g',cITimeS: '1774564245',cTplC:0,cTplO:0,cTplV:5,cTplB: '0',fa:"/docs/gettingstarted/installation?__cf_chl_f_tk=lBsNXijGkbla46qF63YzI6qaWbeve6PYbjxkpRn4o1w-1774564245-1.0.1.1-Cynax2BaYb8nR070_uXnBdLveaQxMjc12I2iMTXXW.g",md: 'JdSwpp2ihvulw5up3GpZOk6kOaCH6JwLjPnrQTsCiro-1774564245-1.2.1.1-g.6VdvR030rznaWYNqSLjg6gEMrvzfhggibDaupiE2_DJ5PAXz4nVeuIiMypv3xpWz8Psrv9J8irgUwJCIRRKMRSxVjnO.w5NyS1zugcPNptmf5NYIgeOpr1xYhKnwIqE793r4oDWcIJ6dGGZphqOiXKuOQu_kyFriNLYn.zMXlFoRUWaGBNhL_S14R24R9RmPcON_XGBfmusel8D1ZQR0maYDfKxYAkWNyjqlhp_5Zb.2O2F1tPKil2DShw_mixTshmvvjYvlCEt2MSs1dESSLv.18ibH2rfnCWExQEffUXyljyzKSnxDyn3BkD2a1phwdoFkWvPhpwPXrOgdDXx70Fu._uel5SmIzPoCR8Uqm4U.dH6xycUbLi6kzhSOx5v2OPtKq4YOTN8yitshvUnk6gxtauM0pNOcyTzZNsbBAPfrjDNP24TBVzzz.gz2xP0nJFLVNt4HRYSJbKjLDeWbc5XzjqnbDiq6xBfvC741whC81xjj3bONdrMOAhYi8i5vlufEcr5cpjljvpL_z54JMXJn84926l2SLgMuT8YE12E9xFEPCgQIt7KSSwYjwBkDgPfto1NQMskeQ0wIPrBcaWuREgY3.AeIcoKhJR2hco4IAR8SEhRYceZTgRiLUHQpMDx9Ums6Wo6xiqVfZ05gFHyJxb3w.ZHvbL543vJnUXkS2Guz4yHoOKvmEH5QKVtBIJxLhLhdpl.zGaq0lvSpajGHr7uGaJX3GR2jvLBXgKFewRNRf3isQUw.4FrLBJDrN.bCoNz86Vx1KYmClIRvuKI9X0yQuYqngyOzSrb9KsDId0RYsWpFiEtMl4Txtxx0a4O2RFdJJGAgkydZgCaUvwB2Z1T.fkA8qlz4lNe3r0_PnI8Px2Zm87SKeo3iFvuvvfBPM3kZGWvWE9bcLmhdbJ3Fttus3HZm_Bq_PhFuowUCgJFQIA1Bj2nM7BJpQh1z0mHjEDD99H5vyLkbqkAr9G9UT5QzVhVpkQL_1JoYw',mdrd: 'WaDrt9ut3AsnICzctc3PN01hvbNMuf2.iOzzMqkdBaY-1774564245-1.2.1.1-ejPY5G3xuKUc8NLLjqrS_P0rwzeAHnrrFU0phn5UY.OLi4vS.Gfhhbjmvp6aYvQNifn8DqrjD8hfulbX6MXzAjcNCr55uYm7jVMiCsHfp0UEL4dCzpryuVOAP1mSCKXuR9wPJyQB84DOZZvaNlgaF8BkDSHNqyNtVzyhylNfHq.knefHHbQMSBWGtxvDevUbGoM_QjEMLFuUCbJJ1CjRZ8QTEYHDP.f9Ayde4yM_dfg8PUHA9ldiNF1PDXbFzTq3',};var a = document.createElement('script');a.src = '/cdn-cgi/challenge-platform/h/g/orchestrate/chl_page/v1?ray=9e299a074d90d592';window._cf_chl_opt.cOgUHash = location.hash === '' && location.href.indexOf('#') !== -1 ? '#' : location.hash;window._cf_chl_opt.cOgUQuery = location.search === '' && location.href.slice(0, location.href.length - window._cf_chl_opt.cOgUHash.length).indexOf('?') !== -1 ? '?' : location.search;if (window.history && window.history.replaceState) {var ogU = location.pathname + window._cf_chl_opt.cOgUQuery + window._cf_chl_opt.cOgUHash;history.replaceState(null, null,"/docs/gettingstarted/installation?__cf_chl_rt_tk=lBsNXijGkbla46qF63YzI6qaWbeve6PYbjxkpRn4o1w-1774564245-1.0.1.1-Cynax2BaYb8nR070_uXnBdLveaQxMjc12I2iMTXXW.g"+ window._cf_chl_opt.cOgUHash);a.onload = function() {history.replaceState(null, null, ogU);}}document.getElementsByTagName('head')[0].appendChild(a);}());</script></body></html>
```

</details>
```
<!DOCTYPE html><html lang="en-US"><head><title>Just a moment...</title><meta http-equiv="Content-Type" content="text/html; charset=UTF-8"><meta http-equiv="X-UA-Compatible" content="IE=Edge"><meta name="robots" content="noindex,nofollow"><meta name="viewport" content="width=device-width,initial-scale=1"><style>*{box-sizing:border-box;margin:0;padding:0}html{line-height:1.15;-webkit-text-size-adjust:100%;color:#313131;font-family:system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,"Helvetica Neue",Arial,"Noto Sans",sans-serif,"Apple Color Emoji","Segoe UI Emoji","Segoe UI Symbol","Noto Color Emoji"}body{display:flex;flex-direction:column;height:100vh;min-height:100vh}.main-content{margin:8rem auto;padding-left:1.5rem;max-width:60rem}@media (width <= 720px){.main-content{margin-top:4rem}}#challenge-error-text{background-image:url("data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIzMiIgaGVpZ2h0PSIzMiIgZmlsbD0ibm9uZSI+PHBhdGggZmlsbD0iI0IyMEYwMyIgZD0iTTE2IDNhMTMgMTMgMCAxIDAgMTMgMTNBMTMuMDE1IDEzLjAxNSAwIDAgMCAxNiAzbTAgMjRhMTEgMTEgMCAxIDEgMTEtMTEgMTEuMDEgMTEuMDEgMCAwIDEtMTEgMTEiLz48cGF0aCBmaWxsPSIjQjIwRjAzIiBkPSJNMTcuMDM4IDE4LjYxNUgxNC44N0wxNC41NjMgOS41aDIuNzgzem0tMS4wODQgMS40MjdxLjY2IDAgMS4wNTcuMzg4LjQwNy4zODkuNDA3Ljk5NCAwIC41OTYtLjQwNy45ODQtLjM5Ny4zOS0xLjA1Ny4zODktLjY1IDAtMS4wNTYtLjM4OS0uMzk4LS4zODktLjM5OC0uOTg0IDAtLjU5Ny4zOTgtLjk4NS40MDYtLjM5NyAxLjA1Ni0uMzk3Ii8+PC9zdmc+");background-repeat:no-repeat;background-size:contain;padding-left:34px}</style><meta http-equiv="refresh" content="360"></head><body><div class="main-wrapper" role="main"><div class="main-content"><noscript><div class="h2"><span id="challenge-error-text">Enable JavaScript and cookies to continue</span></div></noscript></div></div><script>(function(){window._cf_chl_opt = {cvId: '3',cZone: 'wails.io',cType: 'managed',cRay: '9e299a334f5bb93e',cH: 'gn9fNusFDf80dLtumLfL8NDtZpGi447kPPJRzsUd9qw-1774564252-1.2.1.1-U6_i5hXN8U8Jx9viIpRt2yB6ERfBmtaWxk5UiP60I1RF6F5Vp2fAelREEt7eQ3ew',cUPMDTk:"/docs/guides/linux/?__cf_chl_tk=2Aud7rfPivbhV6_UNcI_t5oA8KAkUAC1XFtRfOhUoiI-1774564252-1.0.1.1-mUKErRt0SeYHotQL4mjHu0JNAGtggunj_21i11A9nJM",cFPWv: 'g',cITimeS: '1774564252',cTplC:0,cTplO:0,cTplV:5,cTplB: '0',fa:"/docs/guides/linux/?__cf_chl_f_tk=2Aud7rfPivbhV6_UNcI_t5oA8KAkUAC1XFtRfOhUoiI-1774564252-1.0.1.1-mUKErRt0SeYHotQL4mjHu0JNAGtggunj_21i11A9nJM",md: '8MoIhnEmDT3WxNcB8kMgIqH5.NSj98TdB9z7JJas9Fo-1774564252-1.2.1.1-_fcUk33TG9hH90DnjzBj50qROnGcamIUHBghoIUb.ecIPi2pt828etmdkswF_uXv1GQdkhoL77XgZ0JThCzT7a_qmRgxRGxDSnZKWpwiQW_S1B.nKcpKVVWMthsT06VNVdU9_ZRb3dG4u.1jppEMnXkX38G1lJZmUvqCL.mIK841gkxsZniMgmM8ehHqXtC2RE5PC3eFS_h9WXczCMM1BlWLZ61_P3J0RlGfAwfHXRuI4Oadd.e9rg0RWXXZBvrgw9MI6ssHSqexVLOC4DTJHqRgHYs0vEwwDX.6CyKweOyDlORE2U.1Ge6lDMIoW0kHQwPmXHzoCuMa0L9D_RX3mpALJFuckIxpzM_pxWBSY07GwSjqPemLq68FBHVamnbZTDfZkjB0RWFEnTpjkErGPoggODi_EPhvIL3GzOJh_u9tsLoInNJB2GZKtdSDWA114pvaMc67cGroKtSRc3fN73tc8YNlMynn4tXZlX5gq7lLwENRzy09nocnyJ7uxLIdZXg.KT3iJnhb3ZZ2u4SfI.eN7V2Art5AdMhfPO.ashK5AysahRPqUFwDv_zsFz4ehwTSpOCkt8x4vMzSm.UQwNgAYcblvQkRvPMqgLM84_A1ZnkV_r745r_V2nigSM2uGpo6y4dAo.ttMn8m.hlB2fsPWkDupRc0jsPg_p6O3aBWoHJ5IEYHp1NdB9YcrJkjJcbf3xmu5vF_xjZW6pM9DltMWgnqDJ4z64PbGaCfXUFeUo5597CEMKFOi.E6AnohGJDhT7OvA71la25fYB_bhc_PeQ.cLiyWhh_1Hm3KZgEFKGu6NtCs6Ir0J0bTOIWQvUoVeOIMgWsrhgsarbao8H5NSZJJAQPmIZyzYHO.xbiZhH6WE9F2PicxEQJHFh7wh9uAnfq3T3O3GrMCaknT1z7dQF8wc4MFv0PvBgQD2YJR5ZC.X8sW6hMX2t2mFX42APElbfnFHdGY8Ci5Rj7bBQ',mdrd: 'GhPmzaV2DrdhC0EGX4soAHoKc84MtU5JbR2xJJfXPI0-1774564252-1.2.1.1-oEXRvvu1zyXc3_XhMv0X6Q5TGY7HbFU04LUtgwmg82Tbf2hvAs4l15EJkgKJqMegmXxDAkhmFREzmNtj2chWrQMHerYWpkVFugPQyJQdcxvhoWcFLc0j9AkaXQq0s0q6u9NqDnB0fEwTohlivlhiXFK7O1WzeuYf1eIfIUKR2BLQa4aLSL5D1.fZPwJ.IHERmo3k5iLqQNBHo4CiJjeXmlsrE4dyYIcU7XvN4y5DUydjZNc4onZYKxLMLBlNtQhR',};var a = document.createElement('script');a.src = '/cdn-cgi/challenge-platform/h/g/orchestrate/chl_page/v1?ray=9e299a334f5bb93e';window._cf_chl_opt.cOgUHash = location.hash === '' && location.href.indexOf('#') !== -1 ? '#' : location.hash;window._cf_chl_opt.cOgUQuery = location.search === '' && location.href.slice(0, location.href.length - window._cf_chl_opt.cOgUHash.length).indexOf('?') !== -1 ? '?' : location.search;if (window.history && window.history.replaceState) {var ogU = location.pathname + window._cf_chl_opt.cOgUQuery + window._cf_chl_opt.cOgUHash;history.replaceState(null, null,"/docs/guides/linux/?__cf_chl_rt_tk=2Aud7rfPivbhV6_UNcI_t5oA8KAkUAC1XFtRfOhUoiI-1774564252-1.0.1.1-mUKErRt0SeYHotQL4mjHu0JNAGtggunj_21i11A9nJM"+ window._cf_chl_opt.cOgUHash);a.onload = function() {history.replaceState(null, null, ogU);}}document.getElementsByTagName('head')[0].appendChild(a);}());</script></body></html>
```
```
<!DOCTYPE html><html lang="en-US"><head><title>Just a moment...</title><meta http-equiv="Content-Type" content="text/html; charset=UTF-8"><meta http-equiv="X-UA-Compatible" content="IE=Edge"><meta name="robots" content="noindex,nofollow"><meta name="viewport" content="width=device-width,initial-scale=1"><style>*{box-sizing:border-box;margin:0;padding:0}html{line-height:1.15;-webkit-text-size-adjust:100%;color:#313131;font-family:system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,"Helvetica Neue",Arial,"Noto Sans",sans-serif,"Apple Color Emoji","Segoe UI Emoji","Segoe UI Symbol","Noto Color Emoji"}body{display:flex;flex-direction:column;height:100vh;min-height:100vh}.main-content{margin:8rem auto;padding-left:1.5rem;max-width:60rem}@media (width <= 720px){.main-content{margin-top:4rem}}#challenge-error-text{background-image:url("data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIzMiIgaGVpZ2h0PSIzMiIgZmlsbD0ibm9uZSI+PHBhdGggZmlsbD0iI0IyMEYwMyIgZD0iTTE2IDNhMTMgMTMgMCAxIDAgMTMgMTNBMTMuMDE1IDEzLjAxNSAwIDAgMCAxNiAzbTAgMjRhMTEgMTEgMCAxIDEgMTEtMTEgMTEuMDEgMTEuMDEgMCAwIDEtMTEgMTEiLz48cGF0aCBmaWxsPSIjQjIwRjAzIiBkPSJNMTcuMDM4IDE4LjYxNUgxNC44N0wxNC41NjMgOS41aDIuNzgzem0tMS4wODQgMS40MjdxLjY2IDAgMS4wNTcuMzg4LjQwNy4zODkuNDA3Ljk5NCAwIC41OTYtLjQwNy45ODQtLjM5Ny4zOS0xLjA1Ny4zODktLjY1IDAtMS4wNTYtLjM4OS0uMzk4LS4zODktLjM5OC0uOTg0IDAtLjU5Ny4zOTgtLjk4NS40MDYtLjM5NyAxLjA1Ni0uMzk3Ii8+PC9zdmc+");background-repeat:no-repeat;background-size:contain;padding-left:34px}</style><meta http-equiv="refresh" content="360"></head><body><div class="main-wrapper" role="main"><div class="main-content"><noscript><div class="h2"><span id="challenge-error-text">Enable JavaScript and cookies to continue</span></div></noscript></div></div><script>(function(){window._cf_chl_opt = {cvId: '3',cZone: 'wails.io',cType: 'managed',cRay: '9e299a38288d511b',cH: 'QN79HlaI9gT57zYb8xj3qy1gdEknNPbx65MTk.Ow.3g-1774564253-1.2.1.1-yr.a8YdmEBlvnMPDE32CATWDa8Qww9uAi6tDLBG1zhra38msHbKqjaUS4ormzqxy',cUPMDTk:"/docs/reference/project-config?__cf_chl_tk=5zKyRP0phApVx5nKP7tK3V0NLKQpm57Ml_twh9AqYbw-1774564253-1.0.1.1-ybMEnfeWGUJqLM2Qc6lOcVqXkoh2wKBEEF5YXoSEmgA",cFPWv: 'g',cITimeS: '1774564253',cTplC:0,cTplO:0,cTplV:5,cTplB: '0',fa:"/docs/reference/project-config?__cf_chl_f_tk=5zKyRP0phApVx5nKP7tK3V0NLKQpm57Ml_twh9AqYbw-1774564253-1.0.1.1-ybMEnfeWGUJqLM2Qc6lOcVqXkoh2wKBEEF5YXoSEmgA",md: 'wXqiqRA3QQhRv5b_MNSJkRJiugh8_EabgaU0X9BH0gI-1774564253-1.2.1.1-EyvyQWa1ozZRSDW22yDJZ1OLVAP9X2GyQLhXjf0WWZUC4h3tSK.Fxol95UuO0YUP0fQ_KnLUbivrfCAvXGSVPyzlqJ25U2q21eDMdAsQgQoJVjsdHsXEe6JybnLSQ2N7TuYO7IKZAOIwBW6Zzgc7F2tSu1Xp07Xi5mV8dnfwox1nB4IDIyBscezjJs6ahWs0aojFhyaR4qzgbR7UN.YdzqAJxP2szOTzinolHUW13QJ46HEDYy23EMrt9uFHPTQbAR9qdc8o_3niwuIX2rcoC303wVl.sh4igpVUb94bACe0UpjziUe.XOQ6uqyz76CqCI_eqNn84QiY4AZ6bCvpVD8K0XtzTQT045vbjkIyh4DwKKn7NGrXEiUsmLT5kIOY_ChJ6jKsGQFv1FLdURxVkecQS2RO2zf87VEGa2VblwShc62PEygEekY55zp120U0HUK_RRYVmpN_au8yvKAlzgVL3V7TwvwOhLeAMAQr8nvdathQv2lBS6VR0mQR38XSWXDMgCB7Lz1hb6Tr_dWGQGUQX4RVakyOhSVYsw4nfW7bk28YyqFmzqDlGj3f9Bq.yVcFCf.HUYsu8MNLt2tWVt42BrBEjyeLND5NJyU9asQkUQbw318271MAUlB5lRWbk84NfK8cG53nDI24gbbCHzKjgoST0Zu62ckPwET2Asb72HMnMTL7KHXM7ZJNNJPlSTeygHvpaCumwzYRp_OayXLqebQLkJCx8c.MG4XT3M9GsplHnmD2IJ3r.MZ5PvJbwSn.GTC_mZnzjOWHA3aAa9B9E_JxwSjXaVg717x6xOvwKSY04e3cCCempKB1Tf8G5Fjm7ChurmLr2LjKyQUgkYhnUZNGtVjhE3g5DsVecLBUf8_3.He5rUc.TFevu0kS2FAPqzAxhFtLx0bbfbmhuYy9SgkGBgEt4Jqp6pBOwihAatgrPAqmMBVfClMc0uGkxxiOhugXJmRWtf9exMmCTA',mdrd: 'R_p8HRaJLeo7qPmV0LWAd4Ulgl8a2i79cSE0YUSlC2U-1774564253-1.2.1.1-wKE1Wx.F.aZ3569qnTxDN6Exo0g0SlqG6LYHObT9ReIXBwVNIscQUIBSfFnqUD1fwI3HP7MslgZHAlWD.pIfJZij7uFRw.YG2avJ4qgGc.Ih.AeqcE50xsVz8BjIdCznqs8Ww7B0LFT64hXr7LZg.Jwha5XI40ecxUfVTXMgeRr5SM7ymT00vH8EPcONdzM7Wh_VUEIN7GZ1E_V_gsGFQrl6tLRPV2_j.cVNsqQT0d95yQeEWFFyhF0emNd18Gwn',};var a = document.createElement('script');a.src = '/cdn-cgi/challenge-platform/h/g/orchestrate/chl_page/v1?ray=9e299a38288d511b';window._cf_chl_opt.cOgUHash = location.hash === '' && location.href.indexOf('#') !== -1 ? '#' : location.hash;window._cf_chl_opt.cOgUQuery = location.search === '' && location.href.slice(0, location.href.length - window._cf_chl_opt.cOgUHash.length).indexOf('?') !== -1 ? '?' : location.search;if (window.history && window.history.replaceState) {var ogU = location.pathname + window._cf_chl_opt.cOgUQuery + window._cf_chl_opt.cOgUHash;history.replaceState(null, null,"/docs/reference/project-config?__cf_chl_rt_tk=5zKyRP0phApVx5nKP7tK3V0NLKQpm57Ml_twh9AqYbw-1774564253-1.0.1.1-ybMEnfeWGUJqLM2Qc6lOcVqXkoh2wKBEEF5YXoSEmgA"+ window._cf_chl_opt.cOgUHash);a.onload = function() {history.replaceState(null, null, ogU);}}document.getElementsByTagName('head')[0].appendChild(a);}());</script></body></html>
```
- **ToolSearch**: `max_results, query`
- **WebSearch**: `Search `Wails v2 Linux requirements webkit2gtk dependencies installation``
- **WebSearch**: `Search `Wails v2 Linux build guide distribution packaging AppImage deb rpm``
- **WebSearch**: `Search `Wails v2 Linux known issues limitations compared macOS Windows``
- **WebSearch**: `Search `Wails v2 Linux CI GitHub Actions ubuntu webkit2gtk build``
- **WebSearch**: `Search `Linux XDG Base Directory specification app config data path``
<details>
<summary>WebSearch: `Search `Linux desktop app auto-update strategy AppImage Electron alternative``</summary>

```
Web search results for query: "Wails v2 Linux requirements webkit2gtk dependencies installation"

Links: [{"title":"Installation | Wails","url":"https://wails.io/docs/gettingstarted/installation/"},{"title":"Installation and Setup | wailsapp/wails | DeepWiki","url":"https://deepwiki.com/wailsapp/wails/2.1-installation"},{"title":"Ubuntu 24.04 dependency issue (libwebkit) · Issue #3581 · wailsapp/wails","url":"https://github.com/wailsapp/wails/issues/3581"},{"title":"Options | Wails","url":"https://wails.io/docs/reference/options/"},{"title":"Linux Distro Support | Wails","url":"https://wails.io/docs/guides/linux-distro-support/"},{"title":"Changelog | Wails","url":"https://wails.io/changelog/"},{"title":"libwebkit2gtk-4.0 not available in Ubuntu 24 · Issue #3513 · wailsapp/wails","url":"https://github.com/wailsapp/wails/issues/3513"},{"title":"[linux] Add support for WebKit2GTK 2.36+ features by stffabi · Pull Request #2151 · wailsapp/wails","url":"https://github.com/wailsapp/wails/pull/2151"},{"title":"Installation","url":"https://v3alpha.wails.io/quick-start/installation/"},{"title":"[Documentation]Adding notes for latest linux version and libwebkit issue by pratikmota · Pull Request #3806 · wailsapp/wails","url":"https://github.com/wailsapp/wails/pull/3806"}]

Here's what I found about Wails v2 Linux requirements and webkit2gtk dependencies:

## Installation Approach

Rather than listing commands for different distros, Wails can determine installation commands for your specific distribution by running `wails doctor` after installation.

## WebKit2GTK Dependencies

The Linux platform implementation uses WebKit2GTK, which is the core dependency needed. However, there's an important version consideration:

### Standard Installation
For most Linux distributions, the standard webkit2gtk-4.0 package is required:
- **Ubuntu/Debian**: `libwebkit2gtk-4.0-dev`

### Ubuntu 24.04+ Issue

...
```

</details>
```
Web search results for query: "Wails v2 Linux build guide distribution packaging AppImage deb rpm"

Links: [{"title":"Build Customization","url":"https://v3alpha.wails.io/guides/build/customization/"},{"title":"Linux Distro Support | Wails","url":"https://wails.io/docs/guides/linux-distro-support/"},{"title":"[V3-Linux] Support for deb,rpm,arch linux packager packaging by atterpac · Pull Request #3909 · wailsapp/wails","url":"https://github.com/wailsapp/wails/pull/3909"},{"title":"Installation | Wails","url":"https://wails.io/docs/gettingstarted/installation/"},{"title":"CLI Reference","url":"https://v3alpha.wails.io/guides/cli/"},{"title":"Your First Application","url":"https://v3alpha.wails.io/getting-started/your-first-app/"},{"title":"Creating Installers","url":"https://v3alpha.wails.io/guides/installers/"},{"title":"Installation","url":"https://v3alpha.wails.io/quick-start/installation/"},{"title":"Linux App Packaging via AppImage","url":"https://blog.jamesreed.dev/packaging-apps-for-linux-with-appimage"},{"title":"Linux | Wails","url":"https://wails.io/docs/guides/linux/"}]

Based on the search results, here's information about Wails v2 Linux build distribution packaging for AppImage, deb, and rpm formats:

## Linux Package Support

The Linux-specific Taskfile includes tasks for building, packaging, and running the application on Linux, with key features including creating AppImage, deb, rpm, and Arch Linux packages.

## Package Formats Available

Wails can generate Linux packages in deb, rpm, and archlinux formats. Wails uses nFPM to quickly release packaged applications for specific Linux distros including .deb (Debian based Linux) and .rpm (Redhat Package Manager for Fedora).

## Build and Packaging Process

For AppImage, you use the appimagetool to create the package from an AppDir directory. The documentation also mentions approaches using tools like GoReleaser for automated packaging with support for multiple formats.

## Getting Started

...
```
```
Web search results for query: "Wails v2 Linux known issues limitations compared macOS Windows"

Links: [{"title":"Wails dev/build very slow on Windows compared to MacOS/Linux · Issue #2545 · wailsapp/wails","url":"https://github.com/wailsapp/wails/issues/2545"},{"title":"Changelog | Wails","url":"https://wails.io/changelog/"},{"title":"🚀 Building Desktop Apps with Wails: A Go Developer's Perspective - DEV Community","url":"https://dev.to/kaizerpwn/building-desktop-apps-with-wails-a-go-developers-perspective-526p"},{"title":"[v2, mac] Could not create transparent window · Issue #3036 · wailsapp/wails","url":"https://github.com/wailsapp/wails/issues/3036"},{"title":"Wails v2 Beta for MacOS | Wails","url":"https://wails.io/blog/wails-v2-beta-for-mac/"},{"title":"Installation | Wails","url":"https://wails.io/docs/gettingstarted/installation/"},{"title":"Options | Wails","url":"https://wails.io/docs/reference/options/"},{"title":"[v2 linux] Cross-compilation targetting windows doesn't work in v2 but works with manual build · Issue #1921 · wailsapp/wails","url":"https://github.com/wailsapp/wails/issues/1921"},{"title":"Cross-compiling for Windows on Linux for command line components. · Issue #2995 · wailsapp/wails","url":"https://github.com/wailsapp/wails/issues/2995"},{"title":"Cannot cross compile for windows on Mac after updating from v2.9.2 to v2.10.1 · Issue #4112 · wailsapp/wails","url":"https://github.com/wailsapp/wails/issues/4112"}]

Based on the search results, here are some key known issues and limitations for Wails v2 on Linux compared to macOS and Windows:

## Linux-Specific Issues and Limitations

**WebKit2GTK Dependency Issues:**
On latest Linux versions (like Ubuntu 24.04), libwebkit2gtk-4.0-dev may not be supported, requiring installation of libwebkit2gtk-4.1-dev instead and building with the tag `-tags webkit2_41`.

**Window Manager Compatibility:**
Gnome on Wayland does not display window icons, requiring a .desktop file to show application icons. Additionally, window translucency may be ignored by some window managers or result in a black window.

**Cross-Compilation Issues:**
The right mouse event was not being raised correctly for Linux, though this appears to have been fixed in recent versions.

**General Limitations Across All Platforms:**
One notable limitation is the lack of support for cookies, which some developers work around using global window variables.

...
```
```
Web search results for query: "Wails v2 Linux CI GitHub Actions ubuntu webkit2gtk build"

Links: [{"title":"allow builds for webkit2gtk-4.1 by atterpac · Pull Request #3465 · wailsapp/wails","url":"https://github.com/wailsapp/wails/pull/3465"},{"title":"GitHub - dAppServer/wails-build-action: USE: host-uk/build@v4 · GitHub","url":"https://github.com/dAppServer/wails-build-action"},{"title":"libwebkit error on Ubuntu 24.04 · Issue #3587 · wailsapp/wails","url":"https://github.com/wailsapp/wails/issues/3587"},{"title":"Installation and Setup | wailsapp/wails | DeepWiki","url":"https://deepwiki.com/wailsapp/wails/2.1-installation"},{"title":"[linux] Add support for WebKit2GTK 2.36+ features by stffabi · Pull Request #2151 · wailsapp/wails","url":"https://github.com/wailsapp/wails/pull/2151"},{"title":"libwebkit2gtk-4.0 not available in Ubuntu 24 · Issue #3513 · wailsapp/wails","url":"https://github.com/wailsapp/wails/issues/3513"},{"title":"[Documentation]Adding notes for latest linux version and libwebkit issue by pratikmota · Pull Request #3806 · wailsapp/wails","url":"https://github.com/wailsapp/wails/pull/3806"},{"title":"libwebkit2gtk-4.0 issue? - ubuntu · Issue #2971 · wailsapp/wails","url":"https://github.com/wailsapp/wails/issues/2971"},{"title":"Ubuntu 24.04 dependency issue (libwebkit) · Issue #3581 · wailsapp/wails","url":"https://github.com/wailsapp/wails/issues/3581"},{"title":"Wails Action CI/CD - GitHub Marketplace","url":"https://github.com/marketplace/actions/wails-action-ci-cd"}]

Based on the search results, here's what I found about Wails v2 Linux CI with GitHub Actions and webkit2gtk builds:

## Key Issue and Solution

Ubuntu 24.04 no longer provides libwebkit2gtk-4.0-dev, which has been replaced with libwebkit2gtk-4.1-dev.

## GitHub Actions Configuration

For Wails v2 Linux CI in GitHub Actions on Ubuntu, the required changes are:

In test.yml and release.yml workflows: change to libwebkit2gtk-4.1-dev and add -tags webkit2_41 for Linux builds. The webkit2_41 build tag tells Wails to use the newer webkit2gtk-4.1 API instead of the older 4.0 version.

## Affected Distributions

This is required for Ubuntu 24.04 (Noble Numbat), Debian 13 (Trixie), and any newer Linux distributions.

...
```
```
Web search results for query: "Linux XDG Base Directory specification app config data path"

Links: [{"title":"XDG Base Directory - ArchWiki","url":"https://wiki.archlinux.org/title/XDG_Base_Directory"},{"title":"XDG Base Directory Specification","url":"https://specifications.freedesktop.org/basedir/latest/"},{"title":"Use the XDG Base Directory Specification!","url":"https://xdgbasedirectoryspecification.com/"},{"title":"Syncthing and XDG Base Directory Specification - Development - Syncthing Community Forum","url":"https://forum.syncthing.net/t/syncthing-and-xdg-base-directory-specification/2023"},{"title":"XDGBaseDirectorySpecification - Debian Wiki","url":"https://wiki.debian.org/XDGBaseDirectorySpecification"},{"title":"XDG Base Directory Specification | Alchemists","url":"https://alchemists.io/articles/xdg_base_directory_specification"},{"title":"XDG cheat sheet · GitHub","url":"https://gist.github.com/roalcantara/107ba66dfa3b9d023ac9329e639bc58c"},{"title":"GitHub - bkuhlmann/xdg: A XDG Base Directory Specification implementation. · GitHub","url":"https://github.com/bkuhlmann/xdg"},{"title":"XDG Base Directory shell scripting tutorial | Ctrl blog","url":"https://www.ctrl.blog/entry/xdg-basedir-scripting.html"},{"title":"xdg/README.md at master · adrg/xdg","url":"https://github.com/adrg/xdg/blob/master/README.md"}]

Based on the search results, here's a comprehensive overview of the Linux XDG Base Directory specification for app configuration and data paths:

## XDG Base Directory Specification Overview

The XDG Base Directory Specification defines an organized folder and file structure for applications to store associated user configuration, cache, data, state, and runtime information on UNIX-like systems.

## Key Directory Paths

The specification defines these main directories for different types of application data:

### Configuration
$XDG_CONFIG_HOME defines the base directory relative to which user-specific configuration files should be stored. If $XDG_CONFIG_HOME is either not set or empty, a default equal to $HOME/.config should be used.

### Data
$XDG_DATA_HOME defines the base directory relative to which user-specific data files should be stored. If $XDG_DATA_HOME is either not set or empty, a default equal to $HOME/.local/share should be used.

...
```
```
Web search results for query: "Linux desktop app auto-update strategy AppImage Electron alternative"

Links: [{"title":"Auto Update - electron-builder","url":"https://www.electron.build/auto-update.html"},{"title":"GitHub - electron-userland/electron-builder: A complete solution to package and build a ready for distribution Electron app with “auto update” support out of the box","url":"https://github.com/electron-userland/electron-builder"},{"title":"Does electron-updater supports auto updating on Linux non AppImage packages i.e. deb, rpm etc? · Issue #6330 · electron-userland/electron-builder","url":"https://github.com/electron-userland/electron-builder/issues/6330"},{"title":"Guide to Distributing Electron Apps For Linux | Beekeeper Studio","url":"https://www.beekeeperstudio.io/blog/distribute-electron-apps-for-linux"},{"title":"Linux auto-update · Issue #1138 · electron-userland/electron-builder","url":"https://github.com/electron-userland/electron-builder/issues/1138"},{"title":"Making AppImages updateable — AppImage documentation","url":"https://docs.appimage.org/packaging-guide/optional/updates.html"},{"title":"electron-simple-updater - npm","url":"https://www.npmjs.com/package/electron-simple-updater"},{"title":"autoUpdater | Electron","url":"https://www.electronjs.org/docs/latest/api/auto-updater/"},{"title":"electron-updater - npm","url":"https://www.npmjs.com/package/electron-updater"},{"title":"GitHub - megahertz/electron-simple-updater: Simple way to enable update for the electron application · GitHub","url":"https://github.com/megahertz/electron-simple-updater"}]

I've completed the web search for your query. Here are the key findings about Linux desktop app auto-update strategies for AppImage and Electron:

## Key Findings:

**Electron Auto-Update Support:**
- There is no built-in support for auto-updater on Linux, so it is recommended to use the distribution's package manager to update your app.

**electron-updater (Recommended for Electron apps):**
- electron-updater supports Linux formats including AppImage, rpm, and deb.
- Linux is supported (not only macOS and Windows).
- Different providers are supported out of the box: GitHub Releases, Amazon S3, DigitalOcean Spaces, Keygen and generic HTTP(s) server.

**AppImage Auto-Update Options:**
- AppImages are easy to build, don't require signing, they auto-update with electron-updater, and can be hosted on your website like a regular file.
- AppImageUpdate provides an efficient solution by comparing the local AppImage with the remote file, using all usable data from the existing file, and downloading only the remaining data.

...
```
- **WebSearch**: `Search `Wails v2 Linux window options frameless transparent titlebar styling``
- **WebSearch**: `Search `Wails v2 nfpm linux packaging deb rpm configuration``
- **WebSearch**: `Search `Cursor editor VS Code Linux config path ~/.config location``
- **WebFetch**: `Fetch `https://github.com/dAppServer/wails-build-action``
- **WebFetch**: `Fetch `https://deepwiki.com/wailsapp/wails/2.1-installation``
<details>
<summary>WebFetch: `Fetch `https://docs.appimage.org/packaging-guide/optional/updates.html``</summary>

```
Web search results for query: "Cursor editor VS Code Linux config path ~/.config location"

Links: [{"title":"User and workspace settings","url":"https://code.visualstudio.com/docs/configure/settings"},{"title":"Visual Studio Code tips and tricks","url":"https://code.visualstudio.com/docs/getstarted/tips-and-tricks"},{"title":"Cursor Setup for WSL - Scott Spence","url":"https://scottspence.com/posts/cursor-setup-for-wsl"},{"title":"Import VSCode settings to Linux - Bug Reports - Cursor - Community Forum","url":"https://forum.cursor.com/t/import-vscode-settings-to-linux/2674"},{"title":"Hypereal","url":"https://hypereal.cloud/a/how-to-install-cursor-on-windows-mac-linux"},{"title":"Transforming Cursor into a VSCode-like Experience in WSL: Setting Up the AI Code Editor | by Abdullah Alqahtani | Medium","url":"https://anqorithm.medium.com/transforming-cursor-into-a-vscode-like-experience-in-wsl-setting-up-the-ai-code-editor-fee5a531015d"},{"title":"VS Code Migration | Cursor Docs","url":"https://cursor.com/docs/configuration/migrations/vscode"},{"title":"Where is the default environment settings path? - Help - Cursor - Community Forum","url":"https://forum.cursor.com/t/where-is-the-default-environment-settings-path/17269"},{"title":"How to set up Cursor for the first time","url":"https://daily.dev/blog/setup-cursor-first-time"},{"title":"Sync Cursor Settings the Dotfiles Way - DEV Community","url":"https://dev.to/0916dhkim/sync-cursor-settings-the-dotfiles-way-20c9"}]

Based on the web search results, here's the configuration path information for Cursor editor on Linux:

## Cursor Configuration Path on Linux

On Linux, VS Code settings are located at `~/.config/Code/User/settings.json` and Cursor settings are at `~/.config/Cursor/User/settings.json`.

### Key Configuration Locations:

- **Settings**: `~/.config/Cursor/User/settings.json`
- **Extensions**: `~/.config/Cursor/` (extensions directory)
- **User data**: `~/.config/Cursor/`
- **Cache**: `~/.cache/Cursor/`

### VS Code to Cursor Migration

When you first open Cursor, it offers to import your VS Code configuration, which includes extensions, settings (settings.json), keybindings, snippets, and themes. On Linux, you can manually copy settings with: `cp ~/.config/Code/User/settings.json ~/.config/Cursor/User/settings.json`.
...
```

</details>
```
Web search results for query: "Wails v2 nfpm linux packaging deb rpm configuration"

Links: [{"title":"[V3-Linux] Support for deb,rpm,arch linux packager packaging by atterpac · Pull Request #3909 · wailsapp/wails","url":"https://github.com/wailsapp/wails/pull/3909"},{"title":"nFPM","url":"https://nfpm.goreleaser.com/"},{"title":"GitHub - goreleaser/nfpm: nFPM is Not FPM - a simple deb, rpm, apk, ipk, and arch linux packager written in Go · GitHub","url":"https://github.com/goreleaser/nfpm"},{"title":"Tips & Hints – nFPM","url":"https://nfpm.goreleaser.com/docs/tips/"},{"title":"Correct nfpm.yaml template package dependencies. by Mihara · Pull Request #4481 · wailsapp/wails","url":"https://github.com/wailsapp/wails/pull/4481"},{"title":"nfpm package - github.com/goreleaser/nfpm - Go Packages","url":"https://pkg.go.dev/github.com/goreleaser/nfpm"},{"title":"nfpm_rpm_package | Pantsbuild","url":"https://www.pantsbuild.org/dev/reference/targets/nfpm_rpm_package"},{"title":"nfpm package - github.com/goreleaser/nfpm/v2 - Go Packages","url":"https://pkg.go.dev/github.com/goreleaser/nfpm/v2"},{"title":"Your First Application","url":"https://v3alpha.wails.io/getting-started/your-first-app/"},{"title":"nfpm_deb_package | Pantsbuild","url":"https://www.pantsbuild.org/dev/reference/targets/nfpm_deb_package"}]

Based on the search results, here's what I found about Wails v2 with nFPM for Linux packaging:

## Overview

Wails supports the ability to use nFPM to quickly release packaged applications for specific Linux distributions including .deb (Debian-based Linux) and .rpm (Redhat Package Manager for Fedora and related systems).

nFPM can create deb, rpm, apk, ipk, arch linux, and msix packages, with your package defined through a simple YAML configuration file.

## nFPM Configuration for Wails

Wails uses an `nfpm.yaml` template file for Linux packaging configuration. The nfpm.yaml template includes package dependencies for the built Wails application.

The dependency lists in the Linux packaging template were updated to replace development and build tool packages with their corresponding runtime libraries for Debian, Ubuntu, RPM-based, and Arch Linux distributions.

## Package Format Support

...
```
```
Web search results for query: "Wails v2 Linux window options frameless transparent titlebar styling"

Links: [{"title":"Wails - Application Options","url":"https://wails.io/docs/reference/options/"},{"title":"Options | Wails","url":"https://wails.io/docs/next/reference/options/"},{"title":"Frameless Applications | Wails","url":"https://wails.io/docs/guides/frameless/"},{"title":"[v2, mac] Could not create transparent window · Issue #3036 · wailsapp/wails","url":"https://github.com/wailsapp/wails/issues/3036"},{"title":"[v2] Can the background be transparent? · Issue #1296 · wailsapp/wails","url":"https://github.com/wailsapp/wails/issues/1296"},{"title":"Window Management | wailsapp/wails | DeepWiki","url":"https://deepwiki.com/wailsapp/wails/4.1-window-api"},{"title":"Window | Wails","url":"https://wails.io/docs/reference/runtime/window/"},{"title":"Frameless Windows","url":"https://v3alpha.wails.io/features/windows/frameless/"},{"title":"[v2 Linux] Transparent window on linux · Issue #1923 · wailsapp/wails","url":"https://github.com/wailsapp/wails/issues/1923"},{"title":"Add Custom Title Bar in Frameless Application · wailsapp/wails · Discussion #1067","url":"https://github.com/wailsapp/wails/discussions/1067"}]

Based on the search results, here's information about Wails v2 Linux window options for frameless, transparent, and titlebar styling:

## Frameless Windows

When set to true, the window will have no borders or title bar. You can enable this with the `Frameless: true` option in your application configuration.

## Linux-Specific Options

For Linux, platform-specific configuration options include program name, GPU policy, and window translucency. The Linux options are available through the `github.com/wailsapp/wails/v2/pkg/options/linux` package.

## Transparent Windows

For transparency on Linux, you can set `WindowIsTranslucent: true` in the Linux options and use `BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 0}` for a fully transparent background. However, transparency requires a compositor (most modern desktop environments have one).

## Titlebar Styling

...
```
```
# GitHub Actions Workflow for Wails Linux Builds

Based on the repository documentation, here's the essential workflow configuration:

## Basic Setup

```yaml
- uses: snider/build@v3
  with:
    build-name: wailsApp
    build-platform: linux/amd64
```

## Key Components

**Required Inputs:**
- `build-name`: Application identifier for outputs
- `build-platform`: Target architecture (e.g., `linux/amd64`)

**Optional Controls:**
...
```
```
# Linux Installation Requirements for Wails v2

## Core Dependencies

Linux requires three essential packages for Wails development:

1. **GCC Build Tools** - C compiler for CGO compilation
2. **GTK3+ Development Libraries** (`libgtk3`)
3. **WebKit2GTK Libraries** - Either `libwebkit2gtk-4.0-dev` or `libwebkit2gtk-4.1-dev`

## Version Compatibility Note

"Ubuntu 24.04+ Note: Newer distributions may not support `libwebkit2gtk-4.0-dev`. Install `libwebkit2gtk-4.1-dev` instead and use the build tag `-tags webkit2_41` when building."

When targeting newer Ubuntu releases with WebKit2GTK 4.1, you must compile with the appropriate build tag to ensure compatibility.

## Distribution Detection

Rather than providing distribution-specific installation commands, Wails includes "intelligent detection in the `wails doctor` command that identifies your distribution and suggests appropriate installation commands."

...
```
```
# AppImage Update Mechanisms

## Update Methods

AppImages can be updated through three primary approaches:

1. **External tools** like `AppImageUpdate` or `appimagetool` command-line utilities
2. **Built-in updaters** bundled within the AppImage itself
3. **libappimageupdate integration** directly into the payload application

## Making AppImages Updateable via External Tools

Update information must be embedded directly in the AppImage by developers. This approach differs from traditional Linux distributions because "the information where to look for updates is not contained in separate repository description files...but is directly embedded inside the AppImage."

### Tools for Embedding Update Information

**appimagetool**: Uses the `-u` flag syntax:
```
appimagetool -u "zsync|https://example.com/app.AppImage.zsync"
```
...
```
- **WebSearch**: `Search `Wails v2 Linux distro support page supported distributions``
- **WebSearch**: `Search `wails v2 linux appimage packaging guide tutorial``
- **WebFetch**: `Fetch `https://github.com/wailsapp/wails/pull/4481``
<details>
<summary>WebFetch: `Fetch `https://github.com/wailsapp/wails/issues/1923``</summary>

```
Web search results for query: "Wails v2 Linux distro support page supported distributions"

Links: [{"title":"Linux Distro Support | Wails","url":"https://wails.io/docs/guides/linux-distro-support/"},{"title":"Installation | Wails","url":"https://wails.io/docs/gettingstarted/installation/"},{"title":"Add support for your Linux distro :: Wails","url":"https://wails.app/guides/distro/"},{"title":"Installation and Setup | wailsapp/wails | DeepWiki","url":"https://deepwiki.com/wailsapp/wails/2.1-installation"},{"title":"Linux | Wails","url":"https://wails.io/docs/guides/linux/"},{"title":"Installation","url":"https://v3alpha.wails.io/getting-started/installation/"},{"title":"Wails v2 Beta for Linux | Wails","url":"https://wails.io/blog/wails-v2-beta-for-linux/"},{"title":"Install wails on linux - DEV Community","url":"https://dev.to/farooquememon385/install-wails-on-linux-kph"},{"title":"[v2] [Linux] Feat - Wayland Support · Issue #1420 · wailsapp/wails","url":"https://github.com/wailsapp/wails/issues/1420"},{"title":"The Wails Project | Wails","url":"https://wails.io/"}]

I found the Wails v2 Linux distribution support page. Here's what I found:

Wails offers Linux support but providing installation instructions for all available distributions is an impossible task. Instead, Wails tries to determine if the packages you need to develop applications are available via your system's package manager.

Rather than maintaining a fixed list of supported distributions, Wails can try to determine what the installation commands are for your specific distribution by running wails doctor after installation to be shown how to install the dependencies.

The approach Wails takes is to support multiple package managers rather than individual distros. Wails attempts to find the correct package by iterating through a list of package names stored in the packagemanager specific file in the v2/internal/system/packagemanager directory.

If you're on a distribution that isn't supported, you can consult the Add Linux Distro guide if your distro/package manager is not supported.


REMINDER: You MUST include the sources above in your response to the user using markdown hyperlinks.
```

</details>
```
Web search results for query: "wails v2 linux appimage packaging guide tutorial"

Links: [{"title":"Linux App Packaging via AppImage","url":"https://blog.jamesreed.dev/packaging-apps-for-linux-with-appimage"},{"title":"Build Customization","url":"https://v3alpha.wails.io/guides/build/customization/"},{"title":"Your First Application","url":"https://v3alpha.wails.io/getting-started/your-first-app/"},{"title":"Packaging Guide — AppImage documentation","url":"https://docs.appimage.org/packaging-guide/index.html"},{"title":"Linux Distro Support | Wails","url":"https://wails.io/docs/guides/linux-distro-support/"},{"title":"Installation | Wails","url":"https://wails.io/docs/gettingstarted/installation/"},{"title":"Wails v2 Beta for Linux | Wails","url":"https://wails.io/blog/wails-v2-beta-for-linux/"},{"title":"linuxdeploy user guide — AppImage documentation","url":"https://docs.appimage.org/packaging-guide/from-source/linuxdeploy-user-guide.html"},{"title":"File Association | Wails","url":"https://wails.io/docs/v2.9.0/guides/file-association/"},{"title":"CLI Reference","url":"https://v3alpha.wails.io/guides/cli/"}]

I found several relevant resources for Wails v2 Linux AppImage packaging. Here's what the search results show:

## Key Resources

A practical Makefile-based approach has been documented for packaging Wails applications as AppImages, including downloading appimagetool and creating the AppImage from an app directory.

## Wails v2 Packaging Support

The Linux-specific Taskfile in Wails includes tasks for building, packaging, and running applications on Linux, with key features including creating AppImage, deb, rpm, and Arch Linux packages, and generating .desktop files for Linux applications.

## AppImage Structure

The AppImage ecosystem provides appimagetool as an essential tool that can be included in a build pipeline to produce an AppImage file from a basic directory structure. The AppImage requires an AppRun file (either the application binary itself or a script that performs setup before executing it) and a .desktop file in the Desktop Entry specification format.

## Additional Context

...
```
```
# Transparent Window Support on Linux in Wails v2

## Issue Summary
A feature request was opened on September 30, 2022 (Issue #1923) by user lyimmi requesting transparent window functionality for Linux, which was already available on Windows and macOS.

## Key Details

**Problem Identified:**
The transparency feature existed in Windows and Mac implementations but was absent from the Linux version.

**Proposed Solution:**
The requester suggested adding transparency support to `internal/frontend/desktop/linux/window.go` and creating a new option in `linux.Options` to enable the feature.

**Status:**
The issue was marked as "CLOSED" and "COMPLETED" after pull request #1926 was merged on October 1, 2022. The merged PR was titled "Feature: Add window transparency for linux."

## Implementation Notes

The original poster mentioned: *"I was able achieve window transparency without any blur/frosting effect. (I think blur could be added in GTK4)"*

...
```
```
I appreciate your request, but I cannot extract the complete `nfpm.yaml` template from the provided content. The webpage shows a GitHub pull request discussion about correcting Linux package dependencies, but the actual full template file contents are not displayed in the page HTML you provided.

What I can tell you from the PR description is that the changes addressed:

**Key corrections made:**
- "The `nfpm.yaml` template pulls in `-dev` versions of the packages the built Wails application actually relies on" which were replaced with runtime libraries instead
- Fixed outdated RPM package naming for webkit2gtk versions across different distributions
- Tested on Ubuntu 22.04 and Rocky Linux 10

The PR references the file `v3/internal/commands/updatable_build_assets/linux/nfpm/nfpm.yaml.tmpl`, but the actual dependency lists aren't visible in this page snapshot.

To access the complete corrected template with all dependency specifications for Debian, RPM, and Arch distributions, you would need to view the actual merged PR files on GitHub or check the Wails repository directly at that template file path.
```
- **WebFetch**: `Fetch `https://blog.jamesreed.dev/packaging-apps-for-linux-with-appimage``
- **WebSearch**: `Search `Wails v2 Wayland support issues linux window manager``
<details>
<summary>WebSearch: `Search `Go os.UserConfigDir os.UserCacheDir Linux XDG path``</summary>

```
Request failed with status code 429
```

</details>
```
Web search results for query: "Go os.UserConfigDir os.UserCacheDir Linux XDG path"

Links: [{"title":"proposal: os: add UserConfigDir · Issue #29960 · golang/go","url":"https://github.com/golang/go/issues/29960"},{"title":"os: `UserConfigDir` and `UserCacheDir` should disallow relative paths in the environment variables · Issue #68470 · golang/go","url":"https://github.com/golang/go/issues/68470"},{"title":"os: os.UserConfigDir should respect XDG_CONFIG_HOME on Darwin · Issue #76320 · golang/go","url":"https://github.com/golang/go/issues/76320"},{"title":"os: check relative paths in UserConfigDir and UserCacheDir by rhysd · Pull Request #68471 · golang/go","url":"https://github.com/golang/go/pull/68471"},{"title":"GitHub - adrg/xdg: Go implementation of the XDG Base Directory Specification and XDG user directories · GitHub","url":"https://github.com/adrg/xdg"},{"title":"appdirs package - github.com/wessie/appdirs - Go Packages","url":"https://pkg.go.dev/github.com/wessie/appdirs"},{"title":"os package - os - Go Packages","url":"https://pkg.go.dev/os"},{"title":"GitHub - christopherobin/go-appdirs: Like the appdirs python package, but for go","url":"https://github.com/christopherobin/go-appdirs"},{"title":"appdirs package - github.com/cavcrosby/appdirs - Go Packages","url":"https://pkg.go.dev/github.com/cavcrosby/appdirs"},{"title":"xdg package - github.com/adrg/xdg - Go Packages","url":"https://pkg.go.dev/github.com/adrg/xdg"}]

I've completed the web search for your query about Go's `os.UserConfigDir` and `os.UserCacheDir` with XDG paths on Linux. Here are the key findings:

## Go's os.UserConfigDir and os.UserCacheDir Functions

On Unix-like systems, `os.UserConfigDir()` returns `$XDG_CONFIG_HOME` or `$HOME/.config`.

On Unix-like systems, `os.UserCacheDir()` tries `$XDG_CACHE_HOME` and falls back to `$HOME/.cache`.

## Path Validation

An important consideration for Linux/XDG compliance: On Unix systems, these functions return `$XDG_CONFIG_HOME` as specified by the XDG Base Directory Specification if non-empty. However, the specification requires all paths in these environment variables to be absolute. If an implementation encounters a relative path, it should consider the path invalid and ignore it.

If the location cannot be determined (for example, `$HOME` is not defined) or the path in `$XDG_CACHE_HOME` or `$XDG_CONFIG_HOME` is relative, the functions will return an error.

## Alternative Libraries

...
```
```
Web search results for query: "Wails v2 Wayland support issues linux window manager"

Links: [{"title":"[v2] [Linux] Feat - Wayland Support · Issue #1420 · wailsapp/wails","url":"https://github.com/wailsapp/wails/issues/1420"},{"title":"Window maximum width and height issues on Linux Wayland · Issue #2431 · wailsapp/wails","url":"https://github.com/wailsapp/wails/issues/2431"},{"title":"Nothing shown at all on Linux in latest 2.0 beta · Issue #1563 · wailsapp/wails","url":"https://github.com/wailsapp/wails/issues/1563"},{"title":"Wayland - ArchWiki","url":"https://wiki.archlinux.org/title/Wayland"},{"title":"Options | Wails","url":"https://wails.io/docs/reference/options/"},{"title":"Think twice about Wayland. It breaks everything! · GitHub","url":"https://gist.github.com/probonopd/9feb7c20257af5dd915e3a9f2d1f2277"},{"title":"Support Multiple Windows (Linux) · Issue #2167 · wailsapp/wails","url":"https://github.com/wailsapp/wails/issues/2167"},{"title":"Changelog | Wails","url":"https://wails.io/changelog/"},{"title":"Separating the Wayland compositor and window manager | Hacker News","url":"https://news.ycombinator.com/item?id=47388137"},{"title":"[v2 Linux] Transparent window on linux · Issue #1923 · wailsapp/wails","url":"https://github.com/wailsapp/wails/issues/1923"}]

I found relevant search results about Wails v2 and Wayland support issues on Linux. Here's what the search revealed:

## Wayland Support Status

Fedora, Ubuntu, and other distros are moving to Wayland and moving away from X11. Currently Wails uses the XWayland compatibility layer.

## Known Issues

Users who were actually running Wayland would not be able to use the app due to GTK errors. This was documented as a feature request rather than a fully supported feature.

It is not possible to resize window beyond a certain width and height for some reason (although MaxWidth and MaxHeight are unset). This is a specific issue reported on Linux Wayland implementations.

Another issue involved users installing Wails from scratch on Arch Linux/GNOME Wayland finding that no window shows up, though the application is running in the taskbar.

## Documented Wayland-Specific Limitations

...
```
<details>
<summary>Glob: `Glob `**/wails.json``</summary>

```
/Users/admin/Documents/projects/contrails/wails.json
```

</details>
<details>
<summary>Grep: `Grep `UserConfigDir|UserCacheDir|UserHomeDir|appData|configDir``</summary>

```
Found 12 files
agent_contrails/1774456978 - I'd like to allow categorizing contrails that are already processed. (create category, add to catego.md
app.go
agent_contrails/1772990949 - Right now we support Mac only (universal, both apple silicon and intel).md
platform_darwin.go
agent/claudecode/hook.go
platform_windows.go
agent/cursor/paths_windows.go
agent/cursor/paths_darwin.go
analytics.go
app_test.go
agent/claudecode/scanner.go
watcher.go
```

</details>
<details>
<summary>WebSearch: `Search `wails-build-action GitHub Actions linux ubuntu 24.04 workflow yaml example``</summary>

```
Web search results for query: "wails-build-action GitHub Actions linux ubuntu 24.04 workflow yaml example"

Links: [{"title":"wails-build-action/action.yml at main · dAppServer/wails-build-action","url":"https://github.com/dAppServer/wails-build-action/blob/main/action.yml"},{"title":"GitHub - dAppServer/wails-build-action: USE: host-uk/build@v4 · GitHub","url":"https://github.com/dAppServer/wails-build-action"},{"title":"Crossplatform build with Github Actions | Wails","url":"https://wails.io/docs/next/guides/crossplatform-build/"},{"title":"Build + Test v2 · Workflow runs · wailsapp/wails","url":"https://github.com/wailsapp/wails/actions/workflows/build-and-test.yml"},{"title":"Wails Build · Actions · GitHub Marketplace · GitHub","url":"https://github.com/marketplace/actions/wails-build"},{"title":"[FEATURE]: Upgrade GitHub Actions workflows from Ubuntu 20.04 to 24.04 · Issue #3095 · youki-dev/youki","url":"https://github.com/youki-dev/youki/issues/3095"},{"title":"Wails Build Action tylertravisty - GitHub Marketplace","url":"https://github.com/marketplace/actions/wails-build-action-tylertravisty"},{"title":"Workflow runs · wailsapp/wails","url":"https://github.com/wailsapp/wails/actions"},{"title":"Wails Action CI/CD - GitHub Marketplace","url":"https://github.com/marketplace/actions/wails-action-ci-cd"},{"title":"Actions · wailsapp/awesome-wails","url":"https://github.com/wailsapp/awesome-wails/actions"}]

Based on the search results, here's what I found about wails-build-action with GitHub Actions for Linux/Ubuntu 24.04:

## Ubuntu 24.04 Support

The wails-build-action includes Linux Discovery that detects the distribution version and for Ubuntu 24.04 installs libgtk-3-0, libwebkit2gtk-4.1-dev, and gcc-aarch64-linux-gnu. Additionally, the actions/options sub-action computes BUILD_OPTIONS and adds -tags webkit2_41 on Ubuntu 24.04 when appropriate.

## Basic Workflow Example

Here's a basic example workflow for building Wails on Linux with Ubuntu:

The workflow uses a matrix strategy with `runs-on: ubuntu-latest`, checks out the code with recursive submodules, and uses `dAppServer/wails-build-action` with inputs for build-name, build-platform (e.g., 'linux/amd64'), package setting, and go-version.

A minimal example for Ubuntu 24.04 would look like:

```yaml
name: Wails build
...
```

</details>
- **Read**: `Read `/Users/admin/Documents/projects/contrails/wails.json``
- **Read**: `Read `/Users/admin/Documents/projects/contrails/platform_darwin.go``
- **Read**: `Read `/Users/admin/Documents/projects/contrails/platform_windows.go``
- **Read**: `Read `/Users/admin/Documents/projects/contrails/app.go``
<details>
<summary>Glob: `Glob `**/agent/cursor/paths_*.go``</summary>

```
/Users/admin/Documents/projects/contrails/agent/cursor/paths_darwin.go
/Users/admin/Documents/projects/contrails/agent/cursor/paths_windows.go
```

</details>
<details>
<summary>Glob: `Glob `**/agent/claudecode/*.go``</summary>

```
/Users/admin/Documents/projects/contrails/agent/claudecode/hook_enforcer_test.go
/Users/admin/Documents/projects/contrails/agent/claudecode/hook_test.go
/Users/admin/Documents/projects/contrails/agent/claudecode/scanner.go
/Users/admin/Documents/projects/contrails/agent/claudecode/hook_enforcer.go
/Users/admin/Documents/projects/contrails/agent/claudecode/types.go
/Users/admin/Documents/projects/contrails/agent/claudecode/driver.go
/Users/admin/Documents/projects/contrails/agent/claudecode/parser_test.go
/Users/admin/Documents/projects/contrails/agent/claudecode/parser.go
/Users/admin/Documents/projects/contrails/agent/claudecode/signal_watcher.go
/Users/admin/Documents/projects/contrails/agent/claudecode/hook_command_darwin.go
/Users/admin/Documents/projects/contrails/agent/claudecode/hook_command_windows.go
/Users/admin/Documents/projects/contrails/agent/claudecode/hook.go
```

</details>
- **Read**: `Read `/Users/admin/Documents/projects/contrails/agent/cursor/paths_darwin.go``
- **Read**: `Read `/Users/admin/Documents/projects/contrails/agent/cursor/paths_windows.go``
- **Read**: `Read `/Users/admin/Documents/projects/contrails/agent/claudecode/scanner.go``
- **Read**: `Read `/Users/admin/Documents/projects/contrails/agent/claudecode/hook_command_darwin.go``
- **Read**: `Read `/Users/admin/Documents/projects/contrails/agent/claudecode/hook_command_windows.go``
- **Read**: `Read `/Users/admin/Documents/projects/contrails/main.go``
<details>
<summary>Glob: `Glob `**/.github/workflows/*.yml``</summary>

```
/Users/admin/Documents/projects/contrails/.github/workflows/release.yml
```

</details>
- **Glob**: `Glob `**/Makefile``
<details>
<summary>Glob: `Glob `**/Taskfile*.yml``</summary>

```
No files found
```

</details>
```
No files found
```
- **Read**: `Read `/Users/admin/Documents/projects/contrails/.github/workflows/release.yml``
- **Read**: `Read `/Users/admin/Documents/projects/contrails/agent/claudecode/hook.go``
<details>
<summary>Glob: `Glob `**/build*.sh``</summary>

```
/Users/admin/Documents/projects/contrails/buildMacOS.sh
/Users/admin/Documents/projects/contrails/buildWindows.sh
```

</details>
<details>
<summary>Glob: `Glob `**/agent/vscode/*.go``</summary>

```
/Users/admin/Documents/projects/contrails/agent/vscode/parser_subagent_test.go
/Users/admin/Documents/projects/contrails/agent/vscode/driver.go
/Users/admin/Documents/projects/contrails/agent/vscode/types.go
/Users/admin/Documents/projects/contrails/agent/vscode/parser_plain_test.go
/Users/admin/Documents/projects/contrails/agent/vscode/parser.go
/Users/admin/Documents/projects/contrails/agent/vscode/parser_test.go
```

</details>
- **Read**: `Read `/Users/admin/Documents/projects/contrails/watcher.go``
- **Read**: `Read `/Users/admin/Documents/projects/contrails/buildMacOS.sh``
- **Read**: `Read `/Users/admin/Documents/projects/contrails/buildWindows.sh``
- **Read**: `Read `/Users/admin/Documents/projects/contrails/analytics.go``
<details>
<summary>Grep: `Grep `configDir|UserConfigDir|os\.UserConfigDir``</summary>

```
/Users/admin/Documents/projects/contrails/analytics.go-260-// --- Device ID management ---
/Users/admin/Documents/projects/contrails/analytics.go-261-
analytics.go:262:func (a *Analytics) configDir() (string, error) {
analytics.go:263:	baseDir, err := os.UserConfigDir()
/Users/admin/Documents/projects/contrails/analytics.go-264-	if err != nil {
/Users/admin/Documents/projects/contrails/analytics.go-265-		return "", err
--
/Users/admin/Documents/projects/contrails/analytics.go-273-
/Users/admin/Documents/projects/contrails/analytics.go-274-func (a *Analytics) loadOrCreateDeviceID() (string, error) {
analytics.go:275:	dir, err := a.configDir()
/Users/admin/Documents/projects/contrails/analytics.go-276-	if err != nil {
/Users/admin/Documents/projects/contrails/analytics.go-277-		return "", err
--
/Users/admin/Documents/projects/contrails/app.go-32-	analytics      *Analytics
/Users/admin/Documents/projects/contrails/app.go-33-	drivers        map[AgentSourceType]agent.AgentDriver // source type → driver
app.go:34:	configDir      string           // override for testing; empty = os.UserConfigDir()
/Users/admin/Documents/projects/contrails/app.go-35-	lastFileHashes map[string]string // filePath → content hash at last processing
app.go-36-	/ Guidelines: Don't fire-and-forget goroutines (go-style-guide.md)
--
app.go-269-/ Errors: Handle Errors Once — propagate instead of silently discarding (go-style-guide.md)
...
```

</details>

</details>


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

