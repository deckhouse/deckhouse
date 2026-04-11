package controlplaneoperation

import (
	"context"
	"fmt"
	"log/slog"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"

	"github.com/deckhouse/deckhouse/pkg/log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// waitForPod checks if the static pod is ready with the expected checksums annotations.
func (r *Reconciler) waitForPod(ctx context.Context, state *controlplanev1alpha1.OperationState, logger *log.Logger) (reconcile.Result, error) {
	op := state.Raw()
	podName := fmt.Sprintf("%s-%s", op.Spec.Component.PodComponentName(), r.node.Name)
	pod := &corev1.Pod{}
	if err := r.client.Get(ctx, client.ObjectKey{Name: podName, Namespace: constants.KubeSystemNamespace}, pod); err != nil {
		logger.Info("pod not found yet, requeue", slog.String("pod", podName))
		return reconcile.Result{RequeueAfter: requeueWaitPod}, nil
	}

	if isPodCrashLooping(pod) {
		logger.Warn("pod is crash looping, will retry", slog.String("pod", podName))
		state.SetReadyReason(constants.ReasonWaitingForPod,
			fmt.Sprintf("pod %s is in CrashLoopBackOff, will retry", podName))
		return reconcile.Result{RequeueAfter: requeueWaitPod}, nil
	}

	if !isPodReadyWithChecksums(pod,
		op.Spec.DesiredConfigChecksum,
		op.Spec.DesiredPKIChecksum,
		op.Spec.DesiredCAChecksum,
		op.CertRenewalID(),
	) {
		logger.Info("pod not ready with expected checksums, requeue", slog.String("pod", podName))
		return reconcile.Result{RequeueAfter: requeueWaitPod}, nil
	}

	logger.Info("pod ready with matching checksums", slog.String("pod", podName))

	state.MarkSucceeded()
	return reconcile.Result{}, nil
}

// mapPodToOperations finds in-progress CPOs for the component matching this pod.
func (r *Reconciler) mapPodToOperations(ctx context.Context, obj client.Object) []reconcile.Request {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return nil
	}

	componentName := pod.Labels[constants.StaticPodComponentLabelKey]
	if componentName == "" {
		return nil
	}

	opComponent, ok := controlplanev1alpha1.OperationComponentFromPodName(componentName)
	if !ok {
		return nil
	}

	ops := &controlplanev1alpha1.ControlPlaneOperationList{}
	if err := r.client.List(ctx, ops, client.MatchingLabels{
		constants.ControlPlaneNodeNameLabelKey:  r.node.Name,
		constants.ControlPlaneComponentLabelKey: string(opComponent),
	}); err != nil {
		return nil
	}

	var reqs []reconcile.Request
	for i := range ops.Items {
		if ops.Items[i].Spec.Approved && !ops.Items[i].IsTerminal() {
			reqs = append(reqs, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: ops.Items[i].Name},
			})
		}
	}
	return reqs
}

// isPodCrashLooping returns true if any container in the pod is in CrashLoopBackOff.
func isPodCrashLooping(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil && cs.State.Waiting.Reason == "CrashLoopBackOff" {
			return true
		}
	}
	return false
}

// isPodReadyWithChecksums returns true if the pod has the expected checksum annotations and is in Ready condition.
func isPodReadyWithChecksums(pod *corev1.Pod, configChecksum, pkiChecksum, caChecksum, certRenewalID string) bool {
	if pod == nil {
		return false
	}

	if configChecksum != "" && pod.Annotations[constants.ConfigChecksumAnnotationKey] != configChecksum {
		return false
	}
	if pkiChecksum != "" && pod.Annotations[constants.PKIChecksumAnnotationKey] != pkiChecksum {
		return false
	}
	if caChecksum != "" && pod.Annotations[constants.CAChecksumAnnotationKey] != caChecksum {
		return false
	}
	if certRenewalID != "" && pod.Annotations[constants.CertRenewalIDAnnotationKey] != certRenewalID {
		return false
	}

	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			return true
		}
	}

	return false
}
