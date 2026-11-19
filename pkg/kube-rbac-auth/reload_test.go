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

package kuberbacauth

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// textReloader keeps the text of a file. The text "broken" cannot be loaded.
type textReloader struct {
	*reloader[string]
	file  string
	clock *fakeClock
	logs  *bytes.Buffer
	loads atomic.Int32
	// hold blocks the loads of the text "slow" until it is closed.
	hold    chan struct{}
	loading chan struct{}
}

func newTextReloader(t *testing.T, text string) *textReloader {
	t.Helper()
	r := &textReloader{
		file:    filepath.Join(t.TempDir(), "file"),
		clock:   &fakeClock{now: testNow},
		logs:    &bytes.Buffer{},
		hold:    make(chan struct{}),
		loading: make(chan struct{}, 1),
	}
	writeFile(t, r.file, []byte(text))
	load := func(contents [][]byte) (*string, error) {
		r.loads.Add(1)
		text := string(contents[0])
		switch text {
		case "broken":
			return nil, errors.New("cannot parse")
		case "slow":
			r.loading <- struct{}{}
			<-r.hold
		}
		return &text, nil
	}
	var err error
	r.reloader, err = newReloader("text", []string{r.file}, load, r.clock, slog.New(slog.NewTextHandler(r.logs, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func (r *textReloader) expect(t *testing.T, text string, version uint64) {
	t.Helper()
	got, gotVersion := r.get()
	if *got != text || gotVersion != version {
		t.Fatalf("value %q version %d, want %q version %d", *got, gotVersion, text, version)
	}
}

func TestReloaderReadsFilesOncePerInterval(t *testing.T) {
	r := newTextReloader(t, "one")
	r.expect(t, "one", 1)

	writeFile(t, r.file, []byte("two"))
	r.clock.step(reloadInterval - time.Second)
	r.expect(t, "one", 1)

	r.clock.step(time.Second)
	r.expect(t, "two", 2)

	// The next read waits for the interval again.
	writeFile(t, r.file, []byte("three"))
	r.clock.step(reloadInterval - time.Second)
	r.expect(t, "two", 2)

	r.clock.step(time.Second)
	r.expect(t, "three", 3)
}

func TestReloaderReadsFilesAfterClockIsSetBack(t *testing.T) {
	r := newTextReloader(t, "one")

	writeFile(t, r.file, []byte("two"))
	r.clock.step(-time.Hour)
	r.expect(t, "two", 2)

	// The interval is counted from the new time.
	writeFile(t, r.file, []byte("three"))
	r.clock.step(reloadInterval - time.Second)
	r.expect(t, "two", 2)

	r.clock.step(time.Second)
	r.expect(t, "three", 3)
}

func TestReloaderComparesContent(t *testing.T) {
	r := newTextReloader(t, "one")
	info, err := os.Stat(r.file)
	if err != nil {
		t.Fatal(err)
	}

	// New content of the same size with the old modification time is loaded.
	writeFile(t, r.file, []byte("two"))
	if err := os.Chtimes(r.file, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	r.clock.step(reloadInterval)
	r.expect(t, "two", 2)

	// The same content with a new modification time is not loaded again.
	later := info.ModTime().Add(time.Hour)
	if err := os.Chtimes(r.file, later, later); err != nil {
		t.Fatal(err)
	}
	r.clock.step(reloadInterval)
	r.expect(t, "two", 2)
	if n := r.loads.Load(); n != 2 {
		t.Fatalf("loads = %d, want 2", n)
	}
}

func TestReloaderKeepsPreviousValueAndReportsOnce(t *testing.T) {
	r := newTextReloader(t, "one")
	warnings := func() int { return strings.Count(r.logs.String(), "cannot reload the text, using the previous one") }

	writeFile(t, r.file, []byte("broken"))
	for range 3 {
		r.clock.step(reloadInterval)
		r.expect(t, "one", 1)
	}
	// The broken content is loaded and reported once, not on every read.
	if n := r.loads.Load(); n != 2 {
		t.Fatalf("loads = %d, want 2", n)
	}
	if n := warnings(); n != 1 {
		t.Fatalf("warnings = %d, want 1; log %q", n, r.logs.String())
	}

	// A missing file is another error and is reported once too.
	if err := os.Remove(r.file); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		r.clock.step(reloadInterval)
		r.expect(t, "one", 1)
	}
	if n := warnings(); n != 2 {
		t.Fatalf("warnings = %d, want 2; log %q", n, r.logs.String())
	}

	// The same broken content after another error is reported again, without loading it again.
	writeFile(t, r.file, []byte("broken"))
	r.clock.step(reloadInterval)
	r.expect(t, "one", 1)
	if n := warnings(); n != 3 {
		t.Fatalf("warnings = %d, want 3; log %q", n, r.logs.String())
	}
	if n := r.loads.Load(); n != 2 {
		t.Fatalf("loads = %d, want 2", n)
	}

	writeFile(t, r.file, []byte("two"))
	r.clock.step(reloadInterval)
	r.expect(t, "two", 2)

	// An error that was reported before a successful load is reported again.
	writeFile(t, r.file, []byte("broken"))
	r.clock.step(reloadInterval)
	r.expect(t, "two", 2)
	if n := warnings(); n != 4 {
		t.Fatalf("warnings = %d, want 4; log %q", n, r.logs.String())
	}
}

func TestReloaderDoesNotMakeCallersWait(t *testing.T) {
	r := newTextReloader(t, "one")
	writeFile(t, r.file, []byte("slow"))
	r.clock.step(reloadInterval)

	reloaded := make(chan string, 1)
	go func() {
		text, _ := r.get()
		reloaded <- *text
	}()
	<-r.loading

	// The load is held. Another caller gets the previous value at once, also when the next read is due.
	for range 2 {
		got := make(chan string, 1)
		go func() {
			text, _ := r.get()
			got <- *text
		}()
		select {
		case text := <-got:
			if text != "one" {
				t.Fatalf("value = %q, want the previous one", text)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("a caller waited for the load of another caller")
		}
		r.clock.step(reloadInterval)
	}

	close(r.hold)
	if text := <-reloaded; text != "slow" {
		t.Fatalf("value = %q, want the loaded one", text)
	}
	r.expect(t, "slow", 2)
}

func TestNewReloaderNeedsLoadableFiles(t *testing.T) {
	load := func([][]byte) (*string, error) { return nil, errors.New("cannot parse") }
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	file := filepath.Join(t.TempDir(), "file")

	if _, err := newReloader("text", []string{file}, load, &fakeClock{now: testNow}, logger); err == nil || !strings.HasPrefix(err.Error(), "read the text: ") {
		t.Fatalf("absent file: err = %v", err)
	}
	writeFile(t, file, []byte("text"))
	if _, err := newReloader("text", []string{file}, load, &fakeClock{now: testNow}, logger); err == nil || !strings.HasPrefix(err.Error(), "load the text: ") {
		t.Fatalf("broken file: err = %v", err)
	}
}
