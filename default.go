package logger

import (
	"context"
	"fmt"
	"os"
	"path"

	"go.uber.org/multierr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var _ Logger = (*logger)(nil)

type logger struct {
	opt           Options
	base          *zap.Logger
	ctx           context.Context
	atomicLevel   zap.AtomicLevel
	_writeSyncers []zapcore.WriteSyncer
}

func New(opts ...Option) Logger {
	opt := newOptions(opts...)
	l := &logger{
		opt:         opt,
		atomicLevel: zap.NewAtomicLevelAt(opt.level.unmarshalZapLevel()),
	}

	if err := l.build(); err != nil {
		panic(err)
	}
	return l
}

func (l *logger) build() error {
	var (
		cores []zapcore.Core
	)

	if l.opt.filename != "" && l.opt.console { // 指定文件终端输出
		cores = append(cores, l.buildFileConsole())
	} else if l.opt.filename == "" && l.opt.console { // 开启终端输出
		cores = append(cores, l.buildConsole()...)
	}

	// 指定文件日志输出
	if l.opt.filename != "" && !l.opt.disableDisk {
		_cores, err := l.buildFile()
		if err != nil {
			return err
		}
		cores = append(cores, _cores...)
	} else if !l.opt.disableDisk && l.opt.filename == "" {
		_cores, err := l.buildFiles()
		if err != nil {
			return err
		}
		cores = append(cores, _cores...)
	}

	zapLog := zap.New(zapcore.NewTee(cores...)).WithOptions(zap.AddCaller(), zap.AddCallerSkip(l.opt.callerSkip))
	if l.opt.fields != nil {
		zapLog = zapLog.With(CopyFields(l.opt.fields)...)
	}
	if l.opt.namespace != "" {
		zapLog = zapLog.With(zap.Namespace(l.opt.namespace))
	}

	l.base = zapLog

	return nil
}

func (l *logger) buildEncoder(cfg Options) zapcore.Encoder {
	if cfg.encoder.IsConsole() {
		return zapcore.NewConsoleEncoder(cfg.encoderConfig)
	}
	return zapcore.NewJSONEncoder(cfg.encoderConfig)
}

func (l *logger) LevelEnablerFunc(level zapcore.Level) zap.LevelEnablerFunc {
	enabled := l.atomicLevel.Enabled(level)
	if level == zapcore.FatalLevel {
		return func(lvl zapcore.Level) bool {
			return enabled && lvl >= level
		}
	}
	return func(lvl zapcore.Level) bool {
		return enabled && lvl == level
	}
}

func CopyFields(fields map[string]interface{}) []zap.Field {
	dst := make([]zap.Field, 0, len(fields))
	for k, v := range fields {
		dst = append(dst, zap.Any(k, v))
	}
	return dst
}

func (l *logger) buildConsole() []zapcore.Core {
	syncerStdout := zapcore.AddSync(os.Stdout)
	syncerStderr := zapcore.AddSync(os.Stderr)
	enc := l.buildEncoder(l.opt)

	return []zapcore.Core{
		zapcore.NewCore(enc, syncerStdout, l.LevelEnablerFunc(zap.DebugLevel)),
		zapcore.NewCore(enc, syncerStdout, l.LevelEnablerFunc(zap.InfoLevel)),
		zapcore.NewCore(enc, syncerStdout, l.LevelEnablerFunc(zap.WarnLevel)),
		zapcore.NewCore(enc, syncerStderr, l.LevelEnablerFunc(zap.ErrorLevel)),
		zapcore.NewCore(enc, syncerStderr, l.LevelEnablerFunc(zap.FatalLevel)),
	}
}

func (l *logger) buildFileConsole() zapcore.Core {
	return zapcore.NewCore(l.buildEncoder(l.opt), zapcore.AddSync(os.Stdout), l.atomicLevel)
}

func (l *logger) buildFile() ([]zapcore.Core, error) {
	if err := l.Sync(); err != nil {
		return nil, err
	}

	cores := make([]zapcore.Core, 0, 1)
	var enc zapcore.Encoder
	if l.opt.encoder.IsConsole() {
		enc = zapcore.NewConsoleEncoder(l.opt.encoderConfig)
	} else {
		enc = zapcore.NewJSONEncoder(l.opt.encoderConfig)
	}

	syncerRolling, err := l.createOutput(l.opt.filename)

	if err != nil {
		return nil, err
	}

	cores = append(cores,
		zapcore.NewCore(enc, syncerRolling, l.atomicLevel),
	)

	l._writeSyncers = append(l._writeSyncers, []zapcore.WriteSyncer{syncerRolling}...)

	return cores, nil
}

func (l *logger) buildFiles() ([]zapcore.Core, error) {
	var (
		err   error
		cores = make([]zapcore.Core, 0, 5)
		syncerRollingDebug, syncerRollingInfo, syncerRollingWarn,
		syncerRollingError, syncerRollingFatal zapcore.WriteSyncer
	)

	if err = l.Sync(); err != nil {
		return nil, err
	}

	syncerRollingDebug, err = l.createOutput(debugFilename)
	if err != nil {
		return nil, err
	}
	syncerRollingInfo, err = l.createOutput(infoFilename)
	if err != nil {
		return nil, err
	}
	syncerRollingWarn, err = l.createOutput(warnFilename)
	if err != nil {
		return nil, err
	}
	syncerRollingError, err = l.createOutput(errorFilename)
	if err != nil {
		return nil, err
	}
	syncerRollingFatal, err = l.createOutput(fatalFilename)
	if err != nil {
		return nil, err
	}

	enc := l.buildEncoder(l.opt)
	cores = append(cores,
		zapcore.NewCore(enc, syncerRollingDebug, l.LevelEnablerFunc(zap.DebugLevel)),
		zapcore.NewCore(enc, syncerRollingInfo, l.LevelEnablerFunc(zap.InfoLevel)),
		zapcore.NewCore(enc, syncerRollingWarn, l.LevelEnablerFunc(zap.WarnLevel)),
		zapcore.NewCore(enc, syncerRollingError, l.LevelEnablerFunc(zap.ErrorLevel)),
		zapcore.NewCore(enc, syncerRollingFatal, l.LevelEnablerFunc(zap.FatalLevel)),
	)

	l._writeSyncers = append(l._writeSyncers, []zapcore.WriteSyncer{syncerRollingDebug, syncerRollingInfo, syncerRollingWarn, syncerRollingError, syncerRollingFatal}...)

	return cores, nil
}

func (l *logger) createOutput(filename string) (zapcore.WriteSyncer, error) {
	if len(filename) == 0 {
		return nil, ErrLogPathNotSet
	}

	rollingFile, err := NewRollingFile(path.Join(l.opt.basePath, filename), HourlyRolling)
	if err != nil {
		return nil, err
	}

	return zapcore.AddSync(rollingFile), nil
}

func (l *logger) Clone() *logger {
	_copy := *l
	return &_copy
}

func (l *logger) Init(opts ...Option) error {
	// process options
	for _, o := range opts {
		o(&l.opt)
	}

	return nil
}

func (l *logger) SetLevel(lv Level) {
	l.opt.level = lv
	l.atomicLevel.SetLevel(lv.unmarshalZapLevel())
}

func (l *logger) Options() Options {
	return l.opt
}

func (l *logger) WithContext(ctx context.Context) Logger {
	logger := &logger{
		ctx:         ctx,
		opt:         l.opt,
		atomicLevel: l.atomicLevel,
		base:        l.base.WithOptions(zap.AddCallerSkip(0)),
	}
	return logger
}

func (l *logger) WithFields(fields map[string]interface{}) Logger {
	return &logger{
		opt:         l.opt,
		atomicLevel: l.atomicLevel,
		base:        l.base.With(CopyFields(fields)...).WithOptions(zap.AddCallerSkip(0)),
	}
}

func (l *logger) WithCallDepth(callDepth int) Logger {
	return &logger{
		opt:         l.opt,
		atomicLevel: l.atomicLevel,
		base:        l.base.WithOptions(zap.AddCallerSkip(callDepth)),
	}
}

// Debug uses fmt.Sprint to construct and log a message.
func (l *logger) Debug(args ...interface{}) {
	l.log(DebugLevel, "", args, nil)
}

// Info uses fmt.Sprint to construct and log a message.
func (l *logger) Info(args ...interface{}) {
	l.log(InfoLevel, "", args, nil)
}

// Warn uses fmt.Sprint to construct and log a message.
func (l *logger) Warn(args ...interface{}) {
	l.log(WarnLevel, "", args, nil)
}

// Error uses fmt.Sprint to construct and log a message.
func (l *logger) Error(args ...interface{}) {
	l.log(ErrorLevel, "", args, nil)
}

// Fatal uses fmt.Sprint to construct and log a message, then calls os.Exit.
// Deprecated: 记录消息后，直接调用 os.Exit(1)，这意味着：
// 在其他 goroutine defer 语句不会被执行；
// 各种 buffers 不会被 flush，包括日志的；
// 临时文件或者目录不会被移除；
// 不要使用 fatal 记录日志，而是向调用者返回错误。如果错误一直持续到 main.main。main.main 那就是在退出之前做处理任何清理操作的正确位置。
func (l *logger) Fatal(args ...interface{}) {
	l.log(FatalLevel, "", args, nil)
}

// Debugf uses fmt.Sprintf to log a templated message.
func (l *logger) Debugf(template string, args ...interface{}) {
	l.log(DebugLevel, template, args, nil)
}

// Infof uses fmt.Sprintf to log a templated message.
func (l *logger) Infof(template string, args ...interface{}) {
	l.log(InfoLevel, template, args, nil)
}

// Warnf uses fmt.Sprintf to log a templated message.
func (l *logger) Warnf(template string, args ...interface{}) {
	l.log(WarnLevel, template, args, nil)
}

// Errorf uses fmt.Sprintf to log a templated message.
func (l *logger) Errorf(template string, args ...interface{}) {
	l.log(ErrorLevel, template, args, nil)
}

// Fatalf uses fmt.Sprintf to log a templated message, then calls os.Exit.
// Deprecated: 记录消息后，直接调用 os.Exit(1)，这意味着：
// 在其他 goroutine defer 语句不会被执行；
// 各种 buffers 不会被 flush，包括日志的；
// 临时文件或者目录不会被移除；
// 不要使用 fatal 记录日志，而是向调用者返回错误。如果错误一直持续到 main.main。main.main 那就是在退出之前做处理任何清理操作的正确位置。
func (l *logger) Fatalf(template string, args ...interface{}) {
	l.log(FatalLevel, template, args, nil)
}

// Debugw logs a message with some additional context. The variadic key-value
// pairs are treated as they are in With.
//
// When debug-level logging is disabled, this is much faster than
//
//	s.With(keysAndValues).Debug(msg)
func (l *logger) Debugw(msg string, keysAndValues ...interface{}) {
	l.log(DebugLevel, msg, nil, keysAndValues)
}

// Infow logs a message with some additional context. The variadic key-value
// pairs are treated as they are in With.
func (l *logger) Infow(msg string, keysAndValues ...interface{}) {
	l.log(InfoLevel, msg, nil, keysAndValues)
}

// Warnw logs a message with some additional context. The variadic key-value
// pairs are treated as they are in With.
func (l *logger) Warnw(msg string, keysAndValues ...interface{}) {
	l.log(WarnLevel, msg, nil, keysAndValues)
}

// Errorw logs a message with some additional context. The variadic key-value
// pairs are treated as they are in With.
func (l *logger) Errorw(msg string, keysAndValues ...interface{}) {
	l.log(ErrorLevel, msg, nil, keysAndValues)
}

// Fatalw logs a message with some additional context, then calls os.Exit. The
// variadic key-value pairs are treated as they are in With.
// Deprecated: 记录消息后，直接调用 os.Exit(1)，这意味着：
// 在其他 goroutine defer 语句不会被执行；
// 各种 buffers 不会被 flush，包括日志的；
// 临时文件或者目录不会被移除；
// 不要使用 fatal 记录日志，而是向调用者返回错误。如果错误一直持续到 main.main。main.main 那就是在退出之前做处理任何清理操作的正确位置。
func (l *logger) Fatalw(msg string, keysAndValues ...interface{}) {
	l.log(FatalLevel, msg, nil, keysAndValues)
}

func (l *logger) Sync() error {
	if l.base != nil {
		return l.base.Sync()
	}

	for _, w := range l._writeSyncers {
		r, ok := w.(*RollingFile)
		if ok {
			r.Close()
		}
	}

	return nil
}

// getMessage format with Sprint, Sprintf, or neither.
func getMessage(template string, fmtArgs []interface{}) string {
	if len(fmtArgs) == 0 {
		return template
	}

	if template != "" {
		return fmt.Sprintf(template, fmtArgs...)
	}

	if len(fmtArgs) == 1 {
		if str, ok := fmtArgs[0].(string); ok {
			return str
		}
	}
	return fmt.Sprint(fmtArgs...)
}

type invalidPair struct {
	position   int
	key, value interface{}
}

func (p invalidPair) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddInt64("position", int64(p.position))
	zap.Any("key", p.key).AddTo(enc)
	zap.Any("value", p.value).AddTo(enc)
	return nil
}

