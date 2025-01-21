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
	"fmt"

	"github.com/deckhouse/deckhouse/go_lib/module"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	"github.com/deckhouse/deckhouse/go_lib/set"
)

const (
	cniConfigurationSettledKey = "cniConfigurationSettled"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "deckhouse_mc",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "ModuleConfig",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"cni-flannel", "cni-cilium", "cni-simple-bridge"},
			},
			FilterFunc: applyMCFilter,
		},
	},
}, checkCni)

func applyMCFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	v, _, err := unstructured.NestedBool(obj.UnstructuredContent(), "spec", "enabled")
	if err != nil {
		return nil, err
	}

	if !v {
		return nil, nil
	}

	return obj.GetName(), nil
}

func checkCni(input *go_hook.HookInput) error {
	requirements.RemoveValue(cniConfigurationSettledKey)

	deckhouseMCSnap := input.Snapshots["deckhouse_mc"]

	explicitlyEnabledCNIs := set.NewFromSnapshot(deckhouseMCSnap)

	if len(explicitlyEnabledCNIs) > 1 {
		requirements.SaveValue(cniConfigurationSettledKey, "false")
		return fmt.Errorf("more then one CNI enabled: %v", explicitlyEnabledCNIs.Slice())
	} else if len(explicitlyEnabledCNIs) == 1 {
		input.Logger.Infof("Enabled CNI from Deckhouse ModuleConfig: %s", explicitlyEnabledCNIs.Slice()[0])
		return nil
	}

	controlPlaneEnabled := module.IsEnabled("control-plane-manager", input)
	if controlPlaneEnabled {
		requirements.SaveValue(cniConfigurationSettledKey, "false")
		return fmt.Errorf("the cluster is managed by D8, but there are no explicitly enabled CNI-modules")
	}
	return nil
}
