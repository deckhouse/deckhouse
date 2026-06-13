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
	// Debug (DHCTL_DEBUG) lowers the terminal threshold to include DEBUG records. The file sink
	// always keeps DEBUG regardless.
	Debug bool
}

// NewRoot builds the application root logger. Replaces InitLogger / InitLoggerWithOptions /
// WrapWithTeeLogger / NewLogToFile from the old package.
func NewRoot(opts Options) *slog.Logger {
	// The file sink always captures everything (full debug log), so the level stays at Debug
	// regardless of the flag. The Debug flag instead drives terminal verbosity below.
	lv := new(slog.LevelVar)
	lv.Set(slog.LevelDebug)

	// enableTTY turns on the terminal sink whenever stdout is a terminal — independent of verbosity,
	// so -v never silences the terminal. The pinned pterm bar is used only when Interactive; otherwise
	// (e.g. -v) the handler renders plain linear lines. verbose (-v / Debug) makes the terminal show
	// every record; otherwise it shows only the curated ShowInCompacted()-tagged output (process
	// boxes, step changes, status) — everything else goes to the debug file only.
	enableTTY := opts.IsTTY && opts.TTYWriter != nil
	// Terminal threshold: Info normally, Debug only in debug mode. The file sink stays at Debug (lv).
	ttyLevel := slog.LevelInfo
	if opts.Debug {
		ttyLevel = slog.LevelDebug
	}
	return slog.New(newTerminalUIHandler(opts.FileWriter, opts.TTYWriter, enableTTY, opts.Interactive, lv, opts.Verbose, ttyLevel))
}
