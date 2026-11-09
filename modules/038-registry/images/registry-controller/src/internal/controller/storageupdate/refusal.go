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

package storageupdate

import (
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"

	"github.com/deckhouse/registry-controller/internal/layout"
	"github.com/deckhouse/registry-controller/internal/metrics"
)

// BlockReason is why the update refuses to replace the next replica, as the UpdateBlocked event and
// the d8_registry_storage_update_blocked gauge both name it.
//
// One list for both, because they are read by different people for the same answer: the event by
// whoever looks at the StatefulSet, the gauge by the alert that sends them there. All but the last
// are an air-gapped cache — with an upstream something always serves — and they differ in what ends
// them, which is what an operator needs to know first.
type BlockReason string

const (
	// BlockedSingleReplicaWithoutHolder: the only replica, and no image holder on its node at all.
	// For a moment that is normal — on the update that brings the holder, the StatefulSet can be
	// looked at before the DaemonSet has created its pods — and it resolves once the pod is there.
	// Lasting, it means the DaemonSet does not schedule a pod on that node.
	BlockedSingleReplicaWithoutHolder BlockReason = "SingleReplicaWithoutHolder"

	// BlockedHolderNotReady: the only replica, and the holder on its node does not run every new
	// image yet. Resolves on its own as the holder rolls and pulls — unless the cache does not have
	// an image, which `d8 mirror push` is the answer to, and the one case where it is; or the holder
	// has an image and cannot run it, which its pod's events explain.
	BlockedHolderNotReady BlockReason = "HolderNotReady"

	// BlockedLeaderWithoutFullFollower: the leader is the last replica left, and no follower holds the
	// set the leader holds to take the lease over. Resolves on its own once one has copied it — unless
	// the leader reports that it does not hold the set either, and then only an operator bringing it
	// in ends it.
	BlockedLeaderWithoutFullFollower BlockReason = "LeaderWithoutFullFollower"

	// BlockedLeaderNotServing: the lease names a node where no serving replica runs. Resolves on its
	// own once the lease expires and moves.
	BlockedLeaderNotServing BlockReason = "LeaderNotServing"

	// BlockedStorageUnknown: RegistryStorage does not exist, so nothing is known of an upstream or of
	// the replicas. This controller creates it; a moment, normally.
	BlockedStorageUnknown BlockReason = "StorageUnknown"
)

// blockReasons is every reason, so that each is published — as 0 while it does not apply.
var blockReasons = []BlockReason{
	BlockedSingleReplicaWithoutHolder,
	BlockedHolderNotReady,
	BlockedLeaderWithoutFullFollower,
	BlockedLeaderNotServing,
	BlockedStorageUnknown,
}

// refusal is a replacement that has to wait: the reason, and what to tell an operator about it.
type refusal struct {
	reason BlockReason

	// message says which replica, why, and what ends the wait — and recommends doing something only
	// where doing it helps.
	message string

	// settling is set when what ends the wait is a replica finishing what it is already doing, and
	// nothing wakes this controller for it: a follower's copy shows only in RegistryStorage, which is
	// not watched. Looked at again at settleInterval rather than blockedInterval, so that the update
	// goes on soon after, as it does after a replaced pod becomes ready.
	settling bool

	// state is what decides the refusal, for telling a refusal repeated from a new one: see
	// Reconciler.announce. Set where the message carries something that changes while nothing else
	// does — a follower's digest count — and empty where the message itself serves.
	state string
}

// key is what a repeated refusal is told by.
func (r *refusal) key() string {
	if r.state != "" {
		return r.state
	}
	return r.message
}

// updateReport is what a pass found about the update, as the metrics carry it.
type updateReport struct {
	// outdated is how many replicas are not yet on the update revision.
	outdated int

	// blocked is why the next replacement was refused; empty when it was not.
	blocked BlockReason
}

func (r updateReport) publish() {
	metrics.StorageReplicasOutdated.Set(float64(r.outdated))
	for _, reason := range blockReasons {
		value := 0.0
		if reason == r.blocked {
			value = 1
		}
		metrics.StorageUpdateBlocked.WithLabelValues(string(reason)).Set(value)
	}
}

