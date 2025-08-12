// Copyright 2024 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package log

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/DataDog/gostackparse"
	logContext "github.com/deckhouse/deckhouse/pkg/log/context"
)

const KeyComponent = "component"

type logger = slog.Logger
type handlerOptions = *slog.HandlerOptions

type Handler interface {
	Enabled(context.Context, slog.Level) bool
	Handle(ctx context.Context, r slog.Record) error
	Named(name string) slog.Handler
	SetOutput(w io.Writer)
	WithAttrs(attrs []slog.Attr) slog.Handler
	WithGroup(name string) slog.Handler
}

type HandlerType int

const (
	JSONHandlerType HandlerType = iota
	TextHandlerType
)

type Logger struct {
	*logger

	addSourceVar *AddSourceVar
	level        *slog.LevelVar
	name         string

	slogHandler Handler
}

type Option func(*Options)

type Options struct {
	Level       slog.Level
	Output      io.Writer
	HandlerType HandlerType
	TimeFunc    func(t time.Time) time.Time
}

// WithLevel sets the logging level
func WithLevel(level slog.Level) Option {
	return func(o *Options) {
		o.Level = level
	}
}

// WithOutput sets the output writer
func WithOutput(output io.Writer) Option {
	return func(o *Options) {
		o.Output = output
	}
}

// WithHandlerType sets the handler type
func WithHandlerType(handlerType HandlerType) Option {
	return func(o *Options) {
		o.HandlerType = handlerType
	}
}

// WithTimeFunc sets the time function
func WithTimeFunc(timeFunc func(t time.Time) time.Time) Option {
	return func(o *Options) {
		o.TimeFunc = timeFunc
	}
}

func NewNop() *Logger {
	return NewLogger(WithOutput(io.Discard))
}

func NewLogger(opts ...Option) *Logger {
	options := Options{
		Level:       slog.LevelInfo,
		Output:      os.Stdout,
		HandlerType: JSONHandlerType,
		TimeFunc: func(t time.Time) time.Time {
			return t
		},
	}

	for _, opt := range opts {
		opt(&options)
	}

	l := &Logger{
		addSourceVar: new(AddSourceVar),
		level:        new(slog.LevelVar),
	}

	l.SetLevel(Level(options.Level))

	// getting absolute binary path
	binaryPath := filepath.Dir(os.Args[0])
	// if it's go-build temporary folder
	if strings.Contains(binaryPath, "go-build") {
		binaryPath, _ = filepath.Abs("./../")
	}

	sourceFormat := "%s:%d"
	if os.Getenv("IDEA_DEVELOPMENT") != "" {
		sourceFormat = " %s:%d "
	}

	handlerOpts := &slog.HandlerOptions{
		AddSource: true,
		Level:     l.level,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			switch a.Key {
			// skip standard fields
			case slog.LevelKey, slog.MessageKey, slog.TimeKey:
				return slog.Attr{}
			case slog.SourceKey:
				if !*l.addSourceVar.Source() {
					return slog.Attr{}
				}

				s, ok := a.Value.Any().(*slog.Source)
				if !ok {
					a.Key = "_source"

					return a
				}

				a.Value = slog.StringValue(fmt.Sprintf(sourceFormat,
					// trim all folders before project root
					// trim first '/'
					strings.TrimPrefix(s.File, binaryPath)[1:],
					s.Line,
				))
			}

			return a
		},
	}

	switch options.HandlerType {
	case JSONHandlerType:
		l.slogHandler = NewJSONHandler(options.Output, handlerOpts, options.TimeFunc)
	case TextHandlerType:
		l.slogHandler = NewTextHandler(options.Output, handlerOpts, options.TimeFunc)
	default:
		l.slogHandler = NewJSONHandler(options.Output, handlerOpts, options.TimeFunc)
	}

	l.logger = slog.New(l.slogHandler.WithAttrs(nil))

	return l
}

func (l *Logger) GetLevel() Level {
	return Level(l.level.Level())
}

func (l *Logger) SetLevel(level Level) {
	l.addSourceVar.Set(slog.Level(level) <= slog.LevelDebug)

	l.level.Set(slog.Level(level))
}

