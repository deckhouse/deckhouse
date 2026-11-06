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

package log

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	logContext "github.com/deckhouse/deckhouse/pkg/log/context"
)

var _ Handler = (*handler)(nil)

// handlerConfig is shared by a handler and every handler derived from it.
type handlerConfig struct {
	// serializes writes, so lines of concurrent records never interleave
	mu sync.Mutex

	level     slog.Leveler
	addSource *AddSourceVar
	timeFn    func(t time.Time) time.Time
	text      bool
	sources   *sourceCache
}

// handler encodes a record in one pass into a pooled buffer and writes it under a lock.
//
// The output is the one the previous implementation got by encoding the record with
// slog.JSONHandler, decoding it into a map and encoding the map again: fields are sorted
// by key, a repeated key keeps its last value, attributes named level, msg and time are
// dropped, and an attribute named source becomes _source when sources are on.
type handler struct {
	cfg *handlerConfig

	w      atomic.Pointer[io.Writer]
	name   string
	groups []string
	attrs  []groupedAttr
}

// groupedAttr is an attribute added by WithAttrs and the number of groups open at that moment.
type groupedAttr struct {
	depth int
	attr  slog.Attr
}

func newHandler(out io.Writer, cfg *handlerConfig) *handler {
	h := &handler{cfg: cfg}
	h.w.Store(&out)

	return h
}

// newHandlerConfig keeps the NewJSONHandler and NewTextHandler signatures: only Level and AddSource of opts are used.
func newHandlerConfig(opts *slog.HandlerOptions, timeFn func(t time.Time) time.Time, text bool) *handlerConfig {
	cfg := &handlerConfig{
		level:     slog.LevelInfo,
		addSource: new(AddSourceVar),
		timeFn:    timeFn,
		text:      text,
		sources:   newSourceCache(),
	}

	cfg.addSource.Set(false)

	if opts == nil {
		return cfg
	}

	if opts.Level != nil {
		cfg.level = opts.Level
	}

	cfg.addSource.Set(opts.AddSource)

	return cfg
}

func (h *handler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.cfg.level.Level()
}

func (h *handler) Handle(ctx context.Context, r slog.Record) error {
	addSource := *h.cfg.addSource.Source()

	var stack *string
	if logContext.GetCustomKeyContext(ctx) {
		stack = logContext.GetStackTraceContext(ctx)

		if stack == nil && addSource {
			var pcs [1]uintptr
			// skip [runtime.Callers, this function, slog.(*Logger).log, slog.(*Logger).Log, the wrapper]
			runtime.Callers(5, pcs[:])
			r.PC = pcs[0]
		}
	}

	// if logger was traced - no source
	source := ""
	if addSource && stack == nil {
		source = h.cfg.sources.get(r.PC)
	}

	e := newEncoder()
	defer e.free()

	fields := e.collect(h, r, addSource)

	if h.cfg.text {
		e.appendText(h, r, source, fields, stack)

		return h.write(e.buf)
	}

	e.appendJSON(h, r, source, fields, stack)

	return h.write(e.buf)
}

func (h *handler) write(b []byte) error {
	h.cfg.mu.Lock()
	defer h.cfg.mu.Unlock()

	_, err := (*h.w.Load()).Write(b)

	return err
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) < 1 {
		return h
	}

	h2 := h.clone()
	h2.attrs = slices.Grow(slices.Clip(h.attrs), len(attrs))

	for _, a := range attrs {
		// resolve once, as slog.JSONHandler did, instead of on every record
		a.Value = a.Value.Resolve()
		h2.attrs = append(h2.attrs, groupedAttr{depth: len(h.groups), attr: a})
	}

	return h2
}

func (h *handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}

	h2 := h.clone()
	h2.groups = append(slices.Clip(h.groups), name)

	return h2
}

func (h *handler) Named(name string) slog.Handler {
	h2 := h.clone()
	h2.name = name

	if h.name != "" {
		h2.name = h.name + "." + name
	}

	return h2
}

// SetOutput changes the output of this handler and of the handlers derived from it afterwards.
func (h *handler) SetOutput(w io.Writer) {
	h.w.Store(&w)
}

func (h *handler) clone() *handler {
	h2 := &handler{
		cfg:    h.cfg,
		name:   h.name,
		groups: h.groups,
		attrs:  h.attrs,
	}
	h2.w.Store(h.w.Load())

	return h2
}

// field is a resolved attribute; a group keeps its sorted children in kids.
type field struct {
	key   string
	value slog.Value
	kids  []field
	group bool
}

type encoder struct {
	buf    []byte
	fields []field
}

