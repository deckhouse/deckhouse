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
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"reflect"
	"sync"
	"time"

	"github.com/gookit/color"
	"github.com/werf/logboek"
	"github.com/werf/logboek/pkg/level"
	"github.com/werf/logboek/pkg/types"
	"k8s.io/klog/v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/pkg/log"
)

var (
	defaultLogger Logger
	emptyLogger   Logger = &SilentLogger{}
)

func init() {
	defaultLogger = &DummyLogger{}
}

type LoggerOptions struct {
	OutStream   io.Writer
	Width       int
	IsDebug     bool
	DebugStream io.Writer
}

type debugLogWriter struct {
	DebugStream io.Writer
}

type klogWriterWrapper struct {
	logger Logger
}

func newKlogWriterWrapper(logger Logger) *klogWriterWrapper {
	return &klogWriterWrapper{logger: logger}
}

func (l *klogWriterWrapper) Write(p []byte) (n int, err error) {
	l.logger.LogDebugF("klog: %s", string(p))

	return len(p), nil
}

func InitLogger(loggerType string) {
	InitLoggerWithOptions(loggerType, LoggerOptions{IsDebug: app.IsDebug})
}

func WrapLoggerWithTeeLogger(writer io.WriteCloser, bufSize int) error {
	previousLogger := defaultLogger
	var err error
	defaultLogger, err = NewTeeLogger(defaultLogger, writer, bufSize)
	if err != nil {
		defaultLogger = previousLogger
		return err
	}

	return nil
}

func initKlog(logger Logger) {
	// we always init klog with maximal log level because we use wrapper for klog output which
	// redirects all output to our logger and our logger doing all "perfect"
	// (logs will out in standalone installer and dhctl-server)
	flags := &flag.FlagSet{}
	klog.InitFlags(flags)
	klog.SetLogFilter(&LogSanitizer{}) // filter sensitive keywords
	flags.Set("logtostderr", "false")
	flags.Set("v", "10")

	klog.SetOutput(newKlogWriterWrapper(logger))
}

func InitLoggerWithOptions(loggerType string, opts LoggerOptions) {
	l := defaultLogger
	switch loggerType {
	case "pretty":
		l = NewPrettyLogger(opts)
	// todo: add simple logger when our slog implementation will be support not only json formatter
	// case "simple":
	// 	defaultLogger = NewSimpleLogger(opts)
	case "json":
		l = NewJSONLogger(opts)
	case "silent":
		l = emptyLogger
	default:
		panic("unknown logger type: " + app.LoggerType)
	}

	defaultLogger = l

	initKlog(l)

	// Mute Shell-Operator logs
	log.Default().SetLevel(log.LevelFatal)
	if opts.IsDebug {
		// Enable shell-operator log, because it captures klog output
		// todo: capture output of klog with default logger instead
		log.Default().SetLevel(log.LevelDebug)
		// Wrap them with our default logger
		log.Default().SetOutput(defaultLogger)
	}
}

func WrapWithTeeLogger(writer io.WriteCloser, bufSize int) error {
	l, err := NewTeeLogger(defaultLogger, writer, bufSize)
	if err != nil {
		return err
	}

	defaultLogger = l

	initKlog(l)

	return nil
}

type ProcessLogger interface {
	LogProcessStart(name string)
	LogProcessFail()
	LogProcessEnd()
}

type Logger interface {
	FlushAndClose() error

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
	NewSilentLogger() *SilentLogger

	CreateBufferLogger(buffer *bytes.Buffer) Logger

	Write([]byte) (int, error)
}

var (
	_ Logger    = &TeeLogger{}
	_ Logger    = &PrettyLogger{}
	_ Logger    = &SimpleLogger{}
	_ Logger    = &DummyLogger{}
	_ Logger    = &SilentLogger{}
	_ io.Writer = &PrettyLogger{}
	_ io.Writer = &SimpleLogger{}
	_ io.Writer = &DummyLogger{}
	_ io.Writer = &SilentLogger{}
	_ io.Writer = &TeeLogger{}
)

type styleEntry struct {
	title         string
	optionsSetter func(opts types.LogProcessOptionsInterface)
}

type PrettyLogger struct {
	processTitles  map[string]styleEntry
	isDebug        bool
	logboekLogger  types.LoggerInterface
	debugLogWriter *debugLogWriter
}

