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
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"

	"github.com/deckhouse/registry-syncer/internal/fill"
	"github.com/deckhouse/registry-syncer/internal/gc"
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
	// pull-through is configured, because the answer is what it can FETCH — a follower
	// answers with the tag count of the upstream it proxies. Counting
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
	// did nothing at all, holding nothing if it died meanwhile.
	Full bool

	// AheadSet is the parts of what is ahead of the set the leader holds — see AheadKey. A follower
	// copies those of them it lacks, and only those: a part its leader is short of would only be missed.
	AheadSet string
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
		if !leader.Usable() {
			// No leader worth copying from yet, so report what is already here.
			return ActionCountCatalogue
		}
		if !spec.NeedSync {
			// The controller raises it while any replica is short of what the leader holds, and
			// drops it once every one holds it. A follower that replicated regardless found the
			// whole set already present on every pass, at a cost — see Compute in the controller.
			return ActionNone
		}
		// Fill from the leader now, not on a cache miss. This is what makes a leader
		// change cheap, and in an air-gapped cluster it is the only way a follower
		// gets the content at all.
		return ActionReplicate
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
// True for the leader inside the air-gap transition window: air-gap has been asked for, the upstream
// is still HELD so the cluster keeps working, and images arrive through `d8 mirror push` — a write the
// syncer never sees and cannot count. Without this the push contributes nothing, the leader never
// reads as complete, and the upstream is held forever.
//
// Reading the catalogue is honest ONLY there, which is why the condition has to be exact: a
// pass-through cache writes into its own store on every miss, so a catalogue read on a caching
// cluster counts what it happened to fetch rather than what it holds on purpose, and completeness
// would arrive on its own. The field replaced `spec.Publish`, which stopped implying the window once
// the write endpoint was published on every managed cluster; the held upstream would be wrong in the
// other direction, since inside the window it is held by design.
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

// SetKey is a fingerprint of the set the cluster needs: the platform's versions — deployed and
// previous — every module it keeps with its version and where it comes from, and the module catalogue
// its sources offer.
//
// Only what the set is made of, never what it resolves to. Reading the set out of the releases means
// pulling images; this is a few lists from the API server, cheap enough for every pass, and it moves
// exactly when the set can: a module installed, removed, updated or moved to another source, the
// platform updated or rolled back, a module pushed into the catalogue. Every move takes the leader's
// full report away and so raises needSync until every replica holds the new set.
//
// What is pending is not in it — see AheadKey. A replica found full for this key holds the set and
// nothing more is claimed: counting the pending versions in made a follower that had copied the set,
// and not yet the release copied ahead of it, report itself as holding both.
//
// The catalogue is counted since this fingerprint was first released, and every cluster has one,
// through the source `deckhouse`. So after that upgrade each replica reads a fingerprint other than
// the one it reported: its Full is withdrawn once and verified again, which copies only what is
// missing — and while the StatefulSet rolls, a leader still on the old version may fill on every pass
// until it is replaced. Once, and harmless; not a fingerprint that stays the same.
func SetKey(releases gc.Releases, modules []fill.ModuleRef, catalogue []string) string {
	lines := make([]string, 0, len(modules))
	for _, module := range modules {
		if module.Pending {
			continue
		}
		lines = append(lines, module.Name+"@"+module.Version+"<"+strings.TrimRight(module.Source.Repository, "/"))
	}
	sort.Strings(lines)

	text := "platform " + releases.Deployed + " " + releases.Previous + "\n" + strings.Join(lines, "\n")
	if len(catalogue) > 0 {
		offered := append([]string(nil), catalogue...)
		sort.Strings(offered)
		text += "\ncatalogue " + strings.Join(offered, " ")
	}

	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:16])
}

// AheadKey is what is ahead of the set, by its parts: the key of the platform version the cluster
// updates to next and of each module update it is about to apply — see fill.AheadPart — sorted and
// joined by commas. Empty when nothing is pending.
//
// Its own field beside VerifiedSet, because it is held on its own terms. The set is copied by a pass,
// and a pass that ends full holds it; what is ahead of it is copied afterwards, in the background,
// best-effort, and may be stopped, cut short by a restart, or miss what the source does not have yet. A
// replica reports each part once a copy left it whole — see Loop.heldAhead — and a follower copies the
// parts its leader holds, so a release pushed onto an air-gapped leader reaches every follower even
// while a module beside it never arrives.
func AheadKey(releases gc.Releases, modules []fill.ModuleRef) string {
	var parts []string
	if next := fill.NextPending(releases.Pending); next != "" {
		parts = append(parts, fill.PlatformPart(next))
	}
	for _, module := range modules {
		if module.Pending {
			parts = append(parts, fill.ModulePart(module))
		}
	}
	return joinParts(parts)
}

// joinParts is part keys as AheadKey and AheadSet carry them: sorted, without repeats, comma-joined.
func joinParts(parts []string) string {
	unique := map[string]struct{}{}
	for _, part := range parts {
		if part != "" {
			unique[part] = struct{}{}
		}
	}
	sorted := make([]string, 0, len(unique))
	for part := range unique {
		sorted = append(sorted, part)
	}
	sort.Strings(sorted)
	return strings.Join(sorted, ",")
}

// splitParts is joinParts undone, as a set.
func splitParts(joined string) map[string]bool {
	parts := map[string]bool{}
	for _, part := range strings.Split(joined, ",") {
		if part != "" {
			parts[part] = true
		}
	}
	return parts
}

// Reverify turns a leader's "nothing to do" into a verification when the set has moved since it was
// found full.
//
// The controller asks for a fill only while the leader is not full, and a full leader is not asked
// again — so, left alone, Full outlived the set it was measured against: a module installed, or the
// platform updated, and the store kept reporting itself complete for a set it had never seen, which is
// the answer that authorizes cutting an air-gapped cluster off from its upstream. So when the leader
// would do nothing and its fingerprint is not the current one, it withdraws Full and fills — which
// copies only what is missing.
//
// A fingerprint that could not be computed changes nothing: the pass cannot tell whether the set
// moved, and withdrawing on every such pass would make Full flap with the API server. After a
// restart the reported fingerprint is read back from the status, so an unchanged set costs nothing.
func Reverify(action Action, isLeader bool, spec *registryv1alpha1.RegistryStorageSpec, reported, current string) (Action, bool) {
	if action != ActionNone || !isLeader || spec == nil || spec.Upstream == nil || current == "" {
		return action, false
	}
	if reported == current {
		return action, false
	}
	return ActionFill, true
}

// AlreadyHeld reports that a copy the pass is about to make would only confirm what this replica
// last reported: it was found full against this very set — and, for a follower, while replicating
// from the replica that leads now.
//
// needSync is one switch for every replica and stays up until the last of them holds the set, so a
// replica that already does would otherwise enumerate the set again on every pass while it waits.
//
// The leader is part of the answer because a follower holds what its leader held, not only what the
// set names: a push lands on the leader alone, and after the lease moves the new leader may hold what
// the old one never had. A follower that last replicated from another replica copies once from the
// one that leads now — the controller raises needSync for exactly that, see FollowersBehind — and
// skipping it here would hold needSync up for good.
func AlreadyHeld(action Action, setKey, reportedSet, reportedSource string, leader *Leader) bool {
	if setKey == "" || reportedSet != setKey {
		return false
	}
	switch action {
	case ActionFill:
		return true
	case ActionReplicate:
		return leader != nil && reportedSource == leader.Node
	default:
		return false
	}
}
