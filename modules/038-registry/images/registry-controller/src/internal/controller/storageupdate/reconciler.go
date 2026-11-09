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

// Package storageupdate replaces the cache's replicas when their revision changes.
//
// The StatefulSet is told not to do this itself: its RollingUpdate replaces pods by ordinal, and
// which ordinal holds the fill lease is not a property of the ordinal, so the built-in order takes
// the leader down first about as often as not. The design asks for followers first and the leader
// last, which no update strategy can express — a StatefulSet knows nothing about a lease.
//
// Two conditions guard every replacement, both because the cache serves the images it is made of:
//
//   - something must be able to serve the replica's new images while it is gone, since a replaced
//     pod pulls them AFTER it is deleted, from a registry it was part of. Another serving replica
//     covers that, and so does an upstream. The only replica of an air-gapped cache has neither,
//     and there the images must already be on its node: the image holder, a DaemonSet running
//     every image of the storage pod as `/pause` on the masters, pulls them from the old replica
//     while it still serves and keeps them from the image garbage collector. Until the holder on
//     the replica's node runs all of them, the replacement is REFUSED rather than attempted;
//   - every other replica is serving. Taking down two at once turns "a slower pull" into "no pull".
//
// A refusal is reported with its reason, in an event and in the d8_registry_storage_update_blocked
// gauge, because the reasons call for different things: some resolve on their own — a holder
// finishing its pull, a follower its copy — and one needs an operator to bring images into the
// cluster, which is advice that helps there and nowhere else.
//
// The holder does not stand in for a follower when the leader goes. The lease would then move to a
// follower that does not hold the whole set, and the images only the old leader had would be gone
// from the cluster until someone pushed them again — the new leader pod would start, and nothing
// else would.
//
// The result is deliberately slow. Updating the thing that serves every image in the cluster is the
// one place where being quick is worth nothing.
package storageupdate

import (
	"context"
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"

	"github.com/deckhouse/registry-controller/internal/register"
)

const (
	// ControllerName identifies this controller in logs and in --disable-controllers.
	ControllerName = "registry-storage-update"

	// Namespace holds the module's in-cluster components.
	Namespace = "d8-system"

	// StorageName is the StatefulSet whose replicas this replaces.
	StorageName = "registry-storage"

	// StorageLeaseName is the election lease, and the only authority on which replica leads.
	StorageLeaseName = "registry-storage"

	// storageAppLabel selects the replicas.
	storageAppLabel = "registry-storage"

	// holderAppLabel selects the image holders, one per master: templates/v2/storage/image-holder.yaml.
	holderAppLabel = "registry-storage-image-holder"

	// holderDaemonSet is the holders' DaemonSet, named in what an operator is told to look at.
	holderDaemonSet = "registry-storage-image-holder"

	// revisionLabel is what tells a pod of the new revision from a pod of the old one. Set by
	// the StatefulSet controller on every pod it creates.
	revisionLabel = "controller-revision-hash"

	// servingPortName is the container port the agents and the other replicas reach a replica on.
	servingPortName = "registry"

	// settleInterval is how long to wait before looking again while an update is in flight.
	//
	// A replaced pod has to be pulled, started and become ready, and none of that is worth
	// polling tightly for: the next step cannot begin until it is done anyway.
	settleInterval = 15 * time.Second

	// blockedInterval is how often to look again at a cache that cannot be updated at all.
	//
	// Rare, because what would unblock it is an operator bringing images into the cluster, which
	// happens on a human timescale — and because the refusal is already in an event and in the
	// log, where it will be read.
	blockedInterval = 5 * time.Minute
)

func init() {
	register.RegisterController(ControllerName, &appsv1.StatefulSet{}, &Reconciler{})
}

// Reconciler replaces out-of-date replicas of the cache, one at a time, leader last.
type Reconciler struct {
	register.Base

	// Reader lists the replicas past the cache: see replicas. Unset, they are listed from the cache,
	// which is what the tests that do not care about the difference get.
	Reader client.Reader

	// now is the clock a refusal or a failed Delete is repeated by: see announce and
	// announceDeleteFailure. Nil is time.Now; tests set it.
	now func() time.Time

	// mu guards lastRefusal, lastBlockedEvent and lastDeleteFailure. The controller has one key, which
	// the workqueue never reconciles twice at once, so it is not contended: it keeps them safe without
	// leaning on that.
	mu                sync.Mutex
	lastRefusal       announcedRefusal
	lastBlockedEvent  announcedRefusal
	lastDeleteFailure announcedRefusal
}

