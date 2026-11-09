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

// Package operation reconciles RegistryOperation, an operation every storage replica performs on its
// own store.
//
// The operation itself is not performed here. Each replica works on its own store, and its syncer
// reports its progress into its own entry of the operation's status. What is done here is everything no
// single replica can know: whether the operation may run at all, which replicas it is handed to, what
// to do about a replica that left or stopped answering, the sum of what they report, and when a
// finished operation is deleted.
package operation

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"

	"github.com/deckhouse/registry-controller/internal/layout"
	"github.com/deckhouse/registry-controller/internal/register"
)

// ControllerName identifies this controller in logs, events and the --disable-controllers flag.
const ControllerName = "registry-operation"

const (
	// Namespace is where the storage replicas take their leases.
	Namespace = "d8-system"

	// CollectionLeaseName is the lease a storage replica holds for as long as it collects: the syncer's
	// --gc-lease-name.
	CollectionLeaseName = "registry-storage-gc"
)

// lostLeaseGrace is how long a running entry is still left to its replica after the replica stopped
// holding the collection lease. A replica that is alive does not hold it to the end: it gives the lease
// up the moment its collection returns and writes the outcome after that, retrying every fifteen
// seconds while the write is refused. And the lease is renewed by the clock of its node and read by
// the clock of this one. Five minutes covers both, and is nothing next to the day such an entry held
// the operation before.
const lostLeaseGrace = 5 * time.Minute

// notReadyGrace is how long the node of a waiting replica may stay not Ready before the entry is given
// up on. Longer than a master takes to reboot, and twice the five minutes Kubernetes gives a pod on such
// a node by default before it evicts it: a node that comes back within it keeps its replica's turn.
const notReadyGrace = 10 * time.Minute

func init() {
	register.RegisterController(ControllerName, &registryv1alpha1.RegistryOperation{}, &Reconciler{})
}

// Reconciler accepts operations, watches over them and deletes them a day after they finish.
type Reconciler struct {
	register.Base

	// Now is the clock, so a test can move past a deadline without waiting for it.
	Now func() time.Time
}

var _ register.Reconciler = (*Reconciler)(nil)

func (r *Reconciler) now() time.Time {
	if r.Now == nil {
		return time.Now()
	}
	return r.Now()
}

// SetupWatches follows the storage: which replicas exist is read from its status, and a replica
// leaving it, or the cache being turned off, changes what an unfinished operation can still expect.
func (r *Reconciler) SetupWatches(w register.Watcher) {
	w.Watches(&registryv1alpha1.RegistryStorage{}, handler.EnqueueRequestsFromMapFunc(r.unfinished))
	// And the nodes: a master leaving takes its replica with it, and its entry stays behind. Only
	// masters coming and going, and a master going not Ready or back: a replica waiting there is given
	// up on after a while — see lost.
	w.Watches(&corev1.Node{}, handler.EnqueueRequestsFromMapFunc(r.unfinished),
		builder.WithPredicates(predicate.Or(layout.StorageNodesChanged(), storageNodeReadinessChanged())))
}

// storageNodeReadinessChanged lets through a storage node whose Ready condition changed status. Not its
// heartbeats, which kubelet writes into the same condition every few seconds.
func storageNodeReadinessChanged() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc:  func(event.CreateEvent) bool { return false },
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			if _, isMaster := e.ObjectNew.GetLabels()[layout.ControlPlaneLabel]; !isMaster {
				return false
			}
			before, ok := e.ObjectOld.(*corev1.Node)
			after, okNew := e.ObjectNew.(*corev1.Node)
			return ok && okNew && readyStatus(before) != readyStatus(after)
		},
	}
}

