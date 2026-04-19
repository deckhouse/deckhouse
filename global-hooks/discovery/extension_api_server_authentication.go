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

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/filter"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "extension_api_server_authentication",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"extension-apiserver-authentication"},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			FilterFunc: filter.KeyFromConfigMap("requestheader-client-ca-file"),
		},
	},
}, discoveryExtentsionAPIServerCA)

// discoveryExtentsionAPIServerCA
// here is CM kube-system/extension-apiserver-authentication with CA
// for verification requests to our custom modules from clients inside cluster,
// hook must store it to `global.discovery.extensionAPIServerAuthenticationRequestheaderClientCA`.
func discoveryExtentsionAPIServerCA(_ context.Context, input *go_hook.HookInput) error {
	intervalScrapSnap, err := sdkobjectpatch.UnmarshalToStruct[string](input.Snapshots, "extension_api_server_authentication")
	if err != nil {
		return fmt.Errorf("failed to unmarshal extension_api_server_authentication snapshot: %w", err)
	}
	if len(intervalScrapSnap) == 0 {
		return fmt.Errorf("extension api server authentication not found")
	}

	input.Values.Set("global.discovery.extensionAPIServerAuthenticationRequestheaderClientCA", intervalScrapSnap[0])

	return nil
}
