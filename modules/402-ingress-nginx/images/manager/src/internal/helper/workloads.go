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

package helper

import (
	"context"
	"fmt"
	"sort"

	kruiseappsv1alpha1 "github.com/openkruise/kruise/apis/apps/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type WorkloadService struct {
	client ctrlclient.Client
}

func NewWorkloadService(client ctrlclient.Client) *WorkloadService {
	return &WorkloadService{
		client: client,
	}
}

type DaemonWorkload interface {
	Kind() string
	NamespacedName() string

	GetGeneration() int64
	GetObservedGeneration() int64

	GetDesiredNumberScheduled() int32
	GetCurrentNumberScheduled() int32
	GetUpdatedNumberScheduled() int32
	GetNumberReady() int32
	GetNumberAvailable() int32
	GetNumberUnavailable() int32

	GetPodSelector() *metav1.LabelSelector
	GetNamespace() string
	GetCreationTimestamp() metav1.Time
}

func (s *WorkloadService) ListByLabels(
	ctx context.Context,
	namespace string,
	labels map[string]string,
) ([]DaemonWorkload, error) {
	var result []DaemonWorkload

	var dsList appsv1.DaemonSetList
	if err := s.client.List(
		ctx,
		&dsList,
		ctrlclient.InNamespace(namespace),
		ctrlclient.MatchingLabels(labels),
	); err != nil {
		return nil, err
	}

	for i := range dsList.Items {
		result = append(result, NativeDaemonSet{Obj: &dsList.Items[i]})
	}

	var adsList kruiseappsv1alpha1.DaemonSetList
	if err := s.client.List(
		ctx,
		&adsList,
		ctrlclient.InNamespace(namespace),
		ctrlclient.MatchingLabels(labels),
	); err != nil {
		if !apimeta.IsNoMatchError(err) &&
			!apiruntime.IsNotRegisteredError(err) &&
			!apierrors.IsNotFound(err) {
			return nil, err
		}
	}

	for i := range adsList.Items {
		result = append(result, AdvancedDaemonSet{Obj: &adsList.Items[i]})
	}

	sort.SliceStable(result, func(i, j int) bool {
		ti := result[i].GetCreationTimestamp().Time
		tj := result[j].GetCreationTimestamp().Time
		if ti.Equal(tj) {
			return result[i].NamespacedName() < result[j].NamespacedName()
		}
		return ti.Before(tj)
	})

	return result, nil
}

type SyncCheckResult struct {
	Workload      string
	Kind          string
	Ready         bool
	RealReadyPods int32
	MinReadyPods  int32
	Reasons       []string
}

func (r *SyncCheckResult) IsConverged() bool {
	if r == nil {
		return false
	}

	return r.Ready
}

func (r *SyncCheckResult) AddReason(reason string) {
	r.Ready = false
	r.Reasons = append(r.Reasons, reason)
}

// CheckSynced is kept as a compatibility wrapper while the controller code is evolving.
func (s *WorkloadService) CheckSynced(
	ctx context.Context,
	w DaemonWorkload,
	minReadyPods int32,
) (*SyncCheckResult, error) {
	return s.CheckConverged(ctx, w, minReadyPods)
}

func (s *WorkloadService) CheckConverged(
	ctx context.Context,
	w DaemonWorkload,
	minReadyPods int32,
) (*SyncCheckResult, error) {
	realReadyPods, err := s.CountReadyPods(ctx, w)
	if err != nil {
		return nil, err
	}

	minReadyPods = normalizeMinReadyPods(w, minReadyPods)

	res := &SyncCheckResult{
		Workload:      w.NamespacedName(),
		Kind:          w.Kind(),
		Ready:         true,
		RealReadyPods: realReadyPods,
		MinReadyPods:  minReadyPods,
	}

	evaluateConvergence(res, w)

	if w.GetNumberReady() < minReadyPods {
		res.AddReason(fmt.Sprintf(
			"numberReady(%d) < minReadyPods(%d)",
			w.GetNumberReady(),
			minReadyPods,
		))
	}

	if realReadyPods < minReadyPods {
		res.AddReason(fmt.Sprintf(
			"realReadyPods(%d) < minReadyPods(%d)",
			realReadyPods,
			minReadyPods,
		))
	}

	return res, nil
}

