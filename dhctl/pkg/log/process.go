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

	"github.com/werf/logboek/pkg/types"
)

type processStack struct {
	activeProcesses []*logProcessDescriptor
}

func (s *processStack) push(p *logProcessDescriptor) {
	s.activeProcesses = append(s.activeProcesses, p)
}

func (s *processStack) pop() *logProcessDescriptor {
	procIndx := len(s.activeProcesses) - 1
	if procIndx < 0 {
		return nil
	}

	logProcess := s.activeProcesses[procIndx]
	s.activeProcesses = s.activeProcesses[:procIndx]

	return logProcess
}

type prettyProcessLogger struct {
	processes     *processStack
	logboekLogger types.LoggerInterface
}

func newPrettyProcessLogger(logboekLogger types.LoggerInterface) *prettyProcessLogger {
	return &prettyProcessLogger{
		processes:     &processStack{},
		logboekLogger: logboekLogger,
	}
}

func (l *prettyProcessLogger) LogProcessStart(msg string) {
	// we do not need to store message and date, because logboek store it itself
	// we use stack for prevent panic from logboek
	proc := l.logboekLogger.LogProcess(msg).Options(BoldStartOptions)
	l.processes.push(&logProcessDescriptor{LogboekProcess: proc})
	proc.Start()
}

func (l *prettyProcessLogger) LogProcessFail() {
	p := l.processes.pop()
	if p != nil {
		p.LogboekProcess.Fail()
	}
}

func (l *prettyProcessLogger) LogProcessEnd() {
	p := l.processes.pop()
	if p != nil {
		p.LogboekProcess.End()
	}
}

type logProcessDescriptor struct {
	StartedAt      time.Time
	Msg            string
	LogboekProcess types.LogProcessInterface
}

func (d *logProcessDescriptor) formatTime() string {
	return fmt.Sprintf("%.2f seconds", time.Since(d.StartedAt).Seconds())
}

type wrappedProcessLogger struct {
	logger    Logger
	processes *processStack
}

func newWrappedProcessLogger(logger Logger) *wrappedProcessLogger {
	return &wrappedProcessLogger{
		logger:    logger,
		processes: &processStack{},
	}
}

func (l *wrappedProcessLogger) LogProcessStart(msg string) {
	p := &logProcessDescriptor{
		StartedAt: time.Now(),
		Msg:       msg,
	}

	l.processes.push(p)

	l.logger.LogInfoLn(msg)
}

func (l *wrappedProcessLogger) LogProcessEnd() {
	p := l.processes.pop()

	msg := "SUCCESS"
	if p != nil {
		msg = fmt.Sprintf("%s (%s)", p.Msg, p.formatTime())
	}

	l.logger.LogInfoLn(msg)
}

func (l *wrappedProcessLogger) LogProcessFail() {
	p := l.processes.pop()

	msg := "FAILED"
	if p != nil {
		msg = fmt.Sprintf("%s FAILED (%s)", p.Msg, p.formatTime())
	}

	l.logger.LogErrorLn(msg)
}
