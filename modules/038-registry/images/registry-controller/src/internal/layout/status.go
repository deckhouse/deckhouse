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

package layout

import (
	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

// Aggregate derives the cluster-wide view of the storage from the per-replica
// reports its syncers write.
//
// Each syncer owns its own entry in status.replicas and reports what it actually
// WROTE. That is the whole reason completeness is not probed through the serve
// path: on a miss a pass-through cache fetches the image from the upstream and
// answers 200, so a probe would report a full cache on an empty store and the
// upstream would be dropped from under the cluster.
//
// The aggregate fields are derived here rather than written by any syncer,
// because no single replica can know whether the others are done.
func Aggregate(
	spec *registryv1alpha1.RegistryStorageSpec, replicas []registryv1alpha1.StorageReplicaStatus,
) registryv1alpha1.RegistryStorageStatus {
	status := registryv1alpha1.RegistryStorageStatus{
		// The cache is the only source of images exactly when no upstream is left.
		Authoritative: spec.Upstream == nil,
		Replicas:      replicas,
	}

	if len(replicas) == 0 {
		// No replica has reported yet. Saying "Idle" is honest; saying "Ready"
		// would be catastrophic, since Ready gates dropping the upstream.
		status.Phase = registryv1alpha1.StoragePhaseIdle
		return status
	}

	var leader *registryv1alpha1.StorageReplicaStatus
	allFull := true
	anyFailed := false

	for i := range replicas {
		replica := &replicas[i]

		if replica.Role == registryv1alpha1.ReplicaRoleLeader {
			leader = replica
		}
		if !replica.Full {
			allFull = false
		}
		if replica.Error != "" {
			anyFailed = true
		}
	}

	status.AllReplicasFull = allFull

	if leader == nil {
		// Replicas exist but none holds the lease. This happens during a leader
		// change and must not read as progress: with no leader there is nothing
		// whose completeness could authorize going air-gap.
		status.Phase = registryv1alpha1.StoragePhaseIdle
		return status
	}

	status.Leader = leader.Node
	// Reported through the very same predicate the transition is gated on, so the
	// field cannot claim "safe" while the controller refuses to act on it.
	status.SafeToDropUpstream = LeaderFull(replicas)

	if expected := expectedDigests(spec); expected > 0 {
		status.Fill = &registryv1alpha1.FillProgress{
			Filled: leader.VerifiedDigests,
			Total:  expected,
		}
	}

	switch {
	case leader.Error != "":
		// A failing leader must never look like progress: the upstream stays.
		status.Phase = registryv1alpha1.StoragePhaseFailed
	case leader.Full:
		// Ready on the leader alone, followers still catching up or not. The
		// cluster can already go air-gap safely at this point, and AllReplicasFull
		// reports the remaining redundancy separately.
		status.Phase = registryv1alpha1.StoragePhaseReady
	case anyFailed:
		status.Phase = registryv1alpha1.StoragePhaseFailed
	default:
		status.Phase = registryv1alpha1.StoragePhaseFilling
	}

	return status
}

func expectedDigests(spec *registryv1alpha1.RegistryStorageSpec) int32 {
	if spec.Source == nil {
		return 0
	}
	return spec.Source.ExpectedDigests
}

// LeaderFull reports whether the leader holds the expected set, which is what
// gates the air-gap transition. It reads the reports directly rather than the
// aggregate so that a stale or hand-edited aggregate cannot authorize the
// transition.
func LeaderFull(replicas []registryv1alpha1.StorageReplicaStatus) bool {
	for i := range replicas {
		replica := &replicas[i]
		if replica.Role == registryv1alpha1.ReplicaRoleLeader {
			return replica.Full && replica.Error == ""
		}
	}
	return false
}
