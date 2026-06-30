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
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"
)

func testOpts() Options {
	return Options{
		Color:  false,
		Caps:   caps{warn: 5, logboxMin: 3},
		now:    func() time.Time { return time.Unix(0, 0) },
		width:  func() int { return 80 },
		height: func() int { return 40 },
		tick:   make(chan time.Time),
		resize: make(chan struct{}),
	}
}

func TestBlockStartEntersAltScreen(t *testing.T) {
	var buf bytes.Buffer
	b := New(&buf, testOpts())
	b.Start("Phase")
	defer b.Finish()
	out := buf.String()
	if !strings.HasPrefix(out, ansiEnterAlt+ansiHideCur) {
		t.Fatalf("Start must enter alt screen + hide cursor, got %q", out[:min(20, len(out))])
	}
	if !strings.Contains(out, "Phase") {
		t.Fatalf("Start must draw the bar with the title: %q", out)
	}
}

func TestBlockFinishRestoresAndDumps(t *testing.T) {
	var buf bytes.Buffer
	b := New(&buf, testOpts())
	b.Start("Phase")
	b.Milestone("SUCCESS", "did a thing")
	buf.Reset()
	b.Finish()
	out := buf.String()
	if !strings.Contains(out, ansiShowCur) || !strings.Contains(out, ansiLeaveAlt) {
		t.Fatalf("Finish must restore terminal: %q", out)
	}
	if !strings.Contains(out, "did a thing") {
		t.Fatalf("Finish must dump milestone history to main screen: %q", out)
	}
	buf.Reset()
	b.Finish()
	if buf.Len() != 0 {
		t.Fatalf("second Finish must be a no-op, got %q", buf.String())
	}
}

func TestBlockContentMethodsRepaint(t *testing.T) {
	var buf bytes.Buffer
	b := New(&buf, testOpts())
	b.Start("P")
	defer b.Finish()

	buf.Reset()
	b.SetProgress(0.5, "Phase X")
	if !strings.Contains(buf.String(), "Phase X") || !strings.Contains(buf.String(), "50%") {
		t.Fatalf("SetProgress did not repaint bar: %q", buf.String())
	}

	buf.Reset()
	b.SetAction("doing things")
	if !strings.Contains(buf.String(), "Current action: doing things") {
		t.Fatalf("SetAction did not repaint: %q", buf.String())
	}

	buf.Reset()
	b.Log("detail line")
	if !strings.Contains(buf.String(), "detail line") {
		t.Fatalf("Log did not appear in logbox: %q", buf.String())
	}
}

func TestBlockLogboxIsEphemeralRing(t *testing.T) {
	var buf bytes.Buffer
	b := New(&buf, testOpts())
	for i := 0; i < logboxRingCap+5; i++ {
		b.Log("x")
	}
	b.mu.Lock()
	n := len(b.logbox)
	b.mu.Unlock()
	if n != logboxRingCap {
		t.Fatalf("logbox ring must cap at %d, got %d", logboxRingCap, n)
	}
}

func TestBlockTickerAdvancesSpinner(t *testing.T) {
	var buf bytes.Buffer
	opts := testOpts()
	tick := make(chan time.Time, 1)
	opts.tick = tick
	b := New(&buf, opts)
	b.Start("P")
	defer b.Finish()

	b.mu.Lock()
	first := b.spinnerIdx
	b.mu.Unlock()
	tick <- time.Unix(1, 0)
	waitFor(t, func() bool {
		b.mu.Lock()
		defer b.mu.Unlock()
		return b.spinnerIdx != first
	})
}

func TestBlockPauseErasesAndResumeRedraws(t *testing.T) {
	var buf bytes.Buffer
	b := New(&buf, testOpts())
	b.Start("P")
	buf.Reset()
	b.Pause()
	if !strings.Contains(buf.String(), ansiShowCur) {
		t.Fatalf("Pause must show the cursor for input: %q", buf.String())
	}
	buf.Reset()
	b.Resume()
	if !strings.Contains(buf.String(), ansiHideCur) || !strings.Contains(buf.String(), "P") {
		t.Fatalf("Resume must hide cursor and redraw: %q", buf.String())
	}
}

func TestBlockPausedOutputIsTransientNotBuffered(t *testing.T) {
	// During a pause (interactive y/n prompt) the live block must step aside: lines print straight
	// to the terminal so the prompt is visible, but they are NOT redrawn as a pinned block (which
	// would bury the prompt) and NOT buffered (which would leave the answered prompt lingering).
	var buf bytes.Buffer
	b := New(&buf, testOpts())
	b.Start("P")
	b.Pause()
	buf.Reset()

	b.Warn("Continue? [y/n]: ")
	b.Log("background detail")
	out := buf.String()

	if !strings.Contains(out, "Continue? [y/n]: ") {
		t.Fatalf("paused prompt must be printed transiently for input: %q", out)
	}
	if strings.Contains(out, ansiHome) {
		t.Fatalf("paused output must not trigger a block repaint (would bury the prompt): %q", out)
	}
	b.mu.Lock()
	w, lb := len(b.warnsAll), len(b.logbox)
	b.mu.Unlock()
	if w != 0 || lb != 0 {
		t.Fatalf("paused output must not be buffered into the block (warns=%d logbox=%d)", w, lb)
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("condition not met in time")
}

// syncBuf is a bytes.Buffer guarded by a mutex, safe for concurrent
// reads (String) and writes (Write/WriteString/Reset).
type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuf) WriteString(str string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.WriteString(str)
}

