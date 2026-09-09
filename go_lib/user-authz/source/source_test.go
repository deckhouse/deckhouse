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
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"

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
	mu          sync.Mutex
	updates     []rules.Stats
	synced      []bool
	watchErrors []error
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
func (r *recorder) WatchError(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.watchErrors = append(r.watchErrors, err)
}

func (r *recorder) watchErrorCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.watchErrors)
}

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

// quiesce waits until the rebuild count stops moving and returns it.
func quiesce(t *testing.T, rec *recorder) int {
	t.Helper()
	last := -1
	for i := 0; i < 50; i++ {
		time.Sleep(20 * time.Millisecond)
		if got := rec.count(); got == last {
			return got
		} else {
			last = got
		}
	}
	t.Fatalf("the source never stopped rebuilding")
	return 0
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

// A cluster without the ClusterAuthorizationRule CRD is a normal bootstrap state, not a failure:
// the source must keep retrying, report it as such, and never publish an empty directory that a
// consumer would mistake for "there are no rules".
func TestSource_CRDMissing(t *testing.T) {
	client := newClient()
	client.PrependReactor("list", "clusterauthorizationrules", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(rules.GroupVersionResource.GroupResource(), "")
	})
	client.PrependWatchReactor("clusterauthorizationrules", func(k8stesting.Action) (bool, watch.Interface, error) {
		return true, nil, apierrors.NewNotFound(rules.GroupVersionResource.GroupResource(), "")
	})

	rec := &recorder{}
	s := New(client, Options{Debounce: 20 * time.Millisecond, Observer: rec, Logf: t.Logf})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Run(ctx)

	eventually(t, "the missing CRD to be noticed", func() bool { return rec.watchErrorCount() > 0 })

	if got := s.State(); got != StateCRDMissing {
		t.Errorf("state = %v, want %v", got, StateCRDMissing)
	}
	if s.HasSynced() {
		t.Error("the source cannot be synced without the CRD")
	}
	if s.Directory() != nil {
		t.Error("no directory may be published: nil means \"the rules are not known\", an empty one means \"there are none\"")
	}
	if s.LastError() == nil {
		t.Error("the error must be readable")
	}
	// A nil directory answers every question conservatively.
	if s.Directory().RuleCovers("team-a", "alice", nil) {
		t.Error("a directory that does not exist covers nobody")
	}
}

// A list/watch failure that is not a missing CRD is reported as such, and the source keeps the
// directory it already had rather than dropping the rules of the cluster.
func TestSource_WatchErrorKeepsDirectory(t *testing.T) {
	client := newClient(car("team-a", "1", []string{"alice"}, "team-a"))
	rec := &recorder{}
	s := New(client, Options{Debounce: 20 * time.Millisecond, Observer: rec, Logf: t.Logf})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Run(ctx)

	eventually(t, "the first directory", func() bool { return s.Directory() != nil })
	if got := s.State(); got != StateSynced {
		t.Fatalf("state = %v, want %v", got, StateSynced)
	}

	s.watchError(nil, errors.New("connection refused"))

	if s.Directory() == nil {
		t.Error("a watch error must not drop the rules already known")
	}
	if !s.Directory().KnowsRule("team-a") {
		t.Error("the directory must still hold the rule")
	}
	if got := s.State(); got != StateSynced {
		t.Errorf("state = %v: an unreachable API server is not a missing CRD", got)
	}
	if rec.watchErrorCount() == 0 {
		t.Error("the error must reach the observer, it is what the alert counts")
	}
}

// A watch that keeps failing eventually means the directory has stopped tracking the cluster, and
// saying so is the difference between a stale answer and a wrong one. What must NOT happen is
// reporting it early: this state gates the readiness of a fail-closed authorizer on every master.
func TestSource_StaleAfterEnoughConsecutiveWatchErrors(t *testing.T) {
	client := newClient(car("team-a", "1", []string{"alice"}, "team-a"))
	s := New(client, Options{Debounce: 20 * time.Millisecond, DegradeAfterWatchErrors: 3, Logf: t.Logf})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Run(ctx)

	eventually(t, "the first directory", func() bool { return s.Directory() != nil })

	// Below the threshold nothing changes. A handful of transient errors is a normal day.
	s.watchError(nil, errors.New("connection refused"))
	s.watchError(nil, errors.New("connection refused"))
	if got := s.State(); got != StateSynced {
		t.Fatalf("state = %v after 2 of 3 errors, want %v: the threshold must not be eager", got, StateSynced)
	}

	s.watchError(nil, errors.New("connection refused"))
	if got := s.State(); got != StateStale {
		t.Errorf("state = %v after the threshold, want %v", got, StateStale)
	}

	// The directory itself is untouched: it is a real snapshot, and the webhook keeps answering
	// from it. Only readiness changes.
	if s.Directory() == nil || !s.Directory().KnowsRule("team-a") {
		t.Error("going stale must not drop the rules already known")
	}

	// And a successful sync clears the count, so a source that recovers reports healthy again
	// rather than staying stale for the life of the process.
	s.mu.Lock()
	s.consecutiveWatchErrors = 0
	s.mu.Unlock()
	if got := s.State(); got != StateSynced {
		t.Errorf("state = %v after recovery, want %v", got, StateSynced)
	}
}

