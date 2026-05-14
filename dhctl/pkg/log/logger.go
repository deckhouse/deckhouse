// Copyright 2021 Flant JSC
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
	"io"

	external "github.com/deckhouse/lib-dhctl/pkg/log"

	"github.com/deckhouse/deckhouse/pkg/log"
)

var (
	_ Logger        = &ExternalLogger{}
	_ io.Writer     = &ExternalLogger{}
	_ ProcessLogger = &ExternalProcessLogger{}
)

var (
	defaultLogger Logger = newExternalLogger(external.NewDummyLogger(false))
	emptyLogger   Logger = newExternalLogger(external.NewSilentLogger())
	debugEnabled  bool
)

const (
	ProcessPreflight = "preflight"
)

type Logger interface {
	FlushAndClose() error

	LogProcessCtx(context.Context, string, string, func(context.Context) error) error
	LogProcess(string, string, func() error) error

	LogInfoF(format string, a ...interface{})
	LogInfoLn(a ...interface{})

	LogErrorF(format string, a ...interface{})
	LogErrorLn(a ...interface{})

	LogDebugF(format string, a ...interface{})
	LogDebugLn(a ...interface{})

	LogWarnF(format string, a ...interface{})
	LogWarnLn(a ...interface{})

	LogSuccess(string)
	LogFail(string)
	LogFailRetry(string)

	LogJSON([]byte)

	ProcessLogger() ProcessLogger
	NewSilentLogger() Logger

	CreateBufferLogger(buffer *bytes.Buffer) Logger

	Write([]byte) (int, error)
}

type ProcessLogger interface {
	LogProcessStart(name string)
	LogProcessFail()
	LogProcessEnd()
}

type LoggerOptions struct {
	OutStream   io.Writer
	Width       int
	IsDebug     bool
	DebugStream io.Writer
}

func InitLogger(loggerType string, interactive bool) error {
	return initLoggerWithOptions(
		loggerType,
		LoggerOptions{
			IsDebug: debugEnabled,
		},
		interactive,
	)
}

func SetDebugEnabled(enabled bool) {
	debugEnabled = enabled
}

func InitLoggerWithOptions(loggerType string, opts LoggerOptions, interactive bool) {
	if err := initLoggerWithOptions(loggerType, opts, interactive); err != nil {
		panic(err)
	}
}

func WrapWithTeeLogger(writer io.WriteCloser, bufSize int) error {
	var logger external.Logger

	ext, ok := defaultLogger.(*ExternalLogger)
	if ok {
		logger = ext.logger
	} else {
		i := defaultLogger.(*InteractiveLogger)
		logger = i.logger
	}

	tee, err := external.WrapWithTeeLogger(logger, writer, bufSize)
	if err != nil {
		return err
	}

	if ok {
		ext = &ExternalLogger{logger: tee}
		initExternalKlog(ext)

		defaultLogger = ext
	} else {
		i := newInteractiveLogger(tee, true)
		initInteractiveKlog(i)

		defaultLogger = i
	}

	return nil
}

func initExternalKlog(logger *ExternalLogger) error {
	sanitizer := external.NewKeywordSanitizer().WithAdditionalKeywords(sensitiveKeywords)
	err := external.InitKlog(logger.logger, external.WithKlogSanitizer(sanitizer))
	if err != nil {
		return err
	}

	return nil
}

func getExternalLoggerWrapper(loggerType string, opts LoggerOptions) (*ExternalLogger, error) {
	extOpts := external.LoggerOptions{
		OutStream:   opts.OutStream,
		Width:       opts.Width,
		IsDebug:     opts.IsDebug,
		DebugStream: opts.DebugStream,
		AdditionalProcesses: external.Processes{
			ProcessPreflight: external.StyleEntry{
				Title:         "🎈 ~ Preflight checks %s",
				OptionsSetter: CommonOptions,
			},
		},
	}

	extLogger, err := external.NewLoggerWithOptions(external.Type(loggerType), extOpts)
	if err != nil {
		return nil, err
	}

	l := &ExternalLogger{logger: extLogger}

	err = initExternalKlog(l)
	if err != nil {
		return nil, err
	}
	// Mute Shell-Operator logs
	log.Default().SetLevel(log.LevelFatal)
	if opts.IsDebug {
		// Enable shell-operator log, because it captures klog output
		// todo: capture output of klog with default logger instead
		log.Default().SetLevel(log.LevelDebug)
		// Wrap them with our default logger
		log.Default().SetOutput(defaultLogger)
	}

	return l, nil
}

func initLoggerWithOptions(loggerType string, opts LoggerOptions, interactive bool) error {
	var l Logger
	var err error
	if interactive {
		l, err = getInteractiveLoggerWrapper(loggerType, opts, interactive)
	} else {
		l, err = getExternalLoggerWrapper(loggerType, opts)
	}

	if err != nil {
		return err
	}

	defaultLogger = l

	return nil
}

type ExternalProcessLogger struct {
	logger external.ProcessLogger
}

func (e *ExternalProcessLogger) LogProcessStart(name string) {
	e.logger.ProcessStart(name)
}

func (e *ExternalProcessLogger) LogProcessEnd() {
	e.logger.ProcessEnd()
}

func (e *ExternalProcessLogger) LogProcessFail() {
	e.logger.ProcessFail()
}

func newExternalProcessLogger(logger external.ProcessLogger) *ExternalProcessLogger {
	return &ExternalProcessLogger{logger: logger}
}

type ExternalLogger struct {
	logger external.Logger
}

