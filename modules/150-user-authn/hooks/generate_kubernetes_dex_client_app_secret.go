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

package hooks

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/pwgen"
)

const (
	// kubernetesDexClientAppSecretPath holds the secret of the privileged kubernetes OAuth2Client.
	// It is shared by every consumer of the kubernetes client and must never be handed out to
	// per-application clients.
	kubernetesDexClientAppSecretPath = "userAuthn.internal.kubernetesDexClientAppSecret"

	kubernetesDexClientAppSecretNamespace = "d8-user-authn"
	kubernetesDexClientAppSecretName      = "kubernetes-dex-client-app-secret"
)

type KubernetesSecret []byte

func applyKubernetesSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes secret to secret: %v", err)
	}

	return secret.Data["secret"], nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "kubernetes_secret",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{kubernetesDexClientAppSecretNamespace},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{kubernetesDexClientAppSecretName},
			},
			FilterFunc: applyKubernetesSecretFilter,
		},
	},
}, kubernetesDexClientAppSecret)

func kubernetesDexClientAppSecret(_ context.Context, input *go_hook.HookInput) error {
	if input.Values.Exists(kubernetesDexClientAppSecretPath) && input.Values.Get(kubernetesDexClientAppSecretPath).String() != "" {
		return nil
	}

	kubernetesSecrets := input.Snapshots.Get("kubernetes_secret")
	if len(kubernetesSecrets) > 0 {
		var secretContent []byte
		err := kubernetesSecrets[0].UnmarshalTo(&secretContent)
		if err != nil {
			return fmt.Errorf("cannot convert kubernetes secret to bytes: failed to unmarshal 'kubernetes_secret' snapshot: %w", err)
		}

		// if secret field was removed, generate a new one
		if len(secretContent) == 0 {
			input.Values.Set(kubernetesDexClientAppSecretPath, pwgen.AlphaNum(20))
			return nil
		}

		input.Values.Set(kubernetesDexClientAppSecretPath, string(secretContent))
		return nil
	}

	input.Values.Set(kubernetesDexClientAppSecretPath, pwgen.AlphaNum(20))
	return nil
}
