package main

import (
	"testing"
)

// --- NoopLogger ---

func TestNoopLogger_NoPanic(t *testing.T) {
	logger := &NoopLogger{}
	// Should not panic on any call
	logger.Info("info message")
	logger.Warning("warning message")
	logger.Error("error message")
}

// --- NoopEmitter ---

func TestNoopEmitter_NoPanic(t *testing.T) {
	emitter := &NoopEmitter{}
	emitter.Emit("some:event")
	emitter.Emit("some:event", "data1", "data2")
}

// --- RecordingLogger ---

func TestRecordingLogger_CapturesInfo(t *testing.T) {
	logger := &RecordingLogger{}
	logger.Info("hello")
	logger.Info("world")

	if len(logger.InfoMessages) != 2 {
		t.Fatalf("expected 2 info messages, got %d", len(logger.InfoMessages))
	}
	if logger.InfoMessages[0] != "hello" {
		t.Errorf("expected 'hello', got %q", logger.InfoMessages[0])
	}
	if logger.InfoMessages[1] != "world" {
		t.Errorf("expected 'world', got %q", logger.InfoMessages[1])
	}
}

func TestRecordingLogger_CapturesWarning(t *testing.T) {
	logger := &RecordingLogger{}
	logger.Warning("watch out")

	if len(logger.WarningMessages) != 1 {
		t.Fatalf("expected 1 warning message, got %d", len(logger.WarningMessages))
	}
	if logger.WarningMessages[0] != "watch out" {
		t.Errorf("expected 'watch out', got %q", logger.WarningMessages[0])
	}
}

func TestRecordingLogger_CapturesError(t *testing.T) {
	logger := &RecordingLogger{}
	logger.Error("something broke")

	if len(logger.ErrorMessages) != 1 {
		t.Fatalf("expected 1 error message, got %d", len(logger.ErrorMessages))
	}
	if logger.ErrorMessages[0] != "something broke" {
		t.Errorf("expected 'something broke', got %q", logger.ErrorMessages[0])
	}
}

func TestRecordingLogger_IsolatesLevels(t *testing.T) {
	logger := &RecordingLogger{}
	logger.Info("info")
	logger.Warning("warning")
	logger.Error("error")

	if len(logger.InfoMessages) != 1 {
		t.Errorf("info: expected 1, got %d", len(logger.InfoMessages))
	}
	if len(logger.WarningMessages) != 1 {
		t.Errorf("warning: expected 1, got %d", len(logger.WarningMessages))
	}
	if len(logger.ErrorMessages) != 1 {
		t.Errorf("error: expected 1, got %d", len(logger.ErrorMessages))
	}
}

// --- RecordingEmitter ---

func TestRecordingEmitter_CapturesEvents(t *testing.T) {
	emitter := &RecordingEmitter{}
	emitter.Emit("file:processed", "data1")

	if len(emitter.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(emitter.Events))
	}
	if emitter.Events[0].Name != "file:processed" {
		t.Errorf("expected event name 'file:processed', got %q", emitter.Events[0].Name)
	}
	if len(emitter.Events[0].Data) != 1 {
		t.Fatalf("expected 1 data arg, got %d", len(emitter.Events[0].Data))
	}
	if emitter.Events[0].Data[0] != "data1" {
		t.Errorf("expected data 'data1', got %v", emitter.Events[0].Data[0])
	}
}

func TestRecordingEmitter_CapturesMultipleEvents(t *testing.T) {
	emitter := &RecordingEmitter{}
	emitter.Emit("event:a")
	emitter.Emit("event:b", "x", "y")

	if len(emitter.Events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(emitter.Events))
	}
	if emitter.Events[1].Name != "event:b" {
		t.Errorf("expected 'event:b', got %q", emitter.Events[1].Name)
	}
	if len(emitter.Events[1].Data) != 2 {
		t.Fatalf("expected 2 data args, got %d", len(emitter.Events[1].Data))
	}
}

func TestRecordingEmitter_NoData(t *testing.T) {
	emitter := &RecordingEmitter{}
	emitter.Emit("empty:event")

	if len(emitter.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(emitter.Events))
	}
	if len(emitter.Events[0].Data) != 0 {
		t.Errorf("expected 0 data args, got %d", len(emitter.Events[0].Data))
	}
}

// --- logInfof / logWarningf / logErrorf ---

func TestLogInfof_FormatsAndDelegates(t *testing.T) {
	logger := &RecordingLogger{}
	logInfof(logger, "processed %d file(s) for %s", 3, "my-project")

	if len(logger.InfoMessages) != 1 {
		t.Fatalf("expected 1 info message, got %d", len(logger.InfoMessages))
	}
	expected := "processed 3 file(s) for my-project"
	if logger.InfoMessages[0] != expected {
		t.Errorf("expected %q, got %q", expected, logger.InfoMessages[0])
	}
}

func TestLogWarningf_FormatsAndDelegates(t *testing.T) {
	logger := &RecordingLogger{}
	logWarningf(logger, "failed: %v", "timeout")

	if len(logger.WarningMessages) != 1 {
		t.Fatalf("expected 1 warning message, got %d", len(logger.WarningMessages))
	}
	expected := "failed: timeout"
	if logger.WarningMessages[0] != expected {
		t.Errorf("expected %q, got %q", expected, logger.WarningMessages[0])
	}
}

func TestLogErrorf_FormatsAndDelegates(t *testing.T) {
	logger := &RecordingLogger{}
	logErrorf(logger, "crash in %s: code %d", "handler", 500)

	if len(logger.ErrorMessages) != 1 {
		t.Fatalf("expected 1 error message, got %d", len(logger.ErrorMessages))
	}
	expected := "crash in handler: code 500"
	if logger.ErrorMessages[0] != expected {
		t.Errorf("expected %q, got %q", expected, logger.ErrorMessages[0])
	}
}
