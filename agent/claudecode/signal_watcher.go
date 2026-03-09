package claudecode

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"contrails/agent"

	"github.com/fsnotify/fsnotify"
)

// SignalHandler is the callback interface that the application layer
// implements to handle Claude Code signal events. This inverts the
// dependency: SignalWatcher depends on the abstraction, not on concrete
// application types like Project or WatcherEvent.
type SignalHandler interface {
	// FindProject matches a working directory to a registered project.
	// Returns the project ID, output directory, and whether a match was found.
	FindProject(workingDirectory string) (projectID, outputDir string, found bool)

	// WriteContrail writes the parsed session as a markdown file and returns the output path.
	WriteContrail(session *agent.ParsedSession, outputDir string) (outputPath string, err error)

	// EmitWatcherEvent notifies the frontend that a contrail file was created/modified/removed.
	EmitWatcherEvent(projectID, fileName, eventType string)

	// EmitFileProcessed notifies the frontend that a source file was processed.
	EmitFileProcessed(projectID, fileName string)

	// IsChatIgnored returns true if the chat file is in the project's ignored list.
	IsChatIgnored(projectID, filePath string) bool
}

// SignalWatcher watches the ~/contrails/hook-signals/ directory for new
// signal files from the Claude Code Stop hook. When a signal arrives, it
// uses the SignalHandler to match the cwd against known projects, parses
// the transcript, and writes the contrail markdown.
type SignalWatcher struct {
	ctx             context.Context
	cancel          context.CancelFunc
	fsWatcher       *fsnotify.Watcher
	logger          agent.Logger
	parser          *Parser
	handler         SignalHandler
	signalDirectory string

	// projectPaths tracks registered workspace paths so the watcher
	// can do its own prefix matching without depending on Project types.
	mutex sync.Mutex
	paths map[string]bool // workspacePath → registered
}

// NewSignalWatcher creates a new signal watcher. Call Start() to begin watching.
func NewSignalWatcher(logger agent.Logger, handler SignalHandler) (*SignalWatcher, error) {
	signalDirectory, err := EnsureSignalDirectory()
	if err != nil {
		return nil, fmt.Errorf("ensuring signal directory: %w", err)
	}

	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("creating fsnotify watcher: %w", err)
	}

	return &SignalWatcher{
		fsWatcher:       fsWatcher,
		logger:          logger,
		parser:          &Parser{},
		handler:         handler,
		signalDirectory: signalDirectory,
		paths:           make(map[string]bool),
	}, nil
}

// Start begins watching the signal directory for new files.
func (watcher *SignalWatcher) Start(applicationContext context.Context) {
	ctx, cancel := context.WithCancel(applicationContext)
	watcher.ctx = ctx
	watcher.cancel = cancel

	if err := watcher.fsWatcher.Add(watcher.signalDirectory); err != nil {
		agent.LogErrorf(watcher.logger, "Failed to watch signal directory %s: %v", watcher.signalDirectory, err)
		return
	}

	go watcher.eventLoop()
}

// Stop stops the signal watcher and releases resources.
func (watcher *SignalWatcher) Stop() {
	if watcher.cancel != nil {
		watcher.cancel()
	}
	watcher.fsWatcher.Close()
}

// RegisterPath adds a workspace path so that incoming signals matching it
// will be processed.
func (watcher *SignalWatcher) RegisterPath(workspacePath string) {
	if workspacePath == "" {
		return
	}
	watcher.mutex.Lock()
	defer watcher.mutex.Unlock()
	watcher.paths[workspacePath] = true
}

// UnregisterPath removes a workspace path from signal matching.
func (watcher *SignalWatcher) UnregisterPath(workspacePath string) {
	watcher.mutex.Lock()
	defer watcher.mutex.Unlock()
	delete(watcher.paths, workspacePath)
}

// eventLoop processes filesystem events from the signal directory.
func (watcher *SignalWatcher) eventLoop() {
	for {
		select {
		case <-watcher.ctx.Done():
			return

		case event, ok := <-watcher.fsWatcher.Events:
			if !ok {
				return
			}

			// Only process newly created .json signal files
			if !event.Has(fsnotify.Create) {
				continue
			}
			if !strings.HasSuffix(event.Name, ".json") {
				continue
			}

			// Brief delay for the file to be fully written
			time.Sleep(50 * time.Millisecond)

			watcher.processSignalFile(event.Name)

		case err, ok := <-watcher.fsWatcher.Errors:
			if !ok {
				return
			}
			agent.LogErrorf(watcher.logger, "Signal watcher error: %v", err)
		}
	}
}

