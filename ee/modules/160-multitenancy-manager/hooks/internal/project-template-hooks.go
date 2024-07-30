/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package internal

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/ee/modules/160-multitenancy-manager/hooks/apis/deckhouse.io/v1alpha1"
)

const (
	ProjectTemplatesQueue = "project_templates"
)

var (
	ProjectTemplateHookKubeConfig = projectTemplateHookConfig(filterProjectTemplates)
)

func projectTemplateHookConfig(filterFunc go_hook.FilterFunc) go_hook.KubernetesConfig {
	return go_hook.KubernetesConfig{
		Name:       ProjectTemplatesQueue,
		ApiVersion: ProjectTemplateAPIVersion,
		Kind:       ProjectTemplateKind,
		FilterFunc: filterFunc,
	}
}

type ProjectTemplateSnapshot struct {
	Name string
	Spec v1alpha1.ProjectTemplateSpec
}

func filterProjectTemplates(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	projectTemplate := &v1alpha1.ProjectTemplate{}
	if err := sdk.FromUnstructured(obj, projectTemplate); err != nil {
		return nil, err
	}

	return ProjectTemplateSnapshot{
		Name: projectTemplate.Name,
		Spec: projectTemplate.Spec,
	}, nil
}

func ValidateProjectTemplate(projectTemplate ProjectTemplateSnapshot) error {
	if _, err := LoadOpenAPISchema(projectTemplate.Spec.ParametersSchema.OpenAPIV3Schema); err != nil {
		return fmt.Errorf("can't load open api schema from '%s' ProjectTemplate spec: %s", projectTemplate.Name, err)
	}
	return nil
}

func GetProjectTemplateSnapshots(input *go_hook.HookInput) map[string]ProjectTemplateSnapshot {
	ptSnapshots := input.Snapshots[ProjectTemplatesQueue]

	projectTemplatesValues := make(map[string]ProjectTemplateSnapshot)

	for _, ptSnapshot := range ptSnapshots {
		pt, ok := ptSnapshot.(ProjectTemplateSnapshot)
		if !ok {
			input.LogEntry.Errorf("can't convert snapshot to 'projectTemplateSnapshot': %v", ptSnapshot)
			continue
		}

		if err := ValidateProjectTemplate(pt); err != nil {
			SetProjectTemplateStatusError(input.PatchCollector, pt.Name, err.Error())
			continue
		}

		projectTemplatesValues[pt.Name] = pt

		SetProjectTemplateStatusReady(input.PatchCollector, pt.Name)
	}

	return projectTemplatesValues
}
