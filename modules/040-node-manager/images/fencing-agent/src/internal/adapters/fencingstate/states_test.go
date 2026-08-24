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
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-agent/internal/domain"
)

const (
	testNodeGroup = "worker"
	testPeerName  = "worker-3"
	testPeerUID   = "1a2b3c4d-1111-2222-3333-444455556666"
)

func testPeer() domain.Peer {
	return domain.Peer{Name: testPeerName, IP: "10.0.0.3", UID: testPeerUID}
}

func newStates(t *testing.T, funcs interceptor.Funcs, objects ...client.Object) (*States, client.Client) {
	t.Helper()

	c := fake.NewClientBuilder().
		WithScheme(newScheme(t)).
		WithStatusSubresource(&v1alpha1.FencingFailedNodeState{}).
		WithObjects(objects...).
		WithInterceptorFuncs(funcs).
		Build()

	return NewStates(c, c, testNodeGroup, v1alpha1.ProfileCritical, time.Second), c
}

func stored(t *testing.T, c client.Client, name string) *v1alpha1.FencingFailedNodeState {
	t.Helper()

	var state v1alpha1.FencingFailedNodeState
	if err := c.Get(context.Background(), types.NamespacedName{Name: name}, &state); err != nil {
		t.Fatalf("read back %q: %v", name, err)
	}

	return &state
}

func failedSection() v1alpha1.FencingFailedNodeStateFailed {
	return v1alpha1.FencingFailedNodeStateFailed{
		DetectedAt: metav1.NewTime(time.Date(2026, 6, 2, 15, 0, 1, 0, time.UTC)),
		DetectedBy: "worker-1",
		Reason:     v1alpha1.FailedReasonMemberlistDead,
		AliveCount: 3,
		QuorumSize: 3,
	}
}

func TestCreateFillsIdentityAndSpec(t *testing.T) {
	states, c := newStates(t, interceptor.Funcs{})

	created, err := states.Create(t.Context(), testPeer())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if !created {
		t.Error("Create reported no change after creating the object")
	}

	state := stored(t, c, testPeerName)

	if state.Name != testPeerName {
		t.Errorf("metadata.name = %q, want the node name %q", state.Name, testPeerName)
	}

	if got := state.Labels[domain.NodeGroupLabel]; got != testNodeGroup {
		t.Errorf("label %s = %q, want %q", domain.NodeGroupLabel, got, testNodeGroup)
	}

	if len(state.OwnerReferences) != 1 {
		t.Fatalf("got %d owner references, want exactly one to the Node", len(state.OwnerReferences))
	}

	owner := state.OwnerReferences[0]
	if owner.APIVersion != "v1" || owner.Kind != "Node" || owner.Name != testPeerName || string(owner.UID) != testPeerUID {
		t.Errorf("owner reference = %+v, want v1/Node/%s with uid %s", owner, testPeerName, testPeerUID)
	}

	if state.Spec.NodeGroup != testNodeGroup || state.Spec.ProfileRef.Name != v1alpha1.ProfileCritical {
		t.Errorf("spec = %+v, want nodeGroup %q and profile %q", state.Spec, testNodeGroup, v1alpha1.ProfileCritical)
	}

	if state.Status.Failed != nil || state.Status.Phase != "" {
		t.Errorf("create wrote status %+v, it belongs to the /status request and to the controller", state.Status)
	}
}

func TestCreateRefusesAPeerWithoutUID(t *testing.T) {
	states, _ := newStates(t, interceptor.Funcs{})

	peer := testPeer()
	peer.UID = ""

	if _, err := states.Create(t.Context(), peer); err == nil {
		t.Error("create accepted a peer without a UID, the owner reference would be unusable")
	}
}

func TestCreateAcceptsAnObjectAnotherWriterMade(t *testing.T) {
	existing := &v1alpha1.FencingFailedNodeState{ObjectMeta: metav1.ObjectMeta{Name: testPeerName}}
	states, _ := newStates(t, interceptor.Funcs{}, existing)

	created, err := states.Create(t.Context(), testPeer())
	if err != nil {
		t.Errorf("create over an existing object: %v, want it treated as done", err)
	}

	if created {
		t.Error("Create claimed it made an object another writer had already made")
	}
}

func TestMarkFailedWritesThroughTheStatusSubresource(t *testing.T) {
	// The fake client is built with a status subresource, so a plain update
	// would silently drop the section: seeing it stored proves the /status path.
	existing := &v1alpha1.FencingFailedNodeState{
		ObjectMeta: metav1.ObjectMeta{Name: testPeerName},
		Spec:       v1alpha1.FencingFailedNodeStateSpec{NodeGroup: testNodeGroup},
	}
	states, c := newStates(t, interceptor.Funcs{}, existing)

	if _, err := states.MarkFailed(t.Context(), testPeerName, failedSection()); err != nil {
		t.Fatalf("mark failed: %v", err)
	}

	state := stored(t, c, testPeerName)

	if state.Status.Failed == nil {
		t.Fatal("failed section was not stored")
	}

	if state.Status.Failed.DetectedBy != "worker-1" || state.Status.Failed.AliveCount != 3 {
		t.Errorf("failed section = %+v, want the one that was written", state.Status.Failed)
	}
}