// outdatedReplicas is how many replicas are not yet on the target revision: those still on another
// one, and those missing — a replica deleted a moment ago has not been updated either, and counting
// only the pods that exist would read as progress every time one is taken down.
func outdatedReplicas(set *appsv1.StatefulSet, pods []*corev1.Pod, target string) int {
	updated := 0
	for _, pod := range pods {
		if pod.Labels[revisionLabel] == target {
			updated++
		}
	}
	return max(int(desiredReplicas(set))-updated, 0)
}

// holdsTheSet is whether a follower's own report says it holds the set the leader holds and its last
// pass finished — what it must say of itself to take the lease over. Nil leader, a leader that has
// not reported: nothing to compare with, and so no follower does.
//
// Nor while the leader's own report says its last pass failed, or found it not full, whatever set it
// still names: see leaderTrusted.
//
// Full alone is not enough, and was taken for it once: a follower replaced in this very update comes
// back reading the Full of the set before the update from the status, and copies the new set only
// later. The pods are watched, so the moment it is ready the leader would have gone, and the images
// of the new release with it — on a cluster with nowhere else to pull them from.
func holdsTheSet(report registryv1alpha1.StorageReplicaStatus, leader *registryv1alpha1.StorageReplicaStatus) bool {
	return leaderTrusted(leader) && layout.HoldsLeadersSet(&report, leader)
}

// leaderTrusted is whether the set the leader names is one it holds now: a set named, by a leader that
// is full and whose last pass had no error. The syncer clears the set on any pass that counts and
// fails or comes up short, so a set next to an error or a store not full is one carried over from
// an earlier pass — a pass that did nothing, or a status written before this leader took the lease —
// and says nothing of what the leader holds now. No such report has been seen from an air-gapped
// leader, whose every pass counts; this is defensive, and leaderRefusal treats such a leader as one
// that does not hold the set or whose pass failed.
func leaderTrusted(leader *registryv1alpha1.StorageReplicaStatus) bool {
	return leader != nil && leader.Full && leader.Error == "" && leader.VerifiedSet != ""
}

