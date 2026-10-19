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

package lifecycle

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/retry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
)

func TestInstance(t *testing.T) {
	tests := []struct {
		name, version string
		want          string
	}{
		{name: "a released version", version: "v1.2.3", want: "mod@v1.2.3"},
		{name: "build metadata is kept", version: "v1.2.3+build.5", want: "mod@v1.2.3+build.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := instance("mod", tt.version); got != tt.want {
				t.Errorf("instance = %q, want %q", got, tt.want)
			}
		})
	}
}

// fakeDisk counts attaches and records detaches; attachErr fails attaches while set.
type fakeDisk struct {
	mu        sync.Mutex
	attaches  int
	detached  []string
	attachErr error
	attached  chan struct{} // attached receives one signal per successful attach.
}

func (f *fakeDisk) Attach(context.Context, registry.Remote, string, string, string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.attaches++
	if f.attachErr != nil {
		return f.attachErr
	}

	select {
	case f.attached <- struct{}{}:
	default:
	}

	return nil
}

func (f *fakeDisk) Detach(_ context.Context, instance string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.detached = append(f.detached, instance)

	return nil
}

// loaded is a loaded object; preparation never calls it.
type loaded struct{ model.Package }

func newTestReconciler(t *testing.T, disk *fakeDisk, load LoadFunc) *Reconciler {
	t.Helper()

	return NewReconciler(Config{Name: "ns.app", Kind: model.KindApplication}, Deps{
		Disks: Disks{Apps: Disk{Backend: disk, Dir: t.TempDir()}},
		Load:  load,
	})
}

// attempt starts or retries the preparation of version and takes its result, as the loop does.
func attempt(r *Reconciler, version string) {
	r.prepare(context.Background(), model.Desired{Version: version})
	if done := r.preparationDone(); done != nil {
		r.preparationFinished(<-done)
	}
}

func TestPreparationRetryContinuesAfterAttach(t *testing.T) {
	disk := &fakeDisk{attached: make(chan struct{}, 1)}
	loads := 0
	r := newTestReconciler(t, disk, func(context.Context, model.Kind, model.Image) (model.Package, error) {
		loads++
		if loads == 1 {
			return nil, errors.New("bad chart")
		}

		return loaded{}, nil
	})

	attempt(r, "v1")
	if got := r.candidate.outcome(); got.Step != stepLoad || got.Reason != model.ReasonLoadFromFilesystemFailed {
		t.Fatalf("result = %+v, want a load failure", got)
	}

	r.candidate.backoff = retry.Backoff{} // the retry is due
	attempt(r, "v1")

	if !r.candidate.outcome().Ready || disk.attaches != 1 {
		t.Errorf("ready = %v, attaches = %d; want ready after one attach", r.candidate.outcome().Ready, disk.attaches)
	}
}

func TestPreparationCancelDetachesWhatWasAttached(t *testing.T) {
	disk := &fakeDisk{attached: make(chan struct{}, 1)}
	r := newTestReconciler(t, disk, func(ctx context.Context, _ model.Kind, _ model.Image) (model.Package, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})

	r.prepare(context.Background(), model.Desired{Version: "v1"})
	<-disk.attached

	r.stopCandidate(context.Background())

	if r.candidate != nil || len(disk.detached) != 1 || disk.detached[0] != "ns.app@v1" {
		t.Errorf("candidate = %v, detached = %v; want the attached instance released", r.candidate, disk.detached)
	}
}

func TestPreparationNewTargetResetsBackoff(t *testing.T) {
	disk := &fakeDisk{attachErr: errors.New("no registry")}
	r := newTestReconciler(t, disk, func(context.Context, model.Kind, model.Image) (model.Package, error) {
		return loaded{}, nil
	})

	attempt(r, "v1")
	if !r.candidate.backoff.Holds("v1", r.clock.Now()) {
		t.Fatal("a failed attempt must hold its retry")
	}

	disk.mu.Lock()
	disk.attachErr = nil
	disk.mu.Unlock()

	attempt(r, "v2") // a new target starts at once, whatever v1's backoff holds

	if r.candidate.target.Version != "v2" || !r.candidate.outcome().Ready {
		t.Errorf("candidate = %+v, want v2 prepared at once", r.candidate.outcome())
	}
}
