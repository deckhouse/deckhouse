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
//   - the images of the new revision are already on the node, held there by the image-holder
//     DaemonSet. A replaced pod pulls after it is deleted, from a registry it is itself part of;
//     with one replica and no upstream, that pull has no source at all;
//   - every other replica is serving. Taking down two at once turns "a slower pull" into "no
//     pull", and on a two-master cluster it would leave nothing behind at all.
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

	"github.com/deckhouse/registry-controller/internal/register"
)

const (
	// ControllerName identifies this controller in logs and in --disable-controllers.
	ControllerName = "registry-storage-update"

	// Namespace holds the module's in-cluster components.
	Namespace = "d8-system"

	// StorageName is the StatefulSet whose replicas this replaces.
	StorageName = "registry-storage"

	// ImageHolderName is the DaemonSet that brings the new images onto the masters before a
	// replica is replaced.
	ImageHolderName = "registry-storage-image-holder"

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
	w.Watches(&appsv1.DaemonSet{}, toStorage)
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

	// And the images have to be on that node already, because the replacement pulls them from a
	// registry this pod is part of.
	held, err := r.imagesHeldOn(ctx, next.Spec.NodeName)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !held {
		log.Info("waiting for the new images to be held on the node before replacing its replica",
			"pod", next.Name, "node", next.Spec.NodeName)
		return ctrl.Result{RequeueAfter: settleInterval}, nil
	}

	log.Info("replacing a cache replica",
		"pod", next.Name, "node", next.Spec.NodeName, "revision", target, "leader", leader)
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

// imagesHeldOn reports whether the image holder is running the current revision on a node.
//
// What is asked of the holder is only that its pod on that node is ready: its containers do
// nothing but exist, so ready means kubelet pulled every image and started them. That is exactly
// the fact the replacement needs.
func (r *Reconciler) imagesHeldOn(ctx context.Context, node string) (bool, error) {
	if node == "" {
		return false, nil
	}

	holder := &appsv1.DaemonSet{}
	key := types.NamespacedName{Namespace: Namespace, Name: ImageHolderName}
	if err := r.Client.Get(ctx, key, holder); err != nil {
		if apierrors.IsNotFound(err) {
			// No holder, so nothing can be said about what the node has. Refusing to
			// replace anything is the conservative answer, and a missing holder is a
			// rendering problem that will be fixed by the release that caused it.
			return false, nil
		}
		return false, fmt.Errorf("reading the image holder: %w", err)
	}

	list := &corev1.PodList{}
	err := r.Client.List(ctx, list,
		client.InNamespace(Namespace),
		client.MatchingLabels{"app": ImageHolderName})
	if err != nil {
		return false, fmt.Errorf("listing the image holder pods: %w", err)
	}

	for i := range list.Items {
		pod := &list.Items[i]
		if pod.Spec.NodeName != node || pod.DeletionTimestamp != nil {
			continue
		}
		// The holder must be on the generation that carries the new images, not merely
		// running: a holder still on the previous generation holds the previous images.
		if pod.Labels["pod-template-generation"] != "" &&
			pod.Labels["pod-template-generation"] != fmt.Sprint(holder.Generation) {
			continue
		}
		if podReady(pod) {
			return true, nil
		}
	}
	return false, nil
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
