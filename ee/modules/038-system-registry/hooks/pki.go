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

type pkiLegacyModel struct {
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
			Name:       "legacy",
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

				ret := pkiLegacyModel{
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
		{
			Name:       "ca",
			ApiVersion: "v1",
			Kind:       "Secret",
			NameSelector: &types.NameSelector{
				MatchNames: []string{
					"registry-pki-ca",
				},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			FilterFunc: pkiFilterCertSecret,
		},
		{
			Name:       "token",
			ApiVersion: "v1",
			Kind:       "Secret",
			NameSelector: &types.NameSelector{
				MatchNames: []string{
					"registry-pki-token",
				},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			FilterFunc: pkiFilterCertSecret,
		},
		{
			Name:       "proxy",
			ApiVersion: "v1",
			Kind:       "Secret",
			NameSelector: &types.NameSelector{
				MatchNames: []string{
					"registry-pki-proxy",
				},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			FilterFunc: pkiFilterCertSecret,
		},
	},
}, func(input *go_hook.HookInput) error {
	caSnaps := input.Snapshots["ca"]
	tokenSnaps := input.Snapshots["token"]
	proxySnaps := input.Snapshots["proxy"]
	legacySnaps := input.Snapshots["pki"]

	var caCert, tokenCert, proxyCert *pkiCertModel

	if len(caSnaps) == 1 {
		val := caSnaps[0].(pkiCertModel)
		caCert = &val
	}

	if len(tokenSnaps) == 1 {
		val := tokenSnaps[0].(pkiCertModel)
		tokenCert = &val
	}

	if len(proxySnaps) == 1 {
		val := proxySnaps[0].(pkiCertModel)
		proxyCert = &val
	}

	if caCert == nil && len(legacySnaps) == 1 {
		val := legacySnaps[0].(pkiLegacyModel)

		if val.CA.Cert != "" && val.CA.Key != "" {
			caCert = &val.CA

			if val.Token.Cert != "" && val.Token.Key != "" {
				tokenCert = &val.Token
			}
		}
	}

	if caCert == nil {
		// No CA = no show
		input.Values.Remove("systemRegistry.internal.pki")
		return nil
	}
	input.Values.Set("systemRegistry.internal.pki.ca", caCert)

	if tokenCert != nil {
		input.Values.Set("systemRegistry.internal.pki.token", tokenCert)
	} else {
		input.Values.Remove("systemRegistry.internal.pki.token")
	}

	if proxyCert != nil {
		input.Values.Set("systemRegistry.internal.pki.proxy", tokenCert)
	} else {
		input.Values.Remove("systemRegistry.internal.pki.proxy")
	}

	return nil
})

func pkiFilterCertSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret v1core.Secret

	err := sdk.FromUnstructured(obj, &secret)
	if err != nil {
		return "", fmt.Errorf("failed to convert pki secret to struct: %v", err)
	}

	ret := pkiCertModel{
		Cert: string(secret.Data["tls.crt"]),
		Key:  string(secret.Data["tls.key"]),
	}

	return ret, nil
}
