package logger

import (
	"errors"

	"go.uber.org/zap/zapcore"
)

const (
	debugFilename = "debug"
	infoFilename  = "info"
	warnFilename  = "warn"
	errorFilename = "error"
	fatalFilename = "fatal"

	callerSkipOffset = 2
)

var (
	// ErrLogPathNotSet is an error that indicates the log path is not set.
	ErrLogPathNotSet = errors.New("log path must be set")
)

type Option func(o *Options)

type Options struct {
	// The logging level the Logger should log at. default is `InfoLevel`
	level Level
	// basePath defines base path of log file
	basePath string
	//  Logger file name
	filename string
	// enableConsole display logs to standard output
	console bool
	// disableDisk disable rolling file
	disableDisk bool
	// callerSkip is the number of stack frames to ascend when logging caller info.
	callerSkip int
	// namespace is the namespace of logger.
	namespace string
	// fields is the fields of logger.
	fields map[string]interface{}
	// encoder is the encoder of logger.
	encoder Encoder
	// encoderConfig is the encoder config of logger.
	encoderConfig zapcore.EncoderConfig
}

// Level Get log level.
func (o Options) Level() Level {
	return o.level
}

func newOptions(opts ...Option) Options {
	opt := Options{
		level:       InfoLevel,
		basePath:    "./logs",
		console:     true,
		disableDisk: true,
		callerSkip:  callerSkipOffset,
		encoderConfig: zapcore.EncoderConfig{
			TimeKey:        "ts",
			MessageKey:     "msg",
			LevelKey:       "level",
			CallerKey:      "caller",
			StacktraceKey:  "stack",
			LineEnding:     zapcore.DefaultLineEnding,
			NameKey:        "Logger",
			EncodeCaller:   zapcore.ShortCallerEncoder,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder, // 日期格式改为"ISO8601"，例如："2020-12-16T19:12:48.771+0800"
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeName:     zapcore.FullNameEncoder,
		},
		fields:  make(map[string]interface{}),
		encoder: JsonEncoder,
	}

	for _, o := range opts {
		o(&opt)
	}

	return opt
}

type Encoder string

func (e Encoder) String() string {
	return string(e)
}

// IsJson Whether json encoder.
func (e Encoder) IsJson() bool {
	return e.String() == JsonEncoder.String()
}

// IsConsole Whether console encoder.
func (e Encoder) IsConsole() bool {
	return e.String() == ConsoleEncoder.String()
}

const (
	JsonEncoder    Encoder = "json"
	ConsoleEncoder Encoder = "console"
)

// WithLevel set base path.
func WithLevel(lv Level) Option {
	return func(o *Options) {
		o.level = lv
	}
}

// WithBasePath set base path.
func WithBasePath(path string) Option {
	return func(o *Options) {
		o.basePath = path
	}
}

// WithFilename Logger filename.
func WithFilename(filename string) Option {
	return func(o *Options) {
		o.filename = filename
	}
}

// WithConsole enable console.
func WithConsole(enableConsole bool) Option {
	return func(o *Options) {
		o.console = enableConsole
	}
}

// WithDisableDisk disable disk.
func WithDisableDisk(disableDisk bool) Option {
	return func(o *Options) {
		o.disableDisk = disableDisk
	}
}

// WithCallerSkip increases the number of callers skipped by caller annotation
// (as enabled by the AddCaller option). When building wrappers around the
// Logger and SugaredLogger, supplying this Option prevents base from always
// reporting the wrapper code as the caller.
func WithCallerSkip(skip int) Option {
	return func(o *Options) {
		o.callerSkip = skip + callerSkipOffset
	}
}

// WithFields set default fields for the logger
func WithFields(fields map[string]interface{}) Option {
	return func(o *Options) {
		o.fields = fields
	}
}

// WithEncoder set logger Encoder
func WithEncoder(encoder Encoder) Option {
	return func(o *Options) {
		o.encoder = encoder
	}
}

// WithEncoderConfig set logger encoderConfig
func WithEncoderConfig(encoderConfig zapcore.EncoderConfig) Option {
	return func(o *Options) {
		o.encoderConfig = encoderConfig
	}
}

// WithNamespace creates a named, isolated scope within the logger's context. All
// subsequent fields will be added to the new namespace.
//
// This helps prevent key collisions when injecting loggers into sub-components
// or third-party libraries.
func WithNamespace(name string) Option {
	return func(o *Options) {
		o.namespace = name
	}
}
