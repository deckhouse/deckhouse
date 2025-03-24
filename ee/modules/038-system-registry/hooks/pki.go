/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type pkiSecretModel struct {
	CA    pkiCertModel
	Token pkiCertModel
}

type pkiCertModel struct {
	Cert string `json:"cert,omitempty"`
	Key  string `json:"key,omitempty"`
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Queue:        "/modules/system-registry/pki",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "pki",
			ApiVersion: "v1",
			Kind:       "Secret",
			NameSelector: &types.NameSelector{
				MatchNames: []string{
					"registry-pki",
				},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
				var secret v1core.Secret

				err := sdk.FromUnstructured(obj, &secret)
				if err != nil {
					return "", fmt.Errorf("failed to convert pki secret to struct: %v", err)
				}

				ret := pkiSecretModel{
					CA: pkiCertModel{
						Cert: string(secret.Data["registry-ca.crt"]),
						Key:  string(secret.Data["registry-ca.key"]),
					},
					Token: pkiCertModel{
						Cert: string(secret.Data["token.crt"]),
						Key:  string(secret.Data["token.key"]),
					},
				}

				return ret, nil
			},
		},
	},
}, func(input *go_hook.HookInput) error {
	pkiSnaps := input.Snapshots["pki"]

	var (
		pkiSecret pkiSecretModel
	)

	if len(pkiSnaps) == 1 {
		pkiSecret = pkiSnaps[0].(pkiSecretModel)
	}

	if pkiSecret.CA.Cert == "" && pkiSecret.CA.Key == "" {
		// No CA = no show
		input.Values.Remove("systemRegistry.internal.pki")
		return nil
	}

	input.Values.Set("systemRegistry.internal.pki.ca", pkiSecret.CA)

	if pkiSecret.Token.Cert != "" && pkiSecret.Token.Key != "" {
		input.Values.Set("systemRegistry.internal.pki.token", pkiSecret.CA)
	} else {
		input.Values.Remove("systemRegistry.internal.pki.token")
	}

	return nil
})
