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
	"encoding/base64"
	"fmt"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Settings: &go_hook.HookConfigSettings{
		ExecutionMinInterval: 5 * time.Second,
		ExecutionBurst:       3,
	},
	Queue:      "/modules/node-manager/instance-crd-conversion",
	OnAfterAll: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "instances-crd",
			ApiVersion:                   "apiextensions.k8s.io/v1",
			Kind:                         "CustomResourceDefinition",
			ExecuteHookOnSynchronization: ptr.To(false),
			ExecuteHookOnEvents:          ptr.To(false),
			NameSelector: &types.NameSelector{
				MatchNames: []string{"instances.deckhouse.io"},
			},
			FilterFunc: applyInstanceCRDFilter,
		},
		{
			Name:                         "node-controller-webhook-tls",
			ApiVersion:                   "v1",
			Kind:                         "Secret",
			ExecuteHookOnEvents:          ptr.To(true),
			ExecuteHookOnSynchronization: ptr.To(false),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cloud-instance-manager"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"node-controller-webhook-tls"},
			},
			FilterFunc: applyNodeControllerWebhookTLSFilter,
		},
		{
			Name:                         "node-controller-webhook",
			ApiVersion:                   "v1",
			Kind:                         "Service",
			ExecuteHookOnEvents:          ptr.To(true),
			ExecuteHookOnSynchronization: ptr.To(false),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cloud-instance-manager"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"node-controller-webhook"},
			},
			FilterFunc: applyNodeControllerServiceFilter,
		},
	},
}, injectCAToInstanceCRD)

type InstanceCRD struct {
	Name     string
	CABundle string
}

func applyInstanceCRDFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var crd apiextensionsv1.CustomResourceDefinition

	if err := sdk.FromUnstructured(obj, &crd); err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	var caBundle string
	if crd.Spec.Conversion != nil &&
		crd.Spec.Conversion.Webhook != nil &&
		crd.Spec.Conversion.Webhook.ClientConfig != nil &&
		crd.Spec.Conversion.Webhook.ClientConfig.CABundle != nil {
		caBundle = string(crd.Spec.Conversion.Webhook.ClientConfig.CABundle)
	}

	if len(caBundle) > 0 {
		decoded, err := base64.StdEncoding.DecodeString(caBundle)
		if err != nil {
			caBundle = ""
		} else {
			caBundle = string(decoded)
		}
	}

	return InstanceCRD{Name: crd.Name, CABundle: caBundle}, nil
}

func injectCAToInstanceCRD(_ context.Context, input *go_hook.HookInput) error {
	if len(input.Snapshots.Get("instances-crd")) == 0 {
		input.Logger.Info("instances.deckhouse.io CRD not found, skipping")
		return nil
	}

	if len(input.Snapshots.Get("node-controller-webhook")) == 0 {
		input.Logger.Info("node-controller-webhook service not found, skipping")
		return nil
	}

	if len(input.Snapshots.Get("node-controller-webhook-tls")) == 0 {
		input.Logger.Info("node-controller-webhook-tls secret not found, skipping")
		return nil
	}

	var crd InstanceCRD
	var bundle certificate.Certificate

	if err := input.Snapshots.Get("node-controller-webhook-tls")[0].UnmarshalTo(&bundle); err != nil {
		return fmt.Errorf("failed to unmarshal 'node-controller-webhook-tls' snapshot: %w", err)
	}

	if err := input.Snapshots.Get("instances-crd")[0].UnmarshalTo(&crd); err != nil {
		return fmt.Errorf("failed to unmarshal 'instances-crd' snapshot: %w", err)
	}

	if crd.CABundle == bundle.CA {
		input.Logger.Debug("CA bundle already up to date, skipping patch")
		return nil
	}

	input.Logger.Info("Patching instances.deckhouse.io CRD conversion webhook")

	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"conversion": map[string]interface{}{
				"strategy": "Webhook",
				"webhook": map[string]interface{}{
					"clientConfig": map[string]interface{}{
						"caBundle": base64.StdEncoding.EncodeToString([]byte(bundle.CA)),
						"service": map[string]interface{}{
							"namespace": "d8-cloud-instance-manager",
							"name":      "node-controller-webhook",
							"path":      "/convert",
							"port":      int64(443),
						},
					},
					"conversionReviewVersions": []string{"v1"},
				},
			},
		},
	}

	input.PatchCollector.PatchWithMerge(patch, "apiextensions.k8s.io/v1", "CustomResourceDefinition", "", crd.Name)

	input.Logger.Info("Successfully patched instances.deckhouse.io CRD conversion webhook",
		"service", "node-controller-webhook",
		"namespace", "d8-cloud-instance-manager",
		"path", "/convert",
	)

	return nil
}
