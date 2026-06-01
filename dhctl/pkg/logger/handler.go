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
	"context"
	"io"
	"log/slog"
	"os"

	"github.com/mattn/go-isatty"
)

// TerminalUIHandler is a dual-sink slog.Handler.
//   - File sink: receives every enabled record (JSON).
//   - TTY sink:  receives only records tagged with ShowInCompacted(), and only when stdout is a terminal.
type TerminalUIHandler struct {
	file  slog.Handler
	tty   slog.Handler // nil when not a TTY
	level slog.Leveler
	// verbose (the -v flag) makes the terminal sink show EVERY record (at or above ttyLevel), not
	// just ShowInCompacted()-tagged ones. The file sink always receives everything regardless.
	verbose bool
	// ttyLevel is the minimum level routed to the terminal. It is Info by default and Debug only
	// when DHCTL_DEBUG is set, so DEBUG records reach the terminal solely in debug mode (the file
	// sink always keeps them). Independent of verbose: -v shows all Info+ lines without DEBUG.
	ttyLevel slog.Level
	// compactTagged is true when a ShowInCompacted() marker was applied via WithAttrs (e.g. logger.With(ShowInCompacted())).
	// Such handler-level tagging never reaches the record passed to Handle, so we track it here.
	compactTagged bool
}

func handlerOptions(level slog.Leveler) *slog.HandlerOptions {
	return &slog.HandlerOptions{Level: level, ReplaceAttr: Sanitize}
}

// NewTerminalUIHandler builds a handler writing the file sink to out and, when out is a
// TTY, a terminal sink to the same stream. Used by BufferLogger and similar callers.
func NewTerminalUIHandler(out io.Writer) slog.Handler {
	lv := new(slog.LevelVar)
	lv.Set(slog.LevelDebug)
	isTTY := false
	if f, ok := out.(*os.File); ok {
		isTTY = isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
	}
	var ttyW io.Writer
	if isTTY {
		ttyW = out
	}
	return newTerminalUIHandler(out, ttyW, isTTY, false /* interactive */, lv, false, slog.LevelInfo)
}

func newTerminalUIHandler(fileW, ttyW io.Writer, isTTY, interactive bool, level slog.Leveler, verbose bool, ttyLevel slog.Level) *TerminalUIHandler {
	h := &TerminalUIHandler{
		file:     slog.NewJSONHandler(fileW, handlerOptions(level)),
		level:    level,
		verbose:  verbose,
		ttyLevel: ttyLevel,
	}
	if isTTY && ttyW != nil {
		// Use the progress-capable renderer as the terminal sink. The pinned pterm bar is only used
		// when interactive AND the writer is a real terminal; otherwise (non-interactive -v dump, or
		// any non-terminal writer like a bytes.Buffer in tests or a piped stdout) fall back to a plain
		// progressUI that writes plain lines and never starts a pinned block or pterm's MultiPrinter.
		real := isRealTerminal(ttyW)
		var ui progressUI
		if interactive && real {
			ui = newTerminalProgressUI(ttyW)
		} else {
			ui = newPlainProgressUI(ttyW)
		}
		h.tty = newTTYRenderer(ttyW, ui, level, real, verbose)
	}
	return h
}

// isRealTerminal reports whether w is an *os.File backed by an interactive terminal.
func isRealTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
}

func (h *TerminalUIHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

func (h *TerminalUIHandler) Handle(ctx context.Context, r slog.Record) error {
	// File sink first — it must never be lost.
	if err := h.file.Handle(ctx, r); err != nil {
		return err
	}
	// Terminal sink routing:
	//   - verbose (-v): everything.
	//   - control markers (progress/process): always — they drive the bar/current-action/boxes,
	//     they are not visible text themselves.
	//   - ShowInCompacted(): the curated compact-view text (successful phase transitions).
	//   - Warn and above: always visible, so failures are never hidden in compact mode.
	// DEBUG records reach the terminal only in debug mode (ttyLevel == Debug); the file keeps them always.
	// FileOnly records (lib-connection streamed command output) stay off the compact terminal even at
	// Error level — they would flood it; the user is pointed to the debug-log file. -v shows them.
	if h.tty != nil && r.Level >= h.ttyLevel && !(hasFileOnly(r) && !h.verbose) &&
		(h.verbose || isRendererMarker(r) || h.compactTagged || hasShowInCompacted(r) || r.Level >= slog.LevelWarn) {
		// Terminal output errors are non-fatal for the record's persistence.
		_ = h.tty.Handle(ctx, r)
	}
	return nil
}

func (h *TerminalUIHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	n := &TerminalUIHandler{
		file:          h.file.WithAttrs(attrs),
		level:         h.level,
		verbose:       h.verbose,
		ttyLevel:      h.ttyLevel,
		compactTagged: h.compactTagged || attrsContainShowInCompacted(attrs),
	}
	if h.tty != nil {
		n.tty = h.tty.WithAttrs(attrs)
	}
	return n
}

func (h *TerminalUIHandler) WithGroup(name string) slog.Handler {
	n := &TerminalUIHandler{
		file:          h.file.WithGroup(name),
		level:         h.level,
		verbose:       h.verbose,
		ttyLevel:      h.ttyLevel,
		compactTagged: h.compactTagged,
	}
	if h.tty != nil {
		n.tty = h.tty.WithGroup(name)
	}
	return n
}

func attrsContainShowInCompacted(attrs []slog.Attr) bool {
	for _, a := range attrs {
		if a.Key == attrKeyCompact && a.Value.Kind() == slog.KindBool && a.Value.Bool() {
			return true
		}
	}
	return false
}
