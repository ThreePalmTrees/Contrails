# SOLID Refactoring Plan

This document tracks the architectural refactoring of Contrails through the lens of SOLID principles. The goal is to make adding new agents (Cursor, Antigravity) require zero changes to existing code.

> **North star**: Contrails preserves the agent's complete process — Watch → Parse → Write. Every architectural decision should make this pipeline clearer and more extensible.

---

## Current State (as of session 1771801782)

### The Pipeline

```
Watch (detect changes) → Parse (raw format → ParsedSession) → Write (ParsedSession → .md)
```

Each agent differs in **Watch** and **Parse**. **Write** is always the same. But the code doesn't reflect this cleanly.

### Key Files

| File | Lines | Role |
|------|-------|------|
| `agent/session.go` | 411 | Types + writer + logger + utilities (4 responsibilities) |
| `app.go` | 989 | Composition root + project CRUD + watcher lifecycle + processing + UI + SignalHandler |
| `parser.go` | ~55 | Thin VS Code delegation + re-exports |
| `watcher.go` | 134 | VS Code fsnotify watcher (no abstraction) |
| `agent/claudecode/signal_watcher.go` | 209 | Claude Code signal watcher (has `SignalHandler` interface) |
| `agent/claudecode/parser.go` | 619 | Claude Code JSONL → ParsedSession |
| `agent/vscode/parser.go` | ~200 | VS Code JSON → ParsedSession |
| `types.go` | ~80 | Type aliases + Project/AgentSource/event structs |
| `runtime.go` | 124 | Logger/EventEmitter/DialogOpener interfaces + implementations |

### Currently Supported Agents

- **VS Code Copilot**: fsnotify watcher, JSONL parser
- **Claude Code**: signal hook watcher, JSONL parser

### Planned Agents

- **Cursor**: SQLite-based (`.cursorDirChat/` database)
- **Google Antigravity**: SQLite-based

---

## SOLID Violations

### 1. Single Responsibility Principle (SRP)

#### `agent/session.go` — 4 responsibilities in one file

| Responsibility | What it contains | Changes when... |
|---|---|---|
| **Contract** (abstractions) | `SessionParser` interface, `ParsedSession`, `ParsedMessage`, `MessagePart`, `MessagePartType` constants | The vocabulary of content parts grows |
| **Writer** (pipeline step 3) | `WriteParsedSession`, `writeInterleavedParts`, `writeToolDetailPart` | Markdown rendering changes |
| **Logging abstraction** | `Logger` interface, `LogInfof`/`LogWarningf`/`LogErrorf` | Logging needs change |
| **Utilities** | `Truncate`, `SanitizeFilename`, `FormatTimestamp`, `FormatISO8601Timestamp` | String formatting needs change |

#### `app.go` — 6+ responsibilities in one file

- Project CRUD + persistence
- VS Code watcher lifecycle management
- Claude Code signal watcher lifecycle management
- VS Code file processing (`processFile`, `ProcessFileIfNeeded`, `ProcessModifiedSince`, `HealContrailNames`)
- Claude Code processing (`ProcessClaudeCodeSessions`)
- UI interaction (dialogs, events, progress)
- `SignalHandler` implementation for Claude Code

#### `parser.go` (root) — re-exports non-parsing functions

Re-exports `WriteParsedSession`, `formatTimestamp`, `sanitizeFilename`, `truncate` — none of which are parsing.

---

### 2. Open-Closed Principle (OCP)

**The most significant violation.** `app.go` has 10 occurrences of `hasClaudeCodeSource` across 5 methods:

```go
// startup (line ~90)
if app.signalWatcher != nil && hasClaudeCodeSource(project) { ... }

// FindProject (lines ~147, ~154)
if p.WorkspacePath == workingDirectory && hasClaudeCodeSource(p) { ... }
if hasClaudeCodeSource(p) && strings.HasPrefix(workingDirectory, p.WorkspacePath+"/") { ... }

// AddProject (line ~260)
if hasClaudeCodeSource(project) { /* install hook + register */ }

// UpdateProject (lines ~309, ~311)
if hasClaudeCodeSource(project) && project.Active { /* register */ }
if hasClaudeCodeSource(existing) && !project.Active { /* unregister */ }

// RemoveProject (line ~347)
if hasClaudeCodeSource(project) { /* uninstall hook + unregister */ }
```

