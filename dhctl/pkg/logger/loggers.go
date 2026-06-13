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
