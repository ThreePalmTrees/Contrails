# OpenCode Support Plan

**Original spike date:** 2026-05-12
**Revised:** 2026-09-24 — verified against the OpenCode source repo (`/Users/admin/Documents/projects/opencode`, HEAD `0f549842ee`) and a live OpenCode database (`~/.local/share/opencode/opencode.db`). The original spike's storage claims were partly stale; this revision replaces every storage-shape claim with what the source and the live DB show.
**Source repo:** https://github.com/anomalyco/opencode
**Status:** Research verified, implementation not started

**Version-skew warning:** the installed OpenCode build is *ahead* of the repo HEAD — the live DB has an `idle` message type and two migrations that HEAD's `migration.gen.ts` does not list. Build the parser against the live DB's schema (export a fixture from it), not only against HEAD's source.

---

## 1. What Is OpenCode

OpenCode is an open-source, terminal-based AI coding assistant (similar to Claude Code). It supports multiple LLM providers (Anthropic, OpenAI, Google, etc.) and tracks full conversation history including tool calls, token costs, and file operations.

---

## 2. How OpenCode Stores Conversations

### Storage format: SQLite (WAL), one global database

OpenCode uses a single **SQLite database in WAL mode** — not per-session files.

| Item | Detail |
|------|--------|
| Default path | `~/.local/share/opencode/opencode.db` (XDG data dir; **not** `~/.opencode/` — that directory holds only `bin/`) |
| Data dir resolution | `$XDG_DATA_HOME` if set, else `~/.local/share` (all platforms — see `packages/core/src/global.ts:10-11`) |
| Default filename | `opencode.db` for channels `latest/dev/beta/next/prod` (or `OPENCODE_DISABLE_CHANNEL_DB` set), else `opencode-<channel>.db` (`packages/core/src/database/database.ts:43-55`) |
| Override | `OPENCODE_DB` — a **relative** value is joined to the data dir, absolute paths and `:memory:` are used as-is (`database.ts:44-46`, `flag.ts:47`). `dbPath()` must replicate this, not use the value as-is. |
| Sidecars | `opencode.db-wal` and `opencode.db-shm` — the WAL holds recent writes, so the watcher must cover all three names |
| Per-session files? | No — all sessions in one DB |

This is the same SQLite approach as Cursor's `state.vscdb`; the existing `agent/cursor/parser.go` is the closest reference for implementation.

### Database schema (live, installed build)

The DB holds ~20 tables. The ones that matter to contrails:

#### `session_v2` table — one row per conversation

```
id              TEXT  PRIMARY KEY   -- "ses_…" prefixed
project_id      TEXT  NOT NULL      -- FK → project(id)
parent_id       TEXT                -- parent session ID for subagent sessions
fork_session_id TEXT                -- forked-from link
directory       TEXT  NOT NULL      -- working directory when session started
title           TEXT
version         TEXT                -- opencode version
agent           TEXT                -- agent used ("build", "plan", custom, …)
model           TEXT (JSON)         -- { id, providerID, variant }
cost            REAL
tokens_input / tokens_output / tokens_reasoning / tokens_cache_read / tokens_cache_write
time_created    INTEGER             -- Unix ms
time_updated    INTEGER             -- Unix ms — grouping + dedup key
time_archived   INTEGER             -- Unix ms (null if active)
```

Indexes: `session_v2_project_idx`, `session_v2_parent_idx`.

#### `session_message` table — one row per message, parts embedded in the JSON

There is **no `part` table** in the current data model. Content parts are embedded in the message row's `data` column.

```
id              TEXT  PRIMARY KEY   -- UUID
session_id      TEXT  REFERENCES session_v2(id)
type            TEXT  NOT NULL      -- "user" | "assistant" | others (see below)
seq             INTEGER NOT NULL    -- ORDERING KEY — unique per (session_id, seq)
time_created    INTEGER             -- Unix ms
time_updated    INTEGER             -- Unix ms
data            TEXT (JSON)         -- message payload, parts embedded in data.content
```

