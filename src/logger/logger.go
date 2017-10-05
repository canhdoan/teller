package logger

import (
	"context"
	"io"
	"os"
	"path"
	"runtime"
	"strings"

	prefixed "github.com/gz-c/logrus-prefixed-formatter"
	"github.com/sirupsen/logrus"
)

type ctxKey int

const (
	loggerCtxKey ctxKey = iota
)

const (
	// SkyAddrField should be used for logging skycoin addresses in the logger
	SkyAddrField = "skyAddr"
	// BtcAddrField should be used for logging skycoin addresses in the logger
	BtcAddrField = "btcAddr"
)

// FromContext return a *logrus.Logger from a context
func FromContext(ctx context.Context) logrus.FieldLogger {
	lg := ctx.Value(loggerCtxKey)
	ruslogger, ok := lg.(logrus.FieldLogger)
	if !ok {
		return nil
	}
	return ruslogger
}

// WithContext puts a logrus.FieldLogger into a context
func WithContext(ctx context.Context, lg logrus.FieldLogger) context.Context {
	return context.WithValue(ctx, loggerCtxKey, lg)
}

// NewLogger creates a logrus.Logger, which logs to os.Stdout.
// If debug is true, the log level is logrus.DebugLevel, otherwise logrus.InfoLevel.
// If logFilename is not the empty string, logs will also be written to that file,
// in addition to os.Stdout.
func NewLogger(logFilename string, debug bool) (*logrus.Logger, error) {
	log := logrus.New()
	log.Out = os.Stdout
	log.Formatter = &prefixed.TextFormatter{
		FullTimestamp:      true,
		AlwaysQuoteStrings: true,
		QuoteEmptyFields:   true,
	}
	log.Level = logrus.InfoLevel

	if debug {
		log.Level = logrus.DebugLevel
	}

	if logFilename != "" {
		hook, err := NewFileWriteHook(logFilename)
		if err != nil {
			return nil, err
		}

		log.Hooks.Add(hook)
	}

	log.Hooks.Add(contextHook{})

	return log, nil
}

// WriteHook is a logrus.Hook that logs to an io.Writer
type WriteHook struct {
	w         io.Writer
	formatter logrus.Formatter
}

// NewFileWriteHook returns a new WriteHook for a file
func NewFileWriteHook(filename string) (*WriteHook, error) {
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return nil, err
	}

	return &WriteHook{
		w: f,
		formatter: &TextFormatter{
			DisableColors: true,
			FullTimestamp: true,
		},
	}, nil
}

// NewStdoutWriteHook returns a new WriteHook for stdout
func NewStdoutWriteHook() *WriteHook {
	return &WriteHook{
		w: os.Stdout,
		formatter: &prefixed.TextFormatter{
			FullTimestamp:      true,
			AlwaysQuoteStrings: true,
			QuoteEmptyFields:   true,
		},
	}
}

// Levels returns Levels accepted by the WriteHook.
// All logrus.Levels are returned.
func (f *WriteHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// Fire writes a logrus.Entry to the file
func (f *WriteHook) Fire(e *logrus.Entry) error {
	b, err := f.formatter.Format(e)
	if err != nil {
		return err
	}

	_, err = f.w.Write(b)
	return err
}

// Adds file:func:lineno context to log lines
type contextHook struct{}

func (hook contextHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (hook contextHook) Fire(entry *logrus.Entry) error {
	pc := make([]uintptr, 3)
	n := runtime.Callers(6, pc)

	frames := runtime.CallersFrames(pc[:n])

	for {
		frame, more := frames.Next()
		if strings.Contains(frame.File, "github.com/sirupsen/logrus") {
			continue
		}
		if !more {
			break
		}

		entry.Data["file"] = path.Base(frame.File)
		entry.Data["func"] = path.Base(frame.Function)
		entry.Data["line"] = frame.Line
	}

	return nil
}
