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

package fencingstate

import (
	"context"
	"sync"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
)

func newFakeClient(t *testing.T) *fake.ClientBuilder {
	t.Helper()

	scheme, err := newAPIScheme()
	if err != nil {
		t.Fatal(err)
	}

	return fake.NewClientBuilder().WithScheme(scheme)
}

// The delete keeps an object that changed after the read.
func TestDeleteFFNSChecksTheVersion(t *testing.T) {
	ctx := t.Context()
	i := NewIncidents(newFakeClient(t).Build())

	if err := i.CreateFFNS(ctx, &v1alpha1.FencingFailedNodeState{ObjectMeta: metav1.ObjectMeta{Name: "w-1"}}); err != nil {
		t.Fatal(err)
	}

	read, err := i.GetFFNS(ctx, "w-1")
	if err != nil {
		t.Fatal(err)
	}

	err = i.CreateFFNS(ctx, &v1alpha1.FencingFailedNodeState{ObjectMeta: metav1.ObjectMeta{Name: "w-1"}})
	if !apierrors.IsAlreadyExists(err) {
		t.Fatalf("second create: %v, want AlreadyExists", err)
	}

	if err := i.DeleteFFNS(ctx, "w-1", read.ResourceVersion+"1"); !apierrors.IsConflict(err) {
		t.Fatalf("delete with another version: %v, want a conflict", err)
	}

	if err := i.DeleteFFNS(ctx, "w-1", read.ResourceVersion); err != nil {
		t.Fatalf("delete with the read version: %v", err)
	}

	if _, err := i.GetFFNS(ctx, "w-1"); !apierrors.IsNotFound(err) {
		t.Errorf("get after delete: %v, want NotFound", err)
	}
}

// The status patch changes only the written part, keeps the parts of the
// controller and fails when the object changed after the read.
func TestPatchStatusIsCASAndKeepsTheOtherParts(t *testing.T) {
	ctx := t.Context()
	c := newFakeClient(t).WithStatusSubresource(&v1alpha1.FencingFailedNodeState{}).Build()
	i := NewIncidents(c)

	if err := i.CreateFFNS(ctx, &v1alpha1.FencingFailedNodeState{ObjectMeta: metav1.ObjectMeta{Name: "w-1", Labels: map[string]string{"l": "v"}}}); err != nil {
		t.Fatal(err)
	}

	// The controller writes its parts.
	ctrl, err := i.GetFFNS(ctx, "w-1")
	if err != nil {
		t.Fatal(err)
	}

	ctrl.Status.Phase = v1alpha1.PhaseFallbackAlive
	ctrl.Status.Correlation = &v1alpha1.FencingFailedNodeStateCorrelation{Episode: "e-1"}
	ctrl.Status.Conditions = []metav1.Condition{{Type: "Queued", Status: metav1.ConditionFalse, Reason: "Free", LastTransitionTime: metav1.Now()}}

	if err := c.Status().Update(ctx, ctrl); err != nil {
		t.Fatal(err)
	}

	old, err := i.GetFFNS(ctx, "w-1")
	if err != nil {
		t.Fatal(err)
	}

	at := metav1.NewMicroTime(time.Now())
	ffns := old.DeepCopy()
	ffns.Status.Fallback = &v1alpha1.FencingFailedNodeStateFallback{LastHeartbeatAt: &at}

	if err := i.PatchStatus(ctx, old, ffns); err != nil {
		t.Fatalf("patch on the read version: %v", err)
	}

	got, err := i.GetFFNS(ctx, "w-1")
	if err != nil {
		t.Fatal(err)
	}

	if got.Status.Fallback == nil || got.Status.Phase != v1alpha1.PhaseFallbackAlive || got.Status.Correlation == nil ||
		got.Status.Correlation.Episode != "e-1" || len(got.Status.Conditions) != 1 || got.Labels["l"] != "v" {
		t.Errorf("after the patch: status %+v, labels %v; want the heartbeat and the other parts unchanged", got.Status, got.Labels)
	}

	// old is now out of date.
	stale := old.DeepCopy()
	stale.Status.Attestation = &v1alpha1.FencingFailedNodeStateAttestation{Witness: "w-2", At: at}

	if err := i.PatchStatus(ctx, old, stale); !apierrors.IsConflict(err) {
		t.Fatalf("patch on an old version: %v, want a conflict", err)
	}
}

// noWatchList tells the informer that the fake client does not send the
// initial events of a watch.
type noWatchList struct {
	client.WithWatch
}

func (noWatchList) IsWatchListSemanticsUnSupported() bool { return true }

// The watch reports the FFNS of the own node only.
func TestOwnReportsTheOwnFFNS(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	c := newFakeClient(t).Build()

	var (
		mu  sync.Mutex
		got []bool
	)

	own, err := NewOwn(noWatchList{c}, "w-1", func(ffns *v1alpha1.FencingFailedNodeState) {
		mu.Lock()
		got = append(got, ffns != nil)
		mu.Unlock()
	})
	if err != nil {
		t.Fatal(err)
	}

	go func() { _ = own.Run(ctx) }()

	last := func(want bool) {
		t.Helper()

		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			mu.Lock()
			ok := len(got) > 0 && got[len(got)-1] == want
			mu.Unlock()

			if ok {
				return
			}

			time.Sleep(10 * time.Millisecond)
		}

		t.Fatalf("reports = %v, want the last one %v", got, want)
	}

	last(false)

	other := &v1alpha1.FencingFailedNodeState{ObjectMeta: metav1.ObjectMeta{Name: "w-2"}}
	if err := c.Create(ctx, other); err != nil {
		t.Fatal(err)
	}

	ffns := &v1alpha1.FencingFailedNodeState{ObjectMeta: metav1.ObjectMeta{Name: "w-1"}}
	if err := c.Create(ctx, ffns); err != nil {
		t.Fatal(err)
	}

	last(true)

	if err := c.Delete(ctx, ffns); err != nil {
		t.Fatal(err)
	}

	last(false)

	mu.Lock()
	defer mu.Unlock()

	if len(got) != 3 {
		t.Errorf("reports = %v, want [false true false]: the FFNS of another node is not reported", got)
	}
}