// unfinished maps any change of the storage to the operations it can still affect. Usually none: the
// storage status changes with every report of every replica, and an operation is rare.
func (r *Reconciler) unfinished(ctx context.Context, _ client.Object) []reconcile.Request {
	list := &registryv1alpha1.RegistryOperationList{}
	if err := r.Client.List(ctx, list); err != nil {
		ctrl.LoggerFrom(ctx).Error(err, "cannot list the registry operations")
		return nil
	}

	var requests []reconcile.Request
	for i := range list.Items {
		if !list.Items[i].Status.Phase.Finished() {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: list.Items[i].Name},
			})
		}
	}
	return requests
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	operation := &registryv1alpha1.RegistryOperation{}
	if err := r.Client.Get(ctx, req.NamespacedName, operation); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Truncated to what the API server stores, so that a time written here compares equal to itself
	// when it is read back.
	now := metav1.NewTime(r.now()).Rfc3339Copy()

	if operation.Status.Phase.Finished() {
		return r.expire(ctx, operation, now.Time)
	}

	storage, err := r.storage(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}
	var live liveness

	// Locked: the replicas write their own entries of the same list, and a merge patch replaces the
	// whole list. Refused, the reconciliation is requeued and reads what the replicas reported.
	patch := client.MergeFromWithOptions(operation.DeepCopy(), client.MergeFromWithOptimisticLock{})
	before := operation.Status.DeepCopy()

	if operation.Status.Phase == "" {
		list := &registryv1alpha1.RegistryOperationList{}
		if err := r.Client.List(ctx, list); err != nil {
			return ctrl.Result{}, fmt.Errorf("listing the registry operations: %w", err)
		}
		accept(operation, storage, list.Items, now)
	} else {
		if live, err = r.liveness(ctx, operation); err != nil {
			return ctrl.Result{}, err
		}
		watchOver(operation, storage, live, now)
	}
	summarize(operation, now)

	if !equality.Semantic.DeepEqual(before, &operation.Status) {
		if err := r.Client.Status().Patch(ctx, operation, patch); err != nil {
			if apierrors.IsConflict(err) {
				// A replica wrote its entry in between: read again, which is the ordinary case and no
				// error to back off from.
				return ctrl.Result{RequeueAfter: time.Second}, nil
			}
			return ctrl.Result{}, fmt.Errorf("patching the status of RegistryOperation %s: %w", operation.Name, err)
		}
		if operation.Status.Phase != before.Phase {
			ctrl.LoggerFrom(ctx).Info("registry operation moved on", "operation", operation.Name,
				"type", operation.Spec.Type, "phase", operation.Status.Phase, "reason", operation.Status.Reason)
		}
	}

	if operation.Status.Phase.Finished() {
		return ctrl.Result{RequeueAfter: registryv1alpha1.OperationRetention}, nil
	}
	return ctrl.Result{RequeueAfter: untilNextDeadline(&operation.Status, live, now.Time)}, nil
}

// expire deletes a finished operation once it is a day old, and otherwise asks to be called again when
// it is — the way NodeOperation is collected.
func (r *Reconciler) expire(
	ctx context.Context, operation *registryv1alpha1.RegistryOperation, now time.Time,
) (ctrl.Result, error) {
	finished := operation.CreationTimestamp.Time
	if operation.Status.FinishedAt != nil {
		finished = operation.Status.FinishedAt.Time
	}

	if age := now.Sub(finished); age < registryv1alpha1.OperationRetention {
		return ctrl.Result{RequeueAfter: registryv1alpha1.OperationRetention - age}, nil
	}

	if err := r.Client.Delete(ctx, operation); client.IgnoreNotFound(err) != nil {
		return ctrl.Result{}, fmt.Errorf("deleting the finished RegistryOperation %s: %w", operation.Name, err)
	}
	ctrl.LoggerFrom(ctx).Info("deleted a finished registry operation",
		"operation", operation.Name, "type", operation.Spec.Type)
	return ctrl.Result{}, nil
}

