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

import "encoding/json"

type LogOutput struct {
	Level      string `json:"level"`
	Name       string `json:"logger"`
	Message    string `json:"msg"`
	Source     string `json:"source"`
	FieldsJSON []byte `json:"-"`
	Stacktrace string `json:"stacktrace"`
	Time       string `json:"time"`
}

func (lo *LogOutput) MarshalJSON() ([]byte, error) {
	render := Render{}
	render.buf = append(render.buf, '{')

	render.JSONKeyValue("level", lo.Level)

	render.buf = append(render.buf, ',')

	if lo.Name != "" {
		render.JSONKeyValue("logger", lo.Name)
		render.buf = append(render.buf, ',')
	}

	render.JSONKeyValue("msg", lo.Message)
	render.buf = append(render.buf, ',')

	if lo.Source != "" {
		render.JSONKeyValue("source", lo.Source)
		render.buf = append(render.buf, ',')
	}

	if len(lo.FieldsJSON) > 0 {
		render.buf = append(render.buf, lo.FieldsJSON...)
		render.buf = append(render.buf, ',')
	}

	if lo.Stacktrace != "" {
		if json.Valid([]byte(lo.Stacktrace)) {
			render.RawJsonKeyValue("stacktrace", lo.Stacktrace)
		} else {
			render.JSONKeyValue("stacktrace", lo.Stacktrace)
		}

		render.buf = append(render.buf, ',')
	}

	render.JSONKeyValue("time", lo.Time)

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
