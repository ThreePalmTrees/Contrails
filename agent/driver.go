package agent

// ProcessCallbacks carries optional callback functions that the
// application layer passes to ProcessAll for progress reporting
// and per-file event emission.
type ProcessCallbacks struct {
	// OnProgress is called with (current, total) for each file
	// before processing begins. May be nil.
	OnProgress func(current, total int)

	// OnFileProcessed is called with the output filename and
	// processing duration after each file is successfully written.
	// May be nil.
	OnFileProcessed func(outputFileName string, durationMs int64)
}

// AgentDriver encapsulates agent-specific lifecycle behavior.
// Each agent type (VS Code, Claude Code, Cursor, etc.) implements
// this interface so that the application layer can manage projects
// without agent-type-specific branching.
//
// The application uses a driver registry (map of source type → driver)
// to dispatch lifecycle calls. Adding a new agent requires implementing
// this interface and registering it — no changes to existing code.
type AgentDriver interface {
	// Setup performs one-time initialization when a source of this type
	// is added to a project (e.g., installing hooks, creating directories).
	// Called once during AddProject.
	Setup(workspacePath string) error

	// Teardown cleans up when a source of this type is removed from
	// a project (e.g., uninstalling hooks). Called once during RemoveProject.
	Teardown(workspacePath string) error

	// Activate starts watching/listening for changes for a project.
	// Called on startup for active projects, when a project is added,
	// and when a paused project is resumed.
	Activate(projectID, workspacePath, watchDir string) error

	// Deactivate stops watching/listening for a project.
	// Called when a project is paused or removed.
	Deactivate(workspacePath, watchDir string)

	// ProcessAll processes all sessions in sourceDir and writes
	// contrail markdown files to outputDir. This is the "Process All Now"
	// path. Returns the number of sessions processed.
	ProcessAll(sourceDir, outputDir string, callbacks ProcessCallbacks) (int, error)
}
