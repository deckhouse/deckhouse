/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/ee/modules/160-multitenancy-manager/hooks/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/ee/modules/160-multitenancy-manager/hooks/internal"
)

const (
	projectTypesQueue = "project_types"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: internal.ModuleQueue(projectTypesQueue),
	OnBeforeHelm: &go_hook.OrderedConfig{
		Order: 20,
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       projectTypesQueue,
			ApiVersion: internal.APIVersion,
			Kind:       internal.ProjectTypeKind,
			FilterFunc: filterProjectTypes,
		},
	},
}, handleProjectTypes)

type projectTypeSnapshot struct {
	Name string
	Spec v1alpha1.ProjectTypeSpec
}

func filterProjectTypes(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	pt := &v1alpha1.ProjectType{}
	if err := sdk.FromUnstructured(obj, pt); err != nil {
		return nil, err
	}

	return projectTypeSnapshot{
		Name: pt.Name,
		Spec: pt.Spec,
	}, nil
}

func handleProjectTypes(input *go_hook.HookInput) error {
	ptSnapshots := input.Snapshots[projectTypesQueue]
	if len(ptSnapshots) < 1 {
		return nil
	}

	projectTypesValues := make(map[string]v1alpha1.ProjectTypeSpec)
	for _, ptSnapshot := range ptSnapshots {
		if ptSnapshot == nil {
			continue
		}

		pt, ok := ptSnapshot.(projectTypeSnapshot)
		if !ok {
			input.LogEntry.Errorf("can't convert snapshot to 'projectTypeSnapshot': %v", ptSnapshot)
			continue
		}

		// TODO (alex123012): Add open-api spec validation
		if _, err := internal.LoadOpenAPISchema(pt.Spec.OpenAPI); err != nil {
			errMsg := fmt.Sprintf("can't load open api schema from '%s' ProjectType spec: %s", pt.Name, err)
			internal.SetProjectTypeStatus(input.PatchCollector, pt.Name, false, errMsg)
			continue
		}

		projectTypesValues[pt.Name] = pt.Spec

		internal.SetProjectTypeStatus(input.PatchCollector, pt.Name, true, "")
	}

	input.Values.Set(internal.ModuleValuePath(internal.PTValuesPath), projectTypesValues)
	return nil
}
