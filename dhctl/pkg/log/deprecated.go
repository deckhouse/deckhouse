// Copyright 2026 Flant JSC
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
	"fmt"
	"io"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	external "github.com/deckhouse/lib-dhctl/pkg/log"
)

// WARNING! This loggers is deprecated and saved
// only for backward compatibility because
// this loggers can used in d8 utility and another

var (
	_ Logger          = &TeeLogger{}
	_ Logger          = &PrettyLogger{}
	_ Logger          = &SimpleLogger{}
	_ Logger          = &DummyLogger{}
	_ Logger          = &SilentLogger{}
	_ io.Writer       = &PrettyLogger{}
	_ io.Writer       = &SimpleLogger{}
	_ io.Writer       = &DummyLogger{}
	_ io.Writer       = &SilentLogger{}
	_ io.Writer       = &TeeLogger{}
	_ external.Logger = &externalDummyLoggerWrapper{}
)

type PrettyLogger struct {
	*ExternalLogger
}

func NewPrettyLogger(opts LoggerOptions) *PrettyLogger {
	l, err := getExternalLoggerWrapper(string(external.Pretty), opts)
	if err != nil {
		panic(err)
	}

	return &PrettyLogger{
		ExternalLogger: l,
	}
}

type SimpleLogger struct {
	*ExternalLogger
}

func NewSimpleLogger(opts LoggerOptions) *SimpleLogger {
	l, err := getExternalLoggerWrapper(string(external.Simple), opts)
	if err != nil {
		panic(err)
	}

	return &SimpleLogger{
		ExternalLogger: l,
	}
}

func NewJSONLogger(opts LoggerOptions) *SimpleLogger {
	return NewSimpleLogger(opts)
}

type SilentLogger struct {
	*ExternalLogger
}

func NewSilentLogger() *SilentLogger {
	return &SilentLogger{
		ExternalLogger: newExternalLogger(external.NewSilentLogger()),
	}
}

type TeeLogger struct {
	*ExternalLogger
}

func NewTeeLogger(l Logger, writer io.WriteCloser, bufferSize int) (*TeeLogger, error) {
	ext := extractExternalLogger(l)
	tee, err := external.NewTeeLogger(ext.logger, writer, bufferSize)
	if err != nil {
		return nil, err
	}

	return &TeeLogger{
		ExternalLogger: newExternalLogger(tee),
	}, nil
}

type DummyLogger struct{}

func (d *DummyLogger) ProcessLogger() ProcessLogger {
	return newWrappedProcessLogger(d)
}

func (d *DummyLogger) NewSilentLogger() Logger {
	return newExternalLogger(external.NewSilentLogger())
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

type externalDummyLoggerWrapper struct {
	parent *DummyLogger
}

func newExternalDummyLoggerWrapper(parent *DummyLogger) *externalDummyLoggerWrapper {
	return &externalDummyLoggerWrapper{
		parent: parent,
	}
}

func (w *externalDummyLoggerWrapper) BufferLogger(buffer *bytes.Buffer) external.Logger {
	return external.NewSimpleLogger(external.LoggerOptions{
		IsDebug:   app.IsDebug,
		OutStream: buffer,
	})
}

func (w *externalDummyLoggerWrapper) SilentLogger() *external.SilentLogger {
	return external.NewSilentLogger()
}

func (w *externalDummyLoggerWrapper) FlushAndClose() error {
	return nil
}

func (w *externalDummyLoggerWrapper) Process(p external.Process, title string, action func() error) error {
	return w.parent.LogProcess(string(p), title, action)
}

func (w *externalDummyLoggerWrapper) InfoFWithoutLn(format string, a ...interface{}) {
	w.parent.LogInfoF(format, a...)
}

func (w *externalDummyLoggerWrapper) InfoLn(a ...interface{}) {
	w.parent.LogInfoLn(a...)
}

func (w *externalDummyLoggerWrapper) ErrorFWithoutLn(format string, a ...interface{}) {
	w.parent.LogErrorF(format, a...)
}

func (w *externalDummyLoggerWrapper) ErrorLn(a ...interface{}) {
	w.parent.LogErrorLn(a...)
}

func (w *externalDummyLoggerWrapper) DebugFWithoutLn(format string, a ...interface{}) {
	w.parent.LogDebugF(format, a...)
}

func (w *externalDummyLoggerWrapper) DebugLn(a ...interface{}) {
	w.parent.LogDebugLn(a...)
}

func (w *externalDummyLoggerWrapper) WarnFWithoutLn(format string, a ...interface{}) {
	w.parent.LogWarnF(format, a...)
}

func (w *externalDummyLoggerWrapper) WarnLn(a ...interface{}) {
	w.parent.LogWarnLn(a...)
}

func (w *externalDummyLoggerWrapper) Success(t string) {
	w.parent.LogSuccess(t)
}

func (w *externalDummyLoggerWrapper) Fail(t string) {
	w.parent.LogFail(t)
}

func (w *externalDummyLoggerWrapper) FailRetry(t string) {
	w.parent.LogFailRetry(t)
}

func (w *externalDummyLoggerWrapper) JSON(c []byte) {
	w.parent.LogJSON(c)
}

func (w *externalDummyLoggerWrapper) Write(c []byte) (int, error) {
	return w.parent.Write(c)
}

func (w *externalDummyLoggerWrapper) ProcessLogger() external.ProcessLogger {
	return newWrappedProcessLogger(w.parent)
}

func (w *externalDummyLoggerWrapper) InfoF(format string, a ...any) {
	w.parent.LogInfoF(format+"\n", a...)
}

func (w *externalDummyLoggerWrapper) ErrorF(format string, a ...any) {
	w.parent.LogErrorF(format+"\n", a...)
}

func (w *externalDummyLoggerWrapper) DebugF(format string, a ...any) {
	w.parent.LogDebugF(format+"\n", a...)
}

func (w *externalDummyLoggerWrapper) WarnF(format string, a ...any) {
	w.parent.LogWarnF(format+"\n", a...)
}

func extractExternalLogger(l Logger) *ExternalLogger {
	var ext *ExternalLogger
	switch typedLogger := l.(type) {
	case *ExternalLogger:
		ext = typedLogger
	case *TeeLogger:
		ext = typedLogger.ExternalLogger
	case *PrettyLogger:
		ext = typedLogger.ExternalLogger
	case *SimpleLogger:
		ext = typedLogger.ExternalLogger
	case *SilentLogger:
		ext = typedLogger.ExternalLogger
	case *DummyLogger:
		ext = newExternalLogger(newExternalDummyLoggerWrapper(typedLogger))
	default:
		panic(fmt.Errorf("Incorrect type %T for extract ExternalLogger", l))
	}

	return ext
}
