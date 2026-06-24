/*
Copyright 2026 Flant JSC

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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/node-manager",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "containerd_intregrity_policies",
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
	hasCIP := len(input.Snapshots.Get("containerd_intregrity_policies")) > 0

	if hasCIP {
		input.Logger.Info("One or more ContainerdIntegrityPolicies found, nodeManager.internal.containerdIntegrityControllerEnabled set to true")
		input.Values.Set("nodeManager.internal.containerdIntegrityControllerEnabled", true)
	} else {
		input.Logger.Info("No ContainerdIntegrityPolicies found, nodeManager.internal.containerdIntegrityControllerEnabled removed from values")
		input.Values.Remove("nodeManager.internal.containerdIntegrityControllerEnabled")
	}

	return nil
}
