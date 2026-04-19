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
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"time"

	"github.com/DataDog/gostackparse"

	logContext "github.com/deckhouse/deckhouse/pkg/log/context"
)

const KeyComponent = "component"

type logger = slog.Logger

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

	addSource *atomic.Bool
	level     *slog.LevelVar
	name      string

	slogHandler Handler
}

type Option func(*Options)

type Options struct {
	Level       slog.Level
	Output      io.Writer
	HandlerType HandlerType
	TimeFunc    func(t time.Time) time.Time
}

func WithLevel(level slog.Level) Option {
	return func(o *Options) {
		o.Level = level
	}
}

func WithOutput(output io.Writer) Option {
	return func(o *Options) {
		o.Output = output
	}
}

func WithHandlerType(handlerType HandlerType) Option {
	return func(o *Options) {
		o.HandlerType = handlerType
	}
}

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
		addSource: &atomic.Bool{},
		level:     new(slog.LevelVar),
	}

	l.SetLevel(Level(options.Level))

	binaryPath := filepath.Dir(os.Args[0])
	if strings.Contains(binaryPath, "go-build") {
		binaryPath, _ = filepath.Abs("./../")
	}

	sourceFormat := "%s:%d"
	if os.Getenv("IDEA_DEVELOPMENT") != "" {
		sourceFormat = " %s:%d "
	}

	var enc encoder
	switch options.HandlerType {
	case TextHandlerType:
		enc = textEncoder{}
	default:
		enc = jsonEncoder{}
	}

	l.slogHandler = newHandler(handlerConfig{
		output:       options.Output,
		timeFn:       options.TimeFunc,
		encoder:      enc,
		addSource:    l.addSource,
		level:        l.level,
		sourceFormat: sourceFormat,
		binaryPath:   binaryPath,
	})
	l.logger = slog.New(l.slogHandler)

	return l
}

func (l *Logger) GetLevel() Level {
	return Level(l.level.Level())
}

func (l *Logger) SetLevel(level Level) {
	l.addSource.Store(slog.Level(level) <= slog.LevelDebug)

	l.level.Set(slog.Level(level))
}

func (l *Logger) SetOutput(w io.Writer) {
	l.slogHandler.SetOutput(w)
}

func (l *Logger) Named(name string) *Logger {
	return &Logger{
		logger:    slog.New(l.Handler().(Handler).Named(name)),
		addSource: l.addSource,
		level:     l.level,
		name:      l.name,
	}
}

func (l *Logger) With(args ...any) *Logger {
	return &Logger{
		logger:    l.logger.With(args...),
		addSource: l.addSource,
		level:     l.level,
		name:      l.name,
	}
}

func (l *Logger) WithGroup(name string) *Logger {
	return &Logger{
		logger:    l.logger.WithGroup(name),
		addSource: l.addSource,
		level:     l.level,
		name:      l.name,
	}
}

func (l *Logger) Error(msg string, args ...any) {
	ctx := logContext.SetCustomKeyContext(context.Background())
	ctx = logContext.SetStackTraceContext(ctx, getStack())

	l.Log(ctx, LevelError.Level(), msg, args...)
}

func (l *Logger) ErrorContext(ctx context.Context, msg string, args ...any) {
	ctx = logContext.SetCustomKeyContext(ctx)
	ctx = logContext.SetStackTraceContext(ctx, getStack())

	l.Log(ctx, LevelError.Level(), msg, args...)
}

func (l *Logger) Fatal(msg string, args ...any) {
	ctx := logContext.SetCustomKeyContext(context.Background())
	ctx = logContext.SetStackTraceContext(ctx, getStack())

	l.Log(ctx, LevelFatal.Level(), msg, args...)

	os.Exit(1)
}

func (l *Logger) FatalContext(ctx context.Context, msg string, args ...any) {
	ctx = logContext.SetCustomKeyContext(ctx)
	ctx = logContext.SetStackTraceContext(ctx, getStack())

	l.Log(ctx, LevelFatal.Level(), msg, args...)

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
		return LevelInfo, errors.New("unknown log level: " + rawLogLevel)
	}
}

func LogLevelFromStr(rawLogLevel string) Level {
	level, _ := ParseLevel(rawLogLevel)
	return level
}
