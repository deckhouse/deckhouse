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

// Package run drives one storage replica: it keeps the serving process configured,
// does the work its role calls for, and reports what it holds.
package run

import (
	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

// Action is the work a replica does on one pass, beyond keeping the registry
// configuration in step, which happens unconditionally.
type Action string

const (
	// ActionNone means there is nothing to do but report.
	ActionNone Action = "None"

	// ActionFill means copy from the upstream into the local storage. Only the
	// leader does this: it is the replica the upstream credentials are spent on, and
	// the one whose completeness gates the air-gap transition.
	ActionFill Action = "Fill"

	// ActionCountCatalogue means account for what the storage holds by counting the
	// manifests on its own disk.
	//
	// Used where copying cannot account for the content: an air-gapped storage is
	// filled out of band by `d8 mirror push`, which the syncer never sees.
	//
	// Named after the registry catalogue for continuity — it is a metric label — but it
	// deliberately no longer reads one. Asking a registry what it holds is unsound while
	// pull-through is configured, because the answer is what it can FETCH: a follower
	// once reported 403545 digests, the tag count of the upstream it proxies. Counting
	// the store's own filesystem is sound in every arrangement, which is why this action
	// is no longer confined to the ones where pull-through happens to be off.
	ActionCountCatalogue Action = "CountCatalogue"

	// ActionReplicate means copy from the leader into this replica.
	//
	// Done ahead of time rather than on a cache miss, which is what makes losing the
	// leader cheap: whichever replica takes over already holds the set, instead of
	// refilling it from the upstream — or, in an air-gapped cluster, having no way to
	// refill it at all.
	//
	// Pulled by the follower rather than pushed by the leader. The effect is the
	// same, and this way each replica keeps owning its own store and its own report,
	// which is what lets several syncers write one status without a coordinator.
	ActionReplicate Action = "Replicate"
)

// Leader is what a follower needs to know to replicate from the leader.
type Leader struct {
	// Node the leader runs on.
	Node string

	// Address the leader answers at, as reported by the leader itself.
	Address string

	// Full reports whether the leader holds the expected set.
	//
	// No longer a condition for replicating from it. The reasoning it used to carry — that copying
	// a partial set would have to be redone — does not hold: the copier skips what is already
	// present, so a later pass copies only the difference. What the condition did cost is what the
	// design promises: followers fill AHEAD of time, and while the leader was still filling they
	// did nothing at all, holding nothing if it died meanwhile. Measured on a cluster: three
	// replicas with 428, 337 and 333 digests, none of them full, none of them moving.
	Full bool
}

// Usable reports whether this leader can be replicated from.
//
// Completeness is deliberately not required: whatever the leader already holds is worth having, and
// what it does not hold yet is reported as pending rather than as a failure. What IS required is an
// address to reach and a name to record, and — via the caller — that the leader is not reporting a
// failure of its own.
func (l *Leader) Usable() bool {
	return l != nil && l.Address != "" && l.Node != ""
}

// Decide works out what this replica should do.
//
// A pure function of the spec and the role, so the one decision that can cut a
// cluster off from images — reporting a cache complete — is testable without a
// registry, a lease or a cluster.
func Decide(spec *registryv1alpha1.RegistryStorageSpec, isLeader bool, leader *Leader) Action {
	if spec == nil {
		return ActionNone
	}

	if !isLeader {
		if leader.Usable() {
			// Fill from the leader now, not on a cache miss. This is what makes a leader
			// change cheap, and in an air-gapped cluster it is the only way a follower
			// gets the content at all.
			return ActionReplicate
		}
		// No leader worth copying from yet, so report what is already here.
		return ActionCountCatalogue
	}

	if spec.Upstream == nil {
		// Air-gap: there is nowhere to copy from, and the content arrives through the
		// write endpoint. Reading the catalogue is the only honest accounting left.
		return ActionCountCatalogue
	}

	if !spec.NeedSync {
		// The controller has not asked for a fill, so nothing is claimed about
		// completeness. Counting instead would be sound — the count reads this store's own
		// disk — but it would answer a question nobody asked: with an upstream configured
		// the store legitimately holds whatever it fetched on a cache miss, and that says
		// nothing about whether the EXPECTED set is present. The previous report stands.
		return ActionNone
	}

	return ActionFill
}

// StoreIsAuthority reports whether completeness must be judged by reading the
// store rather than by counting what a copy wrote.
//
// True for the leader inside the transition window of the air-gap story: air-gap has been asked for,
// the upstream is still HELD so the cluster keeps working, and images arrive through `d8 mirror push` —
// a write the syncer never sees and therefore cannot count. Without this the push contributes nothing,
// the leader never reads as complete, and the upstream is held forever; measured on a cluster before it
// was fixed.
//
// Reading the catalogue is honest ONLY there, and this is the condition that has to be got right: a
// pass-through cache writes into its own store on every miss, so a catalogue read on a caching cluster
// counts what the cluster happened to fetch rather than what it deliberately holds — completeness would
// arrive on its own, and the air-gap gate is derived from completeness.
//
// It used to key off `spec.Publish`, which was sound while publication existed exactly in air-gap and
// turned pull-through off. The write endpoint is now a separate instance published on every managed
// cluster, so `Publish` no longer implies any of that, and the condition moved to the field that still
// means it. Keying off the held upstream instead would be wrong in the other direction: inside the
// window the upstream is still held, which is the point of the window.
//
// A follower is excluded because its store is a copy, and what a copy holds authorizes nothing.
func StoreIsAuthority(spec *registryv1alpha1.RegistryStorageSpec, isLeader bool) bool {
	return spec != nil && isLeader && spec.AirGapRequested
}

// ExpectedDigests is how many digests the storage is supposed to hold, or zero
// when nothing said. Zero never counts as complete.
func ExpectedDigests(spec *registryv1alpha1.RegistryStorageSpec) int32 {
	if spec == nil || spec.Source == nil {
		return 0
	}
	return spec.Source.ExpectedDigests
}

// Role names the role a replica reports for itself.
func Role(isLeader bool) registryv1alpha1.ReplicaRole {
	if isLeader {
		return registryv1alpha1.ReplicaRoleLeader
	}
	return registryv1alpha1.ReplicaRoleFollower
}
