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

package revocation

import (
	"context"
	"fmt"
	"math"
	"slices"
	"sync"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
)

const (
	janeSubject = "CgRqYW5lEgVsb2NhbA"
	jdoeSubject = "CiNjbj1qZG9lLG91PXBlb3BsZSxkYz1leGFtcGxlLGRjPW9yZxIEbGRhcA"
)

// revocationObject builds a Revocation object the way the contract of MR3 fixes it (section 2.2).
func revocationObject(name, revocationType, subject string, generation any) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "dex.coreos.com/v1",
		"kind":       "Revocation",
		"metadata": map[string]any{
			"name":      name,
			"namespace": Namespace,
			"labels": map[string]any{
				"user-authn.deckhouse.io/revocation-type": revocationType,
			},
		},
		"type":        revocationType,
		"connectorID": "local",
		"userID":      "jane",
		"subject":     subject,
		"clients":     []any{"dex-authenticator-console@d8-console"},
		"reason":      "Lock",
		"source":      "user-authn-controller",
		"createdAt":   "2026-10-16T10:00:00.123456789Z",
	}}
	if generation != nil {
		obj.Object["generation"] = generation
	}
	return obj
}

// newFakeClient lists objects and feeds the watch from the returned watcher. The watch is fed by
// hand: an object the fake client creates between the list and the watch of the informer would
// never reach it.
func newFakeClient(objects ...runtime.Object) (*fake.FakeDynamicClient, *watch.FakeWatcher) {
	client := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{GVR: "RevocationList"}, objects...)
	watcher := watch.NewFake()
	client.PrependWatchReactor("revocations", k8stesting.DefaultWatchReactor(watcher, nil))
	return client, watcher
}

// send delivers an event to the informer, failing the test if it does not take it.
func send(t *testing.T, event func()) {
	t.Helper()

	sent := make(chan struct{})
	go func() {
		defer close(sent)
		event()
	}()
	select {
	case <-sent:
	case <-time.After(5 * time.Second):
		t.Fatal("the watcher did not take the event")
	}
}

// recorder collects the revocations a watcher reports.
type recorder struct {
	mu    sync.Mutex
	pairs []string
}

func (r *recorder) revoke(subject string, generation uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pairs = append(r.pairs, fmt.Sprintf("%s@%d", subject, generation))
}

func (r *recorder) seen() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Sorted(slices.Values(r.pairs))
}

func startWatcher(t *testing.T, client *fake.FakeDynamicClient, rec *recorder) *Watcher {
	t.Helper()

	w, err := New(client, rec.revoke)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		w.Run(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	syncCtx, syncCancel := context.WithTimeout(ctx, 5*time.Second)
	defer syncCancel()
	if !cache.WaitForCacheSync(syncCtx.Done(), w.informer.HasSynced) {
		t.Fatal("the watcher did not sync")
	}
	return w
}

func eventually(t *testing.T, what string, condition func() bool) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestWatcher(t *testing.T) {
	client, events := newFakeClient(
		revocationObject("pair-jane-g2", "pair", janeSubject, int64(2)),
		revocationObject("pair-jane-g5", "pair", janeSubject, int64(5)),
		revocationObject("pair-jdoe-g1", "pair", jdoeSubject, int64(1)),
		// Sessions and grants end what basic auth never has.
		revocationObject("session-abc", "session", janeSubject, int64(9)),
		revocationObject("grant-abc", "grant", janeSubject, nil),
		// A pair revocation that cannot be read is skipped, not taken for generation 0.
		revocationObject("pair-broken", "pair", jdoeSubject, "7"),
	)
	rec := &recorder{}
	w := startWatcher(t, client, rec)

	eventually(t, "the listed revocations", func() bool { return len(rec.seen()) == 3 })
	if got, want := rec.seen(), []string{janeSubject + "@2", janeSubject + "@5", jdoeSubject + "@1"}; !slices.Equal(got, want) {
		t.Fatalf("revoked %v, want %v", got, want)
	}
	if got := w.Generation(janeSubject); got != 5 {
		t.Errorf("Generation(jane) = %d, want 5", got)
	}
	if got := w.Generation(jdoeSubject); got != 1 {
		t.Errorf("Generation(jdoe) = %d, want 1", got)
	}
	if got := w.Generation("unknown"); got != 0 {
		t.Errorf("Generation(unknown) = %d, want 0", got)
	}

	// A new revocation is reported as it is created.
	g7 := revocationObject("pair-jane-g7", "pair", janeSubject, int64(7))
	send(t, func() { events.Add(g7) })
	eventually(t, "the created revocation", func() bool { return slices.Contains(rec.seen(), janeSubject+"@7") })
	if got := w.Generation(janeSubject); got != 7 {
		t.Errorf("Generation(jane) = %d after the new revocation, want 7", got)
	}

	// Once the controller deletes an expired revocation, the generation comes from the others.
	send(t, func() { events.Delete(g7) })
	eventually(t, "the deletion", func() bool { return w.Generation(janeSubject) == 5 })
}

func TestWatcher_KeepsOnlyWhatItReads(t *testing.T) {
	client, _ := newFakeClient(revocationObject("pair-jane-g2", "pair", janeSubject, int64(2)))
	w := startWatcher(t, client, &recorder{})

	objs := w.informer.GetStore().List()
	if len(objs) != 1 {
		t.Fatalf("store holds %d objects, want 1", len(objs))
	}
	u, ok := objs[0].(*unstructured.Unstructured)
	if !ok {
		t.Fatalf("stored object is %T, want *unstructured.Unstructured", objs[0])
	}
	for _, field := range []string{"userID", "connectorID", "clients", "reason", "createdAt", "source"} {
		if _, found := u.Object[field]; found {
			t.Errorf("the stored revocation keeps %q", field)
		}
	}
	if len(u.GetLabels()) != 0 {
		t.Errorf("the stored revocation keeps the labels %v", u.GetLabels())
	}
}

func TestGenerationOf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   any
		want    uint64
		wantErr bool
	}{
		{name: "integer", value: int64(3), want: 3},
		{name: "integral float", value: float64(4), want: 4},
		{name: "zero", value: int64(0), want: 0},
		{name: "missing", value: nil, wantErr: true},
		{name: "negative", value: int64(-1), wantErr: true},
		{name: "fraction", value: 1.5, wantErr: true},
		{name: "too large", value: math.MaxFloat64, wantErr: true},
		{name: "string", value: "3", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := generationOf(tt.value)
			if (err != nil) != tt.wantErr {
				t.Fatalf("generationOf(%v) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("generationOf(%v) = %d, want %d", tt.value, got, tt.want)
			}
		})
	}
}
