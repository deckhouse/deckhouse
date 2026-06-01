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
	"bytes"
	"strings"
	"testing"
)

func TestDiscardWritesNothing(t *testing.T) {
	l := Discard()
	// Must not panic and must produce no observable output.
	l.Info("ignored")
	l.Error("ignored")
}

func TestNewBufferLoggerCapturesRecords(t *testing.T) {
	var buf bytes.Buffer
	l := NewBufferLogger(&buf)
	l.Info("captured line")
	if !strings.Contains(buf.String(), "captured line") {
		t.Fatalf("buffer missing record: %q", buf.String())
	}
}
