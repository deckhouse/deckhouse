/*
Copyright 2024 Flant JSC

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
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/set"
)

const (
	nodesSnapshot            = "nodes"
	fencingControllerTimeout = time.Duration(60) * time.Second
)

var maintenanceAnnotations = []string{
	`update.node.deckhouse.io/disruption-approved`,
	`update.node.deckhouse.io/approved`,
	`node-manager.deckhouse.io/fencing-disable`,
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager/fencing",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       nodesSnapshot,
			ApiVersion: "v1",
			Kind:       "Node",
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "node-manager.deckhouse.io/fencing-enabled",
						Operator: metav1.LabelSelectorOpExists,
					},
				},
			},
			FilterFunc: fencingControllerNodeFilter,
		},
	},
}, dependency.WithExternalDependencies(fencingControllerHandler))

type fencingControllerNodeResult struct {
	Name          string
	NodeGroupName string
}

type fencingControllerLeaseResult struct {
	NodeName  string
	RenewTime time.Time
}

func fencingControllerNodeFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var res fencingControllerNodeResult

	for _, annotation := range maintenanceAnnotations {
		_, annotationExists := obj.GetAnnotations()[annotation]
		if annotationExists {
			return nil, nil
		}
	}

	res.Name = obj.GetName()
	return res, nil
}

func fencingControllerHandler(input *go_hook.HookInput, dc dependency.Container) error {
	if len(input.Snapshots[nodesSnapshot]) == 0 {
		// No nodes with enabled fencing -> nothing to do
		return nil
	}

	// kubeclient to get node leases and get and delete pods
	kubeClient, err := dc.GetK8sClient()
	if err != nil {
		input.LogEntry.Errorf("%v", err)
		return err
	}

	// make map with nodes to kill
	nodesToKill := set.New()
	for _, nodeRaw := range input.Snapshots[nodesSnapshot] {

		if nodeRaw == nil {
			continue
		}

		node := nodeRaw.(fencingControllerNodeResult)
		nodeLease, err := kubeClient.CoordinationV1().Leases("kube-node-lease").Get(context.TODO(), node.Name, metav1.GetOptions{})
		if err != nil {
			input.LogEntry.Errorf("Can't get node lease: %v", err)
			continue
		}

		if time.Since(nodeLease.Spec.RenewTime.Time) > fencingControllerTimeout {
			nodesToKill.Add(node.Name)
		}
	}

	nodeToKillCount := nodesToKill.Size()
	if nodeToKillCount == 0 {
		// nothing to kill -> skip
		return nil
	}

	input.LogEntry.Warnf("Going to kill %d nodes", nodeToKillCount)

	// kill nodes
	for _, node := range nodesToKill.Slice() {
		input.LogEntry.Warnf("Delete all pods from node %s", node)
		podsToDelete, err := kubeClient.CoreV1().Pods("").List(
			context.TODO(),
			metav1.ListOptions{
				FieldSelector: fields.SelectorFromSet(fields.Set{"spec.nodeName": node}).String(),
			},
		)
		if err != nil {
			input.LogEntry.Errorf("Can't list pods: %v", err)
			continue
		}

		GracePeriodSeconds := int64(0)
		for _, pod := range podsToDelete.Items {
			input.LogEntry.Warnf("Delete pod %s in namespace %s on node %s", pod.Name, pod.Namespace, pod.Spec.NodeName)
			err = kubeClient.CoreV1().Pods(pod.Namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{
				GracePeriodSeconds: &GracePeriodSeconds,
			})
			if err != nil {
				input.LogEntry.Errorf("Can't delete pod %s: %v", pod.Name, err)
			}
		}

		input.LogEntry.Warnf("Delete node %s", node)
		input.PatchCollector.Delete("v1", "Node", "", node, object_patch.InBackground())
	}

	return nil
}