type invalidPairs []invalidPair

func (ps invalidPairs) MarshalLogArray(enc zapcore.ArrayEncoder) error {
	var err error
	for i := range ps {
		err = multierr.Append(err, enc.AppendObject(ps[i]))
	}
	return err
}

const (
	_oddNumberErrMsg    = "Ignored key without a value."
	_nonStringKeyErrMsg = "Ignored key-value pairs with non-string keys."
)

func (l *logger) sweetenFields(args []interface{}) []zap.Field {
	if len(args) == 0 {
		return nil
	}

	// Allocate enough space for the worst case; if users pass only structured
	// fields, we shouldn't penalize them with extra allocations.
	fields := make([]zap.Field, 0, len(args))
	var invalid invalidPairs

	for i := 0; i < len(args); {
		// This is a strongly-typed field. Consume it and move on.
		if f, ok := args[i].(zap.Field); ok {
			fields = append(fields, f)
			i++
			continue
		}

		// Make sure this element isn't a dangling key.
		if i == len(args)-1 {
			l.base.Error(_oddNumberErrMsg, zap.Any("ignored", args[i]))
			break
		}

		// Consume this value and the next, treating them as a key-value pair. If the
		// key isn't a string, add this pair to the slice of invalid pairs.
		key, val := args[i], args[i+1]
		if keyStr, ok := key.(string); !ok {
			// Subsequent errors are likely, so allocate once up front.
			if cap(invalid) == 0 {
				invalid = make(invalidPairs, 0, len(args)/2)
			}
			invalid = append(invalid, invalidPair{i, key, val})
		} else {
			fields = append(fields, zap.Any(keyStr, val))
		}
		i += 2
	}

	// If we encountered any invalid key-value pairs, log an error.
	if len(invalid) > 0 {
		l.base.Error(_nonStringKeyErrMsg, zap.Array("invalid", invalid))
	}
	return fields
}

func (l *logger) log(level Level, template string, fmtArgs []interface{}, context []interface{}) {
	bindValues(l.ctx, fmtArgs)
	// If logging at this level is completely disabled, skip the overhead of
	// string formatting.
	if level < DebugLevel || !l.base.Core().Enabled(level.unmarshalZapLevel()) {
		return
	}

	msg := getMessage(template, fmtArgs)
	if ce := l.base.Check(level.unmarshalZapLevel(), msg); ce != nil {
		ce.Write(l.sweetenFields(context)...)
	}
}

func (l *logger) String() string {
	return "zap"
}
