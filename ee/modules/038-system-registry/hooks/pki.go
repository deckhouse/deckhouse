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

	"github.com/deckhouse/deckhouse/go_lib/system-registry-manager/pki"
)

type pkiLegacyModel struct {
	CA    pkiCertModel
	Token pkiCertModel
}

type pkiCertModel struct {
	Cert string `json:"cert,omitempty"`
	Key  string `json:"key,omitempty"`
}

func (pcm *pkiCertModel) ToPKICertKey() (pki.CertKey, error) {
	if pcm == nil {
		return pki.CertKey{}, fmt.Errorf("cannot convert nil to CertKey")
	}
	return pki.DecodeCertKey([]byte(pcm.Cert), []byte(pcm.Key))
}

func pkiCertKeyToModel(value pki.CertKey) (*pkiCertModel, error) {
	cert, key, err := pki.EncodeCertKey(value)
	if err != nil {
		return nil, err
	}
	return &pkiCertModel{Cert: string(cert), Key: string(key)}, nil
}

const (
	pkiLegacySnapName = "legacy"
	pkiCASnapName     = "ca"
	pkiTokenSnapName  = "token"
	pkiProxySnapName  = "proxy"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Queue:        "/modules/system-registry/pki",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       pkiLegacySnapName,
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
			Name:       pkiCASnapName,
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
			Name:       pkiTokenSnapName,
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
			Name:       pkiProxySnapName,
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
	caSnaps := input.Snapshots[pkiCASnapName]
	tokenSnaps := input.Snapshots[pkiTokenSnapName]
	proxySnaps := input.Snapshots[pkiProxySnapName]
	legacySnaps := input.Snapshots[pkiLegacySnapName]
	legacySnaps = []go_hook.FilterResult{}

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

	caPKI, err := caCert.ToPKICertKey()
	if err != nil {
		input.Logger.Warn("Cannot decode CA certificate and key", "error", err)

		// TODO: add save/restore/generate

		// No CA = no show
		input.Values.Remove("systemRegistry.internal.pki")
		return nil
	}
	input.Values.Set("systemRegistry.internal.pki.ca", caCert)

	tokenPKI, err := tokenCert.ToPKICertKey()
	if err != nil {
		input.Logger.Warn("Cannot decode Token certificate and key", "error", err)
	} else {
		err = pki.ValidateCertWithCAChain(tokenPKI.Cert, caPKI.Cert)
		if err != nil {
			input.Logger.Warn("Token certificate is not belongs to CA certificate", "error", err)
		}
	}

	if err != nil {
		tokenPKI, err = pki.GenerateCertificate("registry-auth-token", caPKI)
		if err != nil {
			return fmt.Errorf("cannot generate Token certificate: %w", err)
		}

		tokenCert, err = pkiCertKeyToModel(tokenPKI)
		if err != nil {
			return fmt.Errorf("cannot convert Token PKI to model: %w", err)
		}
	}
	input.Values.Set("systemRegistry.internal.pki.token", tokenCert)

	proxyPKI, err := proxyCert.ToPKICertKey()
	if err != nil {
		input.Logger.Warn("Cannot decode Proxy certificate and key", "error", err)
	} else {
		err = pki.ValidateCertWithCAChain(proxyPKI.Cert, caPKI.Cert)
		if err != nil {
			input.Logger.Warn("Proxy certificate is not belongs to CA certificate", "error", err)
		}
	}

	if err != nil {
		proxyPKI, err = pki.GenerateCertificate("registry-proxy", caPKI, "registry.d8-system.svc")
		if err != nil {
			return fmt.Errorf("cannot generate Proxy certificate: %w", err)
		}

		proxyCert, err = pkiCertKeyToModel(proxyPKI)
		if err != nil {
			return fmt.Errorf("cannot convert Proxy PKI to model: %w", err)
		}
	}
	input.Values.Set("systemRegistry.internal.pki.proxy", proxyCert)

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
