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
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"

	v1 "manager/src/api/v1"
	"manager/src/internal/helper"
)

const (
	RequeueAfter = 5 * time.Second
)

type trackStatus struct {
	completed bool
	result    ctrl.Result
}

func (r *IngressNginxController) MigrateHostInlet(
	ctx context.Context,
	ic *v1.IngressNginxController,
	bootstrap bool,
) (ctrl.Result, error) {
	// 1. Resolve the main controller workload pair: native DS and legacy ADS.
	status, err := r.migrateWorkloadTrack(
		ctx,
		helper.WorkloadLabels("controller", ic.Name),
		bootstrap,
	)
	if err != nil {
		return ctrl.Result{}, err
	}
	// 2. Wait until the current migration step is completed before proceeding.
	if !status.completed {
		return status.result, nil
	}

	// 3. Migration for the host inlet is complete when no legacy pods are left.
	return ctrl.Result{}, nil
}

func (r *IngressNginxController) MigrateHostWithFailover(
	ctx context.Context,
	ic *v1.IngressNginxController,
	bootstrap bool,
) (ctrl.Result, error) {
	failoverTrackName := ic.Name + "-failover"

	// 1. Migrate the failover controller first. Primary must not move before backup path is stable.
	failoverStatus, err := r.migrateWorkloadTrack(
		ctx,
		helper.WorkloadLabels("controller", failoverTrackName),
		bootstrap,
	)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !failoverStatus.completed {
		return failoverStatus.result, nil
	}

	// 2. Migrate proxy-failover next. Failover path is only complete when proxy is also native.
	proxyStatus, err := r.migrateWorkloadTrack(
		ctx,
		helper.WorkloadLabels("proxy-failover", ic.Name),
		bootstrap,
	)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !proxyStatus.completed {
		return proxyStatus.result, nil
	}

	// 3. Only after the whole failover path is migrated, continue with the primary controller migration.
	primaryStatus, err := r.migrateWorkloadTrack(
		ctx,
		helper.WorkloadLabels("controller", ic.Name),
		bootstrap,
	)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !primaryStatus.completed {
		return primaryStatus.result, nil
	}

	// 4. HostWithFailover migration is complete when both failover and primary tracks are done.
	return ctrl.Result{}, nil
}

func (r *IngressNginxController) MigrateLoadBalancer(
	ctx context.Context,
	ic *v1.IngressNginxController,
	_ bool,
) (ctrl.Result, error) {
	// 1. Load both workload implementations.
	labels := helper.WorkloadLabels("controller", ic.Name)
	workloadList, err := r.Workloads.ListByLabels(ctx, controllerNamespace, labels)
	if err != nil {
		return ctrl.Result{}, err
	}

	nativeWorkload, legacyWorkload := helper.SplitWorkloads(workloadList)
	if nativeWorkload == nil {
		// 2. Native workload must exist before the controller can progress this track.
		return ctrl.Result{RequeueAfter: RequeueAfter}, nil
	}

	// 3. Wait all native pods be ready.
	check, err := r.Workloads.CheckConverged(ctx, nativeWorkload, 0)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !check.IsConverged() {
		return ctrl.Result{RequeueAfter: RequeueAfter}, nil
	}

	// 4. Wait until all legacy pods are gone after the legacy workload is removed.
	podList, err := r.Pods.ListByLabels(ctx, controllerNamespace, labels)
	if err != nil {
		return ctrl.Result{}, err
	}

	legacyPods, _ := helper.SplitPods(podList)
	if legacyWorkload == nil {
		if len(legacyPods) > 0 {
			return ctrl.Result{RequeueAfter: RequeueAfter}, nil
		}

		return ctrl.Result{}, nil
	}

	// 5. Drop the legacy workload only after the native one has fully converged.
	switch workload := legacyWorkload.(type) {
	case helper.AdvancedDaemonSet:
		if err := r.Delete(ctx, workload.Obj); err != nil && !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	default:
		return ctrl.Result{}, nil
	}

	return ctrl.Result{RequeueAfter: RequeueAfter}, nil
}

func (r *IngressNginxController) migrateWorkloadTrack(
	ctx context.Context,
	labels map[string]string,
	bootstrap bool,
) (*trackStatus, error) {
	// 1. Load both workload implementations for the same track.
	workloadList, err := r.Workloads.ListByLabels(ctx, controllerNamespace, labels)
	if err != nil {
		return nil, err
	}

	nativeWorkload, legacyWorkload := helper.SplitWorkloads(workloadList)
	if nativeWorkload == nil {
		// 2. Native workload must exist before the controller can progress this track.
		return &trackStatus{
			result: ctrl.Result{RequeueAfter: RequeueAfter},
		}, nil
	}

	// 3. Inspect pods for the track and split them by legacy/native ownership.
	podList, err := r.Pods.ListByLabels(ctx, controllerNamespace, labels)
	if err != nil {
		return nil, err
	}

	legacyPods, nativePods := helper.SplitPods(podList)

	if len(legacyPods) == 0 {
		// 4. Track is complete only after native workload reaches terminal convergence.
		check, err := r.Workloads.CheckConverged(ctx, nativeWorkload, 0)
		if err != nil {
			return nil, err
		}
		if !check.IsConverged() {
			return &trackStatus{
				result: ctrl.Result{RequeueAfter: RequeueAfter},
			}, nil
		}

		return &trackStatus{completed: true}, nil
	}

	if legacyWorkload == nil {
		// 5. Keep waiting while the legacy object still has pods but was not observed in cache yet.
		return &trackStatus{
			result: ctrl.Result{RequeueAfter: RequeueAfter},
		}, nil
	}

	if bootstrap {
		// 6. Bootstrap step: start migration by deleting the first legacy pod as soon
		// as the native workload has been observed by its controller.
		if nativeWorkload.GetObservedGeneration() != nativeWorkload.GetGeneration() {
			return &trackStatus{
				result: ctrl.Result{RequeueAfter: RequeueAfter},
			}, nil
		}

		if err := r.Delete(ctx, &legacyPods[0]); err != nil {
			return nil, err
		}

		return &trackStatus{
			result: ctrl.Result{RequeueAfter: RequeueAfter},
		}, nil
	}

	if len(nativePods) == 0 {
		// 7. Wait until the bootstrap deletion produces the first native pod.
		return &trackStatus{
			result: ctrl.Result{RequeueAfter: RequeueAfter},
		}, nil
	} else {
		// 8. Once native pods have started appearing, require at least one ready native pod
		// before deleting the next legacy pod.
		check, err := r.Workloads.CheckProgressReady(ctx, nativeWorkload, 1)
		if err != nil {
			return nil, err
		}
		if !check.IsConverged() {
			return &trackStatus{
				result: ctrl.Result{RequeueAfter: RequeueAfter},
			}, nil
		}
	}

	// 9. Delete exactly one legacy pod and wait for the next reconcile cycle to stabilize the system.
	if err := r.Delete(ctx, &legacyPods[0]); err != nil {
		return nil, err
	}

	return &trackStatus{
		result: ctrl.Result{RequeueAfter: RequeueAfter},
	}, nil
}
