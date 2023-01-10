/*
Copyright 2022 Flant JSC

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
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	deschedulerSpecsValuesPath = "descheduler.internal.deschedulers"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
	Queue:        "/modules/descheduler",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "deschedulers",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "Descheduler",
			FilterFunc: applyDeschedulerFilter,
		},
		{
			Name:              "deployments",
			ApiVersion:        "apps/v1",
			Kind:              "Deployments",
			FilterFunc:        deschedulerDeploymentReadiness,
			LabelSelector:     &metav1.LabelSelector{MatchLabels: map[string]string{"app": "descheduler"}},
			NamespaceSelector: &types.NamespaceSelector{NameSelector: &types.NameSelector{MatchNames: []string{"d8-descheduler"}}},
		},
	},
}, generateValues)

type DeschedulerDeploymentInfo struct {
	Name  string
	Ready bool
}

func applyDeschedulerFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	unstructured.RemoveNestedField(obj.UnstructuredContent(), "status")

	return obj.UnstructuredContent(), nil
}

func deschedulerDeploymentReadiness(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	deployment := &v1.Deployment{}
	err := sdk.FromUnstructured(obj, deployment)
	if err != nil {
		return nil, err
	}

	name := strings.TrimPrefix(deployment.Name, "descheduler-")

	deschedulerDeploymentInfo := &DeschedulerDeploymentInfo{
		Name:  name,
		Ready: deployment.Status.ReadyReplicas == deployment.Status.Replicas,
	}

	return deschedulerDeploymentInfo, nil
}

func generateValues(input *go_hook.HookInput) error {
	var (
		deschedulers = input.Snapshots["deschedulers"]
		deployments  = input.Snapshots["deployments"]
	)

	if len(deschedulers) == 0 {
		return nil
	}

	input.Values.Set(deschedulerSpecsValuesPath, deschedulers)

	for _, deploymentRaw := range deployments {
		deployment := deploymentRaw.(*DeschedulerDeploymentInfo)

		input.PatchCollector.MergePatch(map[string]map[string]bool{
			"status": {"ready": deployment.Ready}},
			"deckhouse.io/v1alpha1", "Descheduler", "",
			deployment.Name, object_patch.WithSubresource("status"))
	}

	return nil
}