func (s *syncBuf) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.b.Reset()
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func TestBlockRestoreIsIdempotentAndLeavesAltScreen(t *testing.T) {
	var buf bytes.Buffer
	b := New(&buf, testOpts())
	b.Start("P")
	buf.Reset()
	b.Restore()
	if !strings.Contains(buf.String(), ansiLeaveAlt) {
		t.Fatalf("Restore must leave the alt screen: %q", buf.String())
	}
	buf.Reset()
	b.Restore() // idempotent
	if buf.Len() != 0 {
		t.Fatalf("second Restore must be a no-op, got %q", buf.String())
	}
}

func TestBlockBannerShownWhenTallDroppedWhenShort(t *testing.T) {
	var buf bytes.Buffer
	opts := testOpts()
	h := 40
	opts.height = func() int { return h }
	b := New(&buf, opts)
	b.SetBanner([]string{"=== logo ==="})
	b.Start("P")
	defer b.Finish()
	if !strings.Contains(buf.String(), "=== logo ===") {
		t.Fatalf("tall terminal must show banner: %q", buf.String())
	}
	h = 6 // too short for banner + full region
	buf.Reset()
	b.SetProgress(0.1, "") // trigger a repaint
	if strings.Contains(buf.String(), "=== logo ===") {
		t.Fatalf("short terminal must drop banner: %q", buf.String())
	}
}

func TestBlockFinishDumpsBanner(t *testing.T) {
	var buf bytes.Buffer
	b := New(&buf, testOpts())
	b.SetBanner([]string{"=== logo ==="})
	b.Start("P")
	b.SetConnString("ssh user@host")
	buf.Reset()
	b.Finish()
	out := buf.String()
	if !strings.Contains(out, "=== logo ===") || !strings.Contains(out, "ssh user@host") {
		t.Fatalf("Finish dump must contain banner + connection string: %q", out)
	}
}

func TestBlockConnStringPinnedAndDumped(t *testing.T) {
	var buf bytes.Buffer
	b := New(&buf, testOpts())
	b.Start("P")
	b.SetConnString("ssh user@host")
	// Many milestones must NOT push the conn string out of the live view.
	for i := 0; i < 30; i++ {
		b.Milestone("SUCCESS", "phase")
	}
	if !strings.Contains(buf.String(), "ssh user@host") {
		t.Fatalf("conn string must stay pinned in the live region: %q", buf.String())
	}
	buf.Reset()
	b.Finish()
	if !strings.Contains(buf.String(), "ssh user@host") {
		t.Fatalf("conn string must be in the closing dump: %q", buf.String())
	}
}

func TestBlockResizeRepaints(t *testing.T) {
	var buf syncBuf
	opts := testOpts()
	rz := make(chan struct{}, 1)
	opts.resize = rz
	h := 40
	opts.height = func() int { return h }
	b := New(&buf, opts)
	b.Start("P")
	defer b.Finish()
	for i := 0; i < 11; i++ {
		b.Log("x")
	}
	h = 8 // shrink so the logbox is dropped
	buf.Reset()
	rz <- struct{}{}
	waitFor(t, func() bool { return strings.Contains(buf.String(), ansiHome) })
}

func TestBlockRestoreSurfacesErrors(t *testing.T) {
	var buf bytes.Buffer
	b := New(&buf, testOpts())
	b.Start("P")
	b.Warn("boom: cache exhausted") // ERROR-level lines route through Warn
	buf.Reset()
	b.Restore() // fatal/safety teardown must still surface the error on exit
	if !strings.Contains(buf.String(), "boom: cache exhausted") {
		t.Fatalf("Restore must surface buffered errors/warns on exit: %q", buf.String())
	}
}

// A Warn/Error logged AFTER the block has summarized (e.g. the top-level error reason printed once a
// failed operation unwinds to main, after RunProgress's deferred finish() already closed the block)
// must print straight to the now-main-screen writer instead of vanishing into the dumped ring.
func TestBlockWarnAfterSummaryPrintsDirectly(t *testing.T) {
	var buf bytes.Buffer
	b := New(&buf, testOpts())
	b.Start("P")
	b.Finish() // closes the live UI and prints the summary
	buf.Reset()
	b.Warn(`preflight check "always-fail" failed`)
	if !strings.Contains(buf.String(), `preflight check "always-fail" failed`) {
		t.Fatalf("post-summary Warn must print directly to the main screen: %q", buf.String())
	}
}
