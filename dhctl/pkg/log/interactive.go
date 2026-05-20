// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
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

	external "github.com/deckhouse/lib-dhctl/pkg/log"

	"github.com/deckhouse/deckhouse/pkg/log"
)

type InteractiveProcessLogger struct {
	logger      external.ProcessLogger
	interactive bool

	phaseChan chan string
	pbStarted bool
}

func (i *InteractiveProcessLogger) LogProcessStart(name string) {
	if !i.interactive {
		i.logger.ProcessStart(name)
	} else {
		if i.pbStarted {
			i.phaseChan <- name
		}
	}
}

func (i *InteractiveProcessLogger) ProcessStart(name string) {
	if !i.interactive {
		i.logger.ProcessStart(name)
	} else {
		if i.pbStarted {
			i.phaseChan <- name
		}
	}
}

func (i *InteractiveProcessLogger) LogProcessEnd() {
	if !i.interactive {
		i.logger.ProcessEnd()
	}
}

func (i *InteractiveProcessLogger) ProcessEnd() {
	if !i.interactive {
		i.logger.ProcessEnd()
	}
}

func (i *InteractiveProcessLogger) LogProcessFail() {
	if !i.interactive {
		i.logger.ProcessFail()
	}
}

func (i *InteractiveProcessLogger) ProcessFail() {
	if !i.interactive {
		i.logger.ProcessFail()
	}
}

func newInteractiveProcessLogger(logger external.ProcessLogger, interactive bool, phaseChan chan string, pbStarted bool) *InteractiveProcessLogger {
	return &InteractiveProcessLogger{logger: logger, interactive: interactive, phaseChan: phaseChan, pbStarted: pbStarted}
}

type InteractiveLogger struct {
	logger      external.Logger
	interactive bool

	// channels for updating labels
	phaseChan chan string

	pbStarted bool
}

func newInteractiveLogger(logger external.Logger, interactive bool) *InteractiveLogger {
	// buffered chan to make sure we won't get stucked
	phaseChan := make(chan string, 5)
	return &InteractiveLogger{
		logger:      logger,
		interactive: interactive,
		phaseChan:   phaseChan,
	}
}

func (i *InteractiveLogger) GetLogger() external.Logger {
	return i.logger
}

func (i *InteractiveLogger) GetLoggerProvider() external.LoggerProvider {
	return external.SimpleLoggerProvider(i.logger)
}

func (i *InteractiveLogger) ProcessLogger() ProcessLogger {
	return newInteractiveProcessLogger(i.logger.ProcessLogger(), i.interactive, i.phaseChan, i.pbStarted)
}

func (i *InteractiveLogger) NewSilentLogger() Logger {
	return newExternalLogger(i.logger.SilentLogger())
}

func (i *InteractiveLogger) CreateBufferLogger(buffer *bytes.Buffer) Logger {
	return &ExternalLogger{logger: i.logger.BufferLogger(buffer)}
}

func (i *InteractiveLogger) FlushAndClose() error {
	return i.logger.FlushAndClose()
}

// todo: refactor in lib-dhctl too
func (i *InteractiveLogger) LogProcessCtx(ctx context.Context, p, t string, run func(ctx context.Context) error) error {
	if !i.interactive {
		return i.logger.Process(external.Process(p), t, func() error {
			return run(ctx)
		})
	}

	if i.pbStarted {
		i.phaseChan <- t
	}

	if err := run(ctx); err != nil {
		return err
	}

	return nil
}

func (i *InteractiveLogger) LogProcess(p, t string, run func() error) error {
	if !i.interactive {
		return i.logger.Process(external.Process(p), t, run)
	}

	if i.pbStarted {
		i.phaseChan <- t
	}

	if err := run(); err != nil {
		return err
	}

	return nil
}

func (i *InteractiveLogger) LogInfoF(format string, a ...interface{}) {
	if i.interactive {
		i.logger.DebugF(format, a...)
	} else {
		i.logger.InfoFWithoutLn(format, a...)
	}
}

func (i *InteractiveLogger) LogInfoLn(a ...interface{}) {
	if i.interactive {
		i.logger.DebugLn(a...)
	} else {
		i.logger.InfoLn(a...)
	}
}

