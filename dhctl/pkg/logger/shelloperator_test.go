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
	"io"
	"strings"
	"testing"

	shlog "github.com/deckhouse/deckhouse/pkg/log"
)

func TestBindShellOperatorRoutesIntoSlog(t *testing.T) {
	// BindShellOperator mutates the shell-operator global default logger; restore it
	// afterwards so other tests are not affected by the level/output change.
	t.Cleanup(func() {
		shlog.Default().SetLevel(shlog.LevelInfo)
		shlog.Default().SetOutput(io.Discard)
	})

	var buf bytes.Buffer
	l := NewBufferLogger(&buf)
	BindShellOperator(l, true)

	shlog.Default().Info("shell-operator line")

	if !strings.Contains(buf.String(), "shell-operator line") {
		t.Fatalf("slog buffer missing shell-operator output: %q", buf.String())
	}
}

func TestBindShellOperatorMutesWhenNotDebug(t *testing.T) {
	t.Cleanup(func() {
		shlog.Default().SetLevel(shlog.LevelInfo)
		shlog.Default().SetOutput(io.Discard)
	})

	var buf bytes.Buffer
	l := NewBufferLogger(&buf)
	BindShellOperator(l, false)

	shlog.Default().Info("should be muted")

	if strings.Contains(buf.String(), "should be muted") {
		t.Fatalf("expected shell-operator to be muted, got: %q", buf.String())
	}
}
