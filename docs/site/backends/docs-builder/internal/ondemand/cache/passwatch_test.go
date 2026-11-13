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

package cache

import (
	"context"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/flant/docs-builder/internal/registry"
)

// A pass held past its deadline by a step that ignores its context fails the
// check until it ends.
func TestPassWatchReportsAStuckPass(t *testing.T) {
	// The render waits for release, whatever its context says.
	f := &fakeRender{size: 10, release: make(chan struct{})}
	s, c := newTestStore(t, f, 1000)
	r := Resolver{
		Modules: func(context.Context) ([]registry.Module, error) {
			return []registry.Module{{Name: "echo", Enabled: true, Version: "v1.0.0"}}, nil
		},
		Policy: ClusterPolicy{Logger: log.NewNop()},
		Logger: log.NewNop(),
	}
	w := NewPassWatch()
	w.now = c.now
	p := NewPinner(r, s, time.Hour, log.NewNop())
	p.Watch = w

	done := make(chan struct{})
	go func() {
		defer close(done)
		p.pass(context.Background())
	}()
	deadline := time.Now().Add(5 * time.Second)
	for f.calls.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the pass did not start the render")
		}
		time.Sleep(time.Millisecond)
	}

	if err := w.Check(); err != nil {
		t.Fatalf("a pass within its time: %v", err)
	}
	c.add(passTimeout + passStuckAfter - time.Second)
	if err := w.Check(); err != nil {
		t.Fatalf("a pass less than %s past its deadline: %v", passStuckAfter, err)
	}
	c.add(time.Second)
	if err := w.Check(); err == nil {
		t.Fatal("a pass stuck past its deadline is not reported")
	}

	close(f.release)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the pass did not end after the render")
	}
	if err := w.Check(); err != nil {
		t.Fatalf("after the pass: %v", err)
	}
}
