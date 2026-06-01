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
	"time"
)

func TestTTYAttr(t *testing.T) {
	a := ShowInCompacted()
	if a.Key != attrKeyCompact {
		t.Fatalf("key = %q, want %q", a.Key, attrKeyCompact)
	}
	if a.Value.Kind() != slog.KindBool || !a.Value.Bool() {
		t.Fatalf("value = %v, want bool true", a.Value)
	}
}

func TestHasTTY(t *testing.T) {
	r := slog.NewRecord(time.Time{}, slog.LevelInfo, "x", 0)
	if hasShowInCompacted(r) {
		t.Fatal("expected no TTY attr")
	}
	r.AddAttrs(ShowInCompacted())
	if !hasShowInCompacted(r) {
		t.Fatal("expected TTY attr present")
	}
}

func TestProcessMarkers(t *testing.T) {
	for _, ev := range []processEvent{processStart, processEnd, processFail} {
		a := processAttr(ev, "name")
		if a[0].Key != attrKeyProcessEvent || a[0].Value.String() != string(ev) {
			t.Fatalf("event attr wrong: %v", a)
		}
		if a[1].Key != attrKeyProcessName || a[1].Value.String() != "name" {
			t.Fatalf("name attr wrong: %v", a)
		}
	}
}
