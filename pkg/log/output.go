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
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strconv"
	"strings"
)

const (
	LoggerNameKey = "logger"
	StacktraceKey = "stacktrace"
)

type LogOutput struct {
	Level      string         `json:"level"`
	Name       string         `json:"logger"`
	Message    string         `json:"msg"`
	Source     string         `json:"source"`
	Fields     map[string]any `json:"-"`
	Stacktrace string         `json:"stacktrace"`
	Time       string         `json:"time"`
}

func (lo *LogOutput) MarshalJSON() ([]byte, error) {
	render := Render{}
	render.buf = append(render.buf, '{')

	render.JSONKeyValue(slog.LevelKey, strings.ToLower(lo.Level))

	render.buf = append(render.buf, ',')

	if lo.Name != "" {
		render.JSONKeyValue(LoggerNameKey, lo.Name)
		render.buf = append(render.buf, ',')
	}

	render.JSONKeyValue(slog.MessageKey, lo.Message)
	render.buf = append(render.buf, ',')

	if lo.Source != "" {
		render.JSONKeyValue(slog.SourceKey, lo.Source)
		render.buf = append(render.buf, ',')
	}

	if len(lo.Fields) > 0 {
		b, err := json.Marshal(lo.Fields)
		if err != nil {
			return nil, err
		}

		// ignore first and last '{' and '}' symbols
		render.buf = append(render.buf, b[1:len(b)-1]...)
		render.buf = append(render.buf, ',')
	}

	if lo.Stacktrace != "" {
		if json.Valid([]byte(lo.Stacktrace)) {
			render.RawJsonKeyValue(StacktraceKey, lo.Stacktrace)
		} else {
			render.JSONKeyValue(StacktraceKey, lo.Stacktrace)
		}

		render.buf = append(render.buf, ',')
	}

	render.JSONKeyValue(slog.TimeKey, lo.Time)

	render.buf = append(render.buf, '}')

	return render.buf, nil
}

type Render struct {
	buf []byte
}

var escapes = [256]bool{
	'"': true,
	'<': true,
	// do not escape ' character
	// '\'': true,
	'\\': true,
	'\b': true,
	'\f': true,
	'\n': true,
	'\r': true,
	'\t': true,
}

func (r *Render) JSONKeyValue(key, value string) {
	r.buf = append(r.buf, '"')
	r.string(key)
	r.buf = append(r.buf, '"', ':', '"')
	r.string(value)
	r.buf = append(r.buf, '"')
}

func (r *Render) RawJsonKeyValue(key, value string) {
	r.buf = append(r.buf, '"')
	r.string(key)
	r.buf = append(r.buf, '"', ':')
	r.buf = append(r.buf, []byte(value)...)
}

func (r *Render) string(s string) {
	for _, c := range []byte(s) {
		if escapes[c] {
			r.escapes(s)
			return
		}
	}
	r.buf = append(r.buf, s...)
}

func (r *Render) escapes(s string) {
	n := len(s)
	j := 0
	if n > 0 {
		// Hint the compiler to remove bounds checks in the loop below.
		_ = s[n-1]
	}
	for i := 0; i < n; i++ {
		switch s[i] {
		case '"':
			r.buf = append(r.buf, s[j:i]...)
			r.buf = append(r.buf, '\\', '"')
			j = i + 1
		case '\\':
			r.buf = append(r.buf, s[j:i]...)
			r.buf = append(r.buf, '\\', '\\')
			j = i + 1
		case '\n':
			r.buf = append(r.buf, s[j:i]...)
			r.buf = append(r.buf, '\\', 'n')
			j = i + 1
		case '\r':
			r.buf = append(r.buf, s[j:i]...)
			r.buf = append(r.buf, '\\', 'r')
			j = i + 1
		case '\t':
			r.buf = append(r.buf, s[j:i]...)
			r.buf = append(r.buf, '\\', 't')
			j = i + 1
		case '\f':
			r.buf = append(r.buf, s[j:i]...)
			r.buf = append(r.buf, '\\', 'u', '0', '0', '0', 'c')
			j = i + 1
		case '\b':
			r.buf = append(r.buf, s[j:i]...)
			r.buf = append(r.buf, '\\', 'u', '0', '0', '0', '8')
			j = i + 1
		case '<':
			r.buf = append(r.buf, s[j:i]...)
			r.buf = append(r.buf, '\\', 'u', '0', '0', '3', 'c')
			j = i + 1
		// do not escape ' character
		// case '\'':
		// 	e.buf = append(e.buf, s[j:i]...)
		// 	e.buf = append(e.buf, '\\', 'u', '0', '0', '2', '7')
		// 	j = i + 1
		case 0:
			r.buf = append(r.buf, s[j:i]...)
			r.buf = append(r.buf, '\\', 'u', '0', '0', '0', '0')
			j = i + 1
		}
	}
	r.buf = append(r.buf, s[j:]...)
}