// InjectAPIReader satisfies register.NeedsAPIReader.
func (r *Reconciler) InjectAPIReader(reader client.Reader) { r.Reader = reader }

// SetupWatches wakes the reconciler on the pods it deletes and on the holder it waits for.
//
// The lease is deliberately NOT watched. It changes every few seconds as the holder renews it, and
// a reconcile per renewal would be a busy loop around a decision that only matters when a pod is
// about to be replaced — which is when it is read.
func (r *Reconciler) SetupWatches(w register.Watcher) {
	toStorage := handler.EnqueueRequestsFromMapFunc(func(_ context.Context, _ client.Object) []ctrl.Request {
		return []ctrl.Request{{
			NamespacedName: types.NamespacedName{Namespace: Namespace, Name: StorageName},
		}}
	})
	w.Watches(&corev1.Pod{}, toStorage, builder.WithPredicates(cachePods()))
}

// cachePods lets through the pods a pass decides on: the replicas and the image holders.
//
// Every other pod in the namespace changes for reasons that say nothing about the update, and each
// change used to cost a pass — which, now that the replicas are read from the API server, is a
// request to it, and which repeated whatever the last pass had concluded.
func cachePods() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(object client.Object) bool {
		if object.GetNamespace() != Namespace {
			return false
		}
		switch object.GetLabels()["app"] {
		case storageAppLabel, holderAppLabel:
			return true
		}
		return false
	})
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if req.Namespace != Namespace || req.Name != StorageName {
		return ctrl.Result{}, nil
	}

	// What the metrics say about this update, published whichever way the pass ends. Zero unless the
	// pass finds otherwise, so a refusal does not outlive the state that caused it: the cache turned
	// off, the replicas all replaced, the gate cleared on the next look. Not on an error, which says
	// nothing new about the update — the last pass that got through is still the best account of it.
	var report updateReport
	result, err := r.reconcile(ctx, req, &report)
	if err == nil {
		report.publish()
		if report.blocked == "" {
			// Ended, so the next refusal is logged at Info even if it is the same one again.
			r.forgetRefusal()
		}
	}
	return result, err
}

