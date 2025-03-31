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
	"log/slog"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/pkg/log"
)

type Secret struct {
	apiVersion string
	kind       string
	namespace  string
	name       string
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:         "secret",
			ApiVersion:   "v1",
			Kind:         "Secret",
			NameSelector: &types.NameSelector{MatchNames: []string{"ingress-tls-v10"}},
			FilterFunc:   applySecretFilter,
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-monitoring"},
				},
			},
		},
	},
}, removeSecretGrfana)

func applySecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret corev1.Secret
	err := sdk.FromUnstructured(obj, &secret)
	if err != nil {
		return "", err
	}

	return &Secret{
		name:       secret.Name,
		namespace:  secret.Namespace,
		kind:       secret.Kind,
		apiVersion: secret.APIVersion,
	}, nil
}

func removeSecretGrfana(input *go_hook.HookInput) error {
	if secretSnapshot := input.Snapshots["secret"]; len(secretSnapshot) > 0 {
		for _, snap := range secretSnapshot {
			secret := snap.(*Secret)
			log.Debug("Deleting secret", slog.String("namespace", secret.namespace), slog.String("name", secret.name))
			input.PatchCollector.Delete(secret.apiVersion, secret.kind, secret.namespace, secret.name)
		}
	} else {
		log.Debug("Secrets not found")
	}

	return nil
}
