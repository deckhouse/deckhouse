/*
Copyright 2022 Flant JSC

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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ArgoCD uses configmap/argo-cm for auth settings. This CM can refer to data stored in secrets (in
// the same namespace) only if the secrets have label `app.kubernetes.io/part-of: argocd`.
//
// DexClient is used to point ArgoCD to deckhouse Dex as an auth provider. The DexClient creates its
// secret resource with "client secret". So, to refer to this secret from ArgoCD, we need to label
// it with the required label. That's what this hook does.
//
// ArgoCD does not dynamically read the configmap. To avoid login issues, we need to label the
// secret before ArgoCD starts.
const (
	dexClientSecretName = "dex-client-argocd"
	deliveryNamespace   = "d8-delivery"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/deckhouse/patch_dex_secret_part_of_argocd",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       dexClientSecretName,
			ApiVersion: "v1",
			Kind:       "Secret",
			NameSelector: &types.NameSelector{
				MatchNames: []string{dexClientSecretName},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{deliveryNamespace},
				},
			},
			FilterFunc: filterName,
		},
	},
}, patchSecretWithArgoLabel)

func filterName(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret corev1.Secret
	err := sdk.FromUnstructured(obj, &secret)
	if err != nil {
		return "", err
	}
	return secret.GetName(), nil
}

func patchSecretWithArgoLabel(input *go_hook.HookInput) error {
	// We know the name in advance, so we can just check if it exists.
	names, ok := input.Snapshots[dexClientSecretName]
	if !ok || len(names) != 1 {
		return nil
	}

	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]interface{}{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
	}

	input.PatchCollector.MergePatch(patch, "v1", "Secret", deliveryNamespace, dexClientSecretName)
	return nil
}
