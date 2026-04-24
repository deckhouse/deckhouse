// Copyright 2024 Flant JSC
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

package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	logContext "github.com/deckhouse/deckhouse/pkg/log/context"
)

var _ Handler = (*handler)(nil)

// handler builds LogOutput directly from slog.Record attributes,
// avoiding the serialize-deserialize-reserialize roundtrip that
// the previous implementation used (inner slog.JSONHandler → buffer
// → json.Unmarshal → LogOutput → Encoder).
type handler struct {
	w       io.Writer
	mu      *sync.Mutex
	name    string
	timeFn  func(time.Time) time.Time
	encoder encoder

	addSource    *atomic.Bool
	sourceFormat string
	binaryPath   string
	level        slog.Leveler

	preAttrs []preAttrGroup
	groups   []string
}

// preAttrGroup stores attrs together with the group path that was
// active when they were bound via WithAttrs, so they get nested
// at the correct level in the output.
type preAttrGroup struct {
	groups []string
	attrs  []slog.Attr
}

type handlerConfig struct {
	output       io.Writer
	timeFn       func(time.Time) time.Time
	encoder      encoder
	addSource    *atomic.Bool
	level        slog.Leveler
	sourceFormat string
	binaryPath   string
}

func newHandler(cfg handlerConfig) *handler {
	return &handler{
		w:            cfg.output,
		mu:           &sync.Mutex{},
		timeFn:       cfg.timeFn,
		encoder:      cfg.encoder,
		addSource:    cfg.addSource,
		level:        cfg.level,
		sourceFormat: cfg.sourceFormat,
		binaryPath:   cfg.binaryPath,
	}
}

func (h *handler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

func (h *handler) Handle(ctx context.Context, r slog.Record) error {
	var tracePtr *string

	if logContext.GetCustomKeyContext(ctx) {
		var pcs [1]uintptr
		runtime.Callers(5, pcs[:])
		r.PC = pcs[0]
		tracePtr = logContext.GetStackTraceContext(ctx)
	}

	output := &LogOutput{
		Level:   Level(r.Level).String(),
		Time:    h.timeFn(r.Time).Format(time.RFC3339),
		Message: r.Message,
		Name:    h.name,
	}

	if tracePtr != nil {
		output.Stacktrace = *tracePtr
	} else if h.addSource != nil && h.addSource.Load() && r.PC != 0 {
		fs := runtime.CallersFrames([]uintptr{r.PC})
		f, _ := fs.Next()
		file := f.File
		if h.binaryPath != "" {
			file = strings.TrimPrefix(file, h.binaryPath)
			if len(file) > 0 && file[0] == '/' {
				file = file[1:]
			}
		}
		output.Source = fmt.Sprintf(h.sourceFormat, file, f.Line)
	}

	totalAttrs := r.NumAttrs()
	for _, ag := range h.preAttrs {
		totalAttrs += len(ag.attrs)
	}

	if totalAttrs > 0 {
		fields := make(map[string]any, totalAttrs)
		for _, ag := range h.preAttrs {
			for _, a := range ag.attrs {
				addAttrToFields(fields, ag.groups, a)
			}
		}
		r.Attrs(func(a slog.Attr) bool {
			addAttrToFields(fields, h.groups, a)
			return true
		})
		if len(fields) > 0 {
			output.Fields = fields
		}
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	return h.encoder.Encode(h.w, output)
}

func (h *handler) clone() *handler {
	h2 := *h
	h2.preAttrs = make([]preAttrGroup, len(h.preAttrs))
	copy(h2.preAttrs, h.preAttrs)
	h2.groups = make([]string, len(h.groups))
	copy(h2.groups, h.groups)
	return &h2
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) < 1 {
		return h
	}
	h2 := h.clone()
	h2.preAttrs = append(h2.preAttrs, preAttrGroup{
		groups: h.groups,
		attrs:  attrs,
	})
	return h2
}

func (h *handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	h2 := h.clone()
	h2.groups = append(h2.groups, name)
	return h2
}

func (h *handler) Named(name string) slog.Handler {
	currName := name
	if h.name != "" {
		currName = fmt.Sprintf("%s.%s", h.name, name)
	}
	h2 := h.clone()
	h2.name = currName
	return h2
}

func (h *handler) SetOutput(w io.Writer) {
	h.mu.Lock()
	h.w = w
	h.mu.Unlock()
}

// addAttrToFields places a resolved slog.Attr into the fields map
// at the nesting level indicated by groups.
func addAttrToFields(m map[string]any, groups []string, a slog.Attr) {
	a.Value = a.Value.Resolve()
	if a.Equal(slog.Attr{}) {
		return
	}

	target := m
	for _, g := range groups {
		sub, ok := target[g].(map[string]any)
		if !ok {
			sub = make(map[string]any)
			target[g] = sub
		}
		target = sub
	}

	if a.Value.Kind() == slog.KindGroup {
		if a.Key != "" {
			sub := make(map[string]any)
			target[a.Key] = sub
			target = sub
		}
		for _, ga := range a.Value.Group() {
			addAttrToFields(target, nil, ga)
		}
		return
	}

	target[a.Key] = attrValue(a.Value)
}

func attrValue(v slog.Value) any {
	switch v.Kind() {
	case slog.KindString:
		return v.String()
	case slog.KindInt64:
		return v.Int64()
	case slog.KindUint64:
		return v.Uint64()
	case slog.KindFloat64:
		return v.Float64()
	case slog.KindBool:
		return v.Bool()
	case slog.KindDuration:
		return v.Duration().String()
	case slog.KindTime:
		return v.Time().Format(time.RFC3339Nano)
	case slog.KindAny:
		return v.Any()
	default:
		return v.String()
	}
}

// Backward-compatible type aliases.
type SlogJSONHandler = handler
type SlogTextHandler = handler

// NewJSONHandler creates a handler that outputs JSON-formatted log lines.
func NewJSONHandler(out io.Writer, opts *slog.HandlerOptions, timeFn func(t time.Time) time.Time) *handler {
	addSource := &atomic.Bool{}
	var level slog.Leveler = slog.LevelInfo
	if opts != nil {
		addSource.Store(opts.AddSource)
		if opts.Level != nil {
			level = opts.Level
		}
	}
	return newHandler(handlerConfig{
		output:       out,
		timeFn:       timeFn,
		encoder:      jsonEncoder{},
		addSource:    addSource,
		level:        level,
		sourceFormat: "%s:%d",
	})
}

// NewTextHandler creates a handler that outputs human-readable text log lines.
func NewTextHandler(out io.Writer, opts *slog.HandlerOptions, timeFn func(t time.Time) time.Time) *handler {
	addSource := &atomic.Bool{}
	var level slog.Leveler = slog.LevelInfo
	if opts != nil {
		addSource.Store(opts.AddSource)
		if opts.Level != nil {
			level = opts.Level
		}
	}
	return newHandler(handlerConfig{
		output:       out,
		timeFn:       timeFn,
		encoder:      textEncoder{},
		addSource:    addSource,
		level:        level,
		sourceFormat: "%s:%d",
	})
}
