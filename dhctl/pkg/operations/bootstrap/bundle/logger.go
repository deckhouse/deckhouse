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

package bundle

import (
	"github.com/deckhouse/lib-dhctl/pkg/log"
)

// logger is a thin adapter over log.Logger that adds an optional prefix and
// the ability to demote Info-level messages to Debug (used for verbose registry output).
type logger struct {
	logger      log.Logger
	prefix      string
	infoAsDebug bool
}

func newLogger(base log.Logger) *logger {
	return &logger{logger: base}
}

func (l *logger) Infof(format string, args ...any) {
	if l.infoAsDebug {
		l.logger.DebugF(l.prefixed(format), args...)
		return
	}
	l.logger.InfoF(l.prefixed(format), args...)
}

func (l *logger) Warnf(format string, args ...any) {
	l.logger.WarnF(l.prefixed(format), args...)
}

func (l *logger) Debugf(format string, args ...any) {
	l.logger.DebugF(l.prefixed(format), args...)
}

func (l *logger) Errorf(format string, args ...any) {
	l.logger.ErrorF(l.prefixed(format), args...)
}

func (l *logger) WithPrefix(prefix string) *logger {
	return &logger{logger: l.logger, prefix: prefix, infoAsDebug: l.infoAsDebug}
}

// WithInfoAsDebug returns a logger that routes Infof calls to DebugF, keeping
// verbose registry-server output out of the default user-visible log level.
func (l *logger) WithInfoAsDebug() *logger {
	return &logger{logger: l.logger, prefix: l.prefix, infoAsDebug: true}
}

func (l *logger) prefixed(format string) string {
	if l.prefix == "" {
		return format
	}
	return l.prefix + format
}
