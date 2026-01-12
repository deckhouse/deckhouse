// Copyright 2024 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/pkg/log"
)

type Secret struct {
	APIVersion string
	Kind       string
	Namespace  string
	Name       string
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:         "secret",
			ApiVersion:   "v1",
			Kind:         "Secret",
			NameSelector: &types.NameSelector{MatchNames: []string{"ingress-tls-v10", "prometheus-scraper-tls", "prometheus-api-client-tls"}},
			FilterFunc:   applySecretFilter,
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-monitoring"},
				},
			},
		},
	},
}, removeDeprecatedCertificateSecrets)

func applySecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret corev1.Secret
	err := sdk.FromUnstructured(obj, &secret)
	if err != nil {
		return "", err
	}

	return &Secret{
		Name:       secret.Name,
		Namespace:  secret.Namespace,
		Kind:       secret.Kind,
		APIVersion: secret.APIVersion,
	}, nil
}

func removeDeprecatedCertificateSecrets(_ context.Context, input *go_hook.HookInput) error {
	if secretSnapshot := input.Snapshots.Get("secret"); len(secretSnapshot) > 0 {
		for secret, err := range sdkobjectpatch.SnapshotIter[Secret](secretSnapshot) {
			if err != nil {
				return fmt.Errorf("cannot iterate over secret snapshot: %v", err)
			}

			log.Debug("Deleting secret", slog.String("namespace", secret.Namespace), slog.String("name", secret.Name))
			input.PatchCollector.Delete(secret.APIVersion, secret.Kind, secret.Namespace, secret.Name)
		}
	} else {
		log.Debug("Secrets not found")
	}

	return nil
}