func newExternalLogger(logger external.Logger) *ExternalLogger {
	return &ExternalLogger{
		logger: logger,
	}
}

func (e *ExternalLogger) GetLogger() external.Logger {
	return e.logger
}

func (e *ExternalLogger) GetLoggerProvider() external.LoggerProvider {
	return external.SimpleLoggerProvider(e.logger)
}

func (e *ExternalLogger) ProcessLogger() ProcessLogger {
	return newExternalProcessLogger(e.logger.ProcessLogger())
}

func (e *ExternalLogger) NewSilentLogger() Logger {
	return newExternalLogger(e.logger.SilentLogger())
}

func (e *ExternalLogger) CreateBufferLogger(buffer *bytes.Buffer) Logger {
	return &ExternalLogger{logger: e.logger.BufferLogger(buffer)}
}

func (e *ExternalLogger) FlushAndClose() error {
	return e.logger.FlushAndClose()
}

// todo: refactor in lib-dhctl too
func (e *ExternalLogger) LogProcessCtx(ctx context.Context, p, t string, run func(ctx context.Context) error) error {
	return e.logger.Process(external.Process(p), t, func() error {
		return run(ctx)
	})
}

func (e *ExternalLogger) LogProcess(p, t string, run func() error) error {
	return e.logger.Process(external.Process(p), t, run)
}

func (e *ExternalLogger) LogInfoF(format string, a ...interface{}) {
	e.logger.InfoFWithoutLn(format, a...)
}

func (e *ExternalLogger) LogInfoLn(a ...interface{}) {
	e.logger.InfoLn(a...)
}

func (e *ExternalLogger) LogErrorF(format string, a ...interface{}) {
	e.logger.ErrorF(format, a...)
}

func (e *ExternalLogger) LogErrorLn(a ...interface{}) {
	e.logger.ErrorF("%v", a...)
}

func (e *ExternalLogger) LogDebugF(format string, a ...interface{}) {
	e.logger.DebugF(format, a...)
}

func (e *ExternalLogger) LogDebugLn(a ...interface{}) {
	e.logger.DebugF("%v", a...)
}

func (e *ExternalLogger) LogSuccess(l string) {
	e.logger.Success(l)
}

func (e *ExternalLogger) LogFail(l string) {
	e.logger.Fail(l)
}

func (e *ExternalLogger) LogFailRetry(l string) {
	e.logger.FailRetry(l)
}

func (e *ExternalLogger) LogWarnLn(a ...interface{}) {
	e.logger.WarnF("%s", a...)
}

func (e *ExternalLogger) LogWarnF(format string, a ...interface{}) {
	e.logger.WarnFWithoutLn(format, a...)
}

func (e *ExternalLogger) LogJSON(content []byte) {
	e.logger.JSON(content)
}

func (e *ExternalLogger) Write(content []byte) (int, error) {
	return e.logger.Write(content)
}

func FlushAndClose() error {
	return defaultLogger.FlushAndClose()
}

func ProcessCtx(ctx context.Context, p, t string, run func(ctx context.Context) error) error {
	return defaultLogger.LogProcessCtx(ctx, p, t, run)
}

func Process(p, t string, run func() error) error {
	return defaultLogger.LogProcess(p, t, run)
}

func InfoF(format string, a ...interface{}) {
	defaultLogger.LogInfoF(format, a...)
}

func InfoLn(a ...interface{}) {
	defaultLogger.LogInfoLn(a...)
}

func ErrorF(format string, a ...interface{}) {
	defaultLogger.LogErrorF(format, a...)
}

func ErrorLn(a ...interface{}) {
	defaultLogger.LogErrorLn(a...)
}

func DebugF(format string, a ...interface{}) {
	defaultLogger.LogDebugF(format, a...)
}

func DebugLn(a ...interface{}) {
	defaultLogger.LogDebugLn(a...)
}

func Success(l string) {
	defaultLogger.LogSuccess(l)
}

func Fail(l string) {
	defaultLogger.LogFail(l)
}

func WarnF(format string, a ...interface{}) {
	defaultLogger.LogWarnF(format, a...)
}

func WarnLn(a ...interface{}) {
	defaultLogger.LogWarnLn(a...)
}

func JSON(content []byte) {
	defaultLogger.LogJSON(content)
}

func Write(buf []byte) (int, error) {
	return defaultLogger.Write(buf)
}

func GetProcessLogger() ProcessLogger {
	return defaultLogger.ProcessLogger()
}

func GetDefaultLogger() Logger {
	return defaultLogger
}

func ExternalLoggerProvider(logger Logger) external.LoggerProvider {
	var l external.Logger
	ext, ok := logger.(*ExternalLogger)
	if ok {
		l = ext.logger
	} else {
		i := logger.(*InteractiveLogger)
		wrapper := &InteractiveLoggerWrapper{logger: i.logger, interactive: i.interactive, phaseChan: i.phaseChan}
		return external.SimpleLoggerProvider(wrapper)
	}

	return external.SimpleLoggerProvider(l)
}

func GetSilentLogger() Logger {
	var l external.Logger
	ext, ok := defaultLogger.(*ExternalLogger)
	if ok {
		l = ext.logger
	} else {
		i := defaultLogger.(*InteractiveLogger)
		l = i.logger
	}

	switch l.(type) {
	default:
		return emptyLogger
	case *external.TeeLogger:
		if ok {
			return ext.NewSilentLogger()
		}
		return defaultLogger.(*InteractiveLogger).NewSilentLogger()
	}
}
