package logger

import (
	"context"
)

// WithContext returns a shallow copy of l with its context changed
// to ctx. The provided ctx must be non-nil.
func WithContext(ctx context.Context) Logger {
	return DefaultLogger.WithContext(ctx)
}

// SetLevel set logger level
func SetLevel(lv Level) {
	DefaultLogger.SetLevel(lv)
}

// Debug uses fmt.Sprint to construct and log a message.
func Debug(args ...interface{}) {
	DefaultLogger.WithCallDepth(1).Debug(args...)
}

// Debugf uses fmt.Sprintf to log a templated message.
func Debugf(format string, args ...interface{}) {
	DefaultLogger.WithCallDepth(1).Debugf(format, args...)
}

// Debugw logs a message with some additional context. The variadic key-value
// pairs are treated as they are in With.
//
// When debug-level logging is disabled, this is much faster than
//
//	s.With(keysAndValues).Debug(msg)
func Debugw(msg string, keysAndValues ...interface{}) {
	DefaultLogger.WithCallDepth(1).Debugw(msg, keysAndValues...)
}

// Info uses fmt.Sprint to construct and log a message.
func Info(args ...interface{}) {
	DefaultLogger.WithCallDepth(1).Info(args...)
}

// Infof uses fmt.Sprintf to log a templated message.
func Infof(format string, args ...interface{}) {
	DefaultLogger.WithCallDepth(1).Infof(format, args...)
}

// Infow logs a message with some additional context. The variadic key-value
// pairs are treated as they are in With.
func Infow(msg string, keysAndValues ...interface{}) {
	DefaultLogger.WithCallDepth(1).Infow(msg, keysAndValues...)
}

// Warn uses fmt.Sprint to construct and log a message.
func Warn(args ...interface{}) {
	DefaultLogger.WithCallDepth(1).Warn(args...)
}

// Warnf uses fmt.Sprintf to log a templated message.
func Warnf(format string, args ...interface{}) {
	DefaultLogger.WithCallDepth(1).Warnf(format, args...)
}

// Warnw logs a message with some additional context. The variadic key-value
// pairs are treated as they are in With.
func Warnw(msg string, keysAndValues ...interface{}) {
	DefaultLogger.WithCallDepth(1).Warnw(msg, keysAndValues...)
}

// Error uses fmt.Sprint to construct and log a message.
func Error(args ...interface{}) {
	DefaultLogger.WithCallDepth(1).Error(args...)
}

// Errorf uses fmt.Sprintf to log a templated message.
func Errorf(format string, args ...interface{}) {
	DefaultLogger.WithCallDepth(1).Errorf(format, args...)
}

// Errorw logs a message with some additional context. The variadic key-value
// pairs are treated as they are in With.
func Errorw(msg string, keysAndValues ...interface{}) {
	DefaultLogger.WithCallDepth(1).Errorw(msg, keysAndValues...)
}

// Fatal uses fmt.Sprint to construct and log a message, then calls os.Exit.
// Deprecated: 记录消息后，直接调用 os.Exit(1)，这意味着：
// 在其他 goroutine defer 语句不会被执行；
// 各种 buffers 不会被 flush，包括日志的；
// 临时文件或者目录不会被移除；
// 不要使用 fatal 记录日志，而是向调用者返回错误。如果错误一直持续到 main.main。main.main 那就是在退出之前做处理任何清理操作的正确位置。
func Fatal(args ...interface{}) {
	DefaultLogger.WithCallDepth(1).Fatal(args...)
}

// Fatalf uses fmt.Sprintf to log a templated message, then calls os.Exit.
// Deprecated: 记录消息后，直接调用 os.Exit(1)，这意味着：
// 在其他 goroutine defer 语句不会被执行；
// 各种 buffers 不会被 flush，包括日志的；
// 临时文件或者目录不会被移除；
// 不要使用 fatal 记录日志，而是向调用者返回错误。如果错误一直持续到 main.main。main.main 那就是在退出之前做处理任何清理操作的正确位置。
func Fatalf(format string, args ...interface{}) {
	DefaultLogger.WithCallDepth(1).Fatalf(format, args...)
}

// Fatalw logs a message with some additional context, then calls os.Exit. The
// variadic key-value pairs are treated as they are in With.
// Deprecated: 记录消息后，直接调用 os.Exit(1)，这意味着：
// 在其他 goroutine defer 语句不会被执行；
// 各种 buffers 不会被 flush，包括日志的；
// 临时文件或者目录不会被移除；
// 不要使用 fatal 记录日志，而是向调用者返回错误。如果错误一直持续到 main.main。main.main 那就是在退出之前做处理任何清理操作的正确位置。
func Fatalw(msg string, keysAndValues ...interface{}) {
	DefaultLogger.WithCallDepth(1).Fatalw(msg, keysAndValues...)
}

// Sync flushes any buffered log entries.
func Sync() error {
	return DefaultLogger.Sync()
}
