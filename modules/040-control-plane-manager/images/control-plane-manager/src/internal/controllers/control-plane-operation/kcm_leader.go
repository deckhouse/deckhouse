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

package controlplaneoperation

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/pkg/log"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
	"control-plane-manager/internal/operations"
)

// kube-controller-manager does not release its leader Lease on shutdown, so restarting the leader
// leaves the other replicas waiting for the Lease to expire (--leader-elect-lease-duration). The
// operation records the Lease held by this node's instance before the new manifest is written and
// deletes it once that instance is gone, so a standby acquires it within one retry period.
// The approver restarts the leader's instance last, after the others run the new version
// (control-plane-approver/leader.go), so that standby is an updated one.
// Best effort: any doubt or error leaves the Lease alone and never fails the operation.

// kcmLeaderLease is the Lease held by this node's kube-controller-manager before the restart.
type kcmLeaderLease struct {
	HolderIdentity  string    `json:"holderIdentity"`
	UID             types.UID `json:"uid"`
	ResourceVersion string    `json:"resourceVersion"`
}

var kcmLeaseKey = operations.LeaderLeaseKey(controlplanev1alpha1.OperationComponentKubeControllerManager)

const (
	// kcmLeaseRetryPeriod mirrors --leader-elect-retry-period of kube-controller-manager: a live
	// leader renews its Lease this often.
	kcmLeaseRetryPeriod = 2 * time.Second
	// kcmLeaseStaleAfter is how old RenewTime must be before the holder is treated as stopped: three
	// missed renewals. The holder runs on this node, so RenewTime and the check share one clock.
	kcmLeaseStaleAfter = 3 * kcmLeaseRetryPeriod
	// kcmLeaseDuration mirrors --leader-elect-lease-duration: after it the Lease expires on its own,
	// so waiting longer to release it is pointless.
	kcmLeaseDuration = 30 * time.Second
)

// recordKCMLeader stores the kube-controller-manager Lease in an operation annotation when this
// node's instance holds it and is about to be replaced. The annotation survives a controller
// restart between steps.
func (r *Reconciler) recordKCMLeader(ctx context.Context, op *controlplanev1alpha1.ControlPlaneOperation, logger *log.Logger) {
	if op.Spec.Component != controlplanev1alpha1.OperationComponentKubeControllerManager ||
		op.Annotations[constants.KCMLeaderLeaseAnnotationKey] != "" {
		return
	}
	// Desired state already on disk or already running: kubelet will not restart the instance, so
	// its Lease must never be released.
	if matches, err := manifestMatchesDesired(op); err != nil || matches {
		return
	}
	pod := &corev1.Pod{}
	if err := r.client.Get(ctx, client.ObjectKey{Namespace: constants.KubeSystemNamespace, Name: controlplanev1alpha1.OperationComponentKubeControllerManager.MirrorPodName(r.node.Name)}, pod); err != nil {
		logger.Info("cannot read kube-controller-manager pod, its leader Lease will not be released", log.Err(err))
		return
	}
	if hasChecksumAnnotations(pod, checksumAnnotationsFromSpec(op.Spec)) {
		return
	}

	lease := &coordinationv1.Lease{}
	if err := r.apiReader.Get(ctx, kcmLeaseKey, lease); err != nil {
		logger.Warn("cannot read kube-controller-manager leader Lease, it will not be released", log.Err(err))
		return
	}
	holder := ptr.Deref(lease.Spec.HolderIdentity, "")
	if holder == "" || operations.LeaseHolderNode(holder) != r.node.Name {
		return
	}

	value, err := json.Marshal(kcmLeaderLease{HolderIdentity: holder, UID: lease.UID, ResourceVersion: lease.ResourceVersion})
	if err != nil {
		logger.Warn("cannot encode kube-controller-manager leader Lease", log.Err(err))
		return
	}
	if err := r.setKCMLeaderAnnotation(ctx, op, string(value)); err != nil {
		logger.Warn("cannot record kube-controller-manager leader Lease, it will not be released", log.Err(err))
		return
	}
	logger.Info("this node holds the kube-controller-manager leader Lease", slog.String("holder_identity", holder))
}

// handOverKCMLeader releases the Lease recorded by recordKCMLeader as soon as the replacement
// kube-controller-manager runs. It reports whether the operation should wait for the Lease to go
// stale: the stopped holder still renews it, and the Lease has not expired on its own yet.
func (r *Reconciler) handOverKCMLeader(
	ctx context.Context,
	op *controlplanev1alpha1.ControlPlaneOperation,
	pod *corev1.Pod,
	now time.Time,
	logger *log.Logger,
) bool {
	if op.Spec.Component != controlplanev1alpha1.OperationComponentKubeControllerManager {
		return false
	}
	startedAt, ok := kcmRunningSince(pod, checksumAnnotationsFromSpec(op.Spec))
	if !ok {
		return false
	}
	// Recheck a fresh Lease for up to its duration; after that it expires by itself.
	return r.releaseKCMLeader(ctx, op, now, logger) && now.Sub(startedAt) < kcmLeaseDuration
}

// kcmLeaseAction is what releaseKCMLeader does with the recorded Lease.
type kcmLeaseAction int

