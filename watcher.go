package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"contrails/agent/vscode"

	"github.com/fsnotify/fsnotify"
)

// Watcher manages filesystem watching for chat session directories
// Style: Descriptive Naming (go-style-guide.md)
type Watcher struct {
	ctx       context.Context
	fsWatcher *fsnotify.Watcher
	mutex     sync.Mutex
	watching  map[string]string // watchDir -> projectID
	cancel    context.CancelFunc
	logger    Logger
	emitter   EventEmitter
}

// NewWatcher creates a new filesystem watcher
func NewWatcher(logger Logger, emitter EventEmitter) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("creating watcher: %w", err)
	}
	return &Watcher{
		fsWatcher: fsWatcher,
		watching:  make(map[string]string),
		logger:    logger,
		emitter:   emitter,
	}, nil
}

// Start begins processing watcher events
func (watcher *Watcher) Start(appCtx context.Context) {
	ctx, cancel := context.WithCancel(appCtx)
	watcher.ctx = ctx
	watcher.cancel = cancel

	go watcher.eventLoop()
}

// Stop stops the watcher
func (watcher *Watcher) Stop() {
	if watcher.cancel != nil {
		watcher.cancel()
	}
	watcher.fsWatcher.Close()
}

// AddWatch starts watching a directory for a project
func (watcher *Watcher) AddWatch(projectID, watchDir string) error {
	watcher.mutex.Lock()
	defer watcher.mutex.Unlock()

	// Expand ~ in path
	if strings.HasPrefix(watchDir, "~") {
		home, _ := os.UserHomeDir()
		watchDir = filepath.Join(home, watchDir[1:])
	}

	if err := watcher.fsWatcher.Add(watchDir); err != nil {
		return fmt.Errorf("watching %s: %w", watchDir, err)
	}

	watcher.watching[watchDir] = projectID
	return nil
}

// RemoveWatch stops watching a directory
func (watcher *Watcher) RemoveWatch(watchDir string) {
	watcher.mutex.Lock()
	defer watcher.mutex.Unlock()

	watcher.fsWatcher.Remove(watchDir)
	delete(watcher.watching, watchDir)
}

func (watcher *Watcher) eventLoop() {
	for {
		select {
		case <-watcher.ctx.Done():
			return
		case event, ok := <-watcher.fsWatcher.Events:
			if !ok {
				return
			}
			// Only care about JSON/JSONL chat session files
			if !vscode.IsChatSessionFile(event.Name) {
				continue
			}

			dir := filepath.Dir(event.Name)
			watcher.mutex.Lock()
			projectID, found := watcher.watching[dir]
			watcher.mutex.Unlock()

			if !found {
				continue
			}

			var eventType string
			switch {
			case event.Has(fsnotify.Create):
				eventType = "created"
			case event.Has(fsnotify.Write):
				eventType = "modified"
			case event.Has(fsnotify.Remove):
				eventType = "removed"
			default:
				continue
			}

			watcher.emitter.Emit("watcher:event", WatcherEvent{
				ProjectID: projectID,
				FileName:  filepath.Base(event.Name),
				EventType: eventType,
			})

		case err, ok := <-watcher.fsWatcher.Errors:
			if !ok {
				return
			}
			logErrorf(watcher.logger, "Watcher error: %v", err)
		}
	}
}
