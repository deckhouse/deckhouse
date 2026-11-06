// Copyright 2025 Flant JSC
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
	"io"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// SlogTextHandler writes records as text lines.
type SlogTextHandler = handler

// NewTextHandler returns a handler writing text lines to out. Only Level and AddSource of opts are used.
func NewTextHandler(out io.Writer, opts *slog.HandlerOptions, timeFn func(t time.Time) time.Time) *SlogTextHandler {
	return newHandler(out, newHandlerConfig(opts, timeFn, true))
}

func (e *encoder) appendText(h *handler, r slog.Record, source string, fields []field, stack *string) {
	render := Render{buf: e.buf}

	render.buf = h.cfg.timeFn(r.Time).AppendFormat(render.buf, time.RFC3339)
	render.buf = append(render.buf, ' ')

	render.buf = append(render.buf, levelTextString(r.Level)...)
	render.buf = append(render.buf, ' ')

	if h.name != "" {
		render.TextKeyValue(LoggerNameKey, h.name)
		render.buf = append(render.buf, ' ')
	}

	render.TextQuotedKeyValue(slog.MessageKey, r.Message)
	render.buf = append(render.buf, ' ')

	if source != "" {
		render.TextKeyValue(slog.SourceKey, source)
		render.buf = append(render.buf, ' ')
	}

	if len(fields) > 0 {
		render.textFields(fields, "")
		render.buf = append(render.buf, ' ')
	}

	if stack != nil && *stack != "" {
		render.TextKeyValue(StacktraceKey, *stack)
	}

	render.buf = append(render.buf, '\n')

	e.buf = render.buf
}

func (r *Render) textFields(fs []field, prefix string) {
	for i, f := range fs {
		if i > 0 {
			r.buf = append(r.buf, ' ')
		}

		key := validUTF8(f.key)
		if prefix != "" {
			key = prefix + "." + key
		}

		if f.group {
			r.textFields(f.kids, key)
			continue
		}

		r.textValue(key, f.value)
	}
}

func (r *Render) textValue(key string, v slog.Value) {
	switch v.Kind() {
	case slog.KindString:
		r.TextQuotedKeyValue(key, validUTF8(v.String()))
		return
	case slog.KindTime:
		r.TextQuotedKeyValue(key, v.Time().Format(time.RFC3339Nano))
		return
	case slog.KindAny:
		r.textAny(key, v.Any())
		return
	case slog.KindFloat64:
		if f := v.Float64(); math.IsNaN(f) || math.IsInf(f, 0) {
			r.TextQuotedKeyValue(key, unsupportedFloat(f))
			return
		}
	}

	r.string(key)
	r.buf = append(r.buf, '=', '\'')

	switch v.Kind() {
	case slog.KindInt64:
		r.buf = strconv.AppendInt(r.buf, v.Int64(), 10)
	case slog.KindUint64:
		r.buf = strconv.AppendUint(r.buf, v.Uint64(), 10)
	case slog.KindFloat64:
		r.buf = strconv.AppendFloat(r.buf, v.Float64(), 'f', -1, 64)
	case slog.KindBool:
		r.buf = strconv.AppendBool(r.buf, v.Bool())
	case slog.KindDuration:
		r.buf = strconv.AppendInt(r.buf, int64(v.Duration()), 10)
	}

	r.buf = append(r.buf, '\'')
}

func (r *Render) textAny(key string, a any) {
	if a == nil {
		r.FieldsToString(nil, key)
		return
	}

	if s, ok := errorString(a); ok {
		r.TextQuotedKeyValue(key, validUTF8(s))
		return
	}

	decoded, err := decodeAny(a)
	if err != nil {
		r.TextQuotedKeyValue(key, "!ERROR:"+err.Error())
		return
	}

	r.FieldsToString(decoded, key)
}

// validUTF8 replaces every invalid byte with U+FFFD, as the JSON round trip did.
func validUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}

	var sb strings.Builder
	for _, c := range s {
		sb.WriteRune(c)
	}

	return sb.String()
}
