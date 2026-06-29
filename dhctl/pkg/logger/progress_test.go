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
	"log/slog"
	"testing"
)

func lastRecord(c *capture) slog.Record {
	return c.records[len(c.records)-1]
}

func attrString(r slog.Record, key string) (string, bool) {
	var val string
	var found bool
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == key {
			val = a.Value.String()
			found = true
			return false
		}
		return true
	})
	return val, found
}

func TestStartProgressEmitsStartMarkerWithTTY(t *testing.T) {
	c := &capture{}
	l := slog.New(c)
	StartProgress(context.Background(), l, "bootstrap")

	r := lastRecord(c)
	if !isRendererMarker(r) {
		t.Fatal("StartProgress record not a renderer marker")
	}
	if got := progressEvent(r); got != "start" {
		t.Fatalf("progressEvent = %q, want start", got)
	}
	if name, _ := attrString(r, attrKeyProgressName); name != "bootstrap" {
		t.Fatalf("progress_name = %q, want bootstrap", name)
	}
}

func TestFinishProgressEmitsEndMarker(t *testing.T) {
	c := &capture{}
	l := slog.New(c)
	FinishProgress(context.Background(), l)

	r := lastRecord(c)
	if !isRendererMarker(r) {
		t.Fatal("FinishProgress record not a renderer marker")
	}
	if got := progressEvent(r); got != "end" {
		t.Fatalf("progressEvent = %q, want end", got)
	}
}

func TestPauseResumeProgressEmitMarkers(t *testing.T) {
	c := &capture{}
	l := slog.New(c)
	PauseProgress(context.Background(), l)
	if got := progressEvent(lastRecord(c)); got != "pause" {
		t.Fatalf("progressEvent = %q, want pause", got)
	}
	ResumeProgress(context.Background(), l)
	if got := progressEvent(lastRecord(c)); got != "resume" {
		t.Fatalf("progressEvent = %q, want resume", got)
	}
}

func TestProgressEmitsValueAndTitle(t *testing.T) {
	c := &capture{}
	l := slog.New(c)
	Progress(context.Background(), l, 0.42, "phase one")

	r := lastRecord(c)
	if !isRendererMarker(r) {
		t.Fatal("Progress record not a renderer marker")
	}
	v, ok := progressValue(r)
	if !ok {
		t.Fatal("progressValue not found")
	}
	if v != 0.42 {
		t.Fatalf("progressValue = %v, want 0.42", v)
	}
	if got := progressTitle(r); got != "phase one" {
		t.Fatalf("progressTitle = %q, want phase one", got)
	}
}

func TestProgressValueAbsentReturnsFalse(t *testing.T) {
	c := &capture{}
	l := slog.New(c)
	StartProgress(context.Background(), l, "x")
	if _, ok := progressValue(lastRecord(c)); ok {
		t.Fatal("progressValue should be absent on a start marker")
	}
}
