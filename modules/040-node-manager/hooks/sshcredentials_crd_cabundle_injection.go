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
	"time"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Settings: &go_hook.HookConfigSettings{
		ExecutionMinInterval: 5 * time.Second,
		ExecutionBurst:       3,
	},
	Queue: "/modules/node-manager/sshcredentials-crd",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "sshcredentials",
			ApiVersion:                   "apiextensions.k8s.io/v1",
			Kind:                         "CustomResourceDefinition",
			ExecuteHookOnSynchronization: ptr.To(true),
			ExecuteHookOnEvents:          ptr.To(true),
			NameSelector: &types.NameSelector{
				MatchNames: []string{"sshcredentials.deckhouse.io"},
			},
			FilterFunc: applyCRDFilter,
		},
		{
			Name:                         "cabundle",
			ApiVersion:                   "v1",
			Kind:                         "Secret",
			ExecuteHookOnEvents:          ptr.To(false),
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
	},
}, injectCAtoCRD)

type CRD struct {
	Name string
}

func applyCRDFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var crd apiextensionsv1.CustomResourceDefinition

	err := sdk.FromUnstructured(obj, crd)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	if len(crd.Spec.Conversion.Webhook.ClientConfig.CABundle) == 0 {
		return &CRD{Name: crd.Name}, nil
	}

	return nil, nil
}

func applyCAPSWebhookTLSFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := new(v1.Secret)
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, err
	}

	return certificate.Certificate{
		CA:   string(secret.Data["ca.crt"]),
		Cert: string(secret.Data["tls.crt"]),
		Key:  string(secret.Data["tls.key"]),
	}, nil
}

func injectCAtoCRD(input *go_hook.HookInput) error {
	if len(input.Snapshots["cabundle"]) > 0 {
		bundle := input.Snapshots["cabundle"][0]
		crds := input.Snapshots["sshcredentials"]
		for _, crd := range crds {
			patch := map[string]interface{}{
				"spec": map[string]interface{}{
					"conversion": map[string]interface{}{
						"webhook": map[string]interface{}{
							"clientConfig": map[string]interface{}{
								"caBundle": bundle.(certificate.Certificate).CA,
							},
						},
					},
				},
			}
			input.PatchCollector.PatchWithMerge(patch, "apiextensions.k8s.io/v1", "CustomResourceDefinition", "", crd.(CRD).Name)
		}
	}

	return nil
}
