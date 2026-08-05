/*
Copyright 2025 Flant JSC

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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	ycmeta "github.com/deckhouse/deckhouse/cloud-provider-yandex/pkg/meta"
	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	"github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/hooks/internal"
)

type credentialSecretSnapshot struct {
	Name       string `json:"name"`
	AuthScheme string `json:"authScheme"`
	Identity   string `json:"identity,omitempty"`
	Secret     string `json:"secret"`
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 19},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "credential_secrets",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{ycmeta.Namespace},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{
					cpapi.CredentialSecretName,
					ycmeta.ExporterCredentialSecretName,
				},
			},
			FilterFunc: filterCredentialSecrets,
		},
	},
}, handleCredentials)

func filterCredentialSecrets(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret, managed, err := internal.DecodeCredentialSecret(obj)
	if err != nil || !managed {
		return nil, err
	}

	snap := credentialSecretSnapshot{
		Name:       secret.Name,
		AuthScheme: string(secret.Data["authScheme"]),
		Identity:   string(secret.Data["identity"]),
		Secret:     string(secret.Data["secret"]),
	}
	return snap, nil
}

func handleCredentials(_ context.Context, input *go_hook.HookInput) error {
	result := make(map[string]any)

	snaps, err := sdkobjectpatch.UnmarshalToStruct[credentialSecretSnapshot](input.Snapshots, "credential_secrets")
	if err != nil {
		return err
	}

	for _, snap := range snaps {
		if snap.Name == "" {
			continue
		}

		entry := map[string]any{
			"authScheme": snap.AuthScheme,
			"secret":     snap.Secret,
		}

		if snap.Identity != "" {
			entry["identity"] = snap.Identity
		}

		result[snap.Name] = entry
	}

	input.Values.Set("cloudProviderYandex.internal.credentialSecrets", result)
	return nil
}
