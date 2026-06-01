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
	"errors"
	"log/slog"
	"testing"
)

// capture is a tiny handler that records every Handle call.
type capture struct{ records []slog.Record }

func (c *capture) Enabled(context.Context, slog.Level) bool { return true }
func (c *capture) Handle(_ context.Context, r slog.Record) error {
	c.records = append(c.records, r)
	return nil
}
func (c *capture) WithAttrs([]slog.Attr) slog.Handler { return c }
func (c *capture) WithGroup(string) slog.Handler      { return c }

func eventsOf(c *capture) []string {
	var out []string
	for _, r := range c.records {
		r.Attrs(func(a slog.Attr) bool {
			if a.Key == attrKeyProcessEvent {
				out = append(out, a.Value.String())
			}
			return true
		})
	}
	return out
}

func TestRunProcessSuccessEmitsStartEnd(t *testing.T) {
	c := &capture{}
	l := slog.New(c)
	err := RunProcess(context.Background(), l, "deploy", func(context.Context) error { return nil })
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	got := eventsOf(c)
	if len(got) != 2 || got[0] != "start" || got[1] != "end" {
		t.Fatalf("events = %v, want [start end]", got)
	}
}

func TestRunProcessFailureEmitsStartFailAndReturnsErr(t *testing.T) {
	c := &capture{}
	l := slog.New(c)
	want := errors.New("boom")
	err := RunProcess(context.Background(), l, "deploy", func(context.Context) error { return want })
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
	got := eventsOf(c)
	if len(got) != 2 || got[0] != "start" || got[1] != "fail" {
		t.Fatalf("events = %v, want [start fail]", got)
	}
}

func TestRunProcessMarkersAreRendererMarkers(t *testing.T) {
	c := &capture{}
	l := slog.New(c)
	_ = RunProcess(context.Background(), l, "deploy", func(context.Context) error { return nil })
	for _, r := range c.records {
		if !isRendererMarker(r) {
			t.Fatalf("process record %q is not a renderer marker", r.Message)
		}
	}
}
