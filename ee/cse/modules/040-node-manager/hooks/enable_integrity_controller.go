/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/node-manager",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "containerd_integrity_policies",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "ContainerdIntegrityPolicy",
			FilterFunc: containerdIntegrityPolicyFilter,
		},
	},
}, handleContainerdIntegrityPolicies)

func containerdIntegrityPolicyFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}

func handleContainerdIntegrityPolicies(_ context.Context, input *go_hook.HookInput) error {
	hasCIP := len(input.Snapshots.Get("containerd_integrity_policies")) > 0

	if hasCIP {
		input.Logger.Info("One or more ContainerdIntegrityPolicies found, nodeManager.internal.containerdIntegrityControllerEnabled set to true")
		input.Values.Set("nodeManager.internal.containerdIntegrityControllerEnabled", true)
	} else {
		input.Logger.Info("No ContainerdIntegrityPolicies found, nodeManager.internal.containerdIntegrityControllerEnabled removed from values")
		input.Values.Remove("nodeManager.internal.containerdIntegrityControllerEnabled")
	}

	return nil
}
