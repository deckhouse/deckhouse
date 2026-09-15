/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/hooks/internal"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 19},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "credential_secrets",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{internal.Namespace},
				},
			},
			FilterFunc: internal.FilterCredentialSecret,
		},
	},
}, handleCredentials)

func handleCredentials(_ context.Context, input *go_hook.HookInput) error {
	secrets, err := sdkobjectpatch.UnmarshalToStruct[internal.CredentialSecretFilterResult](input.Snapshots, "credential_secrets")
	if err != nil {
		return err
	}

	result := make(map[string]internal.CredentialSecretValues, len(secrets))
	for _, secret := range secrets {
		if secret.Name == "" {
			continue
		}

		result[secret.Name] = internal.CredentialSecretValues{
			AuthScheme: secret.AuthScheme,
			Identity:   secret.Identity,
			Secret:     secret.Secret,
		}
	}

	input.Values.Set("cloudProviderZvirt.internal.credentialSecrets", result)

	return nil
}
