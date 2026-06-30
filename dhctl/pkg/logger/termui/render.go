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

package termui

import (
	"fmt"
	"strings"
	"time"

	"github.com/pterm/pterm"
)

// frame is an immutable snapshot of what to draw, in display order.
type frame struct {
	title      string
	frac       float64
	elapsed    time.Duration
	action     string
	spinner    rune
	milestones []string // already "STATUS text" formatted; full history, renderFrame shows the last lay.milestones
	warns      []string
	logbox     []string
	width      int
	lay        layout
	color      bool
	banner     []string // startup ASCII banner lines, pinned at the top; nil when terminal is too short
	connString string   // pinned connection string rendered just above the logbox; "" when absent
}

const actionPrefix = "Current action: "

// renderFrame renders the live region top→bottom for the computed layout.
// Every line is truncated to width-1 so the terminal never auto-wraps.
// Banner lines (if any) are pinned at the very top; they are omitted when the terminal
// is too short (frameLocked already leaves banner nil in that case).
func renderFrame(f frame) []string {
	var out []string
	for _, bl := range f.banner {
		out = append(out, trunc(bl, f.width-1))
	}
	out = append(out, barLine(f))
	if !f.lay.action {
		return out
	}
	out = append(out, actionLine(f))
	if !f.lay.region {
		return out
	}
	for _, m := range lastN(f.milestones, f.lay.milestones) {
		out = append(out, trunc(m, f.width-1))
	}
	for _, wl := range f.warns {
		out = append(out, trunc(wl, f.width-1))
	}
	if f.connString != "" {
		out = append(out, trunc(f.connString, f.width-1))
	}
	if f.lay.logbox > 0 {
		for _, l := range lastN(f.logbox, f.lay.logbox) {
			line := "    " + l // indent the detail tail (matches main's logbox offset)
			if f.color {
				line = pterm.NewStyle(pterm.FgDarkGray).Sprint(line)
			}
			out = append(out, trunc(line, f.width-1))
		}
	}
	return out
}

func barLine(f frame) string {
	avail := f.width - 1
	pct := int(clampFrac(f.frac)*100 + 0.5)
	tail := fmt.Sprintf(" [%03d/100] %3d%% | %s", pct, pct, fmtElapsed(f.elapsed))
	const minBar = 10
	titleBudget := avail - len(tail) - 2 /*caps*/ - minBar - 1
	title := trunc(f.title, max(0, titleBudget))
	barWidth := avail - len([]rune(title)) - len(tail) - 2 - 1 // -1 for the space between title and bar
	if barWidth < minBar {
		barWidth = minBar
	}
	filled := int(clampFrac(f.frac) * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	fill := strings.Repeat("█", filled)
	titleOut := title
	tailOut := tail
	if f.color {
		fill = pterm.NewStyle(pterm.FgGreen).Sprint(fill)
		titleOut = pterm.NewStyle(pterm.FgLightCyan).Sprint(title)
		// Keep the count/elapsed default, paint only the percentage red (matches main).
		tailOut = fmt.Sprintf(" [%03d/100] ", pct) +
			pterm.NewStyle(pterm.FgRed).Sprint(fmt.Sprintf("%3d%%", pct)) +
			" | " + fmtElapsed(f.elapsed)
	}
	bar := "▕" + fill + strings.Repeat("░", barWidth-filled) + "▏"
	return trunc(titleOut+" "+bar+tailOut, avail)
}

func actionLine(f frame) string {
	sp := string(f.spinner)
	return trunc(sp+" "+actionPrefix+f.action, f.width-1)
}

// formatMilestone renders a curated milestone line: a colored status badge
// (SUCCESS green / WARNING yellow / FAILED red, matching the legacy logger) followed
// by the text. Without color the badge degrades to the plain padded word.
// CONN is a special case: it renders as cyan text with no badge.
func formatMilestone(color bool, status, text string) string {
	if status == "CONN" {
		if color {
			return pterm.NewStyle(pterm.FgLightCyan).Sprint(text)
		}
		return text
	}
	var label string
	var style *pterm.Style
	switch status {
	case "FAILED":
		label = " FAILED  "
		style = pterm.NewStyle(pterm.BgRed, pterm.FgLightWhite)
	case "WARNING":
		label = " WARNING "
		style = pterm.NewStyle(pterm.BgYellow, pterm.FgBlack)
	default: // SUCCESS
		label = " SUCCESS "
		style = pterm.NewStyle(pterm.BgGreen, pterm.FgBlack)
	}
	if color {
		label = style.Sprint(label)
	}
	return label + " " + text
}

func fmtElapsed(d time.Duration) string {
	s := int(d.Seconds())
	if s < 60 {
		return fmt.Sprintf("%ds", s)
	}
	return fmt.Sprintf("%dm%02ds", s/60, s%60)
}

func lastN(xs []string, n int) []string {
	if n >= len(xs) {
		return xs
	}
	return xs[len(xs)-n:]
}

func clampFrac(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

// visLen returns the number of visible (non-ANSI) runes in s.
func visLen(s string) int {
	n := 0
	inEsc := false
	for _, r := range s {
		if inEsc {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEsc = false
			}
			continue
		}
		if r == '\x1b' {
			inEsc = true
			continue
		}
		n++
	}
	return n
}

// trunc shortens s to n VISIBLE runes (ANSI escape sequences are copied through
// and do not count). When it cuts, it appends an ellipsis and, if the string
// carried any styling, a reset so a severed color never leaks past the cut.
func trunc(s string, n int) string {
	if n < 0 {
		n = 0
	}
	if visLen(s) <= n {
		return s
	}
	keep := n
	ellipsis := ""
	if n >= 1 {
		keep = n - 1
		ellipsis = "…"
	}
	var b strings.Builder
	vis := 0
	inEsc := false
	hasEsc := false
	for _, r := range s {
		if inEsc {
			b.WriteRune(r)
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEsc = false
			}
			continue
		}
		if r == '\x1b' {
			inEsc = true
			hasEsc = true
			b.WriteRune(r)
			continue
		}
		if vis >= keep {
			break
		}
		b.WriteRune(r)
		vis++
	}
	b.WriteString(ellipsis)
	if hasEsc {
		b.WriteString("\x1b[0m")
	}
	return b.String()
}
