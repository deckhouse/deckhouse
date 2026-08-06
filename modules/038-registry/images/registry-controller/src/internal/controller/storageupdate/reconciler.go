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
// The StatefulSet is told not to do this itself. Its own RollingUpdate replaces pods by ordinal,
// highest first, and which ordinal holds the fill lease is not a property of the ordinal — so the
// built-in order takes the leader down first about as often as not. What the design asks for is
// followers first and the leader last, and no update strategy can express that: a StatefulSet
// knows nothing about a lease.
//
// Two conditions guard every replacement, and both exist because the cache serves the images it is
// made of:
//
//   - something else must be able to serve images while the replica is gone. A replaced pod pulls
//     its new image AFTER it is deleted, from a registry that pod was part of. Another serving
//     replica covers that — they are mirrors of each other in every node layout — and so does an
//     upstream, which every node keeps as a fallback. With one replica and no upstream there is
//     nothing, and the replacement is REFUSED rather than attempted: the way into such a cluster
//     is `d8 mirror pull` and `d8 mirror push`, and until the images are there the update has no
//     business starting;
//   - every other replica is serving. Taking down two at once turns "a slower pull" into "no
//     pull", and on a two-master cluster it would leave nothing behind at all.
//
// An earlier attempt held the new images on the masters with a DaemonSet of `/pause` containers,
// the way the previous implementation did. It does not generalise: `/pause` exists in the images
// Deckhouse packages around third-party binaries and not in the distroless ones its own Go
// components are built on, so half the holder containers could not start at all — and the gate
// that waited for them would have stopped the cache from ever being updated.
//
// The result is deliberately slow. An update of the thing that serves every image in the cluster
// is the one place where being quick is worth nothing.
package storageupdate

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"

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

	// revisionLabel is what tells a pod of the new revision from a pod of the old one. Set by
	// the StatefulSet controller on every pod it creates.
	revisionLabel = "controller-revision-hash"

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
}

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
	w.Watches(&corev1.Pod{}, toStorage)
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if req.Namespace != Namespace || req.Name != StorageName {
		return ctrl.Result{}, nil
	}
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
	if len(pods) == 0 {
		return ctrl.Result{}, nil
	}

	// Nothing may move while a replica is missing or not yet serving. That covers the pod this
	// controller replaced on its previous pass, which is how "one at a time" is enforced without
	// keeping any state between reconciles.
	if int32(len(pods)) != desiredReplicas(storage) {
		log.V(1).Info("waiting for every replica to exist before replacing any",
			"have", len(pods), "want", desiredReplicas(storage))
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

	// Followers first, and the leader only once no follower is left behind. A leader replaced
	// while followers are still on the old revision hands the lease to a replica that is itself
	// about to be replaced, so the fill would stop twice for one update.
	leader := r.leaderNode(ctx)
	next := chooseNext(stale, leader)
	if next == nil {
		log.V(1).Info("the only replica left to replace is the leader, waiting for the followers first")
		return ctrl.Result{RequeueAfter: settleInterval}, nil
	}

	// And something has to be able to serve images while this replica is away, because what it
	// pulls when it comes back is its own new image.
	source, err := r.imageSourceWhile(ctx, next, pods)
	if err != nil {
		return ctrl.Result{}, err
	}
	if source == "" {
		// Refused, not deferred: on a cluster with one replica and no upstream this will never
		// become possible on its own, and saying so is the only useful thing to do. The way
		// forward is `d8 mirror pull` then `d8 mirror push`, which puts the images in the
		// cluster before anything is taken down.
		log.Info("refusing to replace the only replica of an air-gapped cache: nothing else could serve its new image",
			"pod", next.Name, "revision", target)
		if r.Recorder != nil {
			r.Recorder.Eventf(storage, corev1.EventTypeWarning, "UpdateBlocked",
				"Not replacing %s: with one replica and no upstream nothing can serve its new image. "+
					"Bring the images into the cluster first (d8 mirror pull, then d8 mirror push).", next.Name)
		}
		return ctrl.Result{RequeueAfter: blockedInterval}, nil
	}

	log.Info("replacing a cache replica",
		"pod", next.Name, "node", next.Spec.NodeName, "revision", target, "leader", leader,
		"servedMeanwhileBy", source)
	if err := r.Client.Delete(ctx, next); err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("deleting %s: %w", next.Name, err)
	}

	return ctrl.Result{RequeueAfter: settleInterval}, nil
}

func (r *Reconciler) replicas(ctx context.Context) ([]*corev1.Pod, error) {
	list := &corev1.PodList{}
	err := r.Client.List(ctx, list,
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

// imageSourceWhile names what can serve images while a replica is away, or empty when nothing can.
//
// Two things qualify, and they are the two the node layout actually contains: another replica of
// the cache, which is a mirror of this one on every node, and the upstream, which every node keeps
// as a fallback for exactly this reason. The name is returned rather than a bool so the decision
// appears in the log next to the replacement it authorized.
func (r *Reconciler) imageSourceWhile(
	ctx context.Context, going *corev1.Pod, pods []*corev1.Pod,
) (string, error) {
	for _, pod := range pods {
		if pod.Name != going.Name && podReady(pod) {
			return "replica " + pod.Name, nil
		}
	}

	storage := &registryv1alpha1.RegistryStorage{}
	key := types.NamespacedName{Name: registryv1alpha1.SingletonName}
	if err := r.Client.Get(ctx, key, storage); err != nil {
		if apierrors.IsNotFound(err) {
			return "", nil
		}
		return "", fmt.Errorf("reading RegistryStorage: %w", err)
	}
	if storage.Spec.Upstream != nil {
		return "upstream " + storage.Spec.Upstream.Host, nil
	}
	return "", nil
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
