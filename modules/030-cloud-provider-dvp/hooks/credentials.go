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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

// credentialSecretSnapshot is the filtered snapshot data for a credential Secret.
type credentialSecretSnapshot struct {
	Name       string `json:"name"`
	AuthScheme string `json:"authScheme"`
	Identity   string `json:"identity,omitempty"`
	Secret     string `json:"secret"`
}

func applyCredentialSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &corev1.Secret{}
	if err := sdk.FromUnstructured(obj, secret); err != nil {
		return nil, err
	}

	// Only process secrets of the correct type.
	if secret.Type != dvpCredentialSecretType {
		return nil, nil
	}

	snap := credentialSecretSnapshot{
		Name:       secret.Name,
		AuthScheme: string(secret.Data["authScheme"]),
		Identity:   string(secret.Data["identity"]),
		Secret:     string(secret.Data["secret"]),
	}
	return snap, nil
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
					MatchNames: []string{dvpNamespace},
				},
			},
			FilterFunc: applyCredentialSecretFilter,
		},
	},
}, handleCredentials)

func handleCredentials(_ context.Context, input *go_hook.HookInput) error {
<<<<<<< HEAD
	result := make(map[string]any)
=======
	result := make(map[string]interface{})
>>>>>>> 2f1a9bafe1 (migrate providerclusterconfiguration)

	snaps, err := sdkobjectpatch.UnmarshalToStruct[credentialSecretSnapshot](input.Snapshots, "credential_secrets")
	if err != nil {
		return err
	}

	for _, snap := range snaps {
		if snap.Name == "" {
			continue
		}
<<<<<<< HEAD
		entry := map[string]any{
=======
		entry := map[string]interface{}{
>>>>>>> 2f1a9bafe1 (migrate providerclusterconfiguration)
			"authScheme": snap.AuthScheme,
			"secret":     snap.Secret,
		}
		if snap.Identity != "" {
			entry["identity"] = snap.Identity
		}
		result[snap.Name] = entry
	}

	input.Values.Set("cloudProviderDvp.internal.credentialSecrets", result)
	return nil
}
