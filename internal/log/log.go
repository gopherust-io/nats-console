package log

import (
	"io"
	"os"
	"strings"
	"sync/atomic"

	"github.com/rs/zerolog"
)

var (
	logger atomic.Pointer[zerolog.Logger]
	exitFn atomic.Pointer[func(int)]
)

func init() {
	defaultLogger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	logger.Store(&defaultLogger)
	fn := os.Exit
	exitFn.Store(&fn)
}

// Options configures the global logger.
type Options struct {
	Level string
	JSON  bool
}

// Init configures structured logging. Safe to call once at startup.
func Init(opts Options) {
	lvl, err := zerolog.ParseLevel(strings.TrimSpace(opts.Level))
	if err != nil {
		lvl = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(lvl)

	var out io.Writer = os.Stdout
	if !opts.JSON {
		out = zerolog.ConsoleWriter{Out: os.Stdout}
	}
	l := zerolog.New(out).With().Timestamp().Logger()
	logger.Store(&l)
}

// SetExitFunc overrides the function called after Fatal logs (for tests).
func SetExitFunc(fn func(int)) {
	exitFn.Store(&fn)
}

func getLogger() *zerolog.Logger {
	return logger.Load()
}

func Debug() *zerolog.Event {
	return getLogger().Debug()
}

func Info() *zerolog.Event {
	return getLogger().Info()
}

func Warn() *zerolog.Event {
	return getLogger().Warn()
}

func Error() *zerolog.Event {
	return getLogger().Error()
}

// Fatal logs at fatal level and invokes the configured exit function (default os.Exit).
func Fatal() *FatalEvent {
	return &FatalEvent{e: getLogger().WithLevel(zerolog.FatalLevel)}
}

// FatalEvent mirrors zerolog's chainable API but exits via SetExitFunc instead of os.Exit directly.
type FatalEvent struct {
	e *zerolog.Event
}

func (f *FatalEvent) Err(err error) *FatalEvent {
	f.e = f.e.Err(err)
	return f
}

func (f *FatalEvent) Str(key, val string) *FatalEvent {
	f.e = f.e.Str(key, val)
	return f
}

func (f *FatalEvent) Msg(msg string) {
	f.e.Msg(msg)
	(*exitFn.Load())(1)
}

func (f *FatalEvent) Msgf(format string, v ...any) {
	f.e.Msgf(format, v...)
	(*exitFn.Load())(1)
}

// setLoggerForTest replaces the global logger (tests only).
func setLoggerForTest(l zerolog.Logger) {
	logger.Store(&l)
}
