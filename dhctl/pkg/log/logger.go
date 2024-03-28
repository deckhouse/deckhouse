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
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"k8s.io/klog"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/gookit/color"
	"github.com/sirupsen/logrus"
	"github.com/werf/logboek"
	"github.com/werf/logboek/pkg/level"
	"github.com/werf/logboek/pkg/types"
)

var (
	defaultLogger Logger
	emptyLogger   Logger = &SilentLogger{}
)

func init() {
	defaultLogger = &DummyLogger{}
}

type LoggerOptions struct {
	OutStream io.Writer
	Width     int
	IsDebug   bool
}

func InitLogger(loggerType string) {
	InitLoggerWithOptions(loggerType, LoggerOptions{IsDebug: app.IsDebug})
}

func InitLoggerWithOptions(loggerType string, opts LoggerOptions) {
	switch loggerType {
	case "pretty":
		defaultLogger = NewPrettyLogger(opts)
	case "simple":
		defaultLogger = NewSimpleLogger(opts)
	case "json":
		defaultLogger = NewJSONLogger(opts)
	case "silent":
		defaultLogger = emptyLogger
	default:
		panic("unknown logger type: " + app.LoggerType)
	}

	// Mute Shell-Operator logs
	logrus.SetLevel(logrus.PanicLevel)
	if opts.IsDebug {
		// Enable shell-operator log, because it captures klog output
		// todo: capture output of klog with default logger instead
		logrus.SetLevel(logrus.DebugLevel)
		klog.InitFlags(nil)
		_ = flag.CommandLine.Parse([]string{"-v=10"})

		// Wrap them with our default logger
		logrus.SetOutput(defaultLogger)
	}
}

type ProcessLogger interface {
	LogProcessStart(name string)
	LogProcessFail()
	LogProcessEnd()
}

type Logger interface {
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

	LogJSON([]byte)

	ProcessLogger() ProcessLogger

	Write([]byte) (int, error)
}

var (
	_ Logger    = &PrettyLogger{}
	_ Logger    = &SimpleLogger{}
	_ Logger    = &DummyLogger{}
	_ Logger    = &SilentLogger{}
	_ io.Writer = &PrettyLogger{}
	_ io.Writer = &SimpleLogger{}
	_ io.Writer = &DummyLogger{}
	_ io.Writer = &SilentLogger{}
)

type styleEntry struct {
	title         string
	optionsSetter func(opts types.LogProcessOptionsInterface)
}

type PrettyLogger struct {
	processTitles map[string]styleEntry
	isDebug       bool
	logboekLogger types.LoggerInterface
}