**Impact**: Adding Cursor requires touching every one of these methods with `if hasCursorSource(project) { ... }`. The file grows linearly with agent count.

The VS Code watcher has no abstraction at all — `app.go` directly creates and manages a concrete `Watcher` type.

---

### 3. Liskov Substitution Principle (LSP)

```go
type SessionParser interface {
    ParseFile(filePath string) (*ParsedSession, error)
}
```

This works for JSONL files, but Cursor and Antigravity use **SQLite databases**. A SQLite DB contains multiple sessions. `ParseFile(filePath)` could still work if the "file" is the DB path and the parser handles querying internally — but the single-session assumption may need revisiting.

Consider whether a `SessionSource` interface is needed:

```go
type SessionSource interface {
    ListSessions(sourcePath string) ([]string, error)
    ParseSession(sourcePath, sessionID string) (*ParsedSession, error)
}
```

**Decision**: defer until we understand Cursor/Antigravity data formats better.

---

### 4. Interface Segregation Principle (ISP)

**`SignalHandler`** (in `claudecode/signal_watcher.go`) has 4 methods:

```go
type SignalHandler interface {
    FindProject(workingDirectory string) (projectID, outputDir string, found bool)
    WriteContrail(session *agent.ParsedSession, outputDir string) (outputPath string, err error)
    EmitWatcherEvent(projectID, fileName, eventType string)
    EmitFileProcessed(projectID, fileName string)
}
```

`FindProject` internally checks `hasClaudeCodeSource(p)` — leaking agent awareness into a method called *from* the agent package. The handler should match by workspace path only; the signal watcher already knows which projects it tracks.

**`Logger`** is fine as an interface but lives in a file called "session" — misleading.

---

### 5. Dependency Inversion Principle (DIP)

Watching is wired inconsistently:

- **VS Code**: `app.go` directly creates/manages concrete `Watcher` type. No abstraction.
- **Claude Code**: `signal_watcher.go` calls back into `app.go` via `SignalHandler` interface (good DIP). But `app.go` directly imports `claudecode` package to call `InstallHook`, `UninstallHook`, `BrowseProjects`, etc.

The root package (`main`) has `import "contrails/agent/claudecode"` — compile-time knowledge of every agent. Adding Cursor means adding `import "contrails/agent/cursor"` and more conditional branches.

---

## `Truncate` — Remove It

> "We're trying to preserve the agent's process that led to a solution, so that the next fresh chat the agent has memory that it can reference."

Where `Truncate` is called today:

| Caller | What it truncates | Characters |
|---|---|---|
| `claudecode/parser.go` `summarizeToolCall` | Bash commands | 150 |
| `claudecode/parser.go` `summarizeToolCall` | URLs | 150 |
| `claudecode/parser.go` `summarizeToolCall` | Task descriptions | 150 |
| `agent/session.go` `writeInterleavedParts` | Tool arguments | 200 |
| `agent/session.go` `writeToolDetailPart` | Terminal commands | 200 |

Every one destroys information. A bash command cut at 150 chars means the agent reading this contrail later won't know the full command. The whole purpose of Contrails is preservation — truncation actively harms the product.

**`SanitizeFilename`** is legitimate (OS constraints). **`FormatTimestamp`** is legitimate. Both are pure utilities unrelated to agents.

---

## Refactoring Steps

Each step is independently shippable and tests should pass after each one.

### Step 1: Split `session.go` + Remove `Truncate`

**Principle**: SRP
**Effort**: Small | **Risk**: Very low | **Ships alone**: Yes

Move code between files within `agent/` package. Zero import changes. Zero API changes.

```
agent/
├── contrail.go       # ParsedSession, ParsedMessage, MessagePart, MessagePartType constants
│                     # SessionParser interface
├── writer.go         # WriteParsedSession, writeInterleavedParts, writeToolDetailPart
├── logger.go         # Logger interface, LogInfof/LogWarningf/LogErrorf
├── format.go         # FormatTimestamp, FormatISO8601Timestamp, SanitizeFilename
```

Remove `Truncate` function entirely. Update callers to emit full content.

