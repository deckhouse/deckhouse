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
	"io"
	"log/slog"
)

type discardHandler struct{}

func (discardHandler) Enabled(context.Context, slog.Level) bool  { return false }
func (discardHandler) Handle(context.Context, slog.Record) error { return nil }
func (discardHandler) WithAttrs([]slog.Attr) slog.Handler        { return discardHandler{} }
func (discardHandler) WithGroup(string) slog.Handler             { return discardHandler{} }

// Discard returns a logger that drops every record. Replaces NewSilentLogger / NewDummyLogger.
func Discard() *slog.Logger { return slog.New(discardHandler{}) }

// NewBufferLogger returns a logger writing every record to w as text. Replaces BufferLogger.
func NewBufferLogger(w io.Writer) *slog.Logger {
	lv := new(slog.LevelVar)
	lv.Set(slog.LevelDebug)
	return slog.New(slog.NewTextHandler(w, handlerOptions(lv)))
}

// NewStreamLogger returns a logger that renders the compact UI (process boxes framed with ┌/│/└,
// milestones, banner, connection string) as plain ANSI-free lines to w — the format the commander
// client expects, instead of raw slog text. It backs the gRPC client-stream logger.
//
// Unlike NewRoot it does not register a global rootHandler, so it is safe to build per request and
// for concurrent operations. There is no file (JSON) sink: w is a server LogWriter that already
// logs every rendered line to the server slog and forwards it to the client. verbose forwards every
// Info+ detail line (e.g. terraform output) to the renderer; DEBUG never reaches the terminal sink.
func NewStreamLogger(w io.Writer) *slog.Logger {
	lv := new(slog.LevelVar)
	lv.Set(slog.LevelDebug)
	h := newTerminalUIHandler(handlerConfig{
		fileW:       io.Discard, // no JSON file sink on the stream path
		ttyW:        w,          // not an *os.File → real=false → plainSink (logboek tree)
		isTTY:       true,       // enable the rendering sink
		interactive: false,      // no pinned pterm block
		level:       lv,
		verbose:     true, // forward every Info+ detail line; DEBUG still excluded by the Info floor
	})
	return slog.New(h)
}