// storage returns the storage singleton with the entries of the replicas still expected only, or nil
// when the cache is turned off.
//
// Only those: an entry outlives its pod, and a master that left would otherwise be handed every
// operation and fail each one at its deadline, alongside replicas that did all the work — see
// layout.CurrentReplicas. The copy is never written back.
func (r *Reconciler) storage(ctx context.Context) (*registryv1alpha1.RegistryStorage, error) {
	storage := &registryv1alpha1.RegistryStorage{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: registryv1alpha1.SingletonName}, storage)
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading the storage: %w", err)
	}

	storageNodes, err := layout.ListStorageNodes(ctx, r.Client)
	if err != nil {
		return nil, err
	}
	storage.Status.Replicas = layout.CurrentReplicas(storage.Status.Replicas, storageNodes)
	return storage, nil
}

// liveness is what tells a replica that is gone from one that is only quiet. A replica writes nothing
// while it waits for its turn or collects, so its silence says nothing by itself.
type liveness struct {
	// notReadySince is when each storage node that is not Ready stopped being Ready.
	notReadySince map[string]time.Time

	// lease is the collection lease, read only while an entry is running. Nil when there is none.
	lease *coordinationv1.Lease
}

// liveness reads the storage nodes' readiness and, while a replica collects, the collection lease. The
// lease is not watched: it is renewed every few seconds, and what matters is that it stops being, so it
// is read again when the replica would be given up on — see untilNextDeadline.
func (r *Reconciler) liveness(
	ctx context.Context, operation *registryv1alpha1.RegistryOperation,
) (liveness, error) {
	live := liveness{notReadySince: map[string]time.Time{}}

	nodes := &corev1.NodeList{}
	if err := r.Client.List(ctx, nodes, client.HasLabels{layout.ControlPlaneLabel}); err != nil {
		return live, fmt.Errorf("listing the storage nodes: %w", err)
	}
	for i := range nodes.Items {
		if condition := readyCondition(&nodes.Items[i]); condition != nil && condition.Status != corev1.ConditionTrue {
			live.notReadySince[nodes.Items[i].Name] = condition.LastTransitionTime.Time
		}
	}

	running := slices.ContainsFunc(operation.Status.Replicas, func(entry registryv1alpha1.ReplicaOperationStatus) bool {
		return entry.Phase == registryv1alpha1.ReplicaOperationRunning
	})
	if !running {
		return live, nil
	}
	lease := &coordinationv1.Lease{}
	err := r.Client.Get(ctx, types.NamespacedName{Namespace: Namespace, Name: CollectionLeaseName}, lease)
	switch {
	case apierrors.IsNotFound(err):
	case err != nil:
		return live, fmt.Errorf("reading the collection lease: %w", err)
	default:
		live.lease = lease
	}
	return live, nil
}

// readyCondition is the node's Ready condition, or nil when kubelet has not reported one.
func readyCondition(node *corev1.Node) *corev1.NodeCondition {
	for i := range node.Status.Conditions {
		if node.Status.Conditions[i].Type == corev1.NodeReady {
			return &node.Status.Conditions[i]
		}
	}
	return nil
}

// readyStatus is the status of the node's Ready condition, empty when there is none.
func readyStatus(node *corev1.Node) corev1.ConditionStatus {
	if condition := readyCondition(node); condition != nil {
		return condition.Status
	}
	return ""
}

