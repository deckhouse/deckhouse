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
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const (
	defaultGatewayConfigmapName      = "default-gateway"
	defaultGatewayConfigmapNamespace = "d8-alb-istio"
	configmapField                   = "defaultGateway"
	configmapSnapshot                = "configmap"
	discoveryDefaultGatewayPath      = "global.discovery.gatewayAPIDefaultGateway"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       configmapSnapshot,
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NameSelector: &types.NameSelector{
				MatchNames: []string{defaultGatewayConfigmapName},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{defaultGatewayConfigmapNamespace},
				},
			},
			FilterFunc: applyCmFilter,
		},
	},
}, setDiscoveredDefaultGateway)

type GatewayDesc struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

func applyCmFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var cm v1core.ConfigMap
	err := sdk.FromUnstructured(obj, &cm)
	if err != nil {
		return nil, fmt.Errorf("from unstructured: %w", err)
	}

	defaultGWstring, ok := cm.Data[configmapField]
	if !ok {
		return &GatewayDesc{}, nil
	}

	defaultGWslice := strings.Split(defaultGWstring, "/")
	if len(defaultGWslice) != 2 || len(defaultGWslice[0]) == 0 || len(defaultGWslice[1]) == 0 {
		return &GatewayDesc{}, nil
	}

	return &GatewayDesc{
		Namespace: defaultGWslice[0],
		Name:      defaultGWslice[1],
	}, nil
}

func setDiscoveredDefaultGateway(_ context.Context, input *go_hook.HookInput) error {
	if len(input.Snapshots.Get(configmapSnapshot)) == 1 {
		defaultGW, err := sdkobjectpatch.UnmarshalToStruct[GatewayDesc](input.Snapshots, configmapSnapshot)
		if err != nil {
			return fmt.Errorf("failed to unmarshal %s snapshot: %w", configmapSnapshot, err)
		}

		if len(defaultGW[0].Name) == 0 || len(defaultGW[0].Namespace) == 0 {
			input.Logger.Warn("could not detect default gateway")
		}

		input.Values.Set(discoveryDefaultGatewayPath, defaultGW[0])
	}

	return nil
}
