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

// Package integrity checks that what a replica's store holds is what its digests say, and repairs it
// when it is not.
//
// Three readers of the same question. The scrub reads the whole store in the background, slowly
// enough that nothing else on the master notices. A Verify reads all of it, or the images it names, at
// once, when somebody asks. A Repair reads the images it names and replaces what is broken or missing
// with a whole copy from another replica or the upstream.
//
// What they find broken goes into one Ledger, which the rest of the syncer reads: the survey takes a
// broken blob for absent, so the replica is not complete while an image of the set references one,
// and the status carries the list, which is how the controller learns to ask for a Repair and how a
// restarted syncer remembers.
package integrity

import (
	"sort"
	"sync"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

// Ledger is the blobs of this replica's store found broken and not repaired since.
//
// Safe for concurrent use. The zero value is empty and ready.
type Ledger struct {
	mu  sync.Mutex
	bad map[string]struct{}

	// changed is told after a change, outside the lock.
	changed []func()
}

// OnChange registers f to be told after the ledger changes. Not for a function that changes it.
func (l *Ledger) OnChange(f func()) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.changed = append(l.changed, f)
}

// Add records broken blobs, and reports whether any was not recorded yet.
func (l *Ledger) Add(digests ...string) bool {
	return l.update(func() bool {
		added := false
		for _, digest := range digests {
			if _, known := l.bad[digest]; known || digest == "" {
				continue
			}
			if l.bad == nil {
				l.bad = map[string]struct{}{}
			}
			l.bad[digest] = struct{}{}
			added = true
		}
		return added
	})
}

// Remove forgets blobs found whole again, replaced, or gone from the store, and reports whether any
// was recorded.
func (l *Ledger) Remove(digests ...string) bool {
	return l.update(func() bool {
		removed := false
		for _, digest := range digests {
			if _, known := l.bad[digest]; known {
				delete(l.bad, digest)
				removed = true
			}
		}
		return removed
	})
}

// Retain forgets every blob for which keep is false, and reports whether any was forgotten. For the
// scrub, which at the end of a pass knows every blob the store still holds.
func (l *Ledger) Retain(keep func(digest string) bool) bool {
	return l.update(func() bool {
		removed := false
		for digest := range l.bad {
			if !keep(digest) {
				delete(l.bad, digest)
				removed = true
			}
		}
		return removed
	})
}

func (l *Ledger) update(change func() bool) bool {
	l.mu.Lock()
	changed := change()
	listeners := l.changed
	l.mu.Unlock()

	if changed {
		for _, f := range listeners {
			f()
		}
	}
	return changed
}

// Has reports whether the blob is recorded as broken.
func (l *Ledger) Has(digest string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	_, known := l.bad[digest]
	return known
}

// Len is how many blobs are recorded.
func (l *Ledger) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.bad)
}

// Snapshot is the recorded blobs as a set the caller owns. Nil when there are none, so that a reader
// that asks "any?" pays nothing.
func (l *Ledger) Snapshot() map[string]bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.bad) == 0 {
		return nil
	}
	snapshot := make(map[string]bool, len(l.bad))
	for digest := range l.bad {
		snapshot[digest] = true
	}
	return snapshot
}

// List is the recorded blobs, sorted, at most registryv1alpha1.MaxCorruptedBlobs of them: what the
// status carries.
func (l *Ledger) List() []string {
	l.mu.Lock()
	list := make([]string, 0, len(l.bad))
	for digest := range l.bad {
		list = append(list, digest)
	}
	l.mu.Unlock()

	sort.Strings(list)
	if len(list) > registryv1alpha1.MaxCorruptedBlobs {
		list = list[:registryv1alpha1.MaxCorruptedBlobs]
	}
	return list
}
