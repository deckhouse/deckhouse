/*
Copyright 2023 Flant JSC

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

	"github.com/clarketm/json"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/google/go-containerregistry/pkg/authn"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/admission-policy-engine/trivy_provider_secrets",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "trivy_provider_secrets",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-admission-policy-engine"},
				},
			},
			FieldSelector: &types.FieldSelector{
				MatchExpressions: []types.FieldSelectorRequirement{
					{
						Field:    "type",
						Operator: "Equals",
						Value:    string(corev1.SecretTypeDockerConfigJson),
					},
				},
			},
			FilterFunc: fileterTrivyProviderSecrets,
		},
	},
}, handleTrivyProviderSecrets)

type dockerConfig struct {
	Auths map[string]authn.AuthConfig `json:"auths"`
}

func fileterTrivyProviderSecrets(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret corev1.Secret
	err := sdk.FromUnstructured(obj, &secret)
	if err != nil {
		return nil, err
	}

	if secret.Type != corev1.SecretTypeDockerConfigJson {
		return nil, nil
	}

	if secret.Data == nil {
		return nil, nil
	}

	rawCreds, ok := secret.Data[corev1.DockerConfigJsonKey]
	if !ok {
		return nil, nil
	}

	var config dockerConfig
	if err := json.Unmarshal(rawCreds, &config); err != nil {
		return nil, fmt.Errorf("cannot decode docker config JSON: %v", err)
	}

	return config.Auths, nil
}

func handleTrivyProviderSecrets(input *go_hook.HookInput) error {
	if !input.Values.Get("admissionPolicyEngine.trivyProvider.enable").Bool() {
		return nil
	}

	resultCfg := dockerConfig{Auths: make(map[string]authn.AuthConfig)}
	for _, authsSnap := range input.Snapshots["trivy_provider_secrets"] {
		if authsSnap == nil {
			continue
		}

		auths, ok := authsSnap.(map[string]authn.AuthConfig)
		if !ok {
			return fmt.Errorf("can't convert auths snaphsot to map[string]authn.AuthConfig{}: %v", authsSnap)
		}

		for registry, config := range auths {
			resultCfg.Auths[registry] = config
		}
	}

	input.Values.Set("admissionPolicyEngine.internal.trivyProvider.dockerConfigJson", resultCfg)
	return nil
}