func (r *Reconciler) reconcile(ctx context.Context, req ctrl.Request, report *updateReport) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	storage := &appsv1.StatefulSet{}
	if err := r.Client.Get(ctx, req.NamespacedName, storage); err != nil {
		// No cache configured, or not yet. Nothing to replace either way.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// The revision the StatefulSet wants everything on. Empty while it is still working out its
	// own state, which is not a moment to start deleting pods in.
	target := storage.Status.UpdateRevision
	if target == "" {
		return ctrl.Result{}, nil
	}

	pods, err := r.replicas(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}
	report.outdated = outdatedReplicas(storage, pods, target)
	if len(pods) == 0 {
		return ctrl.Result{}, nil
	}

	// Nothing may move while a replica is missing or not yet serving. That covers the pod this
	// controller replaced on its previous pass, which is how "one at a time" is enforced without
	// keeping any state between reconciles — and it holds for the pass right after the deletion only
	// because replicas() reads past the cache.
	if int32(len(pods)) != desiredReplicas(storage) {
		log.V(1).Info("waiting for every replica to exist before replacing any",
			"have", len(pods), "want", desiredReplicas(storage))
		return ctrl.Result{RequeueAfter: settleInterval}, nil
	}

	// A replica left behind on the old revision that has never run goes first, and past every
	// gate below. Those gates protect what serves — one replica away at a time, the leader last, never
	// during a transfer, never with nothing else to pull from — and this one serves nothing, so
	// replacing it takes nothing away. Waiting for it to become ready instead waits forever when
	// the old revision is what keeps it from starting: on an Engine cluster the first render runs
	// before the master NodeGroup exists, so the store is created under /opt, which is read-only
	// there, and the corrected revision never reached the pod.
	if next := staleNotReady(pods, target); next != nil {
		log.Info("replacing a cache replica that is not serving on an out-of-date revision",
			"pod", next.Name, "node", next.Spec.NodeName, "revision", target)
		if err := r.deleteReplica(ctx, next); err != nil {
			r.deleteFailed(ctx, storage, next, err)
		}
		return ctrl.Result{RequeueAfter: settleInterval}, nil
	}

	if notReady := notReadyNames(pods); len(notReady) > 0 {
		log.V(1).Info("waiting for the replicas to serve before replacing any", "notReady", notReady)
		return ctrl.Result{RequeueAfter: settleInterval}, nil
	}

	stale := podsOnOtherRevision(pods, target)
	if len(stale) == 0 {
		return ctrl.Result{}, nil
	}

	reports, err := r.replicaReports(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}
	if behind := addressNotReported(pods, target, reports); len(behind) > 0 {
		// Not at V(1) like the waits above: those end on their own as a pod starts, while this one
		// ends only when a syncer writes its report, and a syncer that never does holds the update
		// with nothing else to show for it. A Normal event, so it does not share the spam budget of
		// the UpdateBlocked warning, and repeats of it are folded into one by the event correlator.
		log.Info("waiting for the replaced replicas to report where they serve before replacing another",
			"replicas", behind, "revision", target)
		if r.Recorder != nil {
			r.Recorder.Eventf(storage, corev1.EventTypeNormal, "WaitingForAddress",
				"Not replacing another replica yet: RegistryStorage still gives these replicas a port they do "+
					"not serve on: %s. The agents are pointed at the reported port, so the update waits for "+
					"their syncers to report their own addresses.", strings.Join(behind, ", "))
		}
		return ctrl.Result{RequeueAfter: settleInterval}, nil
	}

	// Followers first, and the leader only once no follower is left behind. A leader replaced
	// while followers are still on the old revision hands the lease to a replica that is itself
	// about to be replaced, so the fill would stop twice for one update.
	leader := r.leaderNode(ctx)
	next := chooseNext(stale, leader)
	if next == nil {
		log.V(1).Info("the only replica left to replace is the leader, waiting for the followers first")
		return ctrl.Result{RequeueAfter: settleInterval}, nil
	}

	// Whether the cache is complete does not decide this. What completeness guards is dropping the
	// upstream — the layout controller holds it until the leader has the whole set — and replacing a
	// replica takes nothing away from that: a fill interrupted here resumes where it stopped on the
	// replica's next pass, and a push through the publication endpoint, which reaches only the
	// leader, is cut only when the leader itself goes, last. Deferring on it instead meant that a
	// cache unable to complete — a budget it had reached, a fill failing on something this very
	// update fixes — could never be updated at all.

	// And something has to be able to serve images while this replica is away, because what it
	// pulls when it comes back is its own new image.
	decision, err := r.imageSourceWhile(ctx, storage, next, pods, leader)
	if err != nil {
		return ctrl.Result{}, err
	}
	if refused := decision.refused; refused != nil {
		// Refused, and retried on a human timescale — unless what ends it is a replica finishing what
		// it is doing, which nothing wakes this controller for: see refusal.settling. The cases differ
		// in what ends them, which is what the reason and the message are for: see BlockReason.
		report.blocked = refused.reason
		const refusing = "refusing to replace a cache replica: nothing could serve its new images while it is away"
		keys := []any{"pod", next.Name, "revision", target, "leader", leader,
			"reason", refused.reason, "detail", refused.message}
		say, event := r.announce(next.Name + "/" + string(refused.reason) + "/" + refused.key())
		if say {
			log.Info(refusing, keys...)
		} else {
			// Said already, a moment ago: see announce. The gauge is published regardless.
			log.V(1).Info(refusing, keys...)
		}
		if event && r.Recorder != nil {
			r.Recorder.Eventf(storage, corev1.EventTypeWarning, "UpdateBlocked",
				"%s (%s)", refused.message, refused.reason)
		}
		if refused.settling {
			return ctrl.Result{RequeueAfter: settleInterval}, nil
		}
		return ctrl.Result{RequeueAfter: blockedInterval}, nil
	}

	log.Info("replacing a cache replica",
		"pod", next.Name, "node", next.Spec.NodeName, "revision", target, "leader", leader,
		"servedMeanwhileBy", decision.source)
	if err := r.deleteReplica(ctx, next); err != nil {
		r.deleteFailed(ctx, storage, next, err)
		return ctrl.Result{RequeueAfter: settleInterval}, nil
	}
	if decision.outage && r.Recorder != nil {
		// Said where it will be read, because it is the one replacement here that the nodes notice —
		// and only once the Delete went through: a denied one announced an outage that never came.
		r.Recorder.Eventf(storage, corev1.EventTypeNormal, "ReplacingOnlyReplica",
			"Replacing %s, the only replica of an air-gapped cache: its new images are already on %s, held by "+
				"%s, so it comes back without pulling. Until it is ready again — as long as any restart of it "+
				"takes — the cluster cannot pull images, and the kubelet retries a pull that fails meanwhile.",
			next.Name, next.Spec.NodeName, decision.holder)
	}

	return ctrl.Result{RequeueAfter: settleInterval}, nil
}

