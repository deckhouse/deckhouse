// Copyright 2025 Flant JSC
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
	"regexp"
	"strings"
	"sync"

	"github.com/name212/govalue"
)

// Match
// if Regex passed Prefix and Suffix will be ignored
type Match struct {
	Prefix []string
	Suffix []string
	Regex  []*regexp.Regexp
}

func (m *Match) IsValid() error {
	if m == nil {
		return fmt.Errorf("Match is nil")
	}

	if len(m.Regex) > 0 {
		return nil
	}

	if len(m.Prefix) == 0 && len(m.Suffix) == 0 {
		return fmt.Errorf("Invalid Match: must pass Regex or Prefix or/and Suffix")
	}

	return nil
}

type InMemoryLogger struct {
	m       sync.RWMutex
	entries []string
	buffer  *bytes.Buffer

	parent Logger

	errorPrefix string
	debugPrefix string
}

func NewInMemoryLogger() *InMemoryLogger {
	return NewInMemoryLoggerWithParent(NewSilentLogger())
}

func NewInMemoryLoggerWithParent(parent Logger) *InMemoryLogger {
	l := &InMemoryLogger{
		entries: make([]string, 0),
	}

	p := parent

	if govalue.IsNil(p) {
		p = NewSilentLogger()
	}

	l.parent = p

	return l
}

func (l *InMemoryLogger) WithErrorPrefix(prefix string) *InMemoryLogger {
	l.errorPrefix = prefix
	return l
}

func (l *InMemoryLogger) WithDebugPrefix(prefix string) *InMemoryLogger {
	l.debugPrefix = prefix
	return l
}

func (l *InMemoryLogger) WithBuffer(buffer *bytes.Buffer) *InMemoryLogger {
	l.m.Lock()
	defer l.m.Unlock()

	l.buffer = buffer
	return l
}

func (l *InMemoryLogger) Parent() Logger {
	return l.parent
}

func (l *InMemoryLogger) FirstMatch(m *Match) (string, error) {
	if err := m.IsValid(); err != nil {
		return "", err
	}

	l.m.RLock()
	defer l.m.RUnlock()

	for _, entry := range l.entries {
		if l.match(m, entry) {
			return entry, nil
		}
	}

	return "", nil
}

func (l *InMemoryLogger) AllMatches(m *Match) ([]string, error) {
	if err := m.IsValid(); err != nil {
		return nil, err
	}

	l.m.RLock()
	defer l.m.RUnlock()

	result := make([]string, 0)

	for _, entry := range l.entries {
		if l.match(m, entry) {
			result = append(result, entry)
		}
	}

	return result, nil
}

func (l *InMemoryLogger) FlushAndClose() error {
	return nil
}

func (l *InMemoryLogger) LogProcess(p string, t string, action func() error) error {
	l.writeEntityFormatted("Start process: %s/%s", p, t)
	err := l.parent.LogProcess(p, t, action)
	l.writeEntityFormatted("End process: %s/%s", p, t)
	return err
}

func (l *InMemoryLogger) LogInfoF(format string, a ...interface{}) {
	l.writeEntityFormatted(format, a...)
	l.parent.LogInfoF(format, a...)
}
func (l *InMemoryLogger) LogInfoLn(a ...interface{}) {
	l.writeEntityFormatted("%v\n", a)
	l.parent.LogInfoLn(a...)
}

func (l *InMemoryLogger) LogErrorF(format string, a ...interface{}) {
	l.writeEntityWithPrefix(l.errorPrefix, format, a...)
	l.parent.LogErrorF(format, a...)
}
func (l *InMemoryLogger) LogErrorLn(a ...interface{}) {
	l.writeEntityWithPrefix(l.errorPrefix, "%v\n", a)
	l.parent.LogErrorLn(a...)
}

func (l *InMemoryLogger) LogDebugF(format string, a ...interface{}) {
	l.writeEntityWithPrefix(l.debugPrefix, format, a...)
	l.parent.LogDebugF(format, a...)
}

func (l *InMemoryLogger) LogDebugLn(a ...interface{}) {
	l.writeEntityWithPrefix(l.debugPrefix, "%v\n", a)
	l.parent.LogDebugLn(a...)
}

func (l *InMemoryLogger) LogWarnF(format string, a ...interface{}) {
	l.writeEntityFormatted(format, a...)
	l.parent.LogWarnF(format, a...)
}
func (l *InMemoryLogger) LogWarnLn(a ...interface{}) {
	l.writeEntityFormatted("%v\n", a)
	l.parent.LogWarnLn(a...)
}

func (l *InMemoryLogger) LogSuccess(s string) {
	l.writeEntityFormatted("Success: %s", s)
	l.parent.LogSuccess(s)
}
func (l *InMemoryLogger) LogFail(s string) {
	l.writeEntityWithPrefix(l.errorPrefix, "Fail: %s", s)
	l.parent.LogFail(s)

}
func (l *InMemoryLogger) LogFailRetry(s string) {
	l.writeEntityWithPrefix(l.errorPrefix, "Fail retry: %s", s)
	l.parent.LogFailRetry(s)
}

func (l *InMemoryLogger) LogJSON(s []byte) {
	l.writeEntity(string(s))
	l.parent.LogJSON(s)
}

func (l *InMemoryLogger) ProcessLogger() ProcessLogger {
	return l
}

func (l *InMemoryLogger) NewSilentLogger() *SilentLogger {
	return NewSilentLogger()
}

func (l *InMemoryLogger) CreateBufferLogger(buffer *bytes.Buffer) Logger {
	return l.WithBuffer(buffer)
}

func (l *InMemoryLogger) Write(s []byte) (int, error) {
	l.writeEntity(string(s))
	return l.parent.Write(s)
}

func (l *InMemoryLogger) LogProcessStart(name string) {
	l.writeEntityFormatted("Start process: %s", name)
}

func (l *InMemoryLogger) LogProcessFail() {
	l.writeEntityWithPrefix(l.errorPrefix, "Fail process")
}

func (l *InMemoryLogger) LogProcessEnd() {
	l.writeEntity("End process")
}

func (l *InMemoryLogger) match(m *Match, entity string) bool {
	if len(m.Regex) > 0 {
		for _, regex := range m.Regex {
			if regex.MatchString(entity) {
				return true
			}
		}

		return false
	}

	for _, prefix := range m.Prefix {
		if strings.HasPrefix(entity, prefix) {
			return true
		}
	}

	for _, suffix := range m.Suffix {
		if strings.HasSuffix(entity, suffix) {
			return true
		}
	}

	return false
}

func (l *InMemoryLogger) writeEntity(entity string) {
	l.m.Lock()
	defer l.m.Unlock()

	l.entries = append(l.entries, entity)

	if l.buffer != nil {
		l.buffer.WriteString(entity)
	}
}

func (l *InMemoryLogger) formatString(f string, a ...any) string {
	format := f
	if format == "" {
		format = "%v"
	}

	return fmt.Sprintf(format, a...)
}

func (l *InMemoryLogger) writeEntityFormatted(f string, a ...any) {
	l.writeEntity(l.formatString(f, a...))
}

func (l *InMemoryLogger) writeEntityWithPrefix(prefix, f string, a ...any) {
	msg := l.formatString(f, a...)

	if prefix != "" {
		l.writeEntityFormatted("%s: %s", prefix, msg)
		return
	}

	l.writeEntity(msg)
}
