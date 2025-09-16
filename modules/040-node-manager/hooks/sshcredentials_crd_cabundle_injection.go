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
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
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
	Queue:      "/modules/node-manager/sshcredentials-crd",
	OnAfterAll: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "sshcredentials",
			ApiVersion:                   "apiextensions.k8s.io/v1",
			Kind:                         "CustomResourceDefinition",
			ExecuteHookOnSynchronization: ptr.To(false),
			ExecuteHookOnEvents:          ptr.To(false),
			NameSelector: &types.NameSelector{
				MatchNames: []string{"sshcredentials.deckhouse.io"},
			},
			FilterFunc: applyCRDFilter,
		},
		{
			Name:                         "cabundle",
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
				MatchNames: []string{"caps-controller-manager-webhook-tls"},
			},
			FilterFunc: applyCAPSWebhookTLSFilter,
		},
		{
			Name:                         "webhook-service",
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
				MatchNames: []string{"caps-controller-manager-webhook-service"},
			},
			FilterFunc: applyCAPSServiceFilter,
		},
	},
}, injectCAtoCRD)

type CRD struct {
	Name     string
	CABundle string
}

type Svc struct {
	Name string
}

func applyCRDFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var crd apiextensionsv1.CustomResourceDefinition

	err := sdk.FromUnstructured(obj, &crd)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	var caBundle string
	if crd.Spec.Conversion != nil && crd.Spec.Conversion.Webhook != nil && crd.Spec.Conversion.Webhook.ClientConfig != nil && crd.Spec.Conversion.Webhook.ClientConfig.CABundle != nil {
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

	return CRD{Name: crd.Name, CABundle: caBundle}, nil
}

func applyCAPSWebhookTLSFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret v1.Secret
	err := sdk.FromUnstructured(obj, &secret)
	if err != nil {
		return nil, err
	}

	return certificate.Certificate{
		CA:   string(secret.Data["ca.crt"]),
		Cert: string(secret.Data["tls.crt"]),
		Key:  string(secret.Data["tls.key"]),
	}, nil
}

func applyCAPSServiceFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var svc v1.Service

	err := sdk.FromUnstructured(obj, &svc)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	return Svc{
		Name: svc.Name,
	}, nil
}

func injectCAtoCRD(_ context.Context, input *go_hook.HookInput) error {
	if len(input.Snapshots.Get("sshcredentials")) == 0 {
		return nil
	}

	if len(input.Snapshots.Get("webhook-service")) == 0 {
		return nil
	}

	if len(input.Snapshots.Get("cabundle")) > 0 {
		var crd CRD
		var bundle certificate.Certificate

		err := input.Snapshots.Get("cabundle")[0].UnmarshalTo(&bundle)
		if err != nil {
			return fmt.Errorf("failed to unmarshal first 'cabundle' snapshot: %w", err)
		}

		err = input.Snapshots.Get("sshcredentials")[0].UnmarshalTo(&crd)
		if err != nil {
			return fmt.Errorf("failed to unmarshal first 'sshcredentials' snapshot: %w", err)
		}

		if crd.CABundle == bundle.CA {
			return nil
		}

		// when updating CRD, the spec.conversion field is not updated, so conversion webhook won't work
		// in this case, we have to set the whole section by appropriate values
		patch := map[string]interface{}{
			"spec": map[string]interface{}{
				"conversion": map[string]interface{}{
					"strategy": "Webhook",
					"webhook": map[string]interface{}{
						"clientConfig": map[string]interface{}{
							"caBundle": base64.StdEncoding.EncodeToString([]byte(bundle.CA)),
							"service": map[string]interface{}{
								"namespace": "d8-cloud-instance-manager",
								"name":      "caps-controller-manager-webhook-service",
								"path":      "/convert",
							},
						},
						"conversionReviewVersions": []string{"v1"},
					},
				},
			},
		}
		input.PatchCollector.PatchWithMerge(patch, "apiextensions.k8s.io/v1", "CustomResourceDefinition", "", crd.Name)
	}

	return nil
}
