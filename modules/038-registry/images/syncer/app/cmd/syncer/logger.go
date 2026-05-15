/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"
)

type handlerOptions struct {
	level      slog.Level
	timeFormat string
	showLevel  bool
}

type handler struct {
	opts  handlerOptions
	attrs []slog.Attr
	group string
	w     io.Writer
}

func newHandler(w io.Writer, opts handlerOptions) slog.Handler {
	return &handler{
		opts: opts,
		w:    w,
	}
}

func (h *handler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.opts.level
}

func (h *handler) Handle(_ context.Context, r slog.Record) error {
	var parts []string

	if h.opts.timeFormat != "" {
		parts = append(parts, r.Time.Format(h.opts.timeFormat))
	}

	if h.opts.showLevel {
		parts = append(parts, r.Level.String())
	}

	parts = append(parts, r.Message)

	for _, a := range h.attrs {
		parts = append(parts, formatAttr(a))
	}

	r.Attrs(func(a slog.Attr) bool {
		parts = append(parts, formatAttr(a))
		return true
	})

	_, err := fmt.Fprintln(h.w, strings.Join(parts, " "))
	return err
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)
	return &handler{
		opts:  h.opts,
		attrs: newAttrs,
		group: h.group,
		w:     h.w,
	}
}

func (h *handler) WithGroup(name string) slog.Handler {
	return &handler{
		opts:  h.opts,
		attrs: h.attrs,
		group: name,
		w:     h.w,
	}
}

func formatAttr(a slog.Attr) string {
	return fmt.Sprintf("%s=%v", a.Key, a.Value)
}

func logLevelFromString(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func newLogger(w io.Writer) *slog.Logger {
	opts := handlerOptions{
		level: logLevelFromString(
			os.Getenv("LOG_LEVEL"),
		),
	}

	if os.Getenv("SHOW_LOG_LEVEL") == "true" {
		opts.showLevel = true
	}

	if os.Getenv("SHOW_LOG_TIMESTAMP") == "true" {
		opts.timeFormat = time.StampMilli
	}
	return slog.New(newHandler(w, opts))
}