// leaderRefusal is why the leader, the last replica left on the old revision, cannot go yet: no
// follower holds the set the leader holds to take the lease over.
//
// What the message says turns on the leader's own report, because that is what tells a wait from a
// dead end:
//
//   - the leader holds the set: the update waits for a follower to copy it, and what the message
//     says of the wait is what the followers report, since a report is all this controller sees of
//     them — it cannot tell a follower about to copy from one that is stuck. Those copying are named
//     as copying; failing that, those still full against an earlier set, as a follower replaced in
//     this very update comes back, are said not to have reported copying yet, with where to look if
//     they do not start. A follower with no report is said to have none, and one whose last pass
//     failed is named with its log. These go together, a follower can be in any of them, and nothing
//     needs doing unless one of them is failing;
//   - the leader holds a set and does not say which — its last pass could not tell which set the
//     cluster needs, which it reads from the API server: no follower can be shown to hold the same
//     one. Normally the leader names it on its next pass. A leader that keeps failing to read it
//     keeps the update waiting, and its log says why, so the message ends with where to look;
//   - the leader has finished a pass without an error and does not hold it: the followers copy from
//     the leader, so they cannot get what it lacks. Without an upstream the only way those images
//     get into the cluster is an operator pushing them, and this is the one place where saying so
//     helps;
//   - the leader has not reported yet, or its last pass ended in an error: whether it holds the set
//     is not known, and so neither is which set a follower would have to hold. A push would answer
//     neither — a leader that has not looked yet, or could not finish looking, does not look sooner
//     for it — so the message says what is known and where to look, and nothing more.
//
// The two that end with a follower's or the leader's next pass are settling; the rest are looked at
// on a human timescale.
func leaderRefusal(
	going *corev1.Pod, pods []*corev1.Pod, reports map[string]registryv1alpha1.StorageReplicaStatus,
) *refusal {
	leader, reported := reports[going.Spec.NodeName]

	var followers []string
	var classes followerClasses
	for _, pod := range pods {
		if pod.Name == going.Name {
			continue
		}
		followers = append(followers, pod.Name+" on "+pod.Spec.NodeName+": "+replicaState(pod, reports, leader))
		classes.add(pod, reports, leader)
	}

	prefix := fmt.Sprintf("Not replacing %s, the leader, until a follower holds the set the leader holds "+
		"to take the lease over", going.Name)

	// What a repeated refusal is told by: the list of followers in the message carries their digest
	// counts, which change on every pass while one copies, and do not decide anything.
	state := func(leaderState string) string { return leaderState + "; " + classes.String() }

	switch {
	case !reported:
		return &refusal{
			reason: BlockedLeaderWithoutFullFollower,
			message: fmt.Sprintf("%s. The leader has not reported what it holds yet, so whether the followers "+
				"can copy the whole set from it is not known; the update continues once it has, and one of "+
				"them holds the same set. Followers: %s.", prefix, strings.Join(followers, "; ")),
			state: state("the leader has not reported"),
		}

	case leader.Error != "":
		return &refusal{
			reason: BlockedLeaderWithoutFullFollower,
			message: fmt.Sprintf("%s. The leader's last pass over the set ended in an error (%s), so whether it "+
				"holds the whole set is not known, and neither is which set a follower would have to hold; "+
				"the cause is in the log of container syncer of pod d8-system/%s, and the update continues "+
				"once a pass of the leader succeeds and a follower holds the same set. Followers: %s.",
				prefix, leader.Error, going.Name, strings.Join(followers, "; ")),
			state: state("the leader's last pass failed: " + leader.Error),
		}

	case !leader.Full:
		return &refusal{
			reason: BlockedLeaderWithoutFullFollower,
			message: fmt.Sprintf("%s: the leader does not hold it (%s), so "+
				"the followers, which copy from it, cannot complete it. Without an upstream only an operator "+
				"can bring the missing images in: push the Deckhouse Platform release and the modules this cluster "+
				"runs with d8 mirror push. The followers then copy them from the leader, and the update "+
				"continues once one of them holds the whole set. Followers: %s.",
				prefix, describeReport(leader, reported, nil), strings.Join(followers, "; ")),
			state: state("the leader does not hold the set"),
		}

	case leader.VerifiedSet == "":
		return &refusal{
			reason: BlockedLeaderWithoutFullFollower,
			message: fmt.Sprintf("%s. The leader holds a set, but its last pass could not tell which set the "+
				"cluster needs, so no follower can be shown to hold the same one. The update continues once a "+
				"pass of the leader names the set and a follower holds it; if the leader names none within a "+
				"few minutes, the log of container syncer of pod d8-system/%s says why it cannot tell. "+
				"Followers: %s.", prefix, going.Name, strings.Join(followers, "; ")),
			settling: true,
			state:    state("the leader names no set"),
		}
	}

	return &refusal{
		reason:   BlockedLeaderWithoutFullFollower,
		message:  fmt.Sprintf("%s. %s Followers: %s.", prefix, classes.wait(), strings.Join(followers, "; ")),
		settling: true,
		state:    state("the leader holds the set"),
	}
}

// followerClasses sorts the followers by what their reports say they are doing, to say what the
// leader is waiting on — the names in each, in the order the replicas were listed.
type followerClasses struct {
	// copying: serving, and its last report has it copying with no error.
	copying []string

	// behind: full, without an error, against another set than the leader's — what a follower
	// replaced in this update reads back from the status until its next pass starts on the new one.
	behind []string

	// failing: its last pass ended in an error.
	failing []string

	// unreported: no report from its node at all.
	unreported []string

	// other: not serving, or full against the set the leader names. A replica not serving is waited
	// out before, and one holding the set of a leader that holds it ends the wait, so only a follower
	// full against the set of a leader not trusted (see leaderTrusted) is ever here. It is said in the
	// list of followers and nowhere else.
	other []string
}

func (c *followerClasses) add(
	pod *corev1.Pod, reports map[string]registryv1alpha1.StorageReplicaStatus, leader registryv1alpha1.StorageReplicaStatus,
) {
	report, reported := reports[pod.Spec.NodeName]
	switch {
	case !podReady(pod):
		c.other = append(c.other, pod.Name)
	case !reported:
		c.unreported = append(c.unreported, pod.Name)
	case report.Error != "":
		c.failing = append(c.failing, pod.Name)
	case !report.Full:
		c.copying = append(c.copying, pod.Name)
	case !layout.HoldsLeadersSet(&report, &leader):
		c.behind = append(c.behind, pod.Name)
	default:
		c.other = append(c.other, pod.Name)
	}
}

