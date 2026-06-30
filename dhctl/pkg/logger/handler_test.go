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

func TestHandlerSanitizesSensitiveMessageOnBothSinks(t *testing.T) {
	var file, tty bytes.Buffer
	l := slog.New(newTestHandler(&file, &tty, true))
	// The full message is replaced with a redaction marker; the unique token "leakprefix"
	// must not survive on either sink, while the FILTERED marker must appear on both.
	l.Info(`leakprefix "kind":"Secret"`, ShowInCompacted())

	for name, sink := range map[string]*bytes.Buffer{"file": &file, "tty": &tty} {
		if !strings.Contains(sink.String(), "[FILTERED") {
			t.Fatalf("%s sink not sanitized: %q", name, sink.String())
		}
		if strings.Contains(sink.String(), "leakprefix") {
			t.Fatalf("%s sink leaked raw message: %q", name, sink.String())
		}
	}
}

func newTestHandler(file, tty *bytes.Buffer, isTTY bool) *TerminalUIHandler {
	lv := new(slog.LevelVar)
	lv.Set(slog.LevelDebug)
	return newTerminalUIHandler(handlerConfig{
		fileW:       file,
		ttyW:        tty,
		isTTY:       isTTY,
		interactive: true,
		level:       lv,
	})
}

func TestHandlerFileGetsEveryRecord(t *testing.T) {
	var file, tty bytes.Buffer
	l := slog.New(newTestHandler(&file, &tty, true))
	l.Info("untagged message")
	if !strings.Contains(file.String(), "untagged message") {
		t.Fatalf("file missing record: %q", file.String())
	}
}

func TestHandlerUntaggedRecordNotOnTTY(t *testing.T) {
	var file, tty bytes.Buffer
	l := slog.New(newTestHandler(&file, &tty, true))
	l.Info("file only")
	if tty.Len() != 0 {
		t.Fatalf("tty should be empty, got %q", tty.String())
	}
}

func TestHandlerTaggedRecordOnTTY(t *testing.T) {
	var file, tty bytes.Buffer
	l := slog.New(newTestHandler(&file, &tty, true))
	l.Info("to terminal", ShowInCompacted())
	if !strings.Contains(tty.String(), "to terminal") {
		t.Fatalf("tty missing record: %q", tty.String())
	}
	if !strings.Contains(file.String(), "to terminal") {
		t.Fatalf("file must still get tagged record: %q", file.String())
	}
}

func TestHandlerNonTTYNeverWritesTerminal(t *testing.T) {
	var file, tty bytes.Buffer
	l := slog.New(newTestHandler(&file, &tty, false))
	l.Info("tagged but no tty", ShowInCompacted())
	if tty.Len() != 0 {
		t.Fatalf("tty must be empty on non-tty, got %q", tty.String())
	}
	if !strings.Contains(file.String(), "tagged but no tty") {
		t.Fatalf("file must still get record: %q", file.String())
	}
}

func TestHandlerWithTTYAttrRoutesToTerminal(t *testing.T) {
	var file, tty bytes.Buffer
	l := slog.New(newTestHandler(&file, &tty, true)).With(ShowInCompacted())
	l.Info("via with")
	if !strings.Contains(tty.String(), "via with") {
		t.Fatalf("tty missing record tagged via With: %q", tty.String())
	}
}

func TestHandlerWithGroupKeepsTTYTag(t *testing.T) {
	var file, tty bytes.Buffer
	l := slog.New(newTestHandler(&file, &tty, true)).With(ShowInCompacted()).WithGroup("g")
	l.Info("grouped")
	if !strings.Contains(tty.String(), "grouped") {
		t.Fatalf("tty missing grouped record after WithGroup: %q", tty.String())
	}
}

func TestHandlerVerboseShowsUntaggedOnTTY(t *testing.T) {
	var file, tty bytes.Buffer
	lv := new(slog.LevelVar)
	lv.Set(slog.LevelDebug)
	l := slog.New(newTerminalUIHandler(handlerConfig{
		fileW: &file, ttyW: &tty, isTTY: true, interactive: true, level: lv, verbose: true,
	})) // verbose
	l.Info("untagged but verbose")
	if !strings.Contains(tty.String(), "untagged but verbose") {
		t.Fatalf("verbose tty should show untagged record: %q", tty.String())
	}
}

func TestHandlerFileOnlyErrorSuppressedOnCompactTTY(t *testing.T) {
	// Streamed lib-connection output tagged FileOnly stays off the compact terminal even at Error
	// level (it would flood), but the file always keeps it.
	var file, tty bytes.Buffer
	l := slog.New(newTestHandler(&file, &tty, true))
	l.LogAttrs(context.Background(), slog.LevelError, "bashible set -x spam", FileOnly())
	if strings.Contains(tty.String(), "bashible set -x spam") {
		t.Fatalf("file-only error leaked to compact tty: %q", tty.String())
	}
	if !strings.Contains(file.String(), "bashible set -x spam") {
		t.Fatalf("file must keep file-only record: %q", file.String())
	}
}

func TestHandlerFileOnlyShownWhenVerbose(t *testing.T) {
	var file, tty bytes.Buffer
	lv := new(slog.LevelVar)
	lv.Set(slog.LevelDebug)
	l := slog.New(newTerminalUIHandler(handlerConfig{
		fileW: &file, ttyW: &tty, isTTY: true, interactive: true, level: lv, verbose: true,
	})) // verbose
	l.LogAttrs(context.Background(), slog.LevelError, "verbose sees stream", FileOnly())
	if !strings.Contains(tty.String(), "verbose sees stream") {
		t.Fatalf("verbose must show file-only record: %q", tty.String())
	}
}

func TestHandlerEnabledHonoursLevel(t *testing.T) {
	var file, tty bytes.Buffer
	lv := new(slog.LevelVar)
	lv.Set(slog.LevelInfo)
	h := newTerminalUIHandler(handlerConfig{
		fileW: &file, ttyW: &tty, isTTY: true, interactive: true, level: lv,
	})
	if h.Enabled(context.Background(), slog.LevelDebug) {
		t.Fatal("debug should be disabled at info level")
	}
	if !h.Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("info should be enabled")
	}
}