// CheckProgressReady validates that the native workload has been observed by the
// controller and already serves traffic on at least one migrated node. Unlike
// CheckConverged, it does not require full desired coverage, which would
// deadlock step-by-step migration while legacy pods still occupy most nodes.
func (s *WorkloadService) CheckProgressReady(
	ctx context.Context,
	w DaemonWorkload,
	minReadyPods int32,
) (*SyncCheckResult, error) {
	realReadyPods, err := s.CountReadyPods(ctx, w)
	if err != nil {
		return nil, err
	}

	if minReadyPods <= 0 {
		minReadyPods = 1
	}

	res := &SyncCheckResult{
		Workload:      w.NamespacedName(),
		Kind:          w.Kind(),
		Ready:         true,
		RealReadyPods: realReadyPods,
		MinReadyPods:  minReadyPods,
	}

	if w.GetObservedGeneration() != w.GetGeneration() {
		res.AddReason(fmt.Sprintf(
			"observedGeneration(%d) != generation(%d)",
			w.GetObservedGeneration(),
			w.GetGeneration(),
		))
	}

	if w.GetNumberReady() < minReadyPods {
		res.AddReason(fmt.Sprintf(
			"numberReady(%d) < minReadyPods(%d)",
			w.GetNumberReady(),
			minReadyPods,
		))
	}

	if realReadyPods < minReadyPods {
		res.AddReason(fmt.Sprintf(
			"realReadyPods(%d) < minReadyPods(%d)",
			realReadyPods,
			minReadyPods,
		))
	}

	return res, nil
}

func evaluateConvergence(res *SyncCheckResult, w DaemonWorkload) {
	if w.GetObservedGeneration() != w.GetGeneration() {
		res.AddReason(fmt.Sprintf(
			"observedGeneration(%d) != generation(%d)",
			w.GetObservedGeneration(),
			w.GetGeneration(),
		))
	}

	if w.GetUpdatedNumberScheduled() != w.GetDesiredNumberScheduled() {
		res.AddReason(fmt.Sprintf(
			"updatedNumberScheduled(%d) != desiredNumberScheduled(%d)",
			w.GetUpdatedNumberScheduled(),
			w.GetDesiredNumberScheduled(),
		))
	}

	if w.GetCurrentNumberScheduled() != w.GetDesiredNumberScheduled() {
		res.AddReason(fmt.Sprintf(
			"currentNumberScheduled(%d) != desiredNumberScheduled(%d)",
			w.GetCurrentNumberScheduled(),
			w.GetDesiredNumberScheduled(),
		))
	}

	if w.GetNumberUnavailable() != 0 {
		res.AddReason(fmt.Sprintf(
			"numberUnavailable(%d) != 0",
			w.GetNumberUnavailable(),
		))
	}

	if w.GetNumberAvailable() < w.GetDesiredNumberScheduled() {
		res.AddReason(fmt.Sprintf(
			"numberAvailable(%d) < desiredNumberScheduled(%d)",
			w.GetNumberAvailable(),
			w.GetDesiredNumberScheduled(),
		))
	}
}

func normalizeMinReadyPods(w DaemonWorkload, minReadyPods int32) int32 {
	if minReadyPods > 0 {
		return minReadyPods
	}

	return w.GetDesiredNumberScheduled()
}

func (s *WorkloadService) CountReadyPods(
	ctx context.Context,
	w DaemonWorkload,
) (int32, error) {
	if w.GetPodSelector() == nil {
		return 0, fmt.Errorf("workload %s has nil pod selector", w.NamespacedName())
	}

	selector, err := metav1.LabelSelectorAsSelector(w.GetPodSelector())
	if err != nil {
		return 0, err
	}

	var podList corev1.PodList
	if err := s.client.List(
		ctx,
		&podList,
		ctrlclient.InNamespace(w.GetNamespace()),
		ctrlclient.MatchingLabelsSelector{Selector: selector},
	); err != nil {
		return 0, err
	}

	var ready int32
	for i := range podList.Items {
		pod := &podList.Items[i]
		if pod.DeletionTimestamp != nil {
			continue
		}
		if isPodReady(pod) {
			ready++
		}
	}

	return ready, nil
}

func isPodReady(pod *corev1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}
