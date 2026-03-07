package agent

import "fmt"

// Logger abstracts logging so that agent sub-packages can log without
// depending on the root application package.
type Logger interface {
	Info(msg string)
	Warning(msg string)
	Error(msg string)
}

// Logf helpers — keep call sites concise.

func LogInfof(l Logger, format string, args ...interface{}) {
	l.Info(fmt.Sprintf(format, args...))
}

func LogWarningf(l Logger, format string, args ...interface{}) {
	l.Warning(fmt.Sprintf(format, args...))
}

func LogErrorf(l Logger, format string, args ...interface{}) {
	l.Error(fmt.Sprintf(format, args...))
}
