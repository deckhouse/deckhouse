/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/system-registry",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "deckhouse-registry",
			ApiVersion: "v1",
			Kind:       "Secret",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":    "registry",
					"name":   "deckhouse-registry",
					"module": "deckhouse",
				},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			FilterFunc: systemRegistrySecretFilter,
		},
	},
}, handleSetRegistryMode)

func systemRegistrySecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &corev1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot parse secret from unstructured: %v", err)
	}

	addressBytes, exists := secret.Data["address"]

	if exists {
		address := string(addressBytes)
		if address == "localhost:5000" {
			return "Indirect", nil
		}
	}
	return "Direct", nil
}

func handleSetRegistryMode(input *go_hook.HookInput) error {
	snapshots := input.Snapshots["deckhouse-registry"]
	if len(snapshots) == 0 {
		input.Values.Set("nodeManager.internal.registryMode", "Direct")
		return nil
	}

	registryMode := snapshots[0].(string)
	input.Values.Set("nodeManager.internal.registryMode", registryMode)

	return nil
}
