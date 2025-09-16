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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
)

const (
	snap          = "d8-kube-dns"
	containerName = "coredns"
)

var ports = []v1.ContainerPort{
	{
		ContainerPort: 5353,
		Name:          "dns",
		Protocol:      "UDP",
	},
	{
		ContainerPort: 5353,
		Name:          "dns-tcp",
		Protocol:      "TCP",
	},
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       snap,
			ApiVersion: "apps/v1",
			Kind:       "Deployment",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-kube-dns"},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			FilterFunc:                   applyDeploymentCorednsPortsFilter,
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
		},
	},
}, ensureCorednsPorts)

func applyDeploymentCorednsPortsFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var depl appsv1.Deployment
	err := sdk.FromUnstructured(obj, &depl)
	if err != nil {
		return nil, err
	}
	ports := []v1.ContainerPort{}
	for _, v := range depl.Spec.Template.Spec.Containers {
		if v.Name == containerName {
			ports = append(ports, v.Ports...)
		}
	}
	return ports, nil
}

func ensureCorednsPorts(_ context.Context, input *go_hook.HookInput) error {
	portsSnap := input.Snapshots.Get(snap)
	if len(portsSnap) == 0 {
		return nil
	}

	applyPorts := func(u *unstructured.Unstructured) (*unstructured.Unstructured, error) {
		var depl appsv1.Deployment
		err := sdk.FromUnstructured(u, &depl)
		if err != nil {
			return nil, err
		}

		for i, v := range depl.Spec.Template.Spec.Containers {
			if v.Name == containerName {
				depl.Spec.Template.Spec.Containers[i].Ports = ports
			}
		}

		return sdk.ToUnstructured(&depl)
	}

	input.PatchCollector.PatchWithMutatingFunc(applyPorts, "apps/v1", "Deployment", "kube-system", "d8-kube-dns")

	return nil
}
