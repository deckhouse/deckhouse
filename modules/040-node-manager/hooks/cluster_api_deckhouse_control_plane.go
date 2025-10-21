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
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

var _ = sdk.RegisterFunc(
	&go_hook.HookConfig{
		Queue: "/modules/node-manager/cluster-api",
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:       "control_plane",
				ApiVersion: "infrastructure.cluster.x-k8s.io/v1alpha1",
				Kind:       "DeckhouseControlPlane",
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{
						MatchNames: []string{clusterAPINamespace},
					},
				},
				FilterFunc: filterControlPlane,
			},
		},
	},
	updateControlPlane,
)

type controlPlane struct {
	APIVersion string
	Kind       string
	Name       string
	Namespace  string
}

func filterControlPlane(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return controlPlane{
		APIVersion: obj.GetAPIVersion(),
		Kind:       obj.GetKind(),
		Name:       obj.GetName(),
		Namespace:  obj.GetNamespace(),
	}, nil
}

func updateControlPlane(_ context.Context, input *go_hook.HookInput) error {
	statusPatch := map[string]interface{}{
		"status": map[string]interface{}{
			"initialized":                 true,
			"ready":                       true,
			"externalManagedControlPlane": true,
		},
	}
	for controlPlane, err := range sdkobjectpatch.SnapshotIter[controlPlane](input.Snapshots.Get("control_plane")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'control_plane' classes: %w", err)
		}

		// patch status
		input.PatchCollector.PatchWithMerge(statusPatch, controlPlane.APIVersion, controlPlane.Kind, controlPlane.Namespace, controlPlane.Name, object_patch.WithIgnoreMissingObject(), object_patch.WithSubresource("/status"))
	}

	return nil
}
