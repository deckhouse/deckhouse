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
	"context"
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const (
	nodeControllerNamespace = "d8-cloud-instance-manager"
	nodeControllerApp       = "node-controller"
	nodeControllerLeaseName = "node-controller.deckhouse.io"
)

type nodeControllerLeaseInfo struct {
	HolderIdentity string
}

type nodeControllerPodInfo struct {
	Name string
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager/set_node_controller_leader_pod",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "lease",
			ApiVersion: "coordination.k8s.io/v1",
			Kind:       "Lease",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{nodeControllerNamespace},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{nodeControllerLeaseName},
			},
			FilterFunc: nodeControllerLeaseFilter,
		},
		{
			Name:       "pods",
			ApiVersion: "v1",
			Kind:       "Pod",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{nodeControllerNamespace},
				},
			},
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": nodeControllerApp,
				},
			},
			FilterFunc: nodeControllerPodFilter,
		},
	},
}, setNodeControllerLeaderPod)

func setNodeControllerLeaderPod(_ context.Context, input *go_hook.HookInput) error {
	var leaderPodName string

	for lease, err := range sdkobjectpatch.SnapshotIter[nodeControllerLeaseInfo](input.Snapshots.Get("lease")) {
		if err != nil {
			return fmt.Errorf("cannot iterate over 'lease' snapshot: %w", err)
		}

		leaderPodName = podNameFromHolderIdentity(lease.HolderIdentity)
		break
	}

	for pod, err := range sdkobjectpatch.SnapshotIter[nodeControllerPodInfo](input.Snapshots.Get("pods")) {
		if err != nil {
			return fmt.Errorf("cannot iterate over 'pods' snapshot: %w", err)
		}

		patch := map[string]any{
			"metadata": map[string]any{
				"labels": map[string]any{
					"leader": trueOrNil(pod.Name == leaderPodName && leaderPodName != ""),
				},
			},
		}

		input.PatchCollector.PatchWithMerge(patch, "v1", "Pod", nodeControllerNamespace, pod.Name)
	}

	return nil
}

func nodeControllerLeaseFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var lease coordinationv1.Lease

	if err := sdk.FromUnstructured(obj, &lease); err != nil {
		return nil, err
	}

	if lease.Spec.HolderIdentity == nil {
		return nodeControllerLeaseInfo{}, nil
	}

	return nodeControllerLeaseInfo{
		HolderIdentity: *lease.Spec.HolderIdentity,
	}, nil
}

func nodeControllerPodFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var pod corev1.Pod

	if err := sdk.FromUnstructured(obj, &pod); err != nil {
		return nil, err
	}

	return nodeControllerPodInfo{
		Name: pod.Name,
	}, nil
}

func podNameFromHolderIdentity(holderIdentity string) string {
	podName, _, _ := strings.Cut(holderIdentity, "_")
	return podName
}

func trueOrNil(b bool) any {
	if b {
		return "true"
	}
	return nil
}
