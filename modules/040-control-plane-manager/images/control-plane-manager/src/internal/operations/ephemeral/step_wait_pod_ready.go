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

package ephemeral

import (
	"context"
	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/operations"
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const waitPodReadyRequeue = 5 * time.Second

func (e *StepExecutor) waitPodReady(ctx context.Context) operations.StepResult {
	const step = controlplanev1alpha1.StepWaitPodReady

	target, err := e.loadTargetStatefulSet(ctx)
	if err != nil {
		return operations.StepHasFailed(step, err)
	}

	sts := &appsv1.StatefulSet{}
	if err := e.client.Get(ctx, client.ObjectKeyFromObject(target), sts); err != nil {
		if apierrors.IsNotFound(err) {
			return operations.StepIsProgressing(step, "statefulset not found yet", waitPodReadyRequeue)
		}
		return operations.StepHasFailed(step, fmt.Errorf("get statefulset: %w", err))
	}

	if !sts.DeletionTimestamp.IsZero() {
		return operations.StepIsProgressing(step, "statefulset is terminating", waitPodReadyRequeue)
	}

	if !isStatefulSetRolloutComplete(sts) {
		return operations.StepIsProgressing(step, e.rolloutProgressMessage(ctx, sts), waitPodReadyRequeue)
	}

	return operations.StepIsCompleted(step, "statefulset is ready")
}

func isStatefulSetRolloutComplete(sts *appsv1.StatefulSet) bool {
	if sts.Status.ObservedGeneration < sts.Generation {
		return false
	}

	desired := desiredReplicas(sts)

	return sts.Status.UpdatedReplicas == desired &&
		sts.Status.ReadyReplicas == desired &&
		sts.Status.CurrentRevision == sts.Status.UpdateRevision
}

func desiredReplicas(sts *appsv1.StatefulSet) int32 {
	if sts.Spec.Replicas != nil {
		return *sts.Spec.Replicas
	}

	return 1
}

func (e *StepExecutor) rolloutProgressMessage(ctx context.Context, sts *appsv1.StatefulSet) string {
	base := fmt.Sprintf("statefulset rolling out (%d/%d ready)", sts.Status.ReadyReplicas, desiredReplicas(sts))

	crashed, err := e.crashLoopingPods(ctx, sts)
	if err != nil || len(crashed) == 0 {
		return base
	}
	return fmt.Sprintf("%s; crash-looping pods: %s", base, strings.Join(crashed, ", "))
}

func (e *StepExecutor) crashLoopingPods(ctx context.Context, sts *appsv1.StatefulSet) ([]string, error) {
	selector, err := metav1.LabelSelectorAsSelector(sts.Spec.Selector)
	if err != nil {
		return nil, fmt.Errorf("build selector for statefulset: %w", err)
	}

	pods := &corev1.PodList{}
	if err := e.client.List(ctx, pods,
		client.InNamespace(sts.Namespace),
		client.MatchingLabelsSelector{Selector: selector},
	); err != nil {
		return nil, fmt.Errorf("list pods for statefulset: %w", err)
	}

	var names []string
	for i := range pods.Items {
		pod := &pods.Items[i]
		// The selector is unique to the STS in our topology, but the ownerRef filter
		// prevents accidental matches if a manifest specifies a broad selector.
		if isOwnedBy(pod, sts) && isPodCrashLooping(pod) {
			names = append(names, pod.Name)
		}
	}
	return names, nil
}

func isOwnedBy(pod *corev1.Pod, sts *appsv1.StatefulSet) bool {
	for _, ref := range pod.OwnerReferences {
		if ref.UID == sts.UID {
			return true
		}
	}
	return false
}

func isPodCrashLooping(pod *corev1.Pod) bool {
	statuses := make([]corev1.ContainerStatus, 0, len(pod.Status.InitContainerStatuses)+len(pod.Status.ContainerStatuses))
	statuses = append(statuses, pod.Status.InitContainerStatuses...)
	statuses = append(statuses, pod.Status.ContainerStatuses...)
	for _, cs := range statuses {
		if cs.State.Waiting != nil && cs.State.Waiting.Reason == "CrashLoopBackOff" {
			return true
		}
	}
	return false
}
