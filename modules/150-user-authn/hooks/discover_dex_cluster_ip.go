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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func applyDexServiceFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	service := &v1.Service{}
	err := sdk.FromUnstructured(obj, service)
	if err != nil {
		return nil, fmt.Errorf("cannot convert to service: %v", err)
	}

	return service.Spec.ClusterIP, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "service",
			ApiVersion: "v1",
			Kind:       "Service",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-user-authn"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"dex"},
			},
			FilterFunc: applyDexServiceFilter,
		},
	},
}, discoverDexClusterIP)

func discoverDexClusterIP(_ context.Context, input *go_hook.HookInput) error {
	const dexClusterIPPath = "userAuthn.internal.discoveredDexClusterIP"

	services := input.Snapshots.Get("service")
	if len(services) == 0 {
		input.Logger.Debug("no dex services found in cluster")
		return nil
	}
	var clusterIP string
	err := services[0].UnmarshalTo(&clusterIP)
	if err != nil {
		return fmt.Errorf("failed to unmarshal dex service clusterIP from start snapshot: %w", err)
	}

	if clusterIP == v1.ClusterIPNone {
		// Migration, delete after rolling it on all clusters
		input.PatchCollector.Delete("v1", "Service", "d8-user-authn", "dex")
		return nil
	}

	input.Values.Set(dexClusterIPPath, clusterIP)
	return nil
}
