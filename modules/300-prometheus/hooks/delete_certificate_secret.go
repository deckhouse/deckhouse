// Copyright 2024 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package hooks

import (
    "strings"

    "github.com/flant/addon-operator/pkg/module_manager/go_hook"
    "github.com/flant/addon-operator/sdk"
    "github.com/flant/shell-operator/pkg/kube_events_manager/types"
    corev1 "k8s.io/api/core/v1"
    "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"    
)

type Secret struct {
	Name        string
}


var _ = sdk.RegisterFunc(&go_hook.HookConfig{
    Queue: "/modules/prometheus/delete_certificate_secret",
    Kubernetes: []go_hook.KubernetesConfig{
        {
			Name:       "secrets",
			ApiVersion: "v1",
			Kind:       "Secret",
            FilterFunc: applySecretFilter,
            NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-monitoring"},
				},
			},
		},
    },
}, handleCertificateDeletion)

func applySecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret corev1.Secret

	err := sdk.FromUnstructured(obj, &sec)
	if err != nil {
		return nil, err
	}

	return secret.Name, nil
}

func handleCertificateDeletion(input *go_hook.HookInput) error {
    secretSnap := input.Snapshots["secrets"]

    for _, secretName := range secretSnap {
        if secretName == "ingress-tls-v10"{
            input.PatchCollector.Delete("v1", "Secret", "d8-monitoring", secretName)
        }

    return nil
}


