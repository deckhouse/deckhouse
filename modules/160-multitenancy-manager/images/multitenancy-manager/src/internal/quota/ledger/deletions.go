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

package ledger

import (
	"fmt"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// deletionMemory is how long a delete is remembered: a quorum list is compared with the informers for
// relistAfter before it is taken again, so a delete that came after the list is still known while
// that list is in use.
const deletionMemory = 2 * relistAfter

// deletions remembers for a while the deletes that the event handlers have handed to the state. An
// object that a quorum list returned and that is deleted afterwards is gone from the informer and from
// the state, which is right: the state has its delete, newer than what the list saw. Without this
// record such an object would look like an informer lagging behind the list, and under a steady stream
// of deletes a new leader would never count itself caught up.
type deletions struct {
	mu        sync.Mutex
	seen      map[deletionKey]deletion
	lastPrune time.Time
	now       func() time.Time
}

type deletionKey struct {
	kind      string
	namespace string
	name      string
}

type deletion struct {
	rv uint64
	at time.Time
}

// record notes the delete of the object. Call it after the state has applied the delete.
func (d *deletions) record(object metav1.Object) {
	d.mu.Lock()
	defer d.mu.Unlock()
	now := d.clock()
	if d.seen == nil {
		d.seen = map[deletionKey]deletion{}
	}
	if now.Sub(d.lastPrune) >= time.Second {
		for key, entry := range d.seen {
			if now.Sub(entry.at) > deletionMemory {
				delete(d.seen, key)
			}
		}
		d.lastPrune = now
	}
	key := keyOfDeletion(object)
	rv := ParseResourceVersion(object.GetResourceVersion())
	if entry, ok := d.seen[key]; ok && entry.rv > rv {
		rv = entry.rv
	}
	d.seen[key] = deletion{rv: rv, at: now}
}

// remaining returns the listed objects whose delete has not reached the state: the ones never
// deleted, and the ones deleted at a version older than the listed one, that is, before the list and
// created again since.
func (d *deletions) remaining(listed []metav1.Object) []metav1.Object {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]metav1.Object, 0, len(listed))
	for _, object := range listed {
		entry, ok := d.seen[keyOfDeletion(object)]
		if ok && entry.rv >= ParseResourceVersion(object.GetResourceVersion()) {
			continue
		}
		out = append(out, object)
	}
	return out
}

func (d *deletions) clock() time.Time {
	if d.now != nil {
		return d.now()
	}
	return time.Now()
}

func keyOfDeletion(object metav1.Object) deletionKey {
	kind := fmt.Sprintf("%T", object)
	if u, ok := object.(*unstructured.Unstructured); ok {
		kind = u.GroupVersionKind().GroupKind().String()
	}
	return deletionKey{kind: kind, namespace: object.GetNamespace(), name: object.GetName()}
}
