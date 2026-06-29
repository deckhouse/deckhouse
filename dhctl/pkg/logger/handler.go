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
	"github.com/pterm/pterm"

	"github.com/deckhouse/deckhouse/dhctl/pkg/logger/termui"
)

// TerminalUIHandler is a dual-sink slog.Handler.
//   - File sink: receives every enabled record (JSON).
//   - TTY sink:  receives only records tagged with ShowInCompacted(), and only when stdout is a terminal.
type TerminalUIHandler struct {
	file  slog.Handler
	tty   slog.Handler // nil when not a TTY
	level slog.Leveler
	// verbose (the -v flag) makes the terminal sink show EVERY record at Info+, not just
	// ShowInCompacted()-tagged ones. The file sink always receives everything regardless. DEBUG
	// records never reach the terminal (the file sink keeps them); DHCTL_DEBUG only enriches the file.
	verbose bool
	// compactTagged is true when a ShowInCompacted() marker was applied via WithAttrs (e.g. logger.With(ShowInCompacted())).
	// Such handler-level tagging never reaches the record passed to Handle, so we track it here.
	compactTagged bool
	// interactiveBlock is true when the terminal sink is a live termui.Block (interactive + real
	// terminal). In that mode ordinary detail (Info + FileOnly) is also forwarded to the tty sink so
	// it feeds the block's ephemeral log box; the file sink keeps it regardless.
	interactiveBlock bool
	// block is the live termui.Block when interactiveBlock is true, nil otherwise. Held so that
	// RestoreTerminal can leave the alternate screen on any exit path (signal, panic, normal).
	block *termui.Block
}

func handlerOptions(level slog.Leveler) *slog.HandlerOptions {
	return &slog.HandlerOptions{Level: level, ReplaceAttr: Sanitize}
}

// handlerConfig holds the parameters of newTerminalUIHandler. Grouping them into named fields —
// especially the three bools (isTTY/interactive/verbose) — keeps call sites self-documenting.
type handlerConfig struct {
	fileW       io.Writer    // always-on file sink; required
	ttyW        io.Writer    // terminal sink stream; nil disables the terminal sink
	isTTY       bool         // ttyW is a terminal (enables the terminal sink at all)
	interactive bool         // prefer the pinned live block when the terminal supports it
	level       slog.Leveler // file-sink threshold (Debug in practice)
	verbose     bool         // -v: terminal shows every record, not just curated output
}

func newTerminalUIHandler(cfg handlerConfig) *TerminalUIHandler {
	h := &TerminalUIHandler{
		file:    slog.NewJSONHandler(cfg.fileW, handlerOptions(cfg.level)),
		level:   cfg.level,
		verbose: cfg.verbose,
	}
	if cfg.isTTY && cfg.ttyW != nil {
		// Pick the terminal backend. The live termui block is only used when interactive AND the writer
		// is a real terminal AND the terminal is tall enough; otherwise (non-interactive -v dump, a tiny
		// terminal, or any non-terminal writer like a bytes.Buffer in tests or a piped stdout) fall back
		// to plainSink, which prints plain logboek lines and has no pinned bar (bar stays nil).
		real := isRealTerminal(cfg.ttyW)
		rc := rendererConfig{out: cfg.ttyW, level: cfg.level, color: real}
		if cfg.interactive && real && fitsLiveBlock() {
			bl := termui.New(cfg.ttyW, termui.Options{Color: real})
			rc.sink = bl
			rc.bar = bl
			h.interactiveBlock = true
			h.block = bl
		} else {
			rc.sink = newPlainSink(cfg.ttyW)
		}
		h.tty = newTTYRenderer(rc)
	}
	return h
}

// fitsLiveBlock reports whether the terminal is tall enough to host the live block. A degenerate
// 1-row terminal cannot draw a pinned multi-line block, so we fall back to plain lines there.
func fitsLiveBlock() bool { return pterm.GetTerminalHeight() >= 2 }

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
	// Redact once, before fan-out, so both sinks see the clean message and no render path can leak.
	// r is a value copy; mutating its Message does not affect the caller's record.
	r.Message = sanitizeMessage(r.Message)
	// File sink first — it must never be lost.
	if err := h.file.Handle(ctx, r); err != nil {
		return err
	}
	if h.routeToTTY(r) {
		// Terminal output errors are non-fatal for the record's persistence.
		_ = h.tty.Handle(ctx, r)
	}
	return nil
}

// routeToTTY decides whether r reaches the terminal sink. The file sink always receives every
// record (handled above); this gate only governs the curated/live terminal view.
//
//   - DEBUG records never reach the terminal (the file sink keeps them); the terminal floor is Info.
//     DHCTL_DEBUG only enriches the file, so the terminal looks identical with or without it.
//   - A live interactive block also wants ordinary detail (Info + FileOnly) to feed its ephemeral
//     log box, so once past the level gate everything is forwarded; the file sink already captured it.
//   - FileOnly records (lib-connection streamed command output) stay off the compact terminal even
//     at Error level — they would flood it; the user is pointed to the debug-log file. -v shows them.
//   - Otherwise a record reaches the terminal when: verbose (-v) shows everything; control markers
//     (progress/process) always pass (they drive the bar/current-action/boxes, not visible text);
//     ShowInCompacted() is the curated compact-view text; Warn and above is always visible so
//     failures are never hidden in compact mode.
func (h *TerminalUIHandler) routeToTTY(r slog.Record) bool {
	if h.tty == nil || r.Level < slog.LevelInfo {
		return false
	}
	if h.interactiveBlock {
		return true
	}
	if hasFileOnly(r) && !h.verbose {
		return false
	}
	return h.verbose || isRendererMarker(r) || h.compactTagged ||
		hasShowInCompacted(r) || r.Level >= slog.LevelWarn
}

func (h *TerminalUIHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	n := &TerminalUIHandler{
		file:             h.file.WithAttrs(attrs),
		level:            h.level,
		verbose:          h.verbose,
		compactTagged:    h.compactTagged || attrsContainShowInCompacted(attrs),
		interactiveBlock: h.interactiveBlock,
		block:            h.block,
	}
	if h.tty != nil {
		n.tty = h.tty.WithAttrs(attrs)
	}
	return n
}

func (h *TerminalUIHandler) WithGroup(name string) slog.Handler {
	n := &TerminalUIHandler{
		file:             h.file.WithGroup(name),
		level:            h.level,
		verbose:          h.verbose,
		compactTagged:    h.compactTagged,
		interactiveBlock: h.interactiveBlock,
		block:            h.block,
	}
	if h.tty != nil {
		n.tty = h.tty.WithGroup(name)
	}
	return n
}

// RestoreTerminal leaves the alternate screen if an interactive Block is active.
// Safe to call multiple times and when no Block exists.
func (h *TerminalUIHandler) RestoreTerminal() {
	if h.block != nil {
		h.block.Restore()
	}
}

func attrsContainShowInCompacted(attrs []slog.Attr) bool {
	for _, a := range attrs {
		if a.Key == attrKeyCompact && a.Value.Kind() == slog.KindBool && a.Value.Bool() {
			return true
		}
	}
	return false
}