**Checklist**:
- [x] Create `contrail.go` with types and `SessionParser` interface
- [x] Create `writer.go` with writing functions
- [x] Create `logger.go` with `Logger` interface and helpers
- [x] Create `format.go` with `FormatTimestamp`, `FormatISO8601Timestamp`, `SanitizeFilename`
- [x] Delete `session.go`
- [x] Remove `Truncate` from `agent/` package
- [x] Remove `truncate` from root `parser.go`
- [x] Update `claudecode/parser.go` `summarizeToolCall` to emit full content
- [x] Update `writeInterleavedParts` and `writeToolDetailPart` to not truncate
- [x] Run all tests, verify pass

---

### Step 2: Define `AgentDriver` Interface

**Principle**: OCP, DIP
**Effort**: Medium | **Risk**: Medium (interface design is critical) | **Ships alone**: Yes

Each agent package exports a type that knows its own lifecycle:

```go
// agent/driver.go (conceptual — needs design iteration)
type AgentDriver interface {
    // Setup is called when a project is added (install hooks, etc.)
    Setup(workspacePath string) error

    // Teardown is called when a project is removed (uninstall hooks, etc.)
    Teardown(workspacePath string) error

    // StartWatching begins watching for changes for a project.
    StartWatching(ctx context.Context, workspacePath string) error

    // StopWatching stops watching a project.
    StopWatching(workspacePath string) error

    // ProcessAll processes all sessions ("Process All Now" button).
    ProcessAll(transcriptDir, outputDir string, onProgress func(current, total int)) (int, error)

    // Parser returns the SessionParser for this agent type.
    Parser() SessionParser
}
```

Then `app.go` becomes:

```go
for _, source := range project.Sources {
    driver := app.drivers[source.Type]  // map[AgentSourceType]AgentDriver
    driver.Setup(project.WorkspacePath)
    driver.StartWatching(ctx, project.WorkspacePath)
}
```

**Key design question**: VS Code uses directory-based fsnotify, Claude Code uses signal-based hooks. The interface must accommodate both without being too broad (ISP). Resolved: "Activate/Deactivate" instead of "StartWatching/StopWatching." Each driver ignores parameters it doesn't need (VS Code ignores `workspacePath`, Claude Code ignores `watchDir`).

**Checklist**:
- [x] Design `AgentDriver` interface (iterate on method signatures)
- [x] Implement for Claude Code (`claudecode.Driver`)
- [x] Implement for VS Code (`vscode.Driver`)
- [x] Create driver registry in `app.go` (`map[AgentSourceType]AgentDriver`)
- [x] Replace `hasClaudeCodeSource` branches with driver dispatch
- [x] Remove `hasClaudeCodeSource` function (replaced with generic `hasSource`)
- [x] Run all tests, verify pass

---

### Step 3: Reconsider `SessionParser` for SQLite

**Principle**: LSP
**Effort**: Design now, implement when needed | **Risk**: Low | **Ships alone**: Yes (design only)

Options:

1. **Keep `ParseFile` as-is** — SQLite parsers treat the DB path as the "file." Parser opens DB, queries conversations, returns `ParsedSession`. Caller doesn't care about format.
2. **Add `SessionSource` interface** — for agents with multi-session sources (SQLite DBs):
   ```go
   type SessionSource interface {
       ListSessions(sourcePath string) ([]string, error)
       ParseSession(sourcePath, sessionID string) (*ParsedSession, error)
   }
   ```
3. **Hybrid** — `SessionParser` for single-file agents, `SessionSource` for multi-session. The driver decides which to use internally.

**Decision**: defer concrete implementation until we see Cursor/Antigravity data formats. For now, note the concern and keep the interface stable.

---

### Step 4: Move Processing to Drivers

**Principle**: SRP, OCP
**Effort**: Medium | **Risk**: Low (after Step 2) | **Ships alone**: Yes

Move agent-specific processing logic out of `app.go`:

- `processFile`, `ProcessFileIfNeeded`, `ProcessModifiedSince`, `HealContrailNames` → `vscode.Driver`
- `ProcessClaudeCodeSessions` → `claudecode.Driver.ProcessAll`

After this, `app.go` knows nothing about how agents process files — it just calls `driver.ProcessAll(...)`.

