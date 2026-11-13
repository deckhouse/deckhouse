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

package operation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

func repairOf(name string, nodes ...string) *registryv1alpha1.RegistryOperation {
	request := newRequest(name, accepted)
	request.Spec.Type = registryv1alpha1.OperationRepair
	request.Spec.Repair = &registryv1alpha1.RepairSpec{Digests: []string{"sha256:aa"}, Nodes: nodes}
	return request
}

func TestARepairIsHandedOnlyToTheReplicasItNames(t *testing.T) {
	r, c := newReconciler(t, accepted, storageWith("master-0", "master-1", "master-2"), repairOf("repair-1", "master-2", "master-0"))
	reconcileRequest(t, r, "repair-1")

	got := getRequest(t, c, "repair-1").Status
	assert.Equal(t, registryv1alpha1.OperationPending, got.Phase)
	require.Len(t, got.Replicas, 2)
	assert.Equal(t, "master-0", got.Replicas[0].Node)
	assert.Equal(t, "master-2", got.Replicas[1].Node)
}

func TestAnOperationNamingANodeWithoutAReplicaIsRefused(t *testing.T) {
	r, c := newReconciler(t, accepted, storageWith("master-0"), repairOf("repair-1", "master-0", "worker-7"))
	reconcileRequest(t, r, "repair-1")

	got := getRequest(t, c, "repair-1").Status
	assert.Equal(t, registryv1alpha1.OperationFailed, got.Phase)
	assert.Equal(t, registryv1alpha1.ReasonOperationUnknownNode, got.Reason)
	assert.Contains(t, got.Message, "worker-7")
	assert.Empty(t, got.Replicas, "what was asked for is not what would run, so nothing runs")
}

func TestAFinishedVerifyAndRepairSayWhatTheyFound(t *testing.T) {
	now := metav1.NewTime(accepted)

	verify := acceptedRequest("verify-1", "master-0", "master-1")
	verify.Spec.Type = registryv1alpha1.OperationVerify
	for i := range verify.Status.Replicas {
		verify.Status.Replicas[i].Phase = registryv1alpha1.ReplicaOperationSucceeded
		verify.Status.Replicas[i].FinishedAt = &now
	}
	verify.Status.Replicas[1].Verify = &registryv1alpha1.VerifyResult{CorruptedCount: 2}
	summarize(verify, now)
	assert.Equal(t, registryv1alpha1.OperationSucceeded, verify.Status.Phase)
	assert.Contains(t, verify.Status.Message, "2 broken blobs found on master-1")

	repair := acceptedRequest("repair-1", "master-0")
	repair.Spec.Type = registryv1alpha1.OperationRepair
	repair.Status.Replicas[0].Phase = registryv1alpha1.ReplicaOperationSucceeded
	repair.Status.Replicas[0].FinishedAt = &now
	repair.Status.Replicas[0].Repair = &registryv1alpha1.RepairResult{Digests: []registryv1alpha1.DigestRepair{
		{Digest: "sha256:aa", Outcome: registryv1alpha1.DigestRepaired},
		{Digest: "sha256:bb", Outcome: registryv1alpha1.DigestIntact},
	}}
	summarize(repair, now)
	assert.Contains(t, repair.Status.Message, "1 repaired, 1 intact, 0 not held, 0 failed")
}

// A Verify that checked nothing found nothing to report, and must not say the store is clean.
func TestAFailedVerifyDoesNotSayTheStoreIsClean(t *testing.T) {
	now := metav1.NewTime(accepted)

	verify := acceptedRequest("verify-1", "master-0", "master-1")
	verify.Spec.Type = registryv1alpha1.OperationVerify
	for i := range verify.Status.Replicas {
		verify.Status.Replicas[i].Phase = registryv1alpha1.ReplicaOperationFailed
		verify.Status.Replicas[i].Reason = registryv1alpha1.ReasonOperationDeadlineExceeded
		verify.Status.Replicas[i].FinishedAt = &now
	}
	summarize(verify, now)
	assert.Equal(t, registryv1alpha1.OperationFailed, verify.Status.Phase)
	assert.NotContains(t, verify.Status.Message, "No broken blobs")

	partly := acceptedRequest("verify-2", "master-0", "master-1")
	partly.Spec.Type = registryv1alpha1.OperationVerify
	partly.Status.Replicas[0].Phase = registryv1alpha1.ReplicaOperationSucceeded
	partly.Status.Replicas[0].FinishedAt = &now
	partly.Status.Replicas[1].Phase = registryv1alpha1.ReplicaOperationFailed
	partly.Status.Replicas[1].Reason = registryv1alpha1.ReasonOperationDeadlineExceeded
	partly.Status.Replicas[1].FinishedAt = &now
	summarize(partly, now)
	assert.Contains(t, partly.Status.Message, "No broken blobs found on the replicas that finished.")

	repair := acceptedRequest("repair-1", "master-0")
	repair.Spec.Type = registryv1alpha1.OperationRepair
	repair.Status.Replicas[0].Phase = registryv1alpha1.ReplicaOperationFailed
	repair.Status.Replicas[0].FinishedAt = &now
	summarize(repair, now)
	assert.NotContains(t, repair.Status.Message, "repaired")
}
