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

package source

import (
	"context"
	"sync"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"

	"github.com/deckhouse/deckhouse/go_lib/user-authz/rules"
)

func car(name, rv string, subjects []string, limit ...string) *unstructured.Unstructured {
	subs := make([]interface{}, 0, len(subjects))
	for _, s := range subjects {
		subs = append(subs, map[string]interface{}{"kind": "User", "name": s})
	}
	limits := make([]interface{}, 0, len(limit))
	for _, l := range limit {
		limits = append(limits, l)
	}
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "deckhouse.io/v1",
		"kind":       "ClusterAuthorizationRule",
		"metadata":   map[string]interface{}{"name": name, "resourceVersion": rv},
		"spec":       map[string]interface{}{"subjects": subs, "limitNamespaces": limits},
	}}
}

type recorder struct {
	mu      sync.Mutex
	updates []rules.Stats
	synced  []bool
}

func (r *recorder) DirectoryRebuilt(stats rules.Stats, _ time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.updates = append(r.updates, stats)
}
func (r *recorder) SyncedChanged(s bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.synced = append(r.synced, s)
}
func (r *recorder) WatchError(error) {}

func (r *recorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.updates)
}

func newClient(objs ...runtime.Object) *fake.FakeDynamicClient {
	scheme := runtime.NewScheme()
	gvrToKind := map[schema.GroupVersionResource]string{rules.GroupVersionResource: "ClusterAuthorizationRuleList"}
	return fake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToKind, objs...)
}

func metav1CreateOptions() metav1.CreateOptions { return metav1.CreateOptions{} }
func metav1DeleteOptions() metav1.DeleteOptions { return metav1.DeleteOptions{} }

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestSource_BuildsAndFollowsChanges(t *testing.T) {
	client := newClient(car("team-a", "1", []string{"alice"}, "team-a"))
	rec := &recorder{}
	s := New(client, Options{Debounce: 20 * time.Millisecond, Observer: rec, Logf: t.Logf})

	if s.Directory() != nil || s.HasSynced() || s.State() != StateUnsynced {
		t.Fatalf("before Run the source knows nothing")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Run(ctx)

	eventually(t, "first directory", func() bool { return s.Directory() != nil })
	if !s.HasSynced() || s.State() != StateSynced {
		t.Fatalf("synced=%v state=%v", s.HasSynced(), s.State())
	}
	dir := s.Directory()
	if !dir.KnowsRule("team-a") || len(dir.Lookup("alice", nil)) != 1 {
		t.Fatalf("directory does not reflect the initial list")
	}

	// a new rule is folded in within the debounce window
	_, err := client.Resource(rules.GroupVersionResource).Create(ctx, car("team-b", "2", []string{"bob"}, "team-b"), metav1CreateOptions())
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, "team-b", func() bool { return s.Directory().KnowsRule("team-b") })
	if s.Directory().MaxResourceVersion() < 2 {
		t.Errorf("watermark = %d", s.Directory().MaxResourceVersion())
	}

	// a deleted rule leaves the directory
	if err := client.Resource(rules.GroupVersionResource).Delete(ctx, "team-a", metav1DeleteOptions()); err != nil {
		t.Fatal(err)
	}
	eventually(t, "team-a gone", func() bool { return !s.Directory().KnowsRule("team-a") })
	if len(s.Directory().Lookup("alice", nil)) != 0 {
		t.Errorf("alice must have no entry after her rule is deleted")
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.synced) != 1 || !rec.synced[0] {
		t.Errorf("SyncedChanged must be reported once: %v", rec.synced)
	}
	if len(rec.updates) < 3 {
		t.Errorf("expected at least three rebuilds, got %d", len(rec.updates))
	}
}

func TestSource_DebounceCoalescesBursts(t *testing.T) {
	client := newClient()
	rec := &recorder{}
	s := New(client, Options{Debounce: 150 * time.Millisecond, Observer: rec})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Run(ctx)
	eventually(t, "initial build", func() bool { return s.Directory() != nil })
	base := rec.count()

	for i := 0; i < 20; i++ {
		name := "burst-" + string(rune('a'+i))
		if _, err := client.Resource(rules.GroupVersionResource).Create(ctx, car(name, "10", []string{"u"}, name), metav1CreateOptions()); err != nil {
			t.Fatal(err)
		}
	}
	eventually(t, "burst folded", func() bool { return s.Directory().Len() == 20 })
	time.Sleep(200 * time.Millisecond)
	if rebuilds := rec.count() - base; rebuilds > 3 {
		t.Errorf("a burst of 20 creates caused %d rebuilds, want a handful", rebuilds)
	}
}

func TestSource_QuarantineIsReported(t *testing.T) {
	client := newClient(car("broken", "1", []string{"dave"}, "team-("))
	rec := &recorder{}
	s := New(client, Options{Debounce: 20 * time.Millisecond, Observer: rec})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Run(ctx)
	eventually(t, "directory", func() bool { return s.Directory() != nil })
	if q := s.Directory().Quarantined(); len(q) != 1 || q["broken"] == nil {
		t.Errorf("quarantined = %v", q)
	}
	entries := s.Directory().Lookup("dave", nil)
	if len(entries) != 1 || !entries[0].Quarantined {
		t.Errorf("dave = %+v", entries)
	}
}