func (i *InteractiveLogger) LogErrorF(format string, a ...interface{}) {
	i.logger.ErrorF(format, a...)
}

func (i *InteractiveLogger) LogErrorLn(a ...interface{}) {
	i.logger.ErrorF("%v", a...)
}

func (i *InteractiveLogger) LogDebugF(format string, a ...interface{}) {
	i.logger.DebugF(format, a...)
}

func (i *InteractiveLogger) LogDebugLn(a ...interface{}) {
	i.logger.DebugF("%v", a...)
}

func (i *InteractiveLogger) LogSuccess(l string) {
	if !i.interactive {
		i.logger.Success(l)
	}
}

func (i *InteractiveLogger) LogFail(l string) {
	if !i.interactive {
		i.logger.Fail(l)
	}
}

func (i *InteractiveLogger) LogFailRetry(l string) {
	if !i.interactive {
		i.logger.FailRetry(l)
	}
}

func (i *InteractiveLogger) LogWarnLn(a ...interface{}) {
	if !i.interactive {
		i.logger.WarnF("%s", a...)
	} else {
		i.logger.DebugF("%v", a...)
	}
}

func (i *InteractiveLogger) LogWarnF(format string, a ...interface{}) {
	if !i.interactive {
		i.logger.WarnFWithoutLn(format, a...)
	} else {
		i.logger.DebugF(format, a...)
	}
}

func (i *InteractiveLogger) LogJSON(content []byte) {
	i.logger.JSON(content)
}

func (i *InteractiveLogger) Write(content []byte) (int, error) {
	return i.logger.Write(content)
}

func (i *InteractiveLogger) GetPhaseChan() chan string {
	return i.phaseChan
}

