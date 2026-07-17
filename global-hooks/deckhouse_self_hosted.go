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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
)

// deckhouseSelfHostedPath exposes whether Deckhouse runs as a Deployment inside
// this cluster. It lives under `global` so any module/hook can read it, unlike
// the module-scoped `deckhouse.internal.selfHosted`.
const deckhouseSelfHostedPath = "global.deckhouseSelfHosted"

const deckhouseDeploymentSnapName = "deckhouse_deployment"

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       deckhouseDeploymentSnapName,
			ApiVersion: "apps/v1",
			Kind:       "Deployment",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"deckhouse"},
			},
			ExecuteHookOnEvents:          ptr.To(true),
			ExecuteHookOnSynchronization: ptr.To(true),
			FilterFunc:                   applyDeckhouseDeploymentExistsFilter,
		},
	},
}, setDeckhouseSelfHosted)

func applyDeckhouseDeploymentExistsFilter(_ *unstructured.Unstructured) (go_hook.FilterResult, error) {
	// we only need to check the deployment existence
	return true, nil
}

func setDeckhouseSelfHosted(_ context.Context, input *go_hook.HookInput) error {
	// Deckhouse is self-hosted when it has its own Deployment in this cluster.
	// Otherwise it runs in a parent cluster and manages this one via a kubeconfig.
	selfHosted := len(input.Snapshots.Get(deckhouseDeploymentSnapName)) > 0
	input.Values.Set(deckhouseSelfHostedPath, selfHosted)

	return nil
}
