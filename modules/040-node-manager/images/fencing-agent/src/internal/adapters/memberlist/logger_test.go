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

package memberlist

import (
	"bytes"
	"strings"
	"testing"

	"github.com/deckhouse/deckhouse/pkg/log"
)

func TestLogWriterLevelsAndDump(t *testing.T) {
	var out bytes.Buffer

	w := newLogWriter(log.NewLogger(log.WithOutput(&out), log.WithHandlerType(log.JSONHandlerType)))

	lines := []string{
		"[ERR] memberlist: Failed to send ack: boom 10.0.0.1:7946",
		"[WARN] memberlist: Refuting an alive message for 'n1' (10.0.0.1:7946) meta:(" + strings.Repeat("1, ", 600) + ") VS ()",
		"[INFO] memberlist: Suspect n2 has failed",
	}
	for _, l := range lines {
		if _, err := w.Write([]byte(l + "\n")); err != nil {
			t.Fatal(err)
		}
	}

	got := out.String()
	if strings.Contains(got, `"level":"error"`) || strings.Contains(got, "stacktrace") {
		t.Fatalf("memberlist errors must not log at error level with a stack trace: %s", got)
	}

	if strings.Contains(got, "1, 1, 1") {
		t.Fatalf("the metadata dump is not trimmed: %s", got)
	}

	for _, want := range []string{"Failed to send ack", "Refuting an alive message", `"level":"info"`, "Suspect n2 has failed"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %s", want, got)
		}
	}
}
