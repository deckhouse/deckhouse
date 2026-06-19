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

// lineSink is the text-output surface every TTY backend implements: it receives the rendered
// milestones, warn/error lines, ordinary detail (including the framed process boxes), the banner,
// and the connection string. termui.Block routes them into its pinned live region; plainProgressUI
// prints them straight to the writer (the logboek-style dump).
type lineSink interface {
	Milestone(status, text string) // curated SUCCESS/WARNING/FAILED line
	Warn(line string)              // pinned Warn+ line
	Log(line string)               // ephemeral detail line (framed boxes + indented detail)
	SetBanner(lines []string)      // pin the startup ASCII banner at the top of the live canvas
	SetConnString(s string)        // pin the SSH connection string just above the logbox
}

// progressBar is the pinned-bar surface only the live termui.Block implements. The plain logboek
// backend has no bar, so the renderer holds it as an optional dependency (nil for plain) and drives
// it only when present — instead of forcing the plain backend to no-op these six methods.
type progressBar interface {
	Start(name string)                      // open a pinned bar titled name
	SetProgress(frac float64, title string) // advance bar to frac (0..1), set bar title
	SetAction(text string)                  // current-action line under the bar
	Finish()                                // close the bar
	Pause()                                 // stop rendering around interactive input
	Resume()                                // restart rendering after a pause
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
	// progress bar is advanced from consumeProgress's goroutine; both mutate the shared sink/bar and
	// the box stack and write interleaving ANSI. mu is a pointer so WithAttrs/WithGroup clones (which
	// share the same sink) share the same lock. Held for the whole Handle body.
	mu *sync.Mutex

	sink  lineSink
	bar   progressBar // nil when the backend has no pinned bar (the plain logboek dump)
	out   io.Writer
	level slog.Leveler
	color bool // ANSI styling, real terminal only

	stack []procFrame
}

// rendererConfig holds the parameters of newTTYRenderer. bar is optional: nil for the plain backend.
type rendererConfig struct {
	out   io.Writer
	sink  lineSink
	bar   progressBar
	level slog.Leveler
	color bool // ANSI styling, real terminal only
}

func newTTYRenderer(cfg rendererConfig) *ttyRenderer {
	// Resize handling now lives inside the UI (termui.Block watches SIGWINCH itself); the renderer
	// no longer starts a watcher.
	return &ttyRenderer{mu: &sync.Mutex{}, sink: cfg.sink, bar: cfg.bar, out: cfg.out, level: cfg.level, color: cfg.color}
}

func (h *ttyRenderer) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

// Handle renders r. Its message is assumed already redacted: the only production caller is
// TerminalUIHandler.Handle, which runs sanitizeMessage before fan-out, so the renderer never
// re-sanitizes and no render path can leak a secret.
func (h *ttyRenderer) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Progress-bar markers drive the optional bar; they carry no printable text.
	if h.handleBarMarker(r) {
		return nil
	}

	if hasBanner(r) {
		h.sink.SetBanner(strings.Split(strings.TrimRight(r.Message, "\n"), "\n"))
		return nil
	}

	if hasConnectionString(r) {
		h.sink.SetConnString(strings.TrimRight(r.Message, "\n"))
		return nil
	}

	// Process blocks always render as framed boxes (┌ … └ with duration), matching the old logboek.
	// A process-start also updates the pinned bar's "current action" line when a bar is present (the
	// live Block); the plain logboek dump has no bar, so only the framed box is emitted.
	switch ev := recordProcessEvent(r); ev {
	case string(processStart):
		name := recordProcessName(r)
		if h.bar != nil {
			h.bar.SetAction(name)
		}
		h.scroll(h.prefix(len(h.stack)) + boxOpen + " " + h.styleTitle(name))
		h.stack = append(h.stack, procFrame{name: name, start: time.Now()})
		return nil
	case string(processEnd), string(processFail):
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
		if ev == string(processFail) {
			// The framed box above is ephemeral detail (sink.Log → the live Block's logbox ring),
			// which is wiped when the Block leaves the alt screen on teardown. A failed process is
			// the primary failure signal and must outlive that: also emit a persistent FAILED
			// milestone, which summarizeLocked keeps on the main screen in compact mode. The plain
			// backend prints both lines — harmless redundancy on the rare failure path.
			h.sink.Milestone(milestoneStatus(badgeFailed), f.name)
		}
		return nil
	}

	// Curated status line: a milestone the sink renders as its own SUCCESS/WARNING/FAILED line.
	if status := badgeStatus(r); status != "" {
		h.sink.Milestone(milestoneStatus(status), r.Message)
		return nil
	}

	// Ordinary log line(s). Warn and above are pinned by the sink; lower levels are ephemeral detail
	// indented to the current process depth. A multi-line message is split so each line is routed
	// individually (and detail lines keep the indent prefix so nested/tabular content stays aligned).
	for _, ln := range strings.Split(strings.TrimRight(r.Message, "\n"), "\n") {
		if r.Level >= slog.LevelWarn {
			h.sink.Warn(h.styleText(r.Level, ln))
		} else {
			h.sink.Log(h.prefix(len(h.stack)) + h.styleText(r.Level, ln))
		}
	}
	return nil
}

// handleBarMarker drives the optional progress bar from a progress marker record and reports
// whether r was a bar marker (and thus fully consumed). When the backend has no bar (h.bar == nil,
// the plain logboek dump) the marker is silently consumed — it is renderer control, not text.
func (h *ttyRenderer) handleBarMarker(r slog.Record) bool {
	switch progressEvent(r) {
	case progressStart:
		if h.bar != nil {
			h.bar.Start(recordProgressName(r))
		}
		return true
	case progressEnd:
		if h.bar != nil {
			h.bar.Finish()
		}
		return true
	case progressPause:
		if h.bar != nil {
			h.bar.Pause()
		}
		return true
	case progressResume:
		if h.bar != nil {
			h.bar.Resume()
		}
		return true
	}
	if v, ok := progressValue(r); ok {
		if h.bar != nil {
			h.bar.SetProgress(v, progressTitle(r))
		}
		return true
	}
	return false
}

// milestoneStatus maps an internal badge value to the UI milestone status string.
func milestoneStatus(badge string) string {
	switch badge {
	case badgeFailed:
		return "FAILED"
	case badgeWarning:
		return "WARNING"
	default:
		return "SUCCESS"
	}
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

// scroll routes one rendered line (the framed-box borders/bodies) to the sink as ephemeral detail.
// Routing through the sink keeps the pinned block consistent even for lines emitted via a
// .With()-derived logger whose renderer clone shares the same sink.
func (h *ttyRenderer) scroll(line string) {
	h.sink.Log(line)
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

// WithAttrs / WithGroup intentionally drop the attrs/group: the terminal view is curated text
// (message + the marker attrs read off the record in Handle), not a structured dump, so persisted
// With-attributes are the file (JSON) sink's concern. The clone shares mu, sink and bar by
// value-copy of the interface/pointer values, so a .With()-derived logger keeps rendering into the
// same pinned block under the same lock.
func (h *ttyRenderer) WithAttrs(_ []slog.Attr) slog.Handler {
	clone := *h
	return &clone
}

func (h *ttyRenderer) WithGroup(_ string) slog.Handler {
	clone := *h
	return &clone
}
