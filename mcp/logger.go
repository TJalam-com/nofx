package mcp

import "log"

// Logger interface (abstract dependency)
// Uses Printf-style method names for easy integration with mainstream logging libraries like logrus, zap, etc.
type Logger interface {
	Debugf(format string, args ...any)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
}

// defaultLogger default logger implementation (wraps standard library log)
type defaultLogger struct{}

func (l *defaultLogger) Debugf(format string, args ...any) {
	log.Printf("[DEBUG] "+format, args...)
}

func (l *defaultLogger) Infof(format string, args ...any) {
	log.Printf("[INFO] "+format, args...)
}

func (l *defaultLogger) Warnf(format string, args ...any) {
	log.Printf("[WARN] "+format, args...)
}

func (l *defaultLogger) Errorf(format string, args ...any) {
	log.Printf("[ERROR] "+format, args...)
}

// noopLogger no-op logger implementation (used in tests)
type noopLogger struct{}

func (l *noopLogger) Debugf(format string, args ...any) {}
func (l *noopLogger) Infof(format string, args ...any)  {}
func (l *noopLogger) Warnf(format string, args ...any)  {}
func (l *noopLogger) Errorf(format string, args ...any) {}

// NewNoopLogger creates no-op logger (for testing)
func NewNoopLogger() Logger {
	return &noopLogger{}
}

// ============================================================
// Third-party logging library adapter examples
// ============================================================

// Logrus adapter example:
// type LogrusLogger struct {
//     logger *logrus.Logger
// }
//
// func (l *LogrusLogger) Infof(format string, args ...any) {
//     l.logger.Infof(format, args...)
// }
//
// Zap adapter example:
// type ZapLogger struct {
//     logger *zap.Logger
// }
//
// func (l *ZapLogger) Infof(format string, args ...any) {
//     l.logger.Sugar().Infof(format, args...)
// }
//
// Then inject via WithLogger(logger)
