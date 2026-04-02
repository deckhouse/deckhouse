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
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	external "github.com/deckhouse/lib-dhctl/pkg/log"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
)

var (
	_ Logger        = &ExternalLogger{}
	_ io.Writer     = &ExternalLogger{}
	_ ProcessLogger = &ExternalProcessLogger{}
)

var (
	defaultLogger Logger = newExternalLogger(external.NewDummyLogger(app.IsDebug))
	emptyLogger   Logger = newExternalLogger(external.NewSilentLogger())
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

func InitLogger(loggerType string) error {
	return initLoggerWithOptions(loggerType, LoggerOptions{
		IsDebug: app.IsDebug,
	})
}

func InitLoggerWithOptions(loggerType string, opts LoggerOptions) {
	if err := initLoggerWithOptions(loggerType, opts); err != nil {
		panic(err)
	}
}

func WrapWithTeeLogger(writer io.WriteCloser, bufSize int) error {
	ext := defaultLogger.(*ExternalLogger)

	tee, err := external.WrapWithTeeLogger(ext.logger, writer, bufSize)
	if err != nil {
		return err
	}

	ext = &ExternalLogger{logger: tee}
	initExternalKlog(ext)

	defaultLogger = ext

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
	if app.IsDebug {
		// Enable shell-operator log, because it captures klog output
		// todo: capture output of klog with default logger instead
		log.Default().SetLevel(log.LevelDebug)
		// Wrap them with our default logger
		log.Default().SetOutput(defaultLogger)
	}

	return l, nil
}

func initLoggerWithOptions(loggerType string, opts LoggerOptions) error {
	l, err := getExternalLoggerWrapper(loggerType, opts)
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
	ext := logger.(*ExternalLogger)
	return external.SimpleLoggerProvider(ext.logger)
}

func GetSilentLogger() Logger {
	ext := defaultLogger.(*ExternalLogger)

	switch ext.logger.(type) {
	default:
		return emptyLogger
	case *external.TeeLogger:
		return ext.NewSilentLogger()
	case *TeeLogger:
		return defaultLogger.NewSilentLogger()
	}
}

type SilentLogger struct {
	t *TeeLogger
}

func NewSilentLogger() *SilentLogger {
	return &SilentLogger{
		t: nil,
	}
}

func (d *SilentLogger) ProcessLogger() ProcessLogger {
	return newWrappedProcessLogger(d)
}

func (d *SilentLogger) NewSilentLogger() *SilentLogger {
	return &SilentLogger{}
}

func (d *SilentLogger) CreateBufferLogger(buffer *bytes.Buffer) Logger {
	return d
}

func (d *SilentLogger) LogProcess(_, t string, run func() error) error {
	err := run()
	return err
}

func (d *SilentLogger) FlushAndClose() error {
	return nil
}

func (d *SilentLogger) LogInfoF(format string, a ...interface{}) {
	if d.t != nil {
		d.t.writeToFile(fmt.Sprintf(format, a...))
	}
}

func (d *SilentLogger) LogInfoLn(a ...interface{}) {
	if d.t != nil {
		d.t.writeToFile(fmt.Sprintln(a...))
	}
}

func (d *SilentLogger) LogErrorF(format string, a ...interface{}) {
	if d.t != nil {
		d.t.writeToFile(fmt.Sprintf(format, a...))
	}
}

func (d *SilentLogger) LogErrorLn(a ...interface{}) {
	if d.t != nil {
		d.t.writeToFile(fmt.Sprintln(a...))
	}
}

func (d *SilentLogger) LogDebugF(format string, a ...interface{}) {
	if d.t != nil {
		d.t.writeToFile(fmt.Sprintf(format, a...))
	}
}

func (d *SilentLogger) LogDebugLn(a ...interface{}) {
	if d.t != nil {
		d.t.writeToFile(fmt.Sprintln(a...))
	}
}

func (d *SilentLogger) LogSuccess(l string) {
	if d.t != nil {
		d.t.writeToFile(l)
	}
}

func (d *SilentLogger) LogFail(l string) {
	if d.t != nil {
		d.t.writeToFile(l)
	}
}

func (d *SilentLogger) LogFailRetry(l string) {
	if d.t != nil {
		d.t.writeToFile(l)
	}
}

func (d *SilentLogger) LogWarnLn(a ...interface{}) {
	if d.t != nil {
		d.t.writeToFile(fmt.Sprintln(a...))
	}
}

func (d *SilentLogger) LogWarnF(format string, a ...interface{}) {
	if d.t != nil {
		d.t.writeToFile(fmt.Sprintf(format, a...))
	}
}

func (d *SilentLogger) LogJSON(content []byte) {
	if d.t != nil {
		d.t.writeToFile(string(content))
	}
}

func (d *SilentLogger) Write(content []byte) (int, error) {
	if d.t != nil {
		d.t.writeToFile(string(content))
	}
	return len(content), nil
}

type TeeLogger struct {
	l      Logger
	closed bool

	bufMutex sync.Mutex
	buf      *bufio.Writer
	out      io.WriteCloser
}

func (d *TeeLogger) GetLogger() Logger {
	return d.l
}

func NewTeeLogger(l Logger, writer io.WriteCloser, bufferSize int) (*TeeLogger, error) {
	buf := bufio.NewWriterSize(writer, bufferSize)

	return &TeeLogger{
		l:   l,
		buf: buf,
		out: writer,
	}, nil
}

func (d *TeeLogger) CreateBufferLogger(buffer *bytes.Buffer) Logger {
	var l Logger
	switch d.l.(type) {
	case *PrettyLogger:
		l = NewPrettyLogger(LoggerOptions{OutStream: buffer})
	case *SimpleLogger:
		l = NewJSONLogger(LoggerOptions{OutStream: buffer})
	default:
		l = d.l
	}

	buf := bufio.NewWriterSize(d.out, 4096) // 1024 bytes may not be enough when executing in parallel

	return &TeeLogger{
		l:   l,
		buf: buf,
		out: d.out,
	}
}

func (d *TeeLogger) FlushAndClose() error {
	if d.closed {
		return nil
	}

	d.bufMutex.Lock()
	defer d.bufMutex.Unlock()

	err := d.buf.Flush()
	if err != nil {
		d.l.LogWarnF("Cannot flush TeeLogger: %v \n", err)
		return err
	}

	d.buf = nil

	err = d.out.Close()
	if err != nil {
		d.l.LogWarnF("Cannot close TeeLogger file: %v \n", err)
		return err
	}

	d.closed = true
	return nil
}

func (d *TeeLogger) ProcessLogger() ProcessLogger {
	return d.l.ProcessLogger()
}

func (d *TeeLogger) NewSilentLogger() *SilentLogger {
	return &SilentLogger{
		t: d,
	}
}

func (d *TeeLogger) LogProcess(msg, t string, run func() error) error {
	d.writeToFile(fmt.Sprintf("Start process %s\n", t))

	err := d.l.LogProcess(msg, t, run)

	d.writeToFile(fmt.Sprintf("End process %s\n", t))

	return err
}

func (d *TeeLogger) LogInfoF(format string, a ...interface{}) {
	d.l.LogInfoF(format, a...)

	d.writeToFile(fmt.Sprintf(format, a...))
}

func (d *TeeLogger) LogInfoLn(a ...interface{}) {
	d.l.LogInfoLn(a...)

	d.writeToFile(fmt.Sprintln(a...))
}

func (d *TeeLogger) LogErrorF(format string, a ...interface{}) {
	d.l.LogErrorF(format, a...)

	d.writeToFile(fmt.Sprintf(format, a...))
}

func (d *TeeLogger) LogErrorLn(a ...interface{}) {
	d.l.LogErrorLn(a...)

	d.writeToFile(fmt.Sprintln(a...))
}

func (d *TeeLogger) LogDebugF(format string, a ...interface{}) {
	d.l.LogDebugF(format, a...)

	d.writeToFile(fmt.Sprintf(format, a...))
}

func (d *TeeLogger) LogDebugLn(a ...interface{}) {
	d.l.LogDebugLn(a...)

	d.writeToFile(fmt.Sprintln(a...))
}

func (d *TeeLogger) LogSuccess(l string) {
	d.l.LogSuccess(l)

	d.writeToFile(l)
}

func (d *TeeLogger) LogFail(l string) {
	d.l.LogFail(l)

	d.writeToFile(l)
}

func (d *TeeLogger) LogFailRetry(l string) {
	d.l.LogFailRetry(l)

	d.writeToFile(l)
}

func (d *TeeLogger) LogWarnLn(a ...interface{}) {
	d.l.LogWarnLn(a...)

	d.writeToFile(fmt.Sprintln(a...))
}

func (d *TeeLogger) LogWarnF(format string, a ...interface{}) {
	d.l.LogWarnF(format, a...)

	d.writeToFile(fmt.Sprintf(format, a...))
}

func (d *TeeLogger) LogJSON(content []byte) {
	d.l.LogJSON(content)

	d.writeToFile(string(content))
}

func (d *TeeLogger) Write(content []byte) (int, error) {
	ln, err := d.l.Write(content)
	if err != nil {
		d.l.LogDebugF("Cannot write to log: %v", err)
	}

	d.writeToFile(string(content))

	return ln, err
}

func (d *TeeLogger) writeToFile(content string) {
	if d.closed {
		return
	}

	d.bufMutex.Lock()
	defer d.bufMutex.Unlock()

	if d.buf == nil {
		return
	}

	timestamp := time.Now().Format(time.DateTime)
	contentWithTimestamp := fmt.Sprintf("%s - %s", timestamp, content)

	if _, err := d.buf.Write([]byte(contentWithTimestamp)); err != nil {
		d.l.LogDebugF("Cannot write to TeeLog: %v", err)
	}
}
