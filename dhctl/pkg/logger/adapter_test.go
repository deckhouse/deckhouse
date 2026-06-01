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
	"log/slog"
	"testing"

	libdhctl_log "github.com/deckhouse/lib-dhctl/pkg/log"
)

func TestAdapter_Interface(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	adapter := NewAdapter(logger)

	// Verify type compatibility via assignment
	var _ libdhctl_log.Logger = adapter

	// Basic method call
	adapter.InfoF("test info %s", "message")
	if buf.Len() == 0 {
		t.Fatal("expected output in buffer")
	}

	out := buf.String()
	if out == "" {
		t.Fatal("empty output")
	}
}

func TestAdapter_Process(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	adapter := NewAdapter(logger)

	err := adapter.Process("test_process", "Test Step", func() error {
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !bytes.Contains([]byte(out), []byte("Starting: Test Step")) {
		t.Errorf("expected output to contain start message, got: %s", out)
	}
	if !bytes.Contains([]byte(out), []byte("Finished: Test Step")) {
		t.Errorf("expected output to contain finish message, got: %s", out)
	}
}

func TestNewLibdhctlAdapterProcessEmitsMarkers(t *testing.T) {
	c := &capture{}
	a := NewAdapter(slog.New(c))
	_ = a.Process("p", "title", func() error { return nil })

	sawMarker := false
	for _, r := range c.records {
		if isRendererMarker(r) {
			sawMarker = true
		}
	}
	if !sawMarker {
		t.Fatal("lib-connection process should emit renderer markers")
	}
}
