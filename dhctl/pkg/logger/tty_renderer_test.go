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
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeProgressUI records every method call for assertions.
type fakeProgressUI struct {
	calls       []string
	startName   string
	lastFrac    float64
	lastTitle   string
	writtenLine string
}

func (f *fakeProgressUI) Start(name string) {
	f.calls = append(f.calls, "Start")
	f.startName = name
}
func (f *fakeProgressUI) SetProgress(frac float64, title string) {
	f.calls = append(f.calls, "SetProgress")
	f.lastFrac = frac
	f.lastTitle = title
}
func (f *fakeProgressUI) SetAction(string) { f.calls = append(f.calls, "SetAction") }
func (f *fakeProgressUI) WriteLine(s string) {
	f.calls = append(f.calls, "WriteLine")
	f.writtenLine += s // accumulate so multi-line renders (e.g. nested boxes) are fully captured
}
func (f *fakeProgressUI) Finish() { f.calls = append(f.calls, "Finish") }
func (f *fakeProgressUI) Pause()  { f.calls = append(f.calls, "Pause") }
func (f *fakeProgressUI) Resume() { f.calls = append(f.calls, "Resume") }
func (f *fakeProgressUI) Resize() { f.calls = append(f.calls, "Resize") }

func (f *fakeProgressUI) has(call string) bool {
	for _, c := range f.calls {
		if c == call {
			return true
		}
	}
	return false
}

func newRecord(level slog.Level, msg string, attrs ...slog.Attr) slog.Record {
	r := slog.NewRecord(time.Now(), level, msg, 0)
	r.AddAttrs(attrs...)
	return r
}

func procStartRec(name string) slog.Record {
	return newRecord(slog.LevelInfo, "Starting: "+name,
		slog.String(attrKeyProcessEvent, string(processStart)),
		slog.String(attrKeyProcessName, name))
}

func procEndRec(name string) slog.Record {
	return newRecord(slog.LevelInfo, "Finished: "+name,
		slog.String(attrKeyProcessEvent, string(processEnd)),
		slog.String(attrKeyProcessName, name))
}

// --- progress bar lifecycle ---

func TestRendererStartOpensBar(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(&bytes.Buffer{}, ui, slog.LevelDebug, false, true)
	_ = rdr.Handle(context.Background(), newRecord(slog.LevelInfo, "p",
		slog.String(attrKeyProgressEvent, string(progressStart)),
		slog.String(attrKeyProgressName, "phase")))
	if !ui.has("Start") || ui.startName != "phase" {
		t.Fatalf("Start not handled: calls=%v", ui.calls)
	}
}

func TestRendererProgressValueAdvancesBar(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(&bytes.Buffer{}, ui, slog.LevelDebug, false, true)
	_ = rdr.Handle(context.Background(), newRecord(slog.LevelInfo, "x",
		slog.Float64(attrKeyProgressValue, 0.5), slog.String(attrKeyProgressTitle, "t")))
	if !ui.has("SetProgress") || ui.lastFrac != 0.5 || ui.lastTitle != "t" {
		t.Fatalf("SetProgress not handled: %v frac=%v title=%q", ui.calls, ui.lastFrac, ui.lastTitle)
	}
}

func TestRendererEndClosesBar(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(&bytes.Buffer{}, ui, slog.LevelDebug, false, true)
	_ = rdr.Handle(context.Background(), newRecord(slog.LevelInfo, "x",
		slog.String(attrKeyProgressEvent, string(progressEnd))))
	if !ui.has("Finish") {
		t.Fatalf("end not handled: %v", ui.calls)
	}
}

// --- process boxes ---
// All rendered lines (boxes + ordinary text) go through ui.WriteLine, so the pinned progress
// block stays consistent regardless of which (possibly .With()-derived) logger emitted them.

func TestRendererProcessOpensBox(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(&bytes.Buffer{}, ui, slog.LevelDebug, false, true)
	_ = rdr.Handle(context.Background(), procStartRec("deploy"))
	if !strings.Contains(ui.writtenLine, boxOpen+" deploy") {
		t.Fatalf("box-open not rendered: %q", ui.writtenLine)
	}
	if len(rdr.stack) != 1 {
		t.Fatalf("process not pushed: %d", len(rdr.stack))
	}
}

func TestRendererProcessClosesBoxWithDuration(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(&bytes.Buffer{}, ui, slog.LevelDebug, false, true)
	_ = rdr.Handle(context.Background(), procStartRec("deploy"))
	_ = rdr.Handle(context.Background(), procEndRec("deploy"))
	s := ui.writtenLine
	if !strings.Contains(s, boxClose+" deploy") || !strings.Contains(s, "seconds)") {
		t.Fatalf("box-close/duration not rendered: %q", s)
	}
	if len(rdr.stack) != 0 {
		t.Fatalf("process not popped: %d", len(rdr.stack))
	}
}

