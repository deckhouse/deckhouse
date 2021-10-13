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
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/go_lib/set"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager/node_lease_handler",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "node_leases",
			ApiVersion:                   "coordination.k8s.io/v1",
			Kind:                         "Lease",
			ExecuteHookOnSynchronization: pointer.BoolPtr(false),
			FilterFunc:                   nameFilter,
		},
		{
			Name:                         "nodes",
			ApiVersion:                   "v1",
			Kind:                         "Node",
			ExecuteHookOnEvents:          pointer.BoolPtr(false),
			ExecuteHookOnSynchronization: pointer.BoolPtr(false),
			FilterFunc:                   nameFilter,
		},
	},
}, handleNodeLease)

func handleNodeLease(input *go_hook.HookInput) error {
	var (
		leases = set.NewFromSnapshot(input.Snapshots["node_leases"])
		nodes  = set.NewFromSnapshot(input.Snapshots["nodes"])
	)

	for nodeName := range nodes {
		if leases.Has(nodeName) {
			// Lease and Node exist. We are interested in deleted Leases only
			continue
		}

		input.PatchCollector.Filter(leaseNodeFilterFunc, "v1", "Node", "", nodeName, object_patch.WithSubresource("status"))
	}

	return nil
}

func leaseNodeFilterFunc(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	var node *corev1.Node

	err := sdk.FromUnstructured(obj, &node)
	if err != nil {
		return nil, err
	}

	for i, cond := range node.Status.Conditions {
		if cond.Type != corev1.NodeReady {
			continue
		}

		ts := metav1.NewTime(time.Now())
		newCondition := corev1.NodeCondition{
			Type:               corev1.NodeReady,
			Status:             corev1.ConditionFalse,
			LastHeartbeatTime:  ts,
			LastTransitionTime: ts,
			Reason:             "KubeletReady",
			Message:            "Status NotReady was set by node_lease_handler hook of node-manager Deckhouse module during bashible reboot step (candi/bashible/common-steps/all/099_reboot.sh)",
		}
		node.Status.Conditions[i] = newCondition
		break
	}

	return sdk.ToUnstructured(node)
}
