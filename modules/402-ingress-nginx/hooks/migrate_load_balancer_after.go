/*
Copyright 2021 Flant CJSC

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

package hooks

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
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
			FilterFunc: ApplyDaemonSetControllerFilter,
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

type DeploymentController struct {
	Name   string                  `json:"name"`
	Status appsv1.DeploymentStatus `json:"status"`
}

func applyDeploymentControllerFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	d := &appsv1.Deployment{}

	err := sdk.FromUnstructured(obj, d)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	return DeploymentController{
		Name:   d.Labels["name"],
		Status: d.Status,
	}, nil
}

func migrateControllerAfterHelm(input *go_hook.HookInput) (err error) {
	daemonsets := input.Snapshots["daemonset"]
	deployments := input.Snapshots["deployment"]

	for _, ds := range daemonsets {
		if ds == nil {
			continue
		}
		daemonset := ds.(DaemonSetController)

		var deploymentReady bool
		for _, d := range deployments {
			if d == nil {
				continue
			}
			deployment := d.(DeploymentController)
			if daemonset.Name == deployment.Name {
				if deployment.Status.Replicas == deployment.Status.ReadyReplicas {
					deploymentReady = true
					break
				}
			}
		}

		if !deploymentReady {
			input.LogEntry.Infof("Deployment is not yet ready, skipping controller %s", daemonset.Name)
			continue
		}

		err := input.ObjectPatcher.DeleteObject("apps/v1", "DaemonSet", namespace, "controller-"+daemonset.Name, "")
		if err != nil {
			return err
		}
	}

	return nil
}
