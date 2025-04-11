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

	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/helpers"
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/helpers/submodule"
	"github.com/deckhouse/deckhouse/go_lib/system-registry-manager/pki"
)

const (
	snapName      = "pki"
	SubmoduleName = "pki"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/system-registry/pki",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:              snapName,
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

				ret := State{
					CA:    secretDataToCertModel(secret, "registry-ca"),
					Token: secretDataToCertModel(secret, "token"),
				}

				return ret, nil
			},
		},
	},
}, func(input *go_hook.HookInput) error {
	moduleConfig := submodule.NewConfigAccessor[any](input, SubmoduleName)
	moduleState := submodule.NewStateAccessor[State](input, SubmoduleName)

	config := moduleConfig.Get()

	if !config.Enabled {
		moduleState.Clear()
		return nil
	}

	var err error

	state := moduleState.Get()
	state.Version = config.Version

	secretData, _ := helpers.SnapshotToSingle[State](input, snapName)

	if state.Hash, err = helpers.ComputeHash(secretData); err != nil {
		return fmt.Errorf("cannot compute data hash: %w", err)
	}

	data := state.Data

	// CA
	caPKI, err := data.CA.ToPKI()
	if err != nil {
		prevErr := err
		if caPKI, err = secretData.CA.ToPKI(); err == nil {
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

	data.CA, err = certKeyToCertModel(caPKI)
	if err != nil {
		return fmt.Errorf("cannot convert CA PKI to model: %w", err)
	}

	// Token
	tokenPKI, err := data.Token.ToPKI()
	if err != nil {
		prevErr := err
		if tokenPKI, err = secretData.Token.ToPKI(); err == nil {
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

	data.Token, err = certKeyToCertModel(tokenPKI)
	if err != nil {
		return fmt.Errorf("cannot convert Token PKI to model: %w", err)
	}

	state.Data = data
	state.Ready = true
	moduleState.Set(state)
	return nil
})

type State struct {
	CA    *certModel `json:"ca,omitempty"`
	Token *certModel `json:"token,omitempty"`
}

type Params struct {
}