// A source that has never listed the cluster is Unsynced, not Stale, however many errors it has
// seen: there is no directory to be stale about, and the two states mean different things to the
// consumers - Unsynced closes the webhook's serving gate, Stale deliberately does not.
func TestSource_NeverListedStaysUnsynced(t *testing.T) {
	s := New(newClient(), Options{Debounce: 20 * time.Millisecond, DegradeAfterWatchErrors: 2, Logf: t.Logf})
	s.watchError(nil, errors.New("connection refused"))
	s.watchError(nil, errors.New("connection refused"))
	s.watchError(nil, errors.New("connection refused"))

	if got := s.State(); got != StateUnsynced {
		t.Errorf("state = %v, want %v", got, StateUnsynced)
	}
}

// A missing CRD outranks staleness: there are no rules to track, so an empty directory is the
// truth and the consumer must not be held unready for it.
func TestSource_CRDMissingOutranksStale(t *testing.T) {
	s := New(newClient(), Options{Debounce: 20 * time.Millisecond, DegradeAfterWatchErrors: 1, Logf: t.Logf})
	s.synced.Store(true)
	s.watchError(nil, apierrors.NewNotFound(rules.GroupVersionResource.GroupResource(), ""))

	if got := s.State(); got != StateCRDMissing {
		t.Errorf("state = %v, want %v", got, StateCRDMissing)
	}
}

// Run returns when its context is cancelled.
func TestSource_RunStopsWithContext(t *testing.T) {
	s := New(newClient(car("team-a", "1", []string{"alice"}, "team-a")), Options{Debounce: 20 * time.Millisecond, Logf: t.Logf})
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()

	eventually(t, "the first directory", func() bool { return s.Directory() != nil })
	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after its context was cancelled")
	}
}

// The first directory is published as soon as the initial list completes. Waiting out the debounce
// window would be pure exposure: until the directory exists every subject of a rule is restricted.
func TestSource_FirstBuildSkipsDebounce(t *testing.T) {
	s := New(newClient(car("team-a", "1", []string{"alice"}, "team-a")),
		Options{Debounce: 30 * time.Second, Logf: t.Logf})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	started := time.Now()
	go s.Run(ctx)
	eventually(t, "the first directory", func() bool { return s.Directory() != nil })

	if took := time.Since(started); took > 5*time.Second {
		t.Errorf("the first build took %s: it waited out the debounce window", took)
	}
	if !s.Directory().KnowsRule("team-a") {
		t.Error("the first directory must hold the rule")
	}
}

// An update that leaves the fields the directory reads untouched does not rebuild it. At a few
// thousand rules a rebuild is not free, and a re-list or a label edit must not pay for one.
func TestSource_UnchangedSpecDoesNotRebuild(t *testing.T) {
	obj := car("team-a", "1", []string{"alice"}, "team-a")
	client := newClient(obj)
	rec := &recorder{}
	s := New(client, Options{Debounce: 20 * time.Millisecond, Observer: rec, Logf: t.Logf})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Run(ctx)

	eventually(t, "the first directory", func() bool { return s.Directory() != nil })
	// Let the startup settle so the count below measures only what the update causes.
	before := quiesce(t, rec)

	// Only a label changes; Project drops labels entirely, so the projected spec is untouched.
	labelled := obj.DeepCopy()
	labelled.SetResourceVersion("2")
	labelled.SetLabels(map[string]string{"touched": "yes"})
	if _, err := client.Resource(rules.GroupVersionResource).Update(ctx, labelled, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("update: %v", err)
	}

	// A rebuild would land within a few debounce windows; give it room and expect none.
	time.Sleep(300 * time.Millisecond)
	if got := rec.count(); got != before {
		t.Errorf("rebuilds went %d -> %d: a metadata-only update must not rebuild the directory", before, got)
	}

	// A real change still does rebuild it.
	changed := obj.DeepCopy()
	changed.SetResourceVersion("3")
	if err := unstructured.SetNestedStringSlice(changed.Object, []string{"team-b"}, "spec", "limitNamespaces"); err != nil {
		t.Fatalf("set limitNamespaces: %v", err)
	}
	if _, err := client.Resource(rules.GroupVersionResource).Update(ctx, changed, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("update: %v", err)
	}
	eventually(t, "the rebuild after a real change", func() bool { return rec.count() > before })
}

// Readers hit the directory while the informer replaces it. Run with -race.
func TestSource_ConcurrentReadsDuringRebuilds(t *testing.T) {
	client := newClient(car("team-a", "1", []string{"alice"}, "team-a"))
	s := New(client, Options{Debounce: time.Millisecond, Logf: func(string, ...interface{}) {}})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Run(ctx)
	eventually(t, "the first directory", func() bool { return s.Directory() != nil })

	stop := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				dir := s.Directory()
				dir.Lookup("alice", []string{"devs"})
				dir.RuleCovers("team-a", "alice", nil)
				dir.KnowsRule("team-a")
				_ = dir.Len()
				_ = dir.MaxResourceVersion()
				_ = s.HasSynced()
				_ = s.State()
			}
		}()
	}

	for i := 2; i < 40; i++ {
		rv := strconv.Itoa(i)
		obj := car("team-"+rv, rv, []string{"alice"}, "ns-"+rv)
		if _, err := client.Resource(rules.GroupVersionResource).Create(ctx, obj, metav1CreateOptions()); err != nil {
			t.Errorf("create: %v", err)
			break
		}
	}

	time.Sleep(200 * time.Millisecond)
	close(stop)
	wg.Wait()
}
