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
	"io"
	"log/slog"
	"sync"
)

// Options configures the root logger.
//   - FileWriter receives every record (the always-on debug-file sink). Required.
//   - TTYWriter, when non-nil and IsTTY is true, receives ShowInCompacted()-tagged records (the terminal).
type Options struct {
	FileWriter io.Writer // required; always-on sink
	TTYWriter  io.Writer // optional; terminal sink for ShowInCompacted()-tagged records
	IsTTY      bool      // whether TTYWriter is a terminal (enables the terminal sink at all)
	// Interactive enables the pinned pterm progress bar. False (e.g. with -v) keeps the terminal
	// sink but renders plain linear lines with no pinned block.
	Interactive bool
	// Verbose (-v) shows every Info+ record on the terminal, not just the curated compact output.
	Verbose bool
}

// rootHandler is the *TerminalUIHandler created by the most recent NewRoot call, stored so the
// no-arg RestoreTerminal can leave the alternate screen on any exit path (SIGINT/SIGTERM, panic,
// normal) without threading the handle through the call stack. A CLI owns exactly one terminal, so
// a single guarded slot is the right model. rootMu makes the slot safe against a signal-fired
// RestoreTerminal racing a concurrent NewRoot rebind (action.go installs a fallback root, then
// rebinds the real one). Nil until the first NewRoot.
var (
	rootMu      sync.Mutex
	rootHandler *TerminalUIHandler
)

// setRootHandler records h as the current terminal owner for RestoreTerminal.
func setRootHandler(h *TerminalUIHandler) {
	rootMu.Lock()
	rootHandler = h
	rootMu.Unlock()
}

// RestoreTerminal leaves the alternate screen if the current root handler is using an interactive
// Block. Safe to call before NewRoot, multiple times, and after the Block has already been
// finished. Designed as a no-arg shutdown hook / defer (see cmd/dhctl/main.go).
func RestoreTerminal() {
	rootMu.Lock()
	h := rootHandler
	rootMu.Unlock()
	if h != nil {
		h.RestoreTerminal()
	}
}

// NewRoot builds the application root logger. Replaces InitLogger / InitLoggerWithOptions /
// WrapWithTeeLogger / NewLogToFile from the old package.
func NewRoot(opts Options) *slog.Logger {
	// The file sink always captures everything (full debug log), so the level stays at Debug. The
	// terminal floor is fixed at Info (DEBUG never reaches it); DHCTL_DEBUG only enriches the file.
	lv := new(slog.LevelVar)
	lv.Set(slog.LevelDebug)

	// enableTTY turns on the terminal sink whenever stdout is a terminal — independent of verbosity,
	// so -v never silences the terminal. The pinned pterm bar is used only when Interactive; otherwise
	// (e.g. -v) the handler renders plain linear lines. verbose (-v) makes the terminal show every
	// Info+ record; otherwise it shows only the curated ShowInCompacted()-tagged output (process
	// boxes, step changes, status) — everything else goes to the debug file only.
	enableTTY := opts.IsTTY && opts.TTYWriter != nil
	h := newTerminalUIHandler(handlerConfig{
		fileW:       opts.FileWriter,
		ttyW:        opts.TTYWriter,
		isTTY:       enableTTY,
		interactive: opts.Interactive,
		level:       lv,
		verbose:     opts.Verbose,
	})
	setRootHandler(h)
	return slog.New(h)
}
