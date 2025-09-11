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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/set"
)

const namespacesValuesPath = `deckhouse.internal.namespaces`

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/flow-schema",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "namespaces",
			ApiVersion: "v1",
			Kind:       "Namespace",
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "heritage",
						Operator: metav1.LabelSelectorOpIn,
						Values: []string{
							"deckhouse",
						},
					},
				},
			},
			FilterFunc: applyNamespaceFilter,
		},
	},
}, handleNamespaces)

func applyNamespaceFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}

func handleNamespaces(_ context.Context, input *go_hook.HookInput) error {
	snap := input.Snapshots.Get("namespaces")
	if len(snap) == 0 {
		input.Logger.Warn("deckhouse namespaces not found")
		return nil
	}

	nsSlice := set.NewFromSnapshot(snap).Slice()

	input.Values.Set(namespacesValuesPath, nsSlice)
	return nil
}