func TestRendererNestedProcessesIndent(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(&bytes.Buffer{}, ui, slog.LevelDebug, false, true)
	_ = rdr.Handle(context.Background(), procStartRec("outer"))
	_ = rdr.Handle(context.Background(), procStartRec("inner"))
	_ = rdr.Handle(context.Background(), newRecord(slog.LevelInfo, "body"))
	s := ui.writtenLine
	if !strings.Contains(s, boxBody+boxOpen+" inner") {
		t.Fatalf("nested box not indented: %q", s)
	}
	if !strings.Contains(s, boxBody+boxBody+"body") {
		t.Fatalf("nested body not indented two levels: %q", s)
	}
}

// --- ordinary lines ---

func TestRendererMultiLineMessageIndentsEveryLine(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(&bytes.Buffer{}, ui, slog.LevelDebug, false, true)
	_ = rdr.Handle(context.Background(), procStartRec("ng")) // depth 1 → prefix "│ "
	_ = rdr.Handle(context.Background(), newRecord(slog.LevelInfo, "NAME\nmaster\nworker"))
	for _, want := range []string{boxBody + "NAME", boxBody + "master", boxBody + "worker"} {
		if !strings.Contains(ui.writtenLine, want+"\n") {
			t.Fatalf("multi-line %q not indented per line: %q", want, ui.writtenLine)
		}
	}
}

func TestRendererOrdinaryLineGoesThroughUI(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(&bytes.Buffer{}, ui, slog.LevelDebug, false, true)
	_ = rdr.Handle(context.Background(), newRecord(slog.LevelInfo, "plainmsg"))
	if !ui.has("WriteLine") || !strings.Contains(ui.writtenLine, "plainmsg") {
		t.Fatalf("line not routed through ui: %q calls=%v", ui.writtenLine, ui.calls)
	}
}

func TestRendererLineScrollsAboveActiveBar(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(&bytes.Buffer{}, ui, slog.LevelDebug, false, true)
	_ = rdr.Handle(context.Background(), newRecord(slog.LevelInfo, "hello"))
	if !ui.has("WriteLine") || !strings.Contains(ui.writtenLine, "hello") {
		t.Fatalf("line not scrolled above bar: %q calls=%v", ui.writtenLine, ui.calls)
	}
}

func TestRendererBoxScrollsAboveActiveBar(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(&bytes.Buffer{}, ui, slog.LevelDebug, false, true)
	_ = rdr.Handle(context.Background(), procStartRec("deploy"))
	if !ui.has("WriteLine") || !strings.Contains(ui.writtenLine, boxOpen+" deploy") {
		t.Fatalf("box not scrolled above bar: %q", ui.writtenLine)
	}
}

func TestRendererPauseResume(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(&bytes.Buffer{}, ui, slog.LevelDebug, false, true)
	_ = rdr.Handle(context.Background(), newRecord(slog.LevelInfo, "x",
		slog.String(attrKeyProgressEvent, string(progressPause))))
	_ = rdr.Handle(context.Background(), newRecord(slog.LevelInfo, "x",
		slog.String(attrKeyProgressEvent, string(progressResume))))
	if !ui.has("Pause") || !ui.has("Resume") {
		t.Fatalf("pause/resume not handled: %v", ui.calls)
	}
}

func TestRendererRespectsLevel(t *testing.T) {
	rdr := newTTYRenderer(&bytes.Buffer{}, &fakeProgressUI{}, slog.LevelInfo, false, true)
	if rdr.Enabled(context.Background(), slog.LevelDebug) {
		t.Fatal("debug should be disabled at info level")
	}
	if !rdr.Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("info should be enabled")
	}
}

// TestRendererConcurrentHandleIsSerialized exercises the real-world wiring: log lines come from the
// operation goroutine while the progress bar is advanced from consumeProgress's goroutine. Both go
// through the same renderer. Run under -race; without the renderer's mutex this trips the detector
// on the shared ui/stack and, in production, corrupts the pinned block (blink, eaten messages).
func TestRendererConcurrentHandleIsSerialized(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(&bytes.Buffer{}, ui, slog.LevelDebug, false, false)
	_ = rdr.Handle(context.Background(), newRecord(slog.LevelInfo, "start",
		slog.String(attrKeyProgressEvent, string(progressStart)), slog.String(attrKeyProgressName, "p")))

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			_ = rdr.Handle(context.Background(), newRecord(slog.LevelInfo, "line",
				ShowInCompacted()))
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			_ = rdr.Handle(context.Background(), newRecord(slog.LevelInfo, "prog",
				slog.Float64(attrKeyProgressValue, 0.5), slog.String(attrKeyProgressTitle, "t")))
		}
	}()
	wg.Wait()
}

func TestRendererErrorStyledWithColor(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(&bytes.Buffer{}, ui, slog.LevelDebug, true, true)
	_ = rdr.Handle(context.Background(), newRecord(slog.LevelError, "boom"))
	s := ui.writtenLine
	if !strings.Contains(s, "boom") || !strings.Contains(s, "\x1b[") {
		t.Fatalf("error not ANSI-styled: %q", s)
	}
}