var encoderPool = sync.Pool{
	New: func() any {
		return &encoder{buf: make([]byte, 0, 1024)}
	},
}

func newEncoder() *encoder {
	return encoderPool.Get().(*encoder)
}

func (e *encoder) free() {
	// a single huge record must not keep its buffer alive
	const maxBufferSize = 64 << 10
	if cap(e.buf) > maxBufferSize {
		return
	}

	e.buf = e.buf[:0]
	// drop references to logged values
	clear(e.fields[:cap(e.fields)])
	e.fields = e.fields[:0]

	encoderPool.Put(e)
}

// collect returns the handler and record attributes as one sorted object tree.
func (e *encoder) collect(h *handler, r slog.Record, addSource bool) []field {
	depth := len(h.groups)
	if depth == 0 {
		fs := e.fields[:0]
		for _, ga := range h.attrs {
			fs = appendField(fs, ga.attr, addSource)
		}

		r.Attrs(func(a slog.Attr) bool {
			fs = appendField(fs, a, addSource)
			return true
		})

		e.fields = fs

		return sortFields(fs)
	}

	// levels[d] holds the fields of the object opened by groups[d-1]
	levels := make([][]field, depth+1)
	for _, ga := range h.attrs {
		levels[ga.depth] = appendField(levels[ga.depth], ga.attr, addSource)
	}

	r.Attrs(func(a slog.Attr) bool {
		levels[depth] = appendField(levels[depth], a, addSource)
		return true
	})

	// a group goes after the attributes added before it was opened; an empty group is omitted
	for d := depth; d > 0; d-- {
		kids := sortFields(levels[d])
		if len(kids) == 0 {
			continue
		}

		levels[d-1] = append(levels[d-1], field{key: h.groups[d-1], kids: kids, group: true})
	}

	return sortFields(levels[0])
}

// appendField applies what the old ReplaceAttr and slog.JSONHandler did to every attribute.
func appendField(dst []field, a slog.Attr, addSource bool) []field {
	a.Value = a.Value.Resolve()

	if a.Value.Kind() == slog.KindGroup {
		return appendGroup(dst, a, addSource)
	}

	switch a.Key {
	// skip standard fields
	case slog.LevelKey, slog.MessageKey, slog.TimeKey:
		return dst
	case slog.SourceKey:
		if !addSource {
			return dst
		}

		a.Key = "_source"
	case "":
		// slog elides empty attributes
		if a.Value.Kind() == slog.KindAny && a.Value.Any() == nil {
			return dst
		}
	}

	return append(dst, field{key: a.Key, value: a.Value})
}

func appendGroup(dst []field, a slog.Attr, addSource bool) []field {
	attrs := a.Value.Group()

	// a group with an empty key is inlined
	if a.Key == "" {
		for _, ga := range attrs {
			dst = appendField(dst, ga, addSource)
		}

		return dst
	}

	var kids []field
	for _, ga := range attrs {
		kids = appendField(kids, ga, addSource)
	}

	kids = sortFields(kids)
	if len(kids) == 0 {
		return dst
	}

	return append(dst, field{key: a.Key, kids: kids, group: true})
}

// sortFields sorts by key and keeps the last of repeated keys, as decoding into a map did.
func sortFields(fs []field) []field {
	if len(fs) < 2 {
		return fs
	}

	slices.SortStableFunc(fs, func(a, b field) int {
		return strings.Compare(a.key, b.key)
	})

	n := 0
	for i := range fs {
		if i+1 < len(fs) && fs[i+1].key == fs[i].key {
			continue
		}

		fs[n] = fs[i]
		n++
	}

	return fs[:n]
}

// errorString mirrors slog.JSONHandler: an error is logged as its message unless it marshals itself.
func errorString(a any) (string, bool) {
	err, ok := a.(error)
	if !ok {
		return "", false
	}

	if _, ok := a.(json.Marshaler); ok {
		return "", false
	}

	return err.Error(), true
}

// decodeAny turns a value into what decoding the slog.JSONHandler output gave: maps, []any, float64.
func decodeAny(a any) (any, error) {
	var sb strings.Builder

	enc := json.NewEncoder(&sb)
	enc.SetEscapeHTML(false)

	if err := enc.Encode(a); err != nil {
		return nil, err
	}

	var decoded any
	if err := json.Unmarshal([]byte(sb.String()), &decoded); err != nil {
		return nil, err
	}

	return decoded, nil
}

func unsupportedFloat(f float64) string {
	return "!ERROR:json: unsupported value: " + strconv.FormatFloat(f, 'g', -1, 64)
}
