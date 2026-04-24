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
	"maps"
	"slices"
	"strings"
	"sync"
)

// encoder defines how log records are serialized for output.
type encoder interface {
	Encode(w io.Writer, output *LogOutput) error
}

var renderPool = sync.Pool{
	New: func() any {
		return &Render{buf: make([]byte, 0, 512)}
	},
}

type jsonEncoder struct{}

func (jsonEncoder) Encode(w io.Writer, output *LogOutput) error {
	r := renderPool.Get().(*Render)
	r.buf = r.buf[:0]

	r.buf = append(r.buf, '{')
	r.JSONKeyValue(slog.LevelKey, strings.ToLower(output.Level))

	if output.Name != "" {
		r.buf = append(r.buf, ',')
		r.JSONKeyValue(LoggerNameKey, output.Name)
	}

	r.buf = append(r.buf, ',')
	r.JSONKeyValue(slog.MessageKey, output.Message)

	if output.Source != "" {
		r.buf = append(r.buf, ',')
		r.JSONKeyValue(slog.SourceKey, output.Source)
	}

	if len(output.Fields) > 0 {
		r.buf = append(r.buf, ',')
		r.appendJSONFields(output.Fields)
	}

	if output.Stacktrace != "" {
		r.buf = append(r.buf, ',')
		if json.Valid([]byte(output.Stacktrace)) {
			r.RawJSONKeyValue(StacktraceKey, output.Stacktrace)
		} else {
			r.JSONKeyValue(StacktraceKey, output.Stacktrace)
		}
	}

	r.buf = append(r.buf, ',')
	r.JSONKeyValue(slog.TimeKey, output.Time)
	r.buf = append(r.buf, '}', '\n')

	_, err := w.Write(r.buf)
	renderPool.Put(r)
	return err
}

type textEncoder struct{}

func (textEncoder) Encode(w io.Writer, output *LogOutput) error {
	r := renderPool.Get().(*Render)
	r.buf = r.buf[:0]

	r.buf = append(r.buf, output.Time...)
	r.buf = append(r.buf, ' ')
	r.buf = append(r.buf, strings.ToUpper(output.Level)...)
	r.buf = append(r.buf, ' ')

	if output.Name != "" {
		r.TextKeyValue(LoggerNameKey, output.Name)
		r.buf = append(r.buf, ' ')
	}

	r.TextQuotedKeyValue(slog.MessageKey, output.Message)
	r.buf = append(r.buf, ' ')

	if output.Source != "" {
		r.TextKeyValue(slog.SourceKey, output.Source)
		r.buf = append(r.buf, ' ')
	}

	if len(output.Fields) > 0 {
		r.FieldsToString(output.Fields, "")
		r.buf = append(r.buf, ' ')
	}

	if output.Stacktrace != "" {
		r.TextKeyValue(StacktraceKey, output.Stacktrace)
	}

	r.buf = append(r.buf, '\n')

	_, err := w.Write(r.buf)
	renderPool.Put(r)
	return err
}

func (r *Render) appendJSONFields(m map[string]any) {
	keys := slices.Collect(maps.Keys(m))
	slices.Sort(keys)
	for i, k := range keys {
		if i > 0 {
			r.buf = append(r.buf, ',')
		}
		r.buf = append(r.buf, '"')
		r.string(k)
		r.buf = append(r.buf, '"', ':')
		r.appendJSONValue(m[k])
	}
}
