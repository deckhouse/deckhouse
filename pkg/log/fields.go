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

	"gopkg.in/yaml.v3"
)

type formatter string

const (
	jsonFormatter = "json"
	yamlFormatter = "yaml"
)

func Type(key string, t any) slog.Attr {
	return slog.Attr{
		Key:   key,
		Value: slog.StringValue(fmt.Sprintf("%T", t)),
	}
}

func Err(err error) slog.Attr {
	return slog.Attr{
		Key:   "error",
		Value: slog.StringValue(err.Error()),
	}
}

var _ slog.LogValuer = (*Raw)(nil)

func RawJSON(key, text string) slog.Attr {
	return slog.Attr{
		Key:   key,
		Value: NewJSONRaw(text).LogValue(),
	}
}

func RawYAML(key, text string) slog.Attr {
	return slog.Attr{
		Key:   key,
		Value: NewYAMLRaw(text).LogValue(),
	}
}

// made them public to use without slog.Attr
func NewJSONRaw(text string) *Raw {
	return &Raw{
		formatter: jsonFormatter,
		text:      text,
	}
}

func NewYAMLRaw(text string) *Raw {
	return &Raw{
		formatter: yamlFormatter,
		text:      text,
	}
}

type Raw struct {
	formatter formatter
	text      string
}

func (r *Raw) LogValue() slog.Value {
	raw := make(map[string]any, 1)

	switch r.formatter {
	case jsonFormatter:
		if err := json.Unmarshal([]byte(r.text), &raw); err == nil {
			return slog.AnyValue(raw)
		}
	case yamlFormatter:
		if err := yaml.Unmarshal([]byte(r.text), &raw); err == nil {
			return slog.AnyValue(raw)
		}
	}

	return slog.StringValue(r.text)
}