// accept decides whether a new operation runs, and hands it to the replicas if it does.
func accept(
	operation *registryv1alpha1.RegistryOperation, storage *registryv1alpha1.RegistryStorage,
	all []registryv1alpha1.RegistryOperation, now metav1.Time,
) {
	status := &operation.Status
	refuse := func(phase registryv1alpha1.RegistryOperationPhase, reason, message string) {
		status.Phase, status.Reason, status.Message = phase, reason, message
		status.FinishedAt = &now
	}

	switch {
	case operation.Canceled():
		refuse(registryv1alpha1.OperationCanceled, registryv1alpha1.ReasonOperationCanceled,
			"The operation was canceled before it was accepted.")
		return
	case storage == nil:
		refuse(registryv1alpha1.OperationFailed, registryv1alpha1.ReasonOperationNoCache,
			"The cache is turned off, so there is no store to work on.")
		return
	}

	if older := olderUnfinished(operation, all); older != "" {
		// One at a time, decided by age rather than by who was reconciled first: two operations created
		// together must not both be accepted because each was looked at before the other was written.
		refuse(registryv1alpha1.OperationFailed, registryv1alpha1.ReasonOperationAnotherActive,
			fmt.Sprintf("Operation %s is not finished yet.", older))
		return
	}

	nodes := make([]string, 0, len(storage.Status.Replicas))
	for _, replica := range storage.Status.Replicas {
		if replica.Node != "" {
			nodes = append(nodes, replica.Node)
		}
	}
	if len(nodes) == 0 {
		refuse(registryv1alpha1.OperationFailed, registryv1alpha1.ReasonOperationNoReplicas,
			"No replica of the storage has reported yet.")
		return
	}
	sort.Strings(nodes)

	deadline := metav1.NewTime(now.Add(registryv1alpha1.OperationStartDeadline))
	status.Phase, status.Reason, status.Message = registryv1alpha1.OperationPending, registryv1alpha1.ReasonOperationAccepted, ""
	status.AcceptedAt = &now
	status.Deadline = &deadline
	status.Replicas = make([]registryv1alpha1.ReplicaOperationStatus, 0, len(nodes))
	for _, node := range nodes {
		status.Replicas = append(status.Replicas, registryv1alpha1.ReplicaOperationStatus{
			Node:  node,
			Phase: registryv1alpha1.ReplicaOperationPending,
		})
	}
}

// olderUnfinished names an operation created before this one that is not finished, or returns "".
func olderUnfinished(
	operation *registryv1alpha1.RegistryOperation, all []registryv1alpha1.RegistryOperation,
) string {
	for i := range all {
		other := &all[i]
		if other.Name == operation.Name || other.Status.Phase.Finished() {
			continue
		}
		// One that is canceled before it was accepted will never be, and holds nothing up.
		if other.Status.Phase == "" && other.Canceled() {
			continue
		}
		if CreatedBefore(other, operation) {
			return other.Name
		}
	}
	return ""
}

// CreatedBefore orders operations by creation, and by name within the same second.
func CreatedBefore(a, b *registryv1alpha1.RegistryOperation) bool {
	if !a.CreationTimestamp.Equal(&b.CreationTimestamp) {
		return a.CreationTimestamp.Before(&b.CreationTimestamp)
	}
	return a.Name < b.Name
}

