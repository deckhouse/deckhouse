/*
Copyright 2026 Flant JSC

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
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	ycicv1 "github.com/deckhouse/deckhouse/cloud-provider-yandex/pkg/api/instanceclass/v1"
	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	"github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/hooks/internal"
)

type instanceClassDefaultsFilterResult struct {
	ImageID string `json:"imageID"`
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "master_instance_class",
			ApiVersion: ycicv1.GroupVersionKind.GroupVersion().String(),
			Kind:       ycicv1.YandexInstanceClassKind,
			NameSelector: &types.NameSelector{
				MatchNames: []string{
					"master",
					cpapi.BuildInstanceClassName("master"),
				},
			},
			FilterFunc: filterMasterInstanceClass,
		},
		{
			// The migration only creates the master class once the new model is in place;
			// until then the legacy PCC is the sole source of the master image.
			Name:       "provider_cluster_configuration",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{internal.PCCSecretName},
			},
			FilterFunc: filterPCCSecret,
		},
	},
}, handleInstanceClassDefaults)

func filterMasterInstanceClass(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	// The fake k8s dynamic client ignores name selectors, so the name is re-checked here.
	if obj.GetName() != "master" && obj.GetName() != cpapi.BuildInstanceClassName("master") {
		return nil, nil
	}

	imageID, found, err := unstructured.NestedString(obj.Object, "spec", "imageID")
	if err != nil {
		return nil, fmt.Errorf("read spec.imageID of %s: %w", obj.GetName(), err)
	}
	if !found {
		return instanceClassDefaultsFilterResult{}, nil
	}

	return instanceClassDefaultsFilterResult{ImageID: imageID}, nil
}

func filterPCCSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	// The fake k8s dynamic client ignores field selectors, so we guard by name here.
	if obj.GetName() != internal.PCCSecretName {
		return nil, nil
	}

	secret := &corev1.Secret{}
	if err := sdk.FromUnstructured(obj, secret); err != nil {
		return nil, fmt.Errorf("cannot convert PCC secret from unstructured: %v", err)
	}

	pccYAML, ok := secret.Data[internal.PCCClusterConfigFilename]
	if !ok || len(pccYAML) == 0 {
		return instanceClassDefaultsFilterResult{}, nil
	}

	pccMap := make(map[string]any)
	if err := yaml.Unmarshal(pccYAML, &pccMap); err != nil {
		return nil, fmt.Errorf("cannot unmarshal PCC cluster config: %v", err)
	}

	imageID, found, err := unstructured.NestedString(pccMap, "masterNodeGroup", "instanceClass", "imageID")
	if err != nil {
		return nil, fmt.Errorf("read spec.imageID of %s: %w", obj.GetName(), err)
	}
	if !found {
		return instanceClassDefaultsFilterResult{}, nil
	}

	return instanceClassDefaultsFilterResult{ImageID: imageID}, nil
}

func handleInstanceClassDefaults(_ context.Context, input *go_hook.HookInput) error {
	masterICResult, err := getInstanceClassDefaultFilterResult(input, "master_instance_class")
	if err != nil {
		return err
	}

	pccResult, err := getInstanceClassDefaultFilterResult(input, "provider_cluster_configuration")
	if err != nil {
		return err
	}

	finalResult := mergeInstanceClassDefaultFilterResult(masterICResult, pccResult)

	input.Values.Set("cloudProviderYandex.internal.instanceClassDefaults", finalResult)

	return nil
}

func getInstanceClassDefaultFilterResult(input *go_hook.HookInput, key string) (instanceClassDefaultsFilterResult, error) {
	for _, snap := range input.Snapshots.Get(key) {
		var result instanceClassDefaultsFilterResult
		if err := snap.UnmarshalTo(&result); err != nil {
			return instanceClassDefaultsFilterResult{}, fmt.Errorf("unmarshal %s snapshot: %w", key, err)
		}
		if result.ImageID != "" {
			return result, nil
		}
	}

	return instanceClassDefaultsFilterResult{}, nil
}

func mergeInstanceClassDefaultFilterResult(res1, res2 instanceClassDefaultsFilterResult) instanceClassDefaultsFilterResult {
	if res1.ImageID == "" && res2.ImageID != "" {
		res1.ImageID = res2.ImageID
	}

	return res1
}
