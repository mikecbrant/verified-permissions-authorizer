package logging

// Logger is a tiny leveled logger for internal library use.
// It intentionally mirrors only the calls we need so tests can pass fakes.
type Logger interface {
    Debugf(format string, args ...any)
    Infof(format string, args ...any)
    Warnf(format string, args ...any)
}

// NopLogger discards all logs.
type NopLogger struct{}

func (NopLogger) Debugf(string, ...any) {}
func (NopLogger) Infof(string, ...any)  {}
func (NopLogger) Warnf(string, ...any)  {}