// watchOver settles the entries no replica will settle itself: a replica that left the storage, one
// that did not pick the operation up or get past waiting for its store by the deadline, and the entries
// of a canceled operation that no replica has started on. An entry a replica is collecting on, or one
// waiting for its turn, is left to it until the replica is gone — see lost. A syncer that stops
// answering — a master gone NotReady, a pod in a crash loop — would otherwise hold every collection out
// until OperationTimeout settles its entry. Giving up on a running entry whose replica is in fact still at
// work is safe: the replica finishes, finds its entry settled and writes nothing, and the lease keeps any
// other replica from collecting beside it.
//
// A waiting entry is settled here as well as by its replica, and that is not a race: a replica that is
// alive settles it the same way, the writes are locked, and a replica whose entry is settled does not
// start — see the syncer's Worker.
func watchOver(
	operation *registryv1alpha1.RegistryOperation, storage *registryv1alpha1.RegistryStorage, live liveness,
	now metav1.Time,
) {
	status := &operation.Status

	present := map[string]bool{}
	if storage != nil {
		for _, replica := range storage.Status.Replicas {
			present[replica.Node] = true
		}
	}

	pastDeadline := status.Deadline != nil && !now.Before(status.Deadline)
	timedOut := status.AcceptedAt != nil &&
		!now.Time.Before(status.AcceptedAt.Add(registryv1alpha1.OperationTimeout))

	for i := range status.Replicas {
		entry := &status.Replicas[i]
		if entry.Phase.Finished() {
			continue
		}

		var (
			phase   registryv1alpha1.ReplicaOperationPhase
			reason  string
			message string

			lostAt, lostWhy, canBeLost = lost(entry, live)
		)
		switch {
		case storage == nil:
			phase, reason, message = registryv1alpha1.ReplicaOperationSkipped, registryv1alpha1.ReasonOperationNoCache,
				"The cache was turned off."
		case !present[entry.Node]:
			phase, reason, message = registryv1alpha1.ReplicaOperationSkipped, registryv1alpha1.ReasonOperationReplicaGone,
				"The replica left the storage."
		case entry.Phase == registryv1alpha1.ReplicaOperationPending && operation.Canceled():
			phase, reason, message = registryv1alpha1.ReplicaOperationCanceled, registryv1alpha1.ReasonOperationCanceled,
				"The operation was canceled before the replica picked it up."
		case entry.Phase == registryv1alpha1.ReplicaOperationWaiting && operation.Canceled():
			phase, reason, message = registryv1alpha1.ReplicaOperationCanceled, registryv1alpha1.ReasonOperationCanceled,
				"The operation was canceled before the replica started."
		case entry.Phase == registryv1alpha1.ReplicaOperationPending && pastDeadline:
			phase, reason, message = registryv1alpha1.ReplicaOperationFailed, registryv1alpha1.ReasonOperationDeadlineExceeded,
				"The replica did not pick the operation up before the deadline."
		case waitsForStore(entry) && pastDeadline:
			phase, reason, message = registryv1alpha1.ReplicaOperationFailed, registryv1alpha1.ReasonOperationDeadlineExceeded,
				fmt.Sprintf("The store was still being written to at the deadline (%s).", entry.Reason)
		case canBeLost && !now.Time.Before(lostAt):
			phase, reason, message = registryv1alpha1.ReplicaOperationFailed, registryv1alpha1.ReasonOperationReplicaLost, lostWhy
		case timedOut:
			phase, reason, message = registryv1alpha1.ReplicaOperationFailed, registryv1alpha1.ReasonOperationTimedOut,
				fmt.Sprintf("The replica did not report a result within %s.", registryv1alpha1.OperationTimeout)
		default:
			continue
		}

		entry.Phase, entry.Reason, entry.Message = phase, reason, message
		entry.FinishedAt = &now
	}
}

// lost says when an unfinished entry is given up on because its replica is gone, and why. False while
// nothing says it is.
//
// A collecting replica holds the collection lease and renews it every few seconds, so the sign is a lease
// it no longer holds: its syncer died, or its node lost the API server. A waiting replica holds nothing,
// and the sign is its node being not Ready. A syncer that crashes while it waits on a node that is Ready
// starts again and takes its place back, or is caught by the timeout.
func lost(entry *registryv1alpha1.ReplicaOperationStatus, live liveness) (time.Time, string, bool) {
	switch entry.Phase {
	case registryv1alpha1.ReplicaOperationRunning:
		return heldUntil(entry, live.lease).Add(lostLeaseGrace),
			"The replica stopped holding the collection lease while it was collecting, and did not report the result.", true
	case registryv1alpha1.ReplicaOperationWaiting:
		since, notReady := live.notReadySince[entry.Node]
		if !notReady {
			return time.Time{}, "", false
		}
		return since.Add(notReadyGrace),
			fmt.Sprintf("The node was not Ready for %s while the replica waited.", notReadyGrace), true
	}
	return time.Time{}, "", false
}

