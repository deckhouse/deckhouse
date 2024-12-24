package hooks

import (
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const observabilityMCSnapshotName = "observability_moduleconfig"

var observabilityMCManifest = map[string]interface{}{
	"apiVersion": "deckhouse.io/v1alpha1",
	"kind":       "ModuleConfig",
	"metadata": map[string]interface{}{
		"name": "observability",
	},
	"spec": map[string]interface{}{
		"enabled": "true",
	},
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       observabilityMCSnapshotName,
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "ModuleConfig",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"observability"},
			},
			FilterFunc: applyObservabilityMCFilter,
		},
	},
}, observabilityMCHookHandler)

func applyObservabilityMCFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return true, nil
}

func observabilityMCHookHandler(input *go_hook.HookInput) error {
	observabilityMCSnapshots := input.Snapshots[observabilityMCSnapshotName]

	if len(observabilityMCSnapshots) < 0 {
		input.PatchCollector.Create(&unstructured.Unstructured{
			Object: observabilityMCManifest,
		})
	}

	return nil
}
