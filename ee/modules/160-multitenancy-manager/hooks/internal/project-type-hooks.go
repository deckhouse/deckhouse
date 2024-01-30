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
	ProjectTypesQueue = "project_types"
)

var (
	ProjectTypeHookKubeConfig = projectTypeHookConfig(filterProjectTypes)
)

func projectTypeHookConfig(filterFunc go_hook.FilterFunc) go_hook.KubernetesConfig {
	return go_hook.KubernetesConfig{
		Name:       ProjectTypesQueue,
		ApiVersion: ProjectTypeAPIVersion,
		Kind:       ProjectTypeKind,
		FilterFunc: filterFunc,
	}
}

type ProjectTypeSnapshot struct {
	Name string
	Spec v1alpha1.ProjectTypeSpec
}

func filterProjectTypes(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	pt := &v1alpha1.ProjectType{}
	if err := sdk.FromUnstructured(obj, pt); err != nil {
		return nil, err
	}

	return ProjectTypeSnapshot{
		Name: pt.Name,
		Spec: pt.Spec,
	}, nil
}

func validateProjectType(projectType ProjectTypeSnapshot) error {
	// TODO (alex123012): Add open-api spec validation
	if _, err := LoadOpenAPISchema(projectType.Spec.OpenAPI); err != nil {
		return fmt.Errorf("can't load open api schema from '%s' ProjectType spec: %s", projectType.Name, err)
	}
	return nil
}

func GetProjectTypeSnapshots(input *go_hook.HookInput) map[string]ProjectTypeSnapshot {
	ptSnapshots := input.Snapshots[ProjectTypesQueue]

	projectTypesValues := make(map[string]ProjectTypeSnapshot)

	for _, ptSnapshot := range ptSnapshots {
		pt, ok := ptSnapshot.(ProjectTypeSnapshot)
		if !ok {
			input.LogEntry.Errorf("can't convert snapshot to 'projectTypeSnapshot': %v", ptSnapshot)
			continue
		}

		if err := validateProjectType(pt); err != nil {
			SetProjectTypeStatusError(input.PatchCollector, pt.Name, err.Error())
			continue
		}

		projectTypesValues[pt.Name] = pt

		SetProjectTypeStatusReady(input.PatchCollector, pt.Name)
	}

	return projectTypesValues
}