**Checklist**:
- [x] Move VS Code processing methods into `vscode.Driver`
  - `ProcessAll` (batch parse + write) with `HealContrailNames` embedded
  - `extractSessionIDFromHeader` helper
  - Incremental methods (`ProcessFileIfNeeded`, `ProcessModifiedSince`) remain on App for now (App-level state deps)
- [x] Move Claude Code processing into `claudecode.Driver`
  - `ProcessAll` (list session files → parse → write)
- [x] Update `app.go` to call `driver.ProcessAll(...)` generically
  - `ProcessChatSessions` and `ProcessClaudeCodeSessions` are now thin wrappers
  - Added `ProcessAllSessions(projectID)` unified method
  - Added `makeProcessCallbacks(projectID)` callback builder
  - Added `ProcessCallbacks` struct to `agent/driver.go`
- [x] Startup healing delegates to `vscode.Driver.HealContrailNames`
- [x] Remove agent-specific imports from `app.go` where possible (addressed in Step 5 — composition root imports are legitimate)
- [x] Run all tests, verify pass

---

### Step 5: Clean Up Root Delegation

**Principle**: SRP
**Effort**: Small | **Risk**: Low | **Ships alone**: Yes

After Steps 2-4, the root `parser.go` becomes mostly unnecessary. The drivers encapsulate their own parsers. Remove re-exports.

`app.go` shrinks to:
- Project CRUD + persistence
- Driver registry + dispatch
- UI interaction (Wails bindings)

**Checklist**:
- [x] Remove unnecessary re-exports from `parser.go`
  - All 7 wrapper functions replaced with direct calls to `agent.*` or `vscode.*`
  - Dead wrappers (`formatTimestamp`, `sanitizeFilename`) had zero callers
- [x] `parser.go` deleted entirely
  - `_vscodeParser` instance moved to `app.go`
  - `parser_test.go` updated to call `vscode.Parser` and `agent.WriteParsedSession` directly
  - `app_test.go` updated: `extractLastMessageDate` → `vscode.ExtractLastMessageDate`
- [x] `app.go` still imports `claudecode` and `vscode` — these are legitimate composition-root wiring (driver creation, signal watcher, browse). No further reduction possible without a plugin registry.
- [x] `watcher.go` now imports `contrails/agent/vscode` for `IsChatSessionFile`
- [x] README.md architecture section updated to reflect new file layout
- [x] Run all tests, verify pass

---

## Sequencing

| Step | Principle | Effort | Risk | Depends on |
|------|-----------|--------|------|------------|
| 1. Split session.go + kill Truncate | SRP | Small | Very low | None |
| 2. AgentDriver interface | OCP, DIP | Medium | Medium | None (but Step 1 makes it cleaner) |
| 3. SessionParser for SQLite | LSP | Low (design) | Low | None |
| 4. Move processing to drivers | SRP, OCP | Medium | Low | Step 2 |
| 5. Clean up root delegation | SRP | Small | Low | Steps 2 + 4 |

**Recommended order**: 1 → 2 → 4 → 5 → 3 (when Cursor is ready)

---

## Notes

### On `FindProject` in `SignalHandler`

The current implementation in `app.go` checks `hasClaudeCodeSource(p)` inside `FindProject`. This should not be necessary — the signal watcher only receives signals for projects it registered. The check is defensive but leaks agent awareness into a generic-seeming method. After Step 2, this goes away because the driver handles its own project matching.

### On the VS Code Watcher

`watcher.go` (root package) is a concrete type with no interface. It's directly managed by `app.go`. After Step 2, this moves into `agent/vscode/` and implements `AgentDriver`. The root `watcher.go` file can be deleted.

### On `parser.go` as a Delegation Layer

The root `parser.go` currently exists so that `app.go` can call `ParseChatSessionFile` without knowing about `vscode.Parser`. After Step 2, each driver has its own parser — the delegation layer is unnecessary.

### On Testing

Each step preserves the existing test suite:
- Step 1 is purely moving code between files in the same package. No behavior changes (except removing `Truncate`).
- Steps 2-5 change wiring but preserve the same Parse/Write behavior through the new interfaces.

Run `go test ./...` after each step to verify.
