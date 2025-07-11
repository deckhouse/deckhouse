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
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "constraint-exporter-cm",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-admission-policy-engine"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"constraint-exporter"},
			},
			FilterFunc: filterExporterCM,
		},
	},
}, dependency.WithExternalDependencies(handleValidationKinds))

func handleValidationKinds(input *go_hook.HookInput, _ dependency.Container) error {
	resourcesRaw, err := sdkobjectpatch.UnmarshalToStruct[matchData](input.NewSnapshots, "constraint-exporter-cm")
	if err != nil {
		return fmt.Errorf("failed to unmarshal constraint-exporter-cm snapshot: %w", err)
	}

	if len(resourcesRaw) == 0 {
		input.Logger.Info("no exporter cm found")
		return nil
	}

	validateRes := make([]matchResource, 0)
	mutateRes := make([]matchResource, 0)

	if err := yaml.Unmarshal([]byte(resourcesRaw[0].ValidateData), &validateRes); err != nil {
		return err
	}

	if err := yaml.Unmarshal([]byte(resourcesRaw[0].MutateData), &mutateRes); err != nil {
		return err
	}

	input.Values.Set("admissionPolicyEngine.internal.trackedConstraintResources", validateRes)
	input.Values.Set("admissionPolicyEngine.internal.trackedMutateResources", mutateRes)

	return nil
}

func filterExporterCM(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var cm corev1.ConfigMap

	err := sdk.FromUnstructured(obj, &cm)
	if err != nil {
		return nil, err
	}

	return matchData{
		ValidateData: cm.Data["validate-resources.yaml"],
		MutateData:   cm.Data["mutate-resources.yaml"],
	}, nil
}

type matchResource struct {
	APIGroups  []string `json:"apiGroups"`
	Resources  []string `json:"resources"`
	Operations []string `json:"operations,omitempty"`
}

type matchData struct {
	ValidateData string
	MutateData   string
}
