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

	// ActionCountCatalogue means account for what the storage holds by reading its
	// catalogue.
	//
	// Used where copying cannot account for the content: an air-gapped storage is
	// filled out of band by `d8 mirror push`, which the syncer never sees.
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

	// Full reports whether the leader holds the expected set. Replicating from an
	// incomplete leader is pointless work: it would copy a partial set and have to be
	// redone.
	Full bool
}

// Usable reports whether this leader can be replicated from.
func (l *Leader) Usable() bool {
	return l != nil && l.Full && l.Address != "" && l.Node != ""
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
		// The controller has not asked for a fill. Counting the catalogue instead
		// would be wrong here: with an upstream configured the storage may hold
		// images it fetched on a cache miss, which says nothing about whether it
		// holds the whole expected set.
		return ActionNone
	}

	return ActionFill
}

// StoreIsAuthority reports whether completeness must be judged by reading the
// store rather than by counting what a copy wrote.
//
// True for the leader while the write endpoint is open, which is the transition
// window of История 3: air-gap has been asked for, the upstream is still HELD so
// the cluster keeps working, and the images arrive through `d8 mirror push` — a
// write the syncer never sees and therefore cannot count. Without this the push
// contributes nothing, the leader never reads as complete, and the upstream is
// held forever; measured on a cluster before it was fixed.
//
// Reading the catalogue is honest here, and for a reason the design already
// provides rather than an assumption: publishing turns pull-through off, so
// nothing but a deliberate write can put anything in the store. That is also why
// this is the leader's business alone — a follower's store is a copy, and what it
// holds authorizes nothing.
func StoreIsAuthority(spec *registryv1alpha1.RegistryStorageSpec, isLeader bool) bool {
	return spec != nil && isLeader && spec.Publish
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