// deleteReplica deletes pod — the very pod the pass decided on, not merely one by its name.
//
// The StatefulSet recreates a replica under the same name, and a pass that decided on the old pod
// must not delete the new one: that is a replica nobody checked the gates against, possibly not
// serving yet. So the deletion carries the pod's UID, and the API server answers one that no longer
// matches with a Conflict — which here means what NotFound means: the pod decided on is gone already,
// and the next pass looks again at what is there now.
func (r *Reconciler) deleteReplica(ctx context.Context, pod *corev1.Pod) error {
	uid := pod.UID
	err := r.Client.Delete(ctx, pod, client.Preconditions{UID: &uid})
	switch {
	case err == nil, apierrors.IsNotFound(err):
		return nil
	case apierrors.IsConflict(err):
		ctrl.LoggerFrom(ctx).V(1).Info("the replica decided on was replaced meanwhile, looking again",
			"pod", pod.Name, "uid", uid)
		return nil
	}
	// Not wrapped: deleteFailed names the pod, and says what the Delete was for.
	return err
}

// deleteFailed reports a replacement whose Delete failed, which the caller then tries again after
// settleInterval rather than by returning the error.
//
// Returned, the error went to the workqueue's back-off, which doubles from 5 ms up to 1000 s and
// is reset only by a pass that succeeds — and while every replica serves, nothing else wakes this
// controller: the lease is not watched, and no replica or holder changes. On a stand an admission
// policy denied the Delete for ten minutes; once it was lifted, nothing tried again for almost five
// minutes, until an unrelated pod event did. A transient error after a run of such failures would
// hold the update for up to 1000 s, about 17 minutes.
//
// Trying again is a whole pass, not the Delete repeated, so it is as safe as the first try: it reads
// the replicas past the cache and checks every gate again. A Delete that went through despite the
// error is seen going and waited out, a gate that closed meanwhile refuses, and the UID precondition
// keeps the StatefulSet's replacement of the same name out of reach. The interval is fixed because
// the pass keeps no count of failures, deciding from what it reads; a Delete per settleInterval
// costs nothing. The event is said less often than that: see announceDeleteFailure.
func (r *Reconciler) deleteFailed(ctx context.Context, set *appsv1.StatefulSet, pod *corev1.Pod, err error) {
	ctrl.LoggerFrom(ctx).Error(err, "could not replace a cache replica, trying again shortly",
		"pod", pod.Name, "retryIn", settleInterval)
	if r.Recorder != nil && r.announceDeleteFailure(string(pod.UID)+"/"+string(apierrors.ReasonForError(err))) {
		// The reason the StatefulSet controller gives its own failed pod deletions.
		r.Recorder.Eventf(set, corev1.EventTypeWarning, "FailedDelete",
			"Could not delete %s to replace it: %v. Retrying in %s.", pod.Name, err, settleInterval)
	}
}

// announcedRefusal is the last refusal said at Info and in an event, or the last failed Delete said
// in one, and when.
type announcedRefusal struct {
	key string
	at  time.Time
}

