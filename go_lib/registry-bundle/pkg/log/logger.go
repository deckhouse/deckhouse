/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package log

import (
	"fmt"
	"log/slog"
)

type Logger interface {
	Errorf(format string, args ...any)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Debugf(format string, args ...any)
}

type Slog struct {
	logger *slog.Logger
}

func NewSlog(handler slog.Handler) Logger {
	return &Slog{logger: slog.New(handler)}
}

func (l *Slog) Errorf(format string, args ...any) {
	l.logger.Error(fmt.Sprintf(format, args...))
}

func (l *Slog) Infof(format string, args ...any) {
	l.logger.Info(fmt.Sprintf(format, args...))
}

func (l *Slog) Warnf(format string, args ...any) {
	l.logger.Warn(fmt.Sprintf(format, args...))
}

func (l *Slog) Debugf(format string, args ...any) {
	l.logger.Debug(fmt.Sprintf(format, args...))
}

type Noop struct{}

func NewNoop() Logger {
	return &Noop{}
}

func (l *Noop) Errorf(_ string, _ ...any) {}

func (l *Noop) Infof(_ string, _ ...any) {}

func (l *Noop) Warnf(_ string, _ ...any) {}

func (l *Noop) Debugf(_ string, _ ...any) {}
