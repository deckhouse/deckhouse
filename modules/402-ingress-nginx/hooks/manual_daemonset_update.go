/*
Copyright 2021 Flant JSC

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

// This is temporary hook, because daemonset controller sometime makes some trash with deleting pods
// It could be removed, when all clusters will be upgraded to 1.21 version

package hooks

import (
	"errors"
	"strconv"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:       "/modules/ingress-nginx/manual_daemonset_update",
	OnAfterHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "controllers",
			ApiVersion:                   "apps/v1",
			Kind:                         "DaemonSet",
			ExecuteHookOnSynchronization: pointer.BoolPtr(false),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-ingress-nginx"},
				},
			},
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"ingress-nginx-manual-update": "true",
					"app":                         "controller",
				},
			},
			FilterFunc: filterManualDS,
		},
		{
			Name:                         "revisions",
			ApiVersion:                   "apps/v1",
			Kind:                         "ControllerRevision",
			ExecuteHookOnSynchronization: pointer.BoolPtr(false),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-ingress-nginx"},
				},
			},
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "controller",
				},
			},
			FilterFunc: filterControllerRevision,
		},
		{
			Name:                         "pods",
			ApiVersion:                   "v1",
			Kind:                         "Pod",
			ExecuteHookOnSynchronization: pointer.BoolPtr(false),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-ingress-nginx"},
				},
			},
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "controller",
				},
			},
			FilterFunc: filterManualPod,
		},
	},
}, manualControllerUpdate)

type manualControllerRevision struct {
	CRName   string
	Revision int64
}
type manualDSController struct {
	CRName string

	DesiredPodCount int32
	CurrentPodCount int32
}

type manualRolloutPod struct {
	Name       string
	Generation int64

	CRName string

	Ready bool
}

func manualControllerUpdate(input *go_hook.HookInput) error {
	var controllers []manualDSController
	snap := input.Snapshots["controllers"]
	if len(snap) == 0 {
		return nil
	}
	for _, sn := range snap {
		controller := sn.(manualDSController)
		controllers = append(controllers, controller)
	}

	revisionMap := make(map[string]int64, len(controllers))
	snap = input.Snapshots["revisions"]
	for _, srev := range snap {
		rev := srev.(manualControllerRevision)
		if prev, ok := revisionMap[rev.CRName]; ok {
			if rev.Revision > prev {
				revisionMap[rev.CRName] = rev.Revision
			}
		} else {
			revisionMap[rev.CRName] = rev.Revision
		}
	}

	// by ds controller name
	podsMap := make(map[string][]manualRolloutPod)
	snap = input.Snapshots["pods"]
	for _, sn := range snap {
		pod := sn.(manualRolloutPod)
		podsMap[pod.CRName] = append(podsMap[pod.CRName], pod)
	}

	for _, controller := range controllers {
		// check pod count to avoid race during creation a new pod
		if controller.CurrentPodCount != controller.DesiredPodCount {
			continue
		}

		podsReadyForUpdate := true

		var podNameForDeletion string
		for _, pod := range podsMap[controller.CRName] {
			if !pod.Ready {
				podsReadyForUpdate = false
				break
			}
			dsRevision, ok := revisionMap[controller.CRName]
			if !ok {
				podsReadyForUpdate = false
				break
			}

			if pod.Generation != dsRevision {
				podNameForDeletion = pod.Name
			}
		}

		if podsReadyForUpdate && podNameForDeletion != "" {
			input.PatchCollector.Delete("v1", "Pod", "d8-ingress-nginx", podNameForDeletion, object_patch.InBackground())
		}
	}

	return nil
}

func filterManualDS(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var ds appsv1.DaemonSet

	err := sdk.FromUnstructured(obj, &ds)
	if err != nil {
		return nil, err
	}

	return manualDSController{
		CRName:          ds.GetLabels()["name"],
		DesiredPodCount: ds.Status.DesiredNumberScheduled,
		CurrentPodCount: ds.Status.CurrentNumberScheduled,
	}, nil
}

func filterControllerRevision(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var cr appsv1.ControllerRevision

	err := sdk.FromUnstructured(obj, &cr)
	if err != nil {
		return nil, err
	}

	return manualControllerRevision{
		CRName:   cr.GetLabels()["name"],
		Revision: cr.Revision,
	}, nil
}

func filterManualPod(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var pod v1.Pod

	err := sdk.FromUnstructured(obj, &pod)
	if err != nil {
		return nil, err
	}

	genLabel := pod.Labels["pod-template-generation"]
	if len(genLabel) == 0 {
		return nil, errors.New("pod-template-generation label missed")
	}
	gen, err := strconv.ParseInt(genLabel, 10, 64)
	if err != nil {
		return nil, err
	}

	var podReady bool
	for _, cond := range pod.Status.Conditions {
		if cond.Type == v1.PodReady && cond.Status == v1.ConditionTrue {
			podReady = true
			break
		}
	}

	return manualRolloutPod{
		Name:       pod.Name,
		Generation: gen,
		CRName:     pod.Labels["name"],
		Ready:      podReady,
	}, nil
}