// processSignalFile reads a signal file, matches it to a project via the
// SignalHandler, and triggers parsing of the Claude Code transcript.
func (watcher *SignalWatcher) processSignalFile(signalPath string) {
	// // Read the raw signal file before we consume (and delete) it
	// rawSignalBytes, readSignalErr := os.ReadFile(signalPath)

	signal, err := ConsumeSignalFile(signalPath)
	if err != nil {
		agent.LogWarningf(watcher.logger, "Failed to consume signal file %s: %v", filepath.Base(signalPath), err)
		return
	}

	// Skip if stop_hook_active is true (intermediate state)
	if signal.StopHookActive {
		agent.LogInfof(watcher.logger, "Skipping signal (stop_hook_active=true) for session %s", signal.SessionID)
		return
	}

	// Match cwd to a known project via the handler
	projectID, outputDir, found := watcher.handler.FindProject(signal.Cwd)
	if !found {
		agent.LogWarningf(watcher.logger, "No matching project for cwd %s (session %s)", signal.Cwd, signal.SessionID)
		return
	}

	// Skip ignored chats
	if watcher.handler.IsChatIgnored(projectID, signal.TranscriptPath) {
		agent.LogInfof(watcher.logger, "Skipping ignored chat for session %s", signal.SessionID)
		return
	}

	// Parse the transcript
	if signal.TranscriptPath == "" {
		agent.LogWarningf(watcher.logger, "Signal has empty transcript_path for session %s", signal.SessionID)
		return
	}

	// Check that the transcript file exists
	if _, err := os.Stat(signal.TranscriptPath); err != nil {
		agent.LogWarningf(watcher.logger, "Transcript file not found: %s", signal.TranscriptPath)
		return
	}

	session, err := watcher.parser.ParseFile(signal.TranscriptPath)
	if err != nil {
		agent.LogWarningf(watcher.logger, "Failed to parse Claude Code transcript %s: %v", signal.TranscriptPath, err)
		return
	}

	// The transcript JSONL doesn't include the final assistant text response.
	// Claude Code delivers it separately via the stop hook's "last_assistant_message"
	// field. Append it to the parsed session so the contrail includes the actual answer.
	if signal.LastAssistantMessage != "" {
		finalPart := agent.MessagePart{
			Type:    agent.PartText,
			Content: signal.LastAssistantMessage,
		}
		// Append to the last assistant message if one exists, otherwise create a new one
		appended := false
		for i := len(session.Messages) - 1; i >= 0; i-- {
			if session.Messages[i].Role == "assistant" {
				session.Messages[i].Parts = append(session.Messages[i].Parts, finalPart)
				appended = true
				break
			}
		}
		if !appended {
			session.Messages = append(session.Messages, agent.ParsedMessage{
				Role:  "assistant",
				Parts: []agent.MessagePart{finalPart},
			})
		}
	}

	// Skip empty sessions
	if len(session.Messages) == 0 {
		return
	}

	// Write the contrail via the handler
	outputPath, err := watcher.handler.WriteContrail(session, outputDir)
	if err != nil {
		agent.LogWarningf(watcher.logger, "Failed to write contrail for session %s: %v", signal.SessionID, err)
		return
	}

	agent.LogInfof(watcher.logger, "Wrote Claude Code contrail: %s", filepath.Base(outputPath))

	// // Save a debug copy of every file we parse, using its original name with a "debug_" prefix.
	// // 1. The transcript .jsonl file
	// transcriptData, readErr := os.ReadFile(signal.TranscriptPath)
	// if readErr == nil {
	// 	debugFileName := "debug_" + filepath.Base(signal.TranscriptPath)
	// 	debugPath := filepath.Join(filepath.Dir(outputPath), debugFileName)
	// 	if writeErr := os.WriteFile(debugPath, transcriptData, 0644); writeErr != nil {
	// 		agent.LogWarningf(watcher.logger, "Failed to write debug transcript file %s: %v", debugFileName, writeErr)
	// 	} else {
	// 		agent.LogInfof(watcher.logger, "Wrote debug copy: %s", debugFileName)
	// 	}
	// }
	//
	// // 2. The parsed signal .json file
	// if readSignalErr == nil {
	// 	debugSignalFileName := "debug_" + filepath.Base(signalPath)
	// 	debugSignalPath := filepath.Join(filepath.Dir(outputPath), debugSignalFileName)
	// 	if writeErr := os.WriteFile(debugSignalPath, rawSignalBytes, 0644); writeErr != nil {
	// 		agent.LogWarningf(watcher.logger, "Failed to write debug signal file %s: %v", debugSignalFileName, writeErr)
	// 	} else {
	// 		agent.LogInfof(watcher.logger, "Wrote debug copy: %s", debugSignalFileName)
	// 	}
	// }

	// Emit events via the handler
	watcher.handler.EmitWatcherEvent(projectID, filepath.Base(outputPath), "created")
	watcher.handler.EmitFileProcessed(projectID, filepath.Base(signal.TranscriptPath))
}
