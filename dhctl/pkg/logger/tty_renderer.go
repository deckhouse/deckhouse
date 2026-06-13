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
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/pterm/pterm"
)

// progressUI is the minimal surface the TTY renderer needs to draw a pinned progress bar. The
// production implementation pins a single bottom line; tests use a fake.
type progressUI interface {
	Start(name string)                      // open a pinned bar titled name
	SetProgress(frac float64, title string) // advance bar to frac (0..1), set bar title
	SetAction(text string)                  // (legacy spinner; boxes now convey the current action)
	WriteLine(s string)                     // print an ordinary line above the bar
	Finish()                                // close the bar
	Pause()                                 // stop rendering around interactive input
	Resume()                                // restart rendering after a pause
	Resize()                                // re-fit the pinned block to the current terminal width
}

type procFrame struct {
	name  string
	start time.Time
}

// ttyRenderer is a slog.Handler that renders the legacy logboek-style UI: process blocks framed
// with ┌/│/└, nested by indentation, with durations; level-styled log lines; and an optional
// pinned progress bar driven by progress markers. Ordinary lines and box borders scroll above the
// bar when one is active.
type ttyRenderer struct {
	// mu serializes Handle across goroutines. Log lines come from the operation goroutine while the
	// progress bar is advanced from consumeProgress's goroutine; both mutate the shared ui and the
	// box stack and write interleaving ANSI. mu is a pointer so WithAttrs/WithGroup clones (which
	// share the same ui) share the same lock. Held for the whole Handle body.
	mu *sync.Mutex

	ui      progressUI
	out     io.Writer
	level   slog.Leveler
	color   bool // ANSI styling, real terminal only
	verbose bool // -v: draw framed process boxes + every line; compact: bar + current-action only

	stack []procFrame
}

func newTTYRenderer(out io.Writer, ui progressUI, level slog.Leveler, color, verbose bool) *ttyRenderer {
	h := &ttyRenderer{mu: &sync.Mutex{}, ui: ui, out: out, level: level, color: color, verbose: verbose}
	// Only the real-terminal UI has a pinned block to re-fit; the plain/test UIs ignore resizes.
	// WithAttrs/WithGroup clones reuse this ui, so the single watcher started here covers them too.
	if _, ok := ui.(*terminalProgressUI); ok {
		h.startResizeWatcher()
	}
	return h
}

// startResizeWatcher redraws the pinned progress block when the terminal is resized, so the bar
// re-fits the new width immediately instead of waiting for the next progress event. The redraw runs
// under the renderer mutex, serialized with log/progress Handle calls.
func (h *ttyRenderer) startResizeWatcher() {
	ch := notifyResize()
	if ch == nil {
		return
	}
	go func() {
		for range ch {
			h.mu.Lock()
			h.ui.Resize()
			h.mu.Unlock()
		}
	}()
}

func (h *ttyRenderer) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

func (h *ttyRenderer) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Progress bar lifecycle.
	switch progressEvent(r) {
	case progressStart:
		h.ui.Start(recordProgressName(r))
		return nil
	case progressEnd:
		h.ui.Finish()
		return nil
	case progressPause:
		h.ui.Pause()
		return nil
	case progressResume:
		h.ui.Resume()
		return nil
	}
	if v, ok := progressValue(r); ok {
		h.ui.SetProgress(v, progressTitle(r))
		return nil
	}

	// Process blocks. In verbose mode they render as framed boxes (┌ … └ with duration); in
	// compact mode a process-start just updates the pinned bar's "current action".
	switch ev := recordProcessEvent(r); ev {
	case string(processStart):
		name := recordProcessName(r)
		if !h.verbose {
			h.ui.SetAction(name)
			return nil
		}
		h.scroll(h.prefix(len(h.stack)) + boxOpen + " " + h.styleTitle(name))
		h.stack = append(h.stack, procFrame{name: name, start: time.Now()})
		return nil
	case string(processEnd), string(processFail):
		if !h.verbose {
			return nil
		}
		var f procFrame
		if n := len(h.stack); n > 0 {
			f = h.stack[n-1]
			h.stack = h.stack[:n-1]
		}
		dur := time.Since(f.start).Seconds()
		title := h.styleTitle(f.name)
		tail := fmt.Sprintf(" (%.2f seconds)", dur)
		if ev == string(processFail) {
			tail = " FAILED" + tail
			if h.color {
				title = pterm.NewStyle(pterm.FgRed).Sprint(f.name)
			}
		}
		h.scroll(h.prefix(len(h.stack)) + boxClose + " " + title + h.dim(tail))
		// Separator line after a closed block: keep the vertical guides of any enclosing
		// process so nested boxes stay visually connected (matches the legacy logger).
		h.scroll(strings.TrimRight(h.prefix(len(h.stack)), " "))
		return nil
	}

	// Curated status line: render the legacy colored badge (┌ green SUCCESS / red FAILED /
	// yellow WARNING) followed by the title, indented to the current process depth.
	if status := badgeStatus(r); status != "" {
		title := Sanitize(nil, slog.String(slog.MessageKey, r.Message)).Value.String()
		h.scroll(h.prefix(len(h.stack)) + h.badge(status) + "  " + title)
		return nil
	}

	// Ordinary log line(s), indented to the current process depth. A multi-line message gets
	// the indent prefix on EVERY line so nested/tabular content stays inside the box guides.
	msg := Sanitize(nil, slog.String(slog.MessageKey, r.Message)).Value.String()
	prefix := h.prefix(len(h.stack))
	for _, ln := range strings.Split(strings.TrimRight(msg, "\n"), "\n") {
		h.scroll(prefix + h.styleText(r.Level, ln))
	}
	return nil
}