Unique index `session_message_session_seq_idx (session_id, seq)`; the repo loads history ordered by `asc(seq)` (`packages/core/src/session/history.ts`). **Order by `seq`, not `time_created`** — equal timestamps make `time_created` ambiguous.

#### Message `type` values — filter to user/assistant

```
assistant, user, idle, synthetic, system, shell, compaction, model-switched, agent-switched
```

(`idle` exists in the live build but not HEAD's `packages/schema/src/session-message.ts:200-213` — treat unknown types as skip.) Only `user` and `assistant` rows carry comparable content; everything else is skipped. The role is the row's `type` column — `data.role` is **not** present in v2 message data (verified: 0 of ~4,000 rows).

#### Companion tables (not needed for v1, but adjacent)

`project` (per git-worktree project), `workspace`, `session_pending` (in-flight rows), `permission`, `event`/`event_sequence`. Sessions also carry `fork_session_id`; history before the latest `compaction` row is not current (`history.ts` `latestCompaction`).

---

## 3. Key Data Structures

### User message (`type = "user"`)

```json
{
  "time":   { "created": 1790274711694 },
  "text":   "…",
  "files":  [],
  "agents": [],
  "skills": [ { "id": "haring", "name": "haring", "text": "…" } ]
}
```

### Assistant message (`type = "assistant"`)

```json
{
  "time":     { "created": 1790274736318, "streamed": 1790274738016 },
  "agent":    "build",
  "model":    { "id": "glm-5.3-flash", "providerID": "opencode-go", "variant": "high" },
  "content":  [ /* parts — see below */ ],
  "finish":   "tool-calls" | "stop" | …,
  "cost":     0.001903,
  "tokens":   { "input": 2415, "output": 233, "reasoning": 0, "cache": { "read": 47488, "write": 0 } },
  "snapshot": { "start": "…" }
}
```

`model` is a nested object `{id, providerID, variant}` — not flat `providerID`/`modelID` fields.

### Part types

In the live v2 data model, assistant messages carry `content` arrays with parts of type `text`, `reasoning`, `tool` (verified across ~6,200 parts in one live DB: tool 3,511 / reasoning 1,623 / text 1,111). The full type system also defines the following (all real, all worth at least a skip-with-fallback):

| Type | Maps to contrails | Notes |
|------|-------------------|-------|
| `text` | `PartText` | `data.text` field |
| `reasoning` | `PartThinking` | `data.text` |
| `tool` | `PartToolCall` / `PartToolResult` | `state` is an **object** discriminated by `state.status` |
| `file` | `PartFileEdit` / `PartReference` | `source` variant of `file` / `symbol` / `resource` |
| `step-start` | skip / boundary marker | snapshot of project state |
| `step-finish` | skip / boundary marker | per-step cost and tokens |
| `patch` | `PartFileEdit` | `hash`, `files` list |
| `snapshot` | skip | project state snapshot |
| `agent` | skip | agent reference |
| `subtask` | `SubParts` (v1: via `parent_id`, see §3b) | prompt/description/agent — no child session id field |
| `retry` | skip or note | retry attempt info with error |
| `compaction` | skip | context window compaction marker |

### Tool part detail (current shape)

```json
{
  "type": "tool",
  "id": "4173e20a-…",
  "tool": "shell",
  "state": {
    "status": "completed",           // pending | running | completed | error
    "input":  { "command": "…" },
    "output": "…",                    // string, ToolStateCompleted only
    "title":  "…",
    "metadata": { },
    "time":   { "start": 1790274738012, "end": 1790274740459, "compacted": null }
  }
}
```

Key differences from the original spike: `state` is an object (not a flat status string), `input`/`output` live **inside** `state`, `output` is a string (`state.output`), and completed-tool metadata can carry nested `content` arrays.

### §3b Subagents — resolved, unlike the original spike's guess

Subagent sessions are their own `session_v2` rows with `parent_id` pointing at the parent session (55 of 113 live sessions are children). The child session id also appears in the subagent tool's output text (`<subagent sessionID="ses_…" state="completed">`) and in a synthetic message's `metadata.childID`.

**v1 includes nesting:** after parsing a session, one extra query lists `session_v2` rows with `parent_id = <session id>` and parses each child as `SubParts` (same shape as Claude Code's `Agent` tool).

### Known OpenCode tool names → rich `ToolDetail`

Builtins: `packages/core/src/tool/builtins.ts:31-47`. Live data shows: `shell`, `read`, `write`, `edit`, `grep`, `glob`, `subagent`, `question`, `skill`, `webfetch`, `websearch`, `execute`, `todowrite`, `apply_patch`.

| OpenCode tool | `ToolDetail.Kind` | Input fields |
|---------------|-------------------|--------------|
| `bash` / `shell` / `execute` | `"terminal"` | `input.command` → `Command` |
| `read` | — | `input.path` → `FilePath` + `PartReference` |
| `write` / `edit` / `apply_patch` | — | `input.path` → `FilePath` + `PartFileEdit` |
| `grep` / `glob` | — | `input.pattern` → `ResultFiles` |
| `todowrite` | `"todoList"` | `input.todos` → `Todos` |
| anything else | — | passthrough: show the tool name |

**The original spike's tool names (`bash`/`exec`, `read_file`, `write_file`, `edit_file`, `search_files`, `todo_write`) do not exist in current OpenCode**, and the input field is `path`, not `filePath`.

---

## 4. Implementation Plan

### Package structure

```
agent/opencode/
├── parser.go          # SessionParser implementation (testable core: parseSession(db, id))
├── types.go           # Internal structs mirroring DB schema + message data JSON
├── paths_darwin.go    # XDG data dir + DB filename resolution
├── paths_linux.go     # Same resolution (shared logic)
├── paths_windows.go   # Windows equivalent — UNVERIFIED, see §6
├── scanner.go         # BrowseProjects + ListSessions — enumerate from DB
├── driver.go          # AgentDriver: fsnotify watcher + ProcessAll
└── parser_test.go     # Tests using in-memory SQLite, fixture exported from the live DB
```

### Step 1 — `paths_*.go`

```go
func dataDir() (string, error) {
    if v := os.Getenv("XDG_DATA_HOME"); v != "" {
        return filepath.Join(v, "opencode"), nil
    }
    home, err := os.UserHomeDir()
    if err != nil {
        return "", err
    }
    return filepath.Join(home, ".local", "share", "opencode"), nil
}

func dbPath() (string, error) {
    if v := os.Getenv("OPENCODE_DB"); v != "" {
        if filepath.IsAbs(v) || v == ":memory:" {
            return v, nil
        }
        // Relative values are joined to the data dir (matches database.ts:44-46).
        d, err := dataDir()
        if err != nil {
            return "", err
        }
        return filepath.Join(d, v), nil
    }
    d, err := dataDir()
    if err != nil {
        return "", err
    }
    // Channel-suffixed name: opencode-<channel>.db for non-stable channels.
    // Probe: opencode.db first, then opencode-*.db, and take the newest.
    // (Candidate list resolved by glob, newest by mod time.)
    return filepath.Join(d, "opencode.db"), nil
}
```

macOS and Linux share the same resolution. Windows is unverified — do not ship `paths_windows.go` until checked against a real Windows OpenCode install (see §6).

### Step 2 — `types.go`

Structs mirror the **v2** schema (fixture-exported, not inferred from HEAD):

```go
type sessionRow struct {
    ID             string
    ProjectID      string
    ParentID       *string
    Directory      string
    Title          *string
    Agent          *string
    Model          json.RawMessage
    Cost           float64
    TokensInput    int64
    TokensOutput   int64
    TimeCreated    int64
    TimeUpdated    int64
    TimeArchived   *int64
}

type messageRow struct {
    ID          string
    SessionID   string
    Type        string   // "user" | "assistant" | skip-types
    Seq         int64    // ordering key
    Data        json.RawMessage
    TimeCreated int64
    TimeUpdated int64
}

// Assistant data (v2): role lives in the row's type column, not in data.
type assistantData struct {
    Time     struct {
        Created  int64 `json:"created"`
        Streamed int64 `json:"streamed"`
    } `json:"time"`
    Agent    string        `json:"agent"`
    Model    struct {
        ID         string `json:"id"`
        ProviderID string `json:"providerID"`
        Variant    string `json:"variant"`
    } `json:"model"`
    Content  []partData    `json:"content"`
    Finish   string        `json:"finish"`
    Cost     float64       `json:"cost"`
    Tokens   tokenUsage    `json:"tokens"`
}

type userData struct {
    Time  struct {
        Created int64 `json:"created"`
    } `json:"time"`
    Text  string         `json:"text"`
    Files json.RawMessage `json:"files"`
}

// partData is a discriminated union on Type.
type partData struct {
    Type  string           `json:"type"`
    ID    string           `json:"id"`
    Tool  string           `json:"tool"`
    State *toolState       `json:"state"`
    Text  string           `json:"text"`
}

type toolState struct {
    Status   string          `json:"status"` // pending | running | completed | error
    Input    json.RawMessage `json:"input"`
    Output   string          `json:"output"`
    Title    string          `json:"title"`
    Metadata json.RawMessage `json:"metadata"`
    Error    json.RawMessage `json:"error"`
    Time     struct {
        Start int64 `json:"start"`
        End   int64 `json:"end"`
    } `json:"time"`
}
```

### Step 2b — `parser.go` core logic

The `ParseFile` argument is the **session ID** (same pattern as Cursor's composer UUID), since there's no per-session file.

```go
func (p *Parser) ParseFile(sessionID string) (*agent.ParsedSession, error) {
    db, err := openDB()   // read-only, busy timeout 5s; WAL — never checkpoint
    if err != nil {
        return nil, err
    }
    defer db.Close()
    return parseSession(db, sessionID)
}

// parseSession is the testable core — accepts an already-open *sql.DB
// so tests can supply an in-memory database (same pattern as Cursor).
func parseSession(db *sql.DB, sessionID string) (*agent.ParsedSession, error) {
    // 1. Fetch session row from session_v2
    // 2. Fetch message rows WHERE session_id = ? ORDER BY seq ASC
    // 3. Skip rows whose type is not user/assistant
    // 4. Walk data.content: text → PartText, reasoning → PartThinking,
    //    tool (status completed/error) → PartToolCall + PartToolResult,
    //    write/edit/apply_patch input → PartFileEdit + FilesEdited
    // 5. Optional (v1): children via SELECT id FROM session_v2 WHERE parent_id = ?
    //    → parse each recursively into SubParts (depth-limited to 1)
    // 6. Populate ParsedSession metadata (title from session_v2.title,
    //    model from session_v2.model, directory, times, cost, tokens)
}
```

**Tool call summarisation** (equivalent to Claude Code's `summarizeToolCall`):

```go
func summarizeTool(tool string, input json.RawMessage) string {
    switch tool {
    case "bash", "shell", "execute":
        var v struct{ Command string `json:"command"` }
        json.Unmarshal(input, &v)
        return v.Command
    case "read", "write", "edit", "apply_patch":
        var v struct{ Path string `json:"path"` }
        json.Unmarshal(input, &v)
        return v.Path
    case "grep", "glob":
        var v struct{ Pattern string `json:"pattern"` }
        json.Unmarshal(input, &v)
        return v.Pattern
    default:
        return tool  // passthrough: show tool name for unknown tools
    }
}
```

### Step 3 — `scanner.go`

OpenCode sessions carry a `directory` field (working directory when the session was started). This is the natural grouping key — analogous to Cursor's `workspacePath`.

```go
// ScannedProject represents one working directory that has OpenCode sessions.
type ScannedProject struct {
    Directory      string  // absolute path (the session's working directory)
    DisplayName    string  // filepath.Base(Directory)
    SessionCount   int
    LastActivityAt int64   // Unix ms — most recent time_updated across sessions
}

// BrowseProjects returns one entry per distinct directory, sorted newest-first.
// Mirrors cursor.BrowseProjects().
func BrowseProjects() ([]ScannedProject, error) {
    db, err := openDB()
    // SELECT directory, COUNT(*), MAX(time_updated) FROM session_v2
    // WHERE time_archived IS NULL
    // GROUP BY directory
    // ORDER BY MAX(time_updated) DESC
    // Normalise symlinks (filepath.EvalSymlinks) so /tmp and /private/tmp
    // (macOS) group as one project.
}

// ListSessions returns all sessions for a given directory, newest-first.
// Mirrors cursor.ListComposers().
func ListSessions(directory string) ([]SessionInfo, error) {
    db, err := openDB()
    // SELECT id, title, time_created, time_updated FROM session_v2
    // WHERE directory = ? AND time_archived IS NULL
    // ORDER BY time_updated DESC
    // Match against the normalised directory too.
}
```

Caveat: legacy rows can carry an empty `directory` (`packages/core/src/database/path.ts` `directoryColumn`) — skip or bucket them as "unknown project".

### Step 4 — `driver.go`

OpenCode has no hook system, so `Setup`/`Teardown` are no-ops (same as Cursor). Real-time change detection uses **fsnotify on the data directory** with a 2-second debounce — the Cursor pattern. One watcher serves **all** registered projects (the DB is global, unlike Cursor's per-workspace databases); Activate/Deactivate toggle a directory filter.

```go
// ChangeHandler is notified when the OpenCode database changes.
type ChangeHandler interface {
    OnOpenCodeDatabaseChanged()
}

// Driver implements agent.AgentDriver for OpenCode.
type Driver struct {
    mu             sync.Mutex
    registered     map[string]struct{}  // directory (normalised) → registered
    fsWatcher      *fsnotify.Watcher
    stopCh         chan struct{}
    doneCh         chan struct{}
    lastTimestamps map[string]int64     // sessionID → last processed time_updated
    changeHandler  ChangeHandler
    logger         agent.Logger
}

func (d *Driver) Setup(_ string) error    { return nil }
func (d *Driver) Teardown(_ string) error { return nil }

// Activate registers a directory and starts the DB watcher on first
// registration. Mirrors cursor.Driver.Activate().
func (d *Driver) Activate(_, workspacePath, _ string) error { … }

// Deactivate removes the directory. Stops the watcher when none remain.
func (d *Driver) Deactivate(workspacePath, _ string) { … }

// ProcessAll enumerates sessions for the directory (sourceDir), skips
// sessions whose time_updated hasn't changed, parses and writes the rest.
// Mirrors cursor.Driver.ProcessAll() including lastTimestamps deduplication.
func (d *Driver) ProcessAll(sourceDir, outputDir string, callbacks agent.ProcessCallbacks) (int, error) { … }
```

**Watcher file targets** — watch the data directory (`~/.local/share/opencode/`), and match all three DB names, plus channel-suffixed variants:

```go
func isOpenCodeDBEvent(event fsnotify.Event) bool {
    if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
        return false
    }
    name := filepath.Base(event.Name)
    return name == "opencode.db" ||
        name == "opencode.db-wal" ||
        name == "opencode.db-shm" ||
        strings.HasPrefix(name, "opencode-") && strings.HasSuffix(name, ".db")
}
```

**`lastTimestamps` deduplication** — on each `ProcessAll` call, sessions whose `time_updated` hasn't advanced since last run are skipped, avoiding re-parsing on every DB write. Same strategy as Cursor.

### Step 5 — `parser_test.go`

Use **in-memory SQLite** with the real v2 schema, and a fixture **exported from the live DB** (a handful of representative sessions incl. tool parts, reasoning, nested child sessions, an `idle` row) — not fixtures guessed from HEAD's types:

```go
func setupTestDB(t *testing.T) *sql.DB {
    db, _ := sql.Open("sqlite", ":memory:")
    // CREATE TABLE session_v2 … session_message … (v2 schema)
    // INSERT exported fixture rows
    return db
}

func TestParseSession_Basic(t *testing.T) { … }
func TestParseSession_ToolCall(t *testing.T) { … }
func TestParseSession_Reasoning(t *testing.T) { … }
func TestParseSession_FileEdit(t *testing.T) { … }
func TestParseSession_SkipsNonUserAssistantTypes(t *testing.T) { … }
func TestParseSession_NestedSubagent(t *testing.T) { … }
func TestParser_ImplementsSessionParser(t *testing.T) {
    var _ agent.SessionParser = (*Parser)(nil)
}
```

### Step 6 — `app.go` integration

Four touch points, all mirroring the Cursor wiring:

1. **Source type** — add `AgentSourceOpenCode AgentSourceType = "opencode"` to `types.go` (alongside `vscode`/`claudecode`/`cursor`, lines 31-33).
2. **Driver registry** — register an `opencode.Driver` alongside the others at `app.go:88`.
3. **Source-type dispatch** — add `"opencode"` to the parser switches: `processFileWithSource` (`app.go:1051`), `processAllForSource` (`app.go:1402`), and `PreviewChatFile` (`app.go:1643`).
4. **Change handler + scanner** — add `BrowseOpenCodeProjects()` next to `BrowseCursorProjects` (`app.go:1800`) and an `OnOpenCodeDatabaseChanged` mirroring `OnCursorDatabaseChanged` (`app.go:1810`), emitting `"opencode:changed"` to the frontend.

### Step 7 — Frontend (`AddProjectDialog.tsx`)

The dialog shows auto-detected workspaces grouped by agent type; OpenCode becomes the fourth group. Backend: `BrowseOpenCodeProjects` Wails binding (auto-generated — `app` is already bound whole, `main.go:85-87`). Frontend changes:

- `AgentTab` union gains `"opencode"` (line 25); `editTargetTab` likewise (line 32).
- Scan state + loader for `opencodeProjects` (mirroring `cursorProjects`, lines 55-58, 153).
- Source-toggle section mirroring the Cursor one (lines ~263, ~335, ~395).
- Agent logo asset: `frontend/src/assets/images/opencode.png` (not yet present — source it, see §7).

### Step 8 — Deleted/archived session handling

Sessions aren't deleted from the filesystem — they're archived (`time_archived IS NOT NULL`; 0 archived in the live DB today, the field works). `ProcessAll` should treat newly-archived sessions the same way contrails treats a deleted Claude Code `.jsonl`: flag the contrail with a deletion banner rather than silently ignoring it. Also handle `fork_session_id`-derived sessions (a forked session's parent contrail stays intact).

### Step 9 — Telemetry

`analytics.go` tracks which agents are used (`TrackAgentSourceAdded`, `TrackProcessAll`, `TrackContrailCreated`). Where `"cursor"`, `"claudecode"`, or `"vscode"` are passed today, `"opencode"` passes for the new source — no new event types.

---

## 5. What each existing agent supports — and OpenCode's equivalent

| Feature | Claude Code | Cursor | VS Code | OpenCode (revised) |
|---------|-------------|--------|---------|--------------------|
| **Storage format** | `.jsonl` files | SQLite (`state.vscdb`) | `.jsonl` files | SQLite WAL (`~/.local/share/opencode/opencode.db` + `-wal`/`-shm`) |
| **Session discovery** | `BrowseProjects()` — scans `~/.claude/projects/` | `BrowseProjects()` — scans `workspaceStorage` | No scanner | `BrowseProjects()` — queries `session_v2`, groups by `directory` |
| **List sessions in a project** | `ListSessionFiles(transcriptDir)` | `ListComposers(workspacePath)` | n/a | `ListSessions(directory)` |
| **Real-time change detection** | fsnotify on transcript dir + signal-file hook | fsnotify on `state.vscdb` dir, debounced 2s | n/a | fsnotify on the XDG data dir, debounced 2s, matching `opencode.db`, `-wal`, `-shm` |
| **Hook installation** | Yes — Stop hook into `.claude/settings.local.json` | No-op | No-op | No-op (OpenCode has no hook system) |
| **Dedup / skip unchanged** | By file mtime | `lastTimestamps` (time_updated per composer) | n/a | `lastTimestamps` (time_updated per session) |
| **Message ordering** | File append order | Composer bubble order | n/a | `session_message.seq` ASC (not `time_created`) |
| **Thinking / reasoning** | `PartThinking` from `thinking` blocks | `PartThinking` from bubble type | `PartThinking` from toolCallRounds | `PartThinking` from `reasoning` content parts |
| **Tool calls** | `PartToolCall` from `tool_use` blocks | `PartToolCall` from bubble content | `PartToolCall` from toolCallRounds | `PartToolCall` from `tool` content parts (`state.status` completed/error) |
| **File edits** | `PartFileEdit` via Write/Edit tool names | `PartFileEdit` via edit bubble metadata | `PartFileEdit` via result file list | `PartFileEdit` via `write`/`edit`/`apply_patch` tool inputs + `patch` parts |
| **Subagent nesting** | `SubParts` via `Agent` tool calls | n/a | `SubParts` via `runSubagent` tool | `SubParts` via `session_v2.parent_id` child query (depth 1) — in v1 |
| **Tool names** | `Write`, `Edit`, `Bash`… | edit bubble metadata | — | `shell`/`read`/`write`/`edit`/`grep`/`glob`/`todowrite`/`apply_patch`/…, input field `path` |
| **Linux support** | `hook_command_linux.go` | `paths_linux.go` | n/a | `paths_linux.go` (same XDG resolution as macOS) |
| **Windows support** | `hook_command_windows.go` | `paths_windows.go` | n/a | `paths_windows.go` needed — path unverified, see §6 |
| **`AgentDriver` interface** | Full | Full | Full | Full |
| **Deleted session handling** | Deletion banner when `.jsonl` removed | Deletion banner when composer disappears | Deletion banner when `.jsonl` removed | Treat `time_archived IS NOT NULL` as deletion — add banner |
| **Telemetry** | `"claudecode"` tag | `"cursor"` tag | `"vscode"` tag | `"opencode"` tag |
| **Frontend auto-detection** | AddProjectDialog | AddProjectDialog | AddProjectDialog | Fourth group in AddProjectDialog (Step 7) |

---

## 6. Open Questions / Risks

| Question | Current best answer | Action needed |
|----------|--------------------|----|
| Is the DB path fixed? | No — filename varies by release channel (`opencode.db` vs `opencode-<channel>.db`), `OPENCODE_DB` override is joined to the data dir when relative. | `dbPath()` globs candidates, picks the newest; honour `OPENCODE_DB` with relative-join semantics. |
| Installed build vs repo HEAD skew | Live DB has `idle` message type + 2 migrations absent from HEAD (`0f549842ee`). | Export parser fixtures from the live DB; treat HEAD's schema as documentation, not truth. Re-check after OpenCode releases. |
| Windows path for `opencode.db` | Unverified. `global.ts` suggests XDG-only (`~/.local/share`), but a Windows-specific override may exist elsewhere in the binary. | Verify on a real Windows install before writing `paths_windows.go`. |
| Legacy rows with empty `directory` | Comment exists in `packages/core/src/database/path.ts`. | Skip empty-directory rows in the scanner, or bucket as "unknown". |
| Symlinked directories (`/tmp` vs `/private/tmp`) | Live DB contains both spellings as distinct projects. | Normalise with `filepath.EvalSymlinks` before grouping and before `ListSessions` matching. |
| Compaction / `session_context_epoch` | History before the latest compaction row is not current (`history.ts` `latestCompaction`). | Parse from the latest compaction boundary forward; verify against real compacted sessions before shipping. |
| In-flight writes (`session_pending`) | Pending rows exist for messages not yet committed. | Read only `session_message`; pending rows surface on the next WAL event anyway. |
| Do tool names stay stable? | New tools appear (`execute`, `skill`, MCP-provided names). | Passthrough fallback in `summarizeTool` — unknown tools show the tool name. |
| OpenCode icon asset | Not yet in `website/icons/` or `frontend/src/assets/images/` | Source the official OpenCode logo before the frontend/website update. |

---

## 7. Closest Existing Reference

The **Cursor driver** (`agent/cursor/driver.go`) and **Cursor scanner** (`agent/cursor/scanner.go`) remain the closest references — OpenCode uses the same SQLite approach, same `ParseFile(id)` contract, same fsnotify + debounce pattern, and same `lastTimestamps` deduplication strategy.

Key differences from Cursor:
- OpenCode has a **global** DB (one watcher, directory-filtered per project) vs Cursor's per-workspace DBs
- OpenCode orders messages by `seq`; message rows are typed and most non-`user`/`assistant` types are noise
- OpenCode stores **cost and token counts per message and per session** — no need to aggregate
- OpenCode has **native reasoning content** → maps directly to `PartThinking`
- OpenCode **groups sessions by the `directory` column**; Cursor resolves workspace paths from `workspace.json` files
- OpenCode **nests subagent sessions via `parent_id`** — resolvable in v1, no extra tool parsing needed

---

## 8. Website (`website/index.html`) and README

The website lists supported agents in several places — all need updating when OpenCode ships (anchors verified 2026-09-24).

**An OpenCode icon is needed first.** Add it to `website/icons/opencode.png` (source from the OpenCode repo or official site). Without the icon, the hero badge and agent card will be incomplete.

### 8a. Meta description (line 7)

```html
<!-- before -->
<meta name="description" content="Contrails watches your coding agent chat sessions (GitHub Copilot, Claude Code & Cursor) …" />

<!-- after -->
<meta name="description" content="Contrails watches your coding agent chat sessions (GitHub Copilot, Claude Code, Cursor & OpenCode) …" />
```

### 8b. Hero badges (lines 40-53)

Add a fourth badge after Cursor (line 49-52):

```html
<div class="hero-badge">
  <img src="icons/opencode.png" alt="OpenCode" class="hero-badge-icon" />
  OpenCode
</div>
```

### 8c. "Agent support" section subtitle (line 320)

```html
<!-- before -->
<p class="section-subtitle">Works with GitHub Copilot, Claude Code, and Cursor.</p>

<!-- after -->
<p class="section-subtitle">Works with GitHub Copilot, Claude Code, Cursor, and OpenCode.</p>
```

### 8d. Agents grid (after line 320)

Add a fourth agent card:

```html
<div class="agent-card">
  <img src="icons/opencode.png" alt="OpenCode" class="agent-icon" />
  <span class="agent-name">OpenCode</span>
  <span class="agent-badge agent-badge-supported">Supported</span>
</div>
```

During development (before the release that ships OpenCode support), use `agent-badge-coming-soon` instead of `agent-badge-supported` so the card reads "Coming Soon" — then flip it on release.

### 8e. README.md

Update the **Supported Agents** table (line 36) to add OpenCode as a fourth row:

```markdown
| **OpenCode** | Scans `~/.local/share/opencode/opencode.db` for sessions grouped by `directory` | `fsnotify` on the OpenCode data dir (`opencode.db` + WAL sidecars, debounced 2 s) | SQLite — `session_v2` + `session_message` (event-sourced) |
```

Also update the introductory sentence in the README (line 18) that says *"Watches your coding agent sessions (VS Code Copilot, Claude Code, and Cursor)"*.
