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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestDeletionsRemaining(t *testing.T) {
	vm := new(unstructured.Unstructured)
	vm.SetGroupVersionKind(schema.GroupVersionKind{Group: "virtualization.deckhouse.io", Version: "v1alpha2", Kind: "VirtualMachine"})
	vm.SetNamespace("a")
	vm.SetName("d8-project-quota")
	vm.SetResourceVersion("30")
	tests := []struct {
		name     string
		deleted  []metav1.Object
		expected []string
	}{
		{name: "nothing deleted", expected: []string{"a", "b"}},
		{name: "deleted after the list", deleted: []metav1.Object{object("b", "25")}, expected: []string{"a"}},
		{name: "deleted at the listed version", deleted: []metav1.Object{object("b", "12")}, expected: []string{"a"}},
		{name: "deleted before the list and created again", deleted: []metav1.Object{object("b", "11")}, expected: []string{"a", "b"}},
		{name: "another kind of the same name", deleted: []metav1.Object{vm}, expected: []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d deletions
			for _, o := range tt.deleted {
				d.record(o)
			}
			names := []string{}
			for _, o := range d.remaining([]metav1.Object{object("a", "10"), object("b", "12")}) {
				names = append(names, o.GetNamespace())
			}
			assert.Equal(t, tt.expected, names)
		})
	}
}

// A delete is forgotten deletionMemory after it is recorded, so the record does not grow with the
// churn of the cluster.
func TestDeletionsForget(t *testing.T) {
	now := time.Unix(1000, 0)
	d := deletions{now: func() time.Time { return now }}
	d.record(object("a", "25"))
	now = now.Add(deletionMemory + time.Second)
	d.record(object("b", "26"))

	assert.Len(t, d.seen, 1)
	assert.Len(t, d.remaining([]metav1.Object{object("a", "10")}), 1, "the delete of a is forgotten")
	assert.Empty(t, d.remaining([]metav1.Object{object("b", "12")}))
}

// replica plays the part of the informers of a replica: every event updates the store, then the
// state, then the record of the deletes, in the order the event handlers do it.
type replica struct {
	state   *State
	store   map[string]*corev1.ResourceQuota
	deleted deletions
}

func newReplica() *replica {
	return &replica{state: state(rl("pods", "100"), nil, nil), store: map[string]*corev1.ResourceQuota{}}
}

func (r *replica) set(q *corev1.ResourceQuota) {
	r.store[q.Namespace] = q
	r.state.SetNamespace(ns(q.Namespace, "p"))
	r.state.SetQuota(q)
}

func (r *replica) delete(q *corev1.ResourceQuota) {
	delete(r.store, q.Namespace)
	r.state.DeleteQuota(q.Namespace)
	r.deleted.record(q)
}

// covered is what runner.covered decides for one quorum list.
func (r *replica) covered(listed []metav1.Object, listRV uint64) bool {
	remaining := r.deleted.remaining(listed)
	cached := make([]metav1.Object, 0, len(r.store))
	for _, q := range r.store {
		cached = append(cached, q)
	}
	return r.state.Covers(remaining) && caughtUp(remaining, cached, listRV)
}

func TestCoveredWithDeletes(t *testing.T) {
	t.Run("a delete between the list and the check", func(t *testing.T) {
		r := newReplica()
		a, b := rq("a", 10, nil), rq("b", 12, nil)
		r.set(a)
		r.set(b)
		listed := []metav1.Object{a, b}

		r.delete(rq("b", 25, nil))

		assert.True(t, r.covered(listed, 20))
	})

	t.Run("a steady stream of creates and deletes", func(t *testing.T) {
		r := newReplica()
		listed := []metav1.Object{}
		for i := range 10 {
			q := rq(fmt.Sprintf("n%d", i), uint64(10+i), nil)
			r.set(q)
			listed = append(listed, q)
		}
		rv := uint64(20)
		for i := range 50 {
			rv++
			created := rq(fmt.Sprintf("new%d", i), rv, nil)
			r.set(created)
			rv++
			r.delete(rq(created.Namespace, rv, nil))
			if i < 10 {
				rv++
				r.delete(rq(fmt.Sprintf("n%d", i), rv, nil))
			}
			assert.True(t, r.covered(listed, 20), "after event %d", i)
		}
	})

	t.Run("an informer behind the list", func(t *testing.T) {
		r := newReplica()
		a := rq("a", 10, nil)
		r.set(a)
		listed := []metav1.Object{a, rq("b", 12, nil)}

		assert.False(t, r.covered(listed, 20), "b is not in the informer yet")

		r.set(rq("b", 9, nil))
		assert.False(t, r.covered(listed, 20), "b is older in the informer than in the list")

		r.set(rq("b", 12, nil))
		assert.True(t, r.covered(listed, 20))
	})

	t.Run("a delete the informer has not seen", func(t *testing.T) {
		r := newReplica()
		a, b := rq("a", 10, nil), rq("b", 12, nil)
		r.set(a)
		r.set(b)
		listed := []metav1.Object{a}

		assert.False(t, r.covered(listed, 20), "b was deleted before the list, the informer still has it")
	})
}