func getInteractiveLoggerWrapper(loggerType string, opts LoggerOptions, interactive bool) (*InteractiveLogger, error) {
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

	l := newInteractiveLogger(extLogger, interactive)

	err = initInteractiveKlog(l)
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

func initInteractiveKlog(logger *InteractiveLogger) error {
	sanitizer := external.NewKeywordSanitizer().WithAdditionalKeywords(sensitiveKeywords)
	err := external.InitKlog(logger.logger, external.WithKlogSanitizer(sanitizer))
	if err != nil {
		return err
	}

	return nil
}

func InteractiveInfoLn(a ...any) {
	provider := ExternalLoggerProvider(defaultLogger)
	logger := provider()
	l, ok := logger.(*InteractiveLoggerWrapper)
	if ok {
		logger = l.logger
	}

	logger.InfoLn(a...)
}

func InteractiveWarnLn(a ...any) {
	provider := ExternalLoggerProvider(defaultLogger)
	logger := provider()
	l, ok := logger.(*InteractiveLoggerWrapper)
	if ok {
		logger = l.logger
	}

	logger.WarnLn(a...)
}

func InteractiveInfoF(format string, a ...any) {
	provider := ExternalLoggerProvider(defaultLogger)
	logger := provider()
	l, ok := logger.(*InteractiveLoggerWrapper)
	if ok {
		logger = l.logger
	}

	logger.InfoF(format, a...)
}

//
// logger from lib wrapping, to be used in lib-connection
//

type InteractiveLoggerWrapper struct {
	logger      external.Logger
	interactive bool

	phaseChan chan string
}

func (i *InteractiveLoggerWrapper) Process(p external.Process, t string, run func() error) error {
	if !i.interactive {
		return i.logger.Process(p, t, run)
	}

	if isPbStarted() {
		i.phaseChan <- t
	}

	if err := run(); err != nil {
		return err
	}

	return nil
}

func (i *InteractiveLoggerWrapper) InfoFWithoutLn(format string, a ...interface{}) {
	if i.interactive {
		i.logger.DebugFWithoutLn(format, a...)
	} else {
		i.logger.InfoFWithoutLn(format, a...)
	}
}

func (i *InteractiveLoggerWrapper) InfoLn(a ...interface{}) {
	if i.interactive {
		i.logger.DebugLn(a...)
	} else {
		i.logger.InfoLn(a...)
	}
}

func (i *InteractiveLoggerWrapper) InfoF(format string, a ...interface{}) {
	if i.interactive {
		i.logger.DebugF(format, a...)
	} else {
		i.logger.InfoF(format, a...)
	}
}

func (i *InteractiveLoggerWrapper) ErrorFWithoutLn(format string, a ...interface{}) {
	if i.interactive {
		i.logger.DebugFWithoutLn(format, a...)
	} else {
		i.logger.ErrorFWithoutLn(format, a...)
	}
}

func (i *InteractiveLoggerWrapper) ErrorLn(a ...interface{}) {
	if i.interactive {
		i.logger.DebugLn(a...)
	} else {
		i.logger.ErrorLn(a...)
	}
}

func (i *InteractiveLoggerWrapper) ErrorF(format string, a ...interface{}) {
	if i.interactive {
		i.logger.DebugF(format, a...)
	} else {
		i.logger.ErrorF(format, a...)
	}
}

func (i *InteractiveLoggerWrapper) DebugFWithoutLn(format string, a ...interface{}) {
	i.logger.DebugFWithoutLn(format, a...)
}

func (i *InteractiveLoggerWrapper) DebugLn(a ...interface{}) {
	i.logger.DebugLn(a...)
}

func (i *InteractiveLoggerWrapper) DebugF(format string, a ...interface{}) {
	i.logger.DebugF(format, a...)
}

func (i *InteractiveLoggerWrapper) WarnFWithoutLn(format string, a ...interface{}) {
	if !i.interactive {
		i.logger.WarnFWithoutLn(format, a...)
	} else {
		i.logger.DebugFWithoutLn(format, a...)
	}

}

func (i *InteractiveLoggerWrapper) WarnLn(a ...interface{}) {
	if !i.interactive {
		i.logger.WarnLn(a...)
	} else {
		i.logger.DebugLn(a...)
	}
}

func (i *InteractiveLoggerWrapper) WarnF(format string, a ...interface{}) {
	if !i.interactive {
		i.logger.WarnF(format, a...)
	} else {
		i.logger.DebugF(format, a...)
	}
}

func (i *InteractiveLoggerWrapper) Success(l string) {
	if !i.interactive {
		i.logger.Success(l)
	}
}

func (i *InteractiveLoggerWrapper) Fail(l string) {
	if !i.interactive {
		i.logger.Fail(l)
	}
}
func (i *InteractiveLoggerWrapper) FailRetry(l string) {
	if !i.interactive {
		i.logger.FailRetry(l)
	}
}

func (i *InteractiveLoggerWrapper) JSON(b []byte) {
	i.logger.JSON(b)
}

func (i *InteractiveLoggerWrapper) SilentLogger() *external.SilentLogger {
	return i.logger.SilentLogger()
}

func (i *InteractiveLoggerWrapper) BufferLogger(buffer *bytes.Buffer) external.Logger {
	return &InteractiveLoggerWrapper{logger: i.logger.BufferLogger(buffer)}
}

func (i *InteractiveLoggerWrapper) FlushAndClose() error {
	return i.logger.FlushAndClose()
}

func (i *InteractiveLoggerWrapper) ProcessLogger() external.ProcessLogger {
	return newInteractiveProcessLogger(i.logger.ProcessLogger(), i.interactive, i.phaseChan, isPbStarted())
}

func (i *InteractiveLoggerWrapper) Write(content []byte) (int, error) {
	return i.logger.Write(content)
}

func NonInteractiveLoggerProvider() external.LoggerProvider {
	logger := GetDefaultLogger()
	l, ok := logger.(*InteractiveLogger)
	if ok {
		logger = &ExternalLogger{logger: l.logger}
	}

	return ExternalLoggerProvider(logger)
}

func isPbStarted() bool {
	started := false
	logger := GetDefaultLogger()
	l, ok := logger.(*InteractiveLogger)
	if ok {
		started = l.pbStarted
	}

	return started
}

func WithProgressBar() {
	logger := GetDefaultLogger()
	l, ok := logger.(*InteractiveLogger)
	if ok {
		l.pbStarted = true
	}
}