func NewPrettyLogger(opts LoggerOptions) *PrettyLogger {
	res := &PrettyLogger{
		processTitles: map[string]styleEntry{
			"common":           {"🎈 ~ Common: %s", CommonOptions},
			"infrastructure":   {"🌱 ~ Infrastructure: %s", InfrastructureOptions},
			"converge":         {"🛸 ~ Converge: %s", ConvergeOptions},
			"bootstrap":        {"⛵ ~ Bootstrap: %s", BootstrapOptions},
			"mirror":           {"🪞 ~ Mirror: %s", MirrorOptions},
			"commander/attach": {"⚓ ~ Attach to commander: %s", CommanderAttachOptions},
			"commander/detach": {"🚢 ~ Detach from commander: %s", CommanderDetachOptions},
			"default":          {"%s", BoldOptions},
		},
		isDebug: opts.IsDebug,
	}

	if opts.OutStream != nil {
		res.logboekLogger = logboek.DefaultLogger().NewSubLogger(opts.OutStream, opts.OutStream)
	} else {
		res.logboekLogger = logboek.DefaultLogger()
	}

	if opts.DebugStream != nil && !reflect.ValueOf(opts.DebugStream).IsNil() {
		res.debugLogWriter = &debugLogWriter{DebugStream: opts.DebugStream}
	}

	res.logboekLogger.SetAcceptedLevel(level.Info)

	if opts.Width != 0 {
		res.logboekLogger.Streams().SetWidth(opts.Width)
	} else {
		res.logboekLogger.Streams().SetWidth(140)
	}

	if opts.IsDebug {
		res.logboekLogger.Streams().DisableProxyStreamDataFormatting()
	} else {
		res.logboekLogger.Streams().EnableProxyStreamDataFormatting()
	}

	return res
}

func (d *PrettyLogger) FlushAndClose() error {
	return nil
}

func (d *PrettyLogger) ProcessLogger() ProcessLogger {
	return newPrettyProcessLogger(d.logboekLogger)
}

func (d *PrettyLogger) NewSilentLogger() *SilentLogger {
	return &SilentLogger{}
}

func (d *PrettyLogger) LogProcess(p, t string, run func() error) error {
	format, ok := d.processTitles[p]
	if !ok {
		format = d.processTitles["default"]
	}
	return d.logboekLogger.LogProcess(format.title, t).Options(format.optionsSetter).DoError(run)
}

func (d *PrettyLogger) LogInfoF(format string, a ...interface{}) {
	d.logboekLogger.Info().LogF(format, a...)
}

func (d *PrettyLogger) LogInfoLn(a ...interface{}) {
	d.logboekLogger.Info().LogLn(a...)
}

func (d *PrettyLogger) LogErrorF(format string, a ...interface{}) {
	d.logboekLogger.Error().LogF(format, a...)
}

func (d *PrettyLogger) LogErrorLn(a ...interface{}) {
	d.logboekLogger.Error().LogLn(a...)
}

func (d *PrettyLogger) LogDebugF(format string, a ...interface{}) {
	if d.debugLogWriter != nil {
		o := fmt.Sprintf(format, a...)
		_, err := d.debugLogWriter.DebugStream.Write([]byte(o))
		if err != nil {
			fmt.Fprintf(os.Stderr, "cannot write debug log (%s): %v", o, err)
		}
	}

	if d.isDebug {
		d.logboekLogger.Info().LogF(format, a...)
	}
}

func (d *PrettyLogger) LogDebugLn(a ...interface{}) {
	if d.debugLogWriter != nil {
		o := fmt.Sprintln(a...)
		_, err := d.debugLogWriter.DebugStream.Write([]byte(o))
		if err != nil {
			d.logboekLogger.Info().LogF("cannot write debug log (%s): %v", o, err)
		}
	}

	if d.isDebug {
		d.logboekLogger.Info().LogLn(a...)
	}
}

func (d *PrettyLogger) LogSuccess(l string) {
	d.LogInfoF("🎉 %s", l)
}

func (d *PrettyLogger) LogFail(l string) {
	d.LogInfoF("️⛱️️ %s", l)
}

func (d *PrettyLogger) LogFailRetry(l string) {
	d.LogFail(l)
}

func (d *PrettyLogger) LogWarnLn(a ...interface{}) {
	a = append([]interface{}{"❗ ~ "}, a...)
	d.LogInfoLn(color.New(color.Bold).Sprint(a...))
}

func (d *PrettyLogger) LogWarnF(format string, a ...interface{}) {
	line := color.New(color.Bold).Sprintf("❗ ~ "+format, a...)
	d.LogInfoF(line)
}

func (d *PrettyLogger) LogJSON(content []byte) {
	d.LogInfoLn(prettyJSON(content))
}

func (d *PrettyLogger) Write(content []byte) (int, error) {
	d.LogInfoF(string(content))
	return len(content), nil
}

func (d *PrettyLogger) CreateBufferLogger(buffer *bytes.Buffer) Logger {
	return NewPrettyLogger(LoggerOptions{OutStream: buffer})
}

func prettyJSON(content []byte) string {
	result := &bytes.Buffer{}
	if err := json.Indent(result, content, "", "  "); err != nil {
		panic(err)
	}

	return result.String()
}

type SimpleLogger struct {
	logger  *log.Logger
	isDebug bool
}

func NewSimpleLogger(opts LoggerOptions) *SimpleLogger {
	//todo: now unused, need change formatter to text when our slog implementation will support it
	l := log.NewLogger()

	if opts.OutStream != nil {
		l.SetOutput(opts.OutStream)
	}

	return &SimpleLogger{
		logger:  l,
		isDebug: opts.IsDebug,
	}

}