func TestMarkFailedKeepsTheFirstDetection(t *testing.T) {
	// Whoever detected the failure first owns detectedAt: moving it forward would
	// push the evacuation deadline away with it.
	first := failedSection()
	existing := &v1alpha1.FencingFailedNodeState{
		ObjectMeta: metav1.ObjectMeta{Name: testPeerName},
		Status:     v1alpha1.FencingFailedNodeStateStatus{Failed: &first},
	}
	states, c := newStates(t, interceptor.Funcs{}, existing)

	later := failedSection()
	later.DetectedBy = "worker-2"
	later.DetectedAt = metav1.NewTime(first.DetectedAt.Add(time.Minute))

	recorded, err := states.MarkFailed(t.Context(), testPeerName, later)
	if err != nil {
		t.Fatalf("mark failed: %v", err)
	}

	if recorded {
		t.Error("MarkFailed claimed it wrote a section that was already there")
	}

	if got := stored(t, c, testPeerName).Status.Failed; got.DetectedBy != "worker-1" || !got.DetectedAt.Equal(&first.DetectedAt) {
		t.Errorf("failed section = %+v, want the first detection kept", got)
	}
}

func TestMarkFailedReReadsTheObjectBeforeRetrying(t *testing.T) {
	// A conflict means the object moved on — here because the controller wrote
	// the phase. A retry that re-sends the object it already had would roll that
	// phase back, so the retry has to start from a fresh read.
	gets := 0
	conflicts := 0

	funcs := interceptor.Funcs{
		Get: func(
			ctx context.Context,
			c client.WithWatch,
			key client.ObjectKey,
			obj client.Object,
			opts ...client.GetOption,
		) error {
			gets++

			return c.Get(ctx, key, obj, opts...)
		},
		SubResourceUpdate: func(
			ctx context.Context,
			c client.Client,
			subResourceName string,
			obj client.Object,
			opts ...client.SubResourceUpdateOption,
		) error {
			if conflicts > 0 {
				return c.Status().Update(ctx, obj, opts...)
			}

			conflicts++

			var current v1alpha1.FencingFailedNodeState
			if err := c.Get(ctx, types.NamespacedName{Name: obj.GetName()}, &current); err != nil {
				return err
			}

			current.Status.Phase = v1alpha1.PhaseSuspected

			if err := c.Status().Update(ctx, &current); err != nil {
				return err
			}

			return apierrors.NewConflict(
				schema.GroupResource{Group: v1alpha1.GroupVersion.Group, Resource: "fencingfailednodestates"},
				obj.GetName(),
				context.DeadlineExceeded,
			)
		},
	}

	existing := &v1alpha1.FencingFailedNodeState{ObjectMeta: metav1.ObjectMeta{Name: testPeerName}}
	states, c := newStates(t, funcs, existing)

	recorded, err := states.MarkFailed(t.Context(), testPeerName, failedSection())
	if err != nil {
		t.Fatalf("mark failed did not survive a conflict: %v", err)
	}

	if !recorded {
		t.Error("MarkFailed reported no change after writing the section")
	}

	if conflicts != 1 {
		t.Errorf("the conflict was not exercised, got %d", conflicts)
	}

	if gets < 2 {
		t.Errorf("the object was read %d times, want a fresh read before the retry", gets)
	}

	state := stored(t, c, testPeerName)

	if state.Status.Failed == nil {
		t.Error("failed section is missing after the retry")
	}

	if state.Status.Phase != v1alpha1.PhaseSuspected {
		t.Errorf("phase = %q, want the controller's %q kept: the retry rolled it back",
			state.Status.Phase, v1alpha1.PhaseSuspected)
	}
}

func TestDeleteSendsTheUIDPrecondition(t *testing.T) {
	// The fake client does not enforce a UID precondition, so this checks that
	// the request carries it: without it a slow delete could take the object of
	// a Node that has already been recreated.
	var got *types.UID

	funcs := interceptor.Funcs{
		Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			options := &client.DeleteOptions{}
			options.ApplyOptions(opts)

			if options.Preconditions != nil {
				got = options.Preconditions.UID
			}

			return c.Delete(ctx, obj, opts...)
		},
	}

	existing := &v1alpha1.FencingFailedNodeState{
		ObjectMeta: metav1.ObjectMeta{Name: testPeerName, UID: types.UID(testPeerUID)},
	}
	states, c := newStates(t, funcs, existing)

	if err := states.Delete(t.Context(), testPeerName, types.UID(testPeerUID)); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if got == nil || string(*got) != testPeerUID {
		t.Errorf("delete precondition uid = %v, want %q", got, testPeerUID)
	}

	var state v1alpha1.FencingFailedNodeState
	if err := c.Get(t.Context(), types.NamespacedName{Name: testPeerName}, &state); !apierrors.IsNotFound(err) {
		t.Errorf("object still present after delete: %v", err)
	}
}

func TestDeleteTreatsAMissingObjectAsDone(t *testing.T) {
	states, _ := newStates(t, interceptor.Funcs{})

	if err := states.Delete(t.Context(), testPeerName, types.UID(testPeerUID)); err != nil {
		t.Errorf("delete of a missing object returned %v, want it treated as done", err)
	}
}

func TestListIsScopedToTheNodeGroup(t *testing.T) {
	mine := &v1alpha1.FencingFailedNodeState{
		ObjectMeta: metav1.ObjectMeta{Name: testPeerName, Labels: map[string]string{domain.NodeGroupLabel: testNodeGroup}},
	}
	other := &v1alpha1.FencingFailedNodeState{
		ObjectMeta: metav1.ObjectMeta{Name: "master-1", Labels: map[string]string{domain.NodeGroupLabel: "master"}},
	}
	states, _ := newStates(t, interceptor.Funcs{}, mine, other)

	list, err := states.List(t.Context())
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(list) != 1 || list[0].Name != testPeerName {
		t.Errorf("list returned %d objects %v, want only the ones of this NodeGroup", len(list), list)
	}
}
