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

package logging_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/deckhouse/deckhouse/go_lib/cloud-provider/logging"
	"github.com/deckhouse/deckhouse/pkg/log"
)

func TestNewLogrAdapter_WithLogger(t *testing.T) {
	t.Parallel()
	adapter := logging.NewLogrAdapter(log.NewLogger())
	adapter.Info("hello", "key", "val")
	adapter.Error(errors.New("boom"), "msg")
	adapter.WithValues("a", 1).Info("x")
	adapter.WithName("test").Info("x")
	if !adapter.Enabled() {
		t.Fatal("expected Enabled() to be true")
	}
}

func TestNewLogrAdapter_VerbosityFollowsLoggerLevel(t *testing.T) {
	t.Parallel()

	var infoOutput bytes.Buffer
	infoAdapter := logging.NewLogrAdapter(log.NewLogger(
		log.WithLevel(log.LevelInfo.Level()),
		log.WithOutput(&infoOutput),
	))
	if !infoAdapter.Enabled() {
		t.Fatal("expected info level to be enabled")
	}
	if infoAdapter.V(1).Enabled() {
		t.Fatal("expected V(1) to be disabled for info logger")
	}
	infoAdapter.V(1).Info("request body", "body", "secret")
	if infoOutput.Len() != 0 {
		t.Fatalf("verbose log output = %q, want empty", infoOutput.String())
	}

	var debugOutput bytes.Buffer
	debugAdapter := logging.NewLogrAdapter(log.NewLogger(
		log.WithLevel(log.LevelDebug.Level()),
		log.WithOutput(&debugOutput),
	))
	if !debugAdapter.V(1).Enabled() {
		t.Fatal("expected V(1) to be enabled for debug logger")
	}
	debugAdapter.V(1).Info("request body", "body", "secret")
	if !strings.Contains(debugOutput.String(), `"level":"debug"`) {
		t.Fatalf("verbose log output = %q, want debug record", debugOutput.String())
	}
	if !strings.Contains(debugOutput.String(), "request body") {
		t.Fatalf("verbose log output = %q, want message", debugOutput.String())
	}

	var traceOutput bytes.Buffer
	traceAdapter := logging.NewLogrAdapter(log.NewLogger(
		log.WithLevel(log.LevelTrace.Level()),
		log.WithOutput(&traceOutput),
	))
	if !traceAdapter.V(2).Enabled() {
		t.Fatal("expected V(2) to be enabled for trace logger")
	}
	traceAdapter.V(2).Info("response body", "body", "secret")
	if !strings.Contains(traceOutput.String(), `"level":"trace"`) {
		t.Fatalf("verbose log output = %q, want trace record", traceOutput.String())
	}
	if !strings.Contains(traceOutput.String(), "response body") {
		t.Fatalf("verbose log output = %q, want message", traceOutput.String())
	}
}

func TestNewLogrAdapter_WithNop(t *testing.T) {
	t.Parallel()
	adapter := logging.NewLogrAdapter(log.NewNop())
	adapter.Info("hello")
	adapter.Error(nil, "msg")
}
