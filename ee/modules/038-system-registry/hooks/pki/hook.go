/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package pki

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/system-registry-manager/pki"
)

type legacyModel struct {
	CA    *certModel
	Token *certModel
}

const (
	snapLegacyName = "legacy"
	snapCAName     = "ca"
	snapTokenName  = "token"
	snapProxyName  = "proxy"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Queue:        "/modules/system-registry/pki",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:              snapLegacyName,
			ApiVersion:        "v1",
			Kind:              "Secret",
			NamespaceSelector: namespaceSelector,
			NameSelector: &types.NameSelector{
				MatchNames: []string{
					"registry-pki",
				},
			},
			FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
				var secret v1core.Secret

				err := sdk.FromUnstructured(obj, &secret)
				if err != nil {
					return "", fmt.Errorf("failed to convert secret \"%v\" to struct: %v", obj.GetName(), err)
				}

				ret := legacyModel{
					CA:    secretDataToCertModel(secret, "registry-ca"),
					Token: secretDataToCertModel(secret, "token"),
				}

				return ret, nil
			},
		},
		{
			Name:              snapCAName,
			ApiVersion:        "v1",
			Kind:              "Secret",
			NamespaceSelector: namespaceSelector,
			NameSelector: &types.NameSelector{
				MatchNames: []string{
					"registry-pki-ca",
				},
			},

			FilterFunc: filterCertSecret,
		},
		{
			Name:              snapTokenName,
			ApiVersion:        "v1",
			Kind:              "Secret",
			NamespaceSelector: namespaceSelector,
			NameSelector: &types.NameSelector{
				MatchNames: []string{
					"registry-pki-token",
				},
			},
			FilterFunc: filterCertSecret,
		},
		{
			Name:              snapProxyName,
			ApiVersion:        "v1",
			Kind:              "Secret",
			NamespaceSelector: namespaceSelector,
			NameSelector: &types.NameSelector{
				MatchNames: []string{
					"registry-pki-proxy",
				},
			},
			FilterFunc: filterCertSecret,
		},
	},
}, func(input *go_hook.HookInput) error {
	type valuesModel struct {
		Mode  modeType   `json:"mode"`
		CA    *certModel `json:"ca,omitempty"`
		Token *certModel `json:"token,omitempty"`
		Proxy *certModel `json:"proxy,omitempty"`
	}

	ret := valuesModel{
		Mode: getMode(input),
	}

	if ret.Mode == modeUnmanaged {
		input.Values.Set(inputValuesPrefix, ret)
		return nil
	}

	if snaps := input.Snapshots[snapCAName]; len(snaps) == 1 {
		val := snaps[0].(certModel)
		ret.CA = &val
	}

	if snaps := input.Snapshots[snapTokenName]; len(snaps) == 1 {
		val := snaps[0].(certModel)
		ret.Token = &val
	}

	if ret.CA == nil {
		if snaps := input.Snapshots[snapLegacyName]; len(snaps) == 1 {
			val := snaps[0].(legacyModel)

			if val.CA != nil {
				ret.CA = val.CA

				if val.Token != nil {
					ret.Token = val.Token
				}
			}
		}
	}

	// CA
	caPKI, err := ret.CA.ToPKI()
	if err != nil {
		prevErr := err
		if caPKI, err = inputValuesToCertModel(input, inputValuesCA).ToPKI(); err == nil {
			input.Logger.Warn("Cannot decode CA certificate and key, restored from memory", "error", prevErr)
		}
	}

	if err != nil {
		input.Logger.Warn("Cannot decode CA certificate and key, will generate new", "error", err)

		caPKI, err = pki.GenerateCACertificate("registry-ca")
		if err != nil {
			return fmt.Errorf("cannot generate CA certificate: %w", err)
		}
	}

	ret.CA, err = certKeyToCertModel(caPKI)
	if err != nil {
		return fmt.Errorf("cannot convert CA PKI to model: %w", err)
	}

	// Token
	tokenPKI, err := ret.Token.ToPKI()
	if err != nil {
		prevErr := err
		if tokenPKI, err = inputValuesToCertModel(input, inputValuesToken).ToPKI(); err == nil {
			input.Logger.Warn("Cannot decode Token certificate and key, restored from memory", "error", prevErr)
		}
	}

	if err == nil {
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
	}

	ret.Token, err = certKeyToCertModel(tokenPKI)
	if err != nil {
		return fmt.Errorf("cannot convert Token PKI to model: %w", err)
	}

	if ret.Mode == modeDirect {

		// Proxy
		proxySnaps := input.Snapshots[snapProxyName]
		if len(proxySnaps) == 1 {
			val := proxySnaps[0].(certModel)
			ret.Proxy = &val
		}

		proxyPKI, err := ret.Proxy.ToPKI()
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

		}

		ret.Proxy, err = certKeyToCertModel(proxyPKI)
		if err != nil {
			return fmt.Errorf("cannot convert Proxy PKI to model: %w", err)
		}
	}

	input.Values.Set(inputValuesPrefix, ret)
	return nil
})

func filterCertSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret v1core.Secret

	err := sdk.FromUnstructured(obj, &secret)
	if err != nil {
		return "", fmt.Errorf("failed to convert secret \"%v\" to struct: %v", obj.GetName(), err)
	}

	if ret := secretDataToCertModel(secret, "tls"); ret != nil {
		return *ret, nil
	}

	return "", nil
}
