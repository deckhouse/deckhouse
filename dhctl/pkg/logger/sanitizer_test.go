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
	"log/slog"
	"testing"
)

func TestSanitizeReplacesSensitiveMessage(t *testing.T) {
	in := slog.String(slog.MessageKey, `secret "kind":"Secret" payload`)
	out := Sanitize(nil, in)
	if out.Value.String() != `[FILTERED - "kind":"Secret"]` {
		t.Fatalf("got %q", out.Value.String())
	}
}

func TestSanitizeLeavesCleanMessage(t *testing.T) {
	in := slog.String(slog.MessageKey, "nothing sensitive here")
	out := Sanitize(nil, in)
	if out.Value.String() != "nothing sensitive here" {
		t.Fatalf("got %q", out.Value.String())
	}
}

func TestSanitizeRedactsNonMessageStringAttrs(t *testing.T) {
	in := slog.String("other", `"kind":"Secret"`)
	out := Sanitize(nil, in)
	if out.Value.String() != `[FILTERED - "kind":"Secret"]` {
		t.Fatalf("got %q", out.Value.String())
	}
}

func TestSanitizePassesThroughControlAttrs(t *testing.T) {
	// Control attributes are structural (time/level/source and renderer markers) and must be
	// returned untouched even when their value would otherwise look sensitive.
	cases := []string{
		slog.TimeKey, slog.LevelKey, slog.SourceKey,
		attrKeyCompact, attrKeyBadge, attrKeyBanner, attrKeyConnString,
		attrKeyProcessEvent, attrKeyProcessName,
		attrKeyProgressEvent, attrKeyProgressName, attrKeyProgressValue, attrKeyProgressTitle,
		attrKeyFileOnly,
	}
	for _, key := range cases {
		in := slog.String(key, `"kind":"Secret"`)
		out := Sanitize(nil, in)
		if out.Value.String() != `"kind":"Secret"` {
			t.Fatalf("control attr %q was redacted to %q", key, out.Value.String())
		}
	}
}

func TestSanitizeDescendsIntoGroups(t *testing.T) {
	// A sensitive value nested inside a group must still be redacted.
	in := slog.GroupAttrs("g",
		slog.String("inner", `dump "kind":"Secret" end`),
		slog.String("clean", "ok"),
		slog.GroupAttrs("g",
			slog.String("inner", `dump "kind":"Secret" end`),
			slog.String("clean", "ok"),
		),
	)
	out := Sanitize(nil, in)

	group := out.Value.Group()
	if len(group) != 3 {
		t.Fatalf("expected 3 children, got %d", len(group))
	}
	if group[0].Value.String() != `[FILTERED - "kind":"Secret"]` {
		t.Fatalf("inner not redacted: %q", group[0].Value.String())
	}
	if group[1].Value.String() != "ok" {
		t.Fatalf("clean child altered: %q", group[1].Value.String())
	}

	group = group[2].Value.Group()
	if len(group) != 2 {
		t.Fatalf("expected 2 children, got %d", len(group))
	}
	if group[0].Value.String() != `[FILTERED - "kind":"Secret"]` {
		t.Fatalf("inner not redacted: %q", group[0].Value.String())
	}
	if group[1].Value.String() != "ok" {
		t.Fatalf("clean child altered: %q", group[1].Value.String())
	}
}

func TestSanitizeLeavesNonStringScalars(t *testing.T) {
	// Bool/int/duration values cannot contain the keyword substrings and pass through unchanged.
	cases := []slog.Attr{
		slog.Int("count", 42),
		slog.Bool("ok", true),
		slog.Duration("elapsed", 5_000_000),
	}
	for _, in := range cases {
		out := Sanitize(nil, in)
		if out.Value.Any() != in.Value.Any() {
			t.Fatalf("scalar %v was altered to %v", in.Value.Any(), out.Value.Any())
		}
	}
}
