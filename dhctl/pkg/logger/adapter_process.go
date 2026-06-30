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

package logger

import (
	"log/slog"

	libdhctl_log "github.com/deckhouse/lib-dhctl/pkg/log"
)

// ProcessLogger implements libdhctl_log.Logger by returning a ProcessLogger bound to this adapter.
func (a *Adapter) ProcessLogger() libdhctl_log.ProcessLogger {
	return &AdapterProcessLogger{logger: a}
}

// AdapterProcessLogger bridges lib-connection's ProcessLogger to the process-block markers the
// terminal handler renders (┌ … └), mirroring RunProcess.
type AdapterProcessLogger struct {
	logger *Adapter
}

func (ap *AdapterProcessLogger) ProcessStart(name string) {
	// Emit a process-start marker so the terminal handler renders a framed box (┌), matching
	// RunProcess. The matching ProcessEnd/Fail closes it.
	emit(ap.logger.ctx, ap.logger.logger, slog.LevelInfo, "Starting: "+name, processAttr(processStart, name))
}

func (ap *AdapterProcessLogger) ProcessStep(name string) {
	ap.logger.logger.InfoContext(ap.logger.ctx, name)
}

func (ap *AdapterProcessLogger) ProcessEnd() {
	emit(ap.logger.ctx, ap.logger.logger, slog.LevelInfo, "Finished", processAttr(processEnd, ""))
}

func (ap *AdapterProcessLogger) ProcessFail() {
	emit(ap.logger.ctx, ap.logger.logger, slog.LevelError, "Failed", processAttr(processFail, ""))
}
