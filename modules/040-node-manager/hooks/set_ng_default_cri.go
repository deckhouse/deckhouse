/*
Copyright 2025 Flant JSC

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
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	ngv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
)

type NodeGroupCRIInfo struct {
	Name string
	Spec ngv1.NodeGroupSpec
}

var DefaultCRIType = "Containerd"

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager/set_cri",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "ngs",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "NodeGroup",
			FilterFunc: setCRING,
		},
	},
}, handleSetCRI)

func setCRING(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var ng ngv1.NodeGroup
	if err := sdk.FromUnstructured(obj, &ng); err != nil {
		return nil, err
	}
	if !ng.Spec.CRI.IsEmpty() {
		return nil, nil
	}
	return NodeGroupCRIInfo{
		Name: ng.GetName(),
		Spec: ng.Spec,
	}, nil
}

func handleSetCRI(input *go_hook.HookInput) error {
	for _, s := range input.Snapshots["ngs"] {
		if s == nil {
			continue
		}
		ng := s.(NodeGroupCRIInfo)
		patch := fmt.Sprintf(`{"spec":{"cri":{"type":"%s"}}}`, DefaultCRIType)
		input.PatchCollector.PatchWithMerge(patch, "deckhouse.io/v1", "NodeGroup", "", ng.Name)
	}
	return nil
}