// announce reports whether a refusal is to be said at Info rather than only at V(1), and whether in
// an UpdateBlocked event as well, and remembers what it says.
//
// Both say it when it differs from the last one they said. A pass also runs on every event of a
// replica or a holder, and each repeated the refusal the pass before had just made: on a stand both
// waits began with the refusal said twice, 22 and 45 ms apart. key is what decides the refusal, not
// its message, because the message carries a follower's digest count, which changes on every pass
// while the follower copies.
//
// The same refusal is said again at Info once settleInterval has passed, which is every requeue of
// a settling refusal, so that the log shows the wait going on. The event is said again only once
// blockedInterval has passed, for the reason given at announceDeleteFailure: the spam filter gives
// every Warning event of the StatefulSet one allowance, and an UpdateBlocked per settleInterval used
// it up within some six minutes of a wait, after which a FailedDelete, or an UpdateBlocked for
// another reason, was dropped. One per blockedInterval spends no faster than it refills. Not
// forgotten with the refusal either, so that a replica whose readiness flaps cannot spend it.
func (r *Reconciler) announce(key string) (bool, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.clock()
	say := key != r.lastRefusal.key || now.Sub(r.lastRefusal.at) >= settleInterval
	if say {
		r.lastRefusal = announcedRefusal{key: key, at: now}
	}
	event := key != r.lastBlockedEvent.key || now.Sub(r.lastBlockedEvent.at) >= blockedInterval
	if event {
		r.lastBlockedEvent = announcedRefusal{key: key, at: now}
	}
	return say, event
}

// announceDeleteFailure reports whether a failed Delete is to be said in a FailedDelete event, and
// remembers it if so. The log says every one regardless.
//
// Said when the pod or the reason of the error differs from the last one said, and again once
// blockedInterval has passed since. The recorder's spam filter gives every Warning event of the
// StatefulSet one allowance, whatever its reason — a burst of 25, then one per five minutes — so an
// event per settleInterval used it up within some six minutes of a denial, and the UpdateBlocked
// said next could be dropped. One per blockedInterval spends no faster than it refills. Not forgotten
// with the refusal: a pass that fails its Delete ends without one.
func (r *Reconciler) announceDeleteFailure(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.clock()
	if key == r.lastDeleteFailure.key && now.Sub(r.lastDeleteFailure.at) < blockedInterval {
		return false
	}
	r.lastDeleteFailure = announcedRefusal{key: key, at: now}
	return true
}

func (r *Reconciler) clock() time.Time {
	if r.now != nil {
		return r.now()
	}
	return time.Now()
}

// forgetRefusal drops the last refusal said in the log, once a pass ends without one. The last one
// said in an event is kept: see announce.
func (r *Reconciler) forgetRefusal() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastRefusal = announcedRefusal{}
}

// replicas lists the cache's replicas, from the API server rather than the cache.
//
// For the pass right after a deletion. Any event during a pass — the deleted pod's own status, a
// holder's, the StatefulSet's — marks the key dirty, and the next pass starts as soon as this one
// returns, some 100 ms after the Delete, when the informer has often not seen the
// deletionTimestamp yet. Read from the cache, the replica just deleted looked present and serving,
// and the pass decided all over again: on a stand four of the nine replacements deleted their pod
// twice, some 100 ms apart. The same pod each time there, since the decision fell the same way —
// but with two stale followers the next could as well have been the other one, two replicas down
// at once, which is the one thing "one at a time" is for. A List from the API server is read after the Delete it follows,
// so the replica is seen going and waited out below, with no state kept between passes. It costs a
// label-selected List per pass, and the pod watch lets through only the replicas and the holders.
func (r *Reconciler) replicas(ctx context.Context) ([]*corev1.Pod, error) {
	reader := r.Reader
	if reader == nil {
		reader = r.Client
	}
	list := &corev1.PodList{}
	err := reader.List(ctx, list,
		client.InNamespace(Namespace),
		client.MatchingLabels{"app": storageAppLabel})
	if err != nil {
		return nil, fmt.Errorf("listing the cache replicas: %w", err)
	}

	out := make([]*corev1.Pod, 0, len(list.Items))
	for i := range list.Items {
		pod := &list.Items[i]
		if pod.DeletionTimestamp != nil {
			// Already going. Counted as absent, which stops anything else from being
			// deleted alongside it.
			continue
		}
		out = append(out, pod)
	}
	return out, nil
}

