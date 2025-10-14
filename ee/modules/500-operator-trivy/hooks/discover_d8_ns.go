/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
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

const enabledNamespacesValuesPath = `operatorTrivy.internal.enabledNamespaces`

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/operator-trivy",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "namespaces",
			ApiVersion: "v1",
			Kind:       "Namespace",
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "security-scanning.deckhouse.io/enabled",
						Operator: metav1.LabelSelectorOpExists,
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
		input.Logger.Warn("trivy namespaces not found")
		return nil
	}

	nsSlice := set.NewFromSnapshot(snap).Slice()

	input.Values.Set(enabledNamespacesValuesPath, nsSlice)
	return nil
}
