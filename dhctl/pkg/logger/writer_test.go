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

package logger

import (
	"log/slog"
	"strings"
	"testing"
)

func newLoggerFromCapture(c *capture) *slog.Logger { return slog.New(c) }

func TestLineWriterEmitsOneRecordPerLine(t *testing.T) {
	c := &capture{}
	w := NewLineWriter(newLoggerFromCapture(c))
	n, err := w.Write([]byte("first line\nsecond line\n"))
	if err != nil || n != len("first line\nsecond line\n") {
		t.Fatalf("write n=%d err=%v", n, err)
	}
	if len(c.records) != 2 {
		t.Fatalf("want 2 records, got %d", len(c.records))
	}
	if c.records[0].Message != "first line" || c.records[1].Message != "second line" {
		t.Fatalf("messages = %q, %q", c.records[0].Message, c.records[1].Message)
	}
}

func TestLineWriterBuffersPartialLine(t *testing.T) {
	c := &capture{}
	w := NewLineWriter(newLoggerFromCapture(c))
	_, _ = w.Write([]byte("partial"))
	if len(c.records) != 0 {
		t.Fatalf("partial line should not emit, got %d", len(c.records))
	}
	_, _ = w.Write([]byte(" rest\n"))
	if len(c.records) != 1 || !strings.Contains(c.records[0].Message, "partial rest") {
		t.Fatalf("records = %v", c.records)
	}
}
