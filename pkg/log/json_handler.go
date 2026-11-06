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
	"encoding/json"
	"io"
	"log/slog"
	"math"
	"strconv"
	"time"
	"unicode/utf8"
	"unsafe"
)

// SlogJSONHandler writes records as JSON lines.
type SlogJSONHandler = handler

// NewJSONHandler returns a handler writing JSON lines to out. Only Level and AddSource of opts are used.
func NewJSONHandler(out io.Writer, opts *slog.HandlerOptions, timeFn func(t time.Time) time.Time) *SlogJSONHandler {
	return newHandler(out, newHandlerConfig(opts, timeFn, false))
}

func (e *encoder) appendJSON(h *handler, r slog.Record, source string, fields []field, stack *string) {
	b := e.buf

	b = append(b, `{"level":"`...)
	b = appendLineString(b, levelString(r.Level))
	b = append(b, '"')

	if h.name != "" {
		b = append(b, `,"logger":"`...)
		b = appendLineString(b, h.name)
		b = append(b, '"')
	}

	b = append(b, `,"msg":"`...)
	b = appendLineString(b, r.Message)
	b = append(b, '"')

	if source != "" {
		b = append(b, `,"source":"`...)
		b = appendLineString(b, source)
		b = append(b, '"')
	}

	if len(fields) > 0 {
		b = append(b, ',')
		b = appendJSONFields(b, fields)
	}

	if stack != nil && *stack != "" {
		b = append(b, `,"stacktrace":`...)
		b = appendStacktrace(b, *stack)
	}

	b = append(b, `,"time":"`...)
	b = h.cfg.timeFn(r.Time).AppendFormat(b, time.RFC3339)
	b = append(b, '"', '}', '\n')

	e.buf = b
}

func appendJSONFields(b []byte, fs []field) []byte {
	for i, f := range fs {
		if i > 0 {
			b = append(b, ',')
		}

		b = appendJSONString(b, f.key)
		b = append(b, ':')

		if f.group {
			b = append(b, '{')
			b = appendJSONFields(b, f.kids)
			b = append(b, '}')

			continue
		}

		b = appendJSONValue(b, f.value)
	}

	return b
}

func appendJSONValue(b []byte, v slog.Value) []byte {
	switch v.Kind() {
	case slog.KindString:
		return appendJSONString(b, v.String())
	case slog.KindInt64:
		return strconv.AppendInt(b, v.Int64(), 10)
	case slog.KindUint64:
		return strconv.AppendUint(b, v.Uint64(), 10)
	case slog.KindFloat64:
		return appendJSONFloat(b, v.Float64())
	case slog.KindBool:
		return strconv.AppendBool(b, v.Bool())
	case slog.KindDuration:
		return strconv.AppendInt(b, int64(v.Duration()), 10)
	case slog.KindTime:
		b = append(b, '"')
		b = v.Time().AppendFormat(b, time.RFC3339Nano)

		return append(b, '"')
	}

	return appendJSONAny(b, v.Any())
}

// appendJSONFloat formats like encoding/json; NaN and infinities become the error slog.JSONHandler logged.
func appendJSONFloat(b []byte, f float64) []byte {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return appendJSONString(b, unsupportedFloat(f))
	}

	format := byte('f')
	if abs := math.Abs(f); abs != 0 && (abs < 1e-6 || abs >= 1e21) {
		format = 'e'
	}

	b = strconv.AppendFloat(b, f, format, -1, 64)
	if format != 'e' {
		return b
	}

	// clean up e-09 to e-9
	n := len(b)
	if n >= 4 && b[n-4] == 'e' && b[n-3] == '-' && b[n-2] == '0' {
		b[n-2] = b[n-1]
		b = b[:n-1]
	}

	return b
}

func appendJSONAny(b []byte, a any) []byte {
	if a == nil {
		return append(b, "null"...)
	}

	if s, ok := errorString(a); ok {
		return appendJSONString(b, s)
	}

	switch a.(type) {
	// already plain JSON values: json.Marshal sorts and escapes them as the round trip did
	case map[string]any, []any, map[string]string, []string:
		raw, err := json.Marshal(a)
		if err != nil {
			return appendJSONString(b, "!ERROR:"+err.Error())
		}

		return append(b, raw...)
	}

	decoded, err := decodeAny(a)
	if err != nil {
		return appendJSONString(b, "!ERROR:"+err.Error())
	}

	raw, err := json.Marshal(decoded)
	if err != nil {
		return appendJSONString(b, "!ERROR:"+err.Error())
	}

	return append(b, raw...)
}