// heldUntil is the latest the replica of a running entry may still have held the collection lease.
//
// While it holds the lease, that is when the lease lapses unless renewed. Once it does not, the lease
// changed hands after it last held it — given up, or taken by another replica — and the time of that
// change is recorded in acquireTime either way. Without a lease at all — a holder that renews puts a
// deleted one back within seconds — nothing has held it since the replica started.
func heldUntil(entry *registryv1alpha1.ReplicaOperationStatus, lease *coordinationv1.Lease) time.Time {
	var started time.Time
	if entry.StartedAt != nil {
		started = entry.StartedAt.Time
	}
	if lease == nil {
		return started
	}

	spec := &lease.Spec
	if spec.HolderIdentity != nil && *spec.HolderIdentity == entry.Node {
		renewed := started
		switch {
		case spec.RenewTime != nil:
			renewed = spec.RenewTime.Time
		case spec.AcquireTime != nil:
			renewed = spec.AcquireTime.Time
		}
		if spec.LeaseDurationSeconds != nil {
			renewed = renewed.Add(time.Duration(*spec.LeaseDurationSeconds) * time.Second)
		}
		return renewed
	}

	switch {
	case spec.AcquireTime != nil:
		return spec.AcquireTime.Time
	case spec.RenewTime != nil:
		return spec.RenewTime.Time
	}
	return started
}

// waitsForStore is an entry still held back by its store being written to. The deadline is for exactly
// that; a replica past it waits for its turn, and says Turn.
func waitsForStore(entry *registryv1alpha1.ReplicaOperationStatus) bool {
	return entry.Phase == registryv1alpha1.ReplicaOperationWaiting &&
		(entry.Reason == registryv1alpha1.ReasonOperationFilling || entry.Reason == registryv1alpha1.ReasonOperationPushing)
}

// summarize derives the top of the status from the entries.
func summarize(operation *registryv1alpha1.RegistryOperation, now metav1.Time) {
	status := &operation.Status
	if status.Phase.Finished() {
		// Refused on acceptance: there are no entries to derive anything from.
		return
	}

	var (
		started, finished               *metav1.Time
		running, waiting                bool
		succeeded, failed, cancels, all int
		failures                        []string
		allFinished                     = true
	)
	for i := range status.Replicas {
		entry := &status.Replicas[i]
		if entry.StartedAt != nil && (started == nil || entry.StartedAt.Before(started)) {
			started = entry.StartedAt
		}
		if entry.FinishedAt != nil && (finished == nil || finished.Before(entry.FinishedAt)) {
			finished = entry.FinishedAt
		}

		all++
		switch entry.Phase {
		case registryv1alpha1.ReplicaOperationRunning:
			running = true
		case registryv1alpha1.ReplicaOperationWaiting:
			waiting = true
		case registryv1alpha1.ReplicaOperationSucceeded:
			succeeded++
		case registryv1alpha1.ReplicaOperationFailed:
			failed++
			failures = append(failures, fmt.Sprintf("%s (%s)", entry.Node, entry.Reason))
		case registryv1alpha1.ReplicaOperationCanceled:
			cancels++
		}
		if !entry.Phase.Finished() {
			allFinished = false
		}
	}

	status.StartedAt = started
	if operation.Spec.Type == registryv1alpha1.OperationGarbageCollection {
		status.GarbageCollection = sumCollections(status.Replicas)
	}

	if !allFinished {
		switch {
		case running || started != nil || succeeded+failed > 0:
			status.Phase, status.Reason = registryv1alpha1.OperationRunning, registryv1alpha1.ReasonOperationInProgress
		default:
			status.Phase, status.Reason = registryv1alpha1.OperationPending, registryv1alpha1.ReasonOperationAccepted
		}
		status.Message = ""
		if waiting && !running {
			status.Message = "Waiting: " + waitingFor(status.Replicas) + "."
		}
		return
	}

	if finished == nil {
		finished = &now
	}
	status.FinishedAt = finished

	// Canceled only when a replica was actually stopped by it: a label set while the last replica was
	// finishing does not undo an operation that went through.
	switch {
	case cancels > 0:
		status.Phase, status.Reason = registryv1alpha1.OperationCanceled, registryv1alpha1.ReasonOperationCanceled
		status.Message = fmt.Sprintf("Canceled after %d of %d replicas finished.", succeeded, all)
	case failed > 0:
		status.Phase, status.Reason = registryv1alpha1.OperationFailed, registryv1alpha1.ReasonOperationReplicaFailed
		status.Message = fmt.Sprintf("%d of %d replicas did not finish: %s.", failed, all, strings.Join(failures, ", "))
	case succeeded == 0 && skippedFor(status.Replicas, registryv1alpha1.ReasonOperationNoCache):
		status.Phase, status.Reason = registryv1alpha1.OperationFailed, registryv1alpha1.ReasonOperationNoCache
		status.Message = "The cache was turned off."
	case succeeded == 0:
		status.Phase, status.Reason = registryv1alpha1.OperationFailed, registryv1alpha1.ReasonOperationNoReplicas
		status.Message = "Every replica left the storage."
	default:
		status.Phase, status.Reason = registryv1alpha1.OperationSucceeded, registryv1alpha1.ReasonOperationCompleted
		status.Message = fmt.Sprintf("%d of %d replicas finished.", succeeded, all)
	}
}

