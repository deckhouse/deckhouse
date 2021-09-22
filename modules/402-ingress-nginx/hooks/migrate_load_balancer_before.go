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
	"math"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "deployment",
			ApiVersion: "apps/v1",
			Kind:       "Deployment",
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
}, dependency.WithExternalDependencies(migrateControllerBeforeHelm))

type deploymentController struct {
	Name    string                  `json:"name"`
	CRDName string                  `json:"crd_name"`
	Status  appsv1.DeploymentStatus `json:"status"`
}

func applyDeploymentControllerFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	ds := &appsv1.Deployment{}

	err := sdk.FromUnstructured(obj, ds)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	if !strings.HasPrefix(ds.Annotations["ingress-nginx-controller.deckhouse.io/inlet"], "LoadBalancer") {
		return nil, nil
	}

	return deploymentController{
		Name:    ds.Name,
		CRDName: ds.Labels["name"],
		Status:  ds.Status,
	}, nil
}

func migrateControllerBeforeHelm(input *go_hook.HookInput, dc dependency.Container) (err error) {
	deployments := input.Snapshots["deployment"]

	for _, ds := range deployments {
		if ds == nil {
			continue
		}
		controller := ds.(deploymentController)

		minReplicas := int(math.Round(float64(controller.Status.Replicas) / 2))
		if minReplicas < 1 {
			minReplicas = 1
		}
		maxReplicas := minReplicas

		patch := map[string]interface{}{
			"spec": map[string]interface{}{
				"minReplicas": minReplicas,
				"maxReplicas": maxReplicas,
			},
		}

		input.PatchCollector.MergePatch(patch, "deckhouse.io/v1", "IngressNginxController", "", controller.CRDName)

		helmPatch := map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": map[string]interface{}{
					"meta.helm.sh/release-name":      nil,
					"meta.helm.sh/release-namespace": nil,
					"helm.sh/resource-policy":        "keep",
				},
				"labels": map[string]interface{}{
					"app.kubernetes.io/managed-by": nil,
				},
			},
		}

		input.PatchCollector.MergePatch(helmPatch, "apps/v1", "Deployment", "d8-ingress-nginx", controller.Name)
	}

	return nil
}
