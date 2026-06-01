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
	"fmt"
	"io"
	"strings"

	"github.com/pterm/pterm"
)

const currentActionPrefix = " Current action: "

// terminalProgressUI pins a status block at the bottom of the terminal using direct ANSI cursor
// control. The block is three lines: a blank separator, the progress bar, and a "current action"
// line. Ordinary log lines scroll above it (erase block, write line, redraw). The bar and action
// lines are truncated to width-1 so they never auto-wrap (which would break the fixed-height
// erase). It is event-driven — redraw only on change or when a line scrolls past — so it never
// flickers. All methods are serialized by the handler mutex.
type terminalProgressUI struct {
	w io.Writer

	active bool // a progress session is open
	paused bool // rendering suspended for interactive input
	shown  bool // the block is currently drawn at the bottom

	frac   float64
	title  string
	action string
}

const progressBlockLines = 3 // blank + bar + action

func newTerminalProgressUI(w io.Writer) progressUI {
	return &terminalProgressUI{w: w}
}

func (t *terminalProgressUI) Start(name string) {
	t.active = true
	t.frac = 0
	t.title = name
	t.action = ""
	t.draw()
}

func (t *terminalProgressUI) SetProgress(frac float64, title string) {
	if !t.active {
		return
	}
	t.frac = clampFrac(frac)
	if title != "" {
		t.title = title
	}
	if !t.paused {
		t.redraw()
	}
}

func (t *terminalProgressUI) SetAction(text string) {
	if !t.active {
		return
	}
	t.action = text
	if !t.paused {
		t.redraw()
	}
}

// WriteLine prints an ordinary line above the pinned block: erase the block, write the line (it
// becomes permanent scrollback), then redraw the block at the new bottom.
func (t *terminalProgressUI) WriteLine(s string) {
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	t.erase()
	_, _ = io.WriteString(t.w, s)
	if t.active && !t.paused {
		t.draw()
	}
}

func (t *terminalProgressUI) Finish() {
	t.erase()
	t.active = false
	t.paused = false
}

func (t *terminalProgressUI) Pause() {
	t.paused = true
	t.erase()
}

func (t *terminalProgressUI) Resume() {
	t.paused = false
	if t.active {
		t.draw()
	}
}

// Resize re-fits the pinned block to the current terminal width. barLine/actionLine already query
// the width on every draw, so a plain redraw picks up the new size. Called on SIGWINCH under the
// renderer mutex.
func (t *terminalProgressUI) Resize() {
	if t.active && !t.paused {
		t.redraw()
	}
}

// erase removes the currently drawn block: move the cursor up the block height and clear to end
// of screen.
func (t *terminalProgressUI) erase() {
	if !t.shown {
		return
	}
	_, _ = io.WriteString(t.w, fmt.Sprintf("\x1b[%dA\x1b[J", progressBlockLines))
	t.shown = false
}

// draw writes the block (blank separator + bar + current-action line) at the cursor; the cursor
// ends below it.
func (t *terminalProgressUI) draw() {
	_, _ = io.WriteString(t.w, "\n"+t.barLine()+"\n"+t.actionLine()+"\n")
	t.shown = true
}

func (t *terminalProgressUI) redraw() {
	t.erase()
	t.draw()
}

func (t *terminalProgressUI) barLine() string {
	width := pterm.GetTerminalWidth()
	if width <= 0 {
		width = 80
	}
	avail := width - 1 // never fill the last column, to avoid auto-wrap

	pct := int(t.frac*100 + 0.5)
	pctStr := fmt.Sprintf("%3d%%", pct)

	const minBar = 10
	titleBudget := avail - 2 /*brackets*/ - 1 /*space*/ - len(pctStr) - 1 /*space*/ - minBar
	title := truncate(t.title, max(0, titleBudget))

	barWidth := avail - 2 - 1 - len(pctStr) - 1 - len(title)
	if barWidth < minBar {
		barWidth = minBar
	}
	filled := int(t.frac * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	if filled < 0 {
		filled = 0
	}
	bar := pterm.NewStyle(pterm.FgGreen).Sprint(strings.Repeat("█", filled)) +
		strings.Repeat("░", barWidth-filled)
	return "[" + bar + "] " + pctStr + " " + title
}

func (t *terminalProgressUI) actionLine() string {
	width := pterm.GetTerminalWidth()
	if width <= 0 {
		width = 80
	}
	return truncate(currentActionPrefix+t.action, width-1)
}

func clampFrac(f float64) float64 {
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

// truncate shortens s (by visible runes) to n, appending an ellipsis when cut. It assumes s has
// no embedded ANSI (titles/actions are plain text).
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:max(0, n)])
	}
	return string(r[:n-1]) + "…"
}
