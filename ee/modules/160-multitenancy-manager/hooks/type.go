/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/ee/modules/160-multitenancy-manager/hooks/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/ee/modules/160-multitenancy-manager/hooks/internal"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: internal.ModuleQueue(internal.ProjectTypesQueue),
	OnBeforeHelm: &go_hook.OrderedConfig{
		Order: 20,
	},
	Kubernetes: []go_hook.KubernetesConfig{
		internal.ProjectTypeHookKubeConfig,
	},
}, handleProjectTypes)

func handleProjectTypes(input *go_hook.HookInput) error {
	ptSnapshots := input.Snapshots[internal.ProjectTypesQueue]

	projectTypesValues := make(map[string]v1alpha1.ProjectTypeSpec)
	for _, ptSnapshot := range ptSnapshots {
		pt, ok := ptSnapshot.(internal.ProjectTypeSnapshot)
		if !ok {
			input.LogEntry.Errorf("can't convert snapshot to 'projectTypeSnapshot': %v", ptSnapshot)
			continue
		}

		if err := validateProjectType(pt); err != nil {
			internal.SetProjectTypeStatus(input.PatchCollector, pt.Name, false, err.Error())
			continue
		}

		projectTypesValues[pt.Name] = pt.Spec

		internal.SetProjectTypeStatus(input.PatchCollector, pt.Name, true, "")
	}

	// input.Values.Set(internal.ModuleValuePath(internal.PTValuesPath), projectTypesValues)
	return nil
}

func validateProjectType(projectType internal.ProjectTypeSnapshot) error {
	// TODO (alex123012): Add open-api spec validation
	if _, err := internal.LoadOpenAPISchema(projectType.Spec.OpenAPI); err != nil {
		return fmt.Errorf("can't load open api schema from '%s' ProjectType spec: %s", projectType.Name, err)
	}
	return nil
}
