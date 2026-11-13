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

package gc

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

func noCollection(t *testing.T) func(context.Context) (Report, error) {
	return func(context.Context) (Report, error) {
		t.Error("a check or a repair must not collect")
		return Report{}, nil
	}
}

// TestAVerifyRunsBesideAFillAndReportsWhatItFound: it only reads, and a replica with a broken blob is
// not complete — its fill is owed until the repair runs, so neither may wait for it.
func TestAVerifyRunsBesideAFillAndReportsWhatItFound(t *testing.T) {
	operations := newFakeOperations()
	operations.kind = registryv1alpha1.OperationVerify
	operations.digests = []string{"sha256:aa"}

	worker := newWorker(t, operations, &directLease{}, noCollection(t))
	worker.Plan = func() Plan { return Plan{Known: true, FillPending: true} }
	worker.Filling = func() bool { return true }
	worker.Pushing = func() bool { return true }
	worker.Pause = func() (func(), bool) {
		t.Error("a check does not pause the store's writers")
		return func() {}, true
	}
	var asked []string
	var done atomic.Int32
	worker.Verify = func(_ context.Context, digests []string) (registryv1alpha1.VerifyResult, error) {
		asked = digests
		return registryv1alpha1.VerifyResult{CheckedBlobs: 3, CheckedBytes: 3 << 20, CorruptedBlobs: []string{"sha256:bb"}, CorruptedCount: 1}, nil
	}
	worker.Done = func(*Operation) { done.Add(1) }

	runUntil(t, worker, operations.settled)

	assert.Equal(t, []string{"sha256:aa"}, asked)
	assert.Equal(t, []string{"Waiting/Turn", "Running/", "Succeeded/"}, operations.phases())
	final := operations.last()
	require.NotNil(t, final.Verify)
	assert.Equal(t, []string{"sha256:bb"}, final.Verify.CorruptedBlobs)
	assert.Nil(t, final.Report)
	assert.Contains(t, final.Message, "1 do not match their digests")
	assert.EqualValues(t, 1, done.Load(), "the index is rebuilt after an operation")
}

func TestARepairThatCouldNotRepairADigestFailsTheReplica(t *testing.T) {
	operations := newFakeOperations()
	operations.kind = registryv1alpha1.OperationRepair
	operations.digests = []string{"sha256:aa", "sha256:bb"}

	worker := newWorker(t, operations, &directLease{}, noCollection(t))
	worker.Plan = func() Plan { return Plan{Known: true, FillPending: true} }
	worker.Repair = func(context.Context, []string) (registryv1alpha1.RepairResult, error) {
		return registryv1alpha1.RepairResult{Digests: []registryv1alpha1.DigestRepair{
			{Digest: "sha256:aa", Outcome: registryv1alpha1.DigestRepaired, RepairedBlobs: 1},
			{Digest: "sha256:bb", Outcome: registryv1alpha1.DigestFailed, Message: "no whole copy"},
		}}, nil
	}

	runUntil(t, worker, operations.settled)

	final := operations.last()
	assert.Equal(t, registryv1alpha1.ReplicaOperationFailed, final.Phase)
	assert.Equal(t, registryv1alpha1.ReasonOperationFailed, final.Reason)
	assert.Contains(t, final.Message, "1 of 2 digests could not be repaired")
	require.NotNil(t, final.Repair, "what was repaired is reported even when something was not")
	assert.Len(t, final.Repair.Digests, 2)
}

func TestAReplicaWithoutARepairerRefusesTheRepair(t *testing.T) {
	operations := newFakeOperations()
	operations.kind = registryv1alpha1.OperationRepair

	worker := newWorker(t, operations, &directLease{}, noCollection(t))
	runUntil(t, worker, operations.settled)

	assert.Equal(t, registryv1alpha1.ReplicaOperationFailed, operations.last().Phase)
	assert.Contains(t, operations.last().Message, "does not run repairs")
}

func TestAVerifyAndARepairAreOwedAndTheirResultsWritten(t *testing.T) {
	item := &registryv1alpha1.RegistryOperation{
		Spec: registryv1alpha1.RegistryOperationSpec{
			Type:   registryv1alpha1.OperationRepair,
			Repair: &registryv1alpha1.RepairSpec{Digests: []string{"sha256:aa"}, Nodes: []string{"master-0"}},
		},
		Status: registryv1alpha1.RegistryOperationStatus{
			Phase:    registryv1alpha1.OperationPending,
			Replicas: []registryv1alpha1.ReplicaOperationStatus{{Node: "master-0", Phase: registryv1alpha1.ReplicaOperationPending}},
		},
	}
	operations := &ClusterOperations{Node: "master-0"}
	assert.True(t, operations.owes(item))

	operation := toOperation(item, "master-0")
	assert.Equal(t, registryv1alpha1.OperationRepair, operation.Type)
	assert.Equal(t, []string{"sha256:aa"}, operation.Digests)

	entry := item.Status.Replica("master-0")
	apply(entry, Progress{
		Phase:  registryv1alpha1.ReplicaOperationSucceeded,
		Repair: &registryv1alpha1.RepairResult{Digests: []registryv1alpha1.DigestRepair{{Digest: "sha256:aa", Outcome: registryv1alpha1.DigestIntact}}},
	})
	require.NotNil(t, entry.Repair)
	assert.Nil(t, entry.Verify)
	assert.Nil(t, entry.GarbageCollection)

	item.Spec.Type = "Unknown"
	assert.False(t, operations.owes(item), "a type this syncer does not know is not picked up")
}