// leaderNode is the node whose replica holds the fill lease, or empty when nobody holds it or it
// cannot be read.
//
// Empty is the safe answer here, and it is safe in an unobvious way: with no known leader every
// stale replica looks like a follower, so the update proceeds in ordinal order. That is the
// built-in behaviour this controller exists to improve on, not to guarantee — and stalling the
// update of the cache indefinitely because a lease could not be read would be worse.
func (r *Reconciler) leaderNode(ctx context.Context) string {
	lease := &coordinationv1.Lease{}
	key := types.NamespacedName{Namespace: Namespace, Name: StorageLeaseName}
	if err := r.Client.Get(ctx, key, lease); err != nil || lease.Spec.HolderIdentity == nil {
		return ""
	}
	return *lease.Spec.HolderIdentity
}

// sourceDecision is what can serve images while a replica is away, or why nothing can.
type sourceDecision struct {
	// source names what serves meanwhile, for the log next to the replacement it authorized.
	source string

	// outage is set when nothing serves meanwhile and the replacement is still safe, because the
	// replica's new images are already on its node. holder is the pod holding them.
	outage bool
	holder string

	// refused is set when the replacement must wait, and says why.
	refused *refusal
}

// imageSourceWhile decides what can serve images while a replica is away.
//
// The agents pull from the leader and from nothing else in the cache, so a replica serves the nodes
// only while it leads. That decides what may stand in for the one going:
//
//   - a follower goes: the leader keeps serving, so its replica has to be up;
//   - the leader goes: the lease moves to another replica and the agents follow it, so that replica
//     has to be up and to hold the set the leader holds — a follower still copying would lead with
//     images missing, and so would one whose Full is for the set before the update.
//
// And the upstream, which every node keeps as a fallback, stands in for either. It used to be any
// other ready replica, as if every replica served every node: in air-gap that let the leader go
// with only incomplete followers behind it, and nothing at all could serve while it came back.
//
// One case is left in which nothing serves and the replacement is still safe: the only replica,
// whose node already holds its new images. The nodes cannot pull while it restarts — exactly as
// when it restarts for any other reason — but it comes back, which without the images on the node
// it would not.
func (r *Reconciler) imageSourceWhile(
	ctx context.Context, set *appsv1.StatefulSet, going *corev1.Pod, pods []*corev1.Pod, leaderNode string,
) (sourceDecision, error) {
	storage := &registryv1alpha1.RegistryStorage{}
	key := types.NamespacedName{Name: registryv1alpha1.SingletonName}
	if err := r.Client.Get(ctx, key, storage); err != nil {
		if apierrors.IsNotFound(err) {
			return sourceDecision{refused: &refusal{
				reason: BlockedStorageUnknown,
				message: fmt.Sprintf("Not replacing %s: RegistryStorage %q does not exist, so nothing is known "+
					"of an upstream or of which replicas hold the whole set. This controller creates it; the "+
					"update continues once it exists.", going.Name, registryv1alpha1.SingletonName),
			}}, nil
		}
		return sourceDecision{}, fmt.Errorf("reading RegistryStorage: %w", err)
	}

	reports := map[string]registryv1alpha1.StorageReplicaStatus{}
	for _, replica := range storage.Status.Replicas {
		reports[replica.Node] = replica
	}

	goingLeads := leaderNode != "" && going.Spec.NodeName == leaderNode
	var leaderReport *registryv1alpha1.StorageReplicaStatus
	if report, reported := reports[leaderNode]; reported && leaderNode != "" {
		leaderReport = &report
	}
	var successors []string
	for _, pod := range pods {
		if pod.Name == going.Name || !podReady(pod) {
			continue
		}
		switch {
		case goingLeads:
			if holdsTheSet(reports[pod.Spec.NodeName], leaderReport) {
				successors = append(successors, pod.Name)
			}
		case leaderNode != "" && pod.Spec.NodeName == leaderNode:
			return sourceDecision{source: "the leader's replica " + pod.Name}, nil
		case leaderNode == "":
			// No known leader: nothing says which replica the agents are pointed at, so any
			// serving one is the best answer there is — as before.
			return sourceDecision{source: "replica " + pod.Name}, nil
		}
	}
	if len(successors) > 0 {
		return sourceDecision{source: leaseSuccessors(going, successors)}, nil
	}

	if storage.Spec.Upstream != nil {
		return sourceDecision{source: "upstream " + storage.Spec.Upstream.Host}, nil
	}

	// The only replica — whether or not anyone holds the lease, since nothing else could lead.
	// Both counts, because a missing replica is already waited out above and this must not be the
	// first of two on a cluster that wants more.
	if len(pods) == 1 && desiredReplicas(set) == 1 {
		held, err := r.heldOn(ctx, going.Spec.NodeName, set)
		if err != nil {
			return sourceDecision{}, err
		}
		if held.complete() {
			return sourceDecision{
				source: "nothing: its new images are held on " + going.Spec.NodeName + " by " + held.pod.Name,
				outage: true,
				holder: held.pod.Name,
			}, nil
		}
		return sourceDecision{refused: held.refusal(going)}, nil
	}

	if goingLeads {
		return sourceDecision{refused: leaderRefusal(going, pods, reports)}, nil
	}

	// A follower going, with every replica serving, and none of them on the node the lease names: a
	// lease held by a node whose replica is gone, which the syncers will move on their own.
	return sourceDecision{refused: &refusal{
		reason: BlockedLeaderNotServing,
		message: fmt.Sprintf("Not replacing %s: the lease names %s as the leader, and no serving replica runs "+
			"there, so nothing would serve the nodes while it is away. The replicas move the lease on their "+
			"own once it expires; the update continues then.", going.Name, leaderNode),
	}}, nil
}