// skippedFor reports whether an entry was skipped for the reason.
func skippedFor(replicas []registryv1alpha1.ReplicaOperationStatus, reason string) bool {
	for i := range replicas {
		if replicas[i].Phase == registryv1alpha1.ReplicaOperationSkipped && replicas[i].Reason == reason {
			return true
		}
	}
	return false
}

// sumCollections adds up what the replicas collected. Nil while none has reported anything.
func sumCollections(replicas []registryv1alpha1.ReplicaOperationStatus) *registryv1alpha1.GarbageCollectionResult {
	var (
		total    registryv1alpha1.GarbageCollectionResult
		reported bool
		bytes    int64
		measured bool
		blobs    int32
		counted  bool
	)
	for _, entry := range replicas {
		result := entry.GarbageCollection
		if result == nil {
			continue
		}
		reported = true
		total.RemovedManifests += result.RemovedManifests
		if result.RemovedBlobs != nil {
			blobs += *result.RemovedBlobs
			counted = true
		}
		for _, version := range result.RemovedVersions {
			if !slices.Contains(total.RemovedVersions, version) {
				total.RemovedVersions = append(total.RemovedVersions, version)
			}
		}
		if result.ReclaimedBytes != nil {
			bytes += *result.ReclaimedBytes
			measured = true
		}
	}
	if !reported {
		return nil
	}
	sort.Strings(total.RemovedVersions)
	if measured {
		total.ReclaimedBytes = &bytes
	}
	if counted {
		total.RemovedBlobs = &blobs
	}
	return &total
}

// waitingFor names the replicas that are held back and by what.
func waitingFor(replicas []registryv1alpha1.ReplicaOperationStatus) string {
	var parts []string
	for _, entry := range replicas {
		if entry.Phase == registryv1alpha1.ReplicaOperationWaiting {
			parts = append(parts, fmt.Sprintf("%s (%s)", entry.Node, entry.Reason))
		}
	}
	return strings.Join(parts, ", ")
}

// untilNextDeadline is when an unfinished operation has to be looked at again even if nothing reports:
// the start deadline while an entry is still not picked up, the moment a replica would be given up on
// as lost, then the timeout. A running replica that keeps renewing the lease moves that moment on, so
// the lease is read again every few minutes while it collects.
func untilNextDeadline(status *registryv1alpha1.RegistryOperationStatus, live liveness, now time.Time) time.Duration {
	if status.AcceptedAt == nil {
		return 0
	}

	next := status.AcceptedAt.Add(registryv1alpha1.OperationTimeout)
	if status.Deadline != nil && now.Before(status.Deadline.Time) {
		for i := range status.Replicas {
			if entry := &status.Replicas[i]; entry.Phase == registryv1alpha1.ReplicaOperationPending || waitsForStore(entry) {
				next = status.Deadline.Time
				break
			}
		}
	}
	for i := range status.Replicas {
		if at, _, ok := lost(&status.Replicas[i], live); ok && now.Before(at) && at.Before(next) {
			next = at
		}
	}

	// A second past it, so the deadline has passed when it is looked at.
	wait := next.Sub(now) + time.Second
	if wait < time.Second {
		wait = time.Second
	}
	return wait
}
