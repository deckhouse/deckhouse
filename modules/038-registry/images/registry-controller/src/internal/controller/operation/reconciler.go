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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"

	"github.com/deckhouse/registry-controller/internal/layout"
	"github.com/deckhouse/registry-controller/internal/register"
)

// ControllerName identifies this controller in logs, events and the --disable-controllers flag.
const ControllerName = "registry-operation"

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
	// masters coming and going.
	w.Watches(&corev1.Node{}, handler.EnqueueRequestsFromMapFunc(r.unfinished),
		builder.WithPredicates(layout.StorageNodesChanged()))
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
		watchOver(operation, storage, now)
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
	return ctrl.Result{RequeueAfter: untilNextDeadline(&operation.Status, now.Time)}, nil
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
// of a canceled operation that no replica has started on. Otherwise an entry a replica is collecting on,
// or one waiting for its turn, is left to it: if its syncer stops answering — a master gone NotReady, a
// pod in a crash loop — only OperationTimeout settles it, and until then the operation holds every
// collection out. Deleting the operation is what releases it sooner.
//
// A waiting entry is settled here as well as by its replica, and that is not a race: a replica that is
// alive settles it the same way, the writes are locked, and a replica whose entry is settled does not
// start — see the syncer's Worker.
func watchOver(
	operation *registryv1alpha1.RegistryOperation, storage *registryv1alpha1.RegistryStorage, now metav1.Time,
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
// the start deadline while an entry is still not picked up, then the timeout.
func untilNextDeadline(status *registryv1alpha1.RegistryOperationStatus, now time.Time) time.Duration {
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

	// A second past it, so the deadline has passed when it is looked at.
	wait := next.Sub(now) + time.Second
	if wait < time.Second {
		wait = time.Second
	}
	return wait
}