// leaseSuccessors says what may take the lease over from going, the leader: every serving follower
// that holds the whole set, or going's own replacement.
//
// Not one name, because which of them leads next is the election's to decide and not this
// controller's: once the old leader releases the lease, whichever replica holding the set asks first
// takes it — a follower on its next retry, or the replacement as it starts, since the StatefulSet puts
// it on the same node over the same store, and that node's report still reads full for the set.
// Naming the first follower in list order said nothing about which one would lead, and two passes on
// a stand named two different ones for the same deletion.
func leaseSuccessors(going *corev1.Pod, followers []string) string {
	followers = slices.Sorted(slices.Values(followers))
	return fmt.Sprintf("a replica holding the whole set, whichever the election picks: %s, or the replacement of %s on %s",
		strings.Join(followers, ", "), going.Name, going.Spec.NodeName)
}

// replicaReports is what the replicas reported in RegistryStorage, or nil when there is no such
// object.
func (r *Reconciler) replicaReports(ctx context.Context) ([]registryv1alpha1.StorageReplicaStatus, error) {
	storage := &registryv1alpha1.RegistryStorage{}
	key := types.NamespacedName{Name: registryv1alpha1.SingletonName}
	if err := r.Client.Get(ctx, key, storage); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading RegistryStorage: %w", err)
	}
	return storage.Status.Replicas, nil
}

// addressNotReported names the replicas on the target revision whose report still gives another
// port than the one their pod serves on.
//
// The registry controller points the agents and the push endpoint at the port a replica reported.
// During the update to the store's current ports a replaced replica's entry keeps its predecessor's
// port until the new syncer writes its own, and replacing the next replica in that window can leave
// the agents with no replica reachable where it is reported. Only a mismatch is waited for: no
// report, an unreadable one or a pod that declares no `registry` port says nothing about a wrong
// address. Once no replica on the previous pod spec can exist, this can go.
func addressNotReported(
	pods []*corev1.Pod, target string, reports []registryv1alpha1.StorageReplicaStatus,
) []string {
	reported := make(map[string]string, len(reports))
	for i := range reports {
		if _, port, err := net.SplitHostPort(reports[i].Address); err == nil {
			reported[reports[i].Node] = port
		}
	}

	var out []string
	for _, pod := range pods {
		if pod.Labels[revisionLabel] != target {
			continue
		}
		served := servingPort(pod)
		port, ok := reported[pod.Spec.NodeName]
		if served == 0 || !ok || port == strconv.Itoa(int(served)) {
			continue
		}
		out = append(out, pod.Name)
	}
	return out
}

