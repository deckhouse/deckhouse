// Copyright 2026 Flant JSC
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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const capiControllerManagerFinalizer = "deckhouse.io/capi-controller-manager"

type capiClusterFinalizerMeta struct {
	APIVersion string
	Kind       string
	Name       string
	Namespace  string
	Finalizers []string
}

func filterCapiClusterFinalizerMeta(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return capiClusterFinalizerMeta{
		APIVersion: obj.GetAPIVersion(),
		Kind:       obj.GetKind(),
		Name:       obj.GetName(),
		Namespace:  obj.GetNamespace(),
		Finalizers: obj.GetFinalizers(),
	}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager/create-capi-cluster-resources",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "capi_cluster",
			ApiVersion: "cluster.x-k8s.io/v1beta1",
			Kind:       "Cluster",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{MatchNames: []string{capiNamespace}},
			},
			FilterFunc: filterCapiClusterFinalizerMeta,
		},
	},
}, migrateCapiClusterFinalizer)

func migrateCapiClusterFinalizer(_ context.Context, input *go_hook.HookInput) error {
	clusters, err := sdkobjectpatch.UnmarshalToStruct[capiClusterFinalizerMeta](input.Snapshots, "capi_cluster")
	if err != nil {
		return fmt.Errorf("unmarshal capi_cluster snapshot: %w", err)
	}

	for _, cluster := range clusters {
		if !hasString(cluster.Finalizers, capiControllerManagerFinalizer) {
			continue
		}

		patch := map[string]interface{}{
			"metadata": map[string]interface{}{
				"finalizers": removeString(cluster.Finalizers, capiControllerManagerFinalizer),
			},
		}
		input.PatchCollector.PatchWithMerge(patch, cluster.APIVersion, cluster.Kind, cluster.Namespace, cluster.Name)
	}

	return nil
}

func hasString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func removeString(values []string, target string) []string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if value != target {
			filtered = append(filtered, value)
		}
	}
	return filtered
}
