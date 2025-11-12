// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const (
	isBootstrappedCmSnapName    = "is_bootstraped_cm"
	readyNotMasterNodesSnapName = "ready_not_master_nodes"
)

const clusterBootstrapFlagPath = "global.clusterIsBootstrapped"

// bootstraped contains typo
// for fix it need do migration
// keep it, but move into const for fix in future
const clusterBootstrappedConfigMap = "d8-cluster-is-bootstraped"

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       isBootstrappedCmSnapName,
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NameSelector: &types.NameSelector{
				MatchNames: []string{clusterBootstrappedConfigMap},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			FilterFunc: applyClusterBootstrapCmFilter,
		},
		{
			Name:       readyNotMasterNodesSnapName,
			ApiVersion: "v1",
			Kind:       "Node",
			FilterFunc: applyReadyNotMasterNodeFilter,
		},
	},
}, clusterIsBootstrapped)

func applyClusterBootstrapCmFilter(_ *unstructured.Unstructured) (go_hook.FilterResult, error) {
	// we only need to check the configmap existence
	return true, nil
}

func applyReadyNotMasterNodeFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node v1core.Node
	err := sdk.FromUnstructured(obj, &node)
	if err != nil {
		return false, fmt.Errorf("from unstructured: %w", err)
	}

	for _, taint := range node.Spec.Taints {
		if taint.Key == "node-role.kubernetes.io/control-plane" || taint.Key == "node-role.kubernetes.io/master" {
			// it is master node
			return false, nil
		}
	}

	// not master - check node is ready
	for _, c := range node.Status.Conditions {
		if c.Type != v1core.NodeReady {
			continue
		}

		isReady := c.Status == v1core.ConditionTrue
		return isReady, nil
	}

	return false, nil
}

func createBootstrapClusterCm(patchCollector go_hook.PatchCollector) {
	cm := &v1core.ConfigMap{
		TypeMeta: v1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      clusterBootstrappedConfigMap,
			Namespace: "kube-system",
		},
	}

	patchCollector.CreateIfNotExists(cm)
}

func clusterIsBootstrapped(_ context.Context, input *go_hook.HookInput) error {
	isBootstrappedCmSnap := input.Snapshots.Get(isBootstrappedCmSnapName)

	if len(isBootstrappedCmSnap) > 0 {
		// if we have cm here then set value and return
		// configmap is source of truth
		input.Values.Set(clusterBootstrapFlagPath, true)
		return nil
	}
	// not have `is bootstrap` configmap
	if input.Values.Exists(clusterBootstrapFlagPath) {
		// here cm was deleted probably
		// create it!
		createBootstrapClusterCm(input.PatchCollector)
		return nil
	}

	readyNodes, err := sdkobjectpatch.UnmarshalToStruct[bool](input.Snapshots, readyNotMasterNodesSnapName)
	if err != nil {
		return fmt.Errorf("failed to unmarshal %s snapshot: %w", readyNotMasterNodesSnapName, err)
	}

	for _, ready := range readyNodes {
		if ready {
			createBootstrapClusterCm(input.PatchCollector)
			input.Values.Set(clusterBootstrapFlagPath, true)
			break
		}
	}

	return nil
}
