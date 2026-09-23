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

package logtest

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"regexp"
	"strings"
	"testing"

	"github.com/deckhouse/deckhouse/pkg/log"
)

func NewJSONLogger(w io.Writer) *log.Logger {
	return log.NewLogger(
		log.WithOutput(w),
		log.WithHandlerType(log.JSONHandlerType),
		log.WithLevel(slog.LevelDebug),
	)
}

type Record map[string]any

func (r Record) Level() string {
	return r.Str("level")
}

func (r Record) Msg() string {
	return r.Str("msg")
}

func (r Record) Str(key string) string {
	s, _ := r[key].(string)

	return s
}

func (r Record) Int(key string) int {
	n, ok := r[key].(float64)
	if !ok {
		return -1
	}

	return int(n)
}

func Decode(t *testing.T, snapshot string) []Record {
	t.Helper()

	return decode(t, strings.NewReader(snapshot))
}

func Drain(t *testing.T, buf *bytes.Buffer) []Record {
	t.Helper()

	return decode(t, buf)
}

func decode(t *testing.T, r io.Reader) []Record {
	t.Helper()

	var records []Record

	dec := json.NewDecoder(r)

	for dec.More() {
		var record Record
		if err := dec.Decode(&record); err != nil {
			t.Fatalf("log output is not JSON lines: %v", err)
		}

		records = append(records, record)
	}

	return records
}

func WithMsg(records []Record, msg string) []Record {
	var matched []Record

	for _, record := range records {
		if record.Msg() == msg {
			matched = append(matched, record)
		}
	}

	return matched
}

func Levels(records []Record) []string {
	levels := make([]string, 0, len(records))
	for _, record := range records {
		levels = append(levels, record.Level())
	}

	return levels
}

var serviceKeys = map[string]bool{
	"level":      true,
	"logger":     true,
	"msg":        true,
	"source":     true,
	"stacktrace": true,
	"time":       true,
}

func IsServiceKey(key string) bool {
	return serviceKeys[key]
}

var snakeCaseKey = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func AssertSnakeCaseKeys(t *testing.T, records []Record) {
	t.Helper()

	for _, record := range records {
		for key, value := range record {
			if serviceKeys[key] {
				continue
			}

			assertSnakeCaseKey(t, record.Msg(), key, key, value)
		}
	}
}

func assertSnakeCaseKey(t *testing.T, msg, path, key string, value any) {
	t.Helper()

	if !snakeCaseKey.MatchString(key) {
		t.Errorf("log key %q in %q is not snake_case", path, msg)
	}

	nested, ok := value.(map[string]any)
	if !ok {
		return
	}

	for nestedKey, nestedValue := range nested {
		assertSnakeCaseKey(t, msg, path+"."+nestedKey, nestedKey, nestedValue)
	}
}
