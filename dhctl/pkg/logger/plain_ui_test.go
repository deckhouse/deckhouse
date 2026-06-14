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
	"testing"
)

// TestNonTerminalTTYWriterStaysPlain proves that when the TTY writer is a non-terminal buffer
// (isTTY=true but not a real *os.File terminal), ordinary ShowInCompacted()-tagged records render as plain
// text with no ANSI escape sequences, because the handler picks the plain sink fallback.
func TestNonTerminalTTYWriterStaysPlain(t *testing.T) {
	var file, tty bytes.Buffer
	l := slog.New(newTestHandler(&file, &tty, true))

	l.Info("hello terminal", ShowInCompacted())

	out := tty.String()
	if !strings.Contains(out, "hello terminal") {
		t.Fatalf("tty missing plain record: %q", out)
	}
	if strings.Contains(out, "\x1b[") {
		t.Fatalf("tty contains ANSI escape sequences, expected plain text: %q", out)
	}
}

// TestPlainProgressSequenceDoesNotCorrupt drives a full StartProgress -> Progress ->
// FinishProgress sequence through the plain sink and asserts it neither panics nor
// corrupts the buffer with control codes; ordinary lines emitted during the session survive.
func TestPlainProgressSequenceDoesNotCorrupt(t *testing.T) {
	var file, tty bytes.Buffer
	l := slog.New(newTestHandler(&file, &tty, true))
	ctx := context.Background()

	StartProgress(ctx, l, "bootstrap")
	Progress(ctx, l, 0.5, "halfway")
	l.Info("mid-progress line", ShowInCompacted())
	FinishProgress(ctx, l)

	out := tty.String()
	if strings.Contains(out, "\x1b[") {
		t.Fatalf("plain progress sequence emitted ANSI escapes: %q", out)
	}
	if !strings.Contains(out, "mid-progress line") {
		t.Fatalf("ordinary line emitted during progress session was lost: %q", out)
	}
}

// TestPlainSinkWritesLines confirms the plain sink writes Log/Warn/Milestone lines to its writer,
// newline-terminated and with no ANSI control. The plain backend has no bar, so the renderer (not
// the sink) absorbs progress markers — the sink itself only carries the line methods.
func TestPlainSinkWritesLines(t *testing.T) {
	var buf bytes.Buffer
	ui := newPlainSink(&buf)
	ui.Log("a line")
	ui.Warn("a warn")
	ui.Milestone("SUCCESS", "done")
	if buf.String() != "a line\na warn\nSUCCESS done\n" {
		t.Fatalf("plain sink corrupted output: %q", buf.String())
	}
}