func (lo *LogOutput) Text() ([]byte, error) {
	render := Render{}

	render.buf = append(render.buf, lo.Time...)
	render.buf = append(render.buf, ' ')

	render.buf = append(render.buf, strings.ToUpper(lo.Level)...)
	render.buf = append(render.buf, ' ')

	if lo.Name != "" {
		render.TextKeyValue(LoggerNameKey, lo.Name)
		render.buf = append(render.buf, ' ')
	}

	render.TextQuotedKeyValue(slog.MessageKey, lo.Message)
	render.buf = append(render.buf, ' ')

	if lo.Source != "" {
		render.TextKeyValue(slog.SourceKey, lo.Source)
		render.buf = append(render.buf, ' ')
	}

	if len(lo.Fields) > 0 {
		render.FieldsToString(lo.Fields, "")
		render.buf = append(render.buf, ' ')
	}

	if lo.Stacktrace != "" {
		render.TextKeyValue(StacktraceKey, lo.Stacktrace)
	}

	render.buf = append(render.buf, '\n')

	return render.buf, nil
}

func (r *Render) TextKeyValue(key, value string) {
	r.string(key)
	r.buf = append(r.buf, '=')
	r.string(value)
}

func (r *Render) TextQuotedKeyValue(key, value string) {
	r.string(key)
	r.buf = append(r.buf, '=', '\'')
	r.string(value)
	r.buf = append(r.buf, '\'')
}

func (r *Render) FieldsToString(m any, keyPrefix string) {
	switch val := m.(type) {
	case map[string]any:
		keys := slices.Collect(maps.Keys(val))
		slices.Sort(keys)
		for i, k := range keys {
			if i > 0 {
				r.buf = append(r.buf, ' ')
			}

			v := val[k]
			if keyPrefix != "" {
				k = keyPrefix + "." + k
			}

			r.FieldsToString(v, k)
		}
	case []any:
		for i, item := range val {
			if i > 0 {
				r.buf = append(r.buf, ' ')
			}

			key := keyPrefix + "[" + strconv.Itoa(i) + "]"
			r.FieldsToString(item, key)
		}
	case string:
		r.TextQuotedKeyValue(keyPrefix, val)
	case float64:
		r.TextQuotedKeyValue(keyPrefix, strconv.FormatFloat(val, 'f', -1, 64))
	case int:
		r.TextQuotedKeyValue(keyPrefix, strconv.Itoa(val))
	case uint:
		r.TextQuotedKeyValue(keyPrefix, strconv.FormatUint(uint64(val), 10))
	case int64:
		r.TextQuotedKeyValue(keyPrefix, strconv.FormatInt(val, 10))
	case uint64:
		r.TextQuotedKeyValue(keyPrefix, strconv.FormatUint(val, 10))
	case bool:
		r.TextQuotedKeyValue(keyPrefix, strconv.FormatBool(val))
	default:
		r.buf = append(r.buf, fmt.Sprintf("!SOMETHING GOES WRONG. type: %T", val)...)
	}
}