func NewJSONLogger(opts LoggerOptions) *SimpleLogger {
	//json is default formatter for our slog implementation
	l := log.NewLogger()

	if opts.OutStream != nil {
		l.SetOutput(opts.OutStream)
	}

	res := &SimpleLogger{
		logger:  l,
		isDebug: opts.IsDebug,
	}

	return res
}

func (d *SimpleLogger) CreateBufferLogger(buffer *bytes.Buffer) Logger {
	return NewJSONLogger(LoggerOptions{OutStream: buffer})
}

func (d *SimpleLogger) ProcessLogger() ProcessLogger {
	return newWrappedProcessLogger(d)
}

func (d *SimpleLogger) NewSilentLogger() *SilentLogger {
	return &SilentLogger{}
}

func (d *SimpleLogger) FlushAndClose() error {
	return nil
}

func (d *SimpleLogger) LogProcess(p, t string, run func() error) error {
	d.logger.With("action", "start").With("process", p).Info(t)
	err := run()
	d.logger.With("action", "end").With("process", p).Info(t)
	return err
}

func (d *SimpleLogger) LogInfoF(format string, a ...interface{}) {
	d.logger.Infof(format, a...)
}

func (d *SimpleLogger) LogInfoLn(a ...interface{}) {
	d.logger.Infof("%v", a)
}

func (d *SimpleLogger) LogErrorF(format string, a ...interface{}) {
	d.logger.Errorf(format, a...)
}

func (d *SimpleLogger) LogErrorLn(a ...interface{}) {
	d.logger.Errorf("%v", a)
}

func (d *SimpleLogger) LogDebugF(format string, a ...interface{}) {
	if d.isDebug {
		d.logger.Debugf(format, a...)
	}
}

func (d *SimpleLogger) LogDebugLn(a ...interface{}) {
	if d.isDebug {
		d.logger.Debugf("%v", a)
	}
}

func (d *SimpleLogger) LogSuccess(l string) {
	d.logger.With("status", "SUCCESS").Info(l)
}

func (d *SimpleLogger) LogFail(l string) {
	d.logger.With("status", "FAIL").Error(l)
}

func (d *SimpleLogger) LogFailRetry(l string) {
	// there used warn log level because in retry cycle we don't want to catch stacktraces which exist as default in Error and Fatal log level of slog logger
	d.logger.With("status", "FAIL").Warn(l)
}

func (d *SimpleLogger) LogWarnF(format string, a ...interface{}) {
	d.logger.Warnf(format, a...)
}

func (d *SimpleLogger) LogWarnLn(a ...interface{}) {
	d.logger.Warnf("%v", a)
}

func (d *SimpleLogger) LogJSON(content []byte) {
	d.logger.Info(string(content))
}

func (d *SimpleLogger) Write(content []byte) (int, error) {
	d.logger.Infof("%s", string(content))
	return len(content), nil
}

type DummyLogger struct{}

func (d *DummyLogger) ProcessLogger() ProcessLogger {
	return newWrappedProcessLogger(d)
}

func (d *DummyLogger) NewSilentLogger() *SilentLogger {
	return &SilentLogger{}
}

func (d *DummyLogger) CreateBufferLogger(buffer *bytes.Buffer) Logger {
	return NewSimpleLogger(LoggerOptions{OutStream: buffer})
}

func (d *DummyLogger) FlushAndClose() error {
	return nil
}

func (d *DummyLogger) LogProcess(_, t string, run func() error) error {
	fmt.Println(t)
	err := run()
	fmt.Println(t)
	return err
}

func (d *DummyLogger) LogInfoF(format string, a ...interface{}) {
	fmt.Printf(format, a...)
}

func (d *DummyLogger) LogInfoLn(a ...interface{}) {
	fmt.Println(a...)
}

func (d *DummyLogger) LogErrorF(format string, a ...interface{}) {
	fmt.Printf(format, a...)
}

func (d *DummyLogger) LogErrorLn(a ...interface{}) {
	fmt.Println(a...)
}

func (d *DummyLogger) LogDebugF(format string, a ...interface{}) {
	if app.IsDebug {
		fmt.Printf(format, a...)
	}
}

func (d *DummyLogger) LogDebugLn(a ...interface{}) {
	if app.IsDebug {
		fmt.Println(a...)
	}
}

func (d *DummyLogger) LogSuccess(l string) {
	fmt.Println(l)
}

func (d *DummyLogger) LogFail(l string) {
	fmt.Println(l)
}

func (d *DummyLogger) LogFailRetry(l string) {
	d.LogFail(l)
}

func (d *DummyLogger) LogWarnLn(a ...interface{}) {
	fmt.Println(a...)
}

func (d *DummyLogger) LogWarnF(format string, a ...interface{}) {
	fmt.Printf(format, a...)
}

func (d *DummyLogger) LogJSON(content []byte) {
	fmt.Println(string(content))
}

func (d *DummyLogger) Write(content []byte) (int, error) {
	fmt.Print(string(content))
	return len(content), nil
}

func FlushAndClose() error {
	return defaultLogger.FlushAndClose()
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

func GetSilentLogger() Logger {
	switch defaultLogger.(type) {
	default:
		return emptyLogger
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
