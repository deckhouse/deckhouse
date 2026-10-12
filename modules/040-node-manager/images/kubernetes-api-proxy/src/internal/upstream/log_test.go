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

package upstream

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
)

type blockingWriter struct {
	entered chan struct{}
	release chan struct{}
}

func (w blockingWriter) Write(p []byte) (int, error) {
	w.entered <- struct{}{}
	<-w.release

	return len(p), nil
}

func TestList_LogsOutsideLock(t *testing.T) {
	w := blockingWriter{entered: make(chan struct{}, 1), release: make(chan struct{})}
	t.Cleanup(func() { close(w.release) })

	list, _, _ := newTestList(t, "m0", "m1")
	list.round(context.Background())
	list.logger = log.NewLogger(log.WithOutput(w), log.WithLevel(slog.LevelInfo))

	go list.Down(NewUpstream("m0", nil))

	select {
	case <-w.entered:
	case <-time.After(time.Second):
		t.Fatal("Down did not log")
	}

	picked := make(chan struct{})
	go func() {
		_, _ = list.Pick()
		close(picked)
	}()

	select {
	case <-picked:
	case <-time.After(time.Second):
		t.Fatal("Pick blocked behind a log write held under list.mu")
	}
}
