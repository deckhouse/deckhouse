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
	"bytes"
	"strings"
	"testing"
)

const eraseSeq = "\x1b[3A\x1b[J"

func TestTerminalProgressUIDrawsBarOnStart(t *testing.T) {
	var buf bytes.Buffer
	ui := newTerminalProgressUI(&buf)
	ui.Start("Phase one")
	out := buf.String()
	if !strings.Contains(out, "Phase one") || !strings.Contains(out, "%") {
		t.Fatalf("Start did not draw the bar: %q", out)
	}
	if strings.Contains(out, eraseSeq) {
		t.Fatalf("Start should not erase before the first draw: %q", out)
	}
	// Block == blank + bar + current-action == three newlines.
	if n := strings.Count(out, "\n"); n != 3 {
		t.Fatalf("bar block should be blank+bar+action (3 newlines), got %d: %q", n, out)
	}
}

func TestTerminalProgressUIWriteLineErasesThenRedraws(t *testing.T) {
	var buf bytes.Buffer
	ui := newTerminalProgressUI(&buf)
	ui.Start("P")
	buf.Reset()

	ui.WriteLine("scrolling log line\n")
	out := buf.String()
	eIdx := strings.Index(out, eraseSeq)
	lIdx := strings.Index(out, "scrolling log line")
	bIdx := strings.LastIndex(out, "%") // redrawn bar
	if eIdx < 0 || lIdx < 0 || bIdx < 0 {
		t.Fatalf("WriteLine missing erase/line/redraw: %q", out)
	}
	if !(eIdx < lIdx && lIdx < bIdx) {
		t.Fatalf("WriteLine order wrong (erase<line<redraw): %q", out)
	}
}

func TestTerminalProgressUIPausedDoesNotRedraw(t *testing.T) {
	var buf bytes.Buffer
	ui := newTerminalProgressUI(&buf)
	ui.Start("P")
	ui.Pause()
	buf.Reset()

	ui.WriteLine("prompt-area line\n")
	ui.SetProgress(0.5, "")
	out := buf.String()
	if strings.Contains(out, "%") {
		t.Fatalf("paused UI redrew the bar: %q", out)
	}
	if !strings.Contains(out, "prompt-area line") {
		t.Fatalf("paused UI dropped the log line: %q", out)
	}

	buf.Reset()
	ui.Resume()
	if !strings.Contains(buf.String(), "%") {
		t.Fatalf("Resume did not redraw the bar: %q", buf.String())
	}
}

func TestTerminalProgressUIFinishErasesAndStops(t *testing.T) {
	var buf bytes.Buffer
	ui := newTerminalProgressUI(&buf)
	ui.Start("P")
	buf.Reset()
	ui.Finish()
	if !strings.Contains(buf.String(), eraseSeq) {
		t.Fatalf("Finish should erase the bar: %q", buf.String())
	}
	buf.Reset()
	ui.SetProgress(0.9, "")
	if buf.Len() != 0 {
		t.Fatalf("calls after Finish should be inert, got: %q", buf.String())
	}
}
