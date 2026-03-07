package main

import (
	"context"

	"contrails/agent"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Logger is an alias to agent.Logger so the root package uses the same
// interface defined in the agent package (Dependency Inversion).
type Logger = agent.Logger

// EventEmitter abstracts event emission so that App methods can be
// unit-tested without a live Wails context.
type EventEmitter interface {
	Emit(eventName string, data ...interface{})
}

// DialogOpener abstracts native OS dialogs (directory pickers, etc.)
// so that dialog-dependent methods can be tested or stubbed.
type DialogOpener interface {
	OpenDirectoryDialog(options wailsRuntime.OpenDialogOptions) (string, error)
}

// --- Production implementations (backed by Wails runtime) ---

// WailsLogger forwards log calls to the Wails runtime.
// Style: Verify Interface Compliance (go-style-guide.md)
var _ Logger = (*WailsLogger)(nil)

type WailsLogger struct {
	ctx context.Context
}

func (l *WailsLogger) Info(msg string)    { wailsRuntime.LogInfo(l.ctx, msg) }
func (l *WailsLogger) Warning(msg string) { wailsRuntime.LogWarning(l.ctx, msg) }
func (l *WailsLogger) Error(msg string)   { wailsRuntime.LogError(l.ctx, msg) }

// WailsEventEmitter forwards event emission to the Wails runtime.
var _ EventEmitter = (*WailsEventEmitter)(nil)

type WailsEventEmitter struct {
	ctx context.Context
}

func (e *WailsEventEmitter) Emit(eventName string, data ...interface{}) {
	wailsRuntime.EventsEmit(e.ctx, eventName, data...)
}

// WailsDialogOpener forwards dialog calls to the Wails runtime.
var _ DialogOpener = (*WailsDialogOpener)(nil)

type WailsDialogOpener struct {
	ctx context.Context
}

func (d *WailsDialogOpener) OpenDirectoryDialog(options wailsRuntime.OpenDialogOptions) (string, error) {
	return wailsRuntime.OpenDirectoryDialog(d.ctx, options)
}

// --- No-op implementations (for tests and headless usage) ---

// NoopLogger discards all log messages. Useful in unit tests.
var _ Logger = (*NoopLogger)(nil)

type NoopLogger struct{}

func (l *NoopLogger) Info(string)    {}
func (l *NoopLogger) Warning(string) {}
func (l *NoopLogger) Error(string)   {}

// RecordingLogger captures log messages for test assertions.
var _ Logger = (*RecordingLogger)(nil)

type RecordingLogger struct {
	InfoMessages    []string
	WarningMessages []string
	ErrorMessages   []string
}

func (l *RecordingLogger) Info(msg string)    { l.InfoMessages = append(l.InfoMessages, msg) }
func (l *RecordingLogger) Warning(msg string) { l.WarningMessages = append(l.WarningMessages, msg) }
func (l *RecordingLogger) Error(msg string)   { l.ErrorMessages = append(l.ErrorMessages, msg) }

// RecordingEmitter captures emitted events for test assertions.
var _ EventEmitter = (*RecordingEmitter)(nil)

type RecordingEmitter struct {
	Events []EmittedEvent
}

// EmittedEvent records a single event emission.
type EmittedEvent struct {
	Name string
	Data []interface{}
}

func (e *RecordingEmitter) Emit(eventName string, data ...interface{}) {
	e.Events = append(e.Events, EmittedEvent{Name: eventName, Data: data})
}

// NoopEmitter discards all events. Useful in unit tests that don't inspect events.
var _ EventEmitter = (*NoopEmitter)(nil)

type NoopEmitter struct{}

func (e *NoopEmitter) Emit(string, ...interface{}) {}

// Sprintf helpers — keep call sites concise after the refactor.
// Delegate to the canonical implementations in the agent package.
func logInfof(l Logger, format string, args ...interface{}) {
	agent.LogInfof(l, format, args...)
}

func logWarningf(l Logger, format string, args ...interface{}) {
	agent.LogWarningf(l, format, args...)
}

func logErrorf(l Logger, format string, args ...interface{}) {
	agent.LogErrorf(l, format, args...)
}