func (l *Logger) SetOutput(w io.Writer) {
	l.slogHandler.SetOutput(w)
}

func (l *Logger) Named(name string) *Logger {
	return &Logger{
		logger:       slog.New(l.Handler().(Handler).Named(name)),
		addSourceVar: l.addSourceVar,
		level:        l.level,
		name:         l.name,
	}
}

func (l *Logger) With(args ...any) *Logger {
	return &Logger{
		logger:       l.logger.With(args...),
		addSourceVar: l.addSourceVar,
		level:        l.level,
		name:         l.name,
	}
}

func (l *Logger) WithGroup(name string) *Logger {
	return &Logger{
		logger:       l.logger.WithGroup(name),
		addSourceVar: l.addSourceVar,
		level:        l.level,
		name:         l.name,
	}
}

// Deprecated: use Log instead
func (l *Logger) Logf(ctx context.Context, level Level, format string, args ...any) {
	ctx = logContext.SetCustomKeyContext(ctx)
	l.Log(ctx, level.Level(), fmt.Sprintf(format, args...))
}

// Deprecated: use Debug instead
func (l *Logger) Debugf(format string, args ...any) {
	ctx := logContext.SetCustomKeyContext(context.Background())
	l.Log(ctx, LevelDebug.Level(), fmt.Sprintf(format, args...))
}

// Deprecated: use Info instead
func (l *Logger) Infof(format string, args ...any) {
	ctx := logContext.SetCustomKeyContext(context.Background())
	l.Log(ctx, LevelInfo.Level(), fmt.Sprintf(format, args...))
}

// Deprecated: use Warn instead
func (l *Logger) Warnf(format string, args ...any) {
	ctx := logContext.SetCustomKeyContext(context.Background())
	l.Log(ctx, LevelWarn.Level(), fmt.Sprintf(format, args...))
}

func (l *Logger) Error(msg string, args ...any) {
	ctx := logContext.SetCustomKeyContext(context.Background())
	ctx = logContext.SetStackTraceContext(ctx, getStack())

	l.Log(ctx, LevelError.Level(), msg, args...)
}

// Deprecated: use Error instead
func (l *Logger) Errorf(format string, args ...any) {
	ctx := logContext.SetCustomKeyContext(context.Background())
	ctx = logContext.SetStackTraceContext(ctx, getStack())

	l.Log(ctx, LevelError.Level(), fmt.Sprintf(format, args...))
}

func (l *Logger) Fatal(msg string, args ...any) {
	ctx := logContext.SetCustomKeyContext(context.Background())
	ctx = logContext.SetStackTraceContext(ctx, getStack())

	l.Log(ctx, LevelFatal.Level(), msg, args...)

	os.Exit(1)
}

// Deprecated: use Fatal instead
func (l *Logger) Fatalf(format string, args ...any) {
	ctx := logContext.SetCustomKeyContext(context.Background())
	ctx = logContext.SetStackTraceContext(ctx, getStack())

	l.Log(ctx, LevelFatal.Level(), fmt.Sprintf(format, args...))

	os.Exit(1)
}

func getStack() string {
	stack := debug.Stack()

	goroutines, _ := gostackparse.Parse(bytes.NewReader(stack))

	buf := bytes.NewBuffer([]byte{})
	_ = json.NewEncoder(buf).Encode(goroutines)

	return strings.TrimSpace(buf.String())
}

func ParseLevel(rawLogLevel string) (Level, error) {
	switch strings.ToLower(rawLogLevel) {
	case "trace":
		return LevelTrace, nil
	case "debug":
		return LevelDebug, nil
	case "info":
		return LevelInfo, nil
	case "warn":
		return LevelWarn, nil
	case "error":
		return LevelError, nil
	case "fatal":
		return LevelFatal, nil
	default:
		return LevelInfo, errors.New("no level found")
	}
}

func LogLevelFromStr(rawLogLevel string) Level {
	switch strings.ToLower(rawLogLevel) {
	case "trace":
		return LevelTrace
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn":
		return LevelWarn
	case "error":
		return LevelError
	case "fatal":
		return LevelFatal
	default:
		return LevelInfo
	}
}
