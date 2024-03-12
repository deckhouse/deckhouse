/*
Copyright 2023 Flant JSC

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

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
)

const (
	nodesSnapshot            = "nodes"
	leasesSnapshot           = "leases"
	podsSnapshot             = "pods"
	fencingControllerTimeout = time.Duration(60) * time.Second
)

var maintanenceAnnotations = []string{
	`update.node.deckhouse.io/disruption-approved`,
	`update.node.deckhouse.io/approved`,
	`node-manager.deckhouse.io/fencing-disable`,
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager/fecning",
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
		{
			Name:       leasesSnapshot,
			ApiVersion: "coordination.k8s.io/v1",
			Kind:       "Lease",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-node-lease"},
				},
			},
			FilterFunc: fencingControllerLeaseFilter,
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

	for _, annotation := range maintanenceAnnotations {
		_, annotationExists := obj.GetAnnotations()[annotation]
		if annotationExists {
			return nil, nil
		}
	}

	res.Name = obj.GetName()
	return res, nil
}

func fencingControllerLeaseFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var lease coordinationv1.Lease
	err := sdk.FromUnstructured(obj, &lease)
	if err != nil {
		return nil, err
	}
	return fencingControllerLeaseResult{
		NodeName:  *lease.Spec.HolderIdentity,
		RenewTime: lease.Spec.RenewTime.Time,
	}, nil
}

func fencingControllerHandler(input *go_hook.HookInput, dc dependency.Container) error {
	if len(input.Snapshots[nodesSnapshot]) == 0 {
		// No nodes with enabled fencing -> nothing to do
		return nil
	}

	// make map with nodes
	nodesMap := make(map[string]struct{})
	for _, nodeRaw := range input.Snapshots[nodesSnapshot] {
		if nodeRaw != nil {
			node := nodeRaw.(fencingControllerNodeResult)
			nodesMap[node.Name] = struct{}{}
		}
	}

	// make map with nodes to kill
	nodesToKill := make(map[string]struct{})
	for _, nodeLeaseRaw := range input.Snapshots[leasesSnapshot] {
		nodeLease := nodeLeaseRaw.(fencingControllerLeaseResult)

		if _, ok := nodesMap[nodeLease.NodeName]; !ok {
			continue
		}
		if time.Since(nodeLease.RenewTime) > fencingControllerTimeout {
			nodesToKill[nodeLease.NodeName] = struct{}{}
		}
	}

	nodeToKillCount := len(nodesToKill)
	if nodeToKillCount == 0 {
		// nothing to kill -> skip
		return nil
	}

	input.LogEntry.Warnf("Going to kill %d nodes", nodeToKillCount)

	// create k8s client to delete pods
	kubeClient, err := dc.GetK8sClient()
	if err != nil {
		input.LogEntry.Errorf("%v", err)
		return err
	}

	// kill nodes
	for node := range nodesToKill {
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

		for _, pod := range podsToDelete.Items {
			input.LogEntry.Warnf("Delete pod %s in namespace %s on node %s", pod.Name, pod.Namespace, pod.Spec.NodeName)
			err = kubeClient.CoreV1().Pods(pod.Namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{
				GracePeriodSeconds: &Int64(0),
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
