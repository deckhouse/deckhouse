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
	// dexSessionsCookieKeyPath holds the key Dex encrypts the cookie of its browser sessions with
	// (AES-256: 32 bytes). Losing it only signs the browsers out of Dex: the sessions of the
	// applications are kept.
	dexSessionsCookieKeyPath = "userAuthn.internal.dexSessionsCookieKey"

	dexSessionsCookieKeyNamespace = "d8-user-authn"
	dexSessionsCookieKeyName      = "dex-sessions-cookie-key"
	dexSessionsCookieKeyLength    = 32
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "sessions_cookie_key",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{dexSessionsCookieKeyNamespace},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{dexSessionsCookieKeyName},
			},
			FilterFunc: applyKubernetesSecretFilter,
		},
	},
}, dexSessionsCookieKey)

// dexSessionsCookieKey keeps the key across restarts: it reads it back from the Secret the module
// renders, and generates one only when there is none. A stored key of another length, which Dex
// would refuse, is replaced.
func dexSessionsCookieKey(_ context.Context, input *go_hook.HookInput) error {
	if len(input.Values.Get(dexSessionsCookieKeyPath).String()) == dexSessionsCookieKeyLength {
		return nil
	}

	secrets := input.Snapshots.Get("sessions_cookie_key")
	if len(secrets) > 0 {
		var content []byte
		if err := secrets[0].UnmarshalTo(&content); err != nil {
			return fmt.Errorf("unmarshal the 'sessions_cookie_key' snapshot: %w", err)
		}
		if len(content) == dexSessionsCookieKeyLength {
			input.Values.Set(dexSessionsCookieKeyPath, string(content))
			return nil
		}
	}

	input.Values.Set(dexSessionsCookieKeyPath, pwgen.AlphaNum(dexSessionsCookieKeyLength))
	return nil
}
