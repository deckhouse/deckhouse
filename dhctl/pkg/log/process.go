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
	"fmt"
	"time"

	"github.com/flant/logboek"
)

type prettyProcessLogger struct{}

func newPrettyProcessLogger() *prettyProcessLogger {
	return &prettyProcessLogger{}
}

func (l *prettyProcessLogger) LogProcessStart(msg string) {
	logboek.LogProcessStart(msg, BoldStartOptions())
}

func (l *prettyProcessLogger) LogProcessFail() {
	logboek.LogProcessFail(BoldFailOptions())
}

func (l *prettyProcessLogger) LogProcessEnd() {
	logboek.LogProcessEnd(BoldEndOptions())
}

type logProcessDescriptor struct {
	StartedAt time.Time
	Msg       string
}

func (d *logProcessDescriptor) formatTime() string {
	return fmt.Sprintf("%.2f seconds", time.Since(d.StartedAt).Seconds())
}

type wrappedProcessLogger struct {
	logger          Logger
	activeProcesses []*logProcessDescriptor
}

func newWrappedProcessLogger(logger Logger) *wrappedProcessLogger {
	return &wrappedProcessLogger{logger: logger}
}

func (l *wrappedProcessLogger) LogProcessStart(msg string) {
	p := &logProcessDescriptor{
		StartedAt: time.Now(),
		Msg:       msg,
	}

	l.activeProcesses = append(l.activeProcesses, p)
	l.logger.LogInfoLn(msg)
}

func (l *wrappedProcessLogger) LogProcessEnd() {
	p := l.popProcess()

	msg := "SUCCESS"
	if p != nil {
		msg = fmt.Sprintf("%s (%s)", p.Msg, p.formatTime())
	}

	l.logger.LogInfoLn(msg)
}

func (l *wrappedProcessLogger) LogProcessFail() {
	p := l.popProcess()

	msg := "FAILED"
	if p != nil {
		msg = fmt.Sprintf("%s FAILED (%s)", p.Msg, p.formatTime())
	}

	l.logger.LogErrorLn(msg)
}

func (l *wrappedProcessLogger) popProcess() *logProcessDescriptor {
	procIndx := len(l.activeProcesses) - 1
	if procIndx < 0 {
		return nil
	}

	logProcess := l.activeProcesses[procIndx]
	l.activeProcesses = l.activeProcesses[:procIndx]

	return logProcess
}