// appendStacktrace writes a JSON stacktrace as is and anything else as a string,
// compacted and HTML-escaped as encoding/json did with the whole line.
func appendStacktrace(b []byte, stack string) []byte {
	raw := unsafe.Slice(unsafe.StringData(stack), len(stack))
	if !json.Valid(raw) {
		b = append(b, '"')
		b = appendLineString(b, stack)

		return append(b, '"')
	}

	return appendCompactJSON(b, raw)
}

// appendCompactJSON drops whitespace outside strings from valid JSON and escapes <, >, &, U+2028 and U+2029.
func appendCompactJSON(b, src []byte) []byte {
	inString, escaped := false, false

	for i := 0; i < len(src); i++ {
		c := src[i]

		if !inString && (c == ' ' || c == '\t' || c == '\n' || c == '\r') {
			continue
		}

		if c == '<' || c == '>' || c == '&' {
			b = append(b, '\\', 'u', '0', '0', hexDigits[c>>4], hexDigits[c&0xF])
			continue
		}

		if c == 0xE2 && i+2 < len(src) && src[i+1] == 0x80 && src[i+2]&^1 == 0xA8 {
			b = append(b, '\\', 'u', '2', '0', '2', hexDigits[src[i+2]&0xF])
			i += 2

			continue
		}

		b = append(b, c)

		switch {
		case escaped:
			escaped = false
		case inString && c == '\\':
			escaped = true
		case c == '"':
			inString = !inString
		}
	}

	return b
}

// appendLineString escapes level, logger, msg and source as Render.string did, plus the HTML
// escaping encoding/json applied to the whole line. Invalid UTF-8 is kept as is.
func appendLineString(b []byte, s string) []byte {
	start := 0

	for i := 0; i < len(s); {
		c := s[i]

		if c < utf8.RuneSelf && htmlSafe[c] {
			i++
			continue
		}

		if c >= utf8.RuneSelf {
			if c == 0xE2 && i+2 < len(s) && s[i+1] == 0x80 && s[i+2]&^1 == 0xA8 {
				b = append(b, s[start:i]...)
				b = append(b, '\\', 'u', '2', '0', '2', hexDigits[s[i+2]&0xF])
				i += 3
				start = i

				continue
			}

			i++

			continue
		}

		b = append(b, s[start:i]...)

		switch c {
		case '"', '\\':
			b = append(b, '\\', c)
		case '\n':
			b = append(b, '\\', 'n')
		case '\r':
			b = append(b, '\\', 'r')
		case '\t':
			b = append(b, '\\', 't')
		default:
			b = append(b, '\\', 'u', '0', '0', hexDigits[c>>4], hexDigits[c&0xF])
		}

		i++
		start = i
	}

	return append(b, s[start:]...)
}

// htmlSafe marks the ASCII bytes encoding/json writes unescaped with HTML escaping on:
// printable ASCII except '"', '\\', '<', '>' and '&'.
var htmlSafe = func() [utf8.RuneSelf]bool {
	var t [utf8.RuneSelf]bool
	for c := 0x20; c < utf8.RuneSelf; c++ {
		t[c] = true
	}

	for _, c := range []byte{'"', '\\', '<', '>', '&'} {
		t[c] = false
	}

	return t
}()

// appendJSONString quotes s exactly as json.Marshal does.
func appendJSONString(b []byte, s string) []byte {
	b = append(b, '"')
	start := 0

	for i := 0; i < len(s); {
		if c := s[i]; c < utf8.RuneSelf {
			if htmlSafe[c] {
				i++
				continue
			}

			b = append(b, s[start:i]...)

			switch c {
			case '\\', '"':
				b = append(b, '\\', c)
			case '\b':
				b = append(b, '\\', 'b')
			case '\f':
				b = append(b, '\\', 'f')
			case '\n':
				b = append(b, '\\', 'n')
			case '\r':
				b = append(b, '\\', 'r')
			case '\t':
				b = append(b, '\\', 't')
			default:
				b = append(b, '\\', 'u', '0', '0', hexDigits[c>>4], hexDigits[c&0xF])
			}

			i++
			start = i

			continue
		}

		c, size := utf8.DecodeRuneInString(s[i:])
		if c == utf8.RuneError && size == 1 {
			b = append(b, s[start:i]...)
			b = append(b, `�`...)
			i += size
			start = i

			continue
		}

		if c == ' ' || c == ' ' {
			b = append(b, s[start:i]...)
			b = append(b, '\\', 'u', '2', '0', '2', hexDigits[c&0xF])
			i += size
			start = i

			continue
		}

		i += size
	}

	b = append(b, s[start:]...)

	return append(b, '"')
}
