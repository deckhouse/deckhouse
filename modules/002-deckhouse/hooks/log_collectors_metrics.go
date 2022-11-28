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

package hooks

import (
	"context"
	"fmt"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

type Node struct {
	Name    string
	CRIType string
}

type Daemonset struct {
	Name string
	Namespace string
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/ceph-csi",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "nodes",
			ApiVersion: "v1",
			Kind:       "Node",
			FilterFunc: applyNodesFilter,
		},
		{
			Name:       "daemonsets",
			ApiVersion: "apps/v1",
			Kind:       "DaemonSet",
			FilterFunc: applyDaemonSetsFilter,
		},
	},
}, dependency.WithExternalDependencies(setMetrics))

func applyNodesFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node = &v1.Node{}
	err := sdk.FromUnstructured(obj, node)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	return Node{
		Name: node.Name,
		CRIType: node.Status.NodeInfo.ContainerRuntimeVersion,
	}, nil
}

func applyDaemonSetsFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var daemonset = &appsv1.DaemonSet{}
	err := sdk.FromUnstructured(obj, daemonset)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	if ! strings.HasPrefix(daemonset.Namespace, "d8-") &&
		 strings.Contains(daemonset.Namespace, "fluent") {
		return Daemonset{
			Name: daemonset.Name,
			Namespace: daemonset.Namespace,
		}, nil
	}

	return nil, nil
}


func setMetrics(input *go_hook.HookInput, dc dependency.Container) error {
	daemonsets := input.Snapshots["daemonsets"]
	nodes := input.Snapshots["nodes"]

	kubeClient, err := dc.GetK8sClient()
	if err != nil {
		return err
	}

	for _, obj := range daemonsets {
		ds := obj.(Daemonset)

		pods, err := kubeClient.CoreV1().Pods(ds.Namespace).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return err
		}

		migrationRequired := 0.0
		if isCRIDocker(pods.Items, nodes) {
			migrationRequired = 1.0
		}

		input.MetricsCollector.Set(
			"migration_to_log_shipper_required",
			migrationRequired,
			map[string]string{
				"daemonset": ds.Name,
				"namespace": ds.Namespace,
			},
			metrics.WithGroup("custom_log_collector"),
		)
	}


	return nil
}

func isCRIDocker(pods []v1.Pod, nodes []go_hook.FilterResult) bool {
	for _, p := range pods {
		for _, n := range nodes {
			node := n.(Node)
			if node.Name == p.Spec.NodeName && strings.Contains(node.CRIType, "docker") {
				return true
			}
		}

	}
	return false
}
