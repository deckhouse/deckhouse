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

package internal

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "manager/src/api/v1"
	"manager/src/internal/helper"
)

const (
	checksumAnnotation = "ingress-nginx-controller.deckhouse.io/checksum"
)

func requeueTrackStatus() *trackStatus {
	return &trackStatus{
		result: ctrl.Result{RequeueAfter: RequeueAfter},
	}
}

func (r *IngressNginxController) SafeUpdateHostWithFailover(
	ctx context.Context,
	ic *v1.IngressNginxController,
) (ctrl.Result, error) {
	// 1. Roll out failover path first and primary controller last.
	tracks := []map[string]string{
		helper.WorkloadLabels("controller", ic.Name+"-failover"),
		helper.WorkloadLabels("proxy-failover", ic.Name),
		helper.WorkloadLabels("controller", ic.Name),
	}

	for _, labels := range tracks {
		// 2. Advance only one track at a time; next track waits until the current one is complete.
		status, err := r.safeUpdateWorkloadTrack(ctx, labels)
		if err != nil {
			return ctrl.Result{}, err
		}
		if !status.completed {
			return status.result, nil
		}
	}

	return ctrl.Result{}, nil
}

func (r *IngressNginxController) safeUpdateWorkloadTrack(
	ctx context.Context,
	labels map[string]string,
) (*trackStatus, error) {
	workloadList, err := r.Workloads.ListByLabels(ctx, controllerNamespace, labels)
	if err != nil {
		return nil, err
	}

	// 1. Safe rollout works only with the native DaemonSet for the target track.
	nativeWorkload, _ := helper.SplitWorkloads(workloadList)
	if nativeWorkload == nil {
		return requeueTrackStatus(), nil
	}

	nativeDaemonSet, ok := nativeWorkload.(helper.NativeDaemonSet)
	if !ok {
		return requeueTrackStatus(), nil
	}

	currentChecksum := nativeDaemonSet.Obj.Annotations[checksumAnnotation]
	if currentChecksum == "" {
		return requeueTrackStatus(), nil
	}

	// 2. Split native pods by revision and stop if a previous deletion is still in flight.
	podList, err := r.Pods.ListByLabels(ctx, controllerNamespace, labels)
	if err != nil {
		return nil, err
	}

	stalePods := make([]client.Object, 0, len(podList))
	for i := range podList {
		pod := &podList[i]
		if pod.DeletionTimestamp != nil {
			return requeueTrackStatus(), nil
		}

		if helper.IsLegacyDaemonSetPod(*pod) {
			continue
		}

		if pod.Annotations[checksumAnnotation] != currentChecksum {
			stalePods = append(stalePods, pod)
		}
	}

	// 3. Track is complete only after the native DaemonSet fully converges on the new revision.
	check, err := r.Workloads.CheckConverged(ctx, nativeWorkload, 0)
	if err != nil {
		return nil, err
	}
	if check.IsConverged() {
		return &trackStatus{completed: true}, nil
	}

	if nativeWorkload.GetObservedGeneration() != nativeWorkload.GetGeneration() {
		return requeueTrackStatus(), nil
	}

	if nativeWorkload.GetCurrentNumberScheduled() != nativeWorkload.GetDesiredNumberScheduled() {
		return requeueTrackStatus(), nil
	}

	if nativeWorkload.GetNumberUnavailable() != 0 {
		return requeueTrackStatus(), nil
	}

	if len(stalePods) == 0 {
		return requeueTrackStatus(), nil
	}

	// 4. Delete exactly one stale pod and wait for the next reconcile to observe the replacement.
	if err := r.Delete(ctx, stalePods[0]); err != nil {
		return nil, err
	}

	return &trackStatus{
		result: ctrl.Result{RequeueAfter: RequeueAfter},
	}, nil
}
