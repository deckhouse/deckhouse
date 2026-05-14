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

	"github.com/deckhouse/deckhouse/modules/101-cert-manager/hooks/internal"
)

const cainjectorEnabledValuesPath = "certManager.internal.enableCAInjector"

var caInjectorAnnotations = map[string]struct{}{
	"cert-manager.io/inject-ca-from":        {},
	"cert-manager.io/inject-ca-from-secret": {},
	"cert-manager.io/inject-apiserver-ca":   {},
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        internal.Queue("cainjector_usage"),
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "validating_webhook_configurations",
			ApiVersion: "admissionregistration.k8s.io/v1",
			Kind:       "ValidatingWebhookConfiguration",
			FilterFunc: cainjectorUsageFilter,
		},
		{
			Name:       "mutating_webhook_configurations",
			ApiVersion: "admissionregistration.k8s.io/v1",
			Kind:       "MutatingWebhookConfiguration",
			FilterFunc: cainjectorUsageFilter,
		},
		{
			Name:       "custom_resource_definitions",
			ApiVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
			FilterFunc: cainjectorUsageFilter,
		},
		{
			Name:       "api_services",
			ApiVersion: "apiregistration.k8s.io/v1",
			Kind:       "APIService",
			FilterFunc: cainjectorUsageFilter,
		},
	},
}, discoverCainjectorUsage)

func cainjectorUsageFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	annotations := obj.GetAnnotations()
	if len(annotations) == 0 {
		return nil, nil
	}

	for annotation := range caInjectorAnnotations {
		if _, ok := annotations[annotation]; ok {
			return true, nil
		}
	}

	return nil, nil
}

func discoverCainjectorUsage(_ context.Context, input *go_hook.HookInput) error {
	detectedUsage := len(input.Snapshots.Get("validating_webhook_configurations")) > 0 ||
		len(input.Snapshots.Get("mutating_webhook_configurations")) > 0 ||
		len(input.Snapshots.Get("custom_resource_definitions")) > 0 ||
		len(input.Snapshots.Get("api_services")) > 0

	isEnabledInConfig := input.ConfigValues.Get("certManager.enableCAInjector").Bool()
	input.Values.Set(cainjectorEnabledValuesPath, isEnabledInConfig || detectedUsage)

	return nil
}
