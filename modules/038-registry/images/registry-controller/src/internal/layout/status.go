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
//
// Which replica leads is decided by `leaseHolder` — the identity in the election lease — and
// not by the Role a replica wrote about itself. A replica owns its entry and cannot update it
// once its pod is gone, so the entry keeps whatever it last claimed: measured on a live
// cluster, the lease moved to another node in eight seconds while the departed replica's
// entry went on saying Leader for as long as the pod stayed away. Two entries then claimed
// leadership, and reading them by array order made `status.Leader` name the replica that no
// longer existed while `LeaderFull` answered from the other one — two fields describing two
// different replicas, one of which was gone. Since LeaderFull is what authorizes dropping the
// upstream, that is the one decision in this module that must never be made from a guess.
func Aggregate(
	spec *registryv1alpha1.RegistryStorageSpec,
	replicas []registryv1alpha1.StorageReplicaStatus,
	leaseHolder string,
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

	leader := leaseHolderReport(replicas, leaseHolder)
	allFull := true
	anyFailed := false

	for i := range replicas {
		replica := &replicas[i]

		if !replica.Full {
			allFull = false
		}
		if replica.Error != "" {
			anyFailed = true
		}
	}

	status.AllReplicasFull = allFull

	if leader == nil {
		// Either no replica holds the lease, or the one that does has not reported
		// yet. Both happen during a leader change and must not read as progress:
		// with no leader there is nothing whose completeness could authorize going
		// air-gap.
		status.Phase = registryv1alpha1.StoragePhaseIdle
		return status
	}

	status.Leader = leader.Node
	// Reported through the very same predicate the transition is gated on, so the
	// field cannot claim "safe" while the controller refuses to act on it.
	status.SafeToDropUpstream = LeaderFull(replicas, leaseHolder)

	// Progress out of whatever denominator is known, and there are two.
	//
	// `expectedDigests` is the operator's air-gap declaration and takes precedence: while a transition
	// is being gated on a stated number, the status has to show progress against THAT number, not
	// against one the cluster computed about itself. Everywhere else — an ordinary cluster with an
	// upstream, which is most of them — the leader's own count of the set is the only denominator
	// there is, and using it is the difference between a progress report and `0/0` beside gigabytes
	// in flight.
	if total := fillDenominator(spec, leader); total > 0 {
		status.Fill = &registryv1alpha1.FillProgress{
			Filled: leader.VerifiedDigests,
			Total:  total,
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

// fillDenominator picks what the fill is reported out of: the declaration if there is one, the
// leader's count of the set otherwise.
func fillDenominator(spec *registryv1alpha1.RegistryStorageSpec, leader *registryv1alpha1.StorageReplicaStatus) int32 {
	if expected := expectedDigests(spec); expected > 0 {
		return expected
	}
	return leader.DeclaredDigests
}

// LeaderFull reports whether the leader holds the expected set, which is what
// gates the air-gap transition. It reads the reports directly rather than the
// aggregate so that a stale or hand-edited aggregate cannot authorize the
// transition.
//
// Which report to read is decided by the lease and not by the Role in the report, for the
// reason given on Aggregate: a departed replica's entry keeps claiming what it last claimed,
// and this predicate is what takes the upstream away.
func LeaderFull(replicas []registryv1alpha1.StorageReplicaStatus, leaseHolder string) bool {
	leader := leaseHolderReport(replicas, leaseHolder)
	return leader != nil && leader.Full && leader.Error == ""
}

// LeaderAddress is where the leading replica answers, or "" when there is no usable leader.
//
// Agents are pointed at the leader first, and the rest of the replicas become its mirrors. Which
// replica is first is not a detail: an agent that gets a 404 from its primary does not ask the next
// one — deliberately, because for the Storage-then-Upstream chain a 404 means the image is genuinely
// absent and retrying it everywhere would turn every missing tag into a request to every backend.
// Mirrors sit in that same list, so the same rule applies to them, and a follower that is one image
// behind answers 404 for an image the cluster does hold.
//
// Measured on `ly-mmc` in air-gap: the agent on master-0 was pointed at master-2, which held 402 of
// 403 images; the missing one was on disk on two other replicas, `crictl pull` returned NotFound, and
// eighteen pods could not start. The leader is the replica whose completeness authorized the air-gap
// in the first place, so it is the one that has the set.
func LeaderAddress(replicas []registryv1alpha1.StorageReplicaStatus, leaseHolder string) string {
	leader := leaseHolderReport(replicas, leaseHolder)
	if leader == nil {
		return ""
	}
	return leader.Address
}

// leaseHolderReport is the report of the replica that holds the election lease, or nil when
// there is no holder or the holder has not reported.
//
// Nil rather than a fallback to whatever else looks like a leader: without the holder's own
// report there is nothing to say about what the leader holds, and every caller here treats
// "nothing to say" as "not safe".
func leaseHolderReport(
	replicas []registryv1alpha1.StorageReplicaStatus, leaseHolder string,
) *registryv1alpha1.StorageReplicaStatus {
	if leaseHolder == "" {
		return nil
	}
	for i := range replicas {
		if replicas[i].Node == leaseHolder {
			return &replicas[i]
		}
	}
	return nil
}
