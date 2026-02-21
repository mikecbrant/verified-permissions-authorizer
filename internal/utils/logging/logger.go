package logging

// Fields represents structured context for a log entry.
// Keys should be short, lowerCamelCase; values must be JSON-serializable.
type Fields map[string]any

// Logger is a tiny leveled logger for internal library use.
// Callers pass a message and optional structured context (key/value pairs).
// Implementations should prefer structured output (JSON-friendly) and avoid
// interpolating user data into the message string.
type Logger interface {
	Debug(msg string, ctx Fields)
	Info(msg string, ctx Fields)
	Warn(msg string, ctx Fields)
}

// NopLogger discards all logs.
type NopLogger struct{}

// Debug discards the log entry.
func (NopLogger) Debug(string, Fields) {}

// Info discards the log entry.
func (NopLogger) Info(string, Fields) {}

// Warn discards the log entry.
func (NopLogger) Warn(string, Fields) {}
