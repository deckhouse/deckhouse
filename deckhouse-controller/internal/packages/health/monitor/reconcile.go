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

package monitor

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
)

// runWorker drains the queue until it is shut down. Returning from this
// function causes wait.UntilWithContext to immediately re-call it after a
// one-second delay; once the queue is shut down Get returns shutdown=true
// and the worker exits for good.
func (m *Monitor) runWorker(_ context.Context) {
	for m.processNext() {
	}
}

// processNext pops one package key, collects every workload that belongs
// to it, and hands the slice to the Reconcile callback. A failing collect
// re-enqueues the key with rate-limited backoff; on success the key is
// forgotten so its backoff resets. Reconcile itself returns no error and
// is never retried. Returns false only when the queue has been shut down.
func (m *Monitor) processNext() bool {
	pkg, shutdown := m.queue.Get()
	if shutdown {
		return false
	}
	defer m.queue.Done(pkg)

	status, err := m.collect(pkg)
	if err != nil {
		m.queue.AddRateLimited(pkg)
		return true
	}

	m.reconcile(pkg, status)
	m.queue.Forget(pkg)

	return true
}

// collect lists every workload that belongs to a package across all
// indexers and reduces each to a WorkloadStatus. The package label is the
// only filter; namespace is intentionally not part of the package identity
// in this PoC.
func (m *Monitor) collect(pkg string) ([]WorkloadStatus, error) {
	var out []WorkloadStatus
	for kind, idx := range m.indexers {
		objs, err := idx.ByIndex(indexName, pkg)
		if err != nil {
			return nil, fmt.Errorf("list %s by package %q: %w", kind, pkg, err)
		}

		for _, obj := range objs {
			if st, ok := classify(obj); ok {
				out = append(out, st)
			}
		}
	}

	return out, nil
}

// classify dispatches to the typed reducer for an indexer-cache object. The
// type switch is intentionally kept here rather than behind an interface:
// the set of workload kinds is closed and the spec calls for typed reducers.
func classify(obj any) (WorkloadStatus, bool) {
	switch v := obj.(type) {
	case *appsv1.Deployment:
		return reduceDeployment(v), true
	case *appsv1.StatefulSet:
		return reduceStatefulSet(v), true
	default:
		return WorkloadStatus{}, false
	}
}