// servingPort is the port the pod declares as `registry`, or zero.
func servingPort(pod *corev1.Pod) int32 {
	for i := range pod.Spec.Containers {
		for _, port := range pod.Spec.Containers[i].Ports {
			if port.Name == servingPortName {
				return port.ContainerPort
			}
		}
	}
	return 0
}

func desiredReplicas(storage *appsv1.StatefulSet) int32 {
	if storage.Spec.Replicas == nil {
		return 1
	}
	return *storage.Spec.Replicas
}

func notReadyNames(pods []*corev1.Pod) []string {
	var out []string
	for _, pod := range pods {
		if !podReady(pod) {
			out = append(out, pod.Name)
		}
	}
	return out
}

func podsOnOtherRevision(pods []*corev1.Pod, target string) []*corev1.Pod {
	var out []*corev1.Pod
	for _, pod := range pods {
		if pod.Labels[revisionLabel] != target {
			out = append(out, pod)
		}
	}
	return out
}

// chooseNext picks the replica to replace: any follower, and nil when only the leader is left.
//
// Nil rather than the leader, because the caller has something to say about that case and this
// has nothing to decide: with the followers done the leader is next, but only after they are
// ready, which the caller checks on its next pass.
func chooseNext(stale []*corev1.Pod, leaderNode string) *corev1.Pod {
	var leader *corev1.Pod
	for _, pod := range stale {
		if leaderNode != "" && pod.Spec.NodeName == leaderNode {
			leader = pod
			continue
		}
		return pod
	}
	if leader != nil && len(stale) == 1 {
		// The leader is the last one left, and every follower is already on the new
		// revision and ready — the caller established that before calling.
		return leader
	}
	return nil
}

// staleNotReady is the first replica on another revision than target that cannot serve on it —
// it has never run, or it is crash-looping — or nil.
//
// Not merely not ready. A replica that has served and is failing its readiness probe for a
// moment — a probe has a one second timeout and three tries — is a replica that will serve
// again, and deleting it past the gates is exactly the deletion they exist to refuse: in air-gap
// with one replica, the one that would have come back. Only a replica that cannot serve on the
// revision it has takes nothing away by going:
//
//   - one that has never started a container, which is the case this bypass was made for: a
//     store created under a path its node cannot mount never starts at all;
//   - one whose container is in CrashLoopBackOff: it serves nothing, and on that revision it will
//     not start serving, so waiting for it waited until someone deleted the pod by hand — while
//     the revision that may well fix it was the very thing held back. Its data is on the node and
//     stays there: the StatefulSet puts the replacement on the same node, over the same hostPath.
func staleNotReady(pods []*corev1.Pod, target string) *corev1.Pod {
	for _, pod := range podsOnOtherRevision(pods, target) {
		if !podReady(pod) && (neverRan(pod) || crashLooping(pod)) {
			return pod
		}
	}
	return nil
}

// crashLooping reports that a container of the pod is waiting out a crash-loop back-off.
func crashLooping(pod *corev1.Pod) bool {
	for _, statuses := range [][]corev1.ContainerStatus{
		pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses,
	} {
		for i := range statuses {
			if waiting := statuses[i].State.Waiting; waiting != nil && waiting.Reason == "CrashLoopBackOff" {
				return true
			}
		}
	}
	return false
}

// neverRan reports that no container of the pod has ever started: none is running, none has
// terminated, and none has restarted.
func neverRan(pod *corev1.Pod) bool {
	for _, statuses := range [][]corev1.ContainerStatus{
		pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses,
	} {
		for i := range statuses {
			status := &statuses[i]
			if status.RestartCount > 0 ||
				status.State.Running != nil || status.State.Terminated != nil ||
				status.LastTerminationState.Terminated != nil ||
				(status.Started != nil && *status.Started) {
				return false
			}
		}
	}
	return true
}

func podReady(pod *corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}
	for i := range pod.Status.Conditions {
		condition := &pod.Status.Conditions[i]
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}
