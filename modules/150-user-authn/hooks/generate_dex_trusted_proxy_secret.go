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
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"

	"github.com/deckhouse/deckhouse/go_lib/pwgen"
)

const (
	// dexTrustedProxySecretPath holds the secret that the Gateway (and basic-auth-proxy, once it passes the
	// client address on) send to Dex in X-Deckhouse-Trusted-Proxy. Dex trusts the client address they pass
	// only when the secret matches, so a pod that merely reaches the Dex Service cannot forge it. The secret
	// is in the HTTPRoute object of the module, readable by whoever can read it. ingress-nginx proves itself
	// with its client certificate instead.
	dexTrustedProxySecretPath = "userAuthn.internal.dexTrustedProxySecret"

	dexTrustedProxySecretNamespace = "d8-user-authn"
	dexTrustedProxySecretName      = "dex-trusted-proxy-secret"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "trusted_proxy_secret",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{dexTrustedProxySecretNamespace},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{dexTrustedProxySecretName},
			},
			FilterFunc: applyKubernetesSecretFilter,
		},
	},
}, dexTrustedProxySecret)

// dexTrustedProxySecret keeps the secret across restarts: it reads it back from the Secret the module
// renders and generates a new one only when there is none.
func dexTrustedProxySecret(_ context.Context, input *go_hook.HookInput) error {
	if input.Values.Get(dexTrustedProxySecretPath).String() != "" {
		return nil
	}

	secrets := input.Snapshots.Get("trusted_proxy_secret")
	if len(secrets) > 0 {
		var content []byte
		if err := secrets[0].UnmarshalTo(&content); err != nil {
			return fmt.Errorf("unmarshal the 'trusted_proxy_secret' snapshot: %w", err)
		}
		if len(content) > 0 {
			input.Values.Set(dexTrustedProxySecretPath, string(content))
			return nil
		}
	}

	input.Values.Set(dexTrustedProxySecretPath, pwgen.AlphaNum(32))
	return nil
}
