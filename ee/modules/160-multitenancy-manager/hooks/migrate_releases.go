/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
)

// TODO: Remove this migration hook after release 1.55

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 25},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "projects",
			ApiVersion:                   "deckhouse.io/v1alpha1",
			Kind:                         "Project",
			WaitForSynchronization:       pointer.Bool(true),
			ExecuteHookOnSynchronization: pointer.Bool(false),
			ExecuteHookOnEvents:          pointer.Bool(false),
			FilterFunc:                   filterProjects,
		},
	},
}, patchNamespaces)

func filterProjects(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}

func patchNamespaces(input *go_hook.HookInput) (err error) {
	projects := input.Snapshots["projects"]

	for _, project := range projects {
		projectName := project.(string)
		patch := map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": map[string]string{
					"helm.sh/resource-policy":        "keep",
					"meta.helm.sh/release-name":      projectName,
					"meta.helm.sh/release-namespace": "",
				},
			},
		}
		input.PatchCollector.MergePatch(patch, "v1", "Namespace", "", projectName, object_patch.IgnoreMissingObject())
	}
	return nil
}
