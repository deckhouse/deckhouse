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
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/testing"
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
		{
			Name:       "nodes",
			ApiVersion: "v1",
			Kind:       "Node",
			FilterFunc: nodesFilter,
		},
	},
}, populateValues)

func nodesFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	node := &corev1.Node{}

	err := sdk.FromUnstructured(obj, node)
	if err != nil {
		return nil, err
	}

	return nodeInfo{
		Name:   node.Name,
		Labels: node.Labels,
	}, nil
}

type nodeInfo struct {
	Name   string
	Labels map[string]string
}

type DeschedulerDeploymentInfo struct {
	Name  string
	Ready bool
}

type Descheduler struct {
	Unstructured map[string]interface{}
	NodeSelector string
}

type deschedulerSpec struct {
	Spec struct {
		DeschedulerPolicy struct {
			GlobalParameters struct {
				NodeSelector string `json:"nodeSelector"`
			} `json:"globalParameters"`
		} `json:"deschedulerPolicy"`
	} `json:"spec"`
}

func applyDeschedulerFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	unstructuredContent := obj.UnstructuredContent()

	// Remove the status field to avoid unnecessary hook runs, because status is patched by the hook
	unstructured.RemoveNestedField(unstructuredContent, "status")

	var deschedulerSpec deschedulerSpec

	err := sdk.FromUnstructured(obj, &deschedulerSpec)
	if err != nil {
		return nil, err
	}

	return Descheduler{
		Unstructured: unstructuredContent,
		NodeSelector: deschedulerSpec.Spec.DeschedulerPolicy.GlobalParameters.NodeSelector,
	}, nil
}

func deschedulerDeploymentReadiness(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	deployment := &v1.Deployment{}
	err := sdk.FromUnstructured(obj, deployment)
	if err != nil {
		return nil, err
	}

	deschedulerName, ok := deployment.GetLabels()["name"]
	if !ok {
		return nil, fmt.Errorf("deployment %q does not have the \"name\" label", deployment.GetName())
	}

	deschedulerDeploymentInfo := &DeschedulerDeploymentInfo{
		Name:  deschedulerName,
		Ready: deployment.Status.ReadyReplicas == deployment.Status.Replicas,
	}

	return deschedulerDeploymentInfo, nil
}

func populateValues(input *go_hook.HookInput) error {
	var (
		deschedulersSnapshots = input.Snapshots["deschedulers"]
		deploymentsSnapshots  = input.Snapshots["deployments"]
		nodesSnapshots        = input.Snapshots["nodes"]
	)

	deschedulers := make([]interface{}, 0)

	for _, deschedulerSnapshot := range deschedulersSnapshots {
		descheduler := deschedulerSnapshot.(Descheduler)

		if matchesNodes(nodesSnapshots, descheduler.NodeSelector) {
			deschedulers = append(deschedulers, descheduler.Unstructured)
		}
	}

	input.Values.Set(deschedulerSpecsValuesPath, deschedulers)

	if len(deschedulers) == 0 {
		return nil
	}

	for _, deploymentSnapshot := range deploymentsSnapshots {
		deployment := deploymentSnapshot.(*DeschedulerDeploymentInfo)

		input.PatchCollector.MergePatch(map[string]map[string]bool{
			"status": {"ready": deployment.Ready}},
			"deckhouse.io/v1alpha1", "Descheduler", "",
			deployment.Name, object_patch.WithSubresource("status"))
	}

	return nil
}

func matchesNodes(nodesSnapshots []go_hook.FilterResult, nodeSelector string) bool {
	label, _, _ := testing.ExtractFromListOptions(metav1.ListOptions{LabelSelector: nodeSelector})
	if label == nil {
		label = labels.Everything()
	}

	count := 0

	for _, nodeSnapshot := range nodesSnapshots {
		nodeInfo := nodeSnapshot.(nodeInfo)

		if label.Matches(labels.Set(nodeInfo.Labels)) {
			count++
		}
	}

	return count > 1
}
