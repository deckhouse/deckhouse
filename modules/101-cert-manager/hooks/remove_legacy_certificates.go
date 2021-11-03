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

// Legacy deckhouse certificates could not be removed by Helm
// remove them manually.

// This hook should be deleted when legacy cert-manager removed

package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/modules/101-cert-manager/hooks/internal"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: internal.Queue("certificates"),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "certificates",
			ApiVersion: "certmanager.k8s.io/v1alpha1",
			Kind:       "Certificate",
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "heritage",
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"deckhouse"},
					},
					{
						Key:      "app.kubernetes.io/managed-by",
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"Helm"},
					},
				},
			},
			FilterFunc: applyLegacyCertManagerCRFilter,
		},
	},
}, removeLegacyCerts)

func removeLegacyCerts(input *go_hook.HookInput) error {
	snap := input.Snapshots["certificates"]
	for _, sn := range snap {
		cert := sn.(legacyObject)

		input.PatchCollector.Delete("certmanager.k8s.io/v1alpha1", "Certificate", cert.Namespace, cert.Name)
	}

	return nil
}
