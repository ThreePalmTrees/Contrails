package claudecode

import (
	"context"
	"sync"
	"time"

	"contrails/agent"
)

// hookEnforcementInterval is how often the enforcer checks that the
// Stop hook is present in each registered project's settings file.
const hookEnforcementInterval = 5 * time.Second

// HookEnforcer periodically ensures that the contrails Stop hook
// remains installed in .claude/settings.local.json for all registered
// workspace paths. This ensures the contrails Stop hook is re-added
// if the file is deleted or modified while Contrails is running.
type HookEnforcer struct {
	logger agent.Logger

	mu             sync.Mutex
	workspacePaths map[string]struct{}

	cancel context.CancelFunc
	done   chan struct{}
}

// NewHookEnforcer creates a HookEnforcer. Call Start to begin the
// periodic enforcement loop.
func NewHookEnforcer(logger agent.Logger) *HookEnforcer {
	return &HookEnforcer{
		logger:         logger,
		workspacePaths: make(map[string]struct{}),
	}
}

// Start begins the periodic enforcement loop. It runs until Stop is
// called or the parent context is cancelled.
func (enforcer *HookEnforcer) Start(parentContext context.Context) {
	ctx, cancel := context.WithCancel(parentContext)
	enforcer.cancel = cancel
	enforcer.done = make(chan struct{})

	go enforcer.loop(ctx)
}

// Stop stops the enforcement loop and waits for it to exit.
func (enforcer *HookEnforcer) Stop() {
	if enforcer.cancel != nil {
		enforcer.cancel()
	}
	if enforcer.done != nil {
		<-enforcer.done
	}
}

// Register adds a workspace path to the enforcement set. The enforcer
// will ensure the Stop hook is installed at this path on every tick.
func (enforcer *HookEnforcer) Register(workspacePath string) {
	enforcer.mu.Lock()
	defer enforcer.mu.Unlock()

	enforcer.workspacePaths[workspacePath] = struct{}{}
}

// Unregister removes a workspace path from the enforcement set.
func (enforcer *HookEnforcer) Unregister(workspacePath string) {
	enforcer.mu.Lock()
	defer enforcer.mu.Unlock()

	delete(enforcer.workspacePaths, workspacePath)
}

// enforce runs a single enforcement pass: for each registered path,
// call InstallHook (which no-ops if the hook is already present).
func (enforcer *HookEnforcer) enforce() {
	enforcer.mu.Lock()
	paths := make([]string, 0, len(enforcer.workspacePaths))
	for path := range enforcer.workspacePaths {
		paths = append(paths, path)
	}
	enforcer.mu.Unlock()

	for _, workspacePath := range paths {
		if err := InstallHook(workspacePath); err != nil {
			agent.LogWarningf(enforcer.logger, "Hook enforcement failed for %s: %v", workspacePath, err)
		}
	}
}

// loop is the goroutine that periodically calls enforce.
func (enforcer *HookEnforcer) loop(ctx context.Context) {
	defer close(enforcer.done)

	ticker := time.NewTicker(hookEnforcementInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			enforcer.enforce()
		}
	}
}