const (
	// kcmLeaseForget: another instance holds the Lease, or it was recreated.
	kcmLeaseForget kcmLeaseAction = iota
	// kcmLeaseKeep: without renewTime there is no telling whether the holder is alive.
	kcmLeaseKeep
	// kcmLeaseRecheck: the recorded holder renewed the Lease recently.
	kcmLeaseRecheck
	// kcmLeaseRelease: the recorded holder stopped renewing the Lease.
	kcmLeaseRelease
)

// decideKCMLease decides on the recorded Lease by its current state. A live holder renews it every
// kcmLeaseRetryPeriod, so renewTime older than kcmLeaseStaleAfter is direct evidence it is gone.
func decideKCMLease(recorded kcmLeaderLease, lease *coordinationv1.Lease, now time.Time) kcmLeaseAction {
	switch {
	case ptr.Deref(lease.Spec.HolderIdentity, "") != recorded.HolderIdentity || lease.UID != recorded.UID:
		return kcmLeaseForget
	case lease.Spec.RenewTime == nil:
		return kcmLeaseKeep
	case now.Sub(lease.Spec.RenewTime.Time) < kcmLeaseStaleAfter:
		return kcmLeaseRecheck
	default:
		return kcmLeaseRelease
	}
}

// releaseKCMLeader deletes the recorded Lease if the same instance still holds it and has stopped
// renewing it. Call it only once the replacement pod runs (see kcmRunningSince). It returns true
// when the recorded holder still renews, so the caller may check again later.
func (r *Reconciler) releaseKCMLeader(ctx context.Context, op *controlplanev1alpha1.ControlPlaneOperation, now time.Time, logger *log.Logger) bool {
	raw := op.Annotations[constants.KCMLeaderLeaseAnnotationKey]
	if op.Spec.Component != controlplanev1alpha1.OperationComponentKubeControllerManager || raw == "" {
		return false
	}
	var recorded kcmLeaderLease
	if err := json.Unmarshal([]byte(raw), &recorded); err != nil || recorded.HolderIdentity == "" {
		logger.Warn("cannot decode recorded kube-controller-manager leader Lease, not releasing it", slog.String("value", raw))
		r.forgetKCMLeader(ctx, op, logger)
		return false
	}

	lease := &coordinationv1.Lease{}
	if err := r.apiReader.Get(ctx, kcmLeaseKey, lease); err != nil {
		logger.Info("not releasing kube-controller-manager leader Lease: cannot read it", log.Err(err))
		if apierrors.IsNotFound(err) {
			r.forgetKCMLeader(ctx, op, logger)
		}
		return false
	}
	switch decideKCMLease(recorded, lease, now) {
	case kcmLeaseForget:
		logger.Debug("not releasing kube-controller-manager leader Lease: holder changed",
			slog.String("holder_identity", ptr.Deref(lease.Spec.HolderIdentity, "")))
		r.forgetKCMLeader(ctx, op, logger)
	case kcmLeaseKeep:
		logger.Info("not releasing kube-controller-manager leader Lease: no renewTime")
	case kcmLeaseRecheck:
		logger.Debug("not releasing kube-controller-manager leader Lease yet: renewed recently",
			slog.Duration("age", now.Sub(lease.Spec.RenewTime.Time)))
		return true
	case kcmLeaseRelease:
		if err := r.client.Delete(ctx, lease, client.Preconditions{UID: &lease.UID, ResourceVersion: &lease.ResourceVersion}); err != nil {
			logger.Info("not releasing kube-controller-manager leader Lease: delete failed", log.Err(err))
			return false
		}
		logger.Info("released kube-controller-manager leader Lease of the stopped instance", slog.String("holder_identity", recorded.HolderIdentity))
		r.forgetKCMLeader(ctx, op, logger)
	}
	return false
}

// forgetKCMLeader drops the record so later wait ticks skip the Lease read.
func (r *Reconciler) forgetKCMLeader(ctx context.Context, op *controlplanev1alpha1.ControlPlaneOperation, logger *log.Logger) {
	if err := r.setKCMLeaderAnnotation(ctx, op, ""); err != nil {
		logger.Info("cannot clear recorded kube-controller-manager leader Lease", log.Err(err))
	}
}

// setKCMLeaderAnnotation sets the record annotation, or removes it when value is empty.
func (r *Reconciler) setKCMLeaderAnnotation(ctx context.Context, op *controlplanev1alpha1.ControlPlaneOperation, value string) error {
	original := op.DeepCopy()
	if value == "" {
		delete(op.Annotations, constants.KCMLeaderLeaseAnnotationKey)
	} else {
		if op.Annotations == nil {
			op.Annotations = make(map[string]string, 1)
		}
		op.Annotations[constants.KCMLeaderLeaseAnnotationKey] = value
	}
	if err := r.client.Patch(ctx, op, client.MergeFrom(original)); err != nil {
		op.Annotations = original.Annotations
		return err
	}
	return nil
}

// kcmRunningSince returns when the kube-controller-manager container of the mirror pod with the
// new checksums started. The old process may still be alive at that point after a kubelet restart
// between the manifest write and the old pod kill; releaseKCMLeader guards that with RenewTime.
func kcmRunningSince(pod *corev1.Pod, expected checksumAnnotations) (time.Time, bool) {
	if !hasChecksumAnnotations(pod, expected) {
		return time.Time{}, false
	}
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Name == controlplanev1alpha1.OperationComponentKubeControllerManager.PodComponentName() && cs.State.Running != nil {
			return cs.State.Running.StartedAt.Time, true
		}
	}
	return time.Time{}, false
}
