/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/values/validation/schema"
	"github.com/flant/addon-operator/sdk"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/validate"

	"github.com/deckhouse/deckhouse/ee/modules/160-multitenancy-manager/hooks/internal"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: internal.ModuleQueue(internal.ProjectsQueue),
	OnBeforeHelm: &go_hook.OrderedConfig{
		Order: 25,
	},
	Kubernetes: []go_hook.KubernetesConfig{
		internal.ProjectHookKubeConfig,
		// subscribe to ProjectTypes to update Projects when ProjectType changes
		internal.ProjectTypeHookKubeConfig,
	},
}, handleProjects)

type projectValues struct {
	Params          map[string]interface{} `json:"params"`
	ProjectTypeName string                 `json:"projectTypeName"`
	ProjectName     string                 `json:"projectName"`
}

func handleProjects(input *go_hook.HookInput) error {
	projectSnapshots := input.Snapshots[internal.ProjectsQueue]

	values := make([]projectValues, 0, len(projectSnapshots))
	for _, projectSnap := range projectSnapshots {
		project, ok := projectSnap.(internal.ProjectSnapshot)
		if !ok {
			input.LogEntry.Errorf("can't convert snapshot to 'projectSnapshot': %v", project)
			continue
		}

		if err := validateProject(input, project); err != nil {
			internal.SetErrorStatusProject(input.PatchCollector, project.Name, err.Error(), project.Conditions)
			continue
		}

		values = append(values, projectValues{
			ProjectTypeName: project.ProjectTypeName,
			ProjectName:     project.Name,
			Params:          project.Template,
		})

		internal.SetDeployingStatusProject(input.PatchCollector, project.Name, project.Conditions)
	}

	input.Values.Set(internal.ModuleValuePath(internal.ProjectValuesPath), values)
	return nil
}

func validateProject(input *go_hook.HookInput, project internal.ProjectSnapshot) error {
	ptSpecValues, ok := input.Values.GetOk(internal.ModuleValuePath(internal.PTValuesPath, project.ProjectTypeName))
	if !ok {
		return fmt.Errorf("can't find valid ProjectType '%s' for Project", project.ProjectTypeName)
	}

	ptValues := ptSpecValues.Value()
	ptValuesMap, ok := ptValues.(map[string]interface{})
	if !ok {
		return fmt.Errorf("can't convert '%s' ProjectType values to map[string]interface: %T", project.ProjectTypeName, ptValues)
	}

	sc, err := internal.LoadOpenAPISchema(ptValuesMap["openAPI"])
	if err != nil {
		return fmt.Errorf("can't load '%s' ProjectType OpenAPI schema: %v", project.ProjectTypeName, err)
	}

	sc = schema.TransformSchema(sc, &schema.AdditionalPropertiesTransformer{})
	if err := validate.AgainstSchema(sc, project.Template, strfmt.Default); err != nil {
		return fmt.Errorf("template data doesn't match the OpenAPI schema for '%s' ProjectType: %v", project.ProjectTypeName, err)
	}
	return nil
}
