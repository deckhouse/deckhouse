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

// Migration from deployment to daemonset(25.08.2021
// can be deleted after getting to 'rock-solid' (~01.12.2021)

package hooks

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterHelm: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "daemonset",
			ApiVersion:                   "apps/v1",
			Kind:                         "DaemonSet",
			ExecuteHookOnSynchronization: pointer.BoolPtr(false),
			ExecuteHookOnEvents:          pointer.BoolPtr(false),
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
			FilterFunc: applyDaemonSetControllerFilter,
		},
		{
			Name:                         "deployment",
			ApiVersion:                   "apps/v1",
			Kind:                         "Deployment",
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
			FilterFunc: applyDeploymentControllerFilter,
		},
	},
}, migrateControllerAfterHelm)

type daemonSetController struct {
	CRDName string                 `json:"crd_name"`
	Status  appsv1.DaemonSetStatus `json:"status"`
}

func applyDaemonSetControllerFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	d := &appsv1.DaemonSet{}

	err := sdk.FromUnstructured(obj, d)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	return daemonSetController{
		CRDName: d.Labels["name"],
		Status:  d.Status,
	}, nil
}

func migrateControllerAfterHelm(input *go_hook.HookInput) (err error) {
	daemonsets := input.Snapshots["daemonset"]
	deployments := input.Snapshots["deployment"]

	daemonsetReadinessMap := make(map[string]bool)
	for _, ds := range daemonsets {
		if ds == nil {
			continue
		}
		var daemonsetReady bool
		daemonset := ds.(daemonSetController)
		if daemonset.Status.NumberReady == daemonset.Status.DesiredNumberScheduled {
			daemonsetReady = true
		}
		daemonsetReadinessMap[daemonset.CRDName] = daemonsetReady
	}

	for _, d := range deployments {
		if d == nil {
			continue
		}
		deployment := d.(deploymentController)

		daemonsetReady, ok := daemonsetReadinessMap[deployment.CRDName]
		if !ok {
			input.LogEntry.Infof("DaemonSet is not found, skipping controller %s", deployment.CRDName)
			continue
		}

		if !daemonsetReady {
			input.LogEntry.Infof("DaemonSet is not yet ready, skipping controller %s", deployment.CRDName)
			continue
		}

		input.PatchCollector.Delete("apps/v1", "Deployment", namespace, deployment.Name, object_patch.InBackground())
	}

	return nil
}