func (c followerClasses) String() string {
	return fmt.Sprintf("copying %v, behind %v, failing %v, unreported %v, other %v",
		c.copying, c.behind, c.failing, c.unreported, c.other)
}

// wait says what the leader, holding the set, is waiting on: the followers that will end the wait
// themselves, then those that will not.
//
// A clause for each kind of follower there is, because a follower can be in any of them and another
// in another: "the followers are copying it" was said of followers that had not started — and of
// one reporting an error, followed by "Nothing needs doing", which an operator with a failing
// follower should not be told. The copying ones end the wait soonest, so the ones behind are said
// only when no follower is copying.
func (c followerClasses) wait() string {
	var says []string
	switch {
	case len(c.copying) > 0:
		says = append(says, fmt.Sprintf("%s %s copying it from the leader, and the update continues once %s "+
			"finished.", names(c.copying), choose(c.copying, "is", "are"), choose(c.copying, "it has", "one of them has")))
	case len(c.behind) > 0:
		says = append(says, fmt.Sprintf("%s %s full against another set than the leader's and %s not "+
			"reported copying the leader's yet. A follower starts copying it on its next pass, and the update "+
			"continues once one holds it; if none has started within a few minutes, the log of container "+
			"syncer of %s says why.",
			names(c.behind), choose(c.behind, "is", "are"), choose(c.behind, "has", "have"), podRefs(c.behind)))
	}
	if len(c.unreported) > 0 {
		says = append(says, fmt.Sprintf("%s %s not reported yet.", names(c.unreported), choose(c.unreported, "has", "have")))
	}
	for _, name := range c.failing {
		says = append(says, fmt.Sprintf("%s reports an error, and the cause is in the log of container "+
			"syncer of pod d8-system/%s.", name, name))
	}
	if len(c.copying) == 0 && len(c.behind) == 0 {
		says = append(says, "The update continues once a follower holds the set.")
	}
	if len(c.failing) == 0 {
		says = append(says, "Nothing needs doing unless a follower reports an error.")
	}
	return strings.Join(says, " ")
}

// names lists names as a sentence would: "a", "a and b", "a, b and c".
func names(list []string) string {
	if len(list) < 2 {
		return strings.Join(list, "")
	}
	return strings.Join(list[:len(list)-1], ", ") + " and " + list[len(list)-1]
}

// podRefs names the pods of list in d8-system, for where to look.
func podRefs(list []string) string {
	refs := make([]string, 0, len(list))
	for _, name := range list {
		refs = append(refs, "d8-system/"+name)
	}
	return choose(list, "pod ", "pods ") + names(refs)
}

// choose is one when list has one name, and many otherwise.
func choose(list []string, one, many string) string {
	if len(list) == 1 {
		return one
	}
	return many
}

// replicaState is a follower's own account of how far it has got, in a few words.
func replicaState(
	pod *corev1.Pod, reports map[string]registryv1alpha1.StorageReplicaStatus, leader registryv1alpha1.StorageReplicaStatus,
) string {
	if !podReady(pod) {
		return "not serving"
	}
	report, reported := reports[pod.Spec.NodeName]
	return describeReport(report, reported, &leader)
}

// describeReport puts a report in a few words. leader is what a follower's Full is measured against,
// and nil for the leader's own report.
func describeReport(
	report registryv1alpha1.StorageReplicaStatus, reported bool, leader *registryv1alpha1.StorageReplicaStatus,
) string {
	switch {
	case !reported:
		return "no report yet"
	case report.Error != "":
		return "error: " + report.Error
	case report.Full && leader != nil && !leaderTrusted(leader):
		// Nothing to measure it against, so not "holds": see holdsTheSet.
		return "reports full"
	case report.Full && leader != nil && !layout.HoldsLeadersSet(&report, leader):
		return "full against another set than the leader's"
	case report.Full:
		return "holds the whole set"
	case report.DeclaredDigests > 0:
		return fmt.Sprintf("copying, %d of %d digests", report.VerifiedDigests, report.DeclaredDigests)
	default:
		return "copying"
	}
}
