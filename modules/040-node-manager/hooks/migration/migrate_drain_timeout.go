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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	ngv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
)

type nodeGroupwithSpec struct {
	Name string
	Spec ngv1.NodeGroupSpec
}

var defaultDrainTimeoutSec int32 = 600

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 1},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "ngs",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "NodeGroup",
			FilterFunc: ngCloudInstancesFilter,
		},
	},
}, migrateDrainTimeout)

func ngCloudInstancesFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var ng ngv1.NodeGroup
	err := sdk.FromUnstructured(obj, &ng)
	if err != nil {
		return "", err
	}
	return &nodeGroupwithSpec{
		Name: ng.Name,
		Spec: ng.Spec,
	}, nil
}

func migrateDrainTimeout(input *go_hook.HookInput) error {
	ngsSnapshot := input.Snapshots["ngs"]
	for _, ngRaw := range ngsSnapshot {
		ng := ngRaw.(*nodeGroupwithSpec)

		quickShutdown := ng.Spec.CloudInstances.QuickShutdown
		if quickShutdown == nil || !*quickShutdown {
			continue
		}

		var drainTimeoutSec int32
		if ng.Spec.CloudInstances.DrainTimeout != nil {
			drainTimeoutSec = *ng.Spec.CloudInstances.DrainTimeout
		} else {
			drainTimeoutSec = int32(defaultDrainTimeoutSec)
		}

		drainTimeoutPatch := map[string]interface{}{
			"spec": map[string]interface{}{
				"cloudInstances": map[string]interface{}{
					"quickShutdown":   nil,
					"drainTimeoutSec": drainTimeoutSec,
				},
			},
		}

		input.PatchCollector.MergePatch(drainTimeoutPatch, "deckhouse.io/v1", "NodeGroup", "", ng.Name)
	}

	return nil
}