const (
	boxOpen  = "┌"
	boxClose = "└"
	boxBody  = "│ "
)

func (h *ttyRenderer) prefix(depth int) string {
	if depth <= 0 {
		return ""
	}
	return strings.Repeat(boxBody, depth)
}

// scroll writes one rendered line (no trailing newline expected) to the terminal. It always goes
// through the ui: when a progress block is pinned, WriteLine erases it, prints the line as
// permanent scrollback, and redraws the block at the new bottom; when no block is shown, WriteLine
// just prints the line. Routing every line through the ui (instead of writing to h.out directly
// when "no bar") keeps the pinned block consistent even for log lines emitted via a .With()-derived
// logger, whose renderer clone would otherwise hold a stale barActive flag.
func (h *ttyRenderer) scroll(line string) {
	h.ui.WriteLine(line + "\n")
}

// styleText level-styles a single line (Info plain, Warn bold-yellow, Error red) when color is
// enabled — matching the legacy pretty logger. Applied per line so colors never bleed across a
// multi-line message.
func (h *ttyRenderer) styleText(level slog.Level, s string) string {
	if !h.color {
		return s
	}
	switch {
	case level >= slog.LevelError:
		return pterm.NewStyle(pterm.FgRed).Sprint(s)
	case level >= slog.LevelWarn:
		return pterm.NewStyle(pterm.FgYellow, pterm.Bold).Sprint(s)
	default:
		return s
	}
}

// badge renders a legacy logboek-style status label: a fixed-width word on a colored background.
// Without color (non-terminal) it degrades to the plain padded word so columns still align.
func (h *ttyRenderer) badge(status string) string {
	var label string
	var style *pterm.Style
	switch status {
	case badgeFailed:
		label = " FAILED  "
		style = pterm.NewStyle(pterm.BgRed, pterm.FgLightWhite)
	case badgeWarning:
		label = " WARNING "
		style = pterm.NewStyle(pterm.BgYellow, pterm.FgBlack)
	default: // badgeSuccess
		label = " SUCCESS "
		style = pterm.NewStyle(pterm.BgGreen, pterm.FgBlack)
	}
	if !h.color {
		return label
	}
	return style.Sprint(label)
}

func (h *ttyRenderer) styleTitle(name string) string {
	if h.color {
		return pterm.NewStyle(pterm.Bold).Sprint(name)
	}
	return name
}

func (h *ttyRenderer) dim(s string) string {
	if h.color {
		return pterm.NewStyle(pterm.FgGray).Sprint(s)
	}
	return s
}

func (h *ttyRenderer) WithAttrs(_ []slog.Attr) slog.Handler {
	clone := *h
	return &clone
}

func (h *ttyRenderer) WithGroup(_ string) slog.Handler {
	clone := *h
	return &clone
}

// recordProgressName returns the progress_name value carried by r, or "" if absent.
func recordProgressName(r slog.Record) string {
	var name string
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == attrKeyProgressName {
			name = a.Value.String()
			return false
		}
		return true
	})
	return name
}

// recordProcessEvent returns the process_event value carried by r, or "" if absent.
func recordProcessEvent(r slog.Record) string {
	var ev string
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == attrKeyProcessEvent {
			ev = a.Value.String()
			return false
		}
		return true
	})
	return ev
}

// recordProcessName returns the process_name value carried by r, or "" if absent.
func recordProcessName(r slog.Record) string {
	var name string
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == attrKeyProcessName {
			name = a.Value.String()
			return false
		}
		return true
	})
	return name
}