func NewPrettyLogger(opts LoggerOptions) *PrettyLogger {
	res := &PrettyLogger{
		processTitles: map[string]styleEntry{
			"common":    {"🎈 ~ Common: %s", CommonOptions},
			"terraform": {"🌱 ~ Terraform: %s", TerraformOptions},
			"converge":  {"🛸 ~ Converge: %s", ConvergeOptions},
			"bootstrap": {"⛵ ~ Bootstrap: %s", BootstrapOptions},
			"mirror":    {"🪞 ~ Mirror: %s", MirrorOptions},
			"default":   {"%s", BoldOptions},
		},
		isDebug: opts.IsDebug,
	}

	if opts.OutStream != nil {
		res.logboekLogger = logboek.DefaultLogger().NewSubLogger(opts.OutStream, opts.OutStream)
	} else {
		res.logboekLogger = logboek.DefaultLogger()
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

func (d *PrettyLogger) ProcessLogger() ProcessLogger {
	return newPrettyProcessLogger(d.logboekLogger)
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
	if d.isDebug {
		d.logboekLogger.Info().LogF(format, a...)
	}
}

func (d *PrettyLogger) LogDebugLn(a ...interface{}) {
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

func prettyJSON(content []byte) string {
	result := &bytes.Buffer{}
	if err := json.Indent(result, content, "", "  "); err != nil {
		panic(err)
	}

	return result.String()
}

type SimpleLogger struct {
	logger  *logrus.Entry
	isDebug bool
}

func NewSimpleLogger(opts LoggerOptions) *SimpleLogger {
	l := &logrus.Logger{
		Level: logrus.DebugLevel,
		Formatter: &logrus.TextFormatter{
			DisableColors:   true,
			TimestampFormat: "2006-01-02 15:04:05",
			FullTimestamp:   true,
		},
	}

	if opts.OutStream != nil {
		l.Out = opts.OutStream
	} else {
		l.Out = os.Stdout
	}

	// l.Formatter = &logrus.JSONFormatter{}
	return &SimpleLogger{
		logger:  logrus.NewEntry(l),
		isDebug: opts.IsDebug,
	}
}

func NewJSONLogger(opts LoggerOptions) *SimpleLogger {
	simpleLogger := NewSimpleLogger(opts)
	simpleLogger.logger.Logger.Formatter = &logrus.JSONFormatter{}

	return simpleLogger
}

func (d *SimpleLogger) ProcessLogger() ProcessLogger {
	return newWrappedProcessLogger(d)
}

func (d *SimpleLogger) LogProcess(p, t string, run func() error) error {
	d.logger.WithField("action", "start").WithField("process", p).Infoln(t)
	err := run()
	d.logger.WithField("action", "end").WithField("process", p).Infoln(t)
	return err
}

func (d *SimpleLogger) LogInfoF(format string, a ...interface{}) {
	d.logger.Infof(format, a...)
}

func (d *SimpleLogger) LogInfoLn(a ...interface{}) {
	d.logger.Infoln(a...)
}

func (d *SimpleLogger) LogErrorF(format string, a ...interface{}) {
	d.logger.Errorf(format, a...)
}

func (d *SimpleLogger) LogErrorLn(a ...interface{}) {
	d.logger.Errorln(a...)
}

func (d *SimpleLogger) LogDebugF(format string, a ...interface{}) {
	if d.isDebug {
		d.logger.Debugf(format, a...)
	}
}

func (d *SimpleLogger) LogDebugLn(a ...interface{}) {
	if d.isDebug {
		d.logger.Debugln(a...)
	}
}

func (d *SimpleLogger) LogSuccess(l string) {
	d.logger.WithField("status", "SUCCESS").Infoln(l)
}

func (d *SimpleLogger) LogFail(l string) {
	d.logger.WithField("status", "FAIL").Errorln(l)
}

func (d *SimpleLogger) LogWarnF(format string, a ...interface{}) {
	d.logger.Warnf(format, a...)
}

func (d *SimpleLogger) LogWarnLn(a ...interface{}) {
	d.logger.Warnln(a...)
}

func (d *SimpleLogger) LogJSON(content []byte) {
	d.logger.Infoln(string(content))
}

func (d *SimpleLogger) Write(content []byte) (int, error) {
	d.logger.Infof(string(content))
	return len(content), nil
}

type DummyLogger struct{}

func (d *DummyLogger) ProcessLogger() ProcessLogger {
	return newWrappedProcessLogger(d)
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
	return emptyLogger
}

type SilentLogger struct{}

func (d *SilentLogger) ProcessLogger() ProcessLogger {
	return newWrappedProcessLogger(d)
}

func (d *SilentLogger) LogProcess(_, t string, run func() error) error {
	err := run()
	return err
}

func (d *SilentLogger) LogInfoF(format string, a ...interface{}) {
}

func (d *SilentLogger) LogInfoLn(a ...interface{}) {
}

func (d *SilentLogger) LogErrorF(format string, a ...interface{}) {
}

func (d *SilentLogger) LogErrorLn(a ...interface{}) {
}

func (d *SilentLogger) LogDebugF(format string, a ...interface{}) {
}

func (d *SilentLogger) LogDebugLn(a ...interface{}) {
}

func (d *SilentLogger) LogSuccess(l string) {
}

func (d *SilentLogger) LogFail(l string) {
}

func (d *SilentLogger) LogWarnLn(a ...interface{}) {
}

func (d *SilentLogger) LogWarnF(format string, a ...interface{}) {
}

func (d *SilentLogger) LogJSON(content []byte) {
}

func (d *SilentLogger) Write(content []byte) (int, error) {
	return len(content), nil
}
