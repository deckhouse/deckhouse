/*
Copyright 2021 Flant JSC

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

package delete_not_matching_certificate_secret

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/module"
)

type CustomCertificateSecret struct {
	Namespace string
	Name      string
	Issuer    string
}

func applySecretIssuerFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return CustomCertificateSecret{Namespace: obj.GetNamespace(), Name: obj.GetName(), Issuer: obj.GetAnnotations()["cert-manager.io/issuer-name"]}, nil
}

func RegisterHook(moduleName string, namespace string) bool {
	return sdk.RegisterFunc(&go_hook.HookConfig{
		OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:              "custom_certificate_secret",
				ApiVersion:        "v1",
				Kind:              "Secret",
				NamespaceSelector: &types.NamespaceSelector{NameSelector: &types.NameSelector{MatchNames: []string{namespace}}},
				LabelSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "owner",
						Operator: "NotIn",
						Values:   []string{"helm"},
					},
				}},
				FilterFunc: applySecretIssuerFilter,
			},
		},
	}, deleteNotMatchingCertificateSecretHandler(moduleName))
}

func deleteNotMatchingCertificateSecretHandler(moduleName string) func(input *go_hook.HookInput) error {
	return func(input *go_hook.HookInput) error {
		httpsMode := module.GetHTTPSMode(moduleName, input)

		if httpsMode != "CertManager" {
			return nil
		}

		snapshots := input.Snapshots["custom_certificate_secret"]
		clusterIssuer := module.GetCertificateIssuerName(moduleName, input)
		secretName := module.GetHTTPSSecretName("ingress-tls", moduleName, input)

		for _, snapshot := range snapshots {
			cs := snapshot.(CustomCertificateSecret)

			if secretName != cs.Name {
				continue
			}

			if cs.Issuer != clusterIssuer {
				input.PatchCollector.Delete("v1", "Secret", cs.Namespace, cs.Name)
			}
		}

		return nil
	}
}
