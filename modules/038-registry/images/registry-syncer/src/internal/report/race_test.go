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

package report

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

// staleFirstRead is a client that answers the first Get with what the object looked like before
// somebody else wrote to it.
//
// That is the whole race, and it needs no timing to reproduce: every replica of the storage reports
// into one object, and each one reads it, changes its own entry and writes the result back. A read
// that happened a moment too early is indistinguishable from a fresh one, so the writer sends a
// status built from a state that no longer exists.
type staleFirstRead struct {
	client.Client

	stale *registryv1alpha1.RegistryStorage
	used  bool
}

func (c *staleFirstRead) Get(
	ctx context.Context, key client.ObjectKey, object client.Object, opts ...client.GetOption,
) error {
	if storage, ok := object.(*registryv1alpha1.RegistryStorage); ok && !c.used {
		c.used = true
		c.stale.DeepCopyInto(storage)
		return nil
	}
	return c.Client.Get(ctx, key, object, opts...)
}

func (c *staleFirstRead) Status() client.SubResourceWriter {
	return c.Client.Status()
}

// TestConcurrentReportsDoNotOverwriteEachOther is the lost report.
//
// `status.replicas` is a list, and a merge patch of a list replaces the whole list — so a replica
// writing its own entry writes every other entry too, as it last saw them. Two replicas reporting at
// the same time therefore do not merge: the later write puts back the earlier reader's picture of
// the other replicas, and whatever they published in between is gone.
//
// It is not cosmetic. `safeToDropUpstream` is derived by the controller from these entries, and the
// air-gap transition is gated on it: a leader's `full: false`, or its withdrawal of that permission,
// can be reverted by a follower's ordinary progress report — leaving the permission standing on a
// fact that has stopped being one, which is how a cluster gets cut off from its upstream while its
// store is incomplete. The mirror case is as bad and quieter: a leader's `full: true` overwritten by
// a stale copy leaves the transition waiting forever on a store that is finished.
func TestConcurrentReportsDoNotOverwriteEachOther(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, registryv1alpha1.AddToScheme(scheme))

	// What the follower saw when it started reporting: only the leader's entry, and it was not
	// full yet.
	object := storageObject(registryv1alpha1.StorageReplicaStatus{
		Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: false,
	})

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(object).
		WithStatusSubresource(&registryv1alpha1.RegistryStorage{}).
		Build()

	stale := getStorage(t, fakeClient)

	// Meanwhile the leader finishes its fill and says so.
	leader := &Publisher{Client: fakeClient}
	require.NoError(t, leader.Publish(context.Background(), State{
		Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, VerifiedDigests: 400,
	}))

	// And only now does the follower write, from the state it read before that.
	follower := &Publisher{Client: &staleFirstRead{Client: fakeClient, stale: stale}}
	require.NoError(t, follower.Publish(context.Background(), State{
		Node: "master-1", Role: registryv1alpha1.ReplicaRoleFollower, Full: false, Source: "master-0",
	}))

	replicas := getReplicas(t, fakeClient)
	require.Len(t, replicas, 2, "both replicas speak for themselves, so both entries must be there")

	byNode := map[string]registryv1alpha1.StorageReplicaStatus{}
	for _, replica := range replicas {
		byNode[replica.Node] = replica
	}

	assert.True(t, byNode["master-0"].Full,
		"the leader reported a complete store; a follower's report must not take that back")
	assert.Equal(t, int32(400), byNode["master-0"].VerifiedDigests)
	assert.False(t, byNode["master-1"].Full)
}

// TestAWithdrawalSurvivesAConcurrentReport is the same race on the field the cluster's safety rests
// on, in the direction that cuts a cluster off from its upstream.
func TestAWithdrawalSurvivesAConcurrentReport(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, registryv1alpha1.AddToScheme(scheme))

	object := storageObject(registryv1alpha1.StorageReplicaStatus{
		Node: "master-1", Role: registryv1alpha1.ReplicaRoleFollower,
	})
	object.Status.SafeToDropUpstream = true
	object.Status.AllReplicasFull = true

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(object).
		WithStatusSubresource(&registryv1alpha1.RegistryStorage{}).
		Build()

	stale := getStorage(t, fakeClient)

	// The leader reads the store, finds it no longer complete, and takes the permission back.
	leader := &Publisher{Client: fakeClient}
	require.NoError(t, leader.Publish(context.Background(), State{
		Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: false,
	}))
	require.False(t, getStorage(t, fakeClient).Status.SafeToDropUpstream)

	// A follower's ordinary progress report, built from a read taken before that withdrawal.
	follower := &Publisher{Client: &staleFirstRead{Client: fakeClient, stale: stale}}
	require.NoError(t, follower.Publish(context.Background(), State{
		Node: "master-1", Role: registryv1alpha1.ReplicaRoleFollower, VerifiedDigests: 12,
	}))

	assert.False(t, getStorage(t, fakeClient).Status.SafeToDropUpstream,
		"a withdrawal is not something another replica's report may undo")
	assert.False(t, getStorage(t, fakeClient).Status.AllReplicasFull)
}
